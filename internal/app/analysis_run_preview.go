package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// Match the daemon’s supported project inventory ceiling; batches remain independent.
const maxAnalysisInventoryFiles = 20000

var analysisStages = [...]AnalysisStage{AnalysisStageSemantic, AnalysisStagePerformance, AnalysisStageSecurityRules, AnalysisStageSecurityAI}

func analysisFingerprint(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode analysis identity: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (s *Service) analysisProviders() ([]AnalysisProviderRequirement, error) {
	providers := make([]AnalysisProviderRequirement, 0, 2)
	for _, entry := range []struct {
		runtime modelRuntime
		stages  []AnalysisStage
		prompts []string
	}{
		{s.runtimes.bug, []AnalysisStage{AnalysisStageSemantic}, []string{semanticAnalysisPromptVersion}},
		{s.runtimes.analyze, []AnalysisStage{AnalysisStagePerformance, AnalysisStageSecurityAI}, []string{project.PerformancePromptVersion, project.SecurityPromptVersion}},
	} {
		id, err := analysisFingerprint(struct {
			Model      EffectiveModel
			Endpoint   string
			Configured bool
			Prompts    []string
		}{entry.runtime.effective, entry.runtime.profile.APIBaseURL, entry.runtime.client != nil, entry.prompts})
		if err != nil {
			return nil, err
		}
		providers = append(providers, AnalysisProviderRequirement{ID: id, Stages: entry.stages, Model: entry.runtime.effective, RemoteConfirmationRequired: entry.runtime.effective.RemoteProvider})
	}
	return providers, nil
}

// Preview is read-only with respect to execution: no requests or passive scans run.
// BatchFiles limits a window; it never truncates the captured inventory.
func (s *Service) PreviewAnalysisRun(ctx context.Context, request AnalysisPreviewRequest) (*AnalysisRunPreview, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := s.restoreAnalysisRunLocked(); err != nil && (c.run == nil || c.fault == nil) {
		return nil, err
	}
	return s.analysisPreviewLocked(ctx, request)
}

func (s *Service) analysisPreviewLocked(ctx context.Context, request AnalysisPreviewRequest) (*AnalysisRunPreview, error) {
	if request.ResumeRun != nil {
		run := s.analysisRun.run
		if run == nil || run.Identity != *request.ResumeRun || run.Plan.RetryStaleFailed != request.RetryStaleFailed {
			return nil, project.ErrRevisionConflict
		}
		request.compatibilityStage = run.Plan.CompatibilityStage
		request.compatibilityBudget = run.Plan.CompatibilityBudget
	}
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return nil, err
	}
	if analysis.ProjectID != request.ProjectID || analysis.ProjectRevision != request.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	providers, err := s.analysisProviders()
	if err != nil {
		return nil, err
	}
	fingerprint, err := analysisFingerprint(providers)
	if err != nil {
		return nil, err
	}
	preview := &AnalysisRunPreview{CompatibilityStage: request.compatibilityStage, CompatibilityBudget: request.compatibilityBudget, RetryStaleFailed: request.RetryStaleFailed, SchemaVersion: AnalysisRunSchemaVersion, Scope: AnalysisRunScopeProject, Refresh: request.Refresh, Limits: request.Limits,
		Identity: AnalysisQueueIdentity{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, PolicyFingerprint: policy.Version(), ProviderFingerprint: fingerprint},
		Files:    []AnalysisPlannedFile{}, Excluded: []AnalysisExcludedFile{}, Providers: providers}
	root := s.manager.Root()
	if err := s.planAnalysisFiles(ctx, root, *analysis, index.Files, policy, request, preview); err != nil {
		return nil, err
	}
	return s.completeAnalysisPreviewLocked(ctx, root, preview, request)
}

func (s *Service) completeAnalysisPreviewLocked(ctx context.Context, root string, preview *AnalysisRunPreview, request AnalysisPreviewRequest) (*AnalysisRunPreview, error) {
	var err error
	if request.compatibilityStage != "" {
		if err := s.scopeCompatibilityPreview(preview, request.ResumeRun); err != nil {
			return nil, err
		}
	}
	if request.RetryStaleFailed {
		if err := s.scopeAnalysisRetryPreview(ctx, preview, request.ResumeRun); err != nil {
			return nil, err
		}
	}
	sort.Slice(preview.Excluded, func(i, j int) bool { return preview.Excluded[i].Path < preview.Excluded[j].Path })
	preview.Identity.QueueID, err = analysisQueueFingerprint(preview)
	if err != nil {
		return nil, err
	}
	if request.ResumeRun != nil {
		if err := s.applyAnalysisResumeBudget(preview, request); err != nil {
			return nil, err
		}
	}
	countAnalysisPreviewRequests(preview)
	preview.PreviewID, err = analysisFingerprint(struct {
		Preview *AnalysisRunPreview
		Resume  *AnalysisRunIdentity
	}{preview, request.ResumeRun})
	if err != nil {
		return nil, err
	}
	if err := s.validateAnalysisQueue(ctx, root, preview, false); err != nil {
		return nil, err
	}
	return preview, nil
}

