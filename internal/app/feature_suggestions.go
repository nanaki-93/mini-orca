package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/features"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	featureSuggestionsPromptVersion = features.PromptVersion
	featureSuggestionsSystemPrompt  = features.SystemPrompt
	legacyFeatureSchemaVersion      = features.LegacySchemaVersion
	featureHistorySchemaVersion     = features.HistorySchemaVersion
	maxFeatureHistoryBytes          = features.MaxHistoryBytes
	maxExistingFeatureIdeasBytes    = features.MaxExistingIdeasBytes
	legacyFeatureGenerationID       = features.LegacyGenerationID
)

var (
	errInvalidFeatureResponse    = features.ErrInvalidResponse
	errFeatureHistoryFull        = features.ErrHistoryFull
	errUnsupportedFeatureHistory = features.ErrUnsupportedHistory
)

type FeatureSuggestion = features.FeatureSuggestion
type FeatureReport = features.FeatureReport
type FeatureGenerateRequest = features.FeatureGenerateRequest
type FeatureRequest = features.FeatureRequest
type FeatureStatusRequest = features.FeatureStatusRequest
type FeatureModelSummary = features.FeatureModelSummary
type FeatureGeneration = features.FeatureGeneration
type FeatureGenerationOutcome = features.FeatureGenerationOutcome

func readFeatures(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	return features.Read(root, index)
}

func writeFeatures(root string, report *FeatureReport) error {
	return features.Write(root, report)
}

func clearFeatureSuggestions(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	return features.Clear(root, index)
}

func existingFeatureIdeas(suggestions []FeatureSuggestion) string {
	return features.ExistingIdeas(suggestions)
}

func featureModelSummary(runtime modelRuntime) *FeatureModelSummary {
	return &FeatureModelSummary{
		Profile:        runtime.effective.Profile,
		Model:          runtime.effective.Model,
		ProviderOrigin: runtime.effective.ProviderOrigin,
		RemoteProvider: runtime.effective.RemoteProvider,
	}
}

func featureGenerationTimeout(runtime modelRuntime) time.Duration {
	return max(10*time.Minute, duration(runtime.effective.Timeout))
}

func (s *Service) Features(ctx context.Context) (*FeatureReport, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	root := s.manager.Root()
	report, err := readFeatures(root, index)
	if err != nil {
		return nil, err
	}
	workspace, err := features.WorkspaceHash(ctx, root, report)
	if err != nil {
		return nil, err
	}
	features.DeriveFreshness(report, index.ProjectRevision, workspace)
	return report, nil
}

func (s *Service) featuresForRequest(request FeatureRequest) (*FeatureReport, string, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, "", err
	}
	if index.ProjectID != request.ProjectID || index.ProjectRevision != request.ProjectRevision {
		return nil, "", project.ErrRevisionConflict
	}
	if len(request.Goals) > 4096 {
		return nil, "", fmt.Errorf("project goals exceed 4096 bytes")
	}
	root := s.manager.Root()
	report, err := readFeatures(root, index)
	if err != nil {
		return nil, root, err
	}
	if request.ExpectedHash != report.Hash {
		return nil, root, project.ErrRevisionConflict
	}
	return report, root, nil
}

// updateFeatures applies one guarded metadata change. The workspace is
// fingerprinted before writing, so deriving freshness cannot fail after a write.
func (s *Service) updateFeatures(ctx context.Context, request FeatureRequest, change func(*FeatureReport) error) (*FeatureReport, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	report, root, err := s.featuresForRequest(request)
	if err != nil {
		return nil, err
	}
	workspace, err := features.WorkspaceHash(ctx, root, report)
	if err != nil {
		return nil, err
	}
	if err := change(report); err != nil {
		return nil, err
	}
	if err := writeFeatures(root, report); err != nil {
		return nil, err
	}
	features.DeriveFreshness(report, request.ProjectRevision, workspace)
	return report, nil
}

func (s *Service) SaveFeatureGoals(ctx context.Context, request FeatureRequest) (*FeatureReport, error) {
	return s.updateFeatures(ctx, request, func(report *FeatureReport) error {
		if report.Goals != request.Goals && len(report.Suggestions) > 0 {
			report.Freshness = "stale"
		}
		report.Goals = request.Goals
		return nil
	})
}

