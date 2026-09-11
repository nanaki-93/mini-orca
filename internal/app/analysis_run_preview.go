package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

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
	preview := &AnalysisRunPreview{SchemaVersion: AnalysisRunSchemaVersion, Scope: AnalysisRunScopeProject, Refresh: request.Refresh, Limits: request.Limits,
		Identity: AnalysisQueueIdentity{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, PolicyFingerprint: policy.Version(), ProviderFingerprint: fingerprint},
		Files:    []AnalysisPlannedFile{}, Excluded: []AnalysisExcludedFile{}, Providers: providers}
	files := append([]project.IndexFile(nil), index.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	root := s.manager.Root()
	paths, err := project.WalkProjectFiles(ctx, project.ProjectWalkOptions{Root: root, IncludeSymlinkFiles: true, MaxFiles: maxAnalysisInventoryFiles})
	if err != nil {
		return nil, err
	}
	indexed := make(map[string]bool, len(files))
	for _, file := range files {
		indexed[file.Path] = true
	}
	for _, path := range paths {
		if !policy.Decide(path).Include {
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: path, Reason: "Excluded by source policy."})
		} else if !indexed[path] {
			return nil, project.ErrRevisionConflict
		}
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reason := ""
		switch {
		case !policy.Decide(file.Path).Include:
			reason = "Excluded by source policy."
		case file.Binary || file.Language == "":
			reason = "Not a supported text source file."
		case file.SizeBytes > maxSemanticAnalysisBytes && file.SizeBytes > project.PerformanceMaxSourceBytes && file.SizeBytes > project.SecurityMaxSourceBytes:
			reason = "Source exceeds analyzer size limits."
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
			return nil, err
		}
		if info.ContentHash != file.ContentHash || info.Binary {
			return nil, project.ErrRevisionConflict
		}
		planned := AnalysisPlannedFile{AnalysisFileIdentity: AnalysisFileIdentity{Path: file.Path, ContentHash: file.ContentHash, Language: file.Language}, SizeBytes: file.SizeBytes, Stages: []AnalysisStagePlan{}}
		for _, stage := range analysisStages {
			plan := AnalysisStagePlan{Stage: stage, Eligible: true}
			switch {
			case stage == AnalysisStageSemantic && (!isSemanticAnalysisCandidate(file) || file.SizeBytes > maxSemanticAnalysisBytes):
				plan.Eligible = false
				plan.Reason = "Not eligible for semantic source analysis."
			case stage == AnalysisStagePerformance && file.SizeBytes > project.PerformanceMaxSourceBytes:
				plan.Eligible = false
				plan.Reason = "Source exceeds analyzer size limits."
			case (stage == AnalysisStageSecurityRules || stage == AnalysisStageSecurityAI) && file.SizeBytes > project.SecurityMaxSourceBytes:
				plan.Eligible = false
				plan.Reason = "Source exceeds analyzer size limits."
			case stage == AnalysisStageSecurityRules && file.Language != "Go":
				plan.Eligible = false
				plan.Reason = "Passive security rules require a Go source file."
			}
			if stage != AnalysisStageSecurityRules {
				provider := providers[1]
				runtime := s.runtimes.analyze
				if stage == AnalysisStageSemantic {
					provider = providers[0]
					runtime = s.runtimes.bug
				}
				plan.ProviderID = provider.ID
				if runtime.client == nil {
					plan.Reason = "The model for this stage is not configured."
				}
				plan.MaxModelRequests = min(request.Limits.MaxAttemptsPerStage, runtime.effective.MaxRetries+1)
			}
			if plan.Eligible {
				plan.Cached, err = s.analysisStageCached(*analysis, file, policy, stage)
				if err != nil {
					return nil, err
				}
				if request.Refresh && stage != AnalysisStageSecurityRules {
					plan.Cached = false
				}
			}
			if !plan.Eligible || plan.Cached || plan.Reason == "The model for this stage is not configured." {
				plan.MaxModelRequests = 0
			}
			planned.Stages = append(planned.Stages, plan)
		}
		preview.Files = append(preview.Files, planned)
	}
	sort.Slice(preview.Excluded, func(i, j int) bool { return preview.Excluded[i].Path < preview.Excluded[j].Path })
	preview.Identity.QueueID, err = analysisQueueFingerprint(preview)
	if err != nil {
		return nil, err
	}
	if request.ResumeRun != nil {
		run := s.analysisRun.run
		if run == nil || run.Identity != *request.ResumeRun || run.Identity.AnalysisQueueIdentity != preview.Identity || run.Plan.Limits != request.Limits || run.Plan.Refresh != request.Refresh {
			return nil, project.ErrRevisionConflict
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
	}
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

func (s *Service) analysisStageCached(analysis project.Analysis, file project.IndexFile, policy *project.ContextPolicy, stage AnalysisStage) (bool, error) {
	switch stage {
	case AnalysisStageSemantic:
		cache, input, err := s.fileAnalysisCacheInput(&analysis, &file, file.ContentHash)
		if err != nil {
			return false, err
		}
		report, err := cache.Load(input)
		return analysisSemanticCacheUsable(report, analysis.ProjectRevision), err
	case AnalysisStagePerformance:
		report, err := project.LoadPerformanceFileReport(s.manager.Root(), file.Path, file.ContentHash, policy)
		return analysisPerformanceCacheUsable(report, analysis, s.runtimes.analyze), err
	default:
		input := analysisSecurityCacheInput(analysis, file, s.runtimes.analyze, policy.Version())
		if stage == AnalysisStageSecurityRules {
			input = securityRulesInput(securityRulesSnapshot{analysis: analysis, file: file, policyVersion: policy.Version()})
		}
		report, err := s.loadSecurityFileReport(s.manager.Root(), input)
		return securityStageCacheUsable(report), err
	}
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
	fingerprint, err := analysisFingerprint(providers)
	if err != nil {
		return err
	}
	if root != s.manager.Root() || plan.Identity.ProjectID != analysis.ProjectID || plan.Identity.ProjectRevision != analysis.ProjectRevision || plan.Identity.PolicyFingerprint != policy.Version() || plan.Identity.ProviderFingerprint != fingerprint {
		return project.ErrRevisionConflict
	}
	if sources {
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
	}
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
