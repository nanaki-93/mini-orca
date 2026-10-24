package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func missionRevalidationFixture(t *testing.T, collision, commitPrevious bool) (*Service, *AgentMissionExecution, MissionExecutionRequest, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	service, _, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		output := workflowApproval
		switch request.Model {
		case "mission-create":
			output = workflowImplementation
		case "mission-test":
			output = workflowTests
		}
		messages, _ := json.Marshal(request.Messages)
		if strings.Contains(string(messages), "caller.go") {
			output = strings.NewReplacer("new.go", "caller.go", "new_test.go", "caller_test.go", "Helper", "Caller").Replace(output)
		}
		if collision && request.Model == "mission-create" {
			output = strings.Replace(output, `package main\n`, `package main\nvar Duplicate = 1\n`, 1)
		}
		writeMissionPlanningAnswer(w, output)
	}, 12, func(_ *Service, plan *agentstate.MissionPlan) {
		plan.Children[1].Dependencies = []string{}
		plan.Children[1].ReadPaths = []string{"main.go"}
		plan.Children[1].Approach = "Add independent Caller returning two and its test."
	})
	if _, err := service.TrustProjectExecution(t.Context(), prepared.Execution.Current.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, missionStartRequest(prepared)); err != nil {
		t.Fatal(err)
	}
	ready := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
	root := service.manager.Root()
	child := ready.Execution.Children[1]
	previous := MissionIntegrationRequest{ChildID: child.ID, SessionID: child.SessionID, WorkflowID: child.WorkflowID, ProposalHash: child.ProposalHash, ProposalRevision: child.ProposalRevision, Path: filepath.Join(root, ".worktrees", child.Directory), Source: agentstate.MissionBase{Head: child.BaseHead, Branch: child.Branch}}
	if commitPrevious {
		previous = *commitMissionChild(t, service, root, ready, child.ID).Integration
	}
	first := commitMissionChild(t, service, root, ready, "helper")
	advanced, err := service.ControlMissionExecution(t.Context(), ready.Execution.Mission.ID, first)
	if err != nil || advanced.Execution.Children[0].State != "integrated" || advanced.Execution.Children[1].State != "awaiting_review" || calls.Load() != 10 {
		t.Fatal("parallel fixture failed", advanced, err, calls.Load())
	}
	request := missionStartRequest(advanced)
	request.Action, request.Base = "revalidate", &advanced.Execution.Current.Base
	request.Revalidation = &MissionRevalidationRequest{Previous: previous}
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	return service, advanced, request, calls
}

func TestMissionRevalidationParallelChildrenIntegrateSequentiallyWithFreshReview(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "draft", true: "committed"}[committed], func(t *testing.T) {
			service, previous, request, calls := missionRevalidationFixture(t, false, committed)
			value, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request)
			if err != nil || value.Execution.PendingRevalidation() != nil || value.Execution.Children[1].Generation != 1 {
				t.Fatal(value, err)
			}
			entry := value.Execution.Revalidations[0]
			if entry.State != "awaiting_review" || !reflect.DeepEqual(entry.Previous, previous.Execution.Children[1]) || entry.Candidate.WorkflowID != "" || !reflect.DeepEqual(value.Execution.Mission, previous.Execution.Mission) || calls.Load() != 10 {
				t.Fatal("revalidation changed history, limits or called a provider", value, calls.Load())
			}
			root := service.manager.Root()
			worker, _, err := service.WorktreeService(t.Context(), filepath.Join(root, ".worktrees", entry.Candidate.Directory))
			if err != nil {
				t.Fatal(err)
			}
			session, err := worker.ChangeSession(t.Context(), entry.Candidate.SessionID)
			if err != nil || session.Workflow != nil || session.ReviewedHash != "" || session.Plan.ApprovedHash != "" || !requiredChecksPassed(session.Checks) || session.State != "draft" || session.MissionOrigin == nil {
				t.Fatal("candidate reused historical authority", session, err)
			}
			if _, err := worker.ApplyChange(t.Context(), session.ID, ChangeApplyRequest{ChangeIdentity: identityForChange(session), GitIdentity: *session.Git, Confirm: true}); err == nil {
				t.Fatal("applied without fresh human review")
			}
			if _, err := worker.ReviewChange(t.Context(), session.ID, identityForChange(session)); err == nil {
				t.Fatal("approved fresh plan without per-file review")
			}
			if _, err := worker.RepairChangeMessage(t.Context(), session.ID, ChangeMessageRequest{ChangeIdentity: identityForChange(session), Message: "retry"}); err == nil || calls.Load() != 10 {
				t.Fatal("revalidation bypassed the fixed provider allocation", err, calls.Load())
			}
			if _, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request); err == nil {
				t.Fatal("replayed candidate creation")
			}
			if _, err := os.Stat(filepath.Join(root, "caller.go")); !os.IsNotExist(err) {
				t.Fatal("revalidation applied source", err)
			}
			integration := commitMissionChild(t, service, root, value, "caller")
			completed, err := service.ControlMissionExecution(t.Context(), value.Execution.Mission.ID, integration)
			if err != nil || completed.Execution.State != "completed" || completed.Execution.Integrations[1].WorkflowID != "" || calls.Load() != 10 {
				t.Fatal("fresh candidate integration failed", completed, err)
			}
			if _, err := os.Stat(request.Revalidation.Previous.Path); err != nil {
				t.Fatal("lost original checkout", err)
			}
			store, err := agentstate.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			archive, err := store.ExportArtifact(t.Context(), entry.Evidence.Key, entry.Evidence.Version)
			if err != nil || !archive.Protected || archive.Hash != entry.Evidence.Hash {
				t.Fatal("lost historical model evidence", archive, err)
			}
		})
	}
}

