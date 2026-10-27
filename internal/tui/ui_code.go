package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// codeSplit is the file list share of the Code workspace.
const codeSplit = 0.3

// renderCodeWorkspace is review when a proposal is open (changed files
// beside the diff) and a read-only explorer otherwise (files beside the
// source of the open file).
func (s *State) renderCodeWorkspace(th ansiTheme, width, rows int) []string {
	if s.isViewingDiff() {
		return s.renderReviewWorkspace(th, width, rows)
	}
	return s.renderExplorerWorkspace(th, width, rows)
}

// renderReviewWorkspace lists changed files with their review state beside
// the active file's diff, like the Files changed tab of a code review.
func (s *State) renderReviewWorkspace(th ansiTheme, width, rows int) []string {
	active, all, _ := s.activeDiffHunk()
	changes := s.diffChanges()
	listW := listPaneWidth(width, codeSplit)
	diffW := detailPaneWidth(width, codeSplit)
	s.viewportBodyRows = detailPaneRows(width, rows)

	reviewed, total := s.reviewCountFor(s.Change)
	list := []string{th.bold + th.fg + theme.TruncatePlain(theme.SanitizeSingleLine(s.Change.Title), listW) + th.reset}
	list = append(list, th.renderProgressLine(reviewed, total, listW), "")
	cursorRow := 0
	for i, fc := range changes {
		done := s.Change.FileReviewHash == s.Change.Hash && domain.ContainsString(s.Change.ReviewedFiles, fc.Path)
		mark := th.muted + "○" + th.reset
		if done {
			mark = th.green + "✔" + th.reset
		}
		isActive := i == active.FileIndex
		if isActive {
			cursorRow = len(list)
		}
		a, r := diffLineCounts(fc)
		name := mark + " " + rowTitleStyle(th, isActive) + path.Base(fc.Path) + th.reset
		list = append(list, th.listRow(isActive, rightAlign(name, th.diffStat(a, r), listW-1), listW))
		if dir := path.Dir(fc.Path); dir != "." {
			dirRow := "  " + th.muted + theme.TruncatePlain(dir, listW-3) + th.reset
			if isActive {
				dirRow = theme.PaintLine(th.selection, th.fg, th.accent+"▌"+th.reset+th.selection+" "+th.muted+theme.TruncatePlain(dir, listW-3)+th.reset, listW)
			} else {
				dirRow = " " + dirRow
			}
			list = append(list, dirRow)
		}
	}
	list = append(list, "")
	list = append(list, s.reviewListFooter(th, listW)...)
	left := theme.Pane{Title: "Changed files", Meta: fmt.Sprintf("%d/%d viewed", reviewed, total), Body: list, Scroll: s.listScroll(cursorRow, len(list), listPaneRows(width, rows))}

	body := s.renderDiffBody(th, active, len(all), diffW, detailPaneRows(width, rows))
	meta := diffModeDisplayName(s.EffectiveDiffMode())
	if s.DiffSinceReview {
		meta += " · since review"
	}
	fileTitle := th.muted + theme.TruncatePlain(path.Dir(active.Path)+"/", max(4, diffW/3)) + th.reset + th.bold + th.fg + path.Base(active.Path) + th.reset
	if path.Dir(active.Path) == "." {
		fileTitle = th.bold + th.fg + active.Path + th.reset
	}
	right := theme.Pane{Title: fileTitle + "  " + th.diffStat(active.FileAdded, active.FileRemoved), Meta: meta, Body: body.lines, Focused: true, Scroll: body.scroll,
		Footer: fmt.Sprintf("hunk %d/%d · file %d/%d", active.GlobalIndex+1, len(all), active.FileIndex+1, max(1, active.FileTotal))}
	return s.splitPanes(th, width, rows, codeSplit, left, right)
}

// renderProgressLine is a compact "n/m viewed" bar.
func (th ansiTheme) renderProgressLine(done, total, width int) string {
	barW := max(6, min(24, width-14))
	filled := 0
	if total > 0 {
		filled = barW * done / total
	}
	bar := th.green + strings.Repeat("━", filled) + th.reset + th.border + strings.Repeat("━", barW-filled) + th.reset
	return bar + th.muted + fmt.Sprintf("  %d/%d viewed", done, total) + th.reset
}

