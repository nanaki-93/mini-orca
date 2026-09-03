package project

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	projectAnalysisSchemaVersion = "1"
	projectAnalysisPromptVersion = "project-analysis-v1"
	projectAnalysisReportPath    = ".mini-orca/project-analysis.json"
	maxProjectAnalysisBytes      = 64 * 1024
	maxProjectAnalysisItems      = 32
	maxProjectAnalysisItemBytes  = 2048

	ProjectAnalysisStatusFresh       = "fresh"
	ProjectAnalysisStatusFailed      = "failed"
	ProjectAnalysisStatusUnavailable = "unavailable"
	ProjectAnalysisStatusStale       = "stale"
)

// ProjectAnalysisRisk is a bounded model interpretation, not a verified issue.
type ProjectAnalysisRisk struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

// ProjectAnalysisReport is the authoritative, persisted interpretation of one
// project revision.
type ProjectAnalysisReport struct {
	SchemaVersion   string                `json:"schema_version"`
	ProjectID       string                `json:"project_id"`
	ProjectRevision string                `json:"project_revision"`
	Purpose         string                `json:"purpose,omitempty"`
	Architecture    string                `json:"architecture,omitempty"`
	Components      []string              `json:"components"`
	EntryPoints     []string              `json:"entry_points"`
	Flows           []string              `json:"flows"`
	Risks           []ProjectAnalysisRisk `json:"risks"`
	NextSteps       []string              `json:"next_steps"`
	Status          string                `json:"status"`
	Failure         string                `json:"failure,omitempty"`
	Model           string                `json:"model"`
	ConfiguredModel string                `json:"configured_model,omitempty"`
	Profile         string                `json:"profile"`
	Scope           string                `json:"scope,omitempty"`
	ProviderOrigin  string                `json:"provider_origin,omitempty"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
	PromptVersion   string                `json:"prompt_version"`
	GeneratedAt     time.Time             `json:"generated_at"`
}

// ProjectAnalysisInput identifies the model and project state a persisted
// report must match before it may be presented as fresh.
type ProjectAnalysisInput struct {
	ProjectID       string
	ProjectRevision string
	Model           string
	Profile         string
	Scope           string
	ProviderOrigin  string
	ReasoningEffort string
	PromptVersion   string
}

type projectAnalysisResponse struct {
	Purpose      string                `json:"purpose"`
	Architecture string                `json:"architecture"`
	Components   []string              `json:"components"`
	EntryPoints  []string              `json:"entry_points"`
	Flows        []string              `json:"flows"`
	Risks        []ProjectAnalysisRisk `json:"risks"`
	NextSteps    []string              `json:"next_steps"`
}

func newProjectAnalysisReportWithProvenance(projectID, revision, model, profile, scope, providerOrigin, reasoningEffort string) ProjectAnalysisReport {
	return ProjectAnalysisReport{
		SchemaVersion: projectAnalysisSchemaVersion,
		ProjectID:     projectID, ProjectRevision: revision,
		Components: []string{}, EntryPoints: []string{}, Flows: []string{}, Risks: []ProjectAnalysisRisk{}, NextSteps: []string{},
		Status: ProjectAnalysisStatusFresh, Model: model, ConfiguredModel: model, Profile: profile, Scope: scope, ProviderOrigin: providerOrigin, ReasoningEffort: reasoningEffort, PromptVersion: projectAnalysisPromptVersion,
		GeneratedAt: time.Now().UTC(),
	}
}

func projectAnalysisMessages(contextText string) []llm.ChatMessage {
	return []llm.ChatMessage{
		{Role: "system", Content: "You are a software architect. Analyze only the supplied project facts and context. Return exactly one JSON object with these fields and no Markdown or prose: purpose (non-empty string), architecture (non-empty string), components (string array), entry_points (string array), flows (string array), risks ({severity,summary} array), next_steps (string array). Do not invent files or dependencies. Risks are suggestions, not verified findings."},
		{Role: "user", Content: contextText},
	}
}

func parseProjectAnalysisResponse(output string) (projectAnalysisResponse, error) {
	var response projectAnalysisResponse
	if len(output) == 0 || len(output) > maxProjectAnalysisBytes {
		return response, fmt.Errorf("project analysis response is empty or too large")
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return response, fmt.Errorf("parse project analysis JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return response, fmt.Errorf("project analysis JSON must contain one object")
	}
	response.Purpose = strings.TrimSpace(response.Purpose)
	response.Architecture = strings.TrimSpace(response.Architecture)
	if response.Purpose == "" || response.Architecture == "" || len(response.Purpose) > maxProjectAnalysisItemBytes || len(response.Architecture) > maxProjectAnalysisItemBytes {
		return response, fmt.Errorf("project analysis JSON requires purpose and architecture")
	}
	if err := validateProjectAnalysisStrings("components", response.Components); err != nil {
		return response, err
	}
	if err := validateProjectAnalysisStrings("entry points", response.EntryPoints); err != nil {
		return response, err
	}
	if err := validateProjectAnalysisStrings("flows", response.Flows); err != nil {
		return response, err
	}
	if err := validateProjectAnalysisStrings("next steps", response.NextSteps); err != nil {
		return response, err
	}
	if len(response.Risks) > maxProjectAnalysisItems {
		return response, fmt.Errorf("project analysis risks exceed limit")
	}
	for index := range response.Risks {
		risk := &response.Risks[index]
		risk.Severity = strings.ToLower(strings.TrimSpace(risk.Severity))
		risk.Summary = strings.TrimSpace(risk.Summary)
		if !validProjectAnalysisSeverity(risk.Severity) || risk.Summary == "" || len(risk.Summary) > maxProjectAnalysisItemBytes {
			return response, fmt.Errorf("project analysis contains an invalid risk")
		}
	}
	return response, nil
}

func validateProjectAnalysisStrings(name string, values []string) error {
	if len(values) > maxProjectAnalysisItems {
		return fmt.Errorf("project analysis %s exceed limit", name)
	}
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
		if values[index] == "" || len(values[index]) > maxProjectAnalysisItemBytes {
			return fmt.Errorf("project analysis contains an invalid %s value", name)
		}
	}
	return nil
}

func validProjectAnalysisSeverity(value string) bool {
	return value == "low" || value == "medium" || value == "high"
}

// LoadProjectAnalysisReport returns a stored report. A valid report with
// changed project, model, profile, or prompt inputs is marked stale.
func LoadProjectAnalysisReport(root string, input ProjectAnalysisInput) (*ProjectAnalysisReport, error) {
	data, err := os.ReadFile(filepath.Join(root, projectAnalysisReportPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project analysis report: %w", err)
	}
	var report ProjectAnalysisReport
	if err := json.Unmarshal(data, &report); err != nil || !validStoredProjectAnalysisReport(report) {
		return nil, fmt.Errorf("parse project analysis report")
	}
	if !projectAnalysisReportMatches(report, input) && report.Status == ProjectAnalysisStatusFresh {
		report.Status = ProjectAnalysisStatusStale
	}
	return cloneProjectAnalysisReport(&report), nil
}

// StoreProjectAnalysisReport atomically replaces the report for one analysis
// outcome. It persists unavailable and failed states with their truthful reason.
func StoreProjectAnalysisReport(root string, report ProjectAnalysisReport) error {
	if !validStoredProjectAnalysisReport(report) {
		return fmt.Errorf("project analysis report is invalid")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project analysis report: %w", err)
	}
	if err := storage.WriteFile(filepath.Join(root, projectAnalysisReportPath), data, 0600); err != nil {
		return fmt.Errorf("store project analysis report: %w", err)
	}
	return nil
}

func validStoredProjectAnalysisReport(report ProjectAnalysisReport) bool {
	if report.SchemaVersion != projectAnalysisSchemaVersion || report.ProjectID == "" || report.ProjectRevision == "" || report.Profile == "" || report.PromptVersion == "" || report.GeneratedAt.IsZero() {
		return false
	}
	switch report.Status {
	case ProjectAnalysisStatusFresh:
		return report.Purpose != "" && report.Architecture != "" && report.Failure == ""
	case ProjectAnalysisStatusFailed, ProjectAnalysisStatusUnavailable:
		return report.Failure != ""
	case ProjectAnalysisStatusStale:
		return true
	default:
		return false
	}
}

func projectAnalysisReportMatches(report ProjectAnalysisReport, input ProjectAnalysisInput) bool {
	scope := input.Scope
	if scope == "" {
		scope = input.Profile
	}
	configuredModel := report.ConfiguredModel
	if configuredModel == "" {
		configuredModel = report.Model
	}
	return report.ProjectID == input.ProjectID && report.ProjectRevision == input.ProjectRevision && configuredModel == input.Model && report.Profile == input.Profile && report.Scope == scope && report.ProviderOrigin == input.ProviderOrigin && report.ReasoningEffort == input.ReasoningEffort && report.PromptVersion == input.PromptVersion
}

func cloneProjectAnalysisReport(source *ProjectAnalysisReport) *ProjectAnalysisReport {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Components = append([]string(nil), source.Components...)
	copy.EntryPoints = append([]string(nil), source.EntryPoints...)
	copy.Flows = append([]string(nil), source.Flows...)
	copy.Risks = append([]ProjectAnalysisRisk(nil), source.Risks...)
	copy.NextSteps = append([]string(nil), source.NextSteps...)
	return &copy
}

func projectAnalysisSummary(report ProjectAnalysisReport) string {
	if report.Status != ProjectAnalysisStatusFresh {
		return report.Failure
	}
	return report.Purpose + "\n\n" + report.Architecture
}
