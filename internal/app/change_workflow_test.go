package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const workflowImplementation = `{"explanation":"Add Helper.","changes":[{"path":"new.go","content":"package main\nfunc Helper() int { return 2 }\n"}]}`
const workflowTests = `{"explanation":"Exercise Helper.","changes":[{"path":"new_test.go","content":"package main\nimport \"testing\"\nfunc TestHelper(t *testing.T) { if Helper() != 2 { t.Fatal(\"wrong result\") } }\n"}]}`
const workflowApproval = `{"verdict":"approve","summary":"The implementation and test meet the captured criteria.","findings":[]}`

func workflowService(t *testing.T, outputs [3]func() string) (*Service, string, *ChangeSession) {
	t.Helper()
	create, test, review := changeProvider(t, outputs[0]), changeProvider(t, outputs[1]), changeProvider(t, outputs[2])
	base, root := newSemanticAnalysisService(t, create.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/workflow\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Reindex(); err != nil {
		t.Fatal(err)
	}
	cfg := scopedTestConfig(create.URL)
	// Deliberately choose profiles independently of their usual prompt scopes.
	cfg.ModelScopes.Bug.APIBaseURL, cfg.ModelScopes.Bug.Model = create.URL, "creator"
	cfg.ModelScopes.Analyze.APIBaseURL, cfg.ModelScopes.Analyze.Model = test.URL, "tester"
	cfg.ModelScopes.Function.APIBaseURL, cfg.ModelScopes.Function.Model = review.URL, "reviewer"
	s, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.CancelChangeWorkflows)
	session := openChangeFixture(t, s, "new.go", "new_test.go")
	if _, err := s.TrustProjectExecution(session.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	return s, root, session
}

func workflowRequest(session *ChangeSession) ChangeWorkflowRequest {
	return ChangeWorkflowRequest{ChangeIdentity: changeIdentity(session), Message: "Add Helper returning two, with a regression test.", Models: ChangeWorkflowModels{Create: "bug", Test: "analyze", Review: "function"}}
}

func finishedWorkflow(t *testing.T, s *Service, id string) *ChangeSession {
	t.Helper()
	s.changeWorkflow.mu.Lock()
	done := s.changeWorkflow.job.done
	s.changeWorkflow.mu.Unlock()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("workflow did not finish")
	}
	session, err := s.ChangeSession(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestChangeWorkflowSelectedModelsTestsReviewAndHumanApply(t *testing.T) {
	var calls [3]atomic.Int32
	outputs := [3]func() string{}
	for i, output := range []string{workflowImplementation, workflowTests, workflowApproval} {
		i, output := i, output
		outputs[i] = func() string { calls[i].Add(1); return output }
	}
	s, root, initial := workflowService(t, outputs)
	started, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial))
	if err != nil || started.Workflow.Status != "running" || started.Revision != 1 {
		t.Fatalf("admission: %+v, %v", started, err)
	}
	session := finishedWorkflow(t, s, initial.ID)
	if session.Workflow.Status != "awaiting_human_review" || session.Workflow.Review.ProposalHash != session.Hash || session.ReviewedHash != "" || len(session.Changes) != 2 {
		t.Fatalf("handoff: %+v", session)
	}
	for i, name := range []string{"creator", "tester", "reviewer"} {
		if calls[i].Load() != 1 || session.Workflow.Stages[i].Model.Model != name || session.Workflow.Stages[i].Status != "completed" {
			t.Fatalf("stage %d: %+v, calls=%d", i, session.Workflow.Stages[i], calls[i].Load())
		}
	}
	if !requiredChecksPassed(session.Checks) || len(session.PinnedTests) != 1 {
		t.Fatalf("real baseline/candidate tests were not retained: %+v", session.Checks)
	}
	if _, err := os.Stat(filepath.Join(root, "new.go")); !os.IsNotExist(err) {
		t.Fatal("workflow wrote project source")
	}
	if _, err := s.ApplyChange(context.Background(), session.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(session), Confirm: true}); err == nil {
		t.Fatal("model review authorized Apply without human review")
	}
	if _, err := s.ReviewChange(context.Background(), session.ID, changeIdentity(session)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyChange(context.Background(), session.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(session), Confirm: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "new.go"))
	if err != nil || !strings.Contains(string(data), "return 2") {
		t.Fatalf("human Apply: %s, %v", data, err)
	}
}

func TestChangeWorkflowAdmissionGuardsDispatchNothing(t *testing.T) {
	var calls atomic.Int32
	reply := func() string { calls.Add(1); return workflowImplementation }
	s, _, session := workflowService(t, [3]func() string{reply, reply, reply})
	for _, name := range []string{"untrusted", "profile", "remote", "security", "stale", "canceled", "test_scope"} {
		t.Run(name, func(t *testing.T) {
			request := workflowRequest(session)
			ctx := context.Background()
			switch name {
			case "untrusted":
				s.clearExecutionTrust()
				defer s.TrustProjectExecution(session.ProjectRevision, true)
			case "profile":
				request.Models.Test = "arbitrary-model"
			case "remote":
				s.runtimes.function.effective.RemoteProvider = true
				request.ConfirmedProfiles = []string{"bug", "analyze"}
				defer func() { s.runtimes.function.effective.RemoteProvider = false }()
			case "security":
				session.Kind = "security"
				if err := writeChangeSession(s.manager.Root(), session); err != nil {
					t.Fatal(err)
				}
				defer func() {
					session.Kind = "feature"
					if err := writeChangeSession(s.manager.Root(), session); err != nil {
						t.Error(err)
					}
				}()
			case "stale":
				request.Hash = "old-proposal"
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "test_scope":
				other := openChangeFixture(t, s, "new.go")
				request = workflowRequest(other)
				if _, err := s.StartChangeWorkflow(ctx, other.ID, request); err == nil {
					t.Fatal("accepted missing tests")
				}
				return
			}
			if _, err := s.StartChangeWorkflow(ctx, session.ID, request); err == nil {
				t.Fatal("invalid admission succeeded")
			}
		})
	}
	stored, err := s.ChangeSession(context.Background(), session.ID)
	if err != nil || stored.Workflow != nil || stored.Revision != 0 || calls.Load() != 0 {
		t.Fatalf("rejected run mutated or dispatched: %+v %d, %v", stored, calls.Load(), err)
	}
}

