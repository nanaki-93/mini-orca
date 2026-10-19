package archcheck

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
		"- Max file lines: 5",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agentsMd), 0644); err != nil {
		t.Fatal(err)
	}
	rules, budgets := loadArchitecturePolicy(root)
	if budgets.maxFileLines != 5 {
		t.Fatalf("expected maxFileLines=5, got %+v", budgets)
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
