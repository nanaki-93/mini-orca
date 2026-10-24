package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func missionSchedulerFixture(t *testing.T, handler http.HandlerFunc, calls int, configure ...func(*Service, *agentstate.MissionPlan)) (*Service, string, *AgentMissionExecution) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	base, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/mission\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "unselected.go"), []byte("package main\nfunc UnselectedSecret() string { return \"PRIVATE_SIBLING_BODY\" }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := base.config
	cfg.ModelScopes.Bug.Model, cfg.ModelScopes.Function.Model, cfg.ModelScopes.Analyze.Model = "mission-create", "mission-test", "mission-review"
	service, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := service.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	initializeChangeGit(t, service, root)
	query := settingsQuery(t, service)
	limit := agentstate.MissionLimits{ModelCalls: calls, ToolCalls: 24, InputTokens: 1 << 20, OutputTokens: 1 << 20, ElapsedSeconds: 120, ConcurrentCalls: 1}
	budget := limit
	budget.ModelCalls *= 2
	budget.ToolCalls *= 2
	budget.InputTokens *= 2
	budget.OutputTokens *= 2
	budget.ConcurrentCalls = 2
	source, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	plan := agentstate.MissionPlan{Goal: "Implement a helper and its dependent caller", Limits: budget, Sources: []agentstate.MissionSource{{Path: "main.go", Exists: true, Hash: source.ContentHash}, {Path: "new.go"}, {Path: "new_test.go"}, {Path: "caller.go"}, {Path: "caller_test.go"}}, IntegrationOrder: []string{"helper", "caller"}, Children: []agentstate.MissionChild{
		{ID: "helper", Title: "Create Helper", Approach: "Add Helper returning two and its regression test.", AcceptanceCriteria: []string{"Helper returns two"}, WritePaths: []string{"new.go", "new_test.go"}, ReadPaths: []string{"main.go"}, Dependencies: []string{}, Limits: limit},
		{ID: "caller", Title: "Use Helper", Approach: "Add Caller that calls Helper and its regression test.", AcceptanceCriteria: []string{"Caller returns Helper result"}, WritePaths: []string{"caller.go", "caller_test.go"}, ReadPaths: []string{"new.go"}, Dependencies: []string{"helper"}, Limits: limit},
	}}
	for _, apply := range configure {
		apply(service, &plan)
	}
	saved, err := service.MutateMission(t.Context(), "", MissionMutation{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "create", Plan: &plan})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"helper", "caller", ""} {
		saved, err = service.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, id))
		if err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, prepareExecutionRequest(t, service, saved))
	if err != nil {
		t.Fatal(err)
	}
	return service, root, prepared
}

func missionStartRequest(value *AgentMissionExecution) MissionExecutionRequest {
	record := value.Execution
	return MissionExecutionRequest{ProjectID: record.Mission.ProjectID, ProjectRevision: record.Mission.ProjectRevision, Action: "start", PlanHash: record.Mission.PlanHash, ExecutionID: record.ID, ExecutionRevision: record.Revision, Confirm: true, ConfirmExecution: true}
}

func missionFixtureAnswer(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var request llm.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t.Error(err)
		return
	}
	data, err := json.Marshal(request.Messages)
	if err != nil {
		t.Error(err)
		return
	}
	if strings.Contains(string(data), "UnselectedSecret") || strings.Contains(string(data), "PRIVATE_SIBLING_BODY") {
		t.Error("mission prompt included an unselected sibling")
	}
	output := workflowApproval
	switch request.Model {
	case "mission-create":
		output = workflowImplementation
	case "mission-test":
		output = workflowTests
	case "mission-review":
	default:
		t.Error("mission used an unselected profile", request.Model)
	}
	writeMissionPlanningAnswer(w, output)
}

