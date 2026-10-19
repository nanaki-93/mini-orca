package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	lipgloss.SetColorProfile(termenv.ANSI256)
}

type ansiTheme struct {
	name     string
	accent   string
	accentBg string
	fg       string
	muted    string
	border   string
	surface  string
	selBg    string
	critBg   string
	warnBg   string
	okBg     string
	infoBg   string
	green    string
	amber    string
	red      string
	purple   string
	cyan     string
	bold     string
	dim      string
	reset    string

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

type paletteColors struct {
	name         string
	accentCode   string
	accentBgCode string
	fgCode       string
	mutedCode    string
	borderCode   string
	selBgCode    string
	greenCode    string
	amberCode    string
	redCode      string
	purpleCode   string
	cyanCode     string
	progStart    string
	progEnd      string
}

func colorsForPalette(p Palette) paletteColors {
	switch p {
	case PaletteCRT:
		return paletteColors{
			name: "CRT PHOSPHOR", accentCode: "46", accentBgCode: "28", fgCode: "120", mutedCode: "28",
			borderCode: "28", selBgCode: "22", greenCode: "46", amberCode: "226",
			redCode: "196", purpleCode: "83", cyanCode: "50",
			progStart: "#00aa00", progEnd: "#00ff66",
		}
	case PaletteAmber:
		return paletteColors{
			name: "AMBER TERMINAL", accentCode: "214", accentBgCode: "130", fgCode: "222", mutedCode: "136",
			borderCode: "136", selBgCode: "236", greenCode: "142", amberCode: "214",
			redCode: "203", purpleCode: "179", cyanCode: "215",
			progStart: "#d78700", progEnd: "#ffd75f",
		}
	case PalettePaper:
		return paletteColors{
			name: "PAPER LIGHT", accentCode: "25", accentBgCode: "25", fgCode: "235", mutedCode: "242",
			borderCode: "247", selBgCode: "252", greenCode: "28", amberCode: "130",
			redCode: "160", purpleCode: "91", cyanCode: "31",
			progStart: "#005f87", progEnd: "#0087af",
		}
	default:
		return paletteColors{
			name: "TOKYO ENTERPRISE", accentCode: "111", accentBgCode: "62", fgCode: "253", mutedCode: "244",
			borderCode: "239", selBgCode: "236", greenCode: "114", amberCode: "215",
			redCode: "203", purpleCode: "176", cyanCode: "117",
			progStart: "#5f87ff", progEnd: "#87d7ff",
		}
	}
}

func themeForPalette(p Palette) ansiTheme {
	c := colorsForPalette(p)
	accentCol := lipgloss.Color(c.accentCode)
	fgCol := lipgloss.Color(c.fgCode)
	mutedCol := lipgloss.Color(c.mutedCode)
	borderCol := lipgloss.Color(c.borderCode)
	selBgCol := lipgloss.Color(c.selBgCode)
	redCol := lipgloss.Color(c.redCode)

	return ansiTheme{
		name:     c.name,
		accent:   "\033[1;38;5;" + c.accentCode + "m",
		accentBg: "\033[1;38;5;255;48;5;" + c.accentBgCode + "m",
		fg:       "\033[38;5;" + c.fgCode + "m",
		muted:    "\033[38;5;" + c.mutedCode + "m",
		border:   "\033[38;5;" + c.borderCode + "m",
		surface:  "\033[48;5;" + c.selBgCode + "m",
		selBg:    "\033[48;5;" + c.selBgCode + "m",
		critBg:   "\033[1;38;5;255;48;5;160m",
		warnBg:   "\033[1;38;5;232;48;5;" + c.amberCode + "m",
		okBg:     "\033[1;38;5;232;48;5;" + c.greenCode + "m",
		infoBg:   "\033[1;38;5;255;48;5;" + c.accentBgCode + "m",
		green:    "\033[1;38;5;" + c.greenCode + "m",
		amber:    "\033[1;38;5;" + c.amberCode + "m",
		red:      "\033[1;38;5;" + c.redCode + "m",
		purple:   "\033[1;38;5;" + c.purpleCode + "m",
		cyan:     "\033[1;38;5;" + c.cyanCode + "m",
		bold:     "\033[1m",
		dim:      "\033[2m",
		reset:    "\033[0m",

		cardStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderCol).
			Padding(0, 1),
		activeTabStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color(c.accentBgCode)).
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
		progStartHex: c.progStart,
		progEndHex:   c.progEnd,
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

// sanitizeSingleLine normalizes tabs, carriage returns, newlines, and control characters
// so terminal cursor movement always matches lipgloss.Width column measurement.
func sanitizeSingleLine(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToValidUTF8(s, " ")
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if seqLen := ansiEscapeLen(s[i:]); seqLen > 0 {
			seq := s[i : i+seqLen]
			if seq[len(seq)-1] == 'm' {
				b.WriteString(seq)
			}
			i += seqLen
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		writeSanitizedRune(&b, r)
	}
	return b.String()
}

func writeSanitizedRune(b *strings.Builder, r rune) {
	switch {
	case r == '\t':
		b.WriteString("    ")
	case r == '\n':
		b.WriteByte(' ')
	case r == '\r' || r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
		return
	default:
		b.WriteRune(r)
	}
}

// sanitizeCodeText normalizes raw source or diff text so tabs, CR/LF, and literal ESC bytes never corrupt terminal layout.
func sanitizeCodeText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToValidUTF8(s, " ")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\x1b' {
			b.WriteString("\\x1b")
			continue
		}
		writeSanitizedRune(&b, r)
	}
	return b.String()
}

