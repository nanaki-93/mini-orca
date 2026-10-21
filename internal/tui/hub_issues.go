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
		batchBadge = fmt.Sprintf("   %s BATCH SELECTED: %d ([Enter] Run Batch) %s", th.warnBg, len(s.SelectedIssueIDs), th.reset)
	}
	openCount, inProgCount, doneCount := s.IssueStatusCounts()
	statusStrip := s.renderIssueStatusFilterStrip(th, openCount, inProgCount, doneCount)
	lines := []string{
		fmt.Sprintf(
			"  %s[2] AGENT STUDIO · ISSUES & BUGS%s   %sCategory (t):%s %s[%s]%s   %sStatus (s):%s %s%s",
			th.bold+th.accent, th.reset,
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(s.IssueCategory), th.reset,
			th.muted, th.reset, statusStrip, batchBadge,
		),
		"",
	}
	nameCol := max(18, width-62)
	headerCols := fmt.Sprintf("    %-3s %-10s  %-9s  %-*s  %-18s  %s", "SEL", "SEVERITY", "CATEGORY", nameCol, "FINDING", "LOCATION", "STATUS")
	lines = append(lines, th.renderTableHeader(headerCols, width)...)
	filtered := s.FilteredIssues()
	if len(filtered) == 0 {
		return append(lines, fmt.Sprintf(
			"  %sNo findings in [%s] status (open:%d · in_progress:%d · done:%d) — press [s] for status or [t] for category.%s",
			th.muted, strings.ToUpper(s.IssueView), openCount, inProgCount, doneCount, th.reset,
		))
	}
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
		if cur.Status == "done" && (cur.CommitCode != "" || cur.CommitOID != "") {
			footer = append(footer, fmt.Sprintf("%sCommit:%s    %s%s%s",
				th.muted, th.reset, th.green, formatIssueCommitLink(cur.CommitCode, cur.CommitOID), th.reset))
		}
		footer = append(footer, fmt.Sprintf("%sActions:%s   %s[Space]%s Select Batch · %s[Enter]%s Run Batch · %s[f]%s Fix Row · %s[i]%s Inspect · %s[s]%s Status · %s[d]%s Dismiss",
			th.muted, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset))
		lines = append(lines, "")
		lines = append(lines, th.renderPanelBox("SELECTED FINDING FOCUS", strings.ToUpper(cur.Category), panelW, footer)...)
	}
	return lines
}

func formatIssueCommitLink(code, oid string) string {
	shortOID := theme.TruncatePlain(strings.TrimSpace(oid), 8)
	switch {
	case code != "" && shortOID != "":
		return fmt.Sprintf("%s (%s)", code, shortOID)
	case code != "":
		return code
	case shortOID != "":
		return shortOID
	default:
		return "verified"
	}
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
	filtered := s.FilteredIssues()
	subTitle := "Deep diagnostic & remediation plan"
	if len(filtered) > 0 {
		subTitle = fmt.Sprintf("Issue %d of %d · [j/k / ←/→] Prev/Next · [i/Esc] Back", s.ClampedIdx()+1, len(filtered))
	}
	lines := renderPageHeader(th, "[2] AGENT STUDIO · ISSUE DETAIL & ROOT CAUSE", subTitle)
	cur := s.SelectedIssue()
	if cur == nil {
		return append(lines, fmt.Sprintf("  %sNo finding selected.%s", th.muted, th.reset))
	}
	rightMeta := cur.ID
	if len(filtered) > 0 {
		rightMeta = fmt.Sprintf("%d/%d · %s", s.ClampedIdx()+1, len(filtered), cur.ID)
	}
	meta := s.buildIssueDetailMetadataLines(th, *cur)
	lines = append(lines, th.renderPanelBox("ISSUE METADATA & LOCATION", rightMeta, panelW, meta)...)
	diag := s.buildIssueDetailDiagnosticLines(th, *cur)
	lines = append(lines, "")
	return append(lines, th.renderPanelBox("ROOT CAUSE DIAGNOSTIC & REMEDIATION", strings.ToUpper(cur.Category), panelW, diag)...)
}

