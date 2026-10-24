package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

type MissionChildOrigin struct {
	MissionID      string `json:"mission_id"`
	ExecutionID    string `json:"execution_id"`
	ChildID        string `json:"child_id"`
	PlanHash       string `json:"plan_hash"`
	RevalidationID string `json:"revalidation_id,omitempty"`
}

func copyMissionChildOrigin(origin *MissionChildOrigin) (*MissionChildOrigin, error) {
	if origin == nil {
		return nil, nil
	}
	if err := validateMissionChildOrigin(origin); err != nil {
		return nil, err
	}
	value := *origin
	return &value, nil
}

func validateMissionChildOrigin(origin *MissionChildOrigin) error {
	if origin == nil {
		return nil
	}
	for _, value := range []string{origin.MissionID, origin.ExecutionID, origin.ChildID, origin.PlanHash} {
		if value == "" || len(value) > 128 || strings.ContainsAny(value, "/\\\x00\n\r") {
			return fmt.Errorf("invalid mission proposal origin")
		}
	}
	if _, err := hex.DecodeString(origin.PlanHash); err != nil || len(origin.PlanHash) != 64 {
		return fmt.Errorf("invalid mission proposal plan identity")
	}
	if origin.RevalidationID != "" && (!strings.HasPrefix(origin.RevalidationID, "revalidation-") || len(origin.RevalidationID) > 128 || strings.ContainsAny(origin.RevalidationID, "/\\\x00\n\r")) {
		return fmt.Errorf("invalid mission revalidation origin")
	}
	return nil
}

type missionChildAdmissionKey struct{}
type missionChildAdmission struct {
	parent      *Service
	job         *missionJob
	origin      MissionChildOrigin
	sessionID   string
	models      ChangeWorkflowModels
	waveContext context.Context
	budget      *agentruntime.Budget
}

func admittedMissionChild(ctx context.Context, session *ChangeSession) (*missionChildAdmission, error) {
	if session.MissionOrigin == nil {
		return nil, nil
	}
	admission, _ := ctx.Value(missionChildAdmissionKey{}).(*missionChildAdmission)
	if admission == nil || admission.origin != *session.MissionOrigin || admission.sessionID != session.ID {
		return nil, fmt.Errorf("mission proposals use their reserved workflow allocation; inspect this proposal or create and review a new mission for another model attempt")
	}
	if err := context.Cause(admission.waveContext); err != nil {
		return nil, err
	}
	return admission, admission.parent.authorizeMissionChild(ctx, admission.job, admission.origin.ChildID)
}

func requireMissionProviderAdmission(ctx context.Context, session *ChangeSession) error {
	admission, err := admittedMissionChild(ctx, session)
	if err != nil || admission == nil {
		return err
	}
	if admission.budget == nil || admission.budget != agentruntime.BudgetFromContext(ctx) {
		return fmt.Errorf("mission provider call lost its reserved budget")
	}
	return nil
}

func bindMissionWorkflowContext(background, request context.Context, session *ChangeSession, models ChangeWorkflowModels) (context.Context, error) {
	admission, err := admittedMissionChild(request, session)
	if err != nil || admission == nil {
		return background, err
	}
	if admission.models != models {
		return nil, project.ErrRevisionConflict
	}
	owned := *admission
	owned.budget = agentruntime.BudgetFromContext(background)
	return context.WithValue(background, missionChildAdmissionKey{}, &owned), nil
}

func publishMissionWorkflow(ctx context.Context, session *ChangeSession) error {
	admission, err := admittedMissionChild(ctx, session)
	if err != nil || admission == nil {
		return err
	}
	return admission.job.store.RecordMissionChild(ctx, missionExecutionIdentityFor(admission.job.record), admission.job.record.Wave.ID, admission.origin.ChildID, missionChildProgress(session, "running", ""), admission.job.retainedAuthority)
}

func linkMissionWorkflowCancellation(ctx context.Context, cancel context.CancelFunc) context.CancelFunc {
	admission, _ := ctx.Value(missionChildAdmissionKey{}).(*missionChildAdmission)
	if admission == nil {
		return cancel
	}
	stop := context.AfterFunc(admission.waveContext, cancel)
	return func() { stop(); cancel() }
}

