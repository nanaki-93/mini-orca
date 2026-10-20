package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) renderBugsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	batchBadge := ""
	if len(s.SelectedIssueIDs) > 0 {
		batchBadge = fmt.Sprintf("   %s BATCH SELECTED: %d %s", th.warnBg, len(s.SelectedIssueIDs), th.reset)
	}
	openCount, inProgCount, doneCount := s.IssueStatusCounts()
	statusStrip := s.renderIssueStatusFilterStrip(th, openCount, inProgCount, doneCount)
	lines := []string{
		fmt.Sprintf(
			"  %s[2] AGENT STUDIO · ISSUES & BUGS%s   %sCategory (f/t):%s %s[%s]%s   %sStatus (s/v):%s %s%s",
			th.bold+th.accent, th.reset,
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(s.IssueCategory), th.reset,
			th.muted, th.reset, statusStrip, batchBadge,
		),
		"",
	}
	lines = append(lines, th.renderTableHeader("   SEL  SEVERITY    CATEGORY   FINDING                        LOCATION            STATUS", width)...)
	filtered := s.FilteredIssues()
	if len(filtered) == 0 {
		return append(lines, fmt.Sprintf(
			"  %sNo findings in [%s] status (open:%d · in_progress:%d · done:%d) — press [s/v] for status or [f/t] for category.%s",
			th.muted, strings.ToUpper(s.IssueView), openCount, inProgCount, doneCount, th.reset,
		))
	}
	nameCol := max(18, width-62)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(filtered), selected, s.adaptiveWindowRows(15, 9))
	for idx := start; idx < end; idx++ {
		row := filtered[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		mark := th.muted + "[ ]" + th.reset
		if domain.ContainsString(s.SelectedIssueIDs, row.ID) {
			mark = th.amber + "[x]" + th.reset
		}
		loc := theme.TruncatePlain(fmt.Sprintf("%s:%d", row.Path, row.Line), 18)
		cat := theme.TruncatePlain(row.Category, 9)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s %s  %s%-9s%s  %s%-*s%s  %s%-18s%s  %s",
			cursor, mark, th.severityBadge(row.Severity),
			th.purple, cat, th.reset,
			tStyle, nameCol, theme.TruncatePlain(row.Name, nameCol), th.reset,
			th.muted, loc, th.reset,
			th.statusBadge(row.Status),
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(filtered))
	if cur := s.SelectedIssue(); cur != nil {
		footer := []string{
			fmt.Sprintf("%sFinding:%s   %s  %s%s%s",
				th.muted, th.reset, th.severityBadge(cur.Severity), th.bold+th.fg, cur.Title, th.reset),
			fmt.Sprintf("%sLocation:%s  %s%s:%d%s   %sSummary:%s %s",
				th.muted, th.reset, th.cyan, cur.Path, cur.Line, th.reset,
				th.muted, th.reset, cur.Cause),
		}
		lines = append(lines, "")
		lines = append(lines, th.renderPanelBox("SELECTED FINDING FOCUS", strings.ToUpper(cur.Category), panelW, footer)...)
	}
	return lines
}

func (s *State) renderIssueStatusFilterStrip(th ansiTheme, openCount, inProgCount, doneCount int) string {
	tabs := []struct {
		id    string
		label string
		count int
	}{
		{"open", "OPEN", openCount},
		{"in_progress", "IN_PROGRESS", inProgCount},
		{"done", "DONE", doneCount},
	}
	var parts []string
	for _, t := range tabs {
		if s.IssueView == t.id {
			parts = append(parts, fmt.Sprintf("%s[%s:%d]%s", th.bold+th.green, t.label, t.count, th.reset))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s:%d%s", th.muted, t.label, t.count, th.reset))
		}
	}
	if s.IssueView == "all" {
		parts = append(parts, fmt.Sprintf("%s[ALL:%d]%s", th.bold+th.green, openCount+inProgCount+doneCount, th.reset))
	}
	return strings.Join(parts, th.border+" · "+th.reset)
}

