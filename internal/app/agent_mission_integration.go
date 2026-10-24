package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/app/gitops"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type MissionIntegrationRequest struct {
	ChildID          string                 `json:"child_id"`
	SessionID        string                 `json:"session_id"`
	WorkflowID       string                 `json:"workflow_id"`
	ProposalHash     string                 `json:"proposal_hash"`
	ProposalRevision int64                  `json:"proposal_revision"`
	Path             string                 `json:"path"`
	Source           agentstate.MissionBase `json:"source"`
	IntegrationID    string                 `json:"integration_id,omitempty"`
}

type missionEffect struct {
	id     string
	cancel context.CancelFunc
	done   chan struct{}
}

func missionIntegrationShape(request MissionExecutionRequest) error {
	if request.ExecutionID == "" || request.ExecutionRevision < 1 || request.CatalogRevision != 0 || request.Base == nil || request.Integration == nil || !request.ConfirmExecution || len(request.ConfirmedProfiles) != 0 {
		return fmt.Errorf("mission integration requires the exact execution, destination, committed proposal and explicit Git execution confirmation")
	}
	input := request.Integration
	if err := input.validateProposal(); err != nil {
		return err
	}
	if (request.Action == "integrate") != (input.IntegrationID == "") {
		return fmt.Errorf("recovery requires the retained integration identity; a new integration cannot reuse one")
	}
	return nil
}

func (input MissionIntegrationRequest) validateProposal() error {
	for _, value := range []string{input.ChildID, input.SessionID, input.WorkflowID, input.Path, input.Source.Head, input.Source.Branch} {
		if value == "" {
			return fmt.Errorf("select the exact owned committed child proposal")
		}
	}
	if (agentstate.MemorySource{Path: "proposal", Hash: input.ProposalHash}).Validate() != nil || input.ProposalRevision < 0 {
		return fmt.Errorf("select the exact owned committed child proposal")
	}
	return nil
}

func missionGitRequest(request MissionExecutionRequest) GitMutationRequest {
	return GitMutationRequest{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, GitIdentity: GitIdentity{Head: request.Base.Head, Branch: request.Base.Branch}, Path: request.Integration.Path, SourceHead: request.Integration.Source.Head, SourceBranch: request.Integration.Source.Branch, Confirm: true, ConfirmExecution: true}
}

func (s *Service) controlMissionIntegration(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	record, err := store.MissionExecution(ctx, request.ProjectID, id)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	if record.ID != request.ExecutionID || record.Revision != request.ExecutionRevision || record.Mission.PlanHash != request.PlanHash {
		return nil, ErrMissionConflict
	}
	repository, err := s.gitMutationPreflight(ctx, missionGitRequest(request), request.Action == "reconcile_integration")
	if err != nil {
		return nil, err
	}
	if request.Action != "integrate" {
		return s.recoverMissionIntegration(ctx, root, store, record, repository, request)
	}
	if err := s.validatePreparedMissionExecution(ctx, root, record); err != nil {
		return nil, err
	}
	source, session, err := s.admitMissionIntegration(ctx, root, record, repository, request)
	if err != nil {
		return nil, err
	}
	evidence, err := archiveMissionProposal(ctx, store, record, session)
	if err != nil {
		return nil, err
	}
	input := request.Integration
	record, err = store.BeginMissionIntegration(ctx, missionExecutionIdentityFor(record), s.missions.owner, agentstate.MissionIntegrationInput{ChildID: input.ChildID, Source: input.Source, SessionID: input.SessionID, WorkflowID: input.WorkflowID, ProposalHash: input.ProposalHash, ProposalRevision: input.ProposalRevision, Evidence: evidence}, func() error { return s.validatePreparedMissionExecution(ctx, root, record) })
	if err != nil {
		return nil, missionExecutionError(err)
	}
	return s.executeMissionIntegration(ctx, root, store, record, source)
}