// reviewListFooter summarizes checks and what Apply needs, under the files.
func (s *State) reviewListFooter(th ansiTheme, width int) []string {
	out := []string{}
	if len(s.Change.Checks) > 0 {
		passed, total := domain.CountPassedChecks(s.Change.Checks)
		tone := th.green
		if passed < total {
			tone = th.red
		}
		out = append(out, th.field("Checks", tone+fmt.Sprintf("%d/%d passed", passed, total)+th.reset, 8))
	} else {
		out = append(out, th.field("Checks", th.muted+"not run"+th.reset, 8))
	}
	out = append(out, wrapText(th.muted+s.diffApplyDestination()+th.reset, width)...)
	return out
}

type diffBody struct {
	lines  []string
	scroll int
}

// renderDiffBody draws the active diff in the selected layout: the focused
// hunk, every hunk as one stream, or the hunk side by side. With details on
// (e), the rationale and the criteria this file claims to meet take at most a
// third of the pane, scrolled with PgUp/PgDn, so code always stays in view.
func (s *State) renderDiffBody(th ansiTheme, active FileDiffHunk, totalHunks, width, rows int) diffBody {
	why := s.diffWhyLines(th, active, width)
	if len(why) > 0 {
		budget := max(2, rows/3)
		if len(why) > budget {
			s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(why), budget-1)
			end := min(len(why), s.DetailScroll+budget-1)
			why = append(append([]string(nil), why[s.DetailScroll:end]...), th.muted+fmt.Sprintf("PgUp/PgDn details %d–%d of %d", s.DetailScroll+1, end, len(why))+th.reset)
		} else {
			s.DetailScroll = 0
		}
		why = append(why, "")
	}
	selected := s.ClampedIdx()
	var lines []string
	cursor := 0
	switch s.EffectiveDiffMode() {
	case "split":
		lines = append(lines, th.hunkHeader(active, width))
		for idx, r := range domain.BuildSplitDiffRows(active.Hunk.Lines) {
			lines = append(lines, th.splitDiffRow(r, idx == selected, s.DiffOffset, width))
		}
		cursor = selected + 1
	case "unified":
		for idx, r := range s.unifiedDiffRows() {
			if r.IsHeader {
				lines = append(lines, th.streamHeader(r, r.HunkGlobalIdx == active.GlobalIndex, idx == selected, width))
			} else {
				lines = append(lines, th.unifiedDiffRow(shiftedDiffLine(r.Line, s.DiffOffset), idx == selected, width))
			}
		}
		cursor = selected
	default:
		lines = append(lines, th.hunkHeader(active, width))
		for idx, dl := range active.Hunk.Lines {
			lines = append(lines, th.unifiedDiffRow(shiftedDiffLine(dl, s.DiffOffset), idx == selected, width))
		}
		cursor = selected + 1
	}
	codeRows := max(1, rows-len(why))
	scroll := s.listScrollFor(PageEditor, cursor, len(lines), codeRows)
	visible := lines[scroll:min(len(lines), scroll+codeRows)]
	return diffBody{lines: append(why, visible...), scroll: 0}
}

// diffWhyLines is the rationale for the active file and the acceptance
// criteria it claims to meet, shown when details are on.
func (s *State) diffWhyLines(th ansiTheme, active FileDiffHunk, width int) []string {
	if !s.ShowInlineWhy {
		return nil
	}
	var out []string
	if summary := strings.TrimSpace(active.Summary); summary != "" {
		out = append(out, wrapText(th.accent+"Why "+th.reset+th.fg+theme.SanitizeCodeText(summary)+th.reset, width)...)
	}
	for _, file := range s.diffChanges() {
		if file.Path != active.Path {
			continue
		}
		for _, n := range file.Criteria {
			if n > 0 && n <= len(s.Change.AcceptanceCriteria) {
				out = append(out, wrapText(th.purple+"Meets "+th.reset+th.fg+fmt.Sprintf("%d. ", n)+theme.SanitizeSingleLine(s.Change.AcceptanceCriteria[n-1])+th.reset, width)...)
			}
		}
	}
	return out
}

