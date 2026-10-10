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
	service     *Service
	root        string
	run         *AnalysisRun
	analysis    *project.Analysis
	index       *project.ProjectIndex
	policy      *project.ContextPolicy
	captured    map[string]AnalysisRunFile
	statuses    map[string]string
	result      *AnalysisSectionResults
	hasEvidence bool
}

func (s *Service) analysisSectionReader(run *AnalysisRun, category project.FindingCategory, path, root string) (*analysisSectionReader, error) {
	identity := run.Identity
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return nil, err
	}
	if analysis.ProjectID != identity.ProjectID {
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
	// A scoped retry or recovery changes dispatch scope, not the saved findings
	// inventory. User/policy exclusions remain excluded; only scope-omitted files are retained.
	if omitted := analysisSelectionMode(run.Plan.RetryStaleFailed, run.Plan.RecoverIncomplete, run.Plan.StaleOnly).exclusion(); path == "" && omitted != "" {
		retained := make(map[string]bool)
		for _, file := range run.Plan.Excluded {
			retained[file.Path] = file.Reason == omitted || file.Reason == analysisStalePathExclusion
		}
		for _, file := range index.Files {
			if !retained[file.Path] {
				continue
			}
			identity := AnalysisFileIdentity{Path: file.Path, ContentHash: file.ContentHash, Language: file.Language}
			captured[file.Path] = AnalysisRunFile{AnalysisFileIdentity: identity, Stages: make([]AnalysisStageProgress, len(analysisStages))}
			hashes[file.Path] = file.ContentHash
			result.RetainedFiles = append(result.RetainedFiles, identity)
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
		if err := reader.readFile(indexed, file); err != nil {
			return err
		}
	}
	if reader.hasEvidence {
		count := len(reader.result.Semantic)
		for _, report := range reader.result.Performance {
			count += len(report.Findings)
		}
		for _, report := range reader.result.Security {
			count += len(report.Findings)
		}
		reader.result.SavedFindingCount = &count
	}
	return nil
}

func (reader *analysisSectionReader) readFile(indexed project.IndexFile, file AnalysisRunFile) error {
	// Run identity describes the saved job; freshness follows each file.
	info, err := project.GetFileInfo(reader.root, file.Path)
	if err != nil {
		return err
	}
	indexed.ContentHash, file.ContentHash = info.ContentHash, info.ContentHash
	if err := reader.readSemantic(indexed, file); err != nil {
		return err
	}
	switch reader.result.Progress.Category {
	case project.FindingCategoryPerformance:
		return reader.readPerformance(file)
	case project.FindingCategorySecurity:
		return reader.readSecurity(indexed, file)
	default:
		return nil
	}
}

func (reader *analysisSectionReader) readSemantic(indexed project.IndexFile, file AnalysisRunFile) error {
	cache, err := project.NewFileAnalysisCache(reader.root)
	if err != nil {
		return err
	}
	semantic, err := cache.Load(semanticCacheInputForRuntime(reader.analysis, &indexed, file.ContentHash, reader.policy.Version(), reader.service.analysisModelRuntime(AnalysisStageSemantic, reader.run.Plan.Models)))
	if err != nil {
		return err
	}
	if file.Stages[0].FindingCount != nil && (semantic == nil || semantic.Status == project.AnalysisStatusMissing) {
		return project.ErrRevisionConflict
	}
	if semantic != nil && semantic.ProjectID == reader.run.Identity.ProjectID {
		reader.hasEvidence = reader.hasEvidence || semantic.Status == project.AnalysisStatusFresh
		reader.appendSemanticFindings(semantic, file)
	}

	return nil
}

func (reader *analysisSectionReader) appendSemanticFindings(semantic *project.FileAnalysis, file AnalysisRunFile) {
	fresh := analysisSemanticCacheUsable(semantic) && semantic.ContentHash == file.ContentHash
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
		if finding.Category != reader.result.Progress.Category {
			continue
		}
		if fresh {
			finding.Freshness = project.FindingFreshnessFresh
		}
		reader.result.Semantic = append(reader.result.Semantic, finding)
		reader.hasEvidence = true
	}
}

func (reader *analysisSectionReader) readPerformance(file AnalysisRunFile) error {
	report, err := project.LoadPerformanceFileReport(reader.root, file.Path, file.ContentHash, reader.policy)
	if err != nil {
		return err
	}
	if report == nil {
		if file.Stages[1].FindingCount != nil {
			return project.ErrRevisionConflict
		}
		return nil
	}
	if report.ProjectID == reader.run.Identity.ProjectID {
		hasEvidence := report.Status == "completed" || report.Status == "completed_empty" || report.Status == "partial" || len(report.Findings) > 0
		if hasEvidence && !analysisPerformanceCacheUsable(report, *reader.analysis, reader.service.analysisModelRuntime(AnalysisStagePerformance, reader.run.Plan.Models)) {
			report.Status = "stale"
		}
		reader.result.Performance = append(reader.result.Performance, *report)
		reader.hasEvidence = reader.hasEvidence || hasEvidence
	}
	return nil
}

func (reader *analysisSectionReader) readSecurity(indexed project.IndexFile, file AnalysisRunFile) error {
	for j := 2; j <= 3; j++ {
		input := analysisSecurityCacheInput(*reader.analysis, indexed, reader.service.analysisModelRuntime(AnalysisStageSecurityAI, reader.run.Plan.Models), reader.policy.Version())
		if j == 2 {
			input = securityRulesInput(securityRulesSnapshot{analysis: *reader.analysis, file: indexed, policyVersion: reader.policy.Version()})
		}
		report, err := reader.service.loadSecurityFileReport(reader.root, input)
		if err != nil {
			return err
		}
		if report == nil {
			if file.Stages[j].FindingCount != nil {
				return project.ErrRevisionConflict
			}
			continue
		}
		if report.ProjectID == reader.run.Identity.ProjectID {
			hasEvidence := securityStageCacheUsable(report) || len(report.Findings) > 0
			reader.result.Security = append(reader.result.Security, *report)
			reader.hasEvidence = reader.hasEvidence || hasEvidence
		}
	}
	return nil
}