func missionChildProgress(session *ChangeSession, state, reason string) agentstate.MissionChildProgress {
	progress := agentstate.MissionChildProgress{State: state, Reason: reason}
	if session != nil && session.Workflow != nil {
		progress.ProjectID, progress.ProjectRevision = session.ProjectID, session.ProjectRevision
		progress.SessionID, progress.WorkflowID = session.ID, session.Workflow.ID
		progress.ProposalHash, progress.ProposalRevision = session.Hash, session.Revision
	}
	return progress
}

func (s *Service) runMissionChild(ctx context.Context, job *missionJob, id string) (agentruntime.MissionNodeResult, error) {
	result := agentruntime.MissionNodeResult{ExecutionID: job.record.ID, WaveID: job.record.Wave.ID, ChildID: id}
	if err := s.authorizeMissionChild(ctx, job, id); err != nil {
		return result, err
	}
	child, specification, err := missionChildAt(job.record, id)
	if err != nil {
		return result, err
	}
	service, err := s.prepareMissionChild(ctx, job, child, specification)
	if err != nil {
		return result, s.finishMissionChildFailure(job, id, nil, "Worktree preparation failed; inspect the owned checkout before cleanup.", err)
	}
	session, err := s.startMissionChild(ctx, job, service, specification)
	if err != nil {
		return result, s.finishMissionChildFailure(job, id, session, "Child workflow admission failed; its allocation remains reserved.", err)
	}
	service.changeWorkflow.mu.Lock()
	worker := service.changeWorkflow.getJob(session.ID)
	service.changeWorkflow.mu.Unlock()
	if worker == nil || worker.id != session.Workflow.ID {
		return result, fmt.Errorf("mission child lost its owned workflow")
	}
	// The workflow's I/O and child processes already honor cancellation. Drain
	// the owned worker before the ADK node and mission wave can finish.
	<-worker.done
	service.changesMu.Lock()
	completed, err := loadChangeSession(service.manager.Root(), session.ID)
	service.changesMu.Unlock()
	if err != nil {
		return result, err
	}
	if completed.MissionOrigin == nil || *completed.MissionOrigin != *session.MissionOrigin || completed.Workflow == nil || completed.Workflow.ID != worker.id {
		return result, project.ErrRevisionConflict
	}
	state, reason := missionChildOutcome(completed)
	if err := s.authorizeMissionChild(ctx, job, id); err != nil {
		state, reason = "interrupted", "Mission admission changed before publication; inspect the retained child proposal."
	}
	cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := job.store.RecordMissionChild(cleanup, missionExecutionIdentityFor(job.record), job.record.Wave.ID, id, missionChildProgress(completed, state, reason), job.retainedAuthority); err != nil {
		return result, err
	}
	result.State, result.SessionID, result.WorkflowID = state, completed.ID, completed.Workflow.ID
	result.ProposalHash, result.ProposalRevision = completed.Hash, completed.Revision
	return result, nil
}

func (s *Service) finishMissionChildFailure(job *missionJob, id string, session *ChangeSession, reason string, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := job.store.RecordMissionChild(ctx, missionExecutionIdentityFor(job.record), job.record.Wave.ID, id, missionChildProgress(session, "failed", reason), job.retainedAuthority)
	return errors.Join(cause, err)
}

func missionChildOutcome(session *ChangeSession) (string, string) {
	workflow := session.Workflow
	if workflow == nil {
		return "interrupted", "No completed workflow receipt is available; the allocation remains reserved."
	}
	if canHumanReviewProposal(session) && requiredChecksPassed(session.Checks) {
		return "awaiting_review", "Review the checked proposal; Apply and merge require separate human decisions."
	}
	if workflow.Status == "changes_requested" || workflow.Review != nil && workflow.Review.Verdict == "changes_requested" {
		return "rejected", "The critic requested changes. Inspect retained findings and prepare a newly reviewed mission for another model attempt."
	}
	if workflow.Status == "canceled" || workflow.Status == "interrupted" {
		return "interrupted", "The workflow stopped; its allocation remains reserved and its proposal evidence is retained."
	}
	return "failed", "The workflow failed. Inspect the retained proposal for check and provider errors."
}

func (s *Service) recoverMissionChildren(ctx context.Context, root string, record agentstate.MissionExecution) map[string]agentstate.MissionChildProgress {
	progress := map[string]agentstate.MissionChildProgress{}
	for _, child := range record.Children {
		if child.WaveID != record.Wave.ID || child.State != "reserved" && child.State != "running" {
			continue
		}
		session, err := s.readMissionChildEvidence(ctx, root, record, child)
		if err != nil {
			progress[child.ID] = agentstate.MissionChildProgress{State: "interrupted", Reason: "Owned checkout or proposal evidence is incomplete, changed or unavailable; inspect it before cleanup. Its allocation remains reserved."}
			continue
		}
		state, reason := missionChildOutcome(session)
		progress[child.ID] = missionChildProgress(session, state, reason)
	}
	return progress
}