func (s *Service) analysisStageCacheState(analysis project.Analysis, file project.IndexFile, policy *project.ContextPolicy, stage AnalysisStage) (bool, string, time.Time, error) {
	switch stage {
	case AnalysisStageSemantic:
		cache, input, err := s.fileAnalysisCacheInput(&analysis, &file, file.ContentHash)
		if err != nil {
			return false, "", time.Time{}, err
		}
		report, err := cache.Load(input)
		if err != nil {
			return false, "", time.Time{}, err
		}
		status := report.Status
		if status == project.AnalysisStatusFresh && !analysisSemanticCacheUsable(report, analysis.ProjectRevision) {
			status = project.AnalysisStatusStale
		}
		return analysisSemanticCacheUsable(report, analysis.ProjectRevision), status, report.GeneratedAt, nil
	case AnalysisStagePerformance:
		return s.analysisPerformanceCacheState(analysis, file, policy)
	default:
		input := analysisSecurityCacheInput(analysis, file, s.runtimes.analyze, policy.Version())
		if stage == AnalysisStageSecurityRules {
			input = securityRulesInput(securityRulesSnapshot{analysis: analysis, file: file, policyVersion: policy.Version()})
		}
		report, err := s.loadSecurityFileReport(s.manager.Root(), input)
		if err != nil || report == nil {
			return false, "", time.Time{}, err
		}
		return securityStageCacheUsable(report), report.Status, report.GeneratedAt, nil
	}
}

func (s *Service) analysisPerformanceCacheState(analysis project.Analysis, file project.IndexFile, policy *project.ContextPolicy) (bool, string, time.Time, error) {
	report, err := project.LoadPerformanceFileReport(s.manager.Root(), file.Path, file.ContentHash, policy)
	if err != nil || report == nil {
		return false, "", time.Time{}, err
	}
	cached := analysisPerformanceCacheUsable(report, analysis, s.runtimes.analyze)
	status := report.Status
	if status == "completed" && !cached {
		status = "stale"
	}
	if cached && report.Warning != "" {
		status = "partial"
	}
	return cached, status, report.GeneratedAt, nil
}

func (s *Service) validateAnalysisQueue(ctx context.Context, root string, plan *AnalysisRunPreview, sources bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return err
	}
	providers, err := s.analysisProviders()
	if err != nil {
		return err
	}
	providers = compatibilityProviders(providers, plan.CompatibilityStage)
	fingerprint, err := analysisFingerprint(providers)
	if err != nil {
		return err
	}
	if root != s.manager.Root() || plan.Identity.ProjectID != analysis.ProjectID || plan.Identity.ProjectRevision != analysis.ProjectRevision || plan.Identity.PolicyFingerprint != policy.Version() || plan.Identity.ProviderFingerprint != fingerprint {
		return project.ErrRevisionConflict
	}
	if sources {
		if err := validateAnalysisInventory(ctx, root, plan); err != nil {
			return err
		}
	}
	return validateAnalysisFiles(ctx, root, plan, index, policy, sources)
}

func validateAnalysisFiles(ctx context.Context, root string, plan *AnalysisRunPreview, index *project.ProjectIndex, policy *project.ContextPolicy, sources bool) error {
	indexed := make(map[string]project.IndexFile, len(index.Files))
	for _, file := range index.Files {
		indexed[file.Path] = file
	}
	for _, file := range plan.Files {
		current, ok := indexed[file.Path]
		if !ok || current.ContentHash != file.ContentHash || current.Language != file.Language || current.SizeBytes != file.SizeBytes || !policy.Decide(file.Path).Include {
			return project.ErrRevisionConflict
		}
		if sources {
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := project.GetFileInfo(root, file.Path)
			if err != nil {
				return err
			}
			if info.ContentHash != file.ContentHash || info.Binary {
				return project.ErrRevisionConflict
			}
		}
	}
	return nil
}

