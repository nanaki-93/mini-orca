package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const AnalysisRunSchemaVersion = "1"
const AnalysisRunScopeProject = "project"

// AnalysisRunStatus describes progress, never the quality of the analyzed code.
type AnalysisRunStatus string

const (
	AnalysisRunQueued         AnalysisRunStatus = "queued"
	AnalysisRunRunning        AnalysisRunStatus = "running"
	AnalysisRunPausing        AnalysisRunStatus = "pausing"
	AnalysisRunPaused         AnalysisRunStatus = "paused"
	AnalysisRunCanceling      AnalysisRunStatus = "canceling"
	AnalysisRunCanceled       AnalysisRunStatus = "canceled"
	AnalysisRunInterrupted    AnalysisRunStatus = "interrupted"
	AnalysisRunCompleted      AnalysisRunStatus = "completed"
	AnalysisRunCompletedEmpty AnalysisRunStatus = "completed_empty"
	AnalysisRunPartial        AnalysisRunStatus = "partial"
	AnalysisRunFailed         AnalysisRunStatus = "failed"
	AnalysisRunUnavailable    AnalysisRunStatus = "unavailable"
	AnalysisRunStale          AnalysisRunStatus = "stale"
)

func (status AnalysisRunStatus) Valid() bool {
	switch status {
	case AnalysisRunQueued, AnalysisRunRunning, AnalysisRunPausing, AnalysisRunPaused,
		AnalysisRunCanceling, AnalysisRunCanceled, AnalysisRunInterrupted, AnalysisRunCompleted,
		AnalysisRunCompletedEmpty, AnalysisRunPartial, AnalysisRunFailed, AnalysisRunUnavailable, AnalysisRunStale:
		return true
	default:
		return false
	}
}

type AnalysisStage string

const (
	AnalysisStageSemantic      AnalysisStage = "semantic"
	AnalysisStagePerformance   AnalysisStage = "performance"
	AnalysisStageSecurityRules AnalysisStage = "security_rules"
	AnalysisStageSecurityAI    AnalysisStage = "security_ai"
)

// Categories identifies consumers of a producer, not a classifier for its findings.
// Semantic output must carry its own validated category for each risk.
func (stage AnalysisStage) Categories() []project.FindingCategory {
	switch stage {
	case AnalysisStageSemantic:
		return []project.FindingCategory{project.FindingCategoryBugs, project.FindingCategoryPerformance, project.FindingCategorySecurity}
	case AnalysisStagePerformance:
		return []project.FindingCategory{project.FindingCategoryPerformance}
	case AnalysisStageSecurityRules, AnalysisStageSecurityAI:
		return []project.FindingCategory{project.FindingCategorySecurity}
	default:
		return nil
	}
}

type AnalysisStageStatus string

const (
	AnalysisStagePending        AnalysisStageStatus = "pending"
	AnalysisStageRunning        AnalysisStageStatus = "running"
	AnalysisStageCompleted      AnalysisStageStatus = "completed"
	AnalysisStageCompletedEmpty AnalysisStageStatus = "completed_empty"
	AnalysisStagePartial        AnalysisStageStatus = "partial"
	AnalysisStageFailed         AnalysisStageStatus = "failed"
	AnalysisStageSkipped        AnalysisStageStatus = "skipped"
	AnalysisStageUnavailable    AnalysisStageStatus = "unavailable"
	AnalysisStageCanceled       AnalysisStageStatus = "canceled"
	AnalysisStageInterrupted    AnalysisStageStatus = "interrupted"
	AnalysisStageStale          AnalysisStageStatus = "stale"
)

// AnalysisRunLimits bounds one dispatch window, not the captured project inventory.
// MaxAttemptsPerStage includes the initial provider request and all transport retries.
type AnalysisRunLimits struct {
	BatchFiles          int `json:"batch_files"`
	BudgetSeconds       int `json:"budget_seconds"`
	MaxAttemptsPerStage int `json:"max_attempts_per_stage"`
}

func (limits AnalysisRunLimits) Validate() error {
	if limits.BatchFiles < 1 || limits.BatchFiles > 500 || limits.BudgetSeconds < 1 || limits.BudgetSeconds > 3600 || limits.MaxAttemptsPerStage < 1 || limits.MaxAttemptsPerStage > 4 {
		return fmt.Errorf("analysis limits require 1-500 batch files, 1-3600 seconds and 1-4 total attempts per stage")
	}
	return nil
}