func (s *Service) admitMissionIntegration(ctx context.Context, root string, record agentstate.MissionExecution, repository *GitRepository, request MissionExecutionRequest) (*GitWorktree, *ChangeSession, error) {
	if err := gitops.RequireMergeTarget(repository); err != nil {
		return nil, nil, err
	}
	child, spec, err := missionChildAt(record, request.Integration.ChildID)
	if err != nil {
		return nil, nil, err
	}
	input := request.Integration
	if child.State != "awaiting_review" || child.BaseHead != record.Current.Base.Head || input.Path != filepath.Join(root, ".worktrees", child.Directory) || input.Source.Branch != child.Branch || input.SessionID != child.SessionID || input.WorkflowID != child.WorkflowID || input.ProposalHash != child.ProposalHash || input.ProposalRevision != child.ProposalRevision {
		return nil, nil, fmt.Errorf("%w: child identity or current base changed; older candidates require revalidation", ErrMissionConflict)
	}
	source, err := gitops.MergeSource(ctx, repository, missionGitRequest(request))
	if err != nil {
		return nil, nil, err
	}
	if err := requireFinishedWorktreeTasks(source.Path); err != nil {
		return nil, nil, err
	}
	session, err := loadChangeSession(source.Path, child.SessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateMissionCommit(ctx, root, record, child, spec, session, input.Source); err != nil {
		return nil, nil, err
	}
	return source, session, nil
}

func validateMissionCommit(ctx context.Context, root string, record agentstate.MissionExecution, child agentstate.MissionExecutionChild, spec agentstate.MissionChild, session *ChangeSession, source agentstate.MissionBase) error {
	origin := MissionChildOrigin{MissionID: record.Mission.ID, ExecutionID: record.ID, ChildID: child.ID, PlanHash: record.Mission.PlanHash}
	identity := ChangeIdentity{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Hash: session.Hash, Revision: session.Revision}
	expected := ChangeIdentity{ProjectID: child.ProjectID, ProjectRevision: child.ProjectRevision, Hash: child.ProposalHash, Revision: child.ProposalRevision}
	if session.MissionOrigin == nil || *session.MissionOrigin != origin || identity != expected || session.ID != child.SessionID || session.Workflow == nil || session.Workflow.ID != child.WorkflowID {
		return ErrMissionConflict
	}
	if err := validateMissionCommitReview(session, child, source); err != nil {
		return err
	}
	for _, edit := range session.Changes {
		if !slices.Contains(spec.WritePaths, edit.Path) {
			return fmt.Errorf("%w: committed proposal exceeds approved child scope", ErrMissionConflict)
		}
	}
	return verifyMissionCommitTree(ctx, root, session, child.BaseHead, source.Head)
}

func validateMissionCommitReview(session *ChangeSession, child agentstate.MissionExecutionChild, source agentstate.MissionBase) error {
	if session.State != "applied" || session.Commit == nil || session.Commit.State != "committed" || session.Commit.OID != source.Head || session.Commit.Head != child.BaseHead || session.Commit.Branch != child.Branch || !allChangeFilesReviewed(session) || session.ReviewedHash != session.Hash || !requiredChecksPassed(session.Checks) {
		return fmt.Errorf("review, check and explicitly Apply/commit the exact child before integration")
	}
	return nil
}

func verifyMissionCommitTree(ctx context.Context, root string, session *ChangeSession, base, head string) error {
	parents, err := gitRead(ctx, root, "rev-list", "--parents", "-n", "1", head)
	if err != nil {
		return err
	}
	if strings.TrimSpace(parents) != head+" "+base {
		return fmt.Errorf("%w: child commit is not the single reviewed change on the recorded base", ErrMissionConflict)
	}
	changed, err := gitRead(ctx, root, "diff-tree", "--no-commit-id", "--name-only", "--no-renames", "-r", "-z", base, head, "--")
	if err != nil {
		return err
	}
	for _, path := range strings.Split(strings.TrimSuffix(changed, "\x00"), "\x00") {
		if path != "" && !slices.ContainsFunc(session.Changes, func(edit ChangeEdit) bool { return edit.Path == path }) {
			return fmt.Errorf("%w: commit contains an unreviewed path", ErrMissionConflict)
		}
	}
	for _, edit := range session.Changes {
		if err := verifyMissionBlob(ctx, root, head, session, edit); err != nil {
			return err
		}
	}
	return nil
}

func verifyMissionBlob(ctx context.Context, root, head string, session *ChangeSession, edit ChangeEdit) error {
	tree, err := gitRead(ctx, root, "ls-tree", "-z", head, "--", edit.Path)
	if err != nil {
		return err
	}
	metadata, path, found := strings.Cut(strings.TrimSuffix(tree, "\x00"), "\t")
	fields := strings.Fields(metadata)
	target := changeTarget(session, edit.Path)
	if !found || path != edit.Path || len(fields) != 3 || fields[1] != "blob" || target == nil || fields[0] != gitFileMode(target.Mode) {
		return fmt.Errorf("%w: commit changed the reviewed file identity or mode", ErrMissionConflict)
	}
	content, err := gitRead(ctx, root, "cat-file", "blob", fields[2])
	if err != nil {
		return err
	}
	if content != edit.Content {
		return fmt.Errorf("%w: commit content differs from the reviewed proposal", ErrMissionConflict)
	}
	return nil
}

func archiveMissionProposal(ctx context.Context, store *agentstate.Store, record agentstate.MissionExecution, session *ChangeSession) (agentstate.MissionProposalEvidence, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return agentstate.MissionProposalEvidence{}, err
	}
	key := agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca-missions", User: "local", ID: record.ID}, Name: session.MissionOrigin.ChildID + "-proposal.json"}
	artifact, err := store.SaveArtifact(ctx, agentstate.ArtifactWrite{Key: key, Kind: "inline", MIME: "application/json", Data: data, Protected: true, Deduplicate: true, Provenance: map[string]string{"project": record.Mission.ProjectID, "mission": record.Mission.ID, "plan_hash": record.Mission.PlanHash, "child": session.MissionOrigin.ChildID, "child_project": session.ProjectID, "proposal": session.Hash}})
	if err != nil {
		return agentstate.MissionProposalEvidence{}, fmt.Errorf("retain complete mission proposal before integration: %w", err)
	}
	return agentstate.MissionProposalEvidence{Key: artifact.Key, Version: artifact.Version, Hash: artifact.Hash}, nil
}

