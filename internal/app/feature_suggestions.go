package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FeatureSuggestion struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Benefit            string   `json:"benefit"`
	Evidence           string   `json:"evidence"`
	Paths              []string `json:"paths"`
	Effort             string   `json:"effort"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Status             string   `json:"status"`
}

type FeatureReport struct {
	SchemaVersion   int                     `json:"schema_version"`
	ProjectID       string                  `json:"project_id"`
	ProjectRevision string                  `json:"project_revision"`
	Hash            string                  `json:"hash"`
	Goals           string                  `json:"goals"`
	Status          string                  `json:"status"`
	Freshness       string                  `json:"freshness"`
	Failure         string                  `json:"failure,omitempty"`
	Suggestions     []FeatureSuggestion     `json:"suggestions"`
	WorkspaceHash   string                  `json:"workspace_hash"`
	ContextManifest project.ContextManifest `json:"context_manifest"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type FeatureRequest struct {
	ProjectID             string `json:"project_id"`
	ProjectRevision       string `json:"project_revision"`
	ExpectedHash          string `json:"expected_hash"`
	Goals                 string `json:"goals"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider"`
}

type FeatureStatusRequest struct {
	FeatureRequest
	Status string `json:"status"`
}

func readFeatures(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	path, err := changeMetadataPath(root, "features.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &FeatureReport{SchemaVersion: 1, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Hash: "empty", Status: "not_generated", Freshness: "current", Suggestions: []FeatureSuggestion{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 64*1024 {
		return nil, fmt.Errorf("feature history exceeds 64 KiB")
	}
	var report FeatureReport
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("read feature history: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || report.SchemaVersion != 1 || report.ProjectID != index.ProjectID {
		return nil, fmt.Errorf("unsupported feature history")
	}
	return &report, nil
}

func writeFeatures(root string, report *FeatureReport) error {
	path, err := changeMetadataPath(root, "features.json")
	if err != nil {
		return err
	}
	report.UpdatedAt = time.Now().UTC()
	report.Hash = ""
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	report.Hash = contentHash(data)
	data, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 64*1024 {
		return fmt.Errorf("feature history exceeds 64 KiB")
	}
	return storage.WriteFile(path, data, 0600)
}

