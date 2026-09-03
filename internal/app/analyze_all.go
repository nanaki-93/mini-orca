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
}

// AnalyzeAllFileJob records one eligible file without source or prompt content.
type AnalyzeAllFileJob struct {
	Path     string `json:"path"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
	Error    string `json:"error,omitempty"`
}

type analysisAllController struct {
	mu        sync.Mutex
	job       *AnalyzeAllJob
	cancel    context.CancelFunc
	persisted chan struct{}
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

func (c *analysisAllController) waitForPersists() {
	for {
		c.mu.Lock()
		persisted := c.persisted
		c.mu.Unlock()
		if persisted == nil {
			return
		}
		<-persisted
	}
}

// StartAnalyzeAll explicitly starts a bounded, sequential cache-warming job.
// Importing a project never calls this method.
func (s *Service) StartAnalyzeAll(_ context.Context, options AnalyzeAllOptions, confirmRemoteProvider bool) (*AnalyzeAllJob, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != analysis.ProjectID || index.ProjectRevision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	options = normalizeAnalyzeAllOptions(options)
	files, err := s.analysisAllCandidates(index, options.MaxFiles)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	job := &AnalyzeAllJob{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Status: analysisAllStateRunning, MaxFiles: options.MaxFiles, MaxRetries: options.MaxRetries, Files: files, CreatedAt: now, UpdatedAt: now}
	s.analysisAll.mu.Lock()
	if current := s.analysisAll.job; current != nil && current.ProjectID == analysis.ProjectID && current.ProjectRevision == analysis.ProjectRevision && (current.Status == analysisAllStateRunning || current.Status == analysisAllStatePaused) {
		existing := cloneAnalyzeAllJob(current)
		s.analysisAll.mu.Unlock()
		return existing, nil
	}
	s.analysisAll.job = job
	s.analysisAll.mu.Unlock()
	if err := s.persistAnalyzeAllJob(job); err != nil {
		s.clearAnalyzeAllJob(job)
		return nil, err
	}
	published := cloneAnalyzeAllJob(job)
	s.startAnalyzeAllWorker()
	return published, nil
}

// AnalyzeAllJob returns durable progress for the active project, if any.
func (s *Service) AnalyzeAllJob() (*AnalyzeAllJob, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	s.analysisAll.waitForPersists()
	s.analysisAll.mu.Lock()
	current := cloneAnalyzeAllJob(s.analysisAll.job)
	s.analysisAll.mu.Unlock()
	if current != nil {
		if current.ProjectID == analysis.ProjectID && current.ProjectRevision == analysis.ProjectRevision {
			return current, nil
		}
		return s.markAnalyzeAllStale(current)
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
		if err := s.persistAnalyzeAllJob(job); err != nil {
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
		if current.Status != analysisAllStatePaused {
			return fmt.Errorf("analyze-all job is not paused")
		}
		current.Status = analysisAllStateRunning
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistAnalyzeAllJob(published); err != nil {
		return nil, err
	}
	s.startAnalyzeAllWorker()
	return published, nil
}

// Reindex invalidates an active job before deterministic facts are refreshed.
func (s *Service) Reindex() (*project.ProjectIndex, error) {
	s.invalidateAnalyzeAll()
	s.cancelGoScan()
	index, err := s.manager.Reindex()
	if err == nil {
		s.ExpireDraftsForOpenFile(index.ProjectID, index.ProjectRevision, "", "")
	}
	return index, err
}

// ProjectChanged stops a job before an imported project replaces the active project.
func (s *Service) ProjectChanged() {
	s.invalidateAnalyzeAll()
	s.cancelGoScan()
	s.clearDraftsForProjectChange()
	s.clearChatSessions()
}

func (s *Service) cancelGoScan() {
	s.goScan.mu.Lock()
	cancel := s.goScan.cancel
	s.goScan.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) changeAnalyzeAllState(next string, cancelWorker, allowCompleted bool) (*AnalyzeAllJob, error) {
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
		if !allowCompleted && (current.Status == analysisAllStateCompleted || current.Status == analysisAllStateCanceled || current.Status == analysisAllStateStale) {
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
	if err := s.persistAnalyzeAllJob(published); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) updateCurrentAnalyzeAllJob(job *AnalyzeAllJob, update func(*AnalyzeAllJob) error) (*AnalyzeAllJob, context.CancelFunc, error) {
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	current := s.analysisAll.job
	if current == nil || current.ProjectID != job.ProjectID || current.ProjectRevision != job.ProjectRevision {
		return nil, nil, project.ErrRevisionConflict
	}
	if err := update(current); err != nil {
		return nil, nil, err
	}
	return cloneAnalyzeAllJob(current), s.analysisAll.cancel, nil
}

func (s *Service) invalidateAnalyzeAll() {
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
	_ = s.persistAnalyzeAllJob(published)
}

func (s *Service) markAnalyzeAllStale(job *AnalyzeAllJob) (*AnalyzeAllJob, error) {
	s.analysisAll.mu.Lock()
	current := s.analysisAll.job
	if current == nil || current.ProjectID != job.ProjectID || current.ProjectRevision != job.ProjectRevision {
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
	if err := s.persistAnalyzeAllJob(published); err != nil {
		return nil, err
	}
	return published, nil
}

func (s *Service) analysisAllCandidates(index *project.ProjectIndex, limit int) ([]AnalyzeAllFileJob, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	files := make([]AnalyzeAllFileJob, 0, limit)
	for _, file := range index.Files {
		if len(files) == limit {
			break
		}
		if file.Binary {
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

func (s *Service) startAnalyzeAllWorker() {
	s.analysisAll.mu.Lock()
	if s.analysisAll.cancel != nil || s.analysisAll.job == nil || s.analysisAll.job.Status != analysisAllStateRunning {
		s.analysisAll.mu.Unlock()
		return
	}
	worker, cancel := context.WithCancel(context.Background())
	s.analysisAll.cancel = cancel
	s.analysisAll.mu.Unlock()
	go s.runAnalyzeAll(worker)
}

func (s *Service) clearAnalyzeAllJob(job *AnalyzeAllJob) {
	s.analysisAll.mu.Lock()
	if s.analysisAll.job == job {
		cancel := s.analysisAll.cancel
		s.analysisAll.job = nil
		s.analysisAll.cancel = nil
		s.analysisAll.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	s.analysisAll.mu.Unlock()
}

func (s *Service) runAnalyzeAll(ctx context.Context) {
	defer func() {
		s.analysisAll.mu.Lock()
		s.analysisAll.cancel = nil
		s.analysisAll.mu.Unlock()
	}()
	for {
		file, ok := s.nextAnalyzeAllFile()
		if !ok {
			return
		}
		result, err := s.AnalyzeFile(ctx, file, true, true)
		if err == nil && result.Status == project.AnalysisStatusFailed {
			err = fmt.Errorf("semantic analysis failed")
		}
		s.recordAnalyzeAllResult(file, err)
		if ctx.Err() != nil {
			return
		}
		_ = result
	}
}

func (s *Service) nextAnalyzeAllFile() (string, bool) {
	s.analysisAll.mu.Lock()
	job := cloneAnalyzeAllJob(s.analysisAll.job)
	s.analysisAll.mu.Unlock()
	if job == nil || job.Status != analysisAllStateRunning {
		return "", false
	}
	if err := s.verifyAnalyzeAllRevision(job); err != nil {
		_, _ = s.markAnalyzeAllStale(job)
		return "", false
	}
	s.analysisAll.mu.Lock()
	current := s.analysisAll.job
	if current == nil || current.ProjectID != job.ProjectID || current.ProjectRevision != job.ProjectRevision || current.Status != analysisAllStateRunning {
		s.analysisAll.mu.Unlock()
		return "", false
	}
	for i := range current.Files {
		if current.Files[i].Status == analysisAllFilePending || current.Files[i].Status == analysisAllFileFailed && current.Files[i].Attempts <= current.MaxRetries {
			current.Files[i].Status = analysisAllFileRunning
			current.Files[i].Attempts++
			current.Files[i].Error = ""
			current.UpdatedAt = time.Now().UTC()
			path := current.Files[i].Path
			published := cloneAnalyzeAllJob(current)
			s.analysisAll.mu.Unlock()
			_ = s.persistAnalyzeAllJob(published)
			return path, true
		}
	}
	current.Status = analysisAllStateCompleted
	current.UpdatedAt = time.Now().UTC()
	published := cloneAnalyzeAllJob(current)
	s.analysisAll.mu.Unlock()
	_ = s.persistAnalyzeAllJob(published)
	return "", false
}

func (s *Service) recordAnalyzeAllResult(path string, cause error) {
	s.analysisAll.mu.Lock()
	job := s.analysisAll.job
	if job == nil || job.Status == analysisAllStateCanceled || job.Status == analysisAllStateStale {
		s.analysisAll.mu.Unlock()
		return
	}
	for i := range job.Files {
		if job.Files[i].Path != path {
			continue
		}
		if cause == nil {
			job.Files[i].Status = analysisAllFileCompleted
		} else if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
			job.Files[i].Status = analysisAllFileFailed
			job.Files[i].Error = "analysis canceled or timed out"
		} else {
			job.Files[i].Status = analysisAllFileFailed
			job.Files[i].Error = "analysis failed"
		}
		job.UpdatedAt = time.Now().UTC()
		published := cloneAnalyzeAllJob(job)
		s.analysisAll.mu.Unlock()
		_ = s.persistAnalyzeAllJob(published)
		return
	}
	s.analysisAll.mu.Unlock()
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
	path := filepath.Join(s.manager.Root(), ".mini-orca", "sessions", "analyze-all.json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("encode analyze-all job: %w", err)
	}
	if err := storage.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("store analyze-all job: %w", err)
	}
	return nil
}

func (s *Service) persistAnalyzeAllJob(job *AnalyzeAllJob) error {
	finished := s.analysisAll.beginPersist()
	defer finished()
	return s.storeAnalyzeAllJob(job)
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