func TestMissionRevalidationFailedChecksRetainCandidateWithoutApply(t *testing.T) {
	service, previous, request, calls := missionRevalidationFixture(t, true, false)
	value, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request)
	if err != nil || len(value.Execution.Revalidations) != 1 || value.Execution.Revalidations[0].State != "failed" || value.Execution.Children[1] != previous.Execution.Children[1] || calls.Load() != 10 {
		t.Fatal(value, err)
	}
	candidate := value.Execution.Revalidations[0].Candidate
	path := filepath.Join(service.manager.Root(), ".worktrees", candidate.Directory)
	session, err := loadChangeSession(path, candidate.SessionID)
	if err != nil || session.Workflow != nil || requiredChecksPassed(session.Checks) || session.State != "draft" {
		t.Fatal(session, err)
	}
	if _, err := os.Stat(filepath.Join(path, "caller.go")); !os.IsNotExist(err) {
		t.Fatal("checks applied proposal in the owned checkout", err)
	}
}

func TestMissionRevalidationRecoveryNeverCreatesOrExecutesCandidate(t *testing.T) {
	service, previous, request, calls := missionRevalidationFixture(t, false, false)
	root := service.manager.Root()
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := service.reserveMissionRevalidation(t.Context(), root, store, previous.Execution.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	job.effect.cancel()
	service.finishMissionEffect(job.record.ID, job.effect)
	restarted, err := New(service.config, service.manager)
	if err != nil {
		t.Fatal(err)
	}
	current, err := restarted.MissionExecution(t.Context(), settingsQuery(t, restarted), previous.Execution.Mission.ID)
	if err != nil || current.OwnerState != "waiting_for_owner" {
		t.Fatal(current, err)
	}
	request.Action = "reconcile_revalidation"
	request.ExecutionRevision = current.Execution.Revision
	request.Revalidation.RevalidationID = current.Execution.PendingRevalidation().ID
	if _, err := restarted.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request); err == nil {
		t.Fatal("recovered before owner lease expiry")
	}
	crash := current.Execution
	crash.Revalidations[0].StartedAt = time.Now().Add(-time.Duration(crash.Revalidations[0].AllowanceMillis+1) * time.Millisecond)
	writeInterruptedMissionFixture(t, root, crash)
	value, err := restarted.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request)
	if err != nil || value.Execution.PendingRevalidation() != nil || value.Execution.Revalidations[0].State != "interrupted" || value.Execution.Children[1] != previous.Execution.Children[1] || calls.Load() != 10 {
		t.Fatal(value, err)
	}
	if value.Execution.ElapsedMillis != previous.Execution.ElapsedMillis+crash.Revalidations[0].AllowanceMillis {
		t.Fatal("recovery reset elapsed allocation", value)
	}
	path := filepath.Join(root, ".worktrees", crash.Revalidations[0].Candidate.Directory)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("recovery created candidate", err)
	}
}

func TestMissionRevalidationCancellationBeforeEffectsRetainsReservation(t *testing.T) {
	for _, action := range []string{"request", "mission", "reindex", "shutdown"} {
		t.Run(action, func(t *testing.T) { testMissionRevalidationCancellation(t, action) })
	}
}