func validateAnalysisConfirmations(preview *AnalysisRunPreview, confirmations AnalysisRunConfirmations) error {
	if preview.SecurityReviewIntentRequired && !confirmations.SecurityReview {
		return fmt.Errorf("analysis requires fresh Security review intent")
	}
	confirmed := make(map[string]bool, len(confirmations.ProviderIDs))
	for _, id := range confirmations.ProviderIDs {
		if confirmed[id] {
			return fmt.Errorf("duplicate analysis provider confirmation")
		}
		confirmed[id] = true
		known := false
		for _, provider := range preview.Providers {
			known = known || provider.ID == id
		}
		if !known {
			return fmt.Errorf("analysis provider confirmation does not match preview")
		}
	}
	for _, file := range preview.Files {
		for _, stage := range file.Stages {
			if stage.MaxModelRequests == 0 {
				continue
			}
			for _, provider := range preview.Providers {
				if provider.ID == stage.ProviderID && provider.RemoteConfirmationRequired && !confirmed[provider.ID] {
					return fmt.Errorf("analysis requires confirmation for each remote provider in the preview")
				}
			}
		}
	}
	return nil
}

// The run's own report writes cannot alter its immutable queue identity.
func analysisQueueFingerprint(preview *AnalysisRunPreview) (string, error) {
	stable := cloneAnalysisPreview(*preview)
	stable.Identity.QueueID = ""
	stable.PreviewID = ""
	stable.ExpectedModelRequests = 0
	stable.MaxModelRequests = 0
	stable.SecurityReviewIntentRequired = false
	for i := range stable.Files {
		for j := range stable.Files[i].Stages {
			stable.Files[i].Stages[j].Cached = false
			stable.Files[i].Stages[j].MaxModelRequests = 0
		}
	}
	return analysisFingerprint(stable)
}

func (s *Service) planAnalysisFiles(ctx context.Context, root string, analysis project.Analysis, indexedFiles []project.IndexFile, policy *project.ContextPolicy, request AnalysisPreviewRequest, preview *AnalysisRunPreview) error {
	excludedPaths, err := loadAnalysisSelection(root)
	if err != nil {
		return err
	}
	ignored := make(map[string]bool, len(excludedPaths))
	for _, path := range excludedPaths {
		ignored[path] = true
	}
	files := append([]project.IndexFile(nil), indexedFiles...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if err := collectAnalysisInventoryExclusions(ctx, root, files, policy, preview); err != nil {
		return err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		reason := analysisFileExclusion(file, policy)
		if reason == "" && ignored[file.Path] {
			reason = analysisSelectionExclusion
		}
		if reason == "Excluded by source policy." {
			continue
		}
		if reason != "" {
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: file.Path, Reason: reason})
			continue
		}
		info, err := project.GetFileInfo(root, file.Path)
		if err != nil {
			return err
		}
		if info.ContentHash != file.ContentHash || info.Binary {
			return project.ErrRevisionConflict
		}
		planned := AnalysisPlannedFile{AnalysisFileIdentity: AnalysisFileIdentity{Path: file.Path, ContentHash: file.ContentHash, Language: file.Language}, SizeBytes: file.SizeBytes, Stages: []AnalysisStagePlan{}}
		for _, stage := range analysisStages {
			plan, err := s.planAnalysisStage(analysis, file, policy, stage, request, preview.Providers)
			if err != nil {
				return err
			}
			planned.Stages = append(planned.Stages, plan)
		}
		preview.Files = append(preview.Files, planned)
	}
	return nil
}

func collectAnalysisInventoryExclusions(ctx context.Context, root string, files []project.IndexFile, policy *project.ContextPolicy, preview *AnalysisRunPreview) error {
	paths, err := project.WalkProjectFiles(ctx, project.ProjectWalkOptions{Root: root, IncludeSymlinkFiles: true, MaxFiles: maxAnalysisInventoryFiles})
	if err != nil {
		return err
	}
	indexed := make(map[string]bool, len(files))
	for _, file := range files {
		indexed[file.Path] = true
	}
	for _, path := range paths {
		if !policy.Decide(path).Include {
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: path, Reason: "Excluded by source policy."})
		} else if !indexed[path] {
			return project.ErrRevisionConflict
		}
	}
	return nil
}

func analysisFileExclusion(file project.IndexFile, policy *project.ContextPolicy) string {
	reason := ""
	switch {
	case !policy.Decide(file.Path).Include:
		reason = "Excluded by source policy."
	case file.Binary || file.Language == "":
		reason = "Not a supported text source file."
	case file.SizeBytes > maxSemanticAnalysisBytes && file.SizeBytes > project.PerformanceMaxSourceBytes && file.SizeBytes > project.SecurityMaxSourceBytes:
		reason = "Source exceeds analyzer size limits."
	}
	return reason
}

