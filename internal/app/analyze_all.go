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

	"github.com/nanaki-93/mini-orca/v2/internal/project"
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
	mu     sync.Mutex
	job    *AnalyzeAllJob
	cancel context.CancelFunc
}

func newAnalysisAllController() *analysisAllController { return &analysisAllController{} }

// StartAnalyzeAll explicitly starts a bounded, sequential cache-warming job.
// Importing a project never calls this method.
func (s *Service) StartAnalyzeAll(ctx context.Context, options AnalyzeAllOptions, confirmRemoteProvider bool) (*AnalyzeAllJob, error) {
	if err := s.RequireRemoteConfirmation(confirmRemoteProvider); err != nil {
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

	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	if current := s.analysisAll.job; current != nil && current.ProjectID == analysis.ProjectID && current.ProjectRevision == analysis.ProjectRevision && (current.Status == analysisAllStateRunning || current.Status == analysisAllStatePaused) {
		return cloneAnalyzeAllJob(current), nil
	}
	files, err := s.analysisAllCandidates(index, options.MaxFiles)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	job := &AnalyzeAllJob{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Status: analysisAllStateRunning, MaxFiles: options.MaxFiles, MaxRetries: options.MaxRetries, Files: files, CreatedAt: now, UpdatedAt: now}
	if err := s.storeAnalyzeAllJobLocked(job); err != nil {
		return nil, err
	}
	s.analysisAll.job = job
	s.startAnalyzeAllWorkerLocked(ctx)
	return cloneAnalyzeAllJob(job), nil
}

// AnalyzeAllJob returns durable progress for the active project, if any.
func (s *Service) AnalyzeAllJob() (*AnalyzeAllJob, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	if s.analysisAll.job != nil {
		if s.analysisAll.job.ProjectID == analysis.ProjectID && s.analysisAll.job.ProjectRevision == analysis.ProjectRevision {
			return cloneAnalyzeAllJob(s.analysisAll.job), nil
		}
		if s.analysisAll.job.Status != analysisAllStateStale {
			s.analysisAll.job.Status = analysisAllStateStale
			s.analysisAll.job.UpdatedAt = time.Now().UTC()
			if s.analysisAll.cancel != nil {
				s.analysisAll.cancel()
			}
			if err := s.storeAnalyzeAllJobLocked(s.analysisAll.job); err != nil {
				return nil, err
			}
		}
		return cloneAnalyzeAllJob(s.analysisAll.job), nil
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
		if err := s.storeAnalyzeAllJobLocked(job); err != nil {
			return nil, err
		}
	}
	s.analysisAll.job = job
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
func (s *Service) ResumeAnalyzeAll(ctx context.Context, confirmRemoteProvider bool) (*AnalyzeAllJob, error) {
	if err := s.RequireRemoteConfirmation(confirmRemoteProvider); err != nil {
		return nil, err
	}
	if _, err := s.manager.Analysis(); err != nil {
		return nil, err
	}
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	if s.analysisAll.job == nil {
		job, err := loadAnalyzeAllJob(s.manager.Root())
		if err != nil {
			return nil, err
		}
		if job == nil {
			return nil, fmt.Errorf("analyze-all job not found")
		}
		s.analysisAll.job = job
	}
	job := s.analysisAll.job
	if err := s.verifyAnalyzeAllRevisionLocked(job); err != nil {
		return nil, err
	}
	if job.Status != analysisAllStatePaused {
		return nil, fmt.Errorf("analyze-all job is not paused")
	}
	job.Status = analysisAllStateRunning
	job.UpdatedAt = time.Now().UTC()
	if err := s.storeAnalyzeAllJobLocked(job); err != nil {
		return nil, err
	}
	s.startAnalyzeAllWorkerLocked(ctx)
	return cloneAnalyzeAllJob(job), nil
}

// Reindex invalidates an active job before deterministic facts are refreshed.
func (s *Service) Reindex() (*project.ProjectIndex, error) {
	s.invalidateAnalyzeAll("project index was refreshed")
	s.cancelGoScan()
	index, err := s.manager.Reindex()
	if err == nil {
		s.ExpireDraftsForOpenFile(index.ProjectID, index.ProjectRevision, "", "")
	}
	return index, err
}

// ProjectChanged stops a job before an imported project replaces the active project.
func (s *Service) ProjectChanged() {
	s.invalidateAnalyzeAll("active project changed")
	s.cancelGoScan()
	s.clearDraftsForProjectChange()
	s.clearChatSessions()
}

func (s *Service) cancelGoScan() {
	s.goScan.mu.Lock()
	defer s.goScan.mu.Unlock()
	if s.goScan.cancel != nil {
		s.goScan.cancel()
	}
}

func (s *Service) changeAnalyzeAllState(next string, cancelWorker, allowCompleted bool) (*AnalyzeAllJob, error) {
	if _, err := s.manager.Analysis(); err != nil {
		return nil, err
	}
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	job := s.analysisAll.job
	if job == nil {
		return nil, fmt.Errorf("analyze-all job not found")
	}
	if err := s.verifyAnalyzeAllRevisionLocked(job); err != nil {
		return nil, err
	}
	if !allowCompleted && (job.Status == analysisAllStateCompleted || job.Status == analysisAllStateCanceled || job.Status == analysisAllStateStale) {
		return nil, fmt.Errorf("analyze-all job cannot be %s from %s", next, job.Status)
	}
	job.Status = next
	job.UpdatedAt = time.Now().UTC()
	if cancelWorker && s.analysisAll.cancel != nil {
		s.analysisAll.cancel()
	}
	if err := s.storeAnalyzeAllJobLocked(job); err != nil {
		return nil, err
	}
	return cloneAnalyzeAllJob(job), nil
}

func (s *Service) invalidateAnalyzeAll(reason string) {
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	if s.analysisAll.job == nil || s.analysisAll.job.Status != analysisAllStateRunning && s.analysisAll.job.Status != analysisAllStatePaused {
		return
	}
	s.analysisAll.job.Status = analysisAllStateStale
	s.analysisAll.job.UpdatedAt = time.Now().UTC()
	if s.analysisAll.cancel != nil {
		s.analysisAll.cancel()
	}
	_ = s.storeAnalyzeAllJobLocked(s.analysisAll.job)
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

func (s *Service) startAnalyzeAllWorkerLocked(parent context.Context) {
	if s.analysisAll.cancel != nil {
		return
	}
	worker, cancel := context.WithCancel(context.Background())
	s.analysisAll.cancel = cancel
	go s.runAnalyzeAll(worker)
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
	defer s.analysisAll.mu.Unlock()
	job := s.analysisAll.job
	if job == nil || job.Status != analysisAllStateRunning {
		return "", false
	}
	if err := s.verifyAnalyzeAllRevisionLocked(job); err != nil {
		job.Status = analysisAllStateStale
		job.UpdatedAt = time.Now().UTC()
		_ = s.storeAnalyzeAllJobLocked(job)
		return "", false
	}
	for i := range job.Files {
		if job.Files[i].Status == analysisAllFilePending || job.Files[i].Status == analysisAllFileFailed && job.Files[i].Attempts <= job.MaxRetries {
			job.Files[i].Status = analysisAllFileRunning
			job.Files[i].Attempts++
			job.Files[i].Error = ""
			job.UpdatedAt = time.Now().UTC()
			_ = s.storeAnalyzeAllJobLocked(job)
			return job.Files[i].Path, true
		}
	}
	job.Status = analysisAllStateCompleted
	job.UpdatedAt = time.Now().UTC()
	_ = s.storeAnalyzeAllJobLocked(job)
	return "", false
}

func (s *Service) recordAnalyzeAllResult(path string, cause error) {
	s.analysisAll.mu.Lock()
	defer s.analysisAll.mu.Unlock()
	job := s.analysisAll.job
	if job == nil || job.Status == analysisAllStateCanceled || job.Status == analysisAllStateStale {
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
		_ = s.storeAnalyzeAllJobLocked(job)
		return
	}
}

func (s *Service) verifyAnalyzeAllRevisionLocked(job *AnalyzeAllJob) error {
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	if index.ProjectID != job.ProjectID || index.ProjectRevision != job.ProjectRevision {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) storeAnalyzeAllJobLocked(job *AnalyzeAllJob) error {
	path := filepath.Join(s.manager.Root(), ".mini-orca", "sessions", "analyze-all.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create analyze-all session directory: %w", err)
	}
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("encode analyze-all job: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".analyze-all-*.tmp")
	if err != nil {
		return fmt.Errorf("create analyze-all job temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write analyze-all job: %w", err)
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set analyze-all job permissions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync analyze-all job: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close analyze-all job: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace analyze-all job: %w", err)
	}
	return nil
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
