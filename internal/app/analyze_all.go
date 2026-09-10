package app

import (
	"context"
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
	analysisAllStateRunning   = "running"
	analysisAllStatePaused    = "paused"
	analysisAllStateCanceled  = "canceled"
	analysisAllStateCompleted = "completed"
	analysisAllStateStale     = "stale"

	analysisAllFilePending   = "pending"
	analysisAllFileRunning   = "running"
	analysisAllFileCompleted = "completed"
	analysisAllFileFailed    = "failed"
	analysisAllTimeoutError  = "analysis timed out"

	defaultAnalyzeAllFileLimit = 100
	maxAnalyzeAllFileLimit     = 500
	defaultAnalyzeAllRetries   = 1
	maxAnalyzeAllRetries       = 3
)

// AnalyzeAllOptions constrains an explicit, project-scoped cache-warming job.
type AnalyzeAllOptions struct {
	MaxFiles   int `json:"max_files,omitempty"`
	MaxRetries int `json:"max_retries,omitempty"`
}

// AnalyzeAllJob is source-free durable progress for sequential semantic analysis.
type AnalyzeAllJob struct {
	ProjectID       string              `json:"project_id"`
	ProjectRevision string              `json:"project_revision"`
	Status          string              `json:"status"`
	MaxFiles        int                 `json:"max_files"`
	MaxRetries      int                 `json:"max_retries"`
	Files           []AnalyzeAllFileJob `json:"files"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	root            string
}

// AnalyzeAllFileJob records one eligible file without source or prompt content.
type AnalyzeAllFileJob struct {
	Path     string `json:"path"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
	Error    string `json:"error,omitempty"`
}

var errAnalyzeAllPersistence = errors.New("analyze-all progress could not be saved; retry pause, cancel, or resume to recover")

type analysisAllController struct {
	mu               sync.Mutex
	job              *AnalyzeAllJob
	cancel           context.CancelFunc
	workerDone       chan struct{}
	persisted        chan struct{}
	persistenceFault error
}

type analyzeAllStartInput struct {
	analysis *project.Analysis
	index    *project.ProjectIndex
	options  AnalyzeAllOptions
	root     string
}

func newAnalysisAllController() *analysisAllController { return &analysisAllController{} }

func (c *analysisAllController) beginPersist() func() {
	for {
		c.mu.Lock()
		if c.persisted == nil {
			finished := make(chan struct{})
			c.persisted = finished
			c.mu.Unlock()
			return func() {
				c.mu.Lock()
				if c.persisted == finished {
					c.persisted = nil
					close(finished)
				}
				c.mu.Unlock()
			}
		}
		previous := c.persisted
		c.mu.Unlock()
		<-previous
	}
}

// StartAnalyzeAll explicitly starts a bounded, sequential cache-warming job.
// Importing a project never calls this method.
func (s *Service) StartAnalyzeAll(_ context.Context, options AnalyzeAllOptions, confirmRemoteProvider bool) (*AnalyzeAllJob, error) {
	input, err := s.prepareAnalyzeAllStart(options, confirmRemoteProvider)
	if err != nil {
		return nil, err
	}
	files, err := s.selectAnalyzeAllCandidates(input.analysis, input.index, input.options.MaxFiles)
	if err != nil {
		return nil, err
	}
	return s.activateAnalyzeAll(input, files)
}

func (s *Service) prepareAnalyzeAllStart(options AnalyzeAllOptions, confirmRemoteProvider bool) (analyzeAllStartInput, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return analyzeAllStartInput{}, err
	}
	if active, _ := s.PerformanceJob(); active != nil && (active.Status == performanceJobRunning || active.Status == performanceJobPaused) {
		return analyzeAllStartInput{}, fmt.Errorf("performance review is already active")
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return analyzeAllStartInput{}, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return analyzeAllStartInput{}, err
	}
	if index.ProjectID != analysis.ProjectID || index.ProjectRevision != analysis.ProjectRevision {
		return analyzeAllStartInput{}, project.ErrRevisionConflict
	}
	return analyzeAllStartInput{analysis: analysis, index: index, options: normalizeAnalyzeAllOptions(options), root: s.manager.Root()}, nil
}

