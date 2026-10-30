package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheWritingAgentsAreToldTheBudgetsWithLiveNumbers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Max file lines: 5\nMax function complexity: 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := "def old(x):\n    if x and x > 1:\n        return 1\n    return 2\n"
	brief := budgetBrief(root, []ChangeTarget{{Path: "app/legacy.py", Exists: true, Content: legacy}, {Path: "app/new.py"}})
	for _, want := range []string{
		"- app/legacy.py: 5 lines per file, cyclomatic complexity 2 per function. Now 4 lines (1 left). Already over, must not grow: line 1: function old has cyclomatic complexity 3 (max 2).",
		"- app/new.py: 5 lines per file, cyclomatic complexity 2 per function.\n",
		"a fix-up step will split it into sibling files",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief lacks %q:\n%s", want, brief)
		}
	}
}
