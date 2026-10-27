package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const branchesSplit = 0.4

// renderBranchesWorkspace lists the checkouts agents work in beside the
// selected one's integration state, tasks and next step.
func (s *State) renderBranchesWorkspace(th ansiTheme, width, rows int) []string {
	listW := listPaneWidth(width, branchesSplit)
	detailW := detailPaneWidth(width, branchesSplit)
	s.viewportBodyRows = listPaneRows(width, rows)
	var trees []GitWorktree
	if s.GitWorkspace != nil {
		trees = s.GitWorkspace.Worktrees
	}
	selected := s.ClampedIdx()
	list := []string{}
	if s.Repository != nil && s.Repository.MergeHead != "" {
		list = append(list, th.red+"▲ merge in progress"+th.reset+th.muted+" · resolve, commit, then R"+th.reset, "")
	}
	if len(trees) == 0 {
		list = append(list, emptyPane(th, "No Git worktrees yet", "Tasks that run in isolation get their own", "worktree and branch; they appear here.", "r  refresh")...)
	}
	offset := len(list)
	for i, wt := range trees {
		status := worktreeStatus(wt)
		glyph, tone := worktreeGlyph(th, status)
		name := client.FirstNonEmpty(wt.Branch, path.Base(wt.Path), "detached")
		left := tone + glyph + th.reset + " " + rowTitleStyle(th, i == selected) + theme.SanitizeSingleLine(name) + th.reset
		if wt.Current {
			left += th.muted + "  main checkout" + th.reset
		}
		list = append(list, th.listRow(i == selected, rightAlign(left, tone+strings.ToLower(strings.ReplaceAll(status, "_", " "))+th.reset, listW-1), listW))
	}
	left := theme.Pane{Title: s.viewTabsFit(th, listW-18), Meta: fmt.Sprintf("%d checkouts", len(trees)), Body: list, Focused: s.Page != PageWorktreeDetail, Scroll: s.listScroll(offset+selected, len(list), s.viewportBodyRows)}
	right := theme.Pane{Title: "Worktree", Body: emptyPane(th, "Select a worktree", "Its branch, tasks and merge state appear here.")}
	if wt := s.selectedWorktree(); wt != nil {
		right.Title = th.purple + "⎇ " + th.reset + th.bold + th.fg + theme.TruncatePlain(client.FirstNonEmpty(wt.Branch, "detached"), max(10, detailW-20)) + th.reset
		right.Meta = strings.ToLower(strings.ReplaceAll(worktreeStatus(*wt), "_", " "))
		right.Body = s.worktreeDetailBody(th, wt, detailW)
		right.Focused = s.Page == PageWorktreeDetail
		s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(right.Body), detailPaneRows(width, rows))
		right.Scroll = s.DetailScroll
	}
	return s.splitPanes(th, width, rows, branchesSplit, left, right)
}

func worktreeGlyph(th ansiTheme, status string) (string, string) {
	switch status {
	case "CURRENT":
		return "●", th.accent
	case "TO_MERGE":
		return "⇡", th.amber
	case "INTEGRATED":
		return "✔", th.green
	case "FAILED", "UNAVAILABLE", "DIRTY":
		return "✖", th.red
	case "LOCKED":
		return "⊘", th.muted
	default:
		return "○", th.muted
	}
}

func (s *State) worktreeDetailBody(th ansiTheme, wt *GitWorktree, width int) []string {
	labelW := 9
	body := []string{
		th.field("Path", th.fg+theme.TruncatePlain(wt.Path, max(10, width-labelW-1))+th.reset, labelW),
		th.field("Commit", th.fg+shortID(client.FirstNonEmpty(wt.Head, "—"))+th.reset, labelW),
	}
	if wt.Integration != "" {
		body = append(body, th.field("Branch", th.fg+wt.Integration+th.reset, labelW))
	}
	if wt.Error != "" {
		body = append(body, wrapText(th.field("Error", th.red+theme.SanitizeSingleLine(wt.Error)+th.reset, labelW), width)...)
	}
	tasks := append(append([]ChangeHistoryEntry{}, wt.Sessions...), wt.Completed...)
	if len(tasks) > 0 {
		body = append(body, "", th.sectionHeader("Tasks on this branch", len(tasks)))
		for _, entry := range tasks {
			tone := s.toneForTask(th, client.FirstNonEmpty(entry.WorkflowStatus, entry.State))
			body = append(body, rightAlign(tone.color+tone.glyph+th.reset+" "+th.fg+theme.SanitizeSingleLine(entry.Title)+th.reset, tone.color+tone.label+th.reset, width))
			if reason := strings.TrimSpace(entry.WorkflowReason); reason != "" {
				body = append(body, wrapText("  "+th.muted+theme.SanitizeSingleLine(reason)+th.reset, width)...)
			}
		}
	}
	if s.Repository != nil && s.Repository.MergeHead != "" && len(s.Repository.Conflicts) > 0 {
		body = append(body, "", th.sectionHeader("Unresolved files", len(s.Repository.Conflicts)))
		for _, p := range s.Repository.Conflicts {
			body = append(body, th.red+"✖ "+th.reset+th.fg+p+th.reset)
		}
	} else if wt.Integration == "behind" && len(wt.OverlapPaths) > 0 {
		body = append(body, "", th.sectionHeader("Changed on both branches", len(wt.OverlapPaths)))
		for _, p := range wt.OverlapPaths {
			body = append(body, th.amber+"◌ "+th.reset+th.fg+p+th.reset)
		}
	}
	body = append(body, "", th.sectionHeader("Next", -1))
	body = append(body, wrapText(th.fg+s.worktreeNextAction(wt)+th.reset, width)...)
	body = append(body, "", th.keyHint("m", "merge", "x", "reject", "c", "clean up", "r", "refresh", "R", "update index"))
	return body
}