func waitMissionFixture(t *testing.T, service *Service, id string) *AgentMissionExecution {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := service.waitMissionJobs(ctx); err != nil {
		t.Fatal(err)
	}
	value, err := service.MissionExecution(t.Context(), settingsQuery(t, service), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestMissionSchedulerProducesCheckedChildAndBlocksUnintegratedDependencies(t *testing.T) {
	var calls atomic.Int32
	service, root, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); missionFixtureAnswer(t, w, r) }, 12)
	request := missionStartRequest(prepared)
	if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err == nil {
		t.Fatal("untrusted mission started")
	}
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	started, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request)
	if err != nil || started.Execution.State != "running" {
		t.Fatal(started, err)
	}
	result := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
	if result.Stale || result.Execution.State != "awaiting_review" || result.OwnerState != "idle" || len(result.ReadyChildren) != 0 || result.Execution.Children[0].State != "awaiting_review" || result.Execution.Children[1].State != "pending" {
		t.Fatalf("unexpected mission outcome: %+v", result)
	}
	if calls.Load() != 5 {
		t.Fatal("wrong child/provider dispatch count", calls.Load())
	}
	child := result.Execution.Children[0]
	path := filepath.Join(root, ".worktrees", child.Directory)
	worker, _, err := service.WorktreeService(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := worker.ChangeSession(t.Context(), child.SessionID)
	if err != nil || proposal.MissionOrigin == nil || proposal.MissionOrigin.ExecutionID != result.Execution.ID || proposal.Hash != child.ProposalHash || proposal.Workflow.Status != "awaiting_human_review" || !requiredChecksPassed(proposal.Checks) || proposal.ReviewedHash != "" {
		t.Fatal("child did not produce checked reviewable evidence", proposal, err)
	}
	for _, checkout := range []string{root, path} {
		if _, err := os.Stat(filepath.Join(checkout, "new.go")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("mission applied source automatically", err)
		}
	}
	if _, err := worker.ApplyChange(t.Context(), proposal.ID, ChangeApplyRequest{ChangeIdentity: identityForChange(proposal), Confirm: true}); err == nil {
		t.Fatal("mission bypassed human review")
	}
	if _, err := worker.StartChangeWorkflow(t.Context(), proposal.ID, ChangeWorkflowRequest{ChangeIdentity: identityForChange(proposal), Message: "Retry", Models: proposal.Workflow.Models, PlanHash: proposal.Plan.Hash}); err == nil {
		t.Fatal("ordinary workflow reset mission allocation")
	}
	if _, err := worker.SendChangeMessage(t.Context(), proposal.ID, ChangeMessageRequest{ChangeIdentity: identityForChange(proposal), Message: "Retry"}); err == nil {
		t.Fatal("ordinary generation reset mission allocation")
	}
	if calls.Load() != 5 {
		t.Fatal("rejected retry dispatched", calls.Load())
	}
	if _, err := service.ControlMissionExecution(t.Context(), result.Execution.Mission.ID, missionStartRequest(result)); err == nil {
		t.Fatal("unapplied dependency released caller")
	}
	if result.Execution.ElapsedMillis < 1 || result.Execution.ElapsedMillis > 120000 {
		t.Fatal("invalid active execution accounting", result.Execution.ElapsedMillis)
	}
}

func TestMissionSchedulerBudgetFailureCannotResetChildReservation(t *testing.T) {
	var calls atomic.Int32
	service, _, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); missionFixtureAnswer(t, w, r) }, 1)
	request := missionStartRequest(prepared)
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err != nil {
		t.Fatal(err)
	}
	result := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
	if calls.Load() != 1 || result.Execution.State != "blocked" || result.Execution.Children[0].State != "failed" || result.Execution.Children[1].State != "pending" {
		t.Fatalf("budget escaped: calls=%d result=%+v", calls.Load(), result)
	}
	if _, err := service.ControlMissionExecution(t.Context(), result.Execution.Mission.ID, missionStartRequest(result)); err == nil {
		t.Fatal("failed child received new allocation")
	}
	if calls.Load() != 1 {
		t.Fatal("retry called provider", calls.Load())
	}
}

func TestMissionSchedulerPauseCancelAndShutdownDrainProviderRequests(t *testing.T) {
	for _, action := range []string{"pause", "cancel", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			entered, exited := make(chan struct{}), make(chan struct{})
			release := make(chan struct{})
			service, _, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
				}
				close(entered)
				select {
				case <-r.Context().Done():
				case <-release:
				}
				close(exited)
			}, 12)
			t.Cleanup(func() { close(release) })
			request := missionStartRequest(prepared)
			if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(15 * time.Second):
				t.Fatal("child did not dispatch")
			}
			if action == "shutdown" {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				if err := service.ShutdownChangeWorkflows(ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				current, err := service.MissionExecution(t.Context(), settingsQuery(t, service), prepared.Execution.Mission.ID)
				if err != nil {
					t.Fatal(err)
				}
				control := cancelExecutionRequest(current, settingsQuery(t, service))
				control.Action = action
				if _, err := service.ControlMissionExecution(t.Context(), current.Execution.Mission.ID, control); err != nil {
					t.Fatal(err)
				}
			}
			result := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				t.Fatal("cancellation left provider request running")
			}
			want := map[string]string{"pause": "paused", "cancel": "canceled", "shutdown": "blocked"}[action]
			if result.Execution.State != want || result.Execution.Children[0].WaveID == "" || result.Execution.Children[1].WaveID != "" {
				t.Fatalf("control lost ownership: %+v", result)
			}
			if result.Execution.Children[0].State == "pending" || result.Execution.Children[0].State == "running" {
				t.Fatal("canceled reservation reset or left running")
			}
		})
	}
}

