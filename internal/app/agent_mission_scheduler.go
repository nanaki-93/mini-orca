package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type missionController struct {
	mu    sync.Mutex
	owner string
	jobs  map[string]*missionJob
}

type missionJob struct {
	root              string
	record            agentstate.MissionExecution
	store             *agentstate.Store
	settings          admittedAgentSettings
	models            ChangeWorkflowModels
	confirmedProfiles []string
	commands          [][]string
	workspaceHash     string
	cancel            context.CancelFunc
	done              chan struct{}
	finished          bool // missions.mu owns finished and failure.
	failure           error
}

func newMissionController() *missionController {
	return &missionController{owner: "owner-" + rand.Text(), jobs: map[string]*missionJob{}}
}

func missionExecutionIdentityFor(record agentstate.MissionExecution) agentstate.MissionExecutionIdentity {
	return agentstate.MissionExecutionIdentity{ProjectID: record.Mission.ProjectID, MissionID: record.Mission.ID, ExecutionID: record.ID, Revision: record.Revision, PlanHash: record.Mission.PlanHash}
}

func (job *missionJob) retainedAuthority() error {
	root, err := project.CanonicalRoot(job.root)
	if err != nil {
		return err
	}
	if root != job.root {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) startMissionWave(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	record, err := store.MissionExecution(ctx, request.ProjectID, id)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	if record.ID != request.ExecutionID || record.Revision != request.ExecutionRevision || record.Mission.PlanHash != request.PlanHash {
		return nil, ErrMissionConflict
	}
	job, err := s.admitMissionWave(ctx, root, store, record, request)
	if err != nil {
		return nil, err
	}
	authorize := func() error { return s.validateMissionWaveAdmission(ctx, job) }
	record, err = store.BeginMissionWave(ctx, missionExecutionIdentityFor(record), s.missions.owner, authorize)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	job.record = record
	background, cancel := context.WithDeadline(context.Background(), record.Wave.Deadline())
	job.cancel, job.done = cancel, make(chan struct{})
	s.missions.mu.Lock()
	s.missions.jobs[record.ID] = job
	s.missions.mu.Unlock()
	if context.Cause(ctx) != nil {
		cancel()
	}
	go s.runMissionWave(background, job)
	return s.inspectMissionExecution(ctx, root, AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}, record)
}

func (s *Service) admitMissionWave(ctx context.Context, root string, store *agentstate.Store, record agentstate.MissionExecution, request MissionExecutionRequest) (*missionJob, error) {
	if err := s.validatePreparedMissionExecution(ctx, root, record); err != nil {
		return nil, err
	}
	frozen, err := s.freezeAgentSettings(ctx, root)
	if err != nil {
		return nil, err
	}
	settings := frozen.Value(agentSettingsKey{}).(admittedAgentSettings)
	if err := record.Mission.Plan.Limits.Within(settings.runtime.Limits); err != nil {
		return nil, err
	}
	models := s.resolveWorkflowModels(frozen, ChangeWorkflowModels{})
	if err := s.validateMissionModels(ctx, models, request.ConfirmedProfiles); err != nil {
		return nil, err
	}
	for _, child := range record.Mission.Plan.Children {
		if !slices.ContainsFunc(child.WritePaths, isTestFilePath) {
			return nil, fmt.Errorf("mission child %s requires an explicit test write path for its checked workflow", child.ID)
		}
	}
	trust, err := s.ExecutionTrust(ctx, record.Mission.ProjectRevision, "")
	if err != nil {
		return nil, err
	}
	if !trust.Trusted {
		return nil, fmt.Errorf("trust the displayed project check commands before starting mission children")
	}
	job := &missionJob{root: root, record: record, store: store, settings: settings, models: models, confirmedProfiles: slices.Clone(request.ConfirmedProfiles), commands: trust.Commands, workspaceHash: trust.WorkspaceHash}
	return job, s.validateMissionWaveAdmission(ctx, job)
}

