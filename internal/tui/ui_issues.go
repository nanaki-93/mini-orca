package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// issuesSplit is the list share of the Issues workspace.
const issuesSplit = 0.45

// renderIssuesWorkspace lists findings with their filters beside the
// selected finding's problem, evidence and proposed fix.
func (s *State) renderIssuesWorkspace(th ansiTheme, width, rows int) []string {
	open, inProgress, done := s.IssueStatusCounts()
	issues := s.FilteredIssues()
	listW := listPaneWidth(width, issuesSplit)
	detailW := detailPaneWidth(width, issuesSplit)
	s.viewportBodyRows = listPaneRows(width, rows)
	selected := s.ClampedIdx()

	list := []string{s.issueFilterStrip(th, open, inProgress, done), th.muted + "category " + th.reset + th.fg + s.IssueCategory + th.reset + th.muted + fmt.Sprintf("  ·  %d selected for a batch", len(s.SelectedIssueIDs)) + th.reset, ""}
	if len(issues) == 0 {
		list = append(list, emptyPane(th, "No findings match these filters", "s  change status", "t  change category", "r  analyze the project")...)
	}
	for i, row := range issues {
		mark := th.muted + "·" + th.reset
		if domain.ContainsString(s.SelectedIssueIDs, row.ID) {
			mark = th.green + "■" + th.reset
		}
		loc := th.muted + path.Base(row.Path) + th.reset
		if row.Line > 0 {
			loc = th.muted + fmt.Sprintf("%s:%d", path.Base(row.Path), row.Line) + th.reset
		}
		left := mark + " " + th.severityChip(row.Severity) + " " + rowTitleStyle(th, i == selected) + theme.SanitizeSingleLine(row.Name) + th.reset
		if listW < 56 {
			loc = ""
		}
		list = append(list, th.listRow(i == selected, rightAlign(left, loc, listW-1), listW))
	}
	focusDetail := s.Page == PageIssueDetail
	left := theme.Pane{Title: s.viewTabsFit(th, listW-16), Meta: fmt.Sprintf("%d shown", len(issues)), Body: list, Focused: !focusDetail, Scroll: s.listScroll(selected+3, len(list), s.viewportBodyRows)}
	right := theme.Pane{Title: "Finding", Body: emptyPane(th, "Select a finding", "Its cause, evidence and proposed fix appear here.")}
	if cur := s.SelectedIssue(); cur != nil {
		right.Title = th.severityChip(cur.Severity) + " " + th.bold + th.fg + theme.TruncatePlain(theme.SanitizeSingleLine(cur.Name), max(10, detailW-16)) + th.reset
		right.Meta = strings.ReplaceAll(cur.Status, "_", " ")
		right.Body = s.issueDetailBody(th, *cur, detailW)
		right.Focused = focusDetail
		s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(right.Body), detailPaneRows(width, rows))
		right.Scroll = s.DetailScroll
	}
	return s.splitPanes(th, width, rows, issuesSplit, left, right)
}

func (s *State) issueFilterStrip(th ansiTheme, open, inProgress, done int) string {
	tabs := []struct {
		id, label string
		count     int
	}{{"open", "Open", open}, {"in_progress", "In progress", inProgress}, {"done", "Done", done}}
	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("%s %d", t.label, t.count)
		if s.IssueView == t.id {
			parts = append(parts, th.bold+th.accent+label+th.reset)
		} else {
			parts = append(parts, th.muted+label+th.reset)
		}
	}
	if s.IssueView == "all" {
		parts = append(parts, th.bold+th.accent+fmt.Sprintf("All %d", open+inProgress+done)+th.reset)
	}
	return strings.Join(parts, th.border+"  │  "+th.reset)
}

// severityChip is a fixed-width colored severity label.
func (th ansiTheme) severityChip(sev string) string {
	label := strings.ToLower(strings.TrimSpace(sev))
	if label == "" {
		label = "info"
	}
	cell := theme.PadVisibleRight(theme.TruncatePlain(label, 8), 8)
	switch label {
	case "critical":
		return th.critBg + " " + strings.TrimRight(cell, " ") + " " + th.reset + strings.Repeat(" ", 8-len(label))
	case "high":
		return th.red + cell + th.reset
	case "medium", "med":
		return th.amber + cell + th.reset
	case "low":
		return th.cyan + cell + th.reset
	default:
		return th.muted + cell + th.reset
	}
}

// issueDetailBody stacks the finding's facts, cause, evidence and proposed
// solution so one scroll reads the whole case.
func (s *State) issueDetailBody(th ansiTheme, cur IssueRow, width int) []string {
	body := s.issueFacts(th, cur, width)
	body = append(body, "", th.sectionHeader("Problem", -1))
	body = append(body, wrapText(th.fg+formatIssueRootCauseText(cur)+th.reset, width)...)
	if strings.TrimSpace(cur.Evidence) != "" {
		body = append(body, "", th.sectionHeader("Evidence", -1))
		body = append(body, wrapText(th.fg+theme.SanitizeSingleLine(cur.Evidence)+th.reset, width)...)
	}
	body = append(body, s.issueSourceLines(th, cur, width)...)
	body = append(body, s.issueFixLines(th, cur, width)...)
	switch cur.Status {
	case "in_progress":
		body = append(body, "", th.amber+s.spinner()+th.reset+" "+th.fg+"A task is working on this"+th.reset+th.muted+"  "+cur.AgentPath+th.reset)
	case "done":
		body = append(body, "", th.green+"✔ "+th.reset+th.fg+"Verified and fixed"+th.reset)
	}
	return append(body, "", th.keyHint("f", "plan a fix", "Space", "add to batch", "a", "plan the batch", "o", "open file", "d", "dismiss"))
}