func (s *Service) GenerateFeatures(ctx context.Context, request FeatureGenerateRequest) (*FeatureReport, error) {
	profile := request.Profile
	if profile == "" {
		profile = "analyze"
	}
	if !validModelSelection(profile) {
		return nil, fmt.Errorf("invalid feature profile")
	}
	if err := s.validateModelSelections(ctx, []string{profile}); err != nil {
		return nil, err
	}
	runtime := s.runtimeForAnalysisScope(config.AnalyzeModelScope, profile)

	var excluded []string
	if request.AnalysisSelectionID != "" {
		loaded, err := loadAnalysisSelection(s.manager.Root())
		if err != nil {
			return nil, err
		}
		fingerprint, err := analysisFingerprint(loaded)
		if err != nil {
			return nil, err
		}
		if fingerprint != request.AnalysisSelectionID {
			return nil, project.ErrRevisionConflict
		}
		excluded = loaded
	}

	return s.generateFeatures(ctx, request.FeatureRequest, runtime, excluded, featureGenerationAuthority{Publish: func(write func() error) error {
		s.changesMu.Lock()
		defer s.changesMu.Unlock()
		return write()
	}})
}

type featureGenerationAuthority struct {
	BeforeAttempt func(context.Context) error
	Publish       func(func() error) error
}

// featureGeneration carries the identities captured before dispatch; publication
// requires them to be unchanged.
type featureGeneration struct {
	id          string
	request     FeatureRequest
	runtime     modelRuntime
	root        string
	previous    *FeatureReport
	messages    []llm.ChatMessage
	manifest    project.ContextManifest
	fingerprint string
}

