package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/app/gitops"
)

type MissionRevalidationRequest struct {
	Previous       MissionIntegrationRequest `json:"previous"`
	RevalidationID string                    `json:"revalidation_id,omitempty"`
}

func missionRevalidationShape(request MissionExecutionRequest) error {
	if request.ExecutionID == "" || request.ExecutionRevision < 1 || request.CatalogRevision != 0 || request.Base == nil || request.Revalidation == nil || request.Integration != nil || !request.ConfirmExecution || len(request.ConfirmedProfiles) != 0 {
		return fmt.Errorf("revalidation requires the exact execution, previous proposal, current base and check execution confirmation")
	}
	input := request.Revalidation
	if err := input.Previous.validateProposal(); err != nil {
		return err
	}
	if input.Previous.IntegrationID != "" || (request.Action == "revalidate") != (input.RevalidationID == "") {
		return fmt.Errorf("recovery requires the retained revalidation identity; a new attempt cannot reuse one")
	}
	return nil
}

func matchesMissionProposal(record agentstate.MissionExecution, child agentstate.MissionExecutionChild, session *ChangeSession) bool {
	origin := MissionChildOrigin{MissionID: record.Mission.ID, ExecutionID: record.ID, ChildID: child.ID, PlanHash: record.Mission.PlanHash}
	if child.Generation > 0 {
		for _, entry := range record.Revalidations {
			if entry.Candidate.ID == child.ID && entry.Candidate.Generation == child.Generation {
				origin.RevalidationID = entry.ID
			}
		}
		if origin.RevalidationID == "" {
			return false
		}
	}
	expected := ChangeIdentity{ProjectID: child.ProjectID, ProjectRevision: child.ProjectRevision, Hash: child.ProposalHash, Revision: child.ProposalRevision}
	if session.MissionOrigin == nil || *session.MissionOrigin != origin || identityForChange(session) != expected || session.ID != child.SessionID {
		return false
	}
	if child.Generation > 0 {
		return child.WorkflowID == "" && session.Workflow == nil
	}
	return session.Workflow != nil && session.Workflow.ID == child.WorkflowID
}

func matchesMissionRevalidationInput(root string, child agentstate.MissionExecutionChild, input MissionIntegrationRequest) bool {
	return input.ChildID == child.ID && input.SessionID == child.SessionID && input.WorkflowID == child.WorkflowID && input.ProposalHash == child.ProposalHash && input.ProposalRevision == child.ProposalRevision && input.Path == filepath.Join(root, ".worktrees", child.Directory) && input.Source.Branch == child.Branch
}

func (s *Service) controlMissionRevalidation(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	if request.Action == "reconcile_revalidation" {
		return s.reconcileMissionRevalidation(ctx, root, store, id, request)
	}
	job, previous, err := s.reserveMissionRevalidation(ctx, root, store, id, request)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	entry := job.record.PendingRevalidation()
	defer s.finishMissionEffect(job.record.ID, job.effect)
	defer job.effect.cancel()
	candidate, err := s.revalidateMissionCandidate(job.ctx, job, previous)
	state, reason := "awaiting_review", ""
	if err != nil {
		state, reason = "failed", "Revalidation failed; retained worktrees and proposal evidence require inspection."
	} else if !requiredChecksPassed(candidate.Checks) {
		state, reason = "failed", "Fresh checks did not pass; the candidate is retained without review authority."
	}
	if context.Cause(job.ctx) != nil {
		state, reason = "interrupted", "Revalidation was interrupted; retained candidates are not automatically executed or applied."
	}
	return s.finishMissionRevalidation(root, job, entry.ID, state, reason)
}

type missionRevalidationJob struct {
	*missionJob
	ctx    context.Context
	effect *missionEffect
}