func testMissionRevalidationCancellation(t *testing.T, action string) {
	t.Helper()
	service, previous, request, calls := missionRevalidationFixture(t, false, false)
	root := service.manager.Root()
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	job, proposal, err := service.reserveMissionRevalidation(ctx, root, store, previous.Execution.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	finish := sync.OnceFunc(func() { service.finishMissionEffect(job.record.ID, job.effect) })
	defer finish()
	defer job.effect.cancel()
	var shutdown chan error
	switch action {
	case "request":
		cancel()
	case "mission":
		stop := cancelExecutionRequest(&AgentMissionExecution{Execution: job.record}, settingsQuery(t, service))
		_, err = service.ControlMissionExecution(t.Context(), job.record.Mission.ID, stop)
	case "reindex":
		_, err = service.Reindex()
	case "shutdown":
		shutdown = make(chan error, 1)
		go func() { shutdown <- service.ShutdownChangeWorkflows(ctx) }()
		select {
		case <-job.ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown did not cancel the admitted effect")
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.revalidateMissionCandidate(job.ctx, job, proposal); err == nil || context.Cause(job.ctx) == nil {
		t.Fatal("canceled owner performed effects", err)
	}
	value, err := service.finishMissionRevalidation(root, job, job.effect.id, "interrupted", "canceled")
	if err != nil || value.Execution.Revalidations[0].State != "interrupted" || calls.Load() != 10 {
		t.Fatal(value, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".worktrees", job.record.PendingRevalidation().Candidate.Directory)); !os.IsNotExist(err) {
		t.Fatal("canceled effect created checkout", err)
	}
	finish()
	if shutdown != nil {
		select {
		case err := <-shutdown:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown did not drain the finished effect")
		}
	}
}

func TestMissionRevalidationRejectsChangedIdentityAndExternalBaseBeforeEffects(t *testing.T) {
	service, previous, request, calls := missionRevalidationFixture(t, false, false)
	for name, change := range map[string]func(*MissionExecutionRequest){
		"revision": func(r *MissionExecutionRequest) { r.ExecutionRevision-- },
		"proposal": func(r *MissionExecutionRequest) { r.Revalidation.Previous.ProposalRevision++ },
		"branch":   func(r *MissionExecutionRequest) { r.Revalidation.Previous.Source.Branch = "codex/foreign" },
		"source":   func(r *MissionExecutionRequest) { r.Revalidation.Previous.Source.Head = strings.Repeat("a", 40) },
		"path":     func(r *MissionExecutionRequest) { r.Revalidation.Previous.Path += "-moved" },
		"grant":    func(r *MissionExecutionRequest) { r.ConfirmedProfiles = []string{"function"} },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := request
			input := *request.Revalidation
			invalid.Revalidation = &input
			change(&invalid)
			if _, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, invalid); err == nil {
				t.Fatal("accepted changed revalidation admission")
			}
		})
	}
	root := service.manager.Root()
	if err := os.Rename(request.Revalidation.Previous.Path, request.Revalidation.Previous.Path+"-missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request); err == nil {
		t.Fatal("recreated missing old candidate")
	}
	if err := os.Rename(request.Revalidation.Previous.Path+"-missing", request.Revalidation.Previous.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "external.txt"), []byte("external commit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "external.txt")
	testGit(t, root, "commit", "-m", "External change")
	index, err := service.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	request.ProjectRevision = index.ProjectRevision
	if _, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request); err == nil {
		t.Fatal("adopted unrelated base change")
	}
	current, err := service.MissionExecution(t.Context(), settingsQuery(t, service), previous.Execution.Mission.ID)
	if err != nil || !current.Stale || len(current.Execution.Revalidations) != 0 || calls.Load() != 10 {
		t.Fatal("rejected actions changed lineage or called provider", current, err, calls.Load())
	}
}

func TestMissionRevalidationLostArchiveRetainsCheckedButUnpublishedCandidate(t *testing.T) {
	service, previous, request, calls := missionRevalidationFixture(t, false, false)
	root := service.manager.Root()
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	job, proposal, err := service.reserveMissionRevalidation(t.Context(), root, store, previous.Execution.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	finish := sync.OnceFunc(func() { service.finishMissionEffect(job.record.ID, job.effect) })
	defer finish()
	defer job.effect.cancel()
	session, err := service.revalidateMissionCandidate(job.ctx, job, proposal)
	if err != nil || !requiredChecksPassed(session.Checks) {
		t.Fatal(session, err)
	}
	paths, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "artifacts.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal(paths, err)
	}
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.finishMissionRevalidation(root, job, job.effect.id, "awaiting_review", ""); !errors.Is(err, ErrMissionUnavailable) {
		t.Fatal("published without protected historical evidence", err)
	}
	finish()
	current, err := service.MissionExecution(t.Context(), settingsQuery(t, service), previous.Execution.Mission.ID)
	if err != nil || current.OwnerState != "recovery_required" || current.Execution.Children[1] != previous.Execution.Children[1] {
		t.Fatal(current, err)
	}
	entry := current.Execution.PendingRevalidation()
	if entry == nil || entry.Candidate.SessionID != session.ID {
		t.Fatal("lost checked candidate identity", entry)
	}
	request.Action, request.ExecutionRevision = "reconcile_revalidation", current.Execution.Revision
	request.Revalidation.RevalidationID = entry.ID
	recovered, err := service.ControlMissionExecution(t.Context(), previous.Execution.Mission.ID, request)
	if err != nil || recovered.Execution.Revalidations[0].State != "interrupted" || calls.Load() != 10 {
		t.Fatal(recovered, err)
	}
	path := filepath.Join(root, ".worktrees", entry.Candidate.Directory)
	if _, err := loadChangeSession(path, session.ID); err != nil {
		t.Fatal("recovery deleted candidate evidence", err)
	}
	if _, err := os.Stat(filepath.Join(root, "caller.go")); !os.IsNotExist(err) {
		t.Fatal("publication failure applied source", err)
	}
}