func (s *State) renderIssueDetailPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · ISSUE DETAIL & ROOT CAUSE", "Deep diagnostic & remediation plan")
	cur := s.SelectedIssue()
	if cur == nil {
		return append(lines, fmt.Sprintf("  %sNo finding selected.%s", th.muted, th.reset))
	}
	meta := []string{
		fmt.Sprintf("%sFinding:%s  %s%s%s", th.muted, th.reset, th.bold+th.fg, cur.Title, th.reset),
		fmt.Sprintf("%sSeverity:%s %s   %sCategory:%s %s%s%s   %sStatus:%s %s",
			th.muted, th.reset, th.severityBadge(cur.Severity),
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(cur.Category), th.reset,
			th.muted, th.reset, th.statusBadge(cur.Status)),
		fmt.Sprintf("%sLocation:%s %s%s:%d%s", th.muted, th.reset, th.cyan, cur.Path, cur.Line, th.reset),
	}
	lines = append(lines, th.renderPanelBox("ISSUE METADATA & LOCATION", cur.ID, panelW, meta)...)
	diag := []string{
		fmt.Sprintf("%s▲ Root Cause:%s      %s%s%s", th.bold+th.red, th.reset, th.amber, cur.Cause, th.reset),
		fmt.Sprintf("%s✔ Recommended Fix:%s %s%s%s", th.bold+th.green, th.reset, th.bold+th.fg, cur.Solution, th.reset),
		s.renderIssueDetailWorkflowLine(th, *cur),
	}
	lines = append(lines, "")
	return append(lines, th.renderPanelBox("ROOT CAUSE DIAGNOSTIC & REMEDIATION", strings.ToUpper(cur.Category), panelW, diag)...)
}

func (s *State) renderIssueDetailWorkflowLine(th ansiTheme, cur IssueRow) string {
	switch cur.Status {
	case "in_progress":
		return fmt.Sprintf("%s⟳ Agent Status:%s    In Progress (%s) · %s[Enter/w]%s Watch Agent Pipeline · %s[Esc]%s Back",
			th.bold+th.amber, th.reset, client.FirstNonEmpty(cur.AgentPath, "active"), th.cyan, th.reset, th.cyan, th.reset)
	case "done":
		oid := ""
		if cur.CommitOID != "" {
			oid = " (" + theme.TruncatePlain(cur.CommitOID, 8) + ")"
		}
		return fmt.Sprintf("%s✔ Resolution:%s      Verified & Fixed%s · %s[Enter/o]%s Inspect File · %s[Esc]%s Back",
			th.bold+th.green, th.reset, oid, th.cyan, th.reset, th.cyan, th.reset)
	default:
		return fmt.Sprintf("%s▸ Next Action:%s     %s[Enter/a]%s Fix Bug with Agent · %s[d]%s Dismiss · %s[o]%s Open File · %s[Esc]%s Back",
			th.bold+th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	}
}

func (s *State) renderFeaturesPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · FEATURES & ENHANCEMENTS", "Architecture-aligned capability proposals")
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, fmt.Sprintf("  %sNo active feature proposals queued.%s", th.muted, th.reset))
	}
	lines = append(lines, th.renderTableHeader("   EFFORT      FEATURE PROPOSAL                    STRATEGIC BENEFIT", width)...)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(features), selected, s.adaptiveWindowRows(15, 9))
	for idx := start; idx < end; idx++ {
		f := features[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		effort := strings.ToUpper(client.FirstNonEmpty(f.Effort, "med"))
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %s%-32s%s  %s%s%s",
			cursor, th.severityBadge(effort), tStyle, f.Title, th.reset,
			th.muted, f.Benefit, th.reset,
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(features))
	cur := features[selected]
	featBox := []string{
		fmt.Sprintf("%sProposal:%s %s%s%s   %sEffort:%s %s",
			th.muted, th.reset, th.bold+th.fg, cur.Title, th.reset,
			th.muted, th.reset, th.severityBadge(client.FirstNonEmpty(cur.Effort, "med"))),
		fmt.Sprintf("%sBenefit:%s  %s%s%s", th.muted, th.reset, th.green, cur.Benefit, th.reset),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("FEATURE IMPLEMENTATION WORKBENCH", "CAPABILITY SPEC", panelW, featBox)...)
	return lines
}
