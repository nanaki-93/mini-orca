package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type ansiTheme struct {
	name    string
	accent  string
	fg      string
	muted   string
	border  string
	surface string
	selBg   string
	critBg  string
	warnBg  string
	okBg    string
	infoBg  string
	green   string
	amber   string
	red     string
	purple  string
	cyan    string
	bold    string
	dim     string
	reset   string
}

func themeForPalette(p Palette) ansiTheme {
	switch p {
	case PaletteCRT:
		return ansiTheme{
			name:    "CRT PHOSPHOR",
			accent:  "\033[1;38;5;46m",
			fg:      "\033[38;5;120m",
			muted:   "\033[38;5;28m",
			border:  "\033[38;5;28m",
			surface: "\033[48;5;22m",
			selBg:   "\033[48;5;22m",
			critBg:  "\033[1;38;5;255;48;5;160m",
			warnBg:  "\033[1;38;5;232;48;5;226m",
			okBg:    "\033[1;38;5;232;48;5;46m",
			infoBg:  "\033[1;38;5;46;48;5;22m",
			green:   "\033[1;38;5;46m",
			amber:   "\033[1;38;5;226m",
			red:     "\033[1;38;5;196m",
			purple:  "\033[38;5;83m",
			cyan:    "\033[1;38;5;50m",
			bold:    "\033[1m",
			dim:     "\033[2m",
			reset:   "\033[0m",
		}
	case PaletteAmber:
		return ansiTheme{
			name:    "AMBER TERMINAL",
			accent:  "\033[1;38;5;214m",
			fg:      "\033[38;5;222m",
			muted:   "\033[38;5;136m",
			border:  "\033[38;5;136m",
			surface: "\033[48;5;52m",
			selBg:   "\033[48;5;236m",
			critBg:  "\033[1;38;5;255;48;5;160m",
			warnBg:  "\033[1;38;5;232;48;5;214m",
			okBg:    "\033[1;38;5;232;48;5;142m",
			infoBg:  "\033[1;38;5;214;48;5;236m",
			green:   "\033[1;38;5;142m",
			amber:   "\033[1;38;5;214m",
			red:     "\033[1;38;5;203m",
			purple:  "\033[38;5;179m",
			cyan:    "\033[1;38;5;215m",
			bold:    "\033[1m",
			dim:     "\033[2m",
			reset:   "\033[0m",
		}
	case PalettePaper:
		return ansiTheme{
			name:    "PAPER LIGHT",
			accent:  "\033[1;38;5;25m",
			fg:      "\033[38;5;235m",
			muted:   "\033[38;5;242m",
			border:  "\033[38;5;247m",
			surface: "\033[48;5;254m",
			selBg:   "\033[48;5;252m",
			critBg:  "\033[1;38;5;255;48;5;160m",
			warnBg:  "\033[1;38;5;255;48;5;130m",
			okBg:    "\033[1;38;5;255;48;5;28m",
			infoBg:  "\033[1;38;5;255;48;5;25m",
			green:   "\033[1;38;5;28m",
			amber:   "\033[1;38;5;130m",
			red:     "\033[1;38;5;160m",
			purple:  "\033[1;38;5;91m",
			cyan:    "\033[1;38;5;31m",
			bold:    "\033[1m",
			dim:     "\033[2m",
			reset:   "\033[0m",
		}
	default:
		return ansiTheme{
			name:    "TOKYO ENTERPRISE",
			accent:  "\033[1;38;5;111m",
			fg:      "\033[38;5;253m",
			muted:   "\033[38;5;245m",
			border:  "\033[38;5;240m",
			surface: "\033[48;5;236m",
			selBg:   "\033[48;5;236m",
			critBg:  "\033[1;38;5;255;48;5;160m",
			warnBg:  "\033[1;38;5;232;48;5;215m",
			okBg:    "\033[1;38;5;232;48;5;114m",
			infoBg:  "\033[1;38;5;255;48;5;62m",
			green:   "\033[1;38;5;114m",
			amber:   "\033[1;38;5;215m",
			red:     "\033[1;38;5;203m",
			purple:  "\033[1;38;5;176m",
			cyan:    "\033[1;38;5;117m",
			bold:    "\033[1m",
			dim:     "\033[2m",
			reset:   "\033[0m",
		}
	}
}

