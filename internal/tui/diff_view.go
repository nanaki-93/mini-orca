package tui

import (
	"fmt"
	"strings"
)

// FileDiffHunk pairs a DiffHunk with its owning FileChange metadata across a multi-file proposal.
type FileDiffHunk struct {
	GlobalIndex   int
	FileIndex     int
	FileTotal     int
	Path          string
	Summary       string
	FileAdded     int
	FileRemoved   int
	FileHunkIndex int
	FileHunkTotal int
	Hunk          DiffHunk
}

// SplitDiffRow represents a paired side-by-side diff row (before on left, after on right).
type SplitDiffRow struct {
	OldLine int
	OldKind string
	OldText string
	NewLine int
	NewKind string
	NewText string
}

// UnifiedDiffRow represents a display row in the multi-file unified diff stream.
type UnifiedDiffRow struct {
	IsHeader      bool
	HunkGlobalIdx int
	FileIndex     int
	HeaderText    string
	Line          DiffLine
}

// CollectChangeHunks extracts all hunks across all modified files in a ChangeSession.
func CollectChangeHunks(changes []FileChange) []FileDiffHunk {
	if len(changes) == 0 {
		return nil
	}
	var out []FileDiffHunk
	for fIdx, fc := range changes {
		fAdd, fRem := countFileDiffLines(fc.Diff.Lines)
		hunks := SplitDiffIntoHunks(fc.Diff.Lines)
		for hIdx, h := range hunks {
			out = append(out, FileDiffHunk{
				GlobalIndex:   len(out),
				FileIndex:     fIdx,
				FileTotal:     len(changes),
				Path:          fc.Path,
				Summary:       fc.Summary,
				FileAdded:     fAdd,
				FileRemoved:   fRem,
				FileHunkIndex: hIdx,
				FileHunkTotal: len(hunks),
				Hunk:          h,
			})
		}
	}
	if len(out) > 0 {
		return out
	}
	return fallbackContextHunks(changes)
}

func fallbackContextHunks(changes []FileChange) []FileDiffHunk {
	for fIdx, fc := range changes {
		if len(fc.Diff.Lines) == 0 {
			continue
		}
		oldStart, oldEnd := lineBounds(fc.Diff.Lines, true)
		newStart, newEnd := lineBounds(fc.Diff.Lines, false)
		return []FileDiffHunk{
			{
				GlobalIndex:   0,
				FileIndex:     fIdx,
				FileTotal:     len(changes),
				Path:          fc.Path,
				Summary:       fc.Summary,
				FileHunkIndex: 0,
				FileHunkTotal: 1,
				Hunk: DiffHunk{
					Index:    0,
					OldStart: oldStart,
					OldEnd:   oldEnd,
					NewStart: newStart,
					NewEnd:   newEnd,
					Lines:    fc.Diff.Lines,
				},
			},
		}
	}
	return nil
}

func countFileDiffLines(lines []DiffLine) (added int, removed int) {
	for _, dl := range lines {
		if IsAddedLine(dl.Kind) {
			added++
		} else if IsRemovedLine(dl.Kind) {
			removed++
		}
	}
	return added, removed
}

// BuildSplitDiffRows pairs consecutive removed and added lines into side-by-side comparison rows.
func BuildSplitDiffRows(lines []DiffLine) []SplitDiffRow {
	var rows []SplitDiffRow
	for i := 0; i < len(lines); {
		dl := lines[i]
		if !IsRemovedLine(dl.Kind) && !IsAddedLine(dl.Kind) {
			rows = append(rows, SplitDiffRow{
				OldLine: dl.OldLine,
				OldKind: "context",
				OldText: dl.Text,
				NewLine: dl.NewLine,
				NewKind: "context",
				NewText: dl.Text,
			})
			i++
			continue
		}
		var dels, adds []DiffLine
		for i < len(lines) && (IsRemovedLine(lines[i].Kind) || IsAddedLine(lines[i].Kind)) {
			if IsRemovedLine(lines[i].Kind) {
				dels = append(dels, lines[i])
			} else {
				adds = append(adds, lines[i])
			}
			i++
		}
		rows = appendPairedChangeRows(rows, dels, adds)
	}
	return rows
}

