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
			border:  "\033[38;5;22m",
			surface: "\033[48;5;22m",
			selBg:   "\033[48;5;235m",
			green:   "\033[38;5;46m",
			amber:   "\033[38;5;226m",
			red:     "\033[38;5;196m",
			purple:  "\033[38;5;83m",
			cyan:    "\033[38;5;50m",
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
			border:  "\033[38;5;94m",
			surface: "\033[48;5;52m",
			selBg:   "\033[48;5;236m",
			green:   "\033[38;5;142m",
			amber:   "\033[38;5;214m",
			red:     "\033[38;5;203m",
			purple:  "\033[38;5;179m",
			cyan:    "\033[38;5;215m",
			bold:    "\033[1m",
			dim:     "\033[2m",
			reset:   "\033[0m",
		}
	case PalettePaper:
		return ansiTheme{
			name:    "PAPER LIGHT",
			accent:  "\033[1;38;5;25m",
			fg:      "\033[38;5;235m",
			muted:   "\033[38;5;244m",
			border:  "\033[38;5;250m",
			surface: "\033[48;5;254m",
			selBg:   "\033[48;5;253m",
			green:   "\033[38;5;28m",
			amber:   "\033[38;5;130m",
			red:     "\033[38;5;160m",
			purple:  "\033[38;5;91m",
			cyan:    "\033[38;5;31m",
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
			border:  "\033[38;5;239m",
			surface: "\033[48;5;236m",
			selBg:   "\033[48;5;237m",
			green:   "\033[38;5;114m",
			amber:   "\033[38;5;215m",
			red:     "\033[38;5;203m",
			purple:  "\033[38;5;176m",
			cyan:    "\033[38;5;117m",
			bold:    "\033[1m",
			dim:     "\033[2m",
			reset:   "\033[0m",
		}
	}
}

// clampWidth normalizes viewport width to a safe enterprise frame range.
func clampWidth(width int) int {
	if width <= 0 {
		return 108
	}
	if width < 68 {
		return 68
	}
	if width > 140 {
		return 140
	}
	return width
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
		return th.red + "[" + upper + "]" + th.reset
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
		th.border, th.accent, titleSeg, th.border, strings.Repeat("─", dashCount), th.muted, metaSeg, th.reset,
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
