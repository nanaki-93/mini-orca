package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type ansiTheme struct {
	name           string
	accent         string
	accentBg       string
	fg             string
	muted          string
	border         string
	critBg         string
	warnBg         string
	okBg           string
	infoBg         string
	green          string
	amber          string
	red            string
	purple         string
	cyan           string
	bold           string
	reset          string
	cardStyle      lipgloss.Style
	activeTabStyle lipgloss.Style
	idleTabStyle   lipgloss.Style
	subActiveStyle lipgloss.Style
	subIdleStyle   lipgloss.Style
	titleStyle     lipgloss.Style
	labelStyle     lipgloss.Style
	mutedStyle     lipgloss.Style
	errorBoxStyle  lipgloss.Style
	progStartHex   string
	progEndHex     string
}

func themeForPalette(p Palette) ansiTheme {
	c := theme.ColorsForPalette(p)
	accentCol := lipgloss.Color(c.AccentCode)
	fgCol := lipgloss.Color(c.FgCode)
	mutedCol := lipgloss.Color(c.MutedCode)
	borderCol := lipgloss.Color(c.BorderCode)
	selBgCol := lipgloss.Color(c.SelBgCode)
	redCol := lipgloss.Color(c.RedCode)

	return ansiTheme{
		name:     c.Name,
		accent:   "\033[1;38;5;" + c.AccentCode + "m",
		accentBg: "\033[1;38;5;255;48;5;" + c.AccentBgCode + "m",
		fg:       "\033[38;5;" + c.FgCode + "m",
		muted:    "\033[38;5;" + c.MutedCode + "m",
		border:   "\033[38;5;" + c.BorderCode + "m",
		critBg:   "\033[1;38;5;255;48;5;160m",
		warnBg:   "\033[1;38;5;232;48;5;" + c.AmberCode + "m",
		okBg:     "\033[1;38;5;232;48;5;" + c.GreenCode + "m",
		infoBg:   "\033[1;38;5;255;48;5;" + c.AccentBgCode + "m",
		green:    "\033[1;38;5;" + c.GreenCode + "m",
		amber:    "\033[1;38;5;" + c.AmberCode + "m",
		red:      "\033[1;38;5;" + c.RedCode + "m",
		purple:   "\033[1;38;5;" + c.PurpleCode + "m",
		cyan:     "\033[1;38;5;" + c.CyanCode + "m",
		bold:     "\033[1m",
		reset:    "\033[0m",

		cardStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderCol).
			Padding(0, 1),
		activeTabStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color(c.AccentBgCode)).
			Padding(0, 1),
		idleTabStyle: lipgloss.NewStyle().
			Foreground(mutedCol).
			Padding(0, 1),
		subActiveStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accentCol).
			Background(selBgCol).
			Padding(0, 1),
		subIdleStyle: lipgloss.NewStyle().
			Foreground(mutedCol).
			Padding(0, 1),
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accentCol),
		labelStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(fgCol),
		mutedStyle: lipgloss.NewStyle().
			Foreground(mutedCol),
		errorBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(redCol).
			Padding(0, 1),
		progStartHex: c.ProgStart,
		progEndHex:   c.ProgEnd,
	}
}

func (th ansiTheme) wrapFrameLine(line string, width int) string {
	inner := max(10, width-2)
	padded := theme.PadVisibleRight(line, inner)
	return fmt.Sprintf("%s┃%s%s%s┃%s", th.border, th.reset, padded, th.border, th.reset)
}

func (th ansiTheme) keyPill(k, desc string) string {
	return theme.KeyPill(th.cyan, th.fg, th.reset, k, desc)
}

func (th ansiTheme) renderCommandInput(value string) string {
	return theme.RenderCommandInput(th.accentBg, th.bold+th.fg, th.reset, value)
}

func (th ansiTheme) renderProgressBar(done, total, barWidth int) string {
	return theme.RenderProgressBar(th.progStartHex, th.progEndHex, done, total, barWidth)
}

func (th ansiTheme) severityBadge(sev string) string {
	upper := strings.ToUpper(strings.TrimSpace(sev))
	if upper == "" {
		upper = "INFO"
	}
	label := fmt.Sprintf("[%-8s]", theme.TruncatePlain(upper, 8))
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

func (th ansiTheme) statusBadge(state string) string {
	upper := strings.ToUpper(strings.TrimSpace(state))
	if upper == "" {
		upper = "IDLE"
	}
	switch upper {
	case "PASSED", "READY", "CHECKED", "DONE", "FIXED", "APPLIED", "MERGED", "CURRENT", "ONLINE", "TRUSTED", "INCL", "COMPLETED", "COMPLETED_EMPTY", "CLEAN", "APPROVE", "APPROVED", "GRADUATED", "CREATED":
		return th.green + "[" + upper + "]" + th.reset
	case "FAILED", "ERROR", "CRITICAL", "EXCL", "MISSING", "DIRTY", "CHANGES_REQUESTED", "REJECTED", "CONFLICTS":
		return th.critBg + "[" + upper + "]" + th.reset
	case "RUNNING", "WORKING", "IN_PROGRESS", "PAUSED", "MERGEABLE", "UNTRUSTED", "WAITING", "AWAITING_HUMAN_REVIEW", "REVIEW", "STALE", "INTERRUPTED", "CANDIDATE", "CANCELING", "PAUSING":
		return th.amber + "[" + upper + "]" + th.reset
	case "IDLE", "PENDING", "QUEUED", "BLOCKED", "SKIPPED", "CANCELED", "STOPPED", "REVERTED", "UNDONE", "DISMISSED", "DISCARDED", "UNAVAILABLE":
		return th.muted + "[" + upper + "]" + th.reset
	default:
		return th.cyan + "[" + upper + "]" + th.reset
	}
}

func (th ansiTheme) padBadge(state string, width int) string {
	b := th.statusBadge(state)
	vis := theme.VisibleRuneCount(b)
	if vis >= width {
		return b
	}
	return b + strings.Repeat(" ", width-vis)
}

func (th ansiTheme) renderPanelBox(title, rightMeta string, innerWidth int, body []string) []string {
	return theme.RenderPanelBox(th.titleStyle, th.mutedStyle, th.cardStyle, th.border, th.reset, title, rightMeta, innerWidth, body)
}

func (th ansiTheme) renderErrorBanner(errText string, innerWidth int) []string {
	return theme.RenderErrorBanner(th.errorBoxStyle, th.critBg, th.bold+th.red, th.reset, errText, innerWidth)
}

func (th ansiTheme) renderKPICards(cards [][2]string, totalWidth int) []string {
	return theme.RenderKPICards(th.mutedStyle, th.cardStyle, cards, totalWidth)
}

func (th ansiTheme) renderTableHeader(columns string, width int) []string {
	ruleWidth := max(30, width-6)
	return []string{
		fmt.Sprintf("  %s%s%s", th.bold+th.cyan, theme.TruncatePlain(columns, ruleWidth), th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", ruleWidth), th.reset),
	}
}

func (th ansiTheme) renderCursorPrefix(selected bool) string {
	if selected {
		return th.accent + "▸ " + th.reset
	}
	return "  "
}