// clampWidth normalizes viewport width across compact and full-screen ultrawide terminals.
func clampWidth(width int) int {
	if width <= 0 {
		return 120
	}
	if width < 68 {
		return 68
	}
	if width > 300 {
		return 300
	}
	return width
}

// clampHeight normalizes viewport height so full-screen terminals are filled cleanly.
func clampHeight(height int) int {
	if height <= 0 {
		return 34
	}
	if height < 20 {
		return 20
	}
	if height > 120 {
		return 120
	}
	return height
}

// visibleRuneCount measures the printable column width of an ANSI-styled string.
func visibleRuneCount(s string) int {
	count := 0
	inEsc := false
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			inEsc = true
			i++
			continue
		}
		if inEsc {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEsc = false
			}
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		count++
		i += size
	}
	return count
}

// padVisibleRight pads an ANSI-styled string with trailing spaces to target visible columns.
func padVisibleRight(s string, target int) string {
	vis := visibleRuneCount(s)
	if vis >= target {
		return s
	}
	return s + strings.Repeat(" ", target-vis)
}

// truncatePlain shortens a plain string to maxCols runes using an ellipsis.
func truncatePlain(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxCols {
		return s
	}
	if maxCols <= 1 {
		return "…"
	}
	runes := []rune(s)
	return string(runes[:maxCols-1]) + "…"
}

// wrapFrameLine encloses a line in left and right outer vertical frame borders sized to width.
func (th ansiTheme) wrapFrameLine(line string, width int) string {
	inner := max(10, width-2)
	padded := padVisibleRight(line, inner)
	return fmt.Sprintf("%s┃%s%s%s┃%s", th.border, th.reset, padded, th.border, th.reset)
}

// keyPill formats a high-contrast keyboard shortcut pill with action label.
func (th ansiTheme) keyPill(key, label string) string {
	return fmt.Sprintf("%s[%s]%s %s%s%s", th.cyan, key, th.reset, th.fg, label, th.reset)
}

// renderProgressBar draws a deterministic block progress bar with percentage.
func (th ansiTheme) renderProgressBar(done, total, barWidth int) string {
	if barWidth < 6 {
		barWidth = 6
	}
	if total <= 0 {
		return th.muted + strings.Repeat("░", barWidth) + "   0%" + th.reset
	}
	clamped := min(max(0, done), total)
	filled := (clamped * barWidth) / total
	pct := (clamped * 100) / total
	color := th.accent
	if clamped == total {
		color = th.green
	}
	bar := color + strings.Repeat("█", filled) + th.muted + strings.Repeat("░", barWidth-filled) + th.reset
	return fmt.Sprintf("%s %3d%%", bar, pct)
}

// severityBadge formats a color-coded enterprise severity badge with high-contrast critical/high emphasis.
func (th ansiTheme) severityBadge(sev string) string {
	upper := strings.ToUpper(strings.TrimSpace(sev))
	if upper == "" {
		upper = "INFO"
	}
	label := fmt.Sprintf("[%-8s]", truncatePlain(upper, 8))
	switch upper {
	case "CRITICAL":
		return th.critBg + label + th.reset
	case "HIGH":
		return th.warnBg + label + th.reset
	case "MEDIUM", "MED":
		return th.purple + label + th.reset
	case "LOW":
		return th.cyan + label + th.reset
	default:
		return th.muted + label + th.reset
	}
}

// statusBadge formats a lifecycle or check state badge with semantic coloring.
func (th ansiTheme) statusBadge(state string) string {
	upper := strings.ToUpper(strings.TrimSpace(state))
	if upper == "" {
		upper = "IDLE"
	}
	switch upper {
	case "PASSED", "READY", "DONE", "FIXED", "APPLIED", "CURRENT", "ONLINE", "TRUSTED", "INCL":
		return th.green + "[" + upper + "]" + th.reset
	case "FAILED", "ERROR", "CRITICAL", "EXCL", "MISSING", "DIRTY":
		return th.critBg + "[" + upper + "]" + th.reset
	case "RUNNING", "WORKING", "IN_PROGRESS", "PAUSED", "MERGEABLE", "UNTRUSTED":
		return th.amber + "[" + upper + "]" + th.reset
	default:
		return th.cyan + "[" + upper + "]" + th.reset
	}
}

