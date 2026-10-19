package features

import (
	"fmt"
	"strings"
	"testing"
)

func TestFeatureTitleNormalizationAndMergeRules(t *testing.T) {
	for title, want := range map[string]string{
		"  Add  Cancellation-Aware Work! ": "add cancellation aware work",
		"Ünïcode ÑAME":                     "ünïcode ñame",
		"v2.0 export":                      "v2 0 export",
		"!!! ***":                          "",
	} {
		if got := NormalizedTitle(title); got != want {
			t.Fatalf("NormalizedTitle(%q)=%q want %q", title, got, want)
		}
	}
	existing := []FeatureSuggestion{{ID: "a", Title: "Export reports", Status: "dismissed", GenerationID: "gen-old"}}
	candidates := []FeatureSuggestion{{ID: "b", Title: "!!!", Status: "open"}, {ID: "c", Title: "???", Status: "open"}, {ID: "d", Title: "export-reports", Status: "open"}}
	merged, outcome := MergeSuggestions(existing, candidates, "gen-new")
	if len(merged) != 3 || merged[0].ID != "a" || merged[0].Status != "dismissed" || merged[0].GenerationID != "gen-old" || merged[1].ID != "b" || merged[2].ID != "c" ||
		merged[2].GenerationID != "gen-new" || outcome.AddedCount != 2 || outcome.DuplicateCount != 1 {
		t.Fatalf("merged=%+v outcome=%+v", merged, outcome)
	}
	if existing[0].Status != "dismissed" || len(existing) != 1 {
		t.Fatal("merge mutated existing ideas")
	}
}

func TestExistingFeatureIdeasBlockIsBoundedNewestFirst(t *testing.T) {
	ideas := make([]FeatureSuggestion, 200)
	for i := range ideas {
		ideas[i] = FeatureSuggestion{Title: fmt.Sprintf("Idea %03d %s\nsecond line", i, strings.Repeat("x", 80)), Status: "open"}
	}
	ideas[199].Status = "dismissed"
	block := ExistingIdeas(ideas)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(block) > MaxExistingIdeasBytes || !strings.HasPrefix(lines[1], "- dismissed: Idea 199 ") || !strings.HasPrefix(lines[2], "- open: Idea 198 ") {
		t.Fatalf("block=%d bytes, first lines=%q", len(block), lines[:3])
	}
	listed := len(lines) - 2
	if want := fmt.Sprintf("%d older ideas omitted.", len(ideas)-listed); lines[len(lines)-1] != want {
		t.Fatalf("omission line=%q want %q", lines[len(lines)-1], want)
	}
}
