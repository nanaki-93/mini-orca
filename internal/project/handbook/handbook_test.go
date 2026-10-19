package handbook

import (
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
				Symbols: []SymbolInfo{
					{Name: "Service", Kind: "struct", StartLine: 10, EndLine: 45},
				},
			},
			{
				Path:      "desktop/frontend/src/main.tsx",
				Language:  "TypeScript",
				LineCount: 100,
			},
		},
	}

	docs := BuildProjectDocs(analysis, index)
	if docs.Status != ProjectAnalysisStatusFresh || docs.Context.Purpose != analysis.Report.Purpose {
		t.Fatalf("unexpected docs context: %+v", docs.Context)
	}
	if docs.Context.ArchitectureDiagram == "" || len(docs.Context.Stack) < 2 || len(docs.Context.GoalPursuit) < 2 {
		t.Fatalf("missing diagram, stack, or goal pursuit: %+v", docs.Context)
	}
	if len(docs.Dependencies) < 2 {
		t.Fatalf("expected dependencies from stack/components, got %+v", docs.Dependencies)
	}
	if len(docs.Components) < 3 {
		t.Fatalf("expected components from report and index, got %+v", docs.Components)
	}
	if len(docs.Scenarios) != 1 || len(docs.Scenarios[0].Steps) != 3 || docs.Scenarios[0].Diagram == "" {
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
