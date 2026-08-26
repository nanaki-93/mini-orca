package app

import (
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestProjectOverviewSeparatesFindingSourcesAndKeepsMissingAnalysisUseful(t *testing.T) {
	service, root, revision := newGoScanService(t, map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package fixture\nfunc Run() {}\n"})
	manager := service.manager
	analysis, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	analysis.Report = project.ProjectAnalysisReport{
		SchemaVersion: "1", ProjectID: analysis.ProjectID, ProjectRevision: revision,
		Purpose: "Runs the fixture.", Architecture: "Single package.", Components: []string{}, EntryPoints: []string{}, Flows: []string{},
		Risks: []project.ProjectAnalysisRisk{{Severity: "low", Summary: "Add an integration test."}}, NextSteps: []string{},
		Status: project.ProjectAnalysisStatusFresh, Profile: "analysis", PromptVersion: "project-analysis-v1", GeneratedAt: time.Now().UTC(),
	}
	if err := manager.Set(root, analysis); err != nil {
		t.Fatal(err)
	}
	store, err := project.NewFindingStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReconcileSource(project.FindingInput{ProjectID: analysis.ProjectID, ProjectRevision: revision}, project.FindingSourceVet, []project.UnifiedFinding{{Source: project.FindingSourceVet, Confidence: project.FindingConfidenceToolReported, Severity: "high", Title: "Vet", Message: "bad call", Evidence: "password: must-not-leak"}}); err != nil {
		t.Fatal(err)
	}

	overview, err := service.ProjectOverview()
	if err != nil {
		t.Fatal(err)
	}
	if overview.Coverage.Total != 2 || overview.Coverage.Missing != 2 || overview.Findings.Verified != 1 || overview.Findings.AI != 1 {
		t.Fatalf("overview = %+v", overview)
	}
	findings, err := service.ListFindings(FindingFilter{Confidence: project.FindingConfidenceToolReported})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Source != project.FindingSourceVet || findings[0].Evidence != "[redacted]" {
		t.Fatalf("findings = %+v", findings)
	}
	if err := service.UpdateFindingStatus(revision, findings[0].ID, project.FindingStatusDismissed); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateFindingStatus("stale", findings[0].ID, project.FindingStatusOpen); err != project.ErrRevisionConflict {
		t.Fatalf("stale update = %v", err)
	}
}