// issueFacts is the finding's title, location and provenance.
func (s *State) issueFacts(th ansiTheme, cur IssueRow, width int) []string {
	body := wrapText(th.bold+th.fg+theme.SanitizeSingleLine(cur.Name)+th.reset, width)
	body = append(body, th.cyan+formatIssueLocationSpan(cur)+th.reset)
	facts := []string{strings.ToUpper(cur.Category), formatIssueOrigin(cur)}
	if cur.Confidence != "" {
		facts = append(facts, cur.Confidence+" confidence")
	}
	if cur.Freshness != "" && cur.Freshness != "current" && cur.Freshness != "fresh" {
		facts = append(facts, th.amber+cur.Freshness+th.reset)
	}
	body = append(body, th.muted+strings.Join(facts, "  ·  ")+th.reset)
	var more []string
	if issues := s.FilteredIssues(); len(issues) > 0 {
		more = append(more, fmt.Sprintf("Issue %d of %d", min(s.ClampedIdx(), len(issues)-1)+1, len(issues)))
	}
	if cur.Rule != "" {
		more = append(more, "rule "+cur.Rule)
	}
	if meta := s.resolveIssueFileMeta(cur); meta != "" {
		more = append(more, meta)
	}
	if len(more) > 0 {
		body = append(body, th.muted+strings.Join(more, "  ·  ")+th.reset)
	}
	if cur.Status == "done" && (cur.CommitCode != "" || cur.CommitOID != "") {
		body = append(body, th.green+"✔ fixed in "+formatIssueCommitLink(cur.CommitCode, cur.CommitOID)+th.reset)
	}
	return body
}

// issueSourceLines excerpts the loaded source around the finding, with its
// lines highlighted.
func (s *State) issueSourceLines(th ansiTheme, cur IssueRow, width int) []string {
	if s.IssueFile == nil || s.IssueFile.Path != cur.Path || cur.Line <= 0 {
		return nil
	}
	body := []string{"", th.sectionHeader("Source", -1)}
	raw := strings.Split(strings.ReplaceAll(s.IssueFile.Content, "\r\n", "\n"), "\n")
	from, to := max(0, cur.Line-3), min(len(raw), max(cur.Line+3, min(cur.EndLine, cur.Line+10)))
	last := max(cur.Line, cur.EndLine)
	for i := from; i < to; i++ {
		code := theme.TruncatePlain(theme.SanitizeCodeText(raw[i]), max(8, width-8))
		if i+1 >= cur.Line && i+1 <= last {
			body = append(body, theme.PaintLine(th.selection, th.fg, th.amber+fmt.Sprintf("%5d", i+1)+th.reset+th.selection+"  "+th.fg+code, width))
			continue
		}
		body = append(body, th.muted+fmt.Sprintf("%5d", i+1)+th.reset+"  "+th.fg+code+th.reset)
	}
	return body
}

// issueFixLines is the proposed fix, the files it would touch, when it is done
// and what is out of scope.
func (s *State) issueFixLines(th ansiTheme, cur IssueRow, width int) []string {
	body := []string{"", th.sectionHeader("Proposed fix", -1)}
	if strings.TrimSpace(cur.Solution) != "" {
		body = append(body, wrapText(th.fg+theme.SanitizeSingleLine(cur.Solution)+th.reset, width)...)
	} else {
		body = append(body, th.muted+"No approach recorded; the task plan proposes one."+th.reset)
	}
	if scope := domain.EnsureChangeTestPaths([]string{cur.Path}, s.Index); len(scope) > 0 && cur.Path != "" {
		body = append(body, th.field("Scope", th.cyan+strings.Join(scope, ", ")+th.reset, 6))
	}
	criteria := cur.AcceptanceCriteria
	if len(criteria) == 0 {
		criteria = []string{domain.SynthesizeIssueVerification(cur, s.Index)}
	}
	body = append(body, "", th.sectionHeader("Done when", len(criteria)))
	for i, c := range criteria {
		body = append(body, wrapText(th.muted+fmt.Sprintf("%d.", i+1)+th.reset+" "+th.fg+theme.SanitizeSingleLine(c)+th.reset, width)...)
	}
	if len(cur.NonGoals) > 0 {
		body = append(body, "", th.sectionHeader("Out of scope", len(cur.NonGoals)))
		for _, g := range cur.NonGoals {
			body = append(body, wrapText(th.muted+"· "+theme.SanitizeSingleLine(g)+th.reset, width)...)
		}
	}
	return body
}
