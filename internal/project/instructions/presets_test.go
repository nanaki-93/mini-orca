package instructions

import (
	"slices"
	"testing"
)

func TestSuggestPresetsMatchesSignalsAndAppliesPolicyFilter(t *testing.T) {
	files := []IndexFile{
		{Path: "cmd/main.go", Language: "Go", Imports: []string{"net/http", "flag"}},
		{Path: "private/secret.py", Language: "Python"},
		{Path: "README.md", Language: "Markdown"},
	}
	presets := SuggestPresets("AGENTS.md", files, func(path string) bool {
		return path != "private/secret.py"
	})
	var ids []string
	for _, p := range presets {
		if len(p.Evidence) > 0 {
			ids = append(ids, p.ID)
		}
	}
	if !slices.Contains(ids, "go") || !slices.Contains(ids, "http") || !slices.Contains(ids, "cli-contracts") || !slices.Contains(ids, "docs") {
		t.Fatalf("expected Go, HTTP, CLI, and Docs presets, got %v", ids)
	}
	if slices.Contains(ids, "python") {
		t.Fatalf("expected excluded Python file to be filtered out, got %v", ids)
	}
}