// renderPanelBox wraps content lines in a rounded enterprise panel frame.
func (th ansiTheme) renderPanelBox(title, rightMeta string, innerWidth int, body []string) []string {
	if innerWidth < 40 {
		innerWidth = 40
	}
	titleSeg := " " + truncatePlain(strings.TrimSpace(title), innerWidth-12) + " "
	metaSeg := ""
	if strings.TrimSpace(rightMeta) != "" {
		metaSeg = " " + strings.TrimSpace(rightMeta) + " "
	}
	dashCount := max(2, innerWidth-visibleRuneCount(titleSeg)-visibleRuneCount(metaSeg)-2)
	top := fmt.Sprintf(
		"  %s╭─%s%s%s%s%s%s─╮%s",
		th.border, th.accent, titleSeg, th.border, strings.Repeat("─", dashCount), th.cyan, metaSeg, th.reset,
	)
	out := make([]string, 0, len(body)+2)
	out = append(out, top)
	for _, line := range body {
		padded := padVisibleRight(line, innerWidth-2)
		out = append(out, fmt.Sprintf("  %s│%s %s %s│%s", th.border, th.reset, padded, th.border, th.reset))
	}
	bottom := fmt.Sprintf("  %s╰%s╯%s", th.border, strings.Repeat("─", innerWidth), th.reset)
	out = append(out, bottom)
	return out
}

// renderErrorBanner draws a high-contrast error callout box when s.Error is active.
func (th ansiTheme) renderErrorBanner(errText string, innerWidth int) []string {
	if innerWidth < 40 {
		innerWidth = 40
	}
	titleSeg := " ▲ ERROR / DIAGNOSTIC ALERT "
	hintSeg := " [Esc] Dismiss · [:reconnect] Retry "
	dashCount := max(2, innerWidth-visibleRuneCount(titleSeg)-visibleRuneCount(hintSeg)-2)
	top := fmt.Sprintf(
		"  %s┏━%s%s%s%s%s%s━┓%s",
		th.red, th.critBg, titleSeg, th.reset+th.red, strings.Repeat("━", dashCount), th.amber, hintSeg, th.reset,
	)
	msg := fmt.Sprintf("%s%s%s", th.red, truncatePlain(strings.TrimSpace(errText), innerWidth-4), th.reset)
	mid := fmt.Sprintf("  %s┃%s %s %s┃%s", th.red, th.reset, padVisibleRight(msg, innerWidth-2), th.red, th.reset)
	bot := fmt.Sprintf("  %s┗%s┛%s", th.red, strings.Repeat("━", innerWidth), th.reset)
	return []string{top, mid, bot}
}

// renderKPICards renders a horizontal strip of compact enterprise KPI cards adapted to totalWidth.
func (th ansiTheme) renderKPICards(cards [][2]string, totalWidth int) []string {
	if len(cards) == 0 {
		return nil
	}
	cardWidth := max(18, (totalWidth-8)/len(cards)-2)
	var tops, mids, bots []string
	for _, c := range cards {
		label := truncatePlain(strings.ToUpper(c[0]), cardWidth-4)
		val := padVisibleRight(c[1], cardWidth-2)
		tops = append(tops, fmt.Sprintf("%s╭─ %s%-*s%s ─╮%s", th.border, th.cyan, cardWidth-4, label, th.border, th.reset))
		mids = append(mids, fmt.Sprintf("%s│%s %s %s│%s", th.border, th.reset, val, th.border, th.reset))
		bots = append(bots, fmt.Sprintf("%s╰%s╯%s", th.border, strings.Repeat("─", cardWidth), th.reset))
	}
	return []string{
		"  " + strings.Join(tops, " "),
		"  " + strings.Join(mids, " "),
		"  " + strings.Join(bots, " "),
	}
}

// renderTableHeader formats an aligned enterprise table header row with underline rule.
func (th ansiTheme) renderTableHeader(columns string, width int) []string {
	ruleWidth := max(30, width-6)
	return []string{
		fmt.Sprintf("  %s%s%s", th.bold+th.cyan, columns, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", ruleWidth), th.reset),
	}
}

// renderCursorPrefix returns the row selection cursor indicator.
func (th ansiTheme) renderCursorPrefix(selected bool) string {
	if selected {
		return th.accent + "▸ " + th.reset
	}
	return "  "
}