func (s *Service) executeMissionIntegration(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, source *GitWorktree) (*AgentMissionExecution, error) {
	entry := record.PendingIntegration()
	s.invalidateJobsForProjectChange()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	effect := &missionEffect{id: entry.ID, cancel: cancel, done: make(chan struct{})}
	s.missions.mu.Lock()
	s.missions.effects[record.ID] = effect
	s.missions.mu.Unlock()
	defer s.finishMissionEffect(record.ID, effect)
	cause := authorizeMissionIntegrationEffect(ctx, store, record, entry.ID)
	if cause == nil {
		_, cause = gitRead(ctx, root, "merge", "--ff-only", "--no-edit", "--no-stat", "--no-autostash", "--no-rerere-autoupdate", source.Head)
	}
	git := s.completeGitMutation(root, "merged", cause)
	// A canceled command may already have moved HEAD. Observe and persist its
	// actual result independently of the request, before allowing any cleanup.
	observe, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	return s.publishMissionIntegration(observe, ctx, root, store, record, git)
}

func authorizeMissionIntegrationEffect(ctx context.Context, store *agentstate.Store, record agentstate.MissionExecution, id string) error {
	current, err := store.MissionExecution(ctx, record.Mission.ProjectID, record.Mission.ID)
	if err != nil {
		return err
	}
	if current.ID != record.ID || current.State != "integrating" || current.PendingIntegration() == nil || current.PendingIntegration().ID != id {
		return ErrMissionConflict
	}
	return context.Cause(ctx)
}

func (s *Service) finishMissionEffect(id string, effect *missionEffect) {
	effect.cancel()
	s.missions.mu.Lock()
	delete(s.missions.effects, id)
	close(effect.done)
	s.missions.mu.Unlock()
	s.notifyEvent("missions")
}