// AnalysisQueueIdentity binds the preview to source, policy and effective providers.
// QueueID also covers ordered stages, exclusions, limits and refresh. Cache availability
// is not identity: a run's own successful writes must not invalidate its remaining queue.
type AnalysisQueueIdentity struct {
	ProjectID           string `json:"project_id"`
	ProjectRevision     string `json:"project_revision"`
	PolicyFingerprint   string `json:"policy_fingerprint"`
	ProviderFingerprint string `json:"provider_fingerprint"`
	QueueID             string `json:"queue_id"`
}

type AnalysisRunIdentity struct {
	AnalysisQueueIdentity
	ID         string `json:"id"`
	Generation string `json:"generation"`
}

func (identity AnalysisQueueIdentity) Validate() error {
	if identity.ProjectID == "" || identity.ProjectRevision == "" || identity.PolicyFingerprint == "" || identity.ProviderFingerprint == "" || identity.QueueID == "" {
		return fmt.Errorf("analysis queue identity is incomplete")
	}
	return nil
}

func (identity AnalysisRunIdentity) Validate() error {
	if err := identity.AnalysisQueueIdentity.Validate(); err != nil {
		return err
	}
	if identity.ID == "" || identity.Generation == "" {
		return fmt.Errorf("analysis run identity is incomplete")
	}
	return nil
}

type AnalysisPreviewRequest struct {
	ProjectID           string               `json:"project_id"`
	ProjectRevision     string               `json:"project_revision"`
	Scope               string               `json:"scope"`
	Refresh             bool                 `json:"refresh"`
	Limits              AnalysisRunLimits    `json:"limits"`
	ResumeRun           *AnalysisRunIdentity `json:"resume_run,omitempty"`
	compatibilityStage  AnalysisStage
	compatibilityBudget time.Duration
}

func (request AnalysisPreviewRequest) Validate() error {
	if request.ProjectID == "" || request.ProjectRevision == "" || request.Scope != AnalysisRunScopeProject {
		return fmt.Errorf("analysis requires a project identity and whole-project scope")
	}
	if request.ResumeRun != nil {
		if err := request.ResumeRun.Validate(); err != nil {
			return err
		}
		if request.ResumeRun.ProjectID != request.ProjectID || request.ResumeRun.ProjectRevision != request.ProjectRevision {
			return fmt.Errorf("resume preview does not match the requested project")
		}
	}
	return request.Limits.Validate()
}

