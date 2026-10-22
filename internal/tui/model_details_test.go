package tui

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func TestModelAndPresetDetailsRemainReachableWithoutDispatch(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{80, 24}, {96, 24}, {120, 36}} {
			s := NewState()
			s.Palette = palette
			s.Models = &ModelCatalog{Scopes: map[string]Model{"analyze": {
				Model: "inspect-model", ProviderOrigin: "cli://inspect-provider", RemoteProvider: true,
				ContextMaxTokens: 131072, Timeout: "7m0s", MaxRetries: 3,
			}}}
			s.CustomPresets = []ModelPreset{{ID: "inspect", Name: "Inspect routing", Analyze: "analyze-choice", Bug: "bug-choice", Function: "code-choice"}}
			originalPreset := s.ActivePresetID
			s.Navigate(PageModels)
			for _, selection := range []struct {
				index  int
				tokens []string
			}{
				{0, []string{"MODEL DETAILS", "inspect-model", "cli://inspect-provider", "timeout:7m0s", "retries:3"}},
				{len(domain.CanonicalModelScopes()) + len(domain.DefaultModelPresets()), []string{"PRESET DETAILS", "analyze-choice", "bug-choice", "code-choice"}},
			} {
				for s.SelectedIdx < selection.index {
					s.HandleKey(KeyEvent{Key: "ArrowDown"})
				}
				var rendered strings.Builder
				for range 8 {
					frame := s.Render(size[0], size[1])
					rendered.WriteString(frame)
					for _, row := range strings.Split(frame, "\n") {
						if theme.VisibleRuneCount(row) != size[0] {
							t.Fatalf("model details overflowed %v: %q", size, row)
						}
					}
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
				for _, token := range selection.tokens {
					if !strings.Contains(rendered.String(), token) {
						t.Fatalf("detail %q inaccessible at %v (selection %d)", token, size, selection.index)
					}
				}
				for range 8 {
					s.HandleKey(KeyEvent{Key: "PageUp"})
				}
				if s.DetailScroll != 0 || s.SelectedIdx != selection.index || s.ActivePresetID != originalPreset || s.AssignedScopeModelID("analyze") != "analyze" || len(s.PendingActions) != 0 {
					t.Fatal("detail navigation changed routing, dispatched work or lost selection")
				}
			}
		}
	}
}