// generateFeatures is the only feature generator. Every entry point merges new
// ideas into the stored history; a failed or rejected attempt keeps prior ideas.
func (s *Service) generateFeatures(ctx context.Context, request FeatureRequest, runtime modelRuntime, excluded []string, authority featureGenerationAuthority) (*FeatureReport, error) {
	if err := requireModelRuntimeConfirmation(runtime, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	generation, err := s.prepareFeatureGeneration(ctx, request, runtime, excluded)
	if err != nil {
		return nil, err
	}
	return runKnowledge(ctx, s, runtime, agentruntime.KnowledgeFeatures, func(ctx context.Context) (*FeatureReport, string, error) {
		report, err := s.executeFeatureGeneration(ctx, generation, excluded, authority)
		return report, "completed", err
	})
}

func (s *Service) executeFeatureGeneration(ctx context.Context, generation *featureGeneration, excluded []string, authority featureGenerationAuthority) (*FeatureReport, error) {
	runtime, request := generation.runtime, generation.request
	timed, cancel := context.WithTimeout(ctx, featureGenerationTimeout(runtime))
	defer cancel()
	schema := features.ResponseSchema()
	authorize := func(ctx context.Context) error {
		if authority.BeforeAttempt != nil {
			if err := authority.BeforeAttempt(ctx); err != nil {
				return err
			}
		}
		return s.verifyFeatureInput(ctx, generation.root, generation.previous, generation.fingerprint, request)
	}
	timed, err := withProjectMemory(timed, generation.root, request.ProjectID, request.ProjectRevision, request.Goals, manifestMemorySources(generation.manifest), authorize)
	if err != nil {
		return nil, err
	}
	result, generationErr := s.retryRequestAuthorized(timed, runtime, generation.messages, &schema, authorize)
	var suggestions []FeatureSuggestion
	if generationErr == nil {
		suggestions, generationErr = s.parseFeaturesForSelection(result.Content, excluded)
		if generationErr != nil {
			generationErr = fmt.Errorf("%w: %w", errInvalidFeatureResponse, generationErr)
		}
	}
	var published *FeatureReport
	err = authority.Publish(func() error {
		var err error
		published, err = s.publishFeatureGeneration(timed, generation, suggestions, generationErr)
		return err
	})
	if err != nil && generationErr != nil && errors.Is(err, errFeatureHistoryFull) {
		// A full history cannot record the failure status; report the actual failure.
		return nil, generationErr
	}
	if err != nil {
		return nil, err
	}
	if generationErr != nil {
		return nil, generationErr
	}
	return published, nil
}

func (s *Service) prepareFeatureGeneration(ctx context.Context, request FeatureRequest, runtime modelRuntime, excluded []string) (*featureGeneration, error) {
	previous, root, err := s.featuresForRequest(request)
	if err != nil {
		return nil, err
	}
	id, err := features.NewGenerationID(previous.Generations)
	if err != nil {
		return nil, err
	}
	text, manifest, fingerprint, err := featureContext(ctx, root, excluded)
	if err != nil {
		return nil, err
	}
	existing := existingFeatureIdeas(previous.Suggestions)
	if limit := runtime.effective.ContextMaxTokens; limit > 0 && len(text)+len(request.Goals)+len(existing) > limit*4 {
		return nil, fmt.Errorf("feature context, goals and existing ideas exceed the %d-token context limit configured for feature model %s (%s profile)",
			limit, runtime.effective.Model, runtime.effective.Profile)
	}
	messages := []llm.ChatMessage{{Role: "system", Content: featureSuggestionsSystemPrompt},
		{Role: "user", Content: "Project goals:\n" + request.Goals + "\n" + existing + "Project context:\n" + text}}
	return &featureGeneration{id: id, request: request, runtime: runtime, root: root, previous: previous, messages: messages, manifest: manifest, fingerprint: fingerprint}, nil
}

// publishFeatureGeneration runs under the caller's publication authority. It
// writes a new report value, so a rejected write leaves the captured report intact.
func (s *Service) publishFeatureGeneration(ctx context.Context, generation *featureGeneration, suggestions []FeatureSuggestion, generationErr error) (*FeatureReport, error) {
	request, previous := generation.request, generation.previous
	if err := s.verifyFeatureInput(ctx, generation.root, previous, generation.fingerprint, request); err != nil {
		return nil, err
	}
	next := *previous
	next.Goals = request.Goals
	if generationErr != nil {
		next.Status, next.Failure = "failed", features.GenerationFailure(generationErr)
	} else {
		record := FeatureGeneration{ID: generation.id, GeneratedAt: time.Now().UTC(), ProjectRevision: request.ProjectRevision,
			WorkspaceHash: generation.fingerprint, GoalsHash: contentHash([]byte(request.Goals)), Model: featureModelSummary(generation.runtime)}
		merged, outcome := features.MergeSuggestions(previous.Suggestions, suggestions, record.ID)
		next.Suggestions, next.Generations, next.LastGeneration = merged, append(slices.Clone(previous.Generations), record), &outcome
		next.ProjectRevision, next.WorkspaceHash = request.ProjectRevision, generation.fingerprint
		next.Status, next.Freshness, next.Failure = "ready", "current", ""
		next.ContextManifest = s.contextManifestForRuntime(generation.manifest, generation.runtime)
	}
	if err := writeFeatures(generation.root, &next); err != nil {
		return nil, err
	}
	features.DeriveFreshness(&next, request.ProjectRevision, generation.fingerprint)
	return &next, nil
}

func featureContext(ctx context.Context, root string, excluded []string) (string, project.ContextManifest, string, error) {
	return features.BuildContext(ctx, root, excluded)
}

func (s *Service) parseFeaturesForSelection(output string, excluded []string) ([]FeatureSuggestion, error) {
	return features.ParseForSelection(output, excluded, func(path string) error {
		file, err := s.manager.IndexedFile(path)
		if err != nil {
			return err
		}
		if file.Binary {
			return fmt.Errorf("feature path does not support implementation")
		}
		_, err = captureChangeTarget(s.manager.Root(), path)
		return err
	})
}

func (s *Service) verifyFeatureInput(ctx context.Context, root string, previous *FeatureReport, fingerprint string, request FeatureRequest) error {
	if s.manager.Root() != root {
		return project.ErrRevisionConflict
	}
	current, _, err := s.featuresForRequest(request)
	if err != nil {
		return err
	}
	if current.Hash != previous.Hash {
		return project.ErrRevisionConflict
	}
	return (workspaceIdentity{fingerprint: fingerprint}).verifyCurrent(ctx, root)
}

func (s *Service) UpdateFeatureStatus(ctx context.Context, id string, request FeatureStatusRequest) (*FeatureReport, error) {
	if request.Status != "open" && request.Status != "saved" && request.Status != "dismissed" {
		return nil, fmt.Errorf("invalid feature status")
	}
	return s.updateFeatures(ctx, request.FeatureRequest, func(report *FeatureReport) error {
		for i := range report.Suggestions {
			if report.Suggestions[i].ID == id {
				report.Suggestions[i].Status = request.Status
				return nil
			}
		}
		return fmt.Errorf("feature suggestion not found")
	})
}