type AnalysisFileIdentity struct {
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`
	Language    string `json:"language"`
}

// AnalysisProviderRequirement contains displayable model configuration, never tokens.
// ID is a fingerprint of the effective configuration, including prompt identity.
type AnalysisProviderRequirement struct {
	ID                         string          `json:"id"`
	Stages                     []AnalysisStage `json:"stages"`
	Model                      EffectiveModel  `json:"model"`
	RemoteConfirmationRequired bool            `json:"remote_confirmation_required"`
}

type AnalysisStagePlan struct {
	Stage            AnalysisStage `json:"stage"`
	Eligible         bool          `json:"eligible"`
	Cached           bool          `json:"cached"`
	Reason           string        `json:"reason,omitempty"`
	ProviderID       string        `json:"provider_id,omitempty"`
	MaxModelRequests int           `json:"max_model_requests"`
}

type AnalysisPlannedFile struct {
	AnalysisFileIdentity
	SizeBytes int64               `json:"size_bytes"`
	Stages    []AnalysisStagePlan `json:"stages"`
}

type AnalysisExcludedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type AnalysisRunPreview struct {
	CompatibilityStage           AnalysisStage                 `json:"compatibility_stage,omitempty"`
	CompatibilityBudget          time.Duration                 `json:"compatibility_budget_nanoseconds,omitempty"`
	SchemaVersion                string                        `json:"schema_version"`
	PreviewID                    string                        `json:"preview_id"`
	Identity                     AnalysisQueueIdentity         `json:"identity"`
	Scope                        string                        `json:"scope"`
	Refresh                      bool                          `json:"refresh"`
	Limits                       AnalysisRunLimits             `json:"limits"`
	Files                        []AnalysisPlannedFile         `json:"files"`
	Excluded                     []AnalysisExcludedFile        `json:"excluded"`
	Providers                    []AnalysisProviderRequirement `json:"providers"`
	ExpectedModelRequests        int                           `json:"expected_model_requests"`
	MaxModelRequests             int                           `json:"max_model_requests"`
	SecurityReviewIntentRequired bool                          `json:"security_review_intent_required"`
}

// Confirmations belong to an explicit start/resume request and are never persisted.
// Resuming an interrupted run must obtain fresh intent for the remaining requests.
type AnalysisRunConfirmations struct {
	ProviderIDs    []string `json:"provider_ids"`
	SecurityReview bool     `json:"security_review"`
}

type AnalysisRunStartRequest struct {
	Identity      AnalysisQueueIdentity    `json:"identity"`
	PreviewID     string                   `json:"preview_id"`
	Limits        AnalysisRunLimits        `json:"limits"`
	Refresh       bool                     `json:"refresh"`
	Confirmations AnalysisRunConfirmations `json:"confirmations"`
}

// Validate checks the request shape; admission must also recompute the preview
// and verify confirmations against its effective provider requirements.
func (request AnalysisRunStartRequest) Validate() error {
	if err := request.Identity.Validate(); err != nil {
		return err
	}
	if request.PreviewID == "" {
		return fmt.Errorf("analysis start requires an admission preview")
	}
	return request.Limits.Validate()
}

type AnalysisRunAction string

const (
	AnalysisRunPause  AnalysisRunAction = "pause"
	AnalysisRunResume AnalysisRunAction = "resume"
	AnalysisRunCancel AnalysisRunAction = "cancel"
)

type AnalysisRunControlRequest struct {
	Identity      AnalysisRunIdentity       `json:"identity"`
	Action        AnalysisRunAction         `json:"action"`
	PreviewID     string                    `json:"preview_id,omitempty"`
	Confirmations *AnalysisRunConfirmations `json:"confirmations,omitempty"`
}

func (request AnalysisRunControlRequest) Validate() error {
	if err := request.Identity.Validate(); err != nil {
		return err
	}
	switch request.Action {
	case AnalysisRunResume:
		if request.PreviewID == "" || request.Confirmations == nil {
			return fmt.Errorf("analysis resume requires a fresh preview and confirmations")
		}
	case AnalysisRunPause, AnalysisRunCancel:
		if request.PreviewID != "" || request.Confirmations != nil {
			return fmt.Errorf("analysis pause or cancel cannot carry admission confirmations")
		}
	default:
		return fmt.Errorf("analysis control action is invalid")
	}
	return nil
}

// Coverage counts file-stage work units. Partial reports retain useful evidence but
// do not count as completely covered units; reused current reports count as succeeded.
type AnalysisRunCoverage struct {
	Total       int `json:"total"`
	Pending     int `json:"pending"`
	Running     int `json:"running"`
	Succeeded   int `json:"succeeded"`
	Partial     int `json:"partial"`
	Failed      int `json:"failed"`
	Skipped     int `json:"skipped"`
	Unavailable int `json:"unavailable"`
}

func (coverage AnalysisRunCoverage) Validate() error {
	remaining := coverage.Total
	if remaining < 0 {
		return fmt.Errorf("analysis coverage cannot be negative")
	}
	for _, count := range []int{coverage.Pending, coverage.Running, coverage.Succeeded, coverage.Partial, coverage.Failed, coverage.Skipped, coverage.Unavailable} {
		if count < 0 || count > remaining {
			return fmt.Errorf("analysis coverage counts do not match total")
		}
		remaining -= count
	}
	if remaining != 0 {
		return fmt.Errorf("analysis coverage counts do not match total")
	}
	return nil
}

type AnalysisSectionProgress struct {
	Category     project.FindingCategory `json:"category"`
	Status       AnalysisRunStatus       `json:"status"`
	Coverage     AnalysisRunCoverage     `json:"coverage"`
	FindingCount *int                    `json:"finding_count"`
}

func (section AnalysisSectionProgress) Validate() error {
	if !section.Category.Valid() || !section.Status.Valid() {
		return fmt.Errorf("analysis section category or status is invalid")
	}
	if err := section.Coverage.Validate(); err != nil {
		return err
	}
	covered := section.Coverage.Succeeded + section.Coverage.Partial
	if (covered == 0) != (section.FindingCount == nil) || (section.FindingCount != nil && *section.FindingCount < 0) {
		return fmt.Errorf("analysis finding count must describe successful evidence, or be unknown")
	}
	if section.Status == AnalysisRunCompleted || section.Status == AnalysisRunCompletedEmpty {
		if section.Coverage.Total == 0 || section.Coverage.Succeeded != section.Coverage.Total || section.FindingCount == nil {
			return fmt.Errorf("completed analysis requires full successful coverage")
		}
		if (section.Status == AnalysisRunCompletedEmpty) != (*section.FindingCount == 0) {
			return fmt.Errorf("completed analysis status does not match finding count")
		}
	}
	if section.Status == AnalysisRunPartial && (covered == 0 || section.Coverage.Succeeded == section.Coverage.Total) {
		return fmt.Errorf("partial analysis requires useful but incomplete coverage")
	}
	if section.Status == AnalysisRunPartial && (section.Coverage.Pending != 0 || section.Coverage.Running != 0) {
		return fmt.Errorf("partial analysis cannot hide pending work")
	}
	if section.Status == AnalysisRunFailed && (covered != 0 || section.Coverage.Failed == 0 || section.Coverage.Pending != 0 || section.Coverage.Running != 0) {
		return fmt.Errorf("failed analysis requires terminal work without successful evidence")
	}
	if section.Status == AnalysisRunUnavailable && (covered != 0 || section.Coverage.Failed != 0 || section.Coverage.Pending != 0 || section.Coverage.Running != 0) {
		return fmt.Errorf("unavailable analysis cannot hide eligible work")
	}
	return nil
}

type AnalysisStageProgress struct {
	Stage        AnalysisStage       `json:"stage"`
	Status       AnalysisStageStatus `json:"status"`
	Attempts     int                 `json:"attempts"`
	Cached       bool                `json:"cached"`
	FindingCount *int                `json:"finding_count"`
	ReportID     string              `json:"report_id,omitempty"`
	Reason       string              `json:"reason,omitempty"`
}

type AnalysisRunFile struct {
	AnalysisFileIdentity
	Stages []AnalysisStageProgress `json:"stages"`
}

// AnalysisRun persists only identities and operational progress. Result prose lives
// in producer-owned report stores; no source, prompt, transcript or consent is saved here.
type AnalysisRun struct {
	CompatibilityElapsed time.Duration             `json:"compatibility_elapsed_nanoseconds,omitempty"`
	SchemaVersion        string                    `json:"schema_version"`
	Identity             AnalysisRunIdentity       `json:"identity"`
	Plan                 AnalysisRunPreview        `json:"plan"`
	Status               AnalysisRunStatus         `json:"status"`
	Files                []AnalysisRunFile         `json:"files"`
	Sections             []AnalysisSectionProgress `json:"sections"`
	ElapsedSeconds       int64                     `json:"elapsed_seconds"`
	WindowElapsedSeconds int64                     `json:"window_elapsed_seconds"`
	WindowFilesCompleted int                       `json:"window_files_completed"`
	Reason               string                    `json:"reason,omitempty"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
}

