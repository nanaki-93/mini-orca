package tuidomain

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/tuiclient"
)

func TestSplitAndRejectDiffHunks(t *testing.T) {
	lines := []tuiclient.DiffLine{
		{Kind: "context", OldLine: 1, NewLine: 1, Text: "package main"},
		{Kind: "delete", OldLine: 2, NewLine: 0, Text: "var old = 1"},
		{Kind: "add", OldLine: 0, NewLine: 2, Text: "var next = 2"},
		{Kind: "context", OldLine: 3, NewLine: 3, Text: "func a() {}"},
		{Kind: "context", OldLine: 4, NewLine: 4, Text: "func b() {}"},
		{Kind: "context", OldLine: 5, NewLine: 5, Text: "func c() {}"},
		{Kind: "context", OldLine: 6, NewLine: 6, Text: "func d() {}"},
		{Kind: "context", OldLine: 7, NewLine: 7, Text: "func e() {}"},
		{Kind: "add", OldLine: 0, NewLine: 8, Text: "func added() {}"},
	}

	hunks := SplitDiffIntoHunks(lines)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[0].OldStart != 1 || hunks[0].NewStart != 1 {
		t.Fatalf("unexpected first hunk bounds: %+v", hunks[0])
	}

	reverted := RejectHunkFromDiff(lines, hunks[0])
	if !strings.Contains(reverted, "var old = 1") || strings.Contains(reverted, "var next = 2") {
		t.Fatalf("expected first hunk to be reverted, got:\n%s", reverted)
	}
	if !strings.Contains(reverted, "func added() {}") {
		t.Fatalf("expected second hunk addition to remain, got:\n%s", reverted)
	}

	added, removed, badge := ClassifyDiffScope([]tuiclient.FileChange{{Diff: tuiclient.FileDiff{Lines: lines}}})
	if added != 2 || removed != 1 || badge != "SURGICAL" {
		t.Fatalf("unexpected diff scope: +%d -%d %s", added, removed, badge)
	}
}

func TestFindingNormalizationAndDeduplication(t *testing.T) {
	if got := FindingTitle("File analysis suggestion", "  Missing   nil check on client "); got != "Missing nil check on client" {
		t.Fatalf("FindingTitle = %q", got)
	}
	short := FindingName("Verify that all outbound HTTP requests propagate context deadlines properly", "")
	if !strings.HasSuffix(short, "…") {
		t.Fatalf("expected truncated finding name with ellipsis, got %q", short)
	}

	items := []IssueRow{
		{
			ID:       "sem-1",
			Title:    "SQL injection risk in query builder",
			Name:     "SQL injection risk in query",
			Path:     "internal/db/query.go",
			Line:     0,
			Severity: "high",
			Category: "security",
			Status:   "open",
			Target:   tuiclient.FindingTarget{Kind: "semantic", ID: "sem-1", Path: "internal/db/query.go"},
		},
		{
			ID:       "sec-1",
			Title:    "SQL injection risk in query builder",
			Name:     "SQL injection risk in query",
			Path:     "internal/db/query.go",
			Line:     42,
			Severity: "critical",
			Category: "security",
			Status:   "open",
			Target:   tuiclient.FindingTarget{Kind: "security", ID: "sec-1", Path: "internal/db/query.go", Source: "ai"},
		},
		{
			ID:       "bug-1",
			Title:    "Race on map write",
			Name:     "Race on map write",
			Path:     "internal/app/run.go",
			Line:     19,
			Severity: "high",
			Category: "bugs",
			Status:   "in_progress",
			Target:   tuiclient.FindingTarget{Kind: "semantic", ID: "bug-1", Path: "internal/app/run.go"},
		},
	}

	deduped := DedupeIssues(items)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 deduplicated issues, got %d", len(deduped))
	}
	SortIssuesBySeverity(deduped)
	if deduped[0].ID != "sec-1" {
		t.Fatalf("expected critical issue first, got %s", deduped[0].ID)
	}

	summary := ComputeIssueSummary(deduped, &tuiclient.FeatureReport{
		Suggestions: []tuiclient.FeatureSuggestion{
			{ID: "f1", Status: "open"},
			{ID: "f2", Status: "dismissed"},
		},
	})
	if summary.TotalOpen != 2 || summary.InProgress != 1 || summary.Security != 1 || summary.Bugs != 1 || summary.Features != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestGuidelineInsertionAndBootstrapChecklist(t *testing.T) {
	base := "# Project instructions\n\n## Architecture\n- Keep handlers thin.\n"
	updated := InsertGuidelineSection(base, "Harness² Lessons & Guardrails", "Propagate context.Context across I/O.")
	if !ContainsGuideline(updated, "Propagate context.Context across I/O.") {
		t.Fatalf("expected inserted guideline in:\n%s", updated)
	}
	again := InsertGuidelineSection(updated, "Harness² Lessons & Guardrails", "- Propagate context.Context across I/O.")
	if again != updated {
		t.Fatalf("expected duplicate guideline insertion to be idempotent")
	}

	pillars := EvaluateBootstrapChecklist(&tuiclient.ProjectIndex{
		Files: []tuiclient.IndexedFile{
			{Path: "AGENTS.md"},
			{Path: "Makefile"},
			{Path: "scripts/validate.sh"},
			{Path: "lefthook.yml"},
		},
	}, nil)
	for _, p := range pillars {
		if !p.Ready {
			t.Fatalf("expected pillar %s to be ready", p.ID)
		}
	}
}

func TestAgentItemsAndArchitectureDiagram(t *testing.T) {
	change := &tuiclient.ChangeSession{
		ID:    "chg-1",
		Title: "Fix race condition",
		State: "checked",
		Changes: []tuiclient.FileChange{
			{
				Path: "internal/app/service.go",
				Diff: tuiclient.FileDiff{Lines: []tuiclient.DiffLine{{Kind: "add", NewLine: 10, Text: "mu.Lock()"}}},
			},
		},
		Checks: []tuiclient.Check{{Name: "test", State: "passed"}},
	}
	items := CollectAgentItems(change, &tuiclient.ChangeMutation{UndoAvailable: true}, nil)
	if len(items) != 1 || items[0].StageNumber != 3 || !items[0].UndoAvailable {
		t.Fatalf("unexpected agent item: %+v", items)
	}

	docs := ResolveProjectDocs(&tuiclient.Overview{
		Analysis: tuiclient.OverviewAnalysis{
			Purpose:      "Local coding assistant",
			Architecture: "Layered daemon + TUI",
			Components:   []string{"cmd/tui", "internal/app"},
		},
	}, nil)
	ascii := strings.Join(RenderArchitectureASCII(docs), "\n")
	if !strings.Contains(ascii, "cmd/tui") || !strings.Contains(ascii, "internal/app") {
		t.Fatalf("unexpected ASCII diagram:\n%s", ascii)
	}
}