func TestMissionSchedulerStartsDisjointChildrenWithinTheApprovedConcurrency(t *testing.T) {
	for _, slots := range []int{1, 2} {
		t.Run(string(rune('0'+slots)), func(t *testing.T) {
			entered, release := make(chan struct{}, 2), make(chan struct{})
			var calls atomic.Int32
			service, _, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
				}
				calls.Add(1)
				entered <- struct{}{}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}, 12, func(_ *Service, plan *agentstate.MissionPlan) {
				plan.Children[1].Dependencies = []string{}
				plan.Children[1].ReadPaths = []string{"main.go"}
				plan.Limits.ConcurrentCalls = slots
			})
			t.Cleanup(func() { close(release) })
			request := missionStartRequest(prepared)
			if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
				t.Fatal(err)
			}
			started, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request)
			if err != nil || len(started.Execution.Wave.Children) != slots {
				t.Fatal(started, err)
			}
			for range slots {
				select {
				case <-entered:
				case <-time.After(15 * time.Second):
					t.Fatal("admitted disjoint child did not dispatch")
				}
			}
			current, err := service.MissionExecution(t.Context(), settingsQuery(t, service), prepared.Execution.Mission.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.ControlMissionExecution(t.Context(), current.Execution.Mission.ID, cancelExecutionRequest(current, settingsQuery(t, service))); err != nil {
				t.Fatal(err)
			}
			finished := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
			if calls.Load() != int32(slots) || finished.Execution.State != "canceled" {
				t.Fatal(calls.Load(), finished)
			}
			if slots == 1 && finished.Execution.Children[1].State != "pending" {
				t.Fatal("unallocated child dispatched")
			}
		})
	}
}

func TestMissionSchedulerCapturesRolePreferencesAndRequiresSelectedRemoteConsent(t *testing.T) {
	var calls atomic.Int32
	service, _, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		call := calls.Add(1)
		model, output := "mission-create", workflowApproval
		if call == 1 {
			model, output = "mission-test", workflowImplementation
		}
		if call == 2 {
			model, output = "mission-review", workflowTests
		}
		if request.Model != model {
			t.Errorf("child lost captured role: call %d model %s want %s", call, request.Model, model)
		}
		writeMissionPlanningAnswer(w, output)
	}, 12, func(service *Service, _ *agentstate.MissionPlan) {
		control := settingsControl(t, service)
		control.Values.Roles.Creation, control.Values.Roles.Testing, control.Values.Roles.Review = "function", "analyze", "bug"
		if _, err := service.ControlAgentSettings(t.Context(), control); err != nil {
			t.Fatal(err)
		}
	})
	service.runtimes.analyze.effective.RemoteProvider = true
	service.runtimes.bug.effective.RemoteProvider = true
	service.runtimes.function.effective.RemoteProvider = true
	request := missionStartRequest(prepared)
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	for _, grants := range [][]string{nil, {"function"}, {"function", "function"}, {"unknown"}} {
		request.ConfirmedProfiles = grants
		if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err == nil {
			t.Fatal("incomplete or forged consent accepted", grants)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected consent dispatched")
	}
	current, err := service.MissionExecution(t.Context(), settingsQuery(t, service), prepared.Execution.Mission.ID)
	if err != nil || current.Execution.Revision != prepared.Execution.Revision {
		t.Fatal("rejected consent consumed allocation", current, err)
	}
	request.ConfirmedProfiles = []string{"function", "analyze", "bug"}
	if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err != nil {
		t.Fatal(err)
	}
	result := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
	if result.Execution.Children[0].State != "awaiting_review" || calls.Load() != 5 {
		t.Fatal(result, calls.Load())
	}
}

func TestMissionSchedulerRechecksAdmissionBeforeLateResultsAndFurtherDispatch(t *testing.T) {
	for _, change := range []string{"source", "settings", "reindex"} {
		t.Run(change, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			service, root, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					close(entered)
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				missionFixtureAnswer(t, w, r)
			}, 12)
			request := missionStartRequest(prepared)
			if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(15 * time.Second):
				t.Fatal("child did not dispatch")
			}
			var err error
			switch change {
			case "source":
				err = os.WriteFile(filepath.Join(root, "unselected.go"), []byte("package main\n// changed during execution\n"), 0600)
			case "settings":
				control := settingsControl(t, service)
				control.Values.Limits.ModelCalls--
				_, err = service.ControlAgentSettings(t.Context(), control)
			case "reindex":
				_, err = service.Reindex()
			}
			close(release)
			if err != nil {
				t.Fatal(err)
			}
			result := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
			if calls.Load() != 1 || result.Execution.Children[0].State != "interrupted" || result.Execution.Children[1].State != "pending" || result.Execution.State != "blocked" {
				t.Fatal("late result escaped admission", calls.Load(), result)
			}
		})
	}
}
