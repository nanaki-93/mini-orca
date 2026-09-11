package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
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
	ProjectRevision string `json:"-"`
	MaxFiles        int    `json:"max_files,omitempty"`
	MaxRetries      int    `json:"max_retries,omitempty"`
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

// StartAnalyzeAll is a semantic-only bounded adapter to the shared run owner.
func (s *Service) StartAnalyzeAll(ctx context.Context, options AnalyzeAllOptions, confirmRemoteProvider bool) (*AnalyzeAllJob, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	options = normalizeAnalyzeAllOptions(options)
	run, err := s.startCompatibilityRun(ctx, AnalysisStageSemantic, AnalysisRunLimits{BatchFiles: options.MaxFiles, BudgetSeconds: 3600, MaxAttemptsPerStage: options.MaxRetries + 1}, 0, PerformanceJobOptions{ProjectRevision: options.ProjectRevision}, confirmRemoteProvider)
	return analyzeAllProjection(run), err
}

// AnalyzeAllJob restores progress without granting execution authority.
func (s *Service) AnalyzeAllJob() (*AnalyzeAllJob, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	run, err := s.currentAnalysisRunLocked(context.Background())
	if err != nil {
		return analyzeAllProjection(run), err
	}
	if run != nil && run.Plan.CompatibilityStage == AnalysisStageSemantic {
		return analyzeAllProjection(run), nil
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	job, err := loadAnalyzeAllJob(s.manager.Root())
	if err != nil || job == nil {
		return job, err
	}
	if job.ProjectID != analysis.ProjectID || job.ProjectRevision != analysis.ProjectRevision {
		job.Status = analysisAllStateStale
	} else if job.Status == analysisAllStateRunning || job.Status == analysisAllStatePaused {
		job.Status = "interrupted"
	}
	return job, nil
}

func (s *Service) PauseAnalyzeAll(revision ...string) (*AnalyzeAllJob, error) {
	run, err := s.controlCompatibilityRun(context.Background(), AnalysisStageSemantic, AnalysisRunPause, "", false, revision...)
	return analyzeAllProjection(run), err
}
func (s *Service) CancelAnalyzeAll(revision ...string) (*AnalyzeAllJob, error) {
	run, err := s.controlCompatibilityRun(context.Background(), AnalysisStageSemantic, AnalysisRunCancel, "", false, revision...)
	return analyzeAllProjection(run), err
}
func (s *Service) ResumeAnalyzeAll(ctx context.Context, confirmRemoteProvider bool, revision ...string) (*AnalyzeAllJob, error) {
	if err := s.RequireRemoteConfirmation(config.BugModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	run, err := s.controlCompatibilityRun(ctx, AnalysisStageSemantic, AnalysisRunResume, "", confirmRemoteProvider, revision...)
	return analyzeAllProjection(run), err
}

func (s *Service) Reindex() (*project.ProjectIndex, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.reindexActiveProject()
}

func (s *Service) reindexActiveProject() (*project.ProjectIndex, error) {
	s.invalidateAnalysisRun()
	s.cancelGoScan()
	index, err := s.manager.Reindex()
	if err == nil {
		s.clearExecutionTrust()
		s.ExpireDraftsForOpenFile(index.ProjectID, index.ProjectRevision, "", "")
	}
	return index, err
}

func (s *Service) ActivateProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error { return s.manager.Set(root, analysis) })
}

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
	s.invalidateAnalysisRun()
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

func isSemanticAnalysisCandidate(file project.IndexFile) bool {
	return !file.Binary && file.Language != "Text" && file.Language != "Markdown"
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
