package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

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

func (s *State) resolveIssueFileMeta(cur IssueRow) string {
	if s.Index == nil || cur.Path == "" {
		return ""
	}
	for _, f := range s.Index.Files {
		if f.Path == cur.Path {
			return formatIndexedFileSummary(f)
		}
	}
	return ""
}

func formatIndexedFileSummary(f IndexedFile) string {
	lang := client.FirstNonEmpty(f.Language, "text")
	if f.LineCount > 0 {
		return fmt.Sprintf("%s · %d lines", lang, f.LineCount)
	}
	return lang
}

func (s *State) renderFeaturesPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "Feature ideas", "Model suggestions · inspect the evidence before planning")
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, "", "  No feature ideas saved yet.", "  [r] Analyze for ideas · :agent <request> to plan your own")
	}
	selected := s.ClampedIdx()
	lines = append(lines, "")
	start, end := windowBounds(len(features), selected, s.adaptiveWindowRows(11, 8))
	for idx := start; idx < end; idx++ {
		f := features[idx]
		effortBadge := th.padBadge(client.FirstNonEmpty(f.Effort, "IDEA"), 10)
		lines = append(lines, "  "+th.renderCursorPrefix(idx == selected)+effortBadge+" "+rowTitleStyle(th, idx == selected)+theme.TruncatePlain(f.Title, max(12, width-18))+th.reset)
	}
	lines = appendWindowFooter(th, lines, start, end, len(features))
	feature := features[selected]
	effort := client.FirstNonEmpty(feature.Effort, "unspecified")
	if s.viewportBodyRows <= 0 || s.viewportBodyRows >= 24 {
		body := []string{
			fmt.Sprintf("%sGoal:%s    %s%s%s", th.cyan, th.reset, th.bold+th.fg, feature.Title, th.reset),
			fmt.Sprintf("%sBenefit:%s %s%s%s", th.green, th.reset, th.fg, client.FirstNonEmpty(feature.Benefit, "Inspect details for full scope and evidence."), th.reset),
		}
		return append(append(lines, ""), th.renderPanelBox("SELECTED IDEA · EFFORT: "+strings.ToUpper(effort), "[Enter] Inspect · [f] Plan", adaptivePanelWidth(width), body)...)
	}
	lines = append(lines, "", "  "+th.muted+"SELECTED IDEA · Effort: "+effort+th.reset)
	lines = appendWrappedText(lines, th.bold+th.fg+feature.Title+th.reset, width)
	return appendWrappedText(lines, feature.Benefit, width)
}