func (s *Service) reserveMissionRevalidation(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (*missionRevalidationJob, *ChangeSession, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	record, err := store.MissionExecution(ctx, request.ProjectID, id)
	if err != nil {
		return nil, nil, err
	}
	if record.ID != request.ExecutionID || record.Revision != request.ExecutionRevision || record.Mission.PlanHash != request.PlanHash || *request.Base != record.Current.Base {
		return nil, nil, ErrMissionConflict
	}
	if err := s.validatePreparedMissionExecution(ctx, root, record); err != nil {
		return nil, nil, err
	}
	trust, err := s.ExecutionTrust(ctx, record.Current.ProjectRevision, "")
	if err != nil {
		return nil, nil, err
	}
	if !trust.Trusted {
		return nil, nil, fmt.Errorf("trust the displayed current project check commands before revalidating")
	}
	previous, err := s.readMissionRevalidationSource(ctx, root, record, request.Revalidation.Previous)
	if err != nil {
		return nil, nil, err
	}
	evidence, err := archiveMissionProposal(ctx, store, record, previous)
	if err != nil {
		return nil, nil, err
	}
	input := request.Revalidation.Previous
	record, err = store.BeginMissionRevalidation(ctx, missionExecutionIdentityFor(record), s.missions.owner, input.ChildID, input.Source, evidence, func() error { return s.validatePreparedMissionExecution(ctx, root, record) })
	if err != nil {
		return nil, nil, err
	}
	entry := record.PendingRevalidation()
	bounded, cancel := context.WithDeadline(ctx, entry.Deadline())
	effect := &missionEffect{id: entry.ID, cancel: cancel, done: make(chan struct{}), cancelOnReindex: true}
	s.missions.mu.Lock()
	s.missions.effects[record.ID] = effect
	s.missions.mu.Unlock()
	job := &missionJob{root: root, record: record, store: store, commands: trust.Commands, workspaceHash: trust.WorkspaceHash}
	return &missionRevalidationJob{missionJob: job, ctx: bounded, effect: effect}, previous, nil
}

func (s *Service) readMissionRevalidationSource(ctx context.Context, root string, record agentstate.MissionExecution, input MissionIntegrationRequest) (*ChangeSession, error) {
	child, spec, err := missionChildAt(record, input.ChildID)
	if err != nil {
		return nil, err
	}
	if child.State != "awaiting_review" || child.BaseHead == record.Current.Base.Head || !matchesMissionRevalidationInput(root, child, input) {
		return nil, ErrMissionConflict
	}
	canonical, _, err := s.validateRegisteredWorktreePath(ctx, input.Path)
	if err != nil {
		return nil, err
	}
	if canonical != input.Path {
		return nil, ErrMissionConflict
	}
	repository, err := readGitRepository(ctx, canonical)
	if err != nil {
		return nil, err
	}
	if err := gitops.RequireMergeTarget(repository); err != nil {
		return nil, err
	}
	if err := requireMatchingGitIdentity(repository, GitIdentity{Head: input.Source.Head, Branch: input.Source.Branch}); err != nil {
		return nil, err
	}
	session, err := loadChangeSession(canonical, child.SessionID)
	if err != nil {
		return nil, err
	}
	return s.validateMissionRevalidationSource(ctx, root, record, child, spec, session, input)
}

func (s *Service) validateMissionRevalidationSource(ctx context.Context, root string, record agentstate.MissionExecution, child agentstate.MissionExecutionChild, spec agentstate.MissionChild, session *ChangeSession, input MissionIntegrationRequest) (*ChangeSession, error) {
	if !matchesMissionProposal(record, child, session) || !missionProposalScope(session, spec) || len(session.Changes) == 0 {
		return nil, ErrMissionConflict
	}
	if session.State == "applied" {
		return session, validateMissionCommit(ctx, root, record, child, spec, session, input.Source)
	}
	if session.State != "draft" || input.Source.Head != child.BaseHead || !canHumanReviewProposal(session) {
		return nil, ErrMissionConflict
	}
	worker, _, err := s.WorktreeService(ctx, input.Path)
	if err != nil {
		return nil, err
	}
	return session, worker.verifyChangeCurrent(ctx, input.Path, session)
}

func (s *Service) authorizeMissionRevalidation(ctx context.Context, job *missionRevalidationJob) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	current, err := job.store.MissionExecution(ctx, job.record.Mission.ProjectID, job.record.Mission.ID)
	if err != nil {
		return err
	}
	entry := current.PendingRevalidation()
	if current.ID != job.record.ID || current.State != "revalidating" || entry == nil || entry.ID != job.effect.id {
		return ErrMissionConflict
	}
	if err := s.validatePreparedMissionExecution(ctx, job.root, job.record); err != nil {
		return err
	}
	return s.requireWorkspaceExecutionTrust(ctx, job.record.Current.ProjectRevision, job.workspaceHash)
}

