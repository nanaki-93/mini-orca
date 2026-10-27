package archgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitecturePolicyDeduplicationAndFileLineBudgetWithoutFunctions(t *testing.T) {
	root := t.TempDir()
	agentsMd := strings.Join([]string{
		"# Agent Contract",
		"- `internal/api` must never import `internal/storage` or `internal/llm`.",
		"- `cmd` must never import `storage`.",
		"- src/ui/ must never import src/db/.",
		"1. Max function complexity: 12",
		"2. Max function lines: 80",
		"3. Max file lines: 5",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agentsMd), 0644); err != nil {
		t.Fatal(err)
	}
	rules, budgets := loadArchitecturePolicy(root)
	if budgets.maxComplexity != 12 || budgets.maxFunctionLines != 80 || budgets.maxFileLines != 5 {
		t.Fatalf("expected maxComplexity=12 maxFunctionLines=80 maxFileLines=5, got %+v", budgets)
	}
	var apiRuleCount, cmdRuleCount, uiRuleCount int
	for _, r := range rules {
		switch r.sourcePrefix {
		case "internal/api":
			apiRuleCount++
		case "cmd":
			cmdRuleCount++
		case "src/ui":
			uiRuleCount++
		}
	}
	if apiRuleCount != 1 || cmdRuleCount != 1 || uiRuleCount != 1 {
		t.Fatalf("expected deduplicated internal/api rule and parsed single-segment/un-backticked rules, got %+v", rules)
	}

	checks := ChangeArchitectureAndComplexityChecks(root, []ChangeEdit{
		{
			Path:    "src/constants.ts",
			Content: "export const A = 1;\nexport const B = 2;\nexport const C = 3;\nexport const D = 4;\nexport const E = 5;\nexport const F = 6;\n",
		},
	})
	var complexityFailed bool
	for _, check := range checks {
		if check.Name == "complexity gate" && check.State == CheckFailed && strings.Contains(check.Output, "file has 7 lines (max 5)") {
			complexityFailed = true
		}
	}
	if !complexityFailed {
		t.Fatalf("expected maxFileLines budget to be enforced even when file has no callable functions, got %+v", checks)
	}
}

func TestRepositoryArchitectureBoundaries(t *testing.T) {
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	var prodEdits []ChangeEdit
	for _, rootDir := range []string{"cmd", "internal"} {
		walkRoot := filepath.Join(repoRoot, rootDir)
		err := filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			prodEdits = append(prodEdits, ChangeEdit{
				Path:    filepath.ToSlash(rel),
				Content: string(content),
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(prodEdits) == 0 {
		t.Fatal("expected production Go files in repository")
	}
	checks := ChangeArchitectureAndComplexityChecks(repoRoot, prodEdits)
	for _, check := range checks {
		if check.State != CheckPassed {
			t.Fatalf("repository check %q failed:\n%s", check.Name, check.Output)
		}
	}

	violationChecks := ChangeArchitectureAndComplexityChecks(repoRoot, []ChangeEdit{
		{
			Path:    "internal/project/diff/bad.go",
			Content: "package diff\nimport _ \"github.com/nanaki-93/mini-orca/v2/internal/project/symbols\"\n",
		},
		{
			Path:    "internal/app/gitops/bad.go",
			Content: "package gitops\nimport _ \"github.com/nanaki-93/mini-orca/v2/internal/app/archgate\"\n",
		},
	})
	if len(violationChecks) == 0 || violationChecks[0].State != CheckFailed {
		t.Fatalf("expected subpackage boundary violations to fail, got %+v", violationChecks)
	}
	for _, want := range []string{
		"internal/project/diff must never import internal/project/symbols",
		"internal/app/gitops must never import internal/app/archgate",
	} {
		if !strings.Contains(violationChecks[0].Output, want) {
			t.Fatalf("expected violation %q in output, got:\n%s", want, violationChecks[0].Output)
		}
	}
}

func TestAgentRuntimeImportBoundary(t *testing.T) {
	for _, test := range []struct {
		path, imported string
		allowed        bool
	}{
		{"internal/app/agentruntime/model.go", "google.golang.org/adk/v2/model", true},
		{"internal/app/agentruntime/model.go", "google.golang.org/genai", true},
		{"internal/app/agentruntime/session.go", "github.com/nanaki-93/mini-orca/v2/internal/app/agentstate", true},
		{"internal/app/agentstate/store.go", "github.com/nanaki-93/mini-orca/v2/internal/storage", true},
		{"internal/app/agentstate/store.go", "github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime", false},
		{"internal/app/agentstate/store.go", "google.golang.org/adk/v2/session", false},
		{"internal/app/agentruntime/model.go", "github.com/nanaki-93/mini-orca/v2/internal/app", false},
		{"internal/app/agentruntime/model.go", "github.com/nanaki-93/mini-orca/v2/internal/project", false},
		{"internal/app/agentruntime/model.go", "github.com/nanaki-93/mini-orca/v2/internal/storage", false},
		{"internal/app/service.go", "google.golang.org/adk/v2/runner", false},
		{"internal/project/project_overview.go", "google.golang.org/adk/v2/model", false},
		{"internal/storage/file.go", "google.golang.org/adk/v2/session", false},
		{"internal/agenteval/eval.go", "google.golang.org/adk/v2/runner", false},
		{"internal/insighteval/eval.go", "google.golang.org/genai", false},
		{"cmd/daemon/main.go", "google.golang.org/adk/v2/runner", false},
		{"cmd/agent-eval/main.go", "google.golang.org/adk/v2/runner", false},
		{"cmd/agent-inspect/main.go", "google.golang.org/genai", false},
		{"main.go", "google.golang.org/adk/v2/runner", true},
	} {
		t.Run(test.path+"/"+test.imported, func(t *testing.T) {
			checks := ChangeArchitectureAndComplexityChecks("", []ChangeEdit{{
				Path: test.path, Content: "package fixture\nimport _ \"" + test.imported + "\"\n",
			}})
			if len(checks) == 0 || (checks[0].State == CheckPassed) != test.allowed {
				t.Fatalf("allowed=%v, checks=%+v", test.allowed, checks)
			}
		})
	}
}