// listScrollFor is listScroll for a page other than the effective one.
func (s *State) listScrollFor(page Page, cursorRow, total, rows int) int {
	if s.viewport.lists == nil {
		s.viewport.lists = map[Page]int{}
	}
	key := page + "#detail"
	next := theme.FollowScroll(s.viewport.lists[key], cursorRow, total, rows)
	s.viewport.lists[key] = next
	return next
}

func (th ansiTheme) hunkHeader(active FileDiffHunk, width int) string {
	text := fmt.Sprintf("@@ lines %d–%d → %d–%d @@  hunk %d of %d in this file", active.Hunk.OldStart, active.Hunk.OldEnd, active.Hunk.NewStart, active.Hunk.NewEnd, active.FileHunkIndex+1, max(1, active.FileHunkTotal))
	return theme.PaintLine(th.surface, th.cyan, th.cyan+theme.TruncatePlain(text, width)+th.reset, width)
}

func (th ansiTheme) streamHeader(r UnifiedDiffRow, active, selected bool, width int) string {
	style := th.cyan
	if active {
		style = th.bold + th.accent
	}
	line := style + "── " + theme.TruncatePlain(theme.SanitizeCodeText(r.HeaderText), max(10, width-6)) + th.reset
	if selected {
		return theme.PaintLine(th.selection, th.fg, line, width)
	}
	return theme.PaintLine(th.surface, th.fg, line, width)
}

// unifiedDiffRow renders one diff line with both line numbers, a sign and a
// tinted background for additions and removals.
func (th ansiTheme) unifiedDiffRow(dl DiffLine, selected bool, width int) string {
	code := theme.TruncatePlain(theme.SanitizeCodeText(dl.Text), max(8, width-13))
	marker := " "
	if selected {
		marker = th.accent + "▌" + th.reset
	}
	num := func(n int) string {
		if n <= 0 {
			return "    "
		}
		return fmt.Sprintf("%4d", n)
	}
	switch {
	case domain.IsAddedLine(dl.Kind):
		row := th.muted + "    " + " " + num(dl.NewLine) + th.reset + " " + th.green + "+" + th.reset + " " + th.fg + code + th.reset
		return marker + theme.PaintLine(th.addBg, th.fg, row, width-1)
	case domain.IsRemovedLine(dl.Kind):
		row := th.muted + num(dl.OldLine) + " " + "    " + th.reset + " " + th.red + "−" + th.reset + " " + th.fg + code + th.reset
		return marker + theme.PaintLine(th.delBg, th.fg, row, width-1)
	default:
		row := th.muted + num(dl.OldLine) + " " + num(dl.NewLine) + th.reset + "   " + th.fg + code + th.reset
		if selected {
			return marker + theme.PaintLine(th.selection, th.fg, row, width-1)
		}
		return marker + row
	}
}

// splitDiffRow renders the old and new side of one aligned row.
func (th ansiTheme) splitDiffRow(r SplitDiffRow, selected bool, offset, width int) string {
	half := max(12, (width-3)/2)
	cell := func(num int, kind, text string) string {
		code := theme.TruncatePlain(shiftedCode(text, offset), max(4, half-7))
		n := "     "
		if num > 0 {
			n = fmt.Sprintf("%4d ", num)
		}
		switch kind {
		case "add":
			return theme.PaintLine(th.addBg, th.fg, th.muted+n+th.reset+th.green+"+"+th.reset+" "+th.fg+code+th.reset, half)
		case "delete":
			return theme.PaintLine(th.delBg, th.fg, th.muted+n+th.reset+th.red+"−"+th.reset+" "+th.fg+code+th.reset, half)
		case "context":
			return theme.PadVisibleRight(th.muted+n+th.reset+"  "+th.fg+code+th.reset, half)
		default:
			return theme.PaintLine(th.surface, th.fg, "", half)
		}
	}
	marker := " "
	if selected {
		marker = th.accent + "▌" + th.reset
	}
	return marker + cell(r.OldLine, r.OldKind, r.OldText) + " " + th.border + "│" + th.reset + cell(r.NewLine, r.NewKind, r.NewText)
}

