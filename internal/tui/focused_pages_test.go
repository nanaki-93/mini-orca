package tui

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func TestFocusedPlanSectionsAreSeparateAndNavigationIsPassive(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{80, 24}, {120, 30}, {160, 40}} {
			s := NewState()
			s.Palette = palette
			s.openTaskPlan(&ChangeSeed{Title: "Fix shared state", Paths: []string{"scope.go"}, AcceptanceCriteria: []string{"ACCEPTANCE_TOKEN"}, Plan: &ChangePlanInput{Problem: "PROBLEM_TOKEN", Approach: "APPROACH_TOKEN", Evidence: "EVIDENCE_TOKEN"}})
			for section, token := range []string{"APPROACH_TOKEN", "EVIDENCE_TOKEN", "scope.go", "ACCEPTANCE_TOKEN"} {
				frame := s.Render(size[0], size[1])
				if !strings.Contains(frame, token) {
					t.Fatalf("section %d inaccessible at %v: %s", section, size, frame)
				}
				if section != 1 && strings.Contains(frame, "EVIDENCE_TOKEN") {
					t.Fatal("evidence leaked into unrelated section")
				}
				for _, row := range strings.Split(frame, "\n") {
					if theme.VisibleRuneCount(row) != size[0] {
						t.Fatalf("invalid row width: %d", theme.VisibleRuneCount(row))
					}
				}
				s.HandleKey(KeyEvent{Key: "Tab"})
			}
			if len(s.PendingActions) != 0 || s.DetailSection != 0 {
				t.Fatal("section navigation dispatched or did not wrap")
			}
			s.HandleKey(KeyEvent{Key: "Tab", Shift: true})
			if s.DetailSection != 3 {
				t.Fatal("reverse section navigation failed")
			}
		}
	}
}

func TestFeatureInspectionAndReturnPreserveSelection(t *testing.T) {
	s := NewState()
	s.Features = &FeatureReport{Suggestions: []FeatureSuggestion{{ID: "a", Title: "First"}, {ID: "b", Title: "Second", Evidence: "FEATURE_EVIDENCE"}}}
	s.Navigate(PageFeatures)
	s.SelectedIdx = 1
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageFeatureDetail || s.SelectedIdx != 1 || len(s.PendingActions) != 0 {
		t.Fatal("feature inspection dispatched or lost selection")
	}
	s.HandleKey(KeyEvent{Key: "Tab"})
	if !strings.Contains(s.Render(120, 35), "FEATURE_EVIDENCE") {
		t.Fatal("feature evidence inaccessible")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.Page != PageFeatures || s.SelectedIdx != 1 {
		t.Fatal("return lost feature selection")
	}
}