func analysisStageExclusion(file project.IndexFile, stage AnalysisStage) string {
	switch {
	case stage == AnalysisStageSemantic && (!isSemanticAnalysisCandidate(file) || file.SizeBytes > maxSemanticAnalysisBytes):
		return "Not eligible for semantic source analysis."
	case stage == AnalysisStagePerformance && file.SizeBytes > project.PerformanceMaxSourceBytes:
		return "Source exceeds analyzer size limits."
	case (stage == AnalysisStageSecurityRules || stage == AnalysisStageSecurityAI) && file.SizeBytes > project.SecurityMaxSourceBytes:
		return "Source exceeds analyzer size limits."
	case stage == AnalysisStageSecurityRules && file.Language != "Go":
		return "Passive security rules require a Go source file."
	}
	return ""
}

func (s *Service) planAnalysisStage(analysis project.Analysis, file project.IndexFile, policy *project.ContextPolicy, stage AnalysisStage, request AnalysisPreviewRequest, providers []AnalysisProviderRequirement) (AnalysisStagePlan, error) {
	var err error
	reason := analysisStageExclusion(file, stage)
	plan := AnalysisStagePlan{Stage: stage, Eligible: reason == "", Reason: reason}
	if stage != AnalysisStageSecurityRules {
		provider := providers[1]
		runtime := s.runtimes.analyze
		if stage == AnalysisStageSemantic {
			provider = providers[0]
			runtime = s.runtimes.bug
		}
		plan.ProviderID = provider.ID
		if plan.Eligible && !s.analysisStageModelAvailable(stage) {
			plan.Reason = "The model for this stage is not configured."
		}
		plan.MaxModelRequests = min(request.Limits.MaxAttemptsPerStage, runtime.effective.MaxRetries+1)
	}
	if plan.Eligible {
		plan.Cached, _, _, err = s.analysisStageCacheState(analysis, file, policy, stage)
		if err != nil {
			return AnalysisStagePlan{}, err
		}
		if request.Refresh && stage != AnalysisStageSecurityRules {
			plan.Cached = false
		}
	}
	if !plan.Eligible || plan.Cached || plan.Reason == "The model for this stage is not configured." {
		plan.MaxModelRequests = 0
	}
	return plan, nil
}

func (s *Service) applyAnalysisResumeBudget(preview *AnalysisRunPreview, request AnalysisPreviewRequest) error {
	run := s.analysisRun.run
	if run == nil || run.Identity != *request.ResumeRun || run.Identity.AnalysisQueueIdentity != preview.Identity || run.Plan.Limits != request.Limits || run.Plan.Refresh != request.Refresh {
		return project.ErrRevisionConflict
	}
	for i := range preview.Files {
		for j := range preview.Files[i].Stages {
			plan := &preview.Files[i].Stages[j]
			progress := run.Files[i].Stages[j]
			if !analysisStageNeedsWork(progress.Status) {
				plan.MaxModelRequests = 0
				continue
			}
			plan.MaxModelRequests = min(plan.MaxModelRequests, max(0, request.Limits.MaxAttemptsPerStage-progress.Attempts))
		}
	}
	return nil
}

func countAnalysisPreviewRequests(preview *AnalysisRunPreview) {
	for _, file := range preview.Files {
		for _, stage := range file.Stages {
			if stage.MaxModelRequests > 0 {
				preview.ExpectedModelRequests++
				preview.MaxModelRequests += stage.MaxModelRequests
				if stage.Stage == AnalysisStageSecurityAI {
					preview.SecurityReviewIntentRequired = true
				}
			}
		}
	}
}

func validateAnalysisInventory(ctx context.Context, root string, plan *AnalysisRunPreview) error {
	paths, err := project.WalkProjectFiles(ctx, project.ProjectWalkOptions{Root: root, IncludeSymlinkFiles: true, MaxFiles: maxAnalysisInventoryFiles})
	if err != nil {
		return err
	}
	captured := make(map[string]bool, len(plan.Files)+len(plan.Excluded))
	for _, file := range plan.Files {
		captured[file.Path] = true
	}
	for _, file := range plan.Excluded {
		captured[file.Path] = true
	}
	if len(paths) != len(captured) {
		return project.ErrRevisionConflict
	}
	for _, path := range paths {
		if !captured[path] {
			return project.ErrRevisionConflict
		}
	}
	return nil
}
