package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
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
		return "SIDE-BY-SIDE SPLIT"
	case "unified":
		return "UNIFIED STREAM"
	default:
		return "FOCUSED HUNK"
	}
}

func (s *State) activeDiffHunk() (FileDiffHunk, []FileDiffHunk, bool) {
	changes := domain.EffectiveSessionChanges(s.Change)
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
	changes := domain.EffectiveSessionChanges(s.Change)
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

func (s *State) renderChangeDiffLines(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	active, allHunks, ok := s.activeDiffHunk()
	if !ok {
		return []string{fmt.Sprintf("  %sNo diff lines available.%s", th.muted, th.reset)}
	}
	added, removed, _ := domain.ClassifyDiffScope(s.Change.Changes)
	mode := s.EffectiveDiffMode()
	meta := s.buildDiffReviewMeta(th, active, len(allHunks), added, removed)
	stateTag := strings.ToUpper(client.FirstNonEmpty(s.Change.State, "draft"))
	rightTag := fmt.Sprintf("%s · +%d / -%d", stateTag, added, removed)
	lines := th.renderPanelBox("GUARDED DIFF REVIEW", rightTag, panelW, meta)

	switch mode {
	case "split":
		return append(lines, s.renderSplitHunkDiff(th, width, active, len(allHunks))...)
	case "unified":
		return append(lines, s.renderUnifiedStreamDiff(th, width, active, len(allHunks))...)
	default:
		return append(lines, s.renderFocusedHunkDiff(th, width, active, len(allHunks))...)
	}
}

func (s *State) buildDiffReviewMeta(th ansiTheme, active FileDiffHunk, totalHunks, _, _ int) []string {
	fileScope := fmt.Sprintf("%sFile %d/%d:%s %s%s%s",
		th.muted, active.FileIndex+1, max(1, active.FileTotal), th.reset,
		th.cyan, active.Path, th.reset)
	meta := []string{
		fmt.Sprintf(
			"%sHunk #%d/%d%s · %s · %sProposal:%s %s%s%s",
			th.bold+th.accent, active.GlobalIndex+1, totalHunks, th.reset,
			fileScope,
			th.muted, th.reset, th.bold+th.fg, s.Change.Title, th.reset,
		),
	}
	if s.ShowInlineWhy && active.Summary != "" {
		meta = append(meta, fmt.Sprintf("%sRationale:%s %s%s%s", th.bold+th.accent, th.reset, th.fg, theme.SanitizeCodeText(active.Summary), th.reset))
	}
	return meta
}

func (s *State) renderFocusedHunkDiff(th ansiTheme, width int, active FileDiffHunk, totalHunks int) []string {
	hdr := fmt.Sprintf("   OLD  NEW │ SIGN  FOCUSED HUNK #%d/%d · %s",
		active.GlobalIndex+1, totalHunks, theme.SanitizeCodeText(active.Path))
	lines := th.renderTableHeader(hdr, width)
	hunkLines := active.Hunk.Lines
	selected := s.ClampedIdx()
	codeCol := max(16, width-20)
	start, end := windowBounds(len(hunkLines), selected, s.adaptiveWindowRows(10, 14))
	for idx := start; idx < end; idx++ {
		lines = append(lines, formatDualNumberDiffLine(th, hunkLines[idx], idx == selected, codeCol))
	}
	return appendWindowFooter(th, lines, start, end, len(hunkLines))
}

func formatDualNumberDiffLine(th ansiTheme, dl DiffLine, selected bool, codeCol int) string {
	cursor := th.renderCursorPrefix(selected)
	text := theme.TruncatePlain(theme.SanitizeCodeText(dl.Text), codeCol)
	switch {
	case domain.IsAddedLine(dl.Kind):
		return fmt.Sprintf("  %s%s   · %4d%s %s┃ + %s%s",
			cursor, th.green, dl.NewLine, th.reset, th.bold+th.green, text, th.reset)
	case domain.IsRemovedLine(dl.Kind):
		return fmt.Sprintf("  %s%s%4d    ·%s %s┃ - %s%s",
			cursor, th.red, dl.OldLine, th.reset, th.bold+th.red, text, th.reset)
	default:
		textStyle := th.muted
		if selected {
			textStyle = th.bold + th.fg
		}
		return fmt.Sprintf("  %s%s%4d %4d │%s   %s%s%s",
			cursor, th.muted, dl.OldLine, dl.NewLine, th.reset, textStyle, text, th.reset)
	}
}

func (s *State) renderSplitHunkDiff(th ansiTheme, width int, active FileDiffHunk, totalHunks int) []string {
	halfW := max(26, (width-7)/2)
	leftHdr := theme.PadVisibleRight(" OLD │ BEFORE (ORIGINAL)", halfW)
	rightHdr := theme.PadVisibleRight(fmt.Sprintf(" NEW │ AFTER (HUNK #%d/%d)", active.GlobalIndex+1, totalHunks), halfW)
	lines := th.renderTableHeader("   "+leftHdr+" │ "+rightHdr, width)

	splitRows := domain.BuildSplitDiffRows(active.Hunk.Lines)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(splitRows), selected, s.adaptiveWindowRows(10, 14))
	for idx := start; idx < end; idx++ {
		r := splitRows[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		leftCell := formatSplitHalfCell(th, r.OldLine, r.OldKind, r.OldText, halfW, idx == selected)
		rightCell := formatSplitHalfCell(th, r.NewLine, r.NewKind, r.NewText, halfW, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%s %s│%s %s", cursor, leftCell, th.border, th.reset, rightCell))
	}
	return appendWindowFooter(th, lines, start, end, len(splitRows))
}

func formatSplitHalfCell(th ansiTheme, lineNum int, kind, rawText string, colWidth int, selected bool) string {
	codeCol := max(8, colWidth-9)
	text := theme.TruncatePlain(theme.SanitizeCodeText(rawText), codeCol)
	var cell string
	switch kind {
	case "delete":
		cell = fmt.Sprintf("%s-%4d │ %s%s", th.bold+th.red, lineNum, text, th.reset)
	case "add":
		cell = fmt.Sprintf("%s+%4d │ %s%s", th.bold+th.green, lineNum, text, th.reset)
	case "context":
		style := th.muted
		if selected {
			style = th.fg
		}
		cell = fmt.Sprintf("%s %4d │ %s%s%s", th.muted, lineNum, style, text, th.reset)
	default:
		cell = fmt.Sprintf("%s    · │%s", th.muted, th.reset)
	}
	return theme.PadVisibleRight(cell, colWidth)
}

func (s *State) renderUnifiedStreamDiff(th ansiTheme, width int, active FileDiffHunk, totalHunks int) []string {
	hdr := fmt.Sprintf("   OLD  NEW │ SIGN  UNIFIED DIFF STREAM (%d HUNKS ACROSS %d FILES)", totalHunks, active.FileTotal)
	lines := th.renderTableHeader(hdr, width)
	uRows := s.unifiedDiffRows()
	selected := s.ClampedIdx()
	codeCol := max(16, width-20)
	start, end := windowBounds(len(uRows), selected, s.adaptiveWindowRows(10, 14))
	for idx := start; idx < end; idx++ {
		r := uRows[idx]
		if r.IsHeader {
			cursor := th.renderCursorPrefix(idx == selected)
			activeTag := ""
			style := th.cyan
			if r.HunkGlobalIdx == active.GlobalIndex {
				activeTag = " [ACTIVE]"
				style = th.bold + th.accent
			}
			banner := theme.TruncatePlain(r.HeaderText+activeTag, max(20, width-10))
			lines = append(lines, fmt.Sprintf("  %s%s── %s ──%s", cursor, style, banner, th.reset))
			continue
		}
		lines = append(lines, formatDualNumberDiffLine(th, r.Line, idx == selected, codeCol))
	}
	return appendWindowFooter(th, lines, start, end, len(uRows))
}