func (s *Service) activateAnalyzeAll(input analyzeAllStartInput, files []AnalyzeAllFileJob) (*AnalyzeAllJob, error) {
	s.jobLifecycleMu.Lock()
	finished := s.analysisAll.beginPersist()
	defer finished()
	now := time.Now().UTC()
	job := &AnalyzeAllJob{ProjectID: input.analysis.ProjectID, ProjectRevision: input.analysis.ProjectRevision, Status: analysisAllStateRunning, MaxFiles: input.options.MaxFiles, MaxRetries: input.options.MaxRetries, Files: files, CreatedAt: now, UpdatedAt: now, root: input.root}
	s.analysisAll.mu.Lock()
	if err := s.verifyAnalyzeAllStart(input); err != nil {
		s.analysisAll.mu.Unlock()
		s.jobLifecycleMu.Unlock()
		return nil, err
	}
	if current := s.analysisAll.job; current != nil && current.ProjectID == job.ProjectID && current.ProjectRevision == job.ProjectRevision && (current.Status == analysisAllStateRunning || current.Status == analysisAllStatePaused) {
		if s.analysisAll.persistenceFault != nil {
			s.analysisAll.mu.Unlock()
			s.jobLifecycleMu.Unlock()
			return nil, errAnalyzeAllPersistence
		}
		existing := cloneAnalyzeAllJob(current)
		s.analysisAll.mu.Unlock()
		s.jobLifecycleMu.Unlock()
		return existing, nil
	}
	s.performance.mu.Lock()
	if activePerformanceJob(s.performance.job) {
		s.performance.mu.Unlock()
		s.analysisAll.mu.Unlock()
		s.jobLifecycleMu.Unlock()
		return nil, fmt.Errorf("performance review is already active")
	}
	oldCancel := s.analysisAll.cancel
	s.analysisAll.cancel = nil
	s.analysisAll.job = job
	s.analysisAll.persistenceFault = nil
	s.performance.mu.Unlock()
	s.analysisAll.mu.Unlock()
	if oldCancel != nil {
		oldCancel()
	}
	if err := s.persistAnalyzeAllJobSerialized(job, false); err != nil {
		s.clearAnalyzeAllJob(job)
		s.jobLifecycleMu.Unlock()
		return nil, err
	}
	published := cloneAnalyzeAllJob(job)
	s.jobLifecycleMu.Unlock()
	s.startAnalyzeAllWorker()
	return published, nil
}

func (s *Service) verifyAnalyzeAllStart(input analyzeAllStartInput) error {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return err
	}
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if s.manager.Root() != input.root || analysis.ProjectID != input.analysis.ProjectID || analysis.ProjectRevision != input.analysis.ProjectRevision || index.ProjectID != input.index.ProjectID || index.ProjectRevision != input.index.ProjectRevision {
		return project.ErrRevisionConflict
	}
	return nil
}

// AnalyzeAllJob returns durable progress for the active project, if any.
func (s *Service) AnalyzeAllJob() (*AnalyzeAllJob, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	finished := s.analysisAll.beginPersist()
	defer finished()
	s.analysisAll.mu.Lock()
	current := cloneAnalyzeAllJob(s.analysisAll.job)
	fault := s.analysisAll.persistenceFault
	s.analysisAll.mu.Unlock()
	if fault != nil {
		return nil, errAnalyzeAllPersistence
	}
	if current != nil {
		if current.ProjectID == analysis.ProjectID && current.ProjectRevision == analysis.ProjectRevision {
			return current, nil
		}
		return s.markAnalyzeAllStaleSerialized(current)
	}
	job, err := loadAnalyzeAllJob(s.manager.Root())
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	if job.ProjectID != analysis.ProjectID || job.ProjectRevision != analysis.ProjectRevision {
		job.Status = analysisAllStateStale
		job.UpdatedAt = time.Now().UTC()
		if err := s.persistAnalyzeAllJobSerialized(job, false); err != nil {
			return nil, err
		}
	}
	s.analysisAll.mu.Lock()
	if s.analysisAll.job != nil {
		current = cloneAnalyzeAllJob(s.analysisAll.job)
		s.analysisAll.mu.Unlock()
		return current, nil
	}
	s.analysisAll.job = cloneAnalyzeAllJob(job)
	s.analysisAll.mu.Unlock()
	return cloneAnalyzeAllJob(job), nil
}

