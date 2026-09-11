package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ReadAnalysisRun guards the project even when it has no saved run yet.
func (s *Service) ReadAnalysisRun(ctx context.Context, projectID, revision string) (*AnalysisRun, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if analysis.ProjectID != projectID || analysis.ProjectRevision != revision {
		return nil, project.ErrRevisionConflict
	}
	return s.currentAnalysisRunLocked(ctx)
}

// ReadAnalysisSection reads producer evidence only. It neither reconciles finding
// stores nor runs a stage; a file filter never changes the captured coverage.
func (s *Service) ReadAnalysisSection(ctx context.Context, identity AnalysisRunIdentity, category project.FindingCategory, path string) (*AnalysisSectionResults, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	if !category.Valid() || path != "" && !analysisMetadataPath(path) {
		return nil, fmt.Errorf("invalid analysis results filter")
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	_, err := s.currentAnalysisRunLocked(ctx)
	if err != nil && err != errAnalysisRunPersistence {
		return nil, err
	}
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	run := cloneAnalysisRun(c.run)
	if run == nil || run.Identity != identity {
		return nil, project.ErrRevisionConflict
	}
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return nil, err
	}
	if analysis.ProjectID != identity.ProjectID || analysis.ProjectRevision != identity.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	result := &AnalysisSectionResults{Identity: identity, Path: path, Semantic: []project.UnifiedFinding{}, Performance: []project.PerformanceFileReport{}, Security: []project.SecurityFileReport{}, Unclassified: []project.UnifiedFinding{}}
	for _, section := range run.Sections {
		if section.Category == category {
			result.Progress = section
		}
	}
	captured := map[string]AnalysisRunFile{}
	hashes := map[string]string{}
	for _, file := range run.Files {
		captured[file.Path] = file
		hashes[file.Path] = file.ContentHash
	}
	if path != "" {
		if _, ok := captured[path]; !ok {
			return nil, project.ErrRevisionConflict
		}
	}
	store, err := project.NewFindingStore(c.root)
	if err != nil {
		return nil, err
	}
	triage, err := store.Load(project.FindingInput{ProjectID: identity.ProjectID, ProjectRevision: identity.ProjectRevision, FileHashes: hashes})
	if err != nil {
		return nil, err
	}
	statuses := map[string]string{}
	for _, finding := range triage {
		if finding.ProjectID == identity.ProjectID {
			statuses[finding.ID] = finding.Status
		}
	}
	for _, indexed := range index.Files {
		file, ok := captured[indexed.Path]
		if !ok || path != "" && path != file.Path {
			continue
		}
		if !policy.Decide(file.Path).Include {
			return nil, project.ErrExcludedFile
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cache, input, err := s.fileAnalysisCacheInput(analysis, &indexed, file.ContentHash)
		if err != nil {
			return nil, err
		}
		semantic, err := cache.Load(input)
		if err != nil {
			return nil, err
		}
		if file.Stages[0].FindingCount != nil && (semantic == nil || semantic.Status == project.AnalysisStatusMissing) {
			return nil, project.ErrRevisionConflict
		}
		if semantic != nil && semantic.ProjectID == identity.ProjectID {
			fresh := semantic.Status == project.AnalysisStatusFresh && semantic.ContentHash == file.ContentHash && semantic.ProjectRevision == identity.ProjectRevision && run.Status != AnalysisRunStale
			// The adapter only accepts fresh status. Extract historical risks from a copy,
			// then explicitly label them stale; no cache or history record is rewritten.
			view := *semantic
			view.Status = project.AnalysisStatusFresh
			for _, finding := range project.SuggestedFindingsForFile(view) {
				finding.ID = project.FindingID(finding)
				finding.ProjectID, finding.ProjectRevision = semantic.ProjectID, semantic.ProjectRevision
				finding.DetectedAt = semantic.GeneratedAt
				finding.Status = project.FindingStatusOpen
				if status := statuses[finding.ID]; status != "" {
					finding.Status = status
				}
				finding.Freshness = project.FindingFreshnessStale
				if !finding.Category.Valid() {
					result.Unclassified = append(result.Unclassified, finding)
					continue
				}
				if finding.Category != category || file.Stages[0].FindingCount == nil {
					continue
				}
				if fresh {
					finding.Freshness = project.FindingFreshnessFresh
				}
				result.Semantic = append(result.Semantic, finding)
			}
		}
		if category == project.FindingCategoryPerformance && file.Stages[1].FindingCount != nil {
			report, err := project.LoadPerformanceFileReport(c.root, file.Path, file.ContentHash, policy)
			if err != nil {
				return nil, err
			}
			if report == nil {
				return nil, project.ErrRevisionConflict
			}
			if report != nil && report.ProjectID == identity.ProjectID && report.ProjectRevision == identity.ProjectRevision {
				if run.Status == AnalysisRunStale || !analysisPerformanceCacheUsable(report, *analysis, s.runtimes.analyze) {
					report.Status = "stale"
				}
				result.Performance = append(result.Performance, *report)
			}
		}
		if category == project.FindingCategorySecurity {
			for j := 2; j <= 3; j++ {
				if file.Stages[j].FindingCount == nil {
					continue
				}
				input := analysisSecurityCacheInput(*analysis, indexed, s.runtimes.analyze, policy.Version())
				if j == 2 {
					input = securityRulesInput(securityRulesSnapshot{analysis: *analysis, file: indexed, policyVersion: policy.Version()})
				}
				report, err := s.loadSecurityFileReport(c.root, input)
				if err != nil {
					return nil, err
				}
				if report == nil {
					return nil, project.ErrRevisionConflict
				}
				if report != nil && report.ProjectID == identity.ProjectID && report.ProjectRevision == identity.ProjectRevision {
					if run.Status == AnalysisRunStale {
						report.Status = project.SecurityStatusStale
					}
					result.Security = append(result.Security, *report)
				}
			}
		}
	}
	return result, nil
}