func (s *State) buildIssueDetailMetadataLines(th ansiTheme, cur IssueRow) []string {
	conf := strings.ToUpper(client.FirstNonEmpty(cur.Confidence, "medium"))
	origin := formatIssueOrigin(cur)
	rule := client.FirstNonEmpty(cur.Rule, cur.Category+"-check")
	fresh := client.FirstNonEmpty(cur.Freshness, "current")
	symbol, signature, fileMeta := s.resolveIssueSymbolAndFileMeta(cur)
	locLine := fmt.Sprintf("%sLocation:%s   %s%s%s  %sSymbol:%s %s%s%s",
		th.muted, th.reset, th.cyan, formatIssueLocationSpan(cur), th.reset,
		th.muted, th.reset, th.bold+th.fg, symbol, th.reset)
	if fileMeta != "" {
		locLine += fmt.Sprintf("  %sFile:%s %s%s%s", th.muted, th.reset, th.muted, fileMeta, th.reset)
	}
	scopePaths := domain.EnsureChangeTestPaths([]string{cur.Path}, s.Index)
	scopeStr := client.FirstNonEmpty(strings.Join(scopePaths, ", "), cur.Path, "workspace")
	scopeLine := fmt.Sprintf("%sScope:%s      %s%s%s", th.muted, th.reset, th.cyan, scopeStr, th.reset)
	if signature != "" {
		scopeLine = fmt.Sprintf("%sSignature:%s  %s%s%s   %sScope:%s %s%s%s",
			th.muted, th.reset, th.purple, signature, th.reset,
			th.muted, th.reset, th.cyan, scopeStr, th.reset)
	}
	return []string{
		fmt.Sprintf("%sFinding:%s    %s%s%s", th.muted, th.reset, th.bold+th.fg, cur.Title, th.reset),
		fmt.Sprintf("%sSeverity:%s   %s   %sCategory:%s %s%s%s   %sStatus:%s %s   %sConfidence:%s %s%s%s",
			th.muted, th.reset, th.severityBadge(cur.Severity),
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(cur.Category), th.reset,
			th.muted, th.reset, th.statusBadge(cur.Status),
			th.muted, th.reset, th.bold+th.cyan, conf, th.reset),
		fmt.Sprintf("%sOrigin:%s     %s%s%s   %sRule:%s %s%s%s   %sFreshness:%s %s%s%s   %sTarget:%s %s%s%s",
			th.muted, th.reset, th.cyan, origin, th.reset,
			th.muted, th.reset, th.purple, rule, th.reset,
			th.muted, th.reset, th.green, fresh, th.reset,
			th.muted, th.reset, th.muted, client.FirstNonEmpty(cur.Target.ID, cur.ID), th.reset),
		locLine,
		scopeLine,
	}
}

func (s *State) buildIssueDetailDiagnosticLines(th ansiTheme, cur IssueRow) []string {
	diag := []string{
		fmt.Sprintf("%s▲ Root Cause:%s      %s%s%s", th.bold+th.red, th.reset, th.amber, cur.Cause, th.reset),
	}
	if cur.Evidence != "" {
		diag = append(diag, fmt.Sprintf("%s◆ Evidence:%s        %s%s%s", th.bold+th.cyan, th.reset, th.fg, cur.Evidence, th.reset))
	}
	diag = append(diag, fmt.Sprintf("%s✔ Recommended Fix:%s %s%s%s", th.bold+th.green, th.reset, th.bold+th.fg, cur.Solution, th.reset))
	diag = append(diag, s.formatIssueVerificationLines(th, cur)...)
	if len(cur.NonGoals) > 0 {
		diag = append(diag, fmt.Sprintf("%s▸ Guardrails:%s      %s%s%s", th.bold+th.purple, th.reset, th.muted, strings.Join(cur.NonGoals, " · "), th.reset))
	}
	if cur.Status == "done" && (cur.CommitCode != "" || cur.CommitOID != "") {
		diag = append(diag, fmt.Sprintf("%s▸ Commit Receipt:%s  %s%s%s", th.bold+th.green, th.reset, th.green, formatIssueCommitLink(cur.CommitCode, cur.CommitOID), th.reset))
	} else if cur.Status == "in_progress" && cur.AgentPath != "" {
		diag = append(diag, fmt.Sprintf("%s▸ Active Worktree:%s %s%s%s", th.bold+th.amber, th.reset, th.cyan, cur.AgentPath, th.reset))
	}
	diag = append(diag, s.renderIssueDetailWorkflowLine(th, cur))
	return diag
}

