package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

func TestCompareCandidatesRequiresSameFocusedBase(t *testing.T) {
	service := comparisonService(t)
	left := comparisonPreview(t, service, "left", "func Run() { println(\"left\") }")
	right := comparisonPreview(t, service, "right", "func Run() { println(\"right\") }")
	service.rememberCandidate(&left)
	service.rememberCandidate(&right)

	comparison, err := service.CompareCandidates(left.GenerationID, right.GenerationID, "Prefer the smaller diff", "Keeps the existing control flow")
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Left.DiffLines != 1 || comparison.Right.DiffLines != 1 || comparison.Left.UserNote != "Prefer the smaller diff" || comparison.Right.UserNote != "Keeps the existing control flow" {
		t.Fatalf("comparison = %+v", comparison)
	}

	differentBase := comparisonPreview(t, service, "other", "func Run() { println(\"other\") }")
	differentBase.BaseFileHash = "changed-base"
	service.rememberCandidate(&differentBase)
	if _, err := service.CompareCandidates(left.GenerationID, differentBase.GenerationID, "", ""); err == nil {
		t.Fatal("expected candidates with different bases to be rejected")
	}
	if _, err := service.CompareCandidates(left.GenerationID, left.GenerationID, "", ""); err == nil {
		t.Fatal("expected a distinct second candidate to be required")
	}
}

func TestExportReviewMarkdownRedactsSecretLikeTextAndSource(t *testing.T) {
	analysis := &project.FileAnalysis{
		Path:    "internal/sample.go",
		Purpose: "Uses API_KEY=super-secret to initialize the client.",
		Risks:   []project.Finding{{Severity: "high", Summary: "password: hunter2 must not be logged."}},
	}
	candidate := comparisonPreview(t, comparisonService(t), "candidate", "package sample\n\nfunc Run() { panic(\"source must not be exported\") }\n")
	report := &CandidateCheckReport{Checks: []CandidateCheck{{Name: "go test", State: CheckPassed}}}
	markdown := ExportReviewMarkdown(analysis, project.SymbolInfo{Name: "Run", Kind: "function"}, &candidate, report, "audit-1")
	for _, forbidden := range []string{"super-secret", "hunter2", "source must not be exported"} {
		if strings.Contains(markdown, forbidden) {
			t.Fatalf("export leaked %q: %s", forbidden, markdown)
		}
	}
	for _, required := range []string{"# Mini-Orca focused review", "[redacted]", "Audit reference: `audit-1`", "Diff lines: 1"} {
		if !strings.Contains(markdown, required) {
			t.Fatalf("export missing %q: %s", required, markdown)
		}
	}
}

func comparisonService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	return &Service{manager: manager, drafts: make(map[string]*storedDraft)}
}

func comparisonPreview(t *testing.T, service *Service, id, content string) GenerationPreview {
	t.Helper()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	info, err := project.GetFileInfo(service.manager.Root(), "sample.go")
	if err != nil {
		t.Fatal(err)
	}
	return GenerationPreview{
		GenerationID: id, ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: info.ContentHash,
		TargetPath: "sample.go", TargetSymbol: "Run", Action: string(workflow.ActionFix), ScopeMode: workflow.ScopeStrictSymbol,
		CandidateContent: content, CandidateHash: candidateHash(content), EffectiveModel: EffectiveModel{Model: "fixture-model"},
		Validation: project.GenerationValidation{Applicable: true, Diff: project.UnifiedDiff{Lines: []project.DiffLine{{Kind: "added", NewLine: 3, Text: "func Run() {}"}}}},
	}
}