// visibleRuneCount measures the printable column width of an ANSI-styled string using lipgloss.Width.
func visibleRuneCount(s string) int {
	return lipgloss.Width(sanitizeSingleLine(s))
}

// padVisibleRight fits an ANSI-styled string to exact target visible columns by truncating or right-padding.
func padVisibleRight(s string, target int) string {
	if target <= 0 {
		return ""
	}
	s = sanitizeSingleLine(s)
	vis := lipgloss.Width(s)
	if vis > target {
		s = truncateVisible(s, target)
		vis = lipgloss.Width(s)
	}
	if vis >= target {
		return s
	}
	return s + strings.Repeat(" ", target-vis)
}

// truncateVisible shortens an ANSI-styled string to maxCols visible columns while preserving SGR sequences.
func truncateVisible(s string, maxCols int) string {
	s = sanitizeSingleLine(s)
	if maxCols <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxCols {
		return s
	}
	if maxCols == 1 {
		return "…"
	}
	limit := maxCols - 1
	var b strings.Builder
	cols := 0
	for i := 0; i < len(s); {
		if seqLen := ansiEscapeLen(s[i:]); seqLen > 0 {
			b.WriteString(s[i : i+seqLen])
			i += seqLen
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := max(1, lipgloss.Width(string(r)))
		if cols+rw > limit {
			break
		}
		b.WriteRune(r)
		cols += rw
		i += size
	}
	if actual := lipgloss.Width(b.String()); actual < limit {
		b.WriteString(strings.Repeat(" ", limit-actual))
	}
	b.WriteString("…\x1b[0m")
	return b.String()
}

func ansiEscapeLen(s string) int {
	if len(s) < 2 || s[0] != '\x1b' {
		return 0
	}
	if s[1] != '[' {
		return 2
	}
	for i := 2; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i + 1
		}
	}
	return len(s)
}

// truncatePlain shortens a plain string to maxCols visible columns using an ellipsis.
func truncatePlain(s string, maxCols int) string {
	s = sanitizeCodeText(s)
	if maxCols <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxCols {
		return s
	}
	if maxCols <= 1 {
		return "…"
	}
	limit := maxCols - 1
	var b strings.Builder
	cols := 0
	for _, r := range s {
		rw := max(1, lipgloss.Width(string(r)))
		if cols+rw > limit {
			break
		}
		b.WriteRune(r)
		cols += rw
	}
	return b.String() + "…"
}

// wrapFrameLine encloses a line in left and right outer vertical frame borders sized to width.
func (th ansiTheme) wrapFrameLine(line string, width int) string {
	inner := max(10, width-2)
	padded := padVisibleRight(line, inner)
	return fmt.Sprintf("%s┃%s%s%s┃%s", th.border, th.reset, padded, th.border, th.reset)
}

// keyPill formats a Bubbles key.Binding shortcut pill with high-contrast styling.
func (th ansiTheme) keyPill(k, desc string) string {
	b := key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc))
	help := b.Help()
	return fmt.Sprintf("%s[%s]%s %s%s%s", th.cyan, help.Key, th.reset, th.fg, help.Desc, th.reset)
}