func (s *Service) readMissionChildEvidence(ctx context.Context, root string, record agentstate.MissionExecution, child agentstate.MissionExecutionChild) (*ChangeSession, error) {
	path := filepath.Join(root, ".worktrees", child.Directory)
	canonical, _, err := s.validateRegisteredWorktreePath(ctx, path)
	if err != nil {
		return nil, err
	}
	if canonical != path || child.SessionID == "" {
		return nil, ErrMissionConflict
	}
	repository, err := readGitRepository(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingGitIdentity(repository, GitIdentity{Head: child.BaseHead, Branch: child.Branch}); err != nil {
		return nil, err
	}
	worker, _, err := s.WorktreeService(ctx, path)
	if err != nil {
		return nil, err
	}
	session, err := worker.ChangeSession(ctx, child.SessionID)
	if err != nil {
		return nil, err
	}
	origin := MissionChildOrigin{MissionID: record.Mission.ID, ExecutionID: record.ID, ChildID: child.ID, PlanHash: record.Mission.PlanHash}
	if session.MissionOrigin == nil || *session.MissionOrigin != origin || session.Workflow == nil || session.Workflow.ID != child.WorkflowID || session.Freshness != "current" {
		return nil, ErrMissionConflict
	}
	_, spec, err := missionChildAt(record, child.ID)
	if err != nil {
		return nil, err
	}
	if !missionProposalScope(session, spec) {
		return nil, ErrMissionConflict
	}
	return session, nil
}

func missionProposalScope(session *ChangeSession, spec agentstate.MissionChild) bool {
	if len(session.Targets) != len(spec.WritePaths) || session.Plan == nil || len(session.Plan.ReadTargets) != len(spec.ReadPaths) {
		return false
	}
	for _, target := range session.Targets {
		if !slices.Contains(spec.WritePaths, target.Path) {
			return false
		}
	}
	for _, target := range session.Plan.ReadTargets {
		if !slices.Contains(spec.ReadPaths, target.Path) {
			return false
		}
	}
	return true
}

func missionChildAt(record agentstate.MissionExecution, id string) (agentstate.MissionExecutionChild, agentstate.MissionChild, error) {
	index := slices.IndexFunc(record.Children, func(child agentstate.MissionExecutionChild) bool { return child.ID == id })
	if index < 0 {
		return agentstate.MissionExecutionChild{}, agentstate.MissionChild{}, ErrMissionConflict
	}
	return record.Children[index], record.Mission.Plan.Children[index], nil
}

func (s *Service) prepareMissionChild(ctx context.Context, job *missionJob, child agentstate.MissionExecutionChild, spec agentstate.MissionChild) (*Service, error) {
	path := filepath.Join(job.root, ".worktrees", child.Directory)
	request := GitMutationRequest{ProjectID: job.record.Mission.ProjectID, ProjectRevision: job.record.Current.ProjectRevision, GitIdentity: GitIdentity{Head: child.BaseHead, Branch: job.record.Current.Base.Branch}, Confirm: true, ConfirmExecution: true, TargetBranch: child.Branch, Path: path}
	created, err := s.CreateGitWorktree(ctx, request)
	if err != nil {
		return nil, err
	}
	if created.State != "created" || created.Error != "" {
		return nil, fmt.Errorf("create mission worktree: %s", created.Error)
	}
	if err := copyMissionContextPolicy(ctx, job.root, path, func() error { return s.authorizeMissionChild(ctx, job, child.ID) }); err != nil {
		return nil, err
	}
	service, _, err := s.WorktreeService(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := validateMissionChildSources(service.manager.Root(), job.record.BoundMission(), spec); err != nil {
		return nil, err
	}
	if err := s.grantMissionChildExecution(ctx, job, service); err != nil {
		return nil, err
	}
	return service, nil
}

func copyMissionContextPolicy(ctx context.Context, parent, child string, authorize func() error) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	source, err := storage.MetadataPath(parent, ".mini-orca", "context-policy.json")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 64<<10 {
		return fmt.Errorf("mission context policy exceeds 64 KiB")
	}
	target, err := storage.MetadataPath(child, ".mini-orca", "context-policy.json")
	if err != nil {
		return err
	}
	return storage.WriteFileAuthorized(target, data, 0600, func() error {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		return authorize()
	})
}

func validateMissionChildSources(root string, mission agentstate.MissionRecord, spec agentstate.MissionChild) error {
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return err
	}
	if policy.Version() != mission.Policy {
		return fmt.Errorf("%w: child context policy differs from the approved checkout", project.ErrRevisionConflict)
	}
	for _, path := range append(slices.Clone(spec.WritePaths), spec.ReadPaths...) {
		index := slices.IndexFunc(mission.Plan.Sources, func(source agentstate.MissionSource) bool { return source.Path == path })
		if index < 0 {
			return ErrMissionConflict
		}
		expected := mission.Plan.Sources[index]
		target, err := captureChangeTarget(root, path)
		if err != nil {
			return err
		}
		instructions, err := project.ResolveInstructions(root, path)
		if err != nil {
			return err
		}
		if target.Exists != expected.Exists || target.Hash != expected.Hash || instructions.Fingerprint != mission.Instructions[path] {
			return fmt.Errorf("%w: child source or instructions differ from the approved plan", project.ErrRevisionConflict)
		}
	}
	return nil
}

