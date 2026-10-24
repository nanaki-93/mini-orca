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

func missionIntegrationFixture(t *testing.T) (*Service, string, *AgentMissionExecution, MissionExecutionRequest, *atomic.Int32) {
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
		if calls.Load() > 5 {
			output = strings.NewReplacer("new.go", "caller.go", "new_test.go", "caller_test.go", "Helper", "Caller").Replace(output)
		}
		writeMissionPlanningAnswer(w, output)
	}, 12)
	request := missionStartRequest(prepared)
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request); err != nil {
		t.Fatal(err)
	}
	ready := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
	root := service.manager.Root()
	return service, root, ready, commitMissionChild(t, service, root, ready, "helper"), calls
}

func commitMissionChild(t *testing.T, service *Service, root string, value *AgentMissionExecution, id string) MissionExecutionRequest {
	t.Helper()
	record := value.Execution
	child, _, err := missionChildAt(record, id)
	if err != nil || child.State != "awaiting_review" {
		t.Fatalf("child unavailable: %+v %v", child, err)
	}
	path := filepath.Join(root, ".worktrees", child.Directory)
	worker, _, err := service.WorktreeService(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	session, err := worker.ChangeSession(t.Context(), child.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range session.Changes {
		session, err = worker.ReviewChangeFile(t.Context(), session.ID, ChangeFileReviewRequest{ChangeIdentity: changeIdentity(session), Path: edit.Path})
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err = worker.ReviewChange(t.Context(), session.ID, changeIdentity(session))
	if err != nil {
		t.Fatal(err)
	}
	mutation, err := worker.ApplyChange(t.Context(), session.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(session), GitIdentity: *session.Git, Confirm: true, CommitMessage: "Implement mission child " + id})
	if err != nil || mutation.Commit == nil || mutation.Commit.State != "committed" {
		t.Fatal(mutation, err)
	}
	return MissionExecutionRequest{ProjectID: record.Mission.ProjectID, ProjectRevision: record.Current.ProjectRevision, Action: "integrate", PlanHash: record.Mission.PlanHash, ExecutionID: record.ID, ExecutionRevision: record.Revision, Base: &record.Current.Base, Confirm: true, ConfirmExecution: true, Integration: &MissionIntegrationRequest{ChildID: child.ID, SessionID: child.SessionID, WorkflowID: child.WorkflowID, ProposalHash: child.ProposalHash, ProposalRevision: child.ProposalRevision, Path: path, Source: agentstate.MissionBase{Head: mutation.Commit.OID, Branch: child.Branch}}}
}

func TestMissionIntegrationAdvancesOnlyVerifiedCheckpointAndDependentChild(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	original, allocations := ready.Execution.Mission, ready.Execution.Children
	value, err := service.ControlMissionExecution(t.Context(), original.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, ready, value, request, "ready")
	if !reflect.DeepEqual(value.ReadyChildren, []string{"caller"}) || value.Execution.Children[1].State != "pending" || !reflect.DeepEqual(value.Execution.Mission, original) {
		t.Fatal("integration changed plan or dependency admission", value)
	}
	if _, err := service.TrustProjectExecution(t.Context(), value.Execution.Current.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ControlMissionExecution(t.Context(), original.ID, missionStartRequest(value)); err != nil {
		t.Fatal(err)
	}
	second := waitMissionFixture(t, service, original.ID)
	if second.Execution.Children[1].BaseHead != value.Execution.Current.Base.Head {
		t.Fatal("dependent child did not use integrated source")
	}
	request = commitMissionChild(t, service, root, second, "caller")
	completed, err := service.ControlMissionExecution(t.Context(), original.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, second, completed, request, "completed")
	if calls.Load() != 10 || len(completed.ReadyChildren) != 0 || completed.Execution.Mission.PlanHash != original.PlanHash {
		t.Fatal("integration dispatched providers or changed approved identity", calls.Load(), completed)
	}
	for index, child := range completed.Execution.Children {
		if child.Limits != allocations[index].Limits {
			t.Fatal("integration changed fixed allocation")
		}
	}
}

func assertIntegratedMission(t *testing.T, root string, before, value *AgentMissionExecution, request MissionExecutionRequest, state string) {
	t.Helper()
	if value.Stale || value.Execution.State != state || value.Execution.Current.Base.Head != request.Integration.Source.Head || value.Execution.Base != before.Execution.Base || value.Git == nil || value.Git.RemovedWorktree != request.Integration.Path || value.Git.RemovedBranch != request.Integration.Source.Branch || len(value.Git.Warnings) != 0 {
		t.Fatalf("integration receipt: %+v git=%+v", value, value.Git)
	}
	entry := value.Execution.Integrations[len(value.Execution.Integrations)-1]
	if entry.State != "merged" || entry.Cleanup != "removed" || value.Execution.Children[len(value.Execution.Integrations)-1].State != "integrated" {
		t.Fatal("missing durable integration", entry)
	}
	if _, err := os.Stat(request.Integration.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned worktree not removed", err)
	}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := store.ExportArtifact(t.Context(), entry.Evidence.Key, entry.Evidence.Version)
	if err != nil || artifact.Hash != entry.Evidence.Hash || !artifact.Protected || !strings.Contains(string(artifact.Data), `"state":"applied"`) {
		t.Fatal("complete checked proposal not retained", artifact, err)
	}
	if _, err := store.MissionExecution(t.Context(), value.Execution.Mission.ProjectID, value.Execution.Mission.ID); err != nil {
		t.Fatal("integration unreadable after reopening", err)
	}
}

func TestMissionIntegrationRejectsForeignStaleAndOrdinaryGitRequests(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	for name, change := range map[string]func(*MissionExecutionRequest){
		"unconfirmed":      func(r *MissionExecutionRequest) { r.ConfirmExecution = false },
		"stale revision":   func(r *MissionExecutionRequest) { r.ExecutionRevision++ },
		"foreign child":    func(r *MissionExecutionRequest) { r.Integration.ChildID = "caller" },
		"foreign proposal": func(r *MissionExecutionRequest) { r.Integration.ProposalHash = strings.Repeat("b", 64) },
		"foreign path":     func(r *MissionExecutionRequest) { r.Integration.Path = root },
		"wrong base":       func(r *MissionExecutionRequest) { r.Base.Head = strings.Repeat("b", 40) },
		"provider grant":   func(r *MissionExecutionRequest) { r.ConfirmedProfiles = []string{"mission-create"} },
	} {
		t.Run(name, func(t *testing.T) {
			candidate, base, input := request, *request.Base, *request.Integration
			candidate.Base, candidate.Integration = &base, &input
			change(&candidate)
			if _, err := service.ControlMissionExecution(t.Context(), ready.Execution.Mission.ID, candidate); err == nil {
				t.Fatal("unsafe integration accepted")
			}
		})
	}
	for _, action := range []func(context.Context, GitMutationRequest) (*GitMutationResult, error){service.MergeGitWorktree, service.CleanupMergedWorktree, service.DiscardGitWorktree} {
		if _, err := action(t.Context(), missionGitRequest(request)); !errors.Is(err, ErrMissionConflict) {
			t.Fatal("ordinary Git route bypassed mission ownership", err)
		}
	}
	if testGit(t, root, "rev-parse", "HEAD") != ready.Execution.Base.Head || calls.Load() != 5 {
		t.Fatal("rejection mutated base or dispatched provider")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			_, err := service.ControlMissionExecution(t.Context(), ready.Execution.Mission.ID, request)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("integration replay won more than once", success)
	}
}

func interruptedMissionIntegration(t *testing.T, service *Service, root string, ready *AgentMissionExecution, request MissionExecutionRequest) agentstate.MissionExecution {
	t.Helper()
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	session, err := loadChangeSession(request.Integration.Path, request.Integration.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := archiveMissionProposal(t.Context(), store, ready.Execution, session)
	if err != nil {
		t.Fatal(err)
	}
	input := request.Integration
	record, err := store.BeginMissionIntegration(t.Context(), missionExecutionIdentityFor(ready.Execution), service.missions.owner, agentstate.MissionIntegrationInput{ChildID: input.ChildID, Source: input.Source, SessionID: input.SessionID, WorkflowID: input.WorkflowID, ProposalHash: input.ProposalHash, ProposalRevision: input.ProposalRevision, Evidence: evidence}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestMissionIntegrationReconcilesRecordedIntentWithoutRetryingGitOrProviders(t *testing.T) {
	for _, merged := range []bool{false, true} {
		t.Run(map[bool]string{false: "before merge", true: "after merge"}[merged], func(t *testing.T) {
			service, root, ready, request, calls := missionIntegrationFixture(t)
			record := interruptedMissionIntegration(t, service, root, ready, request)
			if merged {
				testGit(t, root, "merge", "--ff-only", request.Integration.Source.Head)
			}
			if _, err := service.Reindex(); err != nil {
				t.Fatal(err)
			}
			query := settingsQuery(t, service)
			if _, err := service.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
				t.Fatal(err)
			}
			request.Action, request.ExecutionRevision, request.ProjectRevision = "reconcile_integration", record.Revision, query.ProjectRevision
			request.Integration.IntegrationID = record.PendingIntegration().ID
			repo, err := service.GitRepository(t.Context(), query.ProjectID, query.ProjectRevision)
			if err != nil {
				t.Fatal(err)
			}
			request.Base = &agentstate.MissionBase{Head: repo.Head, Branch: repo.Branch}
			restarted, err := New(service.config, service.manager)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := restarted.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
				t.Fatal(err)
			}
			if _, err := restarted.ControlMissionExecution(t.Context(), record.Mission.ID, request); !errors.Is(err, ErrMissionConflict) {
				t.Fatal("restart stole unexpired integration", err)
			}
			recoveryOwner := service
			if merged {
				record.Integrations[0].StartedAt = time.Now().Add(-4 * time.Minute)
				writeInterruptedMissionFixture(t, root, record)
				recoveryOwner = restarted
			}
			// A stopped local owner or a restarted owner after lease expiry may
			// reconcile the observed effect, without rerunning merge.
			value, err := recoveryOwner.ControlMissionExecution(t.Context(), record.Mission.ID, request)
			if err != nil {
				t.Fatal(err)
			}
			if merged {
				assertIntegratedMission(t, root, ready, value, request, "ready")
			} else if value.Execution.PendingIntegration() != nil || value.Execution.Integrations[0].State != "failed" || value.Execution.Current.Base != ready.Execution.Base {
				t.Fatal("unattempted merge became success", value)
			}
			if calls.Load() != 5 {
				t.Fatal("recovery invoked provider")
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := restarted.ShutdownChangeWorkflows(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMissionIntegrationRetainsCheckoutWhenReceiptPublicationFails(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	record := interruptedMissionIntegration(t, service, root, ready, request)
	testGit(t, root, "merge", "--ff-only", request.Integration.Source.Head)
	git := service.completeGitMutation(root, "merged", nil)
	files, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "mission-executions.json"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	original, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(files[0], []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.publishMissionIntegration(t.Context(), t.Context(), root, store, record, git)
	if err != nil || value.Execution.PendingIntegration() == nil || len(value.ReadyChildren) != 0 || len(value.Git.Warnings) == 0 || value.Git.RemovedWorktree != "" {
		t.Fatal("failed receipt concealed the source mutation or released dependencies", value, err)
	}
	if _, err := os.Stat(request.Integration.Path); err != nil {
		t.Fatal("lost only proposal checkout", err)
	}
	if testGit(t, root, "rev-parse", "HEAD") != request.Integration.Source.Head || calls.Load() != 5 {
		t.Fatal("wrong observed effect")
	}
	if err := os.WriteFile(files[0], original, 0600); err != nil {
		t.Fatal(err)
	}
	query := settingsQuery(t, service)
	if _, err := service.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExecutionRevision, request.ProjectRevision = "reconcile_integration", record.Revision, query.ProjectRevision
	request.Base = &agentstate.MissionBase{Head: request.Integration.Source.Head, Branch: record.Base.Branch}
	request.Integration.IntegrationID = record.PendingIntegration().ID
	recovered, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, ready, recovered, request, "ready")
}

func TestMissionIntegrationCleanupFailureIsRetainedAndExplicitlyRecoverable(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	record := interruptedMissionIntegration(t, service, root, ready, request)
	testGit(t, root, "merge", "--ff-only", request.Integration.Source.Head)
	obstruction := filepath.Join(request.Integration.Path, "keep-untracked.txt")
	if err := os.WriteFile(obstruction, []byte("preserve my work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	query := settingsQuery(t, service)
	if _, err := service.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExecutionRevision, request.ProjectRevision = "reconcile_integration", record.Revision, query.ProjectRevision
	request.Base = &agentstate.MissionBase{Head: request.Integration.Source.Head, Branch: record.Base.Branch}
	request.Integration.IntegrationID = record.PendingIntegration().ID
	value, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil || value.Execution.Integrations[0].State != "merged" || value.Execution.Integrations[0].Cleanup != "failed" || value.Git.RemovedWorktree != "" || len(value.Git.Warnings) == 0 {
		t.Fatal(value, err)
	}
	if _, err := os.Stat(obstruction); err != nil {
		t.Fatal("cleanup removed untracked work", err)
	}
	if err := os.Remove(obstruction); err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExecutionRevision = "cleanup_integration", value.Execution.Revision
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	cleaned, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, ready, cleaned, request, "ready")
	if calls.Load() != 5 {
		t.Fatal("cleanup dispatched providers")
	}
}

func TestMissionIntegrationRejectsExtraCommitContentAndMissingCheckoutIsNotProof(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	session, err := loadChangeSession(request.Integration.Path, request.Integration.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Integration.Path, "unselected.go"), []byte("package main\n// unreviewed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, request.Integration.Path, "add", "unselected.go")
	testGit(t, request.Integration.Path, "commit", "--amend", "--no-edit")
	forged := *session
	commit := *session.Commit
	commit.OID = testGit(t, request.Integration.Path, "rev-parse", "HEAD")
	forged.Commit = &commit
	if err := storeChangeSession(request.Integration.Path, &forged); err != nil {
		t.Fatal(err)
	}
	candidate := request
	input := *request.Integration
	input.Source.Head = commit.OID
	candidate.Integration = &input
	if _, err := service.ControlMissionExecution(t.Context(), ready.Execution.Mission.ID, candidate); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("unreviewed commit content accepted", err)
	}
	// Retain the original immutable proposal and intent, then simulate a checkout
	// removed outside the application without integrating its source commit.
	if err := storeChangeSession(request.Integration.Path, session); err != nil {
		t.Fatal(err)
	}
	record := interruptedMissionIntegration(t, service, root, ready, request)
	testGit(t, root, "worktree", "remove", "--force", "--", request.Integration.Path)
	request.Action, request.ExecutionRevision = "reconcile_integration", record.Revision
	request.Integration.IntegrationID = record.PendingIntegration().ID
	value, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil || value.Execution.Integrations[0].State != "failed" || value.Execution.Current.Base != ready.Execution.Base || len(value.ReadyChildren) != 0 {
		t.Fatal("missing checkout treated as integration", value, err)
	}
	if calls.Load() != 5 {
		t.Fatal("recovery dispatched providers")
	}
}

func TestMissionIntegrationCanceledBeforeGitRetainsBaseAndProposal(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	record := interruptedMissionIntegration(t, service, root, ready, request)
	cancelRequest := cancelExecutionRequest(&AgentMissionExecution{Execution: record}, settingsQuery(t, service))
	if _, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, cancelRequest); err != nil {
		t.Fatal(err)
	}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	source := &GitWorktree{Path: request.Integration.Path, GitIdentity: GitIdentity{Head: request.Integration.Source.Head, Branch: request.Integration.Source.Branch}}
	value, err := service.executeMissionIntegration(t.Context(), root, store, record, source)
	if err != nil || value.Execution.State != "canceled" || value.Execution.Integrations[0].State != "failed" || len(value.ReadyChildren) != 0 || value.Git.RemovedWorktree != "" {
		t.Fatal("canceled intent executed or lost evidence", value, err)
	}
	if testGit(t, root, "rev-parse", "HEAD") != ready.Execution.Base.Head || calls.Load() != 5 {
		t.Fatal("cancellation changed source or provider usage")
	}
}

func TestMissionIntegrationRecoveryProvesCommitAfterExternalCleanup(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	record := interruptedMissionIntegration(t, service, root, ready, request)
	testGit(t, root, "merge", "--ff-only", request.Integration.Source.Head)
	testGit(t, root, "worktree", "remove", "--force", "--", request.Integration.Path)
	testGit(t, root, "branch", "-d", "--", request.Integration.Source.Branch)
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	query := settingsQuery(t, service)
	if _, err := service.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExecutionRevision, request.ProjectRevision = "reconcile_integration", record.Revision, query.ProjectRevision
	request.Base = &agentstate.MissionBase{Head: request.Integration.Source.Head, Branch: record.Base.Branch}
	request.Integration.IntegrationID = record.PendingIntegration().ID
	value, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, ready, value, request, "ready")
	if calls.Load() != 5 {
		t.Fatal("recovery dispatched providers")
	}
}

func TestMissionIntegrationMissingArchiveBlocksPublicationAndCleanup(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	record := interruptedMissionIntegration(t, service, root, ready, request)
	testGit(t, root, "merge", "--ff-only", request.Integration.Source.Head)
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	query := settingsQuery(t, service)
	if _, err := service.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "artifacts.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal(paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExecutionRevision, request.ProjectRevision = "reconcile_integration", record.Revision, query.ProjectRevision
	request.Base = &agentstate.MissionBase{Head: request.Integration.Source.Head, Branch: record.Base.Branch}
	request.Integration.IntegrationID = record.PendingIntegration().ID
	value, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil || value.Execution.State != "blocked" || value.Execution.Integrations[0].State != "unavailable" || value.Execution.Current.Base != ready.Execution.Base || value.OwnerState != "recovery_required" || len(value.ReadyChildren) != 0 || value.Git.RemovedWorktree != "" {
		t.Fatal("missing evidence reported as integrated", value, err)
	}
	if _, err := os.Stat(request.Integration.Path); err != nil {
		t.Fatal("missing archive allowed cleanup", err)
	}
	if err := os.WriteFile(paths[0], data, 0600); err != nil {
		t.Fatal(err)
	}
	request.ExecutionRevision = value.Execution.Revision
	if _, err := service.TrustProjectExecution(t.Context(), query.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, ready, recovered, request, "ready")
	if calls.Load() != 5 {
		t.Fatal("evidence recovery dispatched providers")
	}
}

func TestMissionIntegrationOwnershipSurvivesWorktreeAndBranchRename(t *testing.T) {
	service, root, ready, request, calls := missionIntegrationFixture(t)
	moved := filepath.Join(root, ".worktrees", "renamed-child")
	testGit(t, root, "worktree", "move", "--", request.Integration.Path, moved)
	git := missionGitRequest(request)
	git.Path = moved
	if _, err := service.MergeGitWorktree(t.Context(), git); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("moving checkout bypassed mission ownership", err)
	}
	testGit(t, moved, "branch", "-m", "codex/renamed-child")
	git.SourceBranch = "codex/renamed-child"
	for _, action := range []func(context.Context, GitMutationRequest) (*GitMutationResult, error){service.MergeGitWorktree, service.CleanupMergedWorktree, service.DiscardGitWorktree} {
		if _, err := action(t.Context(), git); !errors.Is(err, ErrMissionConflict) {
			t.Fatal("renaming checkout and branch bypassed retained origin", err)
		}
	}
	if testGit(t, root, "rev-parse", "HEAD") != ready.Execution.Base.Head || calls.Load() != 5 {
		t.Fatal("renamed checkout changed base or dispatch count")
	}
	if _, err := loadChangeSession(moved, request.Integration.SessionID); err != nil {
		t.Fatal("discard removed retained proposal", err)
	}
}

func TestMissionIntegrationCancellationAfterGitDefersCleanupButRecordsCommit(t *testing.T) {
	for _, cancelMission := range []bool{false, true} {
		t.Run(map[bool]string{false: "request canceled", true: "mission canceled"}[cancelMission], func(t *testing.T) {
			testMissionIntegrationCancellationAfterGit(t, cancelMission)
		})
	}
}

func testMissionIntegrationCancellationAfterGit(t *testing.T, cancelMission bool) {
	t.Helper()
	service, root, ready, request, calls := missionIntegrationFixture(t)
	record := interruptedMissionIntegration(t, service, root, ready, request)
	testGit(t, root, "merge", "--ff-only", request.Integration.Source.Head)
	state, readyCount := "ready", 1
	if cancelMission {
		state, readyCount = "canceled", 0
		cancelRequest := cancelExecutionRequest(&AgentMissionExecution{Execution: record}, settingsQuery(t, service))
		if _, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, cancelRequest); err != nil {
			t.Fatal(err)
		}
	}
	store, err := agentstate.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	source := &GitWorktree{Path: request.Integration.Path, GitIdentity: GitIdentity{Head: request.Integration.Source.Head, Branch: request.Integration.Source.Branch}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	value, err := service.executeMissionIntegration(ctx, root, store, record, source)
	if err != nil || value.Execution.State != state || value.Execution.Integrations[0].State != "merged" || value.Execution.Integrations[0].Cleanup != "pending" || len(value.ReadyChildren) != readyCount || value.Git.RemovedWorktree != "" || len(value.Git.Warnings) == 0 {
		t.Fatal("cancellation hid a merge or continued cleanup", value, err)
	}
	if _, err := os.Stat(request.Integration.Path); err != nil {
		t.Fatal("canceled operation deleted checkout", err)
	}
	request.Action, request.ExecutionRevision, request.ProjectRevision = "cleanup_integration", value.Execution.Revision, value.Execution.Current.ProjectRevision
	request.Base = &value.Execution.Current.Base
	request.Integration.IntegrationID = value.Execution.Integrations[0].ID
	if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	cleaned, err := service.ControlMissionExecution(t.Context(), record.Mission.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegratedMission(t, root, ready, cleaned, request, state)
	if calls.Load() != 5 {
		t.Fatal("cancellation or cleanup dispatched providers")
	}
}