// renderExplorerWorkspace lists indexed files beside the open file's source.
func (s *State) renderExplorerWorkspace(th ansiTheme, width, rows int) []string {
	listW := listPaneWidth(width, codeSplit)
	codeW := detailPaneWidth(width, codeSplit)
	s.viewportBodyRows = detailPaneRows(width, rows)
	reading := !s.BrowseFiles && s.File != nil
	var files []IndexedFile
	if s.Index != nil {
		files = s.Index.Files
	}
	listCursor := s.ClampedIdx()
	if reading {
		listCursor = -1
		for i, f := range files {
			if f.Path == s.File.Path {
				listCursor = i
				break
			}
		}
	}
	var list []string
	if len(files) == 0 {
		list = emptyPane(th, "No indexed files", ":reindex  index the project", ":open <path>  open a file")
	}
	for i, f := range files {
		dir := path.Dir(f.Path)
		name := rowTitleStyle(th, i == listCursor) + path.Base(f.Path) + th.reset
		if dir != "." {
			name += " " + th.muted + dir + th.reset
		}
		list = append(list, th.listRow(i == listCursor, name, listW))
	}
	left := theme.Pane{Title: "Files", Meta: fmt.Sprintf("%d", len(files)), Body: list, Focused: !reading, Scroll: s.listScroll(max(0, listCursor), len(list), listPaneRows(width, rows))}

	right := theme.Pane{Title: "Source", Body: s.explorerHint(th, files, listCursor, codeW), Focused: reading}
	if reading {
		lines := strings.Split(strings.ReplaceAll(s.File.Content, "\r\n", "\n"), "\n")
		gutter := len(fmt.Sprint(len(lines)))
		selected := s.ClampedIdx()
		var body []string
		for idx, raw := range lines {
			code := theme.TruncatePlain(shiftedCode(raw, s.DiffOffset), max(8, codeW-gutter-3))
			num := th.muted + fmt.Sprintf("%*d", gutter, idx+1) + th.reset
			if idx == selected {
				body = append(body, theme.PaintLine(th.selection, th.fg, th.accent+fmt.Sprintf("%*d", gutter, idx+1)+th.reset+th.selection+"  "+th.fg+code, codeW))
				continue
			}
			body = append(body, num+"  "+th.fg+code+th.reset)
		}
		right.Title = th.bold + th.fg + theme.SanitizeCodeText(s.File.Path) + th.reset
		right.Meta = fmt.Sprintf("%s · %d lines · read-only", client.FirstNonEmpty(s.File.Language, "text"), len(lines))
		right.Body = body
		right.Scroll = s.listScrollFor(PageEditor, selected, len(body), detailPaneRows(width, rows))
	}
	return s.splitPanes(th, width, rows, codeSplit, left, right)
}

func (s *State) explorerHint(th ansiTheme, files []IndexedFile, cursor, width int) []string {
	if cursor < 0 || cursor >= len(files) {
		return emptyPane(th, "Open a file", "Enter  read the selected file", "d  review the current proposal")
	}
	f := files[cursor]
	body := []string{th.bold + th.fg + theme.SanitizeCodeText(f.Path) + th.reset, ""}
	body = append(body, th.field("Language", th.fg+client.FirstNonEmpty(f.Language, "text")+th.reset, 10))
	body = append(body, th.field("Lines", th.fg+fmt.Sprint(f.LineCount)+th.reset, 10))
	if issues := s.issuesForPath(f.Path); len(issues) > 0 {
		body = append(body, th.field("Issues", th.amber+fmt.Sprintf("%d open", len(issues))+th.reset, 10))
	}
	body = append(body, "", th.keyHint("Enter", "read", ":agent", "new task with this file", ":web files", "open in browser"))
	return body
}

// issuesForPath lists open issues located in path.
func (s *State) issuesForPath(p string) []IssueRow {
	var out []IssueRow
	for _, issue := range s.issuesInView("open") {
		if issue.Path == p {
			out = append(out, issue)
		}
	}
	return out
}

// diffLineCounts counts added and removed lines of one changed file.
func diffLineCounts(fc FileChange) (added, removed int) {
	for _, line := range fc.Diff.Lines {
		switch {
		case domain.IsAddedLine(line.Kind):
			added++
		case domain.IsRemovedLine(line.Kind):
			removed++
		}
	}
	return added, removed
}
