package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// diffSidebarMinWidth is the content width at which the diff view gains a
// file list beside it, like the file tree of a code review tool.
const diffSidebarMinWidth = 120

func diffSidebarWidth(width int) int {
	return min(38, max(26, width/4))
}

// renderDiffWithSidebar lays the changed-file list next to the diff so the
// reviewer always sees which files remain, without leaving the hunk.
func (s *State) renderDiffWithSidebar(th ansiTheme, width int) []string {
	sideW := diffSidebarWidth(width)
	diffW := width - sideW - 3
	side := s.renderDiffFileList(th, sideW)
	diff := s.renderChangeDiffLines(th, diffW)
	selectedPrefix := "  " + th.renderCursorPrefix(true)
	rows := max(len(side), len(diff))
	out := make([]string, 0, rows)
	for i := range rows {
		left, right := "", ""
		if i < len(side) {
			left = side[i]
		}
		if i < len(diff) {
			right = diff[i]
			if strings.HasPrefix(right, selectedPrefix) {
				right = theme.PaintLine(th.selection, th.fg, right, diffW)
			}
		}
		out = append(out, theme.PadVisibleRight(theme.TruncateVisible(left, sideW), sideW)+" "+th.border+"│"+th.reset+" "+right)
	}
	return out
}

// renderDiffFileList lists each changed file with its review state and size.
func (s *State) renderDiffFileList(th ansiTheme, width int) []string {
	changes := s.diffChanges()
	active, _, _ := s.activeDiffHunk()
	reviewedAtHash := s.Change != nil && s.Change.FileReviewHash != "" && s.Change.FileReviewHash == s.Change.Hash
	reviewed, activeRow := 0, 0
	var rows []string
	for i, fc := range changes {
		done := reviewedAtHash && domain.ContainsString(s.Change.ReviewedFiles, fc.Path)
		if done {
			reviewed++
		}
		mark, tone := th.muted+"○", th.fg
		switch {
		case i == active.FileIndex && fc.Path == active.Path:
			mark, tone, activeRow = th.accent+"●", th.bold+th.fg, len(rows)
		case done:
			mark, tone = th.green+"✓", th.muted
		}
		added, removed := diffLineCounts(fc)
		counts := fmt.Sprintf("%s+%d%s %s-%d%s", th.green, added, th.reset, th.red, removed, th.reset)
		name := theme.TruncatePlain(path.Base(fc.Path), max(6, width-14))
		row := fmt.Sprintf("  %s%s %s%s%s", mark, th.reset, tone, name, th.reset)
		rows = append(rows, theme.PadVisibleRight(row, max(1, width-theme.VisibleRuneCount(counts)-1))+counts)
		if dir := path.Dir(fc.Path); dir != "." {
			rows = append(rows, "    "+th.muted+theme.TruncatePlain(dir, max(6, width-5))+th.reset)
		}
	}
	head := []string{
		fmt.Sprintf("  %sFILES%s %s%d/%d reviewed%s", th.bold+th.cyan, th.reset, th.muted, reviewed, len(changes), th.reset),
		"  " + th.border + strings.Repeat("─", max(4, width-3)) + th.reset,
	}
	foot := []string{"", "  " + th.muted + "[f] Next · [W] Unreviewed" + th.reset}
	// Keep the list inside the viewport, centred on the file being read.
	if budget := s.viewportBodyRows - len(head) - len(foot) - 1; budget > 0 && len(rows) > budget {
		start, end := windowBounds(len(rows), activeRow, budget)
		rows = rows[start:end]
	}
	return append(append(head, rows...), foot...)
}

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
