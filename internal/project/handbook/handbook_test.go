package handbook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildAndPersistProjectDocsSynthesizesC4Arc42AndADRSpec(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	analysis := Analysis{
		ProjectID:       "proj-1",
		ProjectRevision: "rev-1",
		Name:            "mini-orca",
		Path:            root,
		Type:            "go",
		BuildFile:       "go.mod",
		FileCount:       4,
		SourceFileCount: 3,
		TotalLines:      240,
		Languages:       map[string]int{"Go": 2, "TypeScript": 1},
		Files:           []string{"Makefile", "scripts/validate.sh", "internal/app/service.go", "desktop/frontend/src/main.tsx"},
		Report: ProjectAnalysisReport{
			SchemaVersion:   "1",
			ProjectID:       "proj-1",
			ProjectRevision: "rev-1",
			Purpose:         "Local-first software architecture and code review assistant.",
			Architecture:    "flowchart TD\n UI[Desktop UI] --> API[HTTP Boundary]\n API --> App[App Orchestrator]",
			Components: []string{
				"internal/app (Workflow Orchestrator): Coordinates analysis runs and guarded proposals.",
				"internal/api (HTTP Boundary): Validates local requests and serializes responses.",
			},
			EntryPoints: []string{"cmd/daemon/main.go"},
			Flows: []string{
				"sequenceDiagram\n Client->>API: Start Analysis\n API->>App: Dispatch Window",
			},
			Risks: []ProjectAnalysisRisk{
				{
					Severity: "medium",
					Summary:  "Guard concurrent worktree mutations with revision checks.",
					EngineeringInsight: &EngineeringInsight{
						Mechanism:             "Revision-guarded state transition",
						WhyItMattersHere:      "Prevents stale proposals from overwriting newer source files",
						TradeoffOrFailureMode: "Rejects out-of-date requests with conflict errors",
						TransferableLesson:    "Bind every write to its observed base hash",
					},
				},
			},
			NextSteps: []string{"Verify worktree isolation under concurrent runs."},
			EngineeringInsight: &EngineeringInsight{
				Mechanism:             "Loopback-only daemon with explicit remote provider consent",
				WhyItMattersHere:      "Keeps source code local unless the user confirms external model calls",
				TradeoffOrFailureMode: "Requires explicit confirmation before remote dispatch",
				TransferableLesson:    "Enforce consent at the transport admission boundary",
			},
			Status:      ProjectAnalysisStatusFresh,
			GeneratedAt: now,
		},
	}
	index := &ProjectIndex{
		ProjectID:       "proj-1",
		ProjectRevision: "rev-1",
		Files: []IndexFile{
			{
				Path:      "internal/app/service.go",
				Language:  "Go",
				LineCount: 140,
			},
			{
				Path:      "desktop/frontend/src/main.tsx",
				Language:  "TypeScript",
				LineCount: 100,
			},
		},
	}

	docs := BuildProjectDocs(analysis, index)
	if docs.SchemaVersion != "2" || docs.Status != ProjectAnalysisStatusFresh || docs.Context.Purpose != "Model interpretation: "+analysis.Report.Purpose {
		t.Fatalf("unexpected docs context: %+v", docs.Context)
	}
	if docs.Context.ArchitectureDiagram == "" || len(docs.Context.Stack) < 2 || len(docs.Context.GoalPursuit) < 2 {
		t.Fatalf("missing diagram, stack, or goal pursuit: %+v", docs.Context)
	}
	if len(docs.Dependencies) < 2 {
		t.Fatalf("expected dependencies from stack/components, got %+v", docs.Dependencies)
	}
	if len(docs.Components) != 2 {
		t.Fatalf("expected only inventory-anchored components, got %+v", docs.Components)
	}
	if len(docs.Scenarios) != 1 || len(docs.Scenarios[0].Steps) != 0 || docs.Scenarios[0].Diagram != analysis.Report.Flows[0] || docs.Scenarios[0].Invariant != "Unverified model interpretation" {
		t.Fatalf("unexpected scenarios: %+v", docs.Scenarios)
	}
	if len(docs.Decisions) < 2 || len(docs.Runbook) < 2 || len(docs.Glossary) < 2 {
		t.Fatalf("expected decisions, runbook, and glossary: decisions=%d runbook=%d glossary=%d", len(docs.Decisions), len(docs.Runbook), len(docs.Glossary))
	}

	if err := StoreProjectDocsReport(root, docs); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProjectDocsReport(root, "proj-1", "rev-1")
	if err != nil || loaded == nil || loaded.Status != ProjectAnalysisStatusFresh {
		t.Fatalf("loaded docs = %+v, err = %v", loaded, err)
	}
	stale, err := LoadProjectDocsReport(root, "proj-1", "rev-2")
	if err != nil || stale == nil || stale.Status != ProjectAnalysisStatusStale {
		t.Fatalf("expected stale docs on revision change, got %+v, err = %v", stale, err)
	}
}

func TestHandbookKeepsMissingEvidenceAbsentAndUsesActualImports(t *testing.T) {
	analysis := Analysis{ProjectID: "p", ProjectRevision: "r", Files: []string{"internal/a/a.go", "internal/b/b.go"}}
	index := &ProjectIndex{Files: []IndexFile{
		{Path: "internal/a/a.go", Language: "Go", Imports: []string{"fmt"}},
		{Path: "internal/b/b.go", Language: "Go"},
	}}
	analysis.Report.Components = []string{"../escape (Escape): outside", "invented/file.go (Fake): nonexistent", "internal/a (A): Suggested responsibility"}
	docs := BuildProjectDocs(analysis, index)
	if len(docs.Scenarios) != 0 || len(docs.Decisions) != 0 || len(docs.Components) != 2 || docs.Status != ProjectAnalysisStatusUnavailable {
		t.Fatalf("invented evidence: %+v", docs)
	}
	if len(docs.Components[0].Dependencies) != 1 || docs.Components[0].Dependencies[0] != "fmt" || len(docs.Components[1].Dependencies) != 0 {
		t.Fatalf("fabricated component links: %+v", docs.Components)
	}
	for _, component := range docs.Components {
		if len(component.Contracts) != 0 {
			t.Fatalf("fabricated boundary contract: %+v", component)
		}
	}
	data, err := json.Marshal(docs)
	if err != nil || strings.Contains(string(data), "validates inputs") || strings.Contains(string(data), "verified state") || strings.Contains(string(data), "Modular Subsystem Separation") {
		t.Fatalf("unsupported guarantees: %s %v", data, err)
	}
}

func TestLegacyHandbookIsRebuiltWithoutRewritingStoredHistory(t *testing.T) {
	root := t.TempDir()
	legacy := ProjectDocsReport{SchemaVersion: "1", ProjectID: "p", ProjectRevision: "r", Status: ProjectAnalysisStatusFresh,
		Scenarios: []ProjectDocsScenario{{Invariant: "fabricated guard"}}}
	if err := StoreProjectDocsReport(root, legacy); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, projectDocsReportPath)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProjectDocsReport(root, "p", "r")
	if err != nil || loaded != nil {
		t.Fatalf("legacy derived claims remained current: %+v %v", loaded, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("passive read rewrote historical data: %v", err)
	}
	current := BuildProjectDocs(Analysis{ProjectID: "p", ProjectRevision: "r"}, nil)
	if current.SchemaVersion != "2" || len(current.Scenarios) != 0 || len(current.Decisions) != 0 {
		t.Fatalf("rebuild fabricated evidence: %+v", current)
	}
}
