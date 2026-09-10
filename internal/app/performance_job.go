package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	performanceJobRunning   = "running"
	performanceJobPaused    = "paused"
	performanceJobCanceled  = "canceled"
	performanceJobCompleted = "completed"
	performanceJobStale     = "stale"

	performanceFilePending   = "pending"
	performanceFileRunning   = "running"
	performanceFileCompleted = "completed"
	performanceFileCached    = "cached"
	performanceFileFailed    = "failed"
	performanceFileSkipped   = "skipped"

	defaultPerformanceFileLimit = 100
	maxPerformanceFileLimit     = 500
	defaultPerformanceRunBudget = 15 * time.Minute
	maxPerformanceRunBudget     = time.Hour
)

// PerformanceJobOptions constrains an explicit source-based review run.
type PerformanceJobOptions struct {
	MaxFiles          int           `json:"max_files,omitempty"`
	RunBudget         time.Duration `json:"run_budget,omitempty"`
	QueueID           string        `json:"queue_id,omitempty"`
	PolicyFingerprint string        `json:"policy_fingerprint,omitempty"`
}

// PerformanceJobFile captures the exact file identity that the job is allowed to review.
type PerformanceJobFile struct {
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`
	Status      string `json:"status"`
	Attempts    int    `json:"attempts"`
	Reason      string `json:"reason,omitempty"`
	Error       string `json:"error,omitempty"`
}

// PerformanceJob is source-free durable progress for a project-scoped review.
type PerformanceJob struct {
	ID                string               `json:"id"`
	Generation        string               `json:"generation"`
	ProjectID         string               `json:"project_id"`
	ProjectRevision   string               `json:"project_revision"`
	Root              string               `json:"root"`
	PolicyFingerprint string               `json:"policy_fingerprint"`
	QueueID           string               `json:"queue_id"`
	Status            string               `json:"status"`
	MaxFiles          int                  `json:"max_files"`
	RunBudget         time.Duration        `json:"run_budget"`
	Elapsed           time.Duration        `json:"elapsed"`
	ActiveStartedAt   time.Time            `json:"active_started_at,omitempty"`
	Files             []PerformanceJobFile `json:"files"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}

// PerformanceQueuePreview describes a prospective queue without invoking a provider.
type PerformanceQueuePreview struct {
	ProjectID         string               `json:"project_id"`
	ProjectRevision   string               `json:"project_revision"`
	PolicyFingerprint string               `json:"policy_fingerprint"`
	QueueID           string               `json:"queue_id"`
	MaxFiles          int                  `json:"max_files"`
	Files             []PerformanceJobFile `json:"files"`
	Excluded          int                  `json:"excluded"`
	Oversized         int                  `json:"oversized"`
	OutsideLimit      int                  `json:"outside_limit"`
	Provider          EffectiveModel       `json:"provider"`
}

// PerformanceReport is derived from the captured queue and matching file reports.
// It deliberately reports review coverage rather than performance measurements.
type PerformanceReport struct {
	ProjectID       string                       `json:"project_id"`
	ProjectRevision string                       `json:"project_revision"`
	QueueID         string                       `json:"queue_id"`
	Status          string                       `json:"status"`
	Counts          map[string]int               `json:"counts"`
	Categories      map[string]int               `json:"categories"`
	Paths           map[string]string            `json:"paths"`
	Findings        []project.PerformanceFinding `json:"findings"`
}

var errPerformancePersistence = errors.New("performance progress could not be saved; resume the job to recover")

type detachedPerformanceFault struct {
	job   *PerformanceJob
	cause error
}

type performanceController struct {
	mu               sync.Mutex
	job              *PerformanceJob
	durableJob       *PerformanceJob
	cancel           context.CancelFunc
	workerGeneration string
	requestStarted   time.Time
	persistMu        sync.Mutex
	beforeRead       func()
	workerDone       chan struct{}
	persistenceFault error
	detachedFaults   map[string]detachedPerformanceFault
}

func newPerformanceController() *performanceController { return &performanceController{} }

// PreviewPerformanceQueue produces the exact source-free queue a start request may bind to.
func (s *Service) PreviewPerformanceQueue(options PerformanceJobOptions) (*PerformanceQueuePreview, error) {
	return s.buildPerformanceQueue(normalizePerformanceOptions(options))
}

func (s *Service) buildPerformanceQueue(options PerformanceJobOptions) (*PerformanceQueuePreview, error) {
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return nil, err
	}
	files, excluded, oversized, outsideLimit, err := s.performanceCandidates(index, policy, options.MaxFiles)
	if err != nil {
		return nil, err
	}
	return &PerformanceQueuePreview{
		ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
		PolicyFingerprint: policy.Version(), QueueID: performanceQueueID(analysis.ProjectID, analysis.ProjectRevision, policy.Version(), files),
		MaxFiles: options.MaxFiles, Files: files, Excluded: excluded, Oversized: oversized, OutsideLimit: outsideLimit,
		Provider: s.runtimes.analyze.effective,
	}, nil
}