func (s *Service) publishMissionIntegration(ctx, cleanupContext context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, git *GitMutationResult) (*AgentMissionExecution, error) {
	entry := record.PendingIntegration()
	outcome := s.observeMissionIntegration(ctx, root, store, record, *entry, git)
	git.State = outcome.State
	if git.Error == "" && outcome.Failure != "" {
		git.Error = outcome.Failure
	}
	identity := missionExecutionIdentityFor(record)
	identity.Revision = 0 // The owned effect must publish even after explicit cancel.
	updated, err := store.FinishMissionIntegration(ctx, identity, entry.ID, outcome, func() error {
		if err := s.missionRetainedRoot(root); err != nil {
			return err
		}
		if outcome.After != nil {
			observed := record
			observed.Current = *outcome.After
			return s.validatePreparedMissionExecution(ctx, root, observed)
		}
		return nil
	})
	if err != nil {
		git.Warnings = append(git.Warnings, "Git was attempted, but its mission receipt could not be saved. Retain the worktree and reconcile the integration: "+err.Error())
	} else {
		record = updated
		if outcome.State == "merged" && record.State != "canceled" && context.Cause(cleanupContext) == nil {
			record = s.cleanupMissionIntegration(cleanupContext, root, store, record, entry.ID, git)
		} else if outcome.State == "merged" {
			git.Warnings = append(git.Warnings, "Merge and mission receipt are saved; checkout cleanup was deferred because the operation stopped. Confirm cleanup separately.")
		}
	}
	return s.missionIntegrationResult(ctx, root, record, git), nil
}

