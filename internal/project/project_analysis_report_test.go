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
}

func (c projectAnalysisFixtureClient) Chat(context.Context, []llm.ChatMessage) (*llm.ChatResponse, error) {
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
	report := newProjectAnalysisReport("project", "revision-one", "model-a", "analysis")
	report.Purpose = "Explains the project."
	report.Architecture = "One daemon."
	if err := StoreProjectAnalysisReport(root, report); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: "project", ProjectRevision: "revision-one", Model: "model-a", Profile: "analysis", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || loaded == nil || loaded.Status != ProjectAnalysisStatusFresh {
		t.Fatalf("fresh stored report = %+v, %v", loaded, err)
	}
	stale, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: "project", ProjectRevision: "revision-two", Model: "model-a", Profile: "analysis", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || stale == nil || stale.Status != ProjectAnalysisStatusStale {
		t.Fatalf("changed revision report = %+v, %v", stale, err)
	}
	if _, err := os.Stat(filepath.Join(root, projectAnalysisReportPath)); err != nil {
		t.Fatalf("structured report was not persisted: %v", err)
	}
}

func TestAnalyzerFailureKeepsInventoryAndWritesTruthfulProjection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	analysis, err := NewAnalyzerWithProfile(projectAnalysisFixtureClient{output: "not JSON"}, "fixture-model", "analysis").Analyze(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.AIStatus != ProjectAnalysisStatusFailed || analysis.FileCount != 1 || analysis.SourceFileCount != 1 || analysis.Report.Failure == "" {
		t.Fatalf("failed analysis = %+v", analysis)
	}
	markdown, err := os.ReadFile(filepath.Join(root, analysisRelativePath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), analysis.Report.Failure) || !strings.Contains(string(markdown), "main.go") {
		t.Fatalf("projection does not retain deterministic facts: %s", markdown)
	}
	stored, err := LoadProjectAnalysisReport(root, ProjectAnalysisInput{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Model: "fixture-model", Profile: "analysis", PromptVersion: projectAnalysisPromptVersion})
	if err != nil || stored == nil || stored.Status != ProjectAnalysisStatusFailed {
		t.Fatalf("stored failed report = %+v, %v", stored, err)
	}
}

func TestProjectAnalysisReportRequiresGeneratedTime(t *testing.T) {
	report := newProjectAnalysisReport("project", "revision", "model", "analysis")
	report.Purpose = "Purpose"
	report.Architecture = "Architecture"
	report.GeneratedAt = time.Time{}
	if validStoredProjectAnalysisReport(report) {
		t.Fatal("report without generated time is valid")
	}
}