// StartPerformanceJob starts only the exact previewed, bounded queue.
func (s *Service) StartPerformanceJob(ctx context.Context, options PerformanceJobOptions, confirmRemoteProvider bool) (*PerformanceJob, error) {
	if err := s.RequireRemoteConfirmation(config.AnalyzeModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	if s.analyzeAllActive() {
		return nil, fmt.Errorf("analyze-all is already active")
	}
	if active, err := s.PerformanceJob(); err != nil {
		return nil, err
	} else if active != nil && (active.Status == performanceJobRunning || active.Status == performanceJobPaused) {
		return active, nil
	}
	input, err := s.preparePerformanceStart(options)
	if err != nil {
		return nil, err
	}
	return s.activatePerformanceStart(ctx, input)
}

type performanceStartInput struct {
	options PerformanceJobOptions
	preview *PerformanceQueuePreview
	root    string
}

func (s *Service) preparePerformanceStart(options PerformanceJobOptions) (performanceStartInput, error) {
	options = normalizePerformanceOptions(options)
	preview, err := s.buildPerformanceQueue(options)
	if err != nil {
		return performanceStartInput{}, err
	}
	if err := validatePerformancePreview(options, preview); err != nil {
		return performanceStartInput{}, err
	}
	return performanceStartInput{options: options, preview: preview, root: s.manager.Root()}, nil
}

func validatePerformancePreview(options PerformanceJobOptions, preview *PerformanceQueuePreview) error {
	if options.QueueID != "" && options.QueueID != preview.QueueID {
		return project.ErrRevisionConflict
	}
	if options.PolicyFingerprint != "" && options.PolicyFingerprint != preview.PolicyFingerprint {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) activatePerformanceStart(ctx context.Context, input performanceStartInput) (*PerformanceJob, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if active, err := s.activePerformanceJobLocked(); err != nil {
		return nil, err
	} else if active != nil {
		return active, nil
	}
	if s.activeAnalyzeAllJob() {
		return nil, fmt.Errorf("analyze-all is already active")
	}
	preview, err := s.buildPerformanceQueue(input.options)
	if err != nil {
		return nil, err
	}
	if err := s.verifyPerformanceStart(input, preview); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	job := &PerformanceJob{
		ID: fmt.Sprintf("performance-%d", now.UnixNano()), Generation: fmt.Sprintf("%d", now.UnixNano()),
		ProjectID: preview.ProjectID, ProjectRevision: preview.ProjectRevision, Root: input.root,
		PolicyFingerprint: preview.PolicyFingerprint, QueueID: preview.QueueID, Status: performanceJobRunning,
		MaxFiles: preview.MaxFiles, RunBudget: input.options.RunBudget,
		Files: preview.Files, CreatedAt: now, UpdatedAt: now,
	}
	s.performance.mu.Lock()
	if activePerformanceJob(s.performance.job) {
		copy := clonePerformanceJob(s.performance.job)
		s.performance.mu.Unlock()
		return copy, nil
	}
	s.performance.job = job
	s.performance.durableJob = nil
	s.performance.persistenceFault = nil
	s.performance.mu.Unlock()
	if err := s.persistPerformanceJob(job); err != nil {
		s.clearPerformanceJob(job)
		return nil, err
	}
	published := clonePerformanceJob(job)
	s.startPerformanceWorker(ctx)
	return published, nil
}

func (s *Service) verifyPerformanceStart(input performanceStartInput, preview *PerformanceQueuePreview) error {
	if s.manager.Root() != input.root || preview.ProjectID != input.preview.ProjectID || preview.ProjectRevision != input.preview.ProjectRevision || preview.PolicyFingerprint != input.preview.PolicyFingerprint || preview.QueueID != input.preview.QueueID || preview.Provider != input.preview.Provider {
		return project.ErrRevisionConflict
	}
	return validatePerformancePreview(input.options, preview)
}

func (s *Service) activePerformanceJobLocked() (*PerformanceJob, error) {
	job, err := s.performanceJobLocked()
	if err != nil || !activePerformanceJob(job) {
		return nil, err
	}
	return job, nil
}

func (s *Service) activeAnalyzeAllJob() bool {
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	return activeAnalyzeAllJob(s.analysisAll.job)
}

func (s *Service) analyzeAllActive() bool {
	active, _ := s.AnalyzeAllJob()
	return activeAnalyzeAllJob(active)
}

func activeAnalyzeAllJob(job *AnalyzeAllJob) bool {
	return job != nil && (job.Status == analysisAllStateRunning || job.Status == analysisAllStatePaused)
}

func activePerformanceJob(job *PerformanceJob) bool {
	return job != nil && (job.Status == performanceJobRunning || job.Status == performanceJobPaused)
}

// PerformanceJob returns the current or persisted job for the active project revision.
func (s *Service) PerformanceJob() (*PerformanceJob, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.performanceJobLocked()
}

// performanceJobLocked reads, loads, attaches, and validates a performance job
// while the active-project lifecycle remains stable.
func (s *Service) performanceJobLocked() (*PerformanceJob, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	s.performance.mu.Lock()
	beforeRead := s.performance.beforeRead
	s.performance.mu.Unlock()
	if beforeRead != nil {
		beforeRead()
	}
	job, err := s.performance.progressSnapshot(s.manager.Root())
	if err != nil {
		return nil, err
	}
	if job == nil {
		job, err = loadPerformanceJob(s.manager.Root())
		if err != nil || job == nil {
			return job, err
		}
		recovered := job.ProjectID == analysis.ProjectID && job.ProjectRevision == analysis.ProjectRevision && recoverPersistedPerformanceJob(job, time.Now().UTC())
		s.performance.mu.Lock()
		s.performance.job = clonePerformanceJob(job)
		s.performance.durableJob = clonePerformanceJob(job)
		s.performance.mu.Unlock()
		if recovered {
			if err := s.persistPerformanceJob(job); err != nil {
				return nil, err
			}
		}
	}
	if job.ProjectID != analysis.ProjectID || job.ProjectRevision != analysis.ProjectRevision {
		return s.markPerformanceStale(job)
	}
	return job, nil
}

// progressSnapshot exposes only a successfully saved state while a write is
// pending. The fault takes precedence once a save has failed.
func (c *performanceController) progressSnapshot(root string) (*PerformanceJob, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if detached, ok := c.detachedFaults[root]; c.job == nil && ok {
		c.job = clonePerformanceJob(detached.job)
		c.persistenceFault = detached.cause
		delete(c.detachedFaults, root)
	}
	if c.persistenceFault != nil {
		return nil, errPerformancePersistence
	}
	if c.durableJob != nil {
		return clonePerformanceJob(c.durableJob), nil
	}
	return clonePerformanceJob(c.job), nil
}

// PausePerformanceJob lets the active single-file request finish, then stops the queue.
func (s *Service) PausePerformanceJob() (*PerformanceJob, error) {
	return s.changePerformanceState(performanceJobPaused, false)
}

// CancelPerformanceJob interrupts active provider work and retains earlier reports.
func (s *Service) CancelPerformanceJob() (*PerformanceJob, error) {
	return s.changePerformanceState(performanceJobCanceled, true)
}

// ResumePerformanceJob continues the same captured queue and remaining budget.
func (s *Service) ResumePerformanceJob(ctx context.Context, expectedID string, confirmRemoteProvider bool) (*PerformanceJob, error) {
	if err := s.RequireRemoteConfirmation(config.AnalyzeModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	if _, err := s.AnalyzeAllJob(); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if s.activeAnalyzeAllJob() {
		return nil, fmt.Errorf("analyze-all is already active")
	}
	job, fault, finish, err := s.beginPerformanceRecoveryLocked()
	if err != nil {
		return nil, err
	}
	defer finish()
	if job == nil || (expectedID != "" && expectedID != job.ID) {
		return nil, fmt.Errorf("performance job is not paused")
	}
	published, dispatch, err := s.preparePerformanceResume(job, fault)
	if err != nil {
		return nil, err
	}
	if err := s.persistPerformanceJobSerialized(published, true); err != nil {
		return nil, err
	}
	if dispatch {
		s.startPerformanceWorker(ctx)
	}
	return published, nil
}

func (s *Service) preparePerformanceResume(job *PerformanceJob, fault bool) (*PerformanceJob, bool, error) {
	// Detached and canceled jobs can only be durably finalized. A new explicit
	// start may follow successful recovery, but this action dispatches no work.
	if fault && (job.Status == performanceJobStale || job.Status == performanceJobCanceled) {
		return job, false, nil
	}
	if job.Status != performanceJobPaused {
		return nil, false, fmt.Errorf("performance job is not paused")
	}
	if err := s.verifyPerformanceJob(job); err != nil {
		return nil, false, err
	}
	exhausted := job.Elapsed >= job.RunBudget
	if exhausted && !fault {
		return nil, false, fmt.Errorf("performance job budget is exhausted")
	}
	published, err := s.updatePerformanceJob(job, func(current *PerformanceJob) error {
		if current.Status != performanceJobPaused {
			return fmt.Errorf("performance job is not paused")
		}
		if fault {
			recoverPersistedPerformanceJob(current, time.Now().UTC())
		}
		current.Status = performanceJobRunning
		if exhausted {
			current.Status = performanceJobCompleted
		}
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	return published, !exhausted, err
}

// PerformanceProjectReport aggregates only reports matching the captured queue identity.
func (s *Service) PerformanceProjectReport() (*PerformanceReport, error) {
	job, err := s.PerformanceJob()
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	policy, err := project.NewContextPolicy(job.Root)
	if err != nil {
		return nil, err
	}
	report := &PerformanceReport{ProjectID: job.ProjectID, ProjectRevision: job.ProjectRevision, QueueID: job.QueueID, Status: job.Status, Counts: map[string]int{}, Categories: map[string]int{}, Paths: map[string]string{}}
	for _, file := range job.Files {
		cached, err := project.LoadPerformanceFileReport(job.Root, file.Path, file.ContentHash, policy)
		if err != nil {
			return nil, err
		}
		if cached != nil && cached.Status == performanceJobStale && performanceFileContributesCoverage(file.Status) {
			if report.Status == performanceJobCompleted {
				report.Status = performanceJobStale
			}
			report.Counts[performanceJobStale]++
			continue
		}
		report.Counts[file.Status]++
		if cached == nil || cached.Status != "completed" || cached.ProjectID != job.ProjectID || cached.ProjectRevision != job.ProjectRevision {
			continue
		}
		for _, finding := range cached.Findings {
			report.Findings = append(report.Findings, finding)
			report.Categories[finding.Category]++
			report.Paths[finding.ID] = file.Path
		}
	}
	return report, nil
}

func performanceFileContributesCoverage(status string) bool {
	return status == performanceFileCompleted || status == performanceFileCached
}

func (s *Service) startPerformanceWorker(parent context.Context) {
	s.performance.mu.Lock()
	if s.performance.persistenceFault != nil || s.performance.job == nil || s.performance.job.Status != performanceJobRunning {
		s.performance.mu.Unlock()
		return
	}
	generation := s.performance.job.Generation
	if s.performance.cancel != nil && s.performance.workerGeneration == generation {
		s.performance.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.performance.cancel = cancel
	s.performance.workerGeneration = generation
	done := make(chan struct{})
	s.performance.workerDone = done
	s.performance.mu.Unlock()
	go func() {
		defer close(done)
		s.runPerformanceJob(ctx, generation)
	}()
}

func (s *Service) runPerformanceJob(ctx context.Context, generation string) {
	defer s.clearPerformanceWorker(generation)
	for {
		job, index, ok := s.nextPerformanceFile(generation)
		if !ok {
			return
		}
		remaining := job.RunBudget - job.Elapsed
		timed, cancel := context.WithTimeout(ctx, remaining)
		_, err := s.reviewPerformanceFile(timed, index.Path, true, func(publish func() error) error {
			return s.authorizePerformancePublication(job, index.Path, index.ContentHash, publish)
		})
		cancel()
		if !s.recordPerformanceResult(job, index.Path, err) || ctx.Err() != nil {
			return
		}
	}
}

func (s *Service) clearPerformanceWorker(generation string) {
	s.performance.mu.Lock()
	defer s.performance.mu.Unlock()
	if s.performance.workerGeneration == generation {
		s.performance.cancel = nil
		s.performance.workerGeneration = ""
		s.performance.workerDone = nil
	}
}

func (s *Service) nextPerformanceFile(generation string) (*PerformanceJob, project.IndexFile, bool) {
	s.performance.mu.Lock()
	job := clonePerformanceJob(s.performance.job)
	s.performance.mu.Unlock()
	if job == nil || job.Generation != generation || job.Status != performanceJobRunning {
		return nil, project.IndexFile{}, false
	}
	if err := s.verifyPerformanceJob(job); err != nil {
		_, _ = s.markPerformanceStale(job)
		return nil, project.IndexFile{}, false
	}
	if job.Elapsed >= job.RunBudget {
		_, _ = s.completePerformanceJob(job)
		return nil, project.IndexFile{}, false
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, project.IndexFile{}, false
	}
	s.performance.mu.Lock()
	current := s.performance.job
	if !samePerformanceJob(current, job) || s.performance.persistenceFault != nil || current.Status != performanceJobRunning {
		s.performance.mu.Unlock()
		return nil, project.IndexFile{}, false
	}
	if i, file, found := pendingPerformanceFile(current, index); found {
		current.Files[i].Status = performanceFileRunning
		current.Files[i].Attempts++
		current.UpdatedAt = time.Now().UTC()
		current.ActiveStartedAt = current.UpdatedAt
		s.performance.requestStarted = current.UpdatedAt
		published := clonePerformanceJob(current)
		s.performance.mu.Unlock()
		return published, file, s.persistPerformanceAdmission(published, file.Path)
	}
	current.Status = performanceJobCompleted
	current.UpdatedAt = time.Now().UTC()
	published := clonePerformanceJob(current)
	s.performance.mu.Unlock()
	_ = s.persistPerformanceJob(published)
	return nil, project.IndexFile{}, false
}

func pendingPerformanceFile(job *PerformanceJob, index *project.ProjectIndex) (int, project.IndexFile, bool) {
	for i, queued := range job.Files {
		if queued.Status != performanceFilePending {
			continue
		}
		for _, file := range index.Files {
			if file.Path == queued.Path && file.ContentHash == queued.ContentHash {
				return i, file, true
			}
		}
	}
	return 0, project.IndexFile{}, false
}

func (s *Service) completePerformanceJob(expected *PerformanceJob) (*PerformanceJob, error) {
	published, err := s.updatePerformanceJob(expected, func(current *PerformanceJob) error {
		if current.Status != performanceJobRunning {
			return fmt.Errorf("performance job is not running")
		}
		current.Status = performanceJobCompleted
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistPerformanceJob(published); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) recordPerformanceResult(expected *PerformanceJob, path string, cause error) bool {
	s.performance.mu.Lock()
	current := s.performance.job
	if !samePerformanceJob(current, expected) || current.Status == performanceJobCanceled || current.Status == performanceJobStale {
		s.performance.mu.Unlock()
		return false
	}
	s.accountPerformanceElapsedLocked(time.Now().UTC())
	for i := range current.Files {
		if current.Files[i].Path != path || current.Files[i].Status != performanceFileRunning {
			continue
		}
		if cause == nil {
			current.Files[i].Status = performanceFileCompleted
		} else if errors.Is(cause, context.Canceled) {
			current.Files[i].Status = performanceFilePending
		} else {
			current.Files[i].Status = performanceFileFailed
			current.Files[i].Error = "source-based review failed"
		}
		current.UpdatedAt = time.Now().UTC()
		published := clonePerformanceJob(current)
		s.performance.mu.Unlock()
		return s.persistPerformanceJob(published) == nil
	}
	s.performance.mu.Unlock()
	return false
}

// beginPerformanceRecoveryLocked acquires persistence authority for an explicit
// action after loading the active job. The caller holds jobLifecycleMu and must
// call finish after preparing and saving its state. A faulted worker is joined
// without persistMu held so its final result can finish before recovery begins.
func (s *Service) beginPerformanceRecoveryLocked() (job *PerformanceJob, fault bool, finish func(), err error) {
	if _, err := s.performanceJobLocked(); err != nil && !errors.Is(err, errPerformancePersistence) {
		return nil, false, nil, err
	}
	for {
		s.performance.persistMu.Lock()
		s.performance.mu.Lock()
		job := clonePerformanceJob(s.performance.job)
		fault := s.performance.persistenceFault != nil
		done := s.performance.workerDone
		s.performance.mu.Unlock()
		if !fault || done == nil {
			return job, fault, s.performance.persistMu.Unlock, nil
		}
		s.performance.persistMu.Unlock()
		<-done
	}
}

func (s *Service) changePerformanceState(next string, cancelWorker bool) (*PerformanceJob, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	job, fault, finish, err := s.beginPerformanceRecoveryLocked()
	if err != nil {
		return nil, err
	}
	defer finish()
	if job == nil || (!fault && job.Status != performanceJobRunning) {
		return nil, fmt.Errorf("performance job is not running")
	}
	published, err := s.updatePerformanceJob(job, func(current *PerformanceJob) error {
		if !fault && current.Status != performanceJobRunning {
			return fmt.Errorf("performance job is not running")
		}
		current.Status = next
		current.UpdatedAt = time.Now().UTC()
		if cancelWorker {
			s.accountPerformanceElapsedLocked(current.UpdatedAt)
			if s.performance.cancel != nil {
				s.performance.cancel()
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistPerformanceJobSerialized(published, true); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) updatePerformanceJob(expected *PerformanceJob, update func(*PerformanceJob) error) (*PerformanceJob, error) {
	s.performance.mu.Lock()
	defer s.performance.mu.Unlock()
	current := s.performance.job
	if !samePerformanceJob(current, expected) {
		return nil, project.ErrRevisionConflict
	}
	if err := update(current); err != nil {
		return nil, err
	}
	return clonePerformanceJob(current), nil
}

func (s *Service) authorizePerformancePublication(expected *PerformanceJob, path, contentHash string, publish func() error) error {
	s.performance.mu.Lock()
	defer s.performance.mu.Unlock()
	current := s.performance.job
	if !samePerformanceJob(current, expected) || s.performance.persistenceFault != nil || (current.Status != performanceJobRunning && current.Status != performanceJobPaused) {
		return context.Canceled
	}
	for _, file := range current.Files {
		if file.Path == path && file.ContentHash == contentHash && file.Status == performanceFileRunning {
			return publish()
		}
	}
	return context.Canceled
}

func samePerformanceJob(current, expected *PerformanceJob) bool {
	return current != nil && expected != nil && current.ID == expected.ID && current.Generation == expected.Generation && current.ProjectID == expected.ProjectID && current.ProjectRevision == expected.ProjectRevision && current.Root == expected.Root && current.PolicyFingerprint == expected.PolicyFingerprint && current.QueueID == expected.QueueID
}

func (s *Service) verifyPerformanceJob(job *PerformanceJob) error {
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return err
	}
	if analysis.ProjectID != job.ProjectID || analysis.ProjectRevision != job.ProjectRevision || s.manager.Root() != job.Root || policy.Version() != job.PolicyFingerprint {
		return project.ErrRevisionConflict
	}
	for _, queued := range job.Files {
		found := false
		for _, indexed := range index.Files {
			if indexed.Path == queued.Path && indexed.ContentHash == queued.ContentHash {
				found = true
				break
			}
		}
		if !found {
			return project.ErrRevisionConflict
		}
	}
	return nil
}

func (s *Service) performanceInputs() (*project.Analysis, *project.ProjectIndex, *project.ContextPolicy, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, nil, nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, nil, nil, err
	}
	if index.ProjectID != analysis.ProjectID || index.ProjectRevision != analysis.ProjectRevision {
		return nil, nil, nil, project.ErrRevisionConflict
	}
	policy, err := project.NewContextPolicy(s.manager.Root())
	if err != nil {
		return nil, nil, nil, err
	}
	return analysis, index, policy, nil
}

func (s *Service) performanceCandidates(index *project.ProjectIndex, policy *project.ContextPolicy, limit int) ([]PerformanceJobFile, int, int, int, error) {
	files := make([]PerformanceJobFile, 0, limit)
	excluded, oversized, outsideLimit := 0, 0, 0
	for _, file := range index.Files {
		if file.Binary || !policy.Decide(file.Path).Include {
			excluded++
			continue
		}
		if file.SizeBytes > project.PerformanceMaxSourceBytes {
			oversized++
			continue
		}
		if len(files) == limit {
			outsideLimit++
			continue
		}
		status := performanceFilePending
		if cached, err := project.LoadPerformanceFileReport(s.manager.Root(), file.Path, file.ContentHash, policy); err != nil {
			return nil, 0, 0, 0, err
		} else if cached != nil && cached.Status == "completed" {
			status = performanceFileCached
		}
		files = append(files, PerformanceJobFile{Path: file.Path, ContentHash: file.ContentHash, Status: status})
	}
	return files, excluded, oversized, outsideLimit, nil
}

func normalizePerformanceOptions(options PerformanceJobOptions) PerformanceJobOptions {
	if options.MaxFiles <= 0 {
		options.MaxFiles = defaultPerformanceFileLimit
	}
	if options.MaxFiles > maxPerformanceFileLimit {
		options.MaxFiles = maxPerformanceFileLimit
	}
	if options.RunBudget <= 0 {
		options.RunBudget = defaultPerformanceRunBudget
	}
	if options.RunBudget > maxPerformanceRunBudget {
		options.RunBudget = maxPerformanceRunBudget
	}
	return options
}

func performanceQueueID(projectID, revision, policy string, files []PerformanceJobFile) string {
	hash := sha256.New()
	for _, value := range []string{projectID, revision, policy} {
		_, _ = hash.Write([]byte(value + "\n"))
	}
	for _, file := range files {
		_, _ = hash.Write([]byte(file.Path + "\n" + file.ContentHash + "\n"))
	}
	return "performance:" + hex.EncodeToString(hash.Sum(nil)[:16])
}

func (s *Service) markPerformanceStale(job *PerformanceJob) (*PerformanceJob, error) {
	published, err := s.updatePerformanceJob(job, func(current *PerformanceJob) error {
		current.Status = performanceJobStale
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistPerformanceJob(published); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) invalidatePerformanceJob() {
	s.performance.mu.Lock()
	current := s.performance.job
	if current == nil || (current.Status != performanceJobRunning && current.Status != performanceJobPaused) {
		s.performance.mu.Unlock()
		return
	}
	current.Status = performanceJobStale
	current.UpdatedAt = time.Now().UTC()
	s.accountPerformanceElapsedLocked(current.UpdatedAt)
	cancel := s.performance.cancel
	published := clonePerformanceJob(current)
	s.performance.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	_ = s.persistPerformanceJob(published)
}

// detachPerformanceJobForProjectChange finalizes an active job at its captured
// root, then removes it from the controller before a different project opens.
// Reindex deliberately uses invalidatePerformanceJob so callers can still see
// the stale job for the current project.
func (s *Service) detachPerformanceJobForProjectChange() {
	s.performance.persistMu.Lock()
	defer s.performance.persistMu.Unlock()

	s.performance.mu.Lock()
	current := s.performance.job
	if current == nil {
		s.performance.mu.Unlock()
		return
	}
	cancel := s.performance.cancel
	var published *PerformanceJob
	if activePerformanceJob(current) || s.performance.persistenceFault != nil {
		current.Status = performanceJobStale
		current.UpdatedAt = time.Now().UTC()
		s.accountPerformanceElapsedLocked(current.UpdatedAt)
		published = clonePerformanceJob(current)
	}
	s.performance.job = nil
	s.performance.durableJob = nil
	s.performance.persistenceFault = nil
	s.performance.workerDone = nil
	s.performance.cancel = nil
	s.performance.workerGeneration = ""
	s.performance.requestStarted = time.Time{}
	s.performance.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if published != nil {
		if err := s.storePerformanceJob(published); err != nil {
			s.performance.mu.Lock()
			if s.performance.detachedFaults == nil {
				s.performance.detachedFaults = make(map[string]detachedPerformanceFault)
			}
			s.performance.detachedFaults[s.manager.Root()] = detachedPerformanceFault{job: published, cause: err}
			s.performance.mu.Unlock()
		}
	}
}

func (s *Service) clearPerformanceJob(job *PerformanceJob) {
	s.performance.mu.Lock()
	if s.performance.job == job {
		s.performance.job = nil
		s.performance.durableJob = nil
		s.performance.persistenceFault = nil
		s.performance.requestStarted = time.Time{}
		cancel := s.performance.cancel
		s.performance.cancel = nil
		s.performance.workerGeneration = ""
		s.performance.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	s.performance.mu.Unlock()
}

func (s *Service) accountPerformanceElapsedLocked(now time.Time) {
	if s.performance.requestStarted.IsZero() || s.performance.job == nil {
		return
	}
	accountPersistedPerformanceElapsed(s.performance.job, now)
	s.performance.requestStarted = time.Time{}
}

func accountPersistedPerformanceElapsed(job *PerformanceJob, now time.Time) {
	if job.ActiveStartedAt.IsZero() {
		return
	}
	if elapsed := now.Sub(job.ActiveStartedAt); elapsed > 0 {
		job.Elapsed += elapsed
		if job.Elapsed > job.RunBudget {
			job.Elapsed = job.RunBudget
		}
	}
	job.ActiveStartedAt = time.Time{}
}

// recoverPersistedPerformanceJob returns an interrupted provider request to a
// resumable queue state. A process restart cannot resume an in-flight request,
// so its elapsed time is charged once and its file becomes pending again.
func recoverPersistedPerformanceJob(job *PerformanceJob, now time.Time) bool {
	inFlight := !job.ActiveStartedAt.IsZero()
	for i := range job.Files {
		if job.Files[i].Status != performanceFileRunning {
			continue
		}
		job.Files[i].Status = performanceFilePending
		job.Files[i].Error = ""
		inFlight = true
	}
	if !inFlight && job.Status != performanceJobRunning {
		return false
	}
	accountPersistedPerformanceElapsed(job, now)
	switch job.Status {
	case performanceJobRunning:
		job.Status = performanceJobPaused
	case performanceJobCompleted:
		if inFlight {
			job.Status = performanceJobPaused
		}
	}
	job.UpdatedAt = now
	return true
}

func (s *Service) storePerformanceJob(job *PerformanceJob) error {
	job, err := canonicalPerformanceJob(job)
	if err != nil {
		return err
	}
	previous, err := loadPerformanceJob(job.Root)
	if err != nil {
		return err
	}
	// A stale job is terminal: a delayed worker snapshot must not revive it.
	if samePerformanceJobAtRoot(previous, job, job.Root) && previous.Status == performanceJobStale && job.Status != performanceJobStale {
		return nil
	}
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("encode performance job: %w", err)
	}
	path := filepath.Join(job.Root, ".mini-orca", "sessions", "performance-job.json")
	write := s.writePerformanceJob
	if write == nil {
		write = storage.WriteFile
	}
	if err := write(path, data, 0600); err != nil {
		return fmt.Errorf("store performance job: %w", err)
	}
	return nil
}

func (s *Service) persistPerformanceJob(job *PerformanceJob) error {
	s.performance.persistMu.Lock()
	defer s.performance.persistMu.Unlock()
	return s.persistPerformanceJobSerialized(job, false)
}

func (s *Service) persistPerformanceJobSerialized(requested *PerformanceJob, recoverFault bool) error {
	s.performance.mu.Lock()
	current := s.performance.job
	if !samePerformanceJob(current, requested) {
		s.performance.mu.Unlock()
		return nil // Obsolete and detached workers have no persistence authority.
	}
	if s.performance.persistenceFault != nil && !recoverFault {
		s.performance.mu.Unlock()
		return errPerformancePersistence
	}
	job := clonePerformanceJob(current)
	s.performance.mu.Unlock()
	err := s.storePerformanceJob(job)
	s.performance.mu.Lock()
	defer s.performance.mu.Unlock()
	if s.performance.job != current {
		return nil
	}
	if err != nil {
		// Retain useful internal context without returning private filesystem text.
		s.performance.persistenceFault = err
		if s.performance.cancel != nil {
			s.performance.cancel()
		}
		if current.Status == performanceJobRunning || current.Status == performanceJobCompleted {
			current.Status = performanceJobPaused
		}
		return errPerformancePersistence
	}
	s.performance.durableJob = clonePerformanceJob(job)
	if recoverFault {
		s.performance.persistenceFault = nil
	}
	return nil
}

// Keep a failed admission and its rollback atomic with respect to detachment.
// No request was dispatched, so neither an attempt nor elapsed time is charged.
func (s *Service) persistPerformanceAdmission(job *PerformanceJob, path string) bool {
	s.performance.persistMu.Lock()
	defer s.performance.persistMu.Unlock()
	if err := s.persistPerformanceJobSerialized(job, false); err != nil {
		s.rollbackPerformanceAdmission(job, path)
		return false
	}
	return s.performanceAdmissionCurrent(job)
}

func (s *Service) rollbackPerformanceAdmission(expected *PerformanceJob, path string) {
	s.performance.mu.Lock()
	defer s.performance.mu.Unlock()
	current := s.performance.job
	if !samePerformanceJob(current, expected) {
		return
	}
	for i := range current.Files {
		file := &current.Files[i]
		if file.Path == path && file.Status == performanceFileRunning {
			file.Status = performanceFilePending
			file.Attempts--
			current.Elapsed = expected.Elapsed
			current.ActiveStartedAt = time.Time{}
			s.performance.requestStarted = time.Time{}
			return
		}
	}
}

func (s *Service) performanceAdmissionCurrent(expected *PerformanceJob) bool {
	s.performance.mu.Lock()
	defer s.performance.mu.Unlock()
	return samePerformanceJob(s.performance.job, expected) && s.performance.persistenceFault == nil && activePerformanceJob(s.performance.job)
}

func loadPerformanceJob(root string) (*PerformanceJob, error) {
	root, err := canonicalPerformanceJobRoot(root)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, ".mini-orca", "sessions", "performance-job.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read performance job: %w", err)
	}
	var job PerformanceJob
	if err := json.Unmarshal(data, &job); err != nil || !validPerformanceJob(&job) {
		if recoverErr := storage.RecoverCorrupt(path); recoverErr != nil {
			return nil, recoverErr
		}
		return nil, nil
	}
	jobRoot, err := canonicalPerformanceJobRoot(job.Root)
	if err != nil || jobRoot != root {
		if recoverErr := storage.RecoverCorrupt(path); recoverErr != nil {
			return nil, recoverErr
		}
		return nil, nil
	}
	job.Root = root
	return clonePerformanceJob(&job), nil
}

// canonicalPerformanceJob normalizes the durable storage boundary so a job can
// only be read back from and persisted to its own canonical project root.
func canonicalPerformanceJob(job *PerformanceJob) (*PerformanceJob, error) {
	if job == nil {
		return nil, fmt.Errorf("performance job is required")
	}
	root, err := canonicalPerformanceJobRoot(job.Root)
	if err != nil {
		return nil, err
	}
	copy := clonePerformanceJob(job)
	copy.Root = root
	return copy, nil
}

func canonicalPerformanceJobRoot(root string) (string, error) {
	canonical, err := project.CanonicalRoot(root)
	if err != nil {
		return "", fmt.Errorf("canonicalize performance job root: %w", err)
	}
	return canonical, nil
}

func samePerformanceJobAtRoot(current, expected *PerformanceJob, root string) bool {
	return current != nil && expected != nil && current.Root == root && expected.Root == root && samePerformanceJob(current, expected)
}

func validPerformanceJob(job *PerformanceJob) bool {
	return validPerformanceJobIdentity(job) && validPerformanceJobBudget(job) && validPerformanceJobFiles(job)
}

func validPerformanceJobIdentity(job *PerformanceJob) bool {
	return job != nil && job.ID != "" && job.Generation != "" && job.ProjectID != "" && job.ProjectRevision != "" && job.Root != "" && job.PolicyFingerprint != "" && job.QueueID != "" && validPerformanceJobState(job.Status)
}

func validPerformanceJobBudget(job *PerformanceJob) bool {
	return job.MaxFiles >= 1 && job.MaxFiles <= maxPerformanceFileLimit && job.RunBudget > 0 && job.RunBudget <= maxPerformanceRunBudget && job.Elapsed >= 0 && job.Elapsed <= job.RunBudget
}

func validPerformanceJobFiles(job *PerformanceJob) bool {
	if len(job.Files) > job.MaxFiles {
		return false
	}
	for _, file := range job.Files {
		if file.Path == "" || file.ContentHash == "" || file.Attempts < 0 || !validPerformanceFileState(file.Status) {
			return false
		}
	}
	return true
}

func validPerformanceJobState(state string) bool {
	switch state {
	case performanceJobRunning, performanceJobPaused, performanceJobCanceled, performanceJobCompleted, performanceJobStale:
		return true
	default:
		return false
	}
}

func validPerformanceFileState(state string) bool {
	switch state {
	case performanceFilePending, performanceFileRunning, performanceFileCompleted, performanceFileCached, performanceFileFailed, performanceFileSkipped:
		return true
	default:
		return false
	}
}

func clonePerformanceJob(source *PerformanceJob) *PerformanceJob {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Files = append([]PerformanceJobFile(nil), source.Files...)
	return &copy
}
