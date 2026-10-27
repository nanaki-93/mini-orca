package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// EffectiveDiffMode returns the active diff view mode ("hunk", "split", or "unified").
func (s *State) EffectiveDiffMode() string {
	switch strings.ToLower(strings.TrimSpace(s.DiffMode)) {
	case "split", "side", "side-by-side":
		return "split"
	case "unified", "all", "stream":
		return "unified"
	default:
		return "hunk"
	}
}

func diffModeDisplayName(mode string) string {
	switch mode {
	case "split":
		return "Side-by-side"
	case "unified":
		return "Unified"
	default:
		return "Focused hunk"
	}
}

func (s *State) activeDiffHunk() (FileDiffHunk, []FileDiffHunk, bool) {
	changes := s.diffChanges()
	if len(changes) == 0 {
		return FileDiffHunk{}, nil, false
	}
	all := domain.CollectChangeHunks(changes)
	if len(all) == 0 {
		return FileDiffHunk{}, nil, false
	}
	idx := min(max(0, s.HunkIdx), len(all)-1)
	return all[idx], all, true
}

func (s *State) unifiedDiffRows() []UnifiedDiffRow {
	changes := s.diffChanges()
	if len(changes) == 0 {
		return nil
	}
	allHunks := domain.CollectChangeHunks(changes)
	var rows []UnifiedDiffRow
	for _, fh := range allHunks {
		header := fmt.Sprintf("File %d/%d: %s · Hunk #%d/%d (@@ -%d,%d +%d,%d @@)",
			fh.FileIndex+1, fh.FileTotal, theme.SanitizeCodeText(fh.Path),
			fh.GlobalIndex+1, len(allHunks),
			fh.Hunk.OldStart, fh.Hunk.OldEnd, fh.Hunk.NewStart, fh.Hunk.NewEnd,
		)
		rows = append(rows, UnifiedDiffRow{
			IsHeader:      true,
			HunkGlobalIdx: fh.GlobalIndex,
			FileIndex:     fh.FileIndex,
			HeaderText:    header,
		})
		for _, dl := range fh.Hunk.Lines {
			rows = append(rows, UnifiedDiffRow{
				IsHeader:      false,
				HunkGlobalIdx: fh.GlobalIndex,
				FileIndex:     fh.FileIndex,
				Line:          dl,
			})
		}
	}
	return rows
}

func (s *State) diffViewRowCount() int {
	active, _, ok := s.activeDiffHunk()
	if !ok {
		return 1
	}
	switch s.EffectiveDiffMode() {
	case "split":
		return max(1, len(domain.BuildSplitDiffRows(active.Hunk.Lines)))
	case "unified":
		return max(1, len(s.unifiedDiffRows()))
	default:
		return max(1, len(active.Hunk.Lines))
	}
}

func (s *State) syncDiffSelectionToHunk(hunkIdx int) {
	active, all, ok := s.activeDiffHunk()
	if !ok {
		s.SelectedIdx = 0
		return
	}
	hunkIdx = min(max(0, hunkIdx), len(all)-1)
	s.HunkIdx = hunkIdx
	s.DetailScroll = 0
	active = all[hunkIdx]
	mode := s.EffectiveDiffMode()
	if mode == "unified" {
		for idx, r := range s.unifiedDiffRows() {
			if r.HunkGlobalIdx == hunkIdx && !r.IsHeader {
				s.SelectedIdx = idx
				return
			}
		}
		s.SelectedIdx = 0
		return
	}
	s.SelectedIdx = domain.FirstChangedRowForMode(active.Hunk, mode)
}

func (s *State) formatFocusedHunkStatus(fh FileDiffHunk, totalHunks int) string {
	if fh.FileTotal > 1 {
		return fmt.Sprintf("Focused diff hunk #%d/%d · file %d/%d %s (lines %d-%d)",
			fh.GlobalIndex+1, totalHunks, fh.FileIndex+1, fh.FileTotal, fh.Path, fh.Hunk.NewStart, fh.Hunk.NewEnd)
	}
	return fmt.Sprintf("Focused diff hunk #%d/%d · %s (lines %d-%d)",
		fh.GlobalIndex+1, totalHunks, fh.Path, fh.Hunk.NewStart, fh.Hunk.NewEnd)
}