func (s *Service) revalidateMissionCandidate(ctx context.Context, job *missionRevalidationJob, previous *ChangeSession) (*ChangeSession, error) {
	if err := s.authorizeMissionRevalidation(ctx, job); err != nil {
		return nil, err
	}
	entry := job.record.PendingRevalidation()
	child := entry.Candidate
	path := filepath.Join(job.root, ".worktrees", child.Directory)
	base := job.record.Current.Base
	created, err := s.CreateGitWorktree(ctx, GitMutationRequest{ProjectID: job.record.Mission.ProjectID, ProjectRevision: job.record.Current.ProjectRevision, GitIdentity: GitIdentity{Head: base.Head, Branch: base.Branch}, Path: path, TargetBranch: child.Branch, Confirm: true, ConfirmExecution: true})
	if err != nil {
		return nil, err
	}
	if created.State != "created" || created.Error != "" {
		return nil, fmt.Errorf("create revalidation worktree: %s", created.Error)
	}
	worker, err := s.prepareMissionRevalidationWorker(ctx, job, path)
	if err != nil {
		return nil, err
	}
	_, spec, err := missionChildAt(job.record, child.ID)
	if err != nil {
		return nil, err
	}
	session, err := openMissionRevalidationCandidate(ctx, worker, job.record, spec, previous)
	if err != nil {
		return nil, err
	}
	identity := missionExecutionIdentityFor(job.record)
	identity.Revision = 0
	progress := agentstate.MissionChildProgress{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, SessionID: session.ID, ProposalHash: session.Hash, ProposalRevision: session.Revision}
	if err := job.store.RecordMissionRevalidationCandidate(ctx, identity, entry.ID, progress, func() error { return s.authorizeMissionRevalidation(ctx, job) }); err != nil {
		return nil, err
	}
	if err := s.authorizeMissionRevalidation(ctx, job); err != nil {
		return nil, err
	}
	return worker.CheckChange(ctx, session.ID, ChangeCheckRequest{ChangeIdentity: identityForChange(session), CheckOptions: CheckOptions{RunTests: true, RunLint: true}})
}

func (s *Service) prepareMissionRevalidationWorker(ctx context.Context, job *missionRevalidationJob, path string) (*Service, error) {
	if err := copyMissionContextPolicy(ctx, job.root, path, func() error { return s.authorizeMissionRevalidation(ctx, job) }); err != nil {
		return nil, err
	}
	worker, _, err := s.WorktreeService(ctx, path)
	if err != nil {
		return nil, err
	}
	_, spec, err := missionChildAt(job.record, job.record.PendingRevalidation().Previous.ID)
	if err != nil {
		return nil, err
	}
	if err := validateMissionChildSources(path, job.record.BoundMission(), spec); err != nil {
		return nil, err
	}
	if err := s.grantMissionChildExecution(ctx, job.missionJob, worker); err != nil {
		return nil, err
	}
	return worker, s.authorizeMissionRevalidation(ctx, job)
}

func openMissionRevalidationCandidate(ctx context.Context, worker *Service, record agentstate.MissionExecution, spec agentstate.MissionChild, previous *ChangeSession) (*ChangeSession, error) {
	index, err := worker.manager.Index()
	if err != nil {
		return nil, err
	}
	origin := MissionChildOrigin{MissionID: record.Mission.ID, ExecutionID: record.ID, ChildID: spec.ID, PlanHash: record.Mission.PlanHash, RevalidationID: record.PendingRevalidation().ID}
	plan := &ChangePlanInput{Problem: record.Mission.Plan.Goal, Approach: spec.Approach, ReadPaths: slices.Clone(spec.ReadPaths), NonGoals: []string{"Historical model review does not verify this candidate. Review every file and explicitly Apply/commit before integration."}}
	session, err := worker.OpenChangeSession(ctx, ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "feature", Title: spec.Title, Paths: slices.Clone(spec.WritePaths), AcceptanceCriteria: slices.Clone(spec.AcceptanceCriteria), Plan: plan, missionOrigin: &origin})
	if err != nil {
		return nil, err
	}
	edits := make([]ChangeEditInput, 0, len(previous.Changes))
	for _, edit := range previous.Changes {
		edits = append(edits, ChangeEditInput{Path: edit.Path, Content: edit.Content, Summary: edit.Summary, Criteria: slices.Clone(edit.Criteria)})
	}
	return worker.SendChangeMessage(ctx, session.ID, ChangeMessageRequest{ChangeIdentity: identityForChange(session), Changes: edits})
}