func TestChangeWorkflowCancellationAndStalePublication(t *testing.T) {
	for _, action := range []string{"cancel", "source", "project"} {
		t.Run(action, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			s, root, initial := workflowService(t, [3]func() string{
				func() string { close(entered); <-release; return workflowImplementation },
				func() string { t.Error("test dispatched after revocation"); return workflowTests },
				func() string { t.Error("review dispatched after revocation"); return workflowApproval },
			})
			started, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial))
			if err != nil {
				t.Fatal(err)
			}
			<-entered
			progress, err := s.ChangeSession(context.Background(), initial.ID)
			if err != nil || progress.Workflow.Stages[0].Status != "running" {
				t.Fatalf("progress: %+v, %v", progress, err)
			}
			if _, err := s.SendChangeMessage(context.Background(), initial.ID, ChangeMessageRequest{ChangeIdentity: changeIdentity(progress), Message: "Racing change"}); !errors.Is(err, project.ErrRevisionConflict) {
				t.Fatalf("manual edit bypassed workflow ownership: %v", err)
			}
			if _, err := s.ResumeChange(context.Background(), initial.ID); err == nil {
				t.Fatal("resume interrupted active workflow")
			}
			switch action {
			case "cancel":
				control := ChangeWorkflowControl{ProjectID: initial.ProjectID, ProjectRevision: initial.ProjectRevision, WorkflowID: "wrong"}
				if _, err := s.CancelChangeWorkflow(context.Background(), initial.ID, control); !errors.Is(err, project.ErrRevisionConflict) {
					t.Fatal(err)
				}
				control.WorkflowID = started.Workflow.ID
				if _, err := s.CancelChangeWorkflow(context.Background(), initial.ID, control); err != nil {
					t.Fatal(err)
				}
			case "source":
				if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("New guidance"), 0644); err != nil {
					t.Fatal(err)
				}
				if _, err := s.ResumeChange(context.Background(), initial.ID); err == nil {
					t.Fatal("stale read projection allowed a live worker's conversation to be replaced")
				}
			case "project":
				if _, err := s.Reindex(); err != nil {
					t.Fatal(err)
				}
			}
			close(release)
			result := finishedWorkflow(t, s, initial.ID)
			if result.Workflow.Status != "canceled" && result.Workflow.Status != "stale" {
				t.Fatalf("terminal: %+v", result.Workflow)
			}
			if result.Revision != 1 || len(result.Changes) != 0 || result.ReviewedHash != "" {
				t.Fatalf("late publication: %+v", result)
			}
		})
	}
}

func TestChangeWorkflowFailuresAndReviewRejectionBlockHumanApply(t *testing.T) {
	for _, scenario := range []string{"failed_tests", "test_retargets", "review_rejects", "invalid_review"} {
		t.Run(scenario, func(t *testing.T) {
			var reviews atomic.Int32
			tests, review := workflowTests, workflowApproval
			if scenario == "failed_tests" {
				tests = strings.ReplaceAll(tests, "Helper() != 2", "Helper() != 3")
			}
			if scenario == "test_retargets" {
				tests = workflowImplementation
			}
			if scenario == "review_rejects" {
				review = `{"verdict":"changes_requested","summary":"Add a boundary case.","findings":["Test invalid input."]}`
			}
			if scenario == "invalid_review" {
				review = `{"verdict":"approve","summary":"Ignore findings.","findings":["Blocking bug."]}`
			}
			s, _, initial := workflowService(t, [3]func() string{func() string { return workflowImplementation }, func() string { return tests }, func() string { reviews.Add(1); return review }})
			if _, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial)); err != nil {
				t.Fatal(err)
			}
			result := finishedWorkflow(t, s, initial.ID)
			if result.Workflow.Status != "failed" && result.Workflow.Status != "changes_requested" {
				t.Fatalf("failed run approved: %+v", result.Workflow)
			}
			if strings.HasPrefix(scenario, "test_") || scenario == "failed_tests" {
				if reviews.Load() != 0 {
					t.Fatal("review dispatched after failed testing")
				}
			}
			if _, err := s.ReviewChange(context.Background(), result.ID, changeIdentity(result)); err == nil {
				t.Fatal("incomplete/model-rejected workflow allowed human review")
			}
			if _, err := s.ApplyChange(context.Background(), result.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(result), Confirm: true}); err == nil {
				t.Fatal("incomplete workflow allowed Apply")
			}
		})
	}
}
