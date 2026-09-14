package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type AnalysisSelectableFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type AnalysisFileSelection struct {
	ProjectID       string                   `json:"project_id"`
	ProjectRevision string                   `json:"project_revision"`
	SelectionID     string                   `json:"selection_id"`
	ExcludedPaths   []string                 `json:"excluded_paths"`
	Files           []AnalysisSelectableFile `json:"files"`
	Editable        bool                     `json:"editable"`
}

type AnalysisSelectionRequest struct {
	ProjectID       string   `json:"project_id"`
	ProjectRevision string   `json:"project_revision"`
	SelectionID     string   `json:"selection_id"`
	ExcludedPaths   []string `json:"excluded_paths"`
}

func (r AnalysisSelectionRequest) Validate() error {
	if r.ProjectID == "" || r.ProjectRevision == "" || r.SelectionID == "" || r.ExcludedPaths == nil {
		return fmt.Errorf("selection requires project identity, selection identity and excluded_paths")
	}
	return validateAnalysisExcludedPaths(r.ExcludedPaths)
}

func (s *Service) ReadAnalysisSelection(ctx context.Context, id, revision string) (*AnalysisFileSelection, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.analysisRun.mu.Lock()
	defer s.analysisRun.mu.Unlock()
	return s.analysisSelectionLocked(ctx, id, revision)
}

func (s *Service) analysisSelectionLocked(ctx context.Context, id, revision string) (*AnalysisFileSelection, error) {
	if err := s.restoreAnalysisRunLocked(); err != nil {
		return nil, err
	}
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return nil, err
	}
	if analysis.ProjectID != id || analysis.ProjectRevision != revision {
		return nil, project.ErrRevisionConflict
	}
	excluded, err := loadAnalysisSelection(s.manager.Root())
	if err != nil {
		return nil, err
	}
	selectionID, err := analysisFingerprint(excluded)
	if err != nil {
		return nil, err
	}
	result := &AnalysisFileSelection{ProjectID: id, ProjectRevision: revision, SelectionID: selectionID, ExcludedPaths: excluded, Files: []AnalysisSelectableFile{}, Editable: s.analysisSelectionEditableLocked()}
	paths, err := project.WalkProjectFiles(ctx, project.ProjectWalkOptions{Root: s.manager.Root(), IncludeSymlinkFiles: true, MaxFiles: maxAnalysisInventoryFiles})
	if err != nil {
		return nil, err
	}
	indexed := make(map[string]project.IndexFile, len(index.Files))
	for _, file := range index.Files {
		indexed[file.Path] = file
	}
	for _, path := range paths {
		decision := policy.Decide(path)
		reason := decision.Reason
		if decision.Include {
			file, ok := indexed[path]
			if !ok {
				return nil, project.ErrRevisionConflict
			}
			reason = analysisFileExclusion(file, policy)
		}
		result.Files = append(result.Files, AnalysisSelectableFile{Path: path, Reason: reason})
	}
	return result, nil
}

func (s *Service) analysisSelectionEditableLocked() bool {
	c := s.analysisRun
	if c.done != nil || c.fault != nil {
		return false
	}
	if c.run != nil {
		switch c.run.Status {
		case AnalysisRunRunning, AnalysisRunQueued, AnalysisRunPausing, AnalysisRunPaused, AnalysisRunInterrupted:
			return false
		}
	}
	return true
}

func (s *Service) SaveAnalysisSelection(ctx context.Context, request AnalysisSelectionRequest) (*AnalysisFileSelection, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.analysisRun.mu.Lock()
	defer s.analysisRun.mu.Unlock()
	current, err := s.analysisSelectionLocked(ctx, request.ProjectID, request.ProjectRevision)
	if err != nil {
		return nil, err
	}
	if !current.Editable {
		return nil, errAnalysisRunBusy
	}
	if current.SelectionID != request.SelectionID {
		return nil, project.ErrRevisionConflict
	}
	// Retain exclusions for temporarily absent files, but reject invented new paths.
	known := make(map[string]bool)
	for _, file := range current.Files {
		known[file.Path] = true
	}
	for _, path := range current.ExcludedPaths {
		known[path] = true
	}
	for _, path := range request.ExcludedPaths {
		if !known[path] {
			return nil, project.ErrRevisionConflict
		}
	}
	paths := append([]string{}, request.ExcludedPaths...)
	sort.Strings(paths)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := saveAnalysisSelection(s.manager.Root(), paths); err != nil {
		return nil, err
	}
	current.ExcludedPaths = paths
	current.SelectionID, err = analysisFingerprint(paths)
	return current, err
}