// renderCommandInput renders the colon command bar using Bubbles textinput.Model.
func (th ansiTheme) renderCommandInput(value string) string {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.SetValue(value)
	ti.Focus()
	return fmt.Sprintf(" %s : COMMAND %s  %s%s%s", th.accentBg, th.reset, th.bold+th.fg, ti.View(), th.reset)
}

// renderProgressBar draws a smooth Bubbles progress.Model bar with percentage.
func (th ansiTheme) renderProgressBar(done, total, barWidth int) string {
	if barWidth < 8 {
		barWidth = 8
	}
	ratio := 0.0
	if total > 0 {
		clamped := min(max(0, done), total)
		ratio = float64(clamped) / float64(total)
	}
	prog := progress.New(
		progress.WithGradient(th.progStartHex, th.progEndHex),
		progress.WithWidth(barWidth+6),
		progress.WithoutPercentage(),
	)
	pct := int(ratio * 100)
	return fmt.Sprintf("%s %3d%%", prog.ViewAs(ratio), pct)
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

// renderPanelBox wraps content lines in a Lip Gloss rounded enterprise panel frame without line-wrap overflow.
func (th ansiTheme) renderPanelBox(title, rightMeta string, innerWidth int, body []string) []string {
	if innerWidth < 40 {
		innerWidth = 40
	}
	contentW := max(24, innerWidth-4)
	header := th.titleStyle.Render(strings.TrimSpace(title))
	if strings.TrimSpace(rightMeta) != "" {
		meta := th.mutedStyle.Render(strings.TrimSpace(rightMeta))
		maxTitleW := max(10, contentW-lipgloss.Width(meta)-2)
		if lipgloss.Width(header) > maxTitleW {
			header = truncateVisible(header, maxTitleW)
		}
		gap := max(2, contentW-lipgloss.Width(header)-lipgloss.Width(meta))
		header = header + strings.Repeat(" ", gap) + meta
	}
	allLines := make([]string, 0, len(body)+2)
	allLines = append(allLines, padVisibleRight(header, contentW))
	if len(body) > 0 {
		allLines = append(allLines, th.border+strings.Repeat("─", contentW)+th.reset)
	}
	for _, line := range body {
		allLines = append(allLines, padVisibleRight(line, contentW))
	}
	rendered := th.cardStyle.Render(strings.Join(allLines, "\n"))
	rawLines := strings.Split(rendered, "\n")
	out := make([]string, 0, len(rawLines))
	for _, rl := range rawLines {
		out = append(out, "  "+rl)
	}
	return out
}

// renderErrorBanner draws a Lip Gloss error callout box when s.Error is active.
func (th ansiTheme) renderErrorBanner(errText string, innerWidth int) []string {
	if innerWidth < 40 {
		innerWidth = 40
	}
	contentW := max(24, innerWidth-4)
	head := padVisibleRight(fmt.Sprintf("%s ▲ ERROR / DIAGNOSTIC ALERT %s", th.critBg, th.reset), contentW)
	msg := padVisibleRight(fmt.Sprintf("%s%s%s", th.bold+th.red, strings.TrimSpace(errText), th.reset), contentW)
	box := th.errorBoxStyle.Render(head + "\n" + msg)
	var out []string
	for _, line := range strings.Split(box, "\n") {
		out = append(out, "  "+line)
	}
	return out
}

// renderKPICards renders a horizontal strip of Lip Gloss KPI cards joined with lipgloss.JoinHorizontal.
func (th ansiTheme) renderKPICards(cards [][2]string, totalWidth int) []string {
	if len(cards) == 0 {
		return nil
	}
	cardInner := max(10, (totalWidth-4)/len(cards)-4)
	var boxes []string
	for _, c := range cards {
		label := padVisibleRight(th.mutedStyle.Render(truncatePlain(strings.ToUpper(c[0]), cardInner)), cardInner)
		val := padVisibleRight(c[1], cardInner)
		box := th.cardStyle.Render(label + "\n" + val)
		boxes = append(boxes, box)
	}
	joined := lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
	var out []string
	for _, line := range strings.Split(joined, "\n") {
		out = append(out, "  "+line)
	}
	return out
}

// renderTableHeader formats an aligned enterprise table header row with underline rule.
func (th ansiTheme) renderTableHeader(columns string, width int) []string {
	ruleWidth := max(30, width-6)
	return []string{
		fmt.Sprintf("  %s%s%s", th.bold+th.cyan, truncatePlain(columns, ruleWidth), th.reset),
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