func appendPairedChangeRows(rows []SplitDiffRow, dels, adds []DiffLine) []SplitDiffRow {
	count := max(len(dels), len(adds))
	for k := 0; k < count; k++ {
		var r SplitDiffRow
		if k < len(dels) {
			r.OldLine = dels[k].OldLine
			r.OldKind = "delete"
			r.OldText = dels[k].Text
		}
		if k < len(adds) {
			r.NewLine = adds[k].NewLine
			r.NewKind = "add"
			r.NewText = adds[k].Text
		}
		rows = append(rows, r)
	}
	return rows
}

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
	if s.Change == nil || len(s.Change.Changes) == 0 {
		return FileDiffHunk{}, nil, false
	}
	all := CollectChangeHunks(s.Change.Changes)
	if len(all) == 0 {
		return FileDiffHunk{}, nil, false
	}
	idx := min(max(0, s.HunkIdx), len(all)-1)
	return all[idx], all, true
}

func (s *State) unifiedDiffRows() []UnifiedDiffRow {
	if s.Change == nil || len(s.Change.Changes) == 0 {
		return nil
	}
	allHunks := CollectChangeHunks(s.Change.Changes)
	var rows []UnifiedDiffRow
	for _, fh := range allHunks {
		header := fmt.Sprintf("File %d/%d: %s · Hunk #%d/%d (@@ -%d,%d +%d,%d @@)",
			fh.FileIndex+1, fh.FileTotal, sanitizeCodeText(fh.Path),
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
		return max(1, len(BuildSplitDiffRows(active.Hunk.Lines)))
	case "unified":
		return max(1, len(s.unifiedDiffRows()))
	default:
		return max(1, len(active.Hunk.Lines))
	}
}

func firstChangedRowForMode(hunk DiffHunk, mode string) int {
	if mode == "split" {
		for idx, r := range BuildSplitDiffRows(hunk.Lines) {
			if r.OldKind == "delete" || r.NewKind == "add" {
				return idx
			}
		}
		return 0
	}
	for idx, dl := range hunk.Lines {
		if IsAddedLine(dl.Kind) || IsRemovedLine(dl.Kind) {
			return idx
		}
	}
	return 0
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
	s.SelectedIdx = firstChangedRowForMode(active.Hunk, mode)
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
	added, removed, badge := ClassifyDiffScope(s.Change.Changes)
	mode := s.EffectiveDiffMode()
	meta := s.buildDiffReviewMeta(th, active, len(allHunks), added, removed, badge)
	rightTag := fmt.Sprintf("%s · %s", diffModeDisplayName(mode), badge)
	lines := th.renderPanelBox("GUARDED DIFF REVIEW", rightTag, panelW, meta)
	lines = append(lines, s.renderDiffFileTabsAndNav(th, width, active, len(allHunks), mode)...)

	switch mode {
	case "split":
		return append(lines, s.renderSplitHunkDiff(th, width, active, len(allHunks))...)
	case "unified":
		return append(lines, s.renderUnifiedStreamDiff(th, width, active, len(allHunks))...)
	default:
		return append(lines, s.renderFocusedHunkDiff(th, width, active, len(allHunks))...)
	}
}

func (s *State) buildDiffReviewMeta(th ansiTheme, active FileDiffHunk, totalHunks, added, removed int, badge string) []string {
	fileScope := fmt.Sprintf("%sFile:%s %s%s%s", th.muted, th.reset, th.cyan, truncatePlain(active.Path, 26), th.reset)
	if active.FileTotal > 1 {
		fileScope = fmt.Sprintf("%sFile %d/%d:%s %s%s%s",
			th.muted, active.FileIndex+1, active.FileTotal, th.reset, th.cyan, truncatePlain(active.Path, 24), th.reset)
	}
	meta := []string{
		fmt.Sprintf(
			"%sHunk #%d/%d%s · %s (%s+%d%s/%s-%d%s) · %sProposal:%s %s%s%s",
			th.bold+th.accent, active.GlobalIndex+1, totalHunks, th.reset,
			fileScope,
			th.green, added, th.reset, th.red, removed, th.reset,
			th.muted, th.reset, th.bold+th.fg, truncatePlain(s.Change.Title, 26), th.reset,
		),
	}
	if s.ShowInlineWhy && active.Summary != "" {
		meta = append(meta, fmt.Sprintf("%sRationale:%s %s%s%s", th.bold+th.accent, th.reset, th.fg, sanitizeCodeText(active.Summary), th.reset))
	}
	return meta
}

func (s *State) renderDiffFileTabsAndNav(th ansiTheme, width int, active FileDiffHunk, totalHunks int, mode string) []string {
	var filePills []string
	for idx, fc := range s.Change.Changes {
		fAdd, fRem := countFileDiffLines(fc.Diff.Lines)
		shortPath := truncatePlain(fc.Path, 20)
		if idx == active.FileIndex {
			filePills = append(filePills, fmt.Sprintf("%s▸ [%d/%d] %s%s %s(+%d/-%d)%s",
				th.bold+th.accent, idx+1, len(s.Change.Changes), shortPath, th.reset, th.green, fAdd, fRem, th.reset))
		} else {
			filePills = append(filePills, fmt.Sprintf("%s[%d/%d] %s (+%d/-%d)%s",
				th.muted, idx+1, len(s.Change.Changes), shortPath, fAdd, fRem, th.reset))
		}
	}
	fileRow := fmt.Sprintf("  %sFILES:%s %s", th.bold+th.cyan, th.reset, strings.Join(filePills, "  "))
	navRow := fmt.Sprintf(
		"  %sKEYS:%s  %s[n/N]%s Hunk #%d/%d  %s[f/F]%s File %d/%d  %s[j/k]%s Line  %s[v]%s View:%s%s%s  %s[x]%s Reject  %s[c]%s Apply",
		th.bold+th.cyan, th.reset,
		th.cyan, th.reset, active.GlobalIndex+1, totalHunks,
		th.cyan, th.reset, active.FileIndex+1, active.FileTotal,
		th.cyan, th.reset,
		th.cyan, th.reset, th.bold+th.amber, strings.ToUpper(mode), th.reset,
		th.cyan, th.reset,
		th.cyan, th.reset,
	)
	return []string{
		padVisibleRight(fileRow, max(20, width-2)),
		padVisibleRight(navRow, max(20, width-2)),
	}
}

func (s *State) renderFocusedHunkDiff(th ansiTheme, width int, active FileDiffHunk, totalHunks int) []string {
	hdr := fmt.Sprintf("   OLD  NEW │ SIGN  FOCUSED HUNK #%d/%d · %s",
		active.GlobalIndex+1, totalHunks, sanitizeCodeText(active.Path))
	lines := th.renderTableHeader(hdr, width)
	hunkLines := active.Hunk.Lines
	selected := s.ClampedIdx()
	codeCol := max(16, width-20)
	start, end := windowBounds(len(hunkLines), selected, 12)
	for idx := start; idx < end; idx++ {
		lines = append(lines, formatDualNumberDiffLine(th, hunkLines[idx], idx == selected, codeCol))
	}
	return appendWindowFooter(th, lines, start, end, len(hunkLines))
}

func formatDualNumberDiffLine(th ansiTheme, dl DiffLine, selected bool, codeCol int) string {
	cursor := th.renderCursorPrefix(selected)
	text := truncatePlain(sanitizeCodeText(dl.Text), codeCol)
	switch {
	case IsAddedLine(dl.Kind):
		return fmt.Sprintf("  %s%s   · %4d%s %s┃ + %s%s",
			cursor, th.green, dl.NewLine, th.reset, th.bold+th.green, text, th.reset)
	case IsRemovedLine(dl.Kind):
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
	leftHdr := padVisibleRight(" OLD │ BEFORE (ORIGINAL)", halfW)
	rightHdr := padVisibleRight(fmt.Sprintf(" NEW │ AFTER (HUNK #%d/%d)", active.GlobalIndex+1, totalHunks), halfW)
	lines := th.renderTableHeader("   "+leftHdr+" │ "+rightHdr, width)

	splitRows := BuildSplitDiffRows(active.Hunk.Lines)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(splitRows), selected, 12)
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
	text := truncatePlain(sanitizeCodeText(rawText), codeCol)
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
	return padVisibleRight(cell, colWidth)
}

func (s *State) renderUnifiedStreamDiff(th ansiTheme, width int, active FileDiffHunk, totalHunks int) []string {
	hdr := fmt.Sprintf("   OLD  NEW │ SIGN  UNIFIED DIFF STREAM (%d HUNKS ACROSS %d FILES)", totalHunks, active.FileTotal)
	lines := th.renderTableHeader(hdr, width)
	uRows := s.unifiedDiffRows()
	selected := s.ClampedIdx()
	codeCol := max(16, width-20)
	start, end := windowBounds(len(uRows), selected, 12)
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
			banner := truncatePlain(r.HeaderText+activeTag, max(20, width-10))
			lines = append(lines, fmt.Sprintf("  %s%s── %s ──%s", cursor, style, banner, th.reset))
			continue
		}
		lines = append(lines, formatDualNumberDiffLine(th, r.Line, idx == selected, codeCol))
	}
	return appendWindowFooter(th, lines, start, end, len(uRows))
}
