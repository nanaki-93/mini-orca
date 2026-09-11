package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
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
	ProjectRevision   string        `json:"-"`
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

// StartPerformanceJob admits only the existing bounded Performance queue.
func (s *Service) StartPerformanceJob(ctx context.Context, options PerformanceJobOptions, confirmRemoteProvider bool) (*PerformanceJob, error) {
	if err := s.RequireRemoteConfirmation(config.AnalyzeModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	options = normalizePerformanceOptions(options)
	limits := AnalysisRunLimits{BatchFiles: options.MaxFiles, BudgetSeconds: int((options.RunBudget + time.Second - 1) / time.Second), MaxAttemptsPerStage: min(4, s.runtimes.analyze.effective.MaxRetries+1)}
	run, err := s.startCompatibilityRun(ctx, AnalysisStagePerformance, limits, options.RunBudget, options, confirmRemoteProvider)
	return performanceProjection(run, s.manager.Root()), err
}
func (s *Service) PerformanceJob() (*PerformanceJob, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.performanceJobLocked()
}
func (s *Service) performanceJobLocked() (*PerformanceJob, error) {
	run, err := s.currentAnalysisRunLocked(context.Background())
	if err != nil {
		return performanceProjection(run, s.manager.Root()), err
	}
	if run != nil && run.Plan.CompatibilityStage == AnalysisStagePerformance {
		return performanceProjection(run, s.manager.Root()), nil
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	job, err := loadPerformanceJob(s.manager.Root())
	if err != nil || job == nil {
		return job, err
	}
	if job.ProjectID != analysis.ProjectID || job.ProjectRevision != analysis.ProjectRevision {
		job.Status = performanceJobStale
	} else if job.Status == performanceJobRunning || job.Status == performanceJobPaused {
		job.Status = "interrupted"
	}
	return job, nil
}
func (s *Service) PausePerformanceJob(expectedID, revision string) (*PerformanceJob, error) {
	run, err := s.controlCompatibilityRun(context.Background(), AnalysisStagePerformance, AnalysisRunPause, expectedID, false, revision)
	return performanceProjection(run, s.manager.Root()), err
}
func (s *Service) CancelPerformanceJob(expectedID, revision string) (*PerformanceJob, error) {
	run, err := s.controlCompatibilityRun(context.Background(), AnalysisStagePerformance, AnalysisRunCancel, expectedID, false, revision)
	return performanceProjection(run, s.manager.Root()), err
}
func (s *Service) ResumePerformanceJob(ctx context.Context, expectedID string, confirmRemoteProvider bool, revision ...string) (*PerformanceJob, error) {
	if err := s.RequireRemoteConfirmation(config.AnalyzeModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	run, err := s.controlCompatibilityRun(ctx, AnalysisStagePerformance, AnalysisRunResume, expectedID, confirmRemoteProvider, revision...)
	return performanceProjection(run, s.manager.Root()), err
}

func (s *Service) PreviewPerformanceQueue(options PerformanceJobOptions) (*PerformanceQueuePreview, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
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

func (s *Service) PerformanceProjectReport() (*PerformanceReport, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	job, err := s.performanceJobLocked()
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
		return nil, fmt.Errorf("saved performance job is invalid; preserved for recovery")
	}
	jobRoot, err := canonicalPerformanceJobRoot(job.Root)
	if err != nil || jobRoot != root {
		return nil, fmt.Errorf("saved performance job is invalid; preserved for recovery")
	}
	job.Root = root
	return clonePerformanceJob(&job), nil
}

func canonicalPerformanceJobRoot(root string) (string, error) {
	canonical, err := project.CanonicalRoot(root)
	if err != nil {
		return "", fmt.Errorf("canonicalize performance job root: %w", err)
	}
	return canonical, nil
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