// PauseAnalyzeAll prevents the next file from starting. An in-flight request is
// allowed to finish so its valid cache entry is retained.
func (s *Service) PauseAnalyzeAll() (*AnalyzeAllJob, error) {
	return s.changeAnalyzeAllState(analysisAllStatePaused, false, false)
}

// CancelAnalyzeAll cancels the active request and leaves completed cache entries intact.
func (s *Service) CancelAnalyzeAll() (*AnalyzeAllJob, error) {
	return s.changeAnalyzeAllState(analysisAllStateCanceled, true, false)
}

// ResumeAnalyzeAll resumes a paused persisted job for the active revision.
func (s *Service) ResumeAnalyzeAll(_ context.Context, confirmRemoteProvider bool) (*AnalyzeAllJob, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	finished := s.beginAnalyzeAllRecovery()
	defer finished()
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	s.analysisAll.mu.Lock()
	job := cloneAnalyzeAllJob(s.analysisAll.job)
	s.analysisAll.mu.Unlock()
	if job == nil {
		job, err = loadAnalyzeAllJob(s.manager.Root())
		if err != nil {
			return nil, err
		}
		if job == nil {
			return nil, fmt.Errorf("analyze-all job not found")
		}
		s.analysisAll.mu.Lock()
		if s.analysisAll.job == nil {
			s.analysisAll.job = cloneAnalyzeAllJob(job)
		} else {
			job = cloneAnalyzeAllJob(s.analysisAll.job)
		}
		s.analysisAll.mu.Unlock()
	}
	if job.ProjectID != analysis.ProjectID || job.ProjectRevision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	if err := s.verifyAnalyzeAllRevision(job); err != nil {
		return nil, err
	}
	published, _, err := s.updateCurrentAnalyzeAllJob(job, func(current *AnalyzeAllJob) error {
		return resumeAnalyzeAllState(current, s.analysisAll.cancel != nil)
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistAnalyzeAllJobSerialized(published, true); err != nil {
		return nil, err
	}
	s.startAnalyzeAllWorker()
	return published, nil
}

func resumeAnalyzeAllState(current *AnalyzeAllJob, workerActive bool) error {
	if current.Status != analysisAllStatePaused && !(current.Status == analysisAllStateRunning && !workerActive) {
		return fmt.Errorf("analyze-all job is not paused")
	}
	// A persisted running file may have been interrupted by a daemon restart.
	// Keep its charged attempts; only explicit resume makes it eligible again.
	if !workerActive {
		for i := range current.Files {
			if current.Files[i].Status == analysisAllFileRunning {
				current.Files[i].Status = analysisAllFileFailed
				current.Files[i].Error = "analysis interrupted"
			}
		}
	}
	current.Status = analysisAllStateRunning
	current.UpdatedAt = time.Now().UTC()
	return nil
}

// Reindex invalidates an active job before deterministic facts are refreshed.
func (s *Service) Reindex() (*project.ProjectIndex, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.reindexActiveProject()
}

func (s *Service) reindexActiveProject() (*project.ProjectIndex, error) {
	s.invalidateAnalyzeAll()
	s.invalidatePerformanceJob()
	s.cancelGoScan()
	index, err := s.manager.Reindex()
	if err == nil {
		s.clearExecutionTrust()
		s.ExpireDraftsForOpenFile(index.ProjectID, index.ProjectRevision, "", "")
	}
	return index, err
}

// ActivateProject replaces the active project after model analysis has completed.
func (s *Service) ActivateProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error { return s.manager.Set(root, analysis) })
}