// AnalysisSectionResults preserves each producer's evidence/triage shape. Semantic
// risks are routed by explicit category; performance hypotheses and Security rule/AI
// reports keep their existing identities. File filtering only changes this read view.
type AnalysisSectionResults struct {
	Identity     AnalysisRunIdentity             `json:"identity"`
	Progress     AnalysisSectionProgress         `json:"progress"`
	Path         string                          `json:"path,omitempty"`
	Semantic     []project.UnifiedFinding        `json:"semantic"`
	Performance  []project.PerformanceFileReport `json:"performance"`
	Security     []project.SecurityFileReport    `json:"security"`
	Unclassified []project.UnifiedFinding        `json:"unclassified"`
}

var errAnalysisRunPersistence = errors.New("analysis progress could not be saved; resume or cancel to recover")
var errAnalysisRunBusy = fmt.Errorf("%w: an analysis run is already active; pause or cancel it before replacement", project.ErrRevisionConflict)

// ErrAnalysisProgressUnavailable identifies recoverable progress storage failures.
var ErrAnalysisProgressUnavailable = errAnalysisRunPersistence

// One mutex owns admission, attempt reservations, progress and report publication.
// Disk writes remain inside that boundary. The worker never takes jobLifecycleMu,
// so project changes can invalidate it without waiting while holding its lock.
type analysisRunRecovery struct {
	run   *AnalysisRun
	fault error
}