// Final human Review approves this newly captured plan only after current checks
// and per-file inspection. Revalidation itself carries no approval forward.
func (s *Service) approveMissionRevalidationReviewPlan(session *ChangeSession) error {
	if session.MissionOrigin == nil || session.MissionOrigin.RevalidationID == "" || session.Workflow != nil || session.Plan == nil {
		return nil
	}
	if err := validateStoredPlan(session); err != nil {
		return err
	}
	if !allChangeFilesReviewed(session) || !s.hasLivePassingChecks(session) {
		return fmt.Errorf("review every changed file and run current checks before approving the revalidated plan")
	}
	return approveWorkflowPlan(session, session.Plan.Hash)
}

func (s *Service) finishMissionRevalidation(root string, job *missionRevalidationJob, id, state, reason string) (*AgentMissionExecution, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	identity := missionExecutionIdentityFor(job.record)
	identity.Revision = 0
	if state == "awaiting_review" {
		if err := s.authorizeMissionRevalidation(job.ctx, job); err != nil {
			state, reason = "interrupted", "Current scope or authority changed before candidate publication."
		}
	}
	authorize := func() error {
		if state == "awaiting_review" {
			return s.verifyMissionRevalidationResult(job.ctx, job)
		}
		return s.missionRetainedRoot(root)
	}
	record, err := job.store.FinishMissionRevalidation(ctx, identity, id, state, reason, false, authorize)
	if err != nil {
		return nil, fmt.Errorf("%w: revalidation evidence is retained; inspect and reconcile its pending receipt: %w", ErrMissionUnavailable, err)
	}
	return s.missionIntegrationResult(ctx, root, record, nil), nil
}

func (s *Service) verifyMissionRevalidationResult(ctx context.Context, job *missionRevalidationJob) error {
	if err := s.authorizeMissionRevalidation(ctx, job); err != nil {
		return err
	}
	record, err := job.store.MissionExecution(ctx, job.record.Mission.ProjectID, job.record.Mission.ID)
	if err != nil {
		return err
	}
	entry := record.PendingRevalidation()
	if entry == nil || entry.ID != job.effect.id {
		return ErrMissionConflict
	}
	archive, err := job.store.ExportArtifact(ctx, entry.Evidence.Key, entry.Evidence.Version)
	if err != nil {
		return fmt.Errorf("verify retained previous proposal: %w", err)
	}
	if !archive.Protected || archive.Hash != entry.Evidence.Hash {
		return ErrMissionConflict
	}
	path := filepath.Join(job.root, ".worktrees", entry.Candidate.Directory)
	worker, _, err := s.WorktreeService(ctx, path)
	if err != nil {
		return err
	}
	session, err := loadChangeSession(path, entry.Candidate.SessionID)
	if err != nil {
		return err
	}
	if !matchesMissionProposal(record, entry.Candidate, session) || !checkedMissionRevalidationSession(session) {
		return ErrMissionConflict
	}
	return worker.verifyChangeCurrent(ctx, path, session)
}

func checkedMissionRevalidationSession(session *ChangeSession) bool {
	return session.State == "draft" && len(session.Checks) > 0 && requiredChecksPassed(session.Checks) && session.CheckOptions.RunTests && session.CheckOptions.RunLint
}

func (s *Service) reconcileMissionRevalidation(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	record, err := store.MissionExecution(ctx, request.ProjectID, id)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	entry := record.PendingRevalidation()
	if record.ID != request.ExecutionID || record.Revision != request.ExecutionRevision || record.Mission.PlanHash != request.PlanHash || *request.Base != record.Current.Base || entry == nil {
		return nil, ErrMissionConflict
	}
	input := request.Revalidation
	if input.RevalidationID != entry.ID || input.Previous.Source != entry.Source || !matchesMissionRevalidationInput(root, entry.Previous, input.Previous) || s.missionOwnerState(record) != "recovery_required" {
		return nil, ErrMissionConflict
	}
	record, err = store.FinishMissionRevalidation(ctx, missionExecutionIdentityFor(record), entry.ID, "interrupted", "Owner stopped; historical evidence and any candidate checkout are retained. No checks were replayed.", true, func() error { return s.missionRetainedRoot(root) })
	if err != nil {
		return nil, missionExecutionError(err)
	}
	return s.missionIntegrationResult(ctx, root, record, nil), nil
}
