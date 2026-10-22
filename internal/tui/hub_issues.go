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
		footer = append(footer, fmt.Sprintf("%sActions:%s   %s[Space]%s Select Batch · %s[Enter]%s Run Batch · %s[f]%s Fix Row · %s[i]%s Inspect · %s[s]%s Status · %s[d]%s Delete",
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
	lines := renderPageHeader(th, "ISSUE", fmt.Sprintf("Issue %d of %d · [f] Plan fix · [j/k] Next/prev · [Esc] Back", s.ClampedIdx()+1, len(s.FilteredIssues())))
	cur := s.SelectedIssue()
	if cur == nil {
		return append(lines, "  No finding selected.")
	}
	lines = append(lines, s.renderDetailSections(th)...)
	panelW := adaptivePanelWidth(width)
	contentW := issuePanelContentWidth(panelW)
	var rows []string
	switch s.DetailSection {
	case 1:
		rows = s.buildIssueDetailMetadataLines(th, *cur)
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Evidence:", th.fg, cur.Evidence, contentW)...)
		rows = append(rows, s.issueSourceExcerpt(*cur, th, contentW)...)
	case 2:
		rows = renderDiagnosticRow(th, th.green, "Proposed approach:", th.fg, cur.Solution, contentW)
		rows = append(rows, s.formatIssueVerificationLines(th, *cur, contentW)...)
		rows = append(rows, formatIssueGuardrailLines(th, cur.NonGoals, contentW)...)
		rows = append(rows, s.renderIssueDetailWorkflowLines(th, *cur, contentW)...)
	default:
		rows = renderDiagnosticRow(th, th.cyan, "Issue:", th.fg, cur.Name, contentW)
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Location:", th.fg, formatIssueLocationSpan(*cur), contentW)...)
		rows = append(rows, fmt.Sprintf("%s · %s", th.severityBadge(cur.Severity), th.statusBadge(cur.Status)))
		rows = append(rows, renderDiagnosticRow(th, th.amber, "Cause hypothesis:", th.fg, formatIssueRootCauseText(*cur), contentW)...)
	}
	return append(lines, th.renderPanelBox(s.detailSections()[s.DetailSection], "", panelW, rows)...)
}

func (s *State) buildIssueDetailMetadataLines(th ansiTheme, cur IssueRow) []string {
	conf := strings.ToUpper(client.FirstNonEmpty(cur.Confidence, "medium"))
	origin := formatIssueOrigin(cur)
	rule := client.FirstNonEmpty(cur.Rule, cur.Category+"-check")
	fresh := client.FirstNonEmpty(cur.Freshness, "current")
	symbol, signature, fileMeta := s.resolveIssueSymbolAndFileMeta(cur)
	locLine := fmt.Sprintf("%sLocation:%s   %s%s%s",
		th.muted, th.reset, th.cyan, formatIssueLocationSpan(cur), th.reset)
	if fileMeta != "" {
		locLine += fmt.Sprintf("   %sFile:%s %s%s%s", th.muted, th.reset, th.muted, fileMeta, th.reset)
	}
	symLine := fmt.Sprintf("%sSymbol:%s     %s%s%s", th.muted, th.reset, th.bold+th.fg, symbol, th.reset)
	if signature != "" {
		symLine += fmt.Sprintf("   %sSignature:%s %s%s%s", th.muted, th.reset, th.purple, signature, th.reset)
	}
	scopePaths := domain.EnsureChangeTestPaths([]string{cur.Path}, s.Index)
	scopeStr := client.FirstNonEmpty(strings.Join(scopePaths, ", "), cur.Path, "workspace")
	return []string{
		fmt.Sprintf("%sSeverity:%s   %s  %sCategory:%s %s%s%s  %sStatus:%s %s  %sConfidence:%s %s%s%s",
			th.muted, th.reset, th.severityBadge(cur.Severity),
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(cur.Category), th.reset,
			th.muted, th.reset, th.statusBadge(cur.Status),
			th.muted, th.reset, th.bold+th.cyan, conf, th.reset),
		fmt.Sprintf("%sOrigin:%s     %s%s%s   %sRule:%s %s%s%s   %sFreshness:%s %s%s%s",
			th.muted, th.reset, th.cyan, origin, th.reset,
			th.muted, th.reset, th.purple, rule, th.reset,
			th.muted, th.reset, th.green, fresh, th.reset),
		locLine,
		symLine,
		fmt.Sprintf("%sScope:%s      %s%s%s", th.muted, th.reset, th.cyan, scopeStr, th.reset),
	}
}

func formatIssueRootCauseText(cur IssueRow) string {
	title := strings.TrimSpace(cur.Title)
	cause := strings.TrimSpace(cur.Cause)
	if title == "" {
		return cause
	}
	if cause == "" || strings.EqualFold(strings.TrimRight(title, ". "), strings.TrimRight(cause, ". ")) ||
		strings.Contains(strings.ToLower(cause), strings.ToLower(title)) {
		return client.FirstNonEmpty(cause, title)
	}
	return fmt.Sprintf("%s — %s", strings.TrimRight(title, ". "), cause)
}

const diagnosticLabelCols = 19

func issuePanelContentWidth(panelW int) int {
	if panelW < 40 {
		panelW = 40
	}
	return max(24, panelW-4)
}

func renderDiagnosticRow(th ansiTheme, labelStyle, label, valueStyle, value string, contentW int) []string {
	prefix := theme.PadVisibleRight(labelStyle+label+th.reset, diagnosticLabelCols)
	rightW := max(16, contentW-diagnosticLabelCols)
	wrapped := theme.WrapVisibleLine(valueStyle+strings.TrimSpace(value)+th.reset, rightW)
	if len(wrapped) == 0 {
		return []string{prefix}
	}
	indent := strings.Repeat(" ", diagnosticLabelCols)
	out := make([]string, 0, len(wrapped))
	for i, w := range wrapped {
		if i == 0 {
			out = append(out, prefix+w)
		} else {
			out = append(out, indent+w)
		}
	}
	return out
}

func renderDiagnosticBulletRows(th ansiTheme, labelStyle, label, bulletStyle, textStyle string, items []string, contentW int) []string {
	prefix := theme.PadVisibleRight(labelStyle+label+th.reset, diagnosticLabelCols)
	indent := strings.Repeat(" ", diagnosticLabelCols)
	bulletPrefix := bulletStyle + "• " + th.reset
	bulletIndent := "  "
	itemW := max(14, contentW-diagnosticLabelCols-2)
	var out []string
	for idx, item := range items {
		wrapped := theme.WrapVisibleLine(textStyle+strings.TrimSpace(item)+th.reset, itemW)
		for lineIdx, w := range wrapped {
			left := indent
			if idx == 0 && lineIdx == 0 {
				left = prefix
			}
			lead := bulletIndent
			if lineIdx == 0 {
				lead = bulletPrefix
			}
			out = append(out, left+lead+w)
		}
	}
	return out
}

func (s *State) formatIssueVerificationLines(th ansiTheme, cur IssueRow, contentW int) []string {
	if len(cur.AcceptanceCriteria) == 0 {
		plan := domain.SynthesizeIssueVerification(cur, s.Index)
		return renderDiagnosticRow(th, th.bold+th.purple, "▸ Verification:", th.cyan, plan, contentW)
	}
	if len(cur.AcceptanceCriteria) == 1 {
		return renderDiagnosticRow(th, th.bold+th.purple, "▸ Verification:", th.cyan, cur.AcceptanceCriteria[0], contentW)
	}
	return renderDiagnosticBulletRows(th, th.bold+th.purple, "▸ Verification:", th.cyan, th.cyan, cur.AcceptanceCriteria, contentW)
}

func formatIssueGuardrailLines(th ansiTheme, nonGoals []string, contentW int) []string {
	if len(nonGoals) == 1 {
		return renderDiagnosticRow(th, th.bold+th.purple, "▸ Guardrails:", th.muted, nonGoals[0], contentW)
	}
	return renderDiagnosticBulletRows(th, th.bold+th.purple, "▸ Guardrails:", th.purple, th.muted, nonGoals, contentW)
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

func (s *State) renderIssueDetailWorkflowLines(th ansiTheme, cur IssueRow, contentW int) []string {
	switch cur.Status {
	case "in_progress":
		val := fmt.Sprintf("In Progress (%s) · %s[Enter/w]%s Watch Pipeline · %s[d]%s Delete · %s[i/Esc]%s Back",
			client.FirstNonEmpty(cur.AgentPath, "active"), th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
		return renderDiagnosticRow(th, th.bold+th.amber, "⟳ Agent Status:", th.fg, val, contentW)
	case "done":
		commitRef := ""
		if cur.CommitCode != "" || cur.CommitOID != "" {
			commitRef = " [" + formatIssueCommitLink(cur.CommitCode, cur.CommitOID) + "]"
		}
		val := fmt.Sprintf("Verified & Fixed%s · %s[Enter/o]%s Inspect File · %s[d]%s Delete · %s[i/Esc]%s Back",
			commitRef, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
		return renderDiagnosticRow(th, th.bold+th.green, "✔ Resolution:", th.fg, val, contentW)
	default:
		val := fmt.Sprintf("%s[Enter/f]%s Fix · %s[d]%s Delete · %s[o]%s Open File · %s[i/Esc]%s Back",
			th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
		return renderDiagnosticRow(th, th.bold+th.cyan, "▸ Next Action:", th.fg, val, contentW)
	}
}

func (s *State) renderFeaturesPage(th ansiTheme, width int) []string {
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
	return append(lines, "", "  [Enter] Inspect feature · [f] Plan implementation")
}