type analysisRunController struct {
	detached                 map[string]analysisRunRecovery
	mu                       sync.Mutex
	run                      *AnalysisRun
	root                     string
	fault                    error
	done                     chan struct{}
	cancel                   context.CancelFunc
	admission                *AnalysisRunPreview
	confirmations            AnalysisRunConfirmations
	windowStart              time.Time
	elapsedBase              int64
	compatibilityElapsedBase time.Duration
	windowBudget             time.Duration
}

func (s *Service) StartAnalysisRun(ctx context.Context, request AnalysisRunStartRequest) (*AnalysisRun, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	return s.startAnalysisRunLocked(ctx, request, "", 0)
}

// Caller holds both lifecycle and controller locks.
func (s *Service) startAnalysisRunLocked(ctx context.Context, request AnalysisRunStartRequest, stage AnalysisStage, budget time.Duration) (*AnalysisRun, error) {
	c := s.analysisRun
	if err := s.restoreAnalysisRunLocked(); err != nil {
		return nil, err
	}
	if c.done != nil {
		return nil, errAnalysisRunBusy
	}
	if c.fault != nil {
		return cloneAnalysisRun(c.run), errAnalysisRunPersistence
	}
	if c.run != nil {
		switch c.run.Status {
		case AnalysisRunRunning, AnalysisRunQueued, AnalysisRunPausing, AnalysisRunPaused, AnalysisRunInterrupted:
			return cloneAnalysisRun(c.run), errAnalysisRunBusy
		}
	}
	preview, err := s.analysisPreviewLocked(ctx, AnalysisPreviewRequest{ProjectID: request.Identity.ProjectID, ProjectRevision: request.Identity.ProjectRevision, Scope: AnalysisRunScopeProject, Refresh: request.Refresh, Limits: request.Limits, compatibilityStage: stage, compatibilityBudget: budget})
	if err != nil {
		return nil, err
	}
	if preview.Identity != request.Identity || preview.PreviewID != request.PreviewID {
		return nil, project.ErrRevisionConflict
	}
	if err := validateAnalysisConfirmations(preview, request.Confirmations); err != nil {
		return nil, err
	}
	if err := s.validateAnalysisQueue(ctx, s.manager.Root(), preview, true); err != nil {
		return nil, err
	}
	c.root = s.manager.Root()
	c.run = newAnalysisRun(*preview)
	if err := s.saveAnalysisRunLocked(false); err != nil {
		return cloneAnalysisRun(c.run), err
	}
	result := cloneAnalysisRun(c.run)
	s.launchAnalysisRunLocked(preview, request.Confirmations)
	return result, nil
}

// CurrentAnalysisRun can restore metadata but never starts a worker or reuses consent.
// A recoverable persistence fault returns the retained progress alongside its error.
func (s *Service) CurrentAnalysisRun(ctx context.Context) (*AnalysisRun, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.currentAnalysisRunLocked(ctx)
}

// The caller holds jobLifecycleMu through its complete project read.
func (s *Service) currentAnalysisRunLocked(ctx context.Context) (*AnalysisRun, error) {
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.restoreAnalysisRunLocked(); err != nil && (c.run == nil || c.fault == nil) {
		return cloneAnalysisRun(c.run), err
	}
	if c.run == nil {
		return nil, nil
	}
	if c.run.Status != AnalysisRunStale {
		if err := s.validateAnalysisQueue(ctx, c.root, &c.run.Plan, true); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.staleAnalysisRunLocked()
			if saveErr := s.saveAnalysisRunLocked(false); saveErr != nil {
				return cloneAnalysisRun(c.run), saveErr
			}
		}
	}
	s.updateAnalysisElapsedLocked()
	return cloneAnalysisRun(c.run), c.fault
}

