package domain

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func TestSplitAndRejectDiffHunks(t *testing.T) {
	lines := []client.DiffLine{
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

	added, removed, badge := ClassifyDiffScope([]client.FileChange{{Diff: client.FileDiff{Lines: lines}}})
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
			Target:   client.FindingTarget{Kind: "semantic", ID: "sem-1", Path: "internal/db/query.go"},
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
			Target:   client.FindingTarget{Kind: "security", ID: "sec-1", Path: "internal/db/query.go", Source: "ai"},
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
			Target:   client.FindingTarget{Kind: "semantic", ID: "bug-1", Path: "internal/app/run.go"},
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

	summary := ComputeIssueSummary(deduped, &client.FeatureReport{
		Suggestions: []client.FeatureSuggestion{
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

	pillars := EvaluateBootstrapChecklist(&client.ProjectIndex{
		Files: []client.IndexedFile{
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
	change := &client.ChangeSession{
		ID:    "chg-1",
		Title: "Fix race condition",
		State: "checked",
		Changes: []client.FileChange{
			{
				Path: "internal/app/service.go",
				Diff: client.FileDiff{Lines: []client.DiffLine{{Kind: "add", NewLine: 10, Text: "mu.Lock()"}}},
			},
		},
		Checks: []client.Check{{Name: "test", State: "passed"}},
	}
	items := CollectAllAgentItems(change, nil, &client.ChangeMutation{UndoAvailable: true}, nil)
	if len(items) != 1 || items[0].StageNumber != 3 || !items[0].UndoAvailable {
		t.Fatalf("unexpected agent item: %+v", items)
	}

	docs := ResolveProjectDocs(&client.Overview{
		Analysis: client.OverviewAnalysis{
			Purpose:      "Local coding assistant",
			Architecture: "Layered daemon + TUI",
			Components:   []string{"cmd/tui", "internal/app"},
		},
	}, nil)
	ascii := strings.Join(RenderArchitectureASCII(docs), "\n")
	if !strings.Contains(ascii, "cmd/tui") || !strings.Contains(ascii, "internal/app") {
		t.Fatalf("unexpected ASCII diagram:\n%s", ascii)
	}

	multiPaths := EnsureChangeTestPaths([]string{"cmd/add.go", "internal/utils/utils.go"}, nil)
	wantPaths := "cmd/add.go,internal/utils/utils.go,cmd/add_test.go,internal/utils/utils_test.go"
	if got := strings.Join(multiPaths, ","); got != wantPaths {
		t.Fatalf("expected companion test paths per directory %q, got %q", wantPaths, got)
	}
}

func TestCommitCodeGenerationAndFormatting(t *testing.T) {
	targets := []client.FindingTarget{{Kind: "semantic", ID: "f1", Path: "main.go"}}
	code := GenerateCommitCode("chg-1", "hash-1", targets)
	if !strings.HasPrefix(code, "ORC-") || len(code) != 12 {
		t.Fatalf("expected ORC-XXXXXXXX commit code, got %q", code)
	}
	if again := GenerateCommitCode("chg-1", "hash-1", targets); again != code {
		t.Fatalf("expected deterministic commit code %q, got %q", code, again)
	}
	formatted := FormatCommitMessageWithCode("Fix nil pointer dereference", code)
	if !strings.HasPrefix(formatted, "["+code+"] ") {
		t.Fatalf("expected prefixed commit message, got %q", formatted)
	}
	if FormatCommitMessageWithCode(formatted, code) != formatted {
		t.Fatalf("expected FormatCommitMessageWithCode to be idempotent")
	}
	if got := ExtractCommitCode(formatted); got != code {
		t.Fatalf("ExtractCommitCode = %q, want %q", got, code)
	}
	if got := ResolveCommitCode("chg-1", "hash-1", targets, &client.ChangeCommit{Message: formatted}); got != code {
		t.Fatalf("ResolveCommitCode = %q, want %q", got, code)
	}
}

func TestSynthesizeIssueRemediationAndSymbolMatching(t *testing.T) {
	syms := []client.SymbolInfo{
		{Name: "Server.Start", Signature: "func (s *Server) Start(ctx context.Context) error", StartLine: 10, EndLine: 30},
		{Name: "parsePayload", Signature: "func parsePayload(raw []byte) error", StartLine: 35, EndLine: 60},
	}
	sym, sig := MatchIndexedSymbolForFinding(syms, "", 0, "Unchecked error in parsePayload", "parsePayload ignores Close error.")
	if sym != "parsePayload" || sig != "func parsePayload(raw []byte) error" {
		t.Fatalf("expected text-matched symbol parsePayload, got sym=%q sig=%q", sym, sig)
	}

	remErr := SynthesizeIssueRemediation(
		"Unchecked error return",
		"Close error is ignored after writing payload.",
		"bugs",
		"",
		"ai",
		"internal/api/server.go",
		"parsePayload",
		42,
		nil,
	)
	if strings.Contains(remErr, "Synthesize a minimal, verified fix") || !strings.Contains(remErr, "parsePayload") || !strings.Contains(remErr, "internal/api/server_test.go") {
		t.Fatalf("expected specific error-handling remediation, got %q", remErr)
	}

	insight := &client.EngineeringInsight{
		Mechanism:          "Token is compared with == instead of subtle.ConstantTimeCompare.",
		WhyItMattersHere:   "Timing side-channel leaks HMAC byte prefix.",
		TransferableLesson: "Use hmac.Equal for constant-time MAC comparison and test rejected tokens.",
	}
	remInsight := SynthesizeIssueRemediation("Timing side-channel", "Non-constant-time comparison", "security", "SEC-01", "ai", "auth.go", "Verify", 12, insight)
	if remInsight != insight.TransferableLesson {
		t.Fatalf("expected insight TransferableLesson %q, got %q", insight.TransferableLesson, remInsight)
	}
	if ev := DistinctInsightEvidence("", "Non-constant-time comparison", insight); ev != insight.WhyItMattersHere {
		t.Fatalf("expected DistinctInsightEvidence to surface WhyItMattersHere, got %q", ev)
	}
}
