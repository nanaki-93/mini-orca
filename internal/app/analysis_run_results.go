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
	reader, err := s.analysisSectionReader(run, category, path, c.root)
	if err != nil {
		return nil, err
	}
	if err := reader.read(ctx, path); err != nil {
		return nil, err
	}
	return reader.result, nil
}

// This projection owns only a captured read snapshot; all callers hold the run locks.
type analysisSectionReader struct {
	service  *Service
	root     string
	run      *AnalysisRun
	analysis *project.Analysis
	index    *project.ProjectIndex
	policy   *project.ContextPolicy
	captured map[string]AnalysisRunFile
	statuses map[string]string
	result   *AnalysisSectionResults
}

func (s *Service) analysisSectionReader(run *AnalysisRun, category project.FindingCategory, path, root string) (*analysisSectionReader, error) {
	identity := run.Identity
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
	statuses, err := loadAnalysisTriage(root, identity, hashes)
	if err != nil {
		return nil, err
	}
	return &analysisSectionReader{service: s, root: root, run: run, analysis: analysis, index: index, policy: policy, captured: captured, statuses: statuses, result: result}, nil
}

func loadAnalysisTriage(root string, identity AnalysisRunIdentity, hashes map[string]string) (map[string]string, error) {
	store, err := project.NewFindingStore(root)
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
	return statuses, nil
}

func (reader *analysisSectionReader) read(ctx context.Context, path string) error {
	for _, indexed := range reader.index.Files {
		file, ok := reader.captured[indexed.Path]
		if !ok || path != "" && path != file.Path {
			continue
		}
		if !reader.policy.Decide(file.Path).Include {
			return project.ErrExcludedFile
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := reader.readSemantic(indexed, file); err != nil {
			return err
		}
		switch reader.result.Progress.Category {
		case project.FindingCategoryPerformance:
			if err := reader.readPerformance(file); err != nil {
				return err
			}
		case project.FindingCategorySecurity:
			if err := reader.readSecurity(indexed, file); err != nil {
				return err
			}
		}
	}
	return nil
}

func (reader *analysisSectionReader) readSemantic(indexed project.IndexFile, file AnalysisRunFile) error {
	cache, input, err := reader.service.fileAnalysisCacheInput(reader.analysis, &indexed, file.ContentHash)
	if err != nil {
		return err
	}
	semantic, err := cache.Load(input)
	if err != nil {
		return err
	}
	if file.Stages[0].FindingCount != nil && (semantic == nil || semantic.Status == project.AnalysisStatusMissing) {
		return project.ErrRevisionConflict
	}
	if semantic != nil && semantic.ProjectID == reader.run.Identity.ProjectID {
		reader.appendSemanticFindings(semantic, file)
	}

	return nil
}

func (reader *analysisSectionReader) appendSemanticFindings(semantic *project.FileAnalysis, file AnalysisRunFile) {
	fresh := semantic.Status == project.AnalysisStatusFresh && semantic.ContentHash == file.ContentHash && semantic.ProjectRevision == reader.run.Identity.ProjectRevision && reader.run.Status != AnalysisRunStale
	// The adapter only accepts fresh status. Extract historical risks from a copy,
	// then explicitly label them stale; no cache or history record is rewritten.
	view := *semantic
	view.Status = project.AnalysisStatusFresh
	for _, finding := range project.SuggestedFindingsForFile(view) {
		finding.ID = project.FindingID(finding)
		finding.ProjectID, finding.ProjectRevision = semantic.ProjectID, semantic.ProjectRevision
		finding.DetectedAt = semantic.GeneratedAt
		finding.Status = project.FindingStatusOpen
		if status := reader.statuses[finding.ID]; status != "" {
			finding.Status = status
		}
		finding.Freshness = project.FindingFreshnessStale
		if !finding.Category.Valid() {
			reader.result.Unclassified = append(reader.result.Unclassified, finding)
			continue
		}
		if finding.Category != reader.result.Progress.Category || file.Stages[0].FindingCount == nil {
			continue
		}
		if fresh {
			finding.Freshness = project.FindingFreshnessFresh
		}
		reader.result.Semantic = append(reader.result.Semantic, finding)
	}
}

func (reader *analysisSectionReader) readPerformance(file AnalysisRunFile) error {
	if file.Stages[1].FindingCount == nil {
		return nil
	}
	report, err := project.LoadPerformanceFileReport(reader.root, file.Path, file.ContentHash, reader.policy)
	if err != nil {
		return err
	}
	if report == nil {
		return project.ErrRevisionConflict
	}
	if report.ProjectID == reader.run.Identity.ProjectID && report.ProjectRevision == reader.run.Identity.ProjectRevision {
		if reader.run.Status == AnalysisRunStale || !analysisPerformanceCacheUsable(report, *reader.analysis, reader.service.runtimes.analyze) {
			report.Status = "stale"
		}
		reader.result.Performance = append(reader.result.Performance, *report)
	}
	return nil
}

func (reader *analysisSectionReader) readSecurity(indexed project.IndexFile, file AnalysisRunFile) error {
	for j := 2; j <= 3; j++ {
		if file.Stages[j].FindingCount == nil {
			continue
		}
		input := analysisSecurityCacheInput(*reader.analysis, indexed, reader.service.runtimes.analyze, reader.policy.Version())
		if j == 2 {
			input = securityRulesInput(securityRulesSnapshot{analysis: *reader.analysis, file: indexed, policyVersion: reader.policy.Version()})
		}
		report, err := reader.service.loadSecurityFileReport(reader.root, input)
		if err != nil {
			return err
		}
		if report == nil {
			return project.ErrRevisionConflict
		}
		if report.ProjectID == reader.run.Identity.ProjectID && report.ProjectRevision == reader.run.Identity.ProjectRevision {
			if reader.run.Status == AnalysisRunStale {
				report.Status = project.SecurityStatusStale
			}
			reader.result.Security = append(reader.result.Security, *report)
		}
	}
	return nil
}