func (s *Service) missionRetainedRoot(root string) error {
	canonical, err := project.CanonicalRoot(root)
	if err != nil {
		return err
	}
	if canonical != root || s.manager.Root() != root {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) observeMissionIntegration(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, entry agentstate.MissionIntegration, git *GitMutationResult) agentstate.MissionIntegrationOutcome {
	failure := agentstate.MissionIntegrationOutcome{State: "unavailable", Failure: "Integration evidence is incomplete; retain the worktree and reconcile the exact intent."}
	repository := git.Repository
	if repository == nil || repository.Branch != entry.Before.Base.Branch || repository.Dirty || repository.Operation != "" {
		if repository != nil && (repository.MergeHead != "" || len(repository.Conflicts) != 0) {
			failure.State, failure.Failure = "conflicts", "Resolve or explicitly abort the Git merge before reconciling mission integration."
		}
		return failure
	}
	if repository.Head == entry.Before.Base.Head {
		return agentstate.MissionIntegrationOutcome{State: "failed", Failure: "Git remains on the recorded base; the committed child and its evidence were retained."}
	}
	if repository.Head != entry.Source.Head || git.Index == nil {
		return failure
	}
	if _, err := retainedMissionProposal(ctx, root, store, record, entry); err != nil {
		return failure
	}
	checkpoint, err := s.captureMissionCheckpoint(ctx, root, record, repository)
	if err != nil {
		return failure
	}
	return agentstate.MissionIntegrationOutcome{State: "merged", After: checkpoint}
}

func retainedMissionProposal(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, entry agentstate.MissionIntegration) (*ChangeSession, error) {
	artifact, err := store.ExportArtifact(ctx, entry.Evidence.Key, entry.Evidence.Version)
	if err != nil {
		return nil, err
	}
	if artifact.Hash != entry.Evidence.Hash || !artifact.Protected {
		return nil, ErrMissionConflict
	}
	var session ChangeSession
	if err := json.Unmarshal(artifact.Data, &session); err != nil {
		return nil, err
	}
	if err := validateStoredChange(session); err != nil {
		return nil, err
	}
	child, spec, err := missionChildAt(record, entry.ChildID)
	if err != nil {
		return nil, err
	}
	if err := validateMissionCommit(ctx, root, record, child, spec, &session, entry.Source); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *Service) captureMissionCheckpoint(ctx context.Context, root string, record agentstate.MissionExecution, repository *GitRepository) (*agentstate.MissionCheckpoint, error) {
	checkpoint := &agentstate.MissionCheckpoint{ProjectRevision: repository.ProjectRevision, Base: agentstate.MissionBase{Head: repository.Head, Branch: repository.Branch}, Sources: []agentstate.MissionSource{}, Instructions: map[string]string{}}
	for _, source := range record.Current.Sources {
		target, err := captureChangeTarget(root, source.Path)
		if err != nil {
			return nil, err
		}
		instructions, err := project.ResolveInstructions(root, source.Path)
		if err != nil {
			return nil, err
		}
		checkpoint.Sources = append(checkpoint.Sources, agentstate.MissionSource{Path: source.Path, Exists: target.Exists, Hash: target.Hash})
		checkpoint.Instructions[source.Path] = instructions.Fingerprint
	}
	bound := record
	bound.Current = *checkpoint
	if err := s.validateMissionBinding(ctx, root, bound.BoundMission()); err != nil {
		return nil, err
	}
	return checkpoint, nil
}

func (s *Service) missionIntegrationResult(ctx context.Context, root string, record agentstate.MissionExecution, git *GitMutationResult) *AgentMissionExecution {
	query := AgentRunQuery{ProjectID: record.Mission.ProjectID, ProjectRevision: record.Current.ProjectRevision}
	if git.Index != nil {
		query.ProjectRevision = git.Index.ProjectRevision
	}
	value, err := s.inspectMissionExecution(ctx, root, query, record)
	if err != nil {
		value = &AgentMissionExecution{Execution: record, Stale: true, SourceStatus: "unavailable", ReadyChildren: []string{}, OwnerState: "recovery_required"}
		git.Warnings = append(git.Warnings, "Refresh the mission receipt: "+err.Error())
	}
	if record.PendingIntegration() != nil {
		value.OwnerState = "recovery_required"
	}
	value.Git = git
	return value
}

func (s *Service) guardMissionWorktree(ctx context.Context, root string, source *GitWorktree) error {
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	store, err := agentstate.Open(root)
	if err != nil {
		return err
	}
	directory := ""
	if filepath.Dir(source.Path) == filepath.Join(root, ".worktrees") {
		directory = filepath.Base(source.Path)
	}
	_, err = store.MissionWorktree(ctx, index.ProjectID, directory, source.Branch)
	if errors.Is(err, agentstate.ErrNotFound) {
		return guardRetainedMissionOrigin(source.Path)
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: this checkout belongs to a mission; use its exact integration or recovery control to retain proposal evidence", ErrMissionConflict)
}

func guardRetainedMissionOrigin(root string) error {
	history, err := listChangeSessions(root)
	if err != nil {
		return err
	}
	for _, entry := range history {
		session, err := loadChangeSession(root, entry.ID)
		if err != nil {
			return err
		}
		if session.MissionOrigin != nil {
			return fmt.Errorf("%w: this proposal belongs to a mission; restore its recorded worktree identity and use mission integration or recovery", ErrMissionConflict)
		}
	}
	return nil
}

func (s *Service) recoverMissionIntegration(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, repository *GitRepository, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	position := slices.IndexFunc(record.Integrations, func(entry agentstate.MissionIntegration) bool { return entry.ID == request.Integration.IntegrationID })
	if position < 0 {
		return nil, ErrMissionConflict
	}
	entry := record.Integrations[position]
	child, _, err := missionChildAt(record, entry.ChildID)
	if err != nil {
		return nil, err
	}
	expected := MissionIntegrationRequest{ChildID: entry.ChildID, SessionID: entry.SessionID, WorkflowID: entry.WorkflowID, ProposalHash: entry.ProposalHash, ProposalRevision: entry.ProposalRevision, Path: filepath.Join(root, ".worktrees", child.Directory), Source: entry.Source, IntegrationID: entry.ID}
	if *request.Integration != expected {
		return nil, ErrMissionConflict
	}
	if request.Action == "cleanup_integration" {
		if entry.State != "merged" || entry.Cleanup == "removed" || record.Current.Base != *request.Base {
			return nil, ErrMissionConflict
		}
		if err := s.validatePreparedMissionExecution(ctx, root, record); err != nil {
			return nil, err
		}
		git := &GitMutationResult{State: "merged", Repository: repository, Warnings: []string{}}
		record = s.cleanupMissionIntegration(ctx, root, store, record, entry.ID, git)
		return s.missionIntegrationResult(ctx, root, record, git), nil
	}
	if record.PendingIntegration() == nil || record.PendingIntegration().ID != entry.ID || s.missionOwnerState(record) != "recovery_required" {
		return nil, fmt.Errorf("%w: the integration must stop or its bounded lease must expire before reconciliation", ErrMissionConflict)
	}
	// Reconciliation never reruns merge. Reindexing observes the actual checkout;
	// retained commit and proposal evidence decides whether it is the result.
	git := s.completeGitMutation(root, "observed", nil)
	return s.publishMissionIntegration(ctx, ctx, root, store, record, git)
}

func (s *Service) cleanupMissionIntegration(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, id string, git *GitMutationResult) agentstate.MissionExecution {
	index := slices.IndexFunc(record.Integrations, func(entry agentstate.MissionIntegration) bool { return entry.ID == id })
	entry := record.Integrations[index]
	state, failure := s.removeIntegratedMissionWorktree(ctx, root, store, record, entry, git)
	updated, err := store.RecordMissionCleanup(ctx, missionExecutionIdentityFor(record), entry.ID, state, failure, func() error { return s.missionRetainedRoot(root) })
	if err != nil {
		git.Warnings = append(git.Warnings, "Integration is recorded, but cleanup status could not be saved; reconcile cleanup before retrying: "+err.Error())
		return record
	}
	return updated
}

func (s *Service) removeIntegratedMissionWorktree(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, entry agentstate.MissionIntegration, git *GitMutationResult) (string, string) {
	if _, err := retainedMissionProposal(ctx, root, store, record, entry); err != nil {
		git.Warnings = append(git.Warnings, "Integration is recorded, but retained proposal evidence is unavailable; checkout and branch were retained.")
		return "failed", "Retained proposal evidence must be available before cleanup."
	}
	if _, err := gitRead(ctx, root, "merge-base", "--is-ancestor", entry.Source.Head, record.Current.Base.Head); err != nil {
		git.Warnings = append(git.Warnings, "Integration ancestry could not be verified; checkout and branch were retained.")
		return "failed", "Integration ancestry is unavailable."
	}
	child, _, err := missionChildAt(record, entry.ChildID)
	if err != nil {
		return "failed", "Child ownership is unavailable."
	}
	source := &GitWorktree{Path: filepath.Join(root, ".worktrees", child.Directory), GitIdentity: GitIdentity{Head: entry.Source.Head, Branch: entry.Source.Branch}}
	if _, err := os.Lstat(source.Path); err == nil {
		return s.cleanupExistingMissionWorktree(ctx, root, source, git)
	} else if !errors.Is(err, os.ErrNotExist) {
		git.Warnings = append(git.Warnings, "Integration is recorded, but checkout status is unavailable: "+err.Error())
		return "failed", "Checkout status is unavailable."
	}
	registered, err := gitops.ReadWorktrees(ctx, root)
	if err != nil || slices.ContainsFunc(registered, func(item GitWorktree) bool { return item.Path == source.Path }) {
		git.Warnings = append(git.Warnings, "The missing checkout remains registered or its registration is unavailable; inspect Git worktrees before cleanup.")
		return "failed", "Missing checkout registration requires manual inspection."
	}
	git.RemovedWorktree = source.Path
	return finishMissionBranchCleanup(ctx, root, source, git)
}

func (s *Service) cleanupExistingMissionWorktree(ctx context.Context, root string, source *GitWorktree, git *GitMutationResult) (string, string) {
	completed, err := gitops.MergedFindingTargets(ctx, git.Repository, source, listChangeSessions)
	if err != nil {
		git.Warnings = append(git.Warnings, "Integration is recorded, but findings could not be read; checkout retained: "+err.Error())
		return "failed", "Finding evidence is unavailable."
	}
	s.completeMergedWorktree(ctx, root, source, completed, git)
	if git.RemovedBranch == source.Branch {
		return "removed", ""
	}
	if git.RemovedWorktree == source.Path {
		return "worktree_removed", "Integration and checkout removal completed; the branch was retained."
	}
	return "failed", "Integration is recorded; checkout cleanup failed."
}

func finishMissionBranchCleanup(ctx context.Context, root string, source *GitWorktree, git *GitMutationResult) (string, string) {
	_, err := gitRead(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+source.Branch)
	if gitops.ExitCode(err, 1) {
		git.RemovedBranch = source.Branch
		return "removed", ""
	}
	if err == nil {
		err = gitops.DeleteMergedBranch(ctx, root, source.GitIdentity)
	}
	if err != nil {
		git.Warnings = append(git.Warnings, "Integration and checkout removal are verified, but the branch was retained: "+err.Error())
		return "worktree_removed", "Integration and checkout removal completed; the branch was retained."
	}
	git.RemovedBranch = source.Branch
	return "removed", ""
}
