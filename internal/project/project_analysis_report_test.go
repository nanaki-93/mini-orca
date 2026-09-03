package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

type projectAnalysisFixtureClient struct {
	output string
	calls  int
}

func (c *projectAnalysisFixtureClient) Chat(context.Context, []llm.ChatMessage) (*llm.ChatResponse, error) {
	c.calls++
	return &llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: c.output}}}}, nil
}

func TestParseProjectAnalysisResponseRejectsMalformedUnknownAndOversizedOutput(t *testing.T) {
	valid := `{"purpose":"Explains the project.","architecture":"One Go daemon.","components":["daemon"],"entry_points":["main.main"],"flows":["request to service"],"risks":[{"severity":"HIGH","summary":"Add a regression test."}],"next_steps":["Document the API"]}`
	if report, err := parseProjectAnalysisResponse(valid); err != nil || report.Purpose != "Explains the project." || len(report.Risks) != 1 || report.Risks[0].Severity != "high" {
		t.Fatalf("parsed report = %+v, %v", report, err)
	}
	for _, output := range []string{
		"not JSON",
		`{"purpose":"x","architecture":"y","components":[],"entry_points":[],"flows":[],"risks":[],"next_steps":[],"unexpected":true}`,
		`{"purpose":"","architecture":"y","components":[],"entry_points":[],"flows":[],"risks":[],"next_steps":[]}`,
		strings.Repeat("x", maxProjectAnalysisBytes+1),
	} {
		if _, err := parseProjectAnalysisResponse(output); err == nil {
			t.Fatalf("expected invalid project analysis output to fail: %q", output[:min(len(output), 32)])
		}
	}
}

func TestProjectAnalysisReportPersistsAndInvalidatesChangedInputs(t *testing.T) {
	root := t.TempDir()
	report := newProjectAnalysisReportWithProvenance("project", "revision-one", "model-a", "analysis", "analysis", "", "")
	report.Purpose = "Explains the project."
	report.Architecture = "One daemon."
	if err := StoreProjectAnalysisReport(root, report); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: "project", ProjectRevision: "revision-one", Model: "model-a", Profile: "analysis", Scope: "analysis", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || loaded == nil || loaded.Status != ProjectAnalysisStatusFresh {
		t.Fatalf("fresh stored report = %+v, %v", loaded, err)
	}
	stale, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: "project", ProjectRevision: "revision-two", Model: "model-a", Profile: "analysis", Scope: "analysis", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || stale == nil || stale.Status != ProjectAnalysisStatusStale {
		t.Fatalf("changed revision report = %+v, %v", stale, err)
	}
	if _, err := os.Stat(filepath.Join(root, projectAnalysisReportPath)); err != nil {
		t.Fatalf("structured report was not persisted: %v", err)
	}
}

func TestProjectAnalysisReportInvalidatesChangedScopeProvider(t *testing.T) {
	root := t.TempDir()
	report := newProjectAnalysisReportWithProvenance("project", "revision", "model", "analyze", "analyze", "https://provider-one.example", "high")
	report.Purpose = "Purpose"
	report.Architecture = "Architecture"
	if err := StoreProjectAnalysisReport(root, report); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: "project", ProjectRevision: "revision", Model: "model", Profile: "analyze", Scope: "analyze", ProviderOrigin: "https://provider-two.example", ReasoningEffort: "high", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || loaded == nil || loaded.Status != ProjectAnalysisStatusStale {
		t.Fatalf("provider-changed report = %+v, %v", loaded, err)
	}
}

func TestProjectAnalysisReportInvalidatesChangedReasoningEffort(t *testing.T) {
	root := t.TempDir()
	report := newProjectAnalysisReportWithProvenance("project", "revision", "model", "analyze", "analyze", "https://provider.example", "high")
	report.Purpose = "Purpose"
	report.Architecture = "Architecture"
	if err := StoreProjectAnalysisReport(root, report); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: "project", ProjectRevision: "revision", Model: "model", Profile: "analyze", Scope: "analyze", ProviderOrigin: "https://provider.example", ReasoningEffort: "low", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || loaded == nil || loaded.Status != ProjectAnalysisStatusStale {
		t.Fatalf("reasoning-effort-changed report = %+v, %v", loaded, err)
	}
}

func TestAnalyzerFailureKeepsInventoryAndWritesCanonicalReport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	analysis, err := NewAnalyzerWithProvenance(&projectAnalysisFixtureClient{output: "not JSON"}, "fixture-model", "analysis", "", "").Analyze(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.AIStatus != ProjectAnalysisStatusFailed || analysis.FileCount != 1 || analysis.SourceFileCount != 1 || analysis.Report.Failure == "" {
		t.Fatalf("failed analysis = %+v", analysis)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "analysis.md")); !os.IsNotExist(err) {
		t.Fatalf("legacy markdown report was generated: %v", err)
	}
	stored, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Model: "fixture-model", Profile: "analysis", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || stored == nil || stored.Status != ProjectAnalysisStatusFailed {
		t.Fatalf("stored failed report = %+v, %v", stored, err)
	}
}

func TestAnalyzerRestoreLoadsStoredReportWithoutContactingModel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	client := &projectAnalysisFixtureClient{output: `{"purpose":"Restored project.","architecture":"One package.","components":[],"entry_points":[],"flows":[],"risks":[],"next_steps":[]}`}
	analyzer := NewAnalyzerWithProvenance(client, "fixture-model", "analysis", "", "")
	imported, err := analyzer.Analyze(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	client.calls = 0

	restored, err := analyzer.Restore(root)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 0 {
		t.Fatalf("restore contacted model %d times", client.calls)
	}
	if restored.ProjectID != imported.ProjectID || restored.Summary != imported.Summary || restored.AIStatus != ProjectAnalysisStatusFresh {
		t.Fatalf("restored analysis = %+v", restored)
	}
}

func TestProjectAnalysisReportRequiresGeneratedTime(t *testing.T) {
	report := newProjectAnalysisReportWithProvenance("project", "revision", "model", "analysis", "analysis", "", "")
	report.Purpose = "Purpose"
	report.Architecture = "Architecture"
	report.GeneratedAt = time.Time{}
	if validStoredProjectAnalysisReport(report) {
		t.Fatal("report without generated time is valid")
	}
}