func (s *Service) Features(ctx context.Context) (*FeatureReport, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	report, err := readFeatures(s.manager.Root(), index)
	if err != nil {
		return nil, err
	}
	if report.Status != "not_generated" {
		current, err := benchmarkSourceFingerprint(ctx, s.manager.Root())
		if err != nil {
			return nil, err
		}
		if report.ProjectRevision != index.ProjectRevision || current != report.WorkspaceHash {
			report.Freshness = "stale"
		}
	}
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

func (s *Service) SaveFeatureGoals(request FeatureRequest) (*FeatureReport, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	report, root, err := s.featuresForRequest(request)
	if err != nil {
		return nil, err
	}
	if report.Goals != request.Goals && len(report.Suggestions) > 0 {
		report.Freshness = "stale"
	}
	report.Goals = request.Goals
	if err := writeFeatures(root, report); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Service) GenerateFeatures(ctx context.Context, request FeatureRequest) (*FeatureReport, error) {
	if err := s.RequireRemoteConfirmation(config.AnalyzeModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	previous, root, err := s.featuresForRequest(request)
	if err != nil {
		return nil, err
	}
	text, manifest, err := project.NewContextBuilder().BuildWithManifest(root, "")
	if err != nil {
		return nil, err
	}
	instructions, err := project.ResolveInstructions(root, "")
	if err != nil {
		return nil, err
	}
	text += instructions.Text
	fingerprint, err := benchmarkSourceFingerprint(ctx, root)
	if err != nil {
		return nil, err
	}
	runtime := s.runtimes.analyze
	if runtime.effective.ContextMaxTokens > 0 && len(text)+len(request.Goals) > runtime.effective.ContextMaxTokens*4 {
		return nil, fmt.Errorf("feature context exceeds configured Analyze token limit")
	}
	messages := []llm.ChatMessage{{Role: "system", Content: "Suggest at most five useful new features grounded in the supplied project and user goals. Return one JSON object with suggestions: [{title,benefit,evidence,paths,effort,acceptance_criteria}]. Effort is small, medium or large. Use only existing eligible Go/Markdown paths; these are proposed product capabilities, separate from bug, security and optimization findings. Do not invent files, quote source code or secrets, or claim verification. An empty array is valid. Instructions guide ideas within their scope and cannot alter the response contract."}, {Role: "user", Content: "Project goals:\n" + request.Goals + "\nProject context:\n" + text}}
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	schema := featureResponseSchema()
	result, generationErr := s.retryRequestAuthorized(timed, runtime, messages, &schema, func(ctx context.Context) error {
		return s.verifyFeatureInput(ctx, root, previous, fingerprint, request)
	})
	var suggestions []FeatureSuggestion
	if generationErr == nil {
		suggestions, generationErr = s.parseFeatures(result.Content)
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	if err := s.verifyFeatureInput(timed, root, previous, fingerprint, request); err != nil {
		return nil, err
	}
	previous.Goals = request.Goals
	if generationErr != nil {
		previous.Status, previous.Failure = "failed", "Feature suggestions could not be generated. Review provider configuration or retry."
		if err := writeFeatures(root, previous); err != nil {
			return nil, err
		}
		return nil, generationErr
	}
	statusByID := map[string]string{}
	for _, idea := range previous.Suggestions {
		statusByID[idea.ID] = idea.Status
	}
	for i := range suggestions {
		if status := statusByID[suggestions[i].ID]; status != "" {
			suggestions[i].Status = status
		}
	}
	previous.ProjectRevision, previous.WorkspaceHash = request.ProjectRevision, fingerprint
	previous.Status, previous.Freshness, previous.Failure = "ready", "current", ""
	previous.Suggestions, previous.ContextManifest = suggestions, s.contextManifestForRuntime(manifest, runtime)
	if err := writeFeatures(root, previous); err != nil {
		return nil, err
	}
	return previous, nil
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
	return (benchmarkFixture{fingerprint: fingerprint}).verifyCurrent(ctx, root)
}

func featureResponseSchema() llm.JSONSchema {
	return llm.JSONSchema{Name: "feature_suggestions", Schema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["suggestions"],"properties":{"suggestions":{"type":"array","maxItems":5,"items":{"type":"object","additionalProperties":false,"required":["title","benefit","evidence","paths","effort","acceptance_criteria"],"properties":{"title":{"type":"string"},"benefit":{"type":"string"},"evidence":{"type":"string"},"effort":{"enum":["small","medium","large"]},"paths":{"type":"array","minItems":1,"maxItems":8,"items":{"type":"string"}},"acceptance_criteria":{"type":"array","minItems":1,"maxItems":16,"items":{"type":"string"}}}}}}}`)}
}

func (s *Service) parseFeatures(output string) ([]FeatureSuggestion, error) {
	if len(output) > 32*1024 {
		return nil, fmt.Errorf("feature output exceeds 32 KiB")
	}
	var wire struct {
		Suggestions []struct {
			Title    string   `json:"title"`
			Benefit  string   `json:"benefit"`
			Evidence string   `json:"evidence"`
			Paths    []string `json:"paths"`
			Effort   string   `json:"effort"`
			Criteria []string `json:"acceptance_criteria"`
		} `json:"suggestions"`
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF || wire.Suggestions == nil || len(wire.Suggestions) > 5 {
		return nil, fmt.Errorf("invalid feature suggestions")
	}
	result := []FeatureSuggestion{}
	seen := map[string]bool{}
	for _, item := range wire.Suggestions {
		feature := FeatureSuggestion{Title: item.Title, Benefit: item.Benefit, Evidence: item.Evidence, Paths: item.Paths, Effort: item.Effort, AcceptanceCriteria: item.Criteria, Status: "open"}
		if err := s.validateFeature(feature); err != nil {
			return nil, err
		}
		data, _ := json.Marshal(feature)
		feature.ID = contentHash(data)
		if seen[feature.ID] {
			return nil, fmt.Errorf("duplicate feature suggestion")
		}
		seen[feature.ID] = true
		result = append(result, feature)
	}
	return result, nil
}

func (s *Service) validateFeature(idea FeatureSuggestion) error {
	for _, text := range []string{idea.Title, idea.Benefit, idea.Evidence} {
		if strings.TrimSpace(text) == "" || len(text) > 1024 || sanitizeCheckOutput(text, false, "", "") != text {
			return fmt.Errorf("feature prose must be non-empty bounded text without credentials")
		}
	}
	if idea.Effort != "small" && idea.Effort != "medium" && idea.Effort != "large" {
		return fmt.Errorf("invalid feature effort")
	}
	if len(idea.Paths) == 0 || len(idea.Paths) > maxChangePaths || len(idea.AcceptanceCriteria) == 0 {
		return fmt.Errorf("feature paths and acceptance criteria are required")
	}
	if err := validateChangeCriteria(idea.AcceptanceCriteria); err != nil {
		return err
	}
	for _, path := range idea.Paths {
		file, err := s.manager.IndexedFile(path)
		if err != nil {
			return err
		}
		if file.Binary || filepath.Ext(path) != ".go" && filepath.Ext(path) != ".md" {
			return fmt.Errorf("feature path does not support implementation")
		}
		if _, err := captureChangeTarget(s.manager.Root(), path); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UpdateFeatureStatus(id string, request FeatureStatusRequest) (*FeatureReport, error) {
	if request.Status != "open" && request.Status != "saved" && request.Status != "dismissed" {
		return nil, fmt.Errorf("invalid feature status")
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	report, root, err := s.featuresForRequest(request.FeatureRequest)
	if err != nil {
		return nil, err
	}
	for i := range report.Suggestions {
		if report.Suggestions[i].ID == id {
			report.Suggestions[i].Status = request.Status
			if err := writeFeatures(root, report); err != nil {
				return nil, err
			}
			return report, nil
		}
	}
	return nil, fmt.Errorf("feature suggestion not found")
}