func (s *Service) ControlAnalysisRun(ctx context.Context, request AnalysisRunControlRequest) (*AnalysisRun, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	// Recoverable write failures retain the in-memory state; only explicit controls
	// may attempt another save. Restore errors without a retained run remain fatal.
	if err := s.restoreAnalysisRunLocked(); err != nil && (c.run == nil || c.fault == nil) {
		return nil, err
	}
	if c.run == nil || c.run.Identity != request.Identity {
		return nil, project.ErrRevisionConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch request.Action {
	case AnalysisRunCancel:
		if c.run.Status == AnalysisRunStale {
			if c.fault == nil {
				return cloneAnalysisRun(c.run), project.ErrRevisionConflict
			}
			// Discarding stale work must remain possible after a failed save.
			// Recovery preserves its stale identity and cannot restart requests.
			c.run.Reason = "Project source, policy or provider identity changed; start a new analysis."
			err := s.saveAnalysisRunLocked(true)
			return cloneAnalysisRun(c.run), err
		}
		c.run.Status = AnalysisRunCanceled
		if c.done != nil {
			c.run.Status = AnalysisRunCanceling
		}
		if c.cancel != nil {
			c.cancel()
		}
		c.run.Reason = "Analysis canceled by the user."
		interruptAnalysisStages(c.run, AnalysisStageCanceled)
		if err := s.saveAnalysisRunLocked(true); err != nil {
			return cloneAnalysisRun(c.run), err
		}
	case AnalysisRunPause:
		if c.run.Status != AnalysisRunRunning && c.run.Status != AnalysisRunQueued && c.run.Status != AnalysisRunPausing && c.run.Status != AnalysisRunPaused && c.fault == nil {
			return nil, fmt.Errorf("%w: analysis cannot be paused in its current state", project.ErrRevisionConflict)
		}
		c.run.Status = AnalysisRunPaused
		if c.done != nil && c.fault == nil {
			c.run.Status = AnalysisRunPausing
		}
		c.run.Reason = "Analysis paused by the user."
		if err := s.saveAnalysisRunLocked(true); err != nil {
			return cloneAnalysisRun(c.run), err
		}
	case AnalysisRunResume:
		if c.done != nil {
			return cloneAnalysisRun(c.run), errAnalysisRunBusy
		}
		if c.run.Status != AnalysisRunPaused && c.run.Status != AnalysisRunInterrupted {
			return nil, fmt.Errorf("%w: analysis requires paused or interrupted progress to resume", project.ErrRevisionConflict)
		}
		if c.run.Plan.CompatibilityBudget > 0 && c.run.CompatibilityElapsed >= c.run.Plan.CompatibilityBudget {
			return nil, fmt.Errorf("%w: performance job budget is exhausted; start a new job", project.ErrRevisionConflict)
		}
		preview, err := s.analysisPreviewLocked(ctx, AnalysisPreviewRequest{ProjectID: c.run.Identity.ProjectID, ProjectRevision: c.run.Identity.ProjectRevision, Scope: AnalysisRunScopeProject, Refresh: c.run.Plan.Refresh, Limits: c.run.Plan.Limits, ResumeRun: &request.Identity})
		if err != nil {
			return nil, err
		}
		if request.PreviewID != preview.PreviewID {
			return nil, project.ErrRevisionConflict
		}
		if err := validateAnalysisConfirmations(preview, *request.Confirmations); err != nil {
			return nil, err
		}
		if err := s.validateAnalysisQueue(ctx, c.root, preview, true); err != nil {
			return nil, err
		}
		c.run.Identity.Generation = newOpaqueID("generation")
		c.run.Status = AnalysisRunQueued
		c.run.Reason = ""
		c.run.WindowElapsedSeconds = 0
		c.run.WindowFilesCompleted = 0
		if err := s.saveAnalysisRunLocked(true); err != nil {
			return cloneAnalysisRun(c.run), err
		}
		result := cloneAnalysisRun(c.run)
		s.launchAnalysisRunLocked(preview, *request.Confirmations)
		return result, nil
	}
	return cloneAnalysisRun(c.run), nil
}

func (s *Service) launchAnalysisRunLocked(preview *AnalysisRunPreview, confirmations AnalysisRunConfirmations) {
	c := s.analysisRun
	c.windowBudget = time.Duration(c.run.Plan.Limits.BudgetSeconds) * time.Second
	if c.run.Plan.CompatibilityBudget > 0 {
		c.windowBudget = min(c.windowBudget, c.run.Plan.CompatibilityBudget-c.run.CompatibilityElapsed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.windowBudget)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.admission = preview
	c.confirmations = AnalysisRunConfirmations{SecurityReview: confirmations.SecurityReview, ProviderIDs: append([]string(nil), confirmations.ProviderIDs...)}
	c.windowStart = time.Now()
	c.elapsedBase = c.run.ElapsedSeconds
	c.compatibilityElapsedBase = c.run.CompatibilityElapsed
	go s.runAnalysisWindow(ctx, c.run.Identity, c.done)
}

func (s *Service) runAnalysisWindow(ctx context.Context, identity AnalysisRunIdentity, done chan struct{}) {
	c := s.analysisRun
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.done == done {
			if c.cancel != nil {
				c.cancel()
			}
			c.cancel = nil
			c.done = nil
			c.admission = nil
			c.confirmations = AnalysisRunConfirmations{}
			c.windowStart = time.Time{}
		}
		close(done)
	}()
	for {
		c.mu.Lock()
		if c.run == nil || c.run.Identity != identity || c.fault != nil {
			c.mu.Unlock()
			return
		}
		if c.run.Status == AnalysisRunQueued {
			c.run.Status = AnalysisRunRunning
		}
		if s.finishAnalysisWindowLocked(ctx) {
			c.mu.Unlock()
			return
		}
		fileIndex, stageIndex, found := nextAnalysisStage(c.run)
		if !found {
			if err := s.validateAnalysisQueue(ctx, c.root, &c.run.Plan, true); err != nil {
				s.staleAnalysisRunLocked()
			} else {
				refreshAnalysisSections(c.run)
				c.run.Status = analysisFinishedStatus(c.run)
			}
			_ = s.saveAnalysisRunLocked(false) // Failure is retained and stops this worker.
			c.mu.Unlock()
			return
		}
		if c.run.WindowFilesCompleted >= c.run.Plan.Limits.BatchFiles {
			c.run.Status = AnalysisRunPaused
			c.run.Reason = "The batch limit was reached; resume to continue pending files."
			_ = s.saveAnalysisRunLocked(false)
			c.mu.Unlock()
			return
		}
		stage := &c.run.Files[fileIndex].Stages[stageIndex]
		if stage.Stage != AnalysisStageSecurityRules && stage.Attempts >= c.run.Plan.Limits.MaxAttemptsPerStage && !c.admission.Files[fileIndex].Stages[stageIndex].Cached {
			stage.Status = AnalysisStageFailed
			stage.Reason = "The stage exhausted its total attempt allowance."
			if analysisFileFinished(c.run.Files[fileIndex]) {
				c.run.WindowFilesCompleted++
			}
			if err := s.saveAnalysisRunLocked(false); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
			continue
		}
		stage.Status = AnalysisStageRunning
		stage.Reason = ""
		if err := s.saveAnalysisRunLocked(false); err != nil {
			c.mu.Unlock()
			return
		}
		plan := c.admission.Files[fileIndex].Stages[stageIndex]
		confirmed := false
		for _, id := range c.confirmations.ProviderIDs {
			confirmed = confirmed || id == plan.ProviderID
		}
		request := analysisFileStageRequest{Run: identity, File: c.run.Files[fileIndex].AnalysisFileIdentity, Stage: stage.Stage, Refresh: c.run.Plan.Refresh,
			RemainingAttempts: min(plan.MaxModelRequests, c.run.Plan.Limits.MaxAttemptsPerStage-stage.Attempts), ConfirmRemoteProvider: confirmed, SecurityReview: c.confirmations.SecurityReview}
		authority := s.analysisRunAuthority(fileIndex, stageIndex)
		c.mu.Unlock()
		result, err := s.analyzeFileStage(ctx, request, authority)
		c.mu.Lock()
		if c.run == nil || c.run.Identity != identity || c.fault != nil {
			c.mu.Unlock()
			return
		}
		if c.run.Status == AnalysisRunStale || c.run.Status == AnalysisRunCanceled || c.run.Status == AnalysisRunCanceling {
			s.finishAnalysisWindowLocked(ctx)
			c.mu.Unlock()
			return
		}
		if err != nil {
			current := &c.run.Files[fileIndex].Stages[stageIndex]
			current.Status = AnalysisStageInterrupted
			current.Reason = "Analysis stage could not complete."
			if errors.Is(err, project.ErrRevisionConflict) {
				s.staleAnalysisRunLocked()
			} else if ctx.Err() != nil || errors.Is(err, errAnalysisAttemptBudget) {
				c.run.Status = AnalysisRunPaused
				c.run.Reason = "The dispatch allowance ended; preview and resume the remaining work."
			} else {
				c.run.Status = AnalysisRunInterrupted
				c.run.Reason = "Analysis stopped before the stage completed; review and resume."
			}
			_ = s.saveAnalysisRunLocked(false)
			c.mu.Unlock()
			return
		}
		recordAnalysisResult(c.run, fileIndex, stageIndex, result)
		if analysisFileFinished(c.run.Files[fileIndex]) {
			c.run.WindowFilesCompleted++
		}
		if err := s.saveAnalysisRunLocked(false); err != nil {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
}

func (s *Service) finishAnalysisWindowLocked(ctx context.Context) bool {
	c := s.analysisRun
	switch c.run.Status {
	case AnalysisRunStale, AnalysisRunCanceled:
		return true
	case AnalysisRunCanceling:
		c.run.Status = AnalysisRunCanceled
		interruptAnalysisStages(c.run, AnalysisStageCanceled)
	case AnalysisRunPausing:
		c.run.Status = AnalysisRunPaused
	case AnalysisRunRunning:
		if ctx.Err() == nil {
			return false
		}
		c.run.Status = AnalysisRunPaused
		c.run.Reason = "The dispatch allowance ended; preview and resume the remaining work."
		interruptAnalysisStages(c.run, AnalysisStageInterrupted)
	default:
		return true
	}
	_ = s.saveAnalysisRunLocked(false)
	return true
}

func (s *Service) analysisRunAuthority(fileIndex, stageIndex int) analysisFileStageAuthority {
	c := s.analysisRun
	validate := func(ctx context.Context, identity AnalysisRunIdentity, file AnalysisFileIdentity, stage AnalysisStage) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if c.run == nil || c.run.Identity != identity || c.run.Files[fileIndex].AnalysisFileIdentity != file || c.run.Files[fileIndex].Stages[stageIndex].Stage != stage {
			return project.ErrRevisionConflict
		}
		if c.fault != nil {
			return errAnalysisRunPersistence
		}
		if c.run.Status != AnalysisRunRunning && c.run.Status != AnalysisRunPausing {
			return project.ErrRevisionConflict
		}
		return s.validateAnalysisQueue(ctx, c.root, &c.run.Plan, false)
	}
	return analysisFileStageAuthority{
		Check: func(ctx context.Context, identity AnalysisRunIdentity) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.run == nil || c.run.Identity != identity {
				return project.ErrRevisionConflict
			}
			return validate(ctx, identity, c.run.Files[fileIndex].AnalysisFileIdentity, c.run.Files[fileIndex].Stages[stageIndex].Stage)
		},
		BeforeAttempt: func(ctx context.Context, identity AnalysisRunIdentity, file AnalysisFileIdentity, stage AnalysisStage) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			if err := validate(ctx, identity, file, stage); err != nil {
				return err
			}
			progress := &c.run.Files[fileIndex].Stages[stageIndex]
			if progress.Attempts >= c.run.Plan.Limits.MaxAttemptsPerStage {
				return errAnalysisAttemptBudget
			}
			progress.Attempts++
			return s.saveAnalysisRunLocked(false)
		},
		Publish: func(identity AnalysisRunIdentity, file AnalysisFileIdentity, stage AnalysisStage, write func() error) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			if err := validate(context.Background(), identity, file, stage); err != nil {
				return err
			}
			if err := write(); err != nil {
				if errors.Is(err, project.ErrRevisionConflict) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				s.failAnalysisPersistenceLocked()
				return errAnalysisRunPersistence
			}
			return nil
		},
	}
}

func (s *Service) updateAnalysisElapsedLocked() {
	c := s.analysisRun
	if c.run == nil || c.windowStart.IsZero() {
		return
	}
	active := min(time.Since(c.windowStart), c.windowBudget)
	if c.run.Plan.CompatibilityBudget > 0 {
		c.run.CompatibilityElapsed = min(c.run.Plan.CompatibilityBudget, c.compatibilityElapsedBase+active)
	}
	elapsed := int64(active / time.Second)
	c.run.WindowElapsedSeconds = elapsed
	c.run.ElapsedSeconds = c.elapsedBase + elapsed
}

func (s *Service) staleAnalysisRunLocked() {
	c := s.analysisRun
	c.run.Status = AnalysisRunStale
	c.run.Reason = "Project source, policy or provider identity changed; start a new analysis."
	interruptAnalysisStages(c.run, AnalysisStageStale)
	if c.cancel != nil {
		c.cancel()
	}
}

func (s *Service) invalidateAnalysisRun() {
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.run == nil {
		return
	}
	s.staleAnalysisRunLocked()
	_ = s.saveAnalysisRunLocked(false) // Retained fault blocks any further dispatch.
}