func (s *Service) grantMissionChildExecution(ctx context.Context, job *missionJob, child *Service) error {
	if err := s.requireWorkspaceExecutionTrust(ctx, job.record.Current.ProjectRevision, job.workspaceHash); err != nil {
		return err
	}
	index, err := child.manager.Index()
	if err != nil {
		return err
	}
	_, trust, err := child.executionTrustScope(index.ProjectRevision, "")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(trust.Commands, job.commands) {
		return fmt.Errorf("child check commands differ from the reviewed execution scope")
	}
	trust.WorkspaceHash, err = child.executionTrustWorkspace(ctx, child.manager.Root(), trust)
	if err != nil {
		return err
	}
	// The mission's explicit check grant is local to this owned child service;
	// it is not remembered as general consent for a future daemon session.
	child.executionTrust.mu.Lock()
	defer child.executionTrust.mu.Unlock()
	child.executionTrust.project, child.executionTrust.revision = trust.ProjectID, trust.ProjectRevision
	child.executionTrust.workspaceHash, child.executionTrust.restorePending = trust.WorkspaceHash, false
	return nil
}

func (s *Service) startMissionChild(ctx context.Context, job *missionJob, child *Service, spec agentstate.MissionChild) (*ChangeSession, error) {
	if err := s.authorizeMissionChild(ctx, job, spec.ID); err != nil {
		return nil, err
	}
	index, err := child.manager.Index()
	if err != nil {
		return nil, err
	}
	origin := MissionChildOrigin{MissionID: job.record.Mission.ID, ExecutionID: job.record.ID, ChildID: spec.ID, PlanHash: job.record.Mission.PlanHash}
	plan := &ChangePlanInput{Problem: job.record.Mission.Plan.Goal, Approach: spec.Approach, ReadPaths: slices.Clone(spec.ReadPaths), NonGoals: []string{"Change only the approved mission child write paths. Integration and source Apply require separate human review."}}
	request := ChangeCreateRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "feature", Title: spec.Title, Paths: slices.Clone(spec.WritePaths), AcceptanceCriteria: slices.Clone(spec.AcceptanceCriteria), Plan: plan, missionOrigin: &origin}
	session, err := child.OpenChangeSession(ctx, request)
	if err != nil {
		return nil, err
	}
	settings := job.settings
	settings.root, settings.runtime.Limits = child.manager.Root(), config.AgentLimits(spec.Limits)
	admission := &missionChildAdmission{parent: s, job: job, origin: origin, sessionID: session.ID, models: job.models, waveContext: ctx}
	requestCtx := context.WithValue(ctx, agentSettingsKey{}, settings)
	requestCtx = context.WithValue(requestCtx, missionChildAdmissionKey{}, admission)
	started, err := child.StartChangeWorkflow(requestCtx, session.ID, ChangeWorkflowRequest{ChangeIdentity: identityForChange(session), Message: spec.Approach, PlanHash: session.Plan.Hash, Models: job.models, ConfirmedProfiles: job.confirmedProfiles, AutoRepair: spec.Limits.Repairs > 0})
	if err != nil {
		return session, err
	}
	return started, nil
}