func (s *Service) validateMissionModels(ctx context.Context, models ChangeWorkflowModels, confirmed []string) error {
	if err := models.validate(); err != nil {
		return err
	}
	if err := s.validateModelSelections(ctx, models.selections()); err != nil {
		return err
	}
	if len(confirmed) > 3 {
		return fmt.Errorf("confirm only the selected mission model profiles")
	}
	seen := map[string]bool{}
	for _, profile := range confirmed {
		if seen[profile] || !slices.Contains(models.selections(), profile) {
			return fmt.Errorf("mission confirmation includes an unselected or repeated profile")
		}
		seen[profile] = true
	}
	for stage, profile := range models.selections() {
		if err := requireModelRuntimeConfirmation(s.workflowStageRuntime(stage, profile), seen[profile]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateMissionWaveAdmission(ctx context.Context, job *missionJob) error {
	if err := s.validatePreparedMissionExecution(ctx, job.root, job.record); err != nil {
		return err
	}
	if err := s.requireWorkspaceExecutionTrust(ctx, job.record.Mission.ProjectRevision, job.workspaceHash); err != nil {
		return err
	}
	return (workspaceIdentity{fingerprint: job.workspaceHash}).verifyCurrent(ctx, job.root)
}

func (s *Service) authorizeMissionChild(ctx context.Context, job *missionJob, id string) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := s.validateMissionWaveAdmission(ctx, job); err != nil {
		return err
	}
	current, err := job.store.MissionExecution(ctx, job.record.Mission.ProjectID, job.record.Mission.ID)
	if err != nil {
		return err
	}
	if current.State != "running" || current.ID != job.record.ID || current.Wave == nil || current.Wave.ID != job.record.Wave.ID {
		return ErrMissionConflict
	}
	child, _, err := missionChildAt(current, id)
	if err != nil {
		return err
	}
	if child.WaveID != current.Wave.ID || child.State != "reserved" && child.State != "running" {
		return ErrMissionConflict
	}
	return nil
}

func (s *Service) runMissionWave(ctx context.Context, job *missionJob) {
	var finishErr error
	defer func() {
		job.cancel()
		s.missions.mu.Lock()
		if finishErr == nil {
			delete(s.missions.jobs, job.record.ID)
		} else {
			job.finished, job.failure = true, finishErr
		}
		s.missions.mu.Unlock()
		close(job.done)
		s.notifyEvent("missions")
	}()
	settings := job.settings
	settings.runtime.Limits = config.AgentLimits(job.record.Mission.Plan.Limits)
	ctx = context.WithValue(ctx, agentSettingsKey{}, settings)
	bounded, release, err := s.workflowAgentContext(ctx, job.models)
	if err == nil {
		input := agentruntime.MissionGraphInput{ProjectID: job.record.Mission.ProjectID, ProjectRevision: job.record.Mission.ProjectRevision, ExecutionID: job.record.ID, WaveID: job.record.Wave.ID, PlanHash: job.record.Mission.PlanHash, Children: job.record.Wave.Children, Concurrency: len(job.record.Wave.Children)}
		_, err = agentruntime.RunMissionGraph(bounded, input, func(ctx context.Context, id string) (agentruntime.MissionNodeResult, error) {
			return s.runMissionChild(ctx, job, id)
		})
		release()
	}
	// Worker failure is preserved by the native run/child proposal. Any child
	// lacking a terminal receipt becomes interrupted, never silently pending.
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	identity := missionExecutionIdentityFor(job.record)
	identity.Revision = 0
	_, finishErr = job.store.FinishMissionWave(cleanup, identity, job.record.Wave.ID, false, missionWaveFailure(err), job.retainedAuthority)
}

func missionWaveFailure(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "Mission execution stopped; fixed allocations and completed evidence are retained."
	case errors.Is(err, context.DeadlineExceeded):
		return "Mission execution reached its time limit; inspect retained child proposals."
	case errors.Is(err, project.ErrRevisionConflict), errors.Is(err, ErrMissionConflict):
		return "Mission admission changed during execution; refresh the captured scope and inspect retained evidence."
	default:
		return "Mission execution could not complete; inspect its child proposals and agent run history."
	}
}

func (s *Service) missionOwnerState(record agentstate.MissionExecution) string {
	if record.Wave == nil {
		return "idle"
	}
	s.missions.mu.Lock()
	job := s.missions.jobs[record.ID]
	active := job != nil && !job.finished && job.record.Wave.ID == record.Wave.ID
	s.missions.mu.Unlock()
	if active {
		return "active"
	}
	if record.Wave.Owner == s.missions.owner || !time.Now().Before(record.Wave.Deadline()) {
		return "recovery_required"
	}
	return "waiting_for_owner"
}

func (s *Service) cancelMissionJobs() {
	s.missions.mu.Lock()
	defer s.missions.mu.Unlock()
	for _, job := range s.missions.jobs {
		if !job.finished {
			job.cancel()
		}
	}
}

func (s *Service) cancelMissionExecution(id string) {
	s.missions.mu.Lock()
	defer s.missions.mu.Unlock()
	if job := s.missions.jobs[id]; job != nil && !job.finished {
		job.cancel()
	}
}

func (s *Service) waitMissionJobs(ctx context.Context) error {
	s.missions.mu.Lock()
	jobs := make([]*missionJob, 0, len(s.missions.jobs))
	for _, job := range s.missions.jobs {
		jobs = append(jobs, job)
	}
	s.missions.mu.Unlock()
	var failure error
	for _, job := range jobs {
		select {
		case <-job.done:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
		s.missions.mu.Lock()
		failure = errors.Join(failure, job.failure)
		s.missions.mu.Unlock()
	}
	return failure
}

func (s *Service) reconcileMissionWave(ctx context.Context, root string, store *agentstate.Store, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	record, err := store.MissionExecution(ctx, request.ProjectID, id)
	if err != nil {
		return nil, missionExecutionError(err)
	}
	if record.ID != request.ExecutionID || record.Revision != request.ExecutionRevision || record.Mission.PlanHash != request.PlanHash || record.Wave == nil {
		return nil, ErrMissionConflict
	}
	if s.missionOwnerState(record) != "recovery_required" {
		return nil, fmt.Errorf("%w: the owned wave must stop or its bounded lease must expire before reconciliation", ErrMissionConflict)
	}
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	unknown := record.Wave.Owner != s.missions.owner
	progress := s.recoverMissionChildren(ctx, root, record)
	record, err = store.RecoverMissionWave(ctx, missionExecutionIdentityFor(record), record.Wave.ID, unknown, progress, func() error { return s.agentHistoryScope(query, root) })
	if err != nil {
		return nil, missionExecutionError(err)
	}
	s.missions.mu.Lock()
	delete(s.missions.jobs, record.ID)
	s.missions.mu.Unlock()
	return s.inspectMissionExecution(ctx, root, query, record)
}

func missionControlShape(request MissionExecutionRequest) error {
	if request.ExecutionID == "" || request.ExecutionRevision < 1 || request.CatalogRevision != 0 || request.Base != nil {
		return fmt.Errorf("mission control requires the exact execution identity and revision without a new catalog or Git base")
	}
	if request.Action == "start" {
		if !request.ConfirmExecution {
			return fmt.Errorf("confirm worktree creation and the displayed project checks for mission execution")
		}
	} else if request.ConfirmExecution || len(request.ConfirmedProfiles) != 0 {
		return fmt.Errorf("stop and recovery controls cannot carry fresh provider or execution grants")
	}
	return nil
}