func (s *State) formatIssueVerificationLines(th ansiTheme, cur IssueRow) []string {
	if len(cur.AcceptanceCriteria) == 0 {
		plan := domain.SynthesizeIssueVerification(cur, s.Index)
		return []string{
			fmt.Sprintf("%s▸ Verification:%s    %s%s%s",
				th.bold+th.purple, th.reset, th.cyan, plan, th.reset),
		}
	}
	if len(cur.AcceptanceCriteria) == 1 {
		return []string{
			fmt.Sprintf("%s▸ Verification:%s    %s%s%s",
				th.bold+th.purple, th.reset, th.cyan, cur.AcceptanceCriteria[0], th.reset),
		}
	}
	limit := min(3, len(cur.AcceptanceCriteria))
	out := []string{
		fmt.Sprintf("%s▸ Acceptance Criteria (%d):%s", th.bold+th.purple, len(cur.AcceptanceCriteria), th.reset),
	}
	for i := 0; i < limit; i++ {
		out = append(out, fmt.Sprintf("  %s•%s %s", th.cyan, th.reset, cur.AcceptanceCriteria[i]))
	}
	return out
}

func formatIssueOrigin(cur IssueRow) string {
	kind := client.FirstNonEmpty(cur.Target.Kind, "semantic")
	src := client.FirstNonEmpty(cur.Source, cur.Target.Source)
	if src != "" && src != kind {
		return fmt.Sprintf("%s (%s)", kind, src)
	}
	return kind
}

func formatIssueLocationSpan(cur IssueRow) string {
	path := client.FirstNonEmpty(cur.Path, "workspace")
	if cur.Line <= 0 {
		return path
	}
	if cur.EndLine > cur.Line {
		return fmt.Sprintf("%s:%d-%d", path, cur.Line, cur.EndLine)
	}
	return fmt.Sprintf("%s:%d", path, cur.Line)
}

func (s *State) resolveIssueSymbolAndFileMeta(cur IssueRow) (symbol, signature, fileMeta string) {
	symbol = cur.Symbol
	signature = cur.Signature
	if s.Index == nil || cur.Path == "" {
		return client.FirstNonEmpty(symbol, "file-scope"), signature, ""
	}
	for _, f := range s.Index.Files {
		if f.Path != cur.Path {
			continue
		}
		fileMeta = formatIndexedFileSummary(f)
		sym, sig := domain.MatchIndexedSymbolForFinding(f.Symbols, symbol, cur.Line, cur.Title, cur.Cause)
		if symbol == "" {
			symbol = sym
		}
		if signature == "" {
			signature = sig
		}
		break
	}
	return client.FirstNonEmpty(symbol, "file-scope"), signature, fileMeta
}

func formatIndexedFileSummary(f IndexedFile) string {
	lang := client.FirstNonEmpty(f.Language, "text")
	if f.LineCount > 0 {
		return fmt.Sprintf("%s · %d lines", lang, f.LineCount)
	}
	return lang
}

func (s *State) renderIssueDetailWorkflowLine(th ansiTheme, cur IssueRow) string {
	switch cur.Status {
	case "in_progress":
		return fmt.Sprintf("%s⟳ Agent Status:%s    In Progress (%s) · %s[Enter/w]%s Watch Agent Pipeline · %s[i/Esc]%s Back",
			th.bold+th.amber, th.reset, client.FirstNonEmpty(cur.AgentPath, "active"), th.cyan, th.reset, th.cyan, th.reset)
	case "done":
		commitRef := ""
		if cur.CommitCode != "" || cur.CommitOID != "" {
			commitRef = " [" + formatIssueCommitLink(cur.CommitCode, cur.CommitOID) + "]"
		}
		return fmt.Sprintf("%s✔ Resolution:%s      Verified & Fixed%s · %s[Enter/o]%s Inspect File · %s[i/Esc]%s Back",
			th.bold+th.green, th.reset, commitRef, th.cyan, th.reset, th.cyan, th.reset)
	default:
		return fmt.Sprintf("%s▸ Next Action:%s     %s[f/Enter/a]%s Fix Bug with Agent · %s[d]%s Dismiss · %s[o]%s Open File · %s[i/Esc]%s Back",
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