// RestoreActiveProject replaces the active project from persisted local analysis.
func (s *Service) RestoreActiveProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error { return s.manager.Restore(root, analysis) })
}

func (s *Service) replaceActiveProject(activate func() error) error {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.invalidateJobsForProjectChange()
	return activate()
}

func (s *Service) invalidateJobsForProjectChange() {
	s.invalidateAnalyzeAll()
	s.detachPerformanceJobForProjectChange()
	s.cancelGoScan()
	s.clearDraftsForProjectChange()
	s.clearChatSessions()
	s.clearExecutionTrust()
}

func (s *Service) cancelGoScan() {
	s.goScan.mu.Lock()
	cancel := s.goScan.cancel
	s.goScan.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Recovery must not race the faulted worker's exit or result publication.
func (s *Service) beginAnalyzeAllRecovery() func() {
	for {
		finished := s.analysisAll.beginPersist()
		s.analysisAll.mu.Lock()
		done := s.analysisAll.workerDone
		wait := s.analysisAll.persistenceFault != nil && done != nil
		s.analysisAll.mu.Unlock()
		if !wait {
			return finished
		}
		finished()
		<-done
	}
}

func (s *Service) changeAnalyzeAllState(next string, cancelWorker, allowCompleted bool) (*AnalyzeAllJob, error) {
	finished := s.beginAnalyzeAllRecovery()
	defer finished()
	if _, err := s.manager.Analysis(); err != nil {
		return nil, err
	}
	s.analysisAll.mu.Lock()
	job := cloneAnalyzeAllJob(s.analysisAll.job)
	s.analysisAll.mu.Unlock()
	if job == nil {
		return nil, fmt.Errorf("analyze-all job not found")
	}
	if err := s.verifyAnalyzeAllRevision(job); err != nil {
		return nil, err
	}
	published, cancel, err := s.updateCurrentAnalyzeAllJob(job, func(current *AnalyzeAllJob) error {
		if !allowCompleted && !(s.analysisAll.persistenceFault != nil && current.Status == next) && (current.Status == analysisAllStateCompleted || current.Status == analysisAllStateCanceled || current.Status == analysisAllStateStale) {
			return fmt.Errorf("analyze-all job cannot be %s from %s", next, job.Status)
		}
		current.Status = next
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cancelWorker && cancel != nil {
		cancel()
	}
	if err := s.persistAnalyzeAllJobSerialized(published, true); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) updateCurrentAnalyzeAllJob(job *AnalyzeAllJob, update func(*AnalyzeAllJob) error) (*AnalyzeAllJob, context.CancelFunc, error) {
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	current := s.analysisAll.job
	if !sameAnalyzeAllJobIdentity(current, job) {
		return nil, nil, project.ErrRevisionConflict
	}
	if err := update(current); err != nil {
		return nil, nil, err
	}
	return cloneAnalyzeAllJob(current), s.analysisAll.cancel, nil
}

func (s *Service) invalidateAnalyzeAll() {
	finished := s.analysisAll.beginPersist()
	defer finished()
	s.analysisAll.mu.Lock()
	if s.analysisAll.job == nil || s.analysisAll.job.Status != analysisAllStateRunning && s.analysisAll.job.Status != analysisAllStatePaused {
		s.analysisAll.mu.Unlock()
		return
	}
	s.analysisAll.job.Status = analysisAllStateStale
	s.analysisAll.job.UpdatedAt = time.Now().UTC()
	cancel := s.analysisAll.cancel
	published := cloneAnalyzeAllJob(s.analysisAll.job)
	s.analysisAll.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	// The controller retains any failure for the progress boundary.
	_ = s.persistAnalyzeAllJobSerialized(published, false)
}

func (s *Service) markAnalyzeAllStaleSerialized(job *AnalyzeAllJob) (*AnalyzeAllJob, error) {
	s.analysisAll.mu.Lock()
	current := s.analysisAll.job
	if !sameAnalyzeAllJobIdentity(current, job) {
		s.analysisAll.mu.Unlock()
		return nil, project.ErrRevisionConflict
	}
	if current.Status == analysisAllStateStale {
		published := cloneAnalyzeAllJob(current)
		s.analysisAll.mu.Unlock()
		return published, nil
	}
	current.Status = analysisAllStateStale
	current.UpdatedAt = time.Now().UTC()
	cancel := s.analysisAll.cancel
	published := cloneAnalyzeAllJob(current)
	s.analysisAll.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := s.persistAnalyzeAllJobSerialized(published, false); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) selectAnalyzeAllCandidates(analysis *project.Analysis, index *project.ProjectIndex, limit int) ([]AnalyzeAllFileJob, error) {
	files := make([]AnalyzeAllFileJob, 0, limit)
	for _, file := range index.Files {
		if len(files) == limit {
			break
		}
		if !isSemanticAnalysisCandidate(file) {
			continue
		}
		cache, input, err := s.fileAnalysisCacheInput(analysis, &file, file.ContentHash)
		if err != nil {
			return nil, err
		}
		cached, err := cache.Load(input)
		if err != nil {
			return nil, err
		}
		if err := s.syncFileAnalysisStatus(input, cached.Status); err != nil {
			return nil, err
		}
		if cached.Status == project.AnalysisStatusMissing || cached.Status == project.AnalysisStatusStale || cached.Status == project.AnalysisStatusFailed {
			files = append(files, AnalyzeAllFileJob{Path: file.Path, Status: analysisAllFilePending})
		}
	}
	return files, nil
}

func isSemanticAnalysisCandidate(file project.IndexFile) bool {
	return !file.Binary && file.Language != "Text" && file.Language != "Markdown"
}

func (s *Service) startAnalyzeAllWorker() {
	s.analysisAll.mu.Lock()
	if s.analysisAll.cancel != nil || s.analysisAll.persistenceFault != nil || s.analysisAll.job == nil || s.analysisAll.job.Status != analysisAllStateRunning {
		s.analysisAll.mu.Unlock()
		return
	}
	job := s.analysisAll.job
	worker, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.analysisAll.cancel = cancel
	s.analysisAll.workerDone = done
	s.analysisAll.mu.Unlock()
	go s.runAnalyzeAll(worker, job, done)
}

func (s *Service) clearAnalyzeAllJob(job *AnalyzeAllJob) {
	s.analysisAll.mu.Lock()
	if s.analysisAll.job == job {
		cancel := s.analysisAll.cancel
		s.analysisAll.job = nil
		s.analysisAll.persistenceFault = nil
		s.analysisAll.cancel = nil
		s.analysisAll.workerDone = nil
		s.analysisAll.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	s.analysisAll.mu.Unlock()
}

func (s *Service) runAnalyzeAll(ctx context.Context, job *AnalyzeAllJob, done chan struct{}) {
	defer func() {
		s.analysisAll.mu.Lock()
		if s.analysisAll.job == job {
			s.analysisAll.cancel = nil
			if s.analysisAll.workerDone == done {
				s.analysisAll.workerDone = nil
			}
		}
		s.analysisAll.mu.Unlock()
		close(done)
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		file, ok := s.nextAnalyzeAllFile(job)
		if !ok {
			return
		}
		result, err := s.AnalyzeFile(ctx, file, true, true)
		if err == nil && result.Status == project.AnalysisStatusFailed {
			err = fmt.Errorf("semantic analysis failed")
		}
		if !s.recordAnalyzeAllResult(job, file, err) {
			return
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (s *Service) nextAnalyzeAllFile(expected *AnalyzeAllJob) (string, bool) {
	finished := s.analysisAll.beginPersist()
	defer finished()
	s.analysisAll.mu.Lock()
	if s.analysisAll.job != expected || s.analysisAll.persistenceFault != nil {
		s.analysisAll.mu.Unlock()
		return "", false
	}
	job := cloneAnalyzeAllJob(expected)
	s.analysisAll.mu.Unlock()
	if job == nil || job.Status != analysisAllStateRunning {
		return "", false
	}
	if err := s.verifyAnalyzeAllRevision(job); err != nil {
		_, _ = s.markAnalyzeAllWorkerStale(expected)
		return "", false
	}
	s.analysisAll.mu.Lock()
	current := s.analysisAll.job
	if current != expected || current.Status != analysisAllStateRunning {
		s.analysisAll.mu.Unlock()
		return "", false
	}
	for i := range current.Files {
		if current.Files[i].Status == analysisAllFilePending || shouldRetryAnalyzeAllFile(current.Files[i], current.MaxRetries) {
			previous := current.Files[i]
			current.Files[i].Status = analysisAllFileRunning
			current.Files[i].Attempts++
			current.Files[i].Error = ""
			current.UpdatedAt = time.Now().UTC()
			path := current.Files[i].Path
			published := cloneAnalyzeAllJob(current)
			s.analysisAll.mu.Unlock()
			if err := s.persistAnalyzeAllJobSerialized(published, false); err != nil {
				s.analysisAll.mu.Lock()
				if s.analysisAll.job == expected {
					// No provider request was dispatched for this failed admission.
					expected.Files[i] = previous
				}
				s.analysisAll.mu.Unlock()
				return "", false
			}
			return path, true
		}
	}
	current.Status = analysisAllStateCompleted
	current.UpdatedAt = time.Now().UTC()
	published := cloneAnalyzeAllJob(current)
	s.analysisAll.mu.Unlock()
	// A failed completion remains a paused, faulted controller state.
	_ = s.persistAnalyzeAllJobSerialized(published, false)
	return "", false
}

func shouldRetryAnalyzeAllFile(file AnalyzeAllFileJob, maxRetries int) bool {
	return file.Status == analysisAllFileFailed && file.Error != analysisAllTimeoutError && file.Attempts <= maxRetries
}

func (s *Service) recordAnalyzeAllResult(expected *AnalyzeAllJob, path string, cause error) bool {
	finished := s.analysisAll.beginPersist()
	defer finished()
	s.analysisAll.mu.Lock()
	job := s.analysisAll.job
	if job != expected || job.Status == analysisAllStateCanceled || job.Status == analysisAllStateStale {
		s.analysisAll.mu.Unlock()
		return false
	}
	for i := range job.Files {
		if job.Files[i].Path != path {
			continue
		}
		if cause == nil {
			job.Files[i].Status = analysisAllFileCompleted
		} else if errors.Is(cause, context.DeadlineExceeded) {
			job.Files[i].Status = analysisAllFileFailed
			job.Files[i].Error = analysisAllTimeoutError
		} else if errors.Is(cause, context.Canceled) {
			job.Files[i].Status = analysisAllFileFailed
			job.Files[i].Error = "analysis canceled"
		} else {
			job.Files[i].Status = analysisAllFileFailed
			job.Files[i].Error = "analysis failed"
		}
		job.UpdatedAt = time.Now().UTC()
		published := cloneAnalyzeAllJob(job)
		s.analysisAll.mu.Unlock()
		return s.persistAnalyzeAllJobSerialized(published, false) == nil
	}
	s.analysisAll.mu.Unlock()
	return false
}

func (s *Service) markAnalyzeAllWorkerStale(expected *AnalyzeAllJob) (*AnalyzeAllJob, error) {
	s.analysisAll.mu.Lock()
	if s.analysisAll.job != expected || expected.Status != analysisAllStateRunning {
		s.analysisAll.mu.Unlock()
		return nil, project.ErrRevisionConflict
	}
	expected.Status = analysisAllStateStale
	expected.UpdatedAt = time.Now().UTC()
	cancel := s.analysisAll.cancel
	published := cloneAnalyzeAllJob(expected)
	s.analysisAll.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := s.persistAnalyzeAllJobSerialized(published, false); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) verifyAnalyzeAllRevision(job *AnalyzeAllJob) error {
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if index.ProjectID != job.ProjectID || index.ProjectRevision != job.ProjectRevision {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) storeAnalyzeAllJob(job *AnalyzeAllJob) error {
	path := filepath.Join(job.root, ".mini-orca", "sessions", "analyze-all.json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("encode analyze-all job: %w", err)
	}
	write := s.writeAnalyzeAllJob
	if write == nil {
		write = storage.WriteFile
	}
	if err := write(path, data, 0600); err != nil {
		return fmt.Errorf("store analyze-all job: %w", err)
	}
	return nil
}

// Callers hold beginPersist across state changes and their writes, so no worker
// can dispatch using an admission superseded by a failed or concurrent update.
func (s *Service) persistAnalyzeAllJobSerialized(requested *AnalyzeAllJob, recoverFault bool) error {
	if requested == nil {
		return fmt.Errorf("analyze-all job is required")
	}
	s.analysisAll.mu.Lock()
	current := s.analysisAll.job
	job := cloneAnalyzeAllJob(requested)
	if current != nil {
		if !sameAnalyzeAllJobIdentity(current, requested) {
			s.analysisAll.mu.Unlock()
			return project.ErrRevisionConflict
		}
		job = cloneAnalyzeAllJob(current)
	}
	if s.analysisAll.persistenceFault != nil && !recoverFault {
		s.analysisAll.mu.Unlock()
		return errAnalyzeAllPersistence
	}
	s.analysisAll.mu.Unlock()
	err := s.storeAnalyzeAllJob(job)
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	if s.analysisAll.job != current {
		return project.ErrRevisionConflict
	}
	if err != nil {
		// Preserve the underlying storage cause internally without exposing paths
		// or arbitrary filesystem error text through the API.
		s.analysisAll.persistenceFault = err
		if s.analysisAll.cancel != nil {
			s.analysisAll.cancel()
		}
		if current != nil && (current.Status == analysisAllStateRunning || current.Status == analysisAllStateCompleted) {
			current.Status = analysisAllStatePaused
		}
		return errAnalyzeAllPersistence
	}
	if recoverFault {
		s.analysisAll.persistenceFault = nil
	}
	return nil
}

func sameAnalyzeAllJobIdentity(current, requested *AnalyzeAllJob) bool {
	return current != nil && requested != nil && current.root == requested.root && current.ProjectID == requested.ProjectID && current.ProjectRevision == requested.ProjectRevision && current.CreatedAt.Equal(requested.CreatedAt)
}

func loadAnalyzeAllJob(root string) (*AnalyzeAllJob, error) {
	path := filepath.Join(root, ".mini-orca", "sessions", "analyze-all.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read analyze-all job: %w", err)
	}
	var job AnalyzeAllJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parse analyze-all job: %w", err)
	}
	if job.ProjectID == "" || job.ProjectRevision == "" || !validAnalyzeAllState(job.Status) {
		return nil, fmt.Errorf("analyze-all job is invalid")
	}
	job.root = root
	return &job, nil
}

func normalizeAnalyzeAllOptions(options AnalyzeAllOptions) AnalyzeAllOptions {
	if options.MaxFiles <= 0 {
		options.MaxFiles = defaultAnalyzeAllFileLimit
	}
	if options.MaxFiles > maxAnalyzeAllFileLimit {
		options.MaxFiles = maxAnalyzeAllFileLimit
	}
	if options.MaxRetries < 0 {
		options.MaxRetries = 0
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = defaultAnalyzeAllRetries
	}
	if options.MaxRetries > maxAnalyzeAllRetries {
		options.MaxRetries = maxAnalyzeAllRetries
	}
	return options
}

func validAnalyzeAllState(state string) bool {
	switch state {
	case analysisAllStateRunning, analysisAllStatePaused, analysisAllStateCanceled, analysisAllStateCompleted, analysisAllStateStale:
		return true
	default:
		return false
	}
}

func cloneAnalyzeAllJob(job *AnalyzeAllJob) *AnalyzeAllJob {
	if job == nil {
		return nil
	}
	copy := *job
	copy.Files = append([]AnalyzeAllFileJob(nil), job.Files...)
	return &copy
}
