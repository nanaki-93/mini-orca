package theme

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// NoColor reports whether the user asked for monochrome output with the
// NO_COLOR convention (https://no-color.org). It is read once at startup.
var NoColor = os.Getenv("NO_COLOR") != ""

func init() {
	if NoColor {
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	lipgloss.SetColorProfile(termenv.ANSI256)
}

// Palette identifies one of the four terminal color themes.
type Palette string

const (
	PaletteTokyo Palette = "tokyo"
	PaletteCRT   Palette = "crt"
	PaletteAmber Palette = "amber"
	PalettePaper Palette = "paper"
)

type PaletteColors struct {
	Name         string
	BgCode       string
	SurfaceCode  string
	AccentCode   string
	AccentBgCode string
	OnAccentCode string
	OnStatusCode string
	FgCode       string
	MutedCode    string
	BorderCode   string
	SelBgCode    string
	GreenCode    string
	AmberCode    string
	RedCode      string
	PurpleCode   string
	CyanCode     string
	ProgStart    string
	ProgEnd      string
	// AddBgCode and DelBgCode tint added and removed diff lines.
	AddBgCode string
	DelBgCode string
}

func ColorsForPalette(p Palette) PaletteColors {
	switch p {
	case PaletteCRT:
		return PaletteColors{
			Name: "CRT PHOSPHOR", BgCode: "232", SurfaceCode: "234", AccentCode: "46", AccentBgCode: "28", OnAccentCode: "231", OnStatusCode: "232", FgCode: "157", MutedCode: "114",
			BorderCode: "28", SelBgCode: "236", GreenCode: "46", AmberCode: "226",
			RedCode: "210", PurpleCode: "157", CyanCode: "50",
			ProgStart: "#00aa00", ProgEnd: "#00ff66", AddBgCode: "22", DelBgCode: "52",
		}
	case PaletteAmber:
		return PaletteColors{
			Name: "AMBER TERMINAL", BgCode: "233", SurfaceCode: "235", AccentCode: "222", AccentBgCode: "130", OnAccentCode: "231", OnStatusCode: "232", FgCode: "223", MutedCode: "180",
			BorderCode: "136", SelBgCode: "237", GreenCode: "150", AmberCode: "222",
			RedCode: "210", PurpleCode: "183", CyanCode: "117",
			ProgStart: "#d78700", ProgEnd: "#ffd75f", AddBgCode: "58", DelBgCode: "52",
		}
	case PalettePaper:
		return PaletteColors{
			Name: "PAPER LIGHT", BgCode: "255", SurfaceCode: "254", AccentCode: "25", AccentBgCode: "25", OnAccentCode: "255", OnStatusCode: "255", FgCode: "235", MutedCode: "240",
			BorderCode: "247", SelBgCode: "253", GreenCode: "22", AmberCode: "58",
			RedCode: "124", PurpleCode: "91", CyanCode: "24",
			ProgStart: "#005f87", ProgEnd: "#0087af", AddBgCode: "194", DelBgCode: "224",
		}
	default:
		return PaletteColors{
			Name: "TOKYO", BgCode: "233", SurfaceCode: "235", AccentCode: "111", AccentBgCode: "62", OnAccentCode: "231", OnStatusCode: "232", FgCode: "253", MutedCode: "248",
			BorderCode: "240", SelBgCode: "237", GreenCode: "114", AmberCode: "222",
			RedCode: "210", PurpleCode: "183", CyanCode: "80",
			ProgStart: "#87afff", ProgEnd: "#5fd7d7", AddBgCode: "22", DelBgCode: "52",
		}
	}
}

// ClampWidth normalizes viewport width across compact and full-screen ultrawide terminals.
func ClampWidth(width int) int {
	if width <= 0 {
		return 120
	}
	return width
}

// ClampHeight normalizes viewport height so full-screen terminals are filled cleanly.
func ClampHeight(height int) int {
	if height <= 0 {
		return 34
	}
	return height
}

// SanitizeSingleLine normalizes tabs, carriage returns, newlines, and control characters
// so terminal cursor movement always matches lipgloss.Width column measurement.
func SanitizeSingleLine(s string) string {
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

// SanitizeCodeText normalizes raw source or diff text so tabs, CR/LF, and literal ESC bytes never corrupt terminal layout.
func SanitizeCodeText(s string) string {
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

// VisibleRuneCount measures the printable column width of an ANSI-styled string using lipgloss.Width.
func VisibleRuneCount(s string) int {
	return lipgloss.Width(SanitizeSingleLine(s))
}

// PadVisibleRight fits an ANSI-styled string to exact target visible columns by truncating or right-padding.
func PadVisibleRight(s string, target int) string {
	if target <= 0 {
		return ""
	}
	s = SanitizeSingleLine(s)
	vis := lipgloss.Width(s)
	if vis > target {
		s = TruncateVisible(s, target)
		vis = lipgloss.Width(s)
	}
	if vis >= target {
		return s
	}
	return s + strings.Repeat(" ", target-vis)
}

// PaintLine fills a terminal row and restores its surface after nested styles reset.
func PaintLine(background, foreground, line string, width int) string {
	base := background + foreground
	line = PadVisibleRight(line, width)
	return base + strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+base) + "\x1b[0m"
}

// TruncateVisible shortens an ANSI-styled string to maxCols visible columns while preserving SGR sequences.
func TruncateVisible(s string, maxCols int) string {
	s = SanitizeSingleLine(s)
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

// TruncatePlain shortens a plain string to maxCols visible columns using an ellipsis.
func TruncatePlain(s string, maxCols int) string {
	s = SanitizeCodeText(s)
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

// KeyPill formats a Bubbles key.Binding shortcut pill with high-contrast styling.
func KeyPill(cyan, fg, reset, k, desc string) string {
	b := key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc))
	help := b.Help()
	return fmt.Sprintf("%s[%s]%s %s%s%s", cyan, help.Key, reset, fg, help.Desc, reset)
}

// RenderCommandInput renders the colon command bar using Bubbles textinput.Model.
func RenderCommandInput(boldFg, reset, value string, cursor, width int) string {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.Width = max(1, width-6)
	ti.SetValue(value)
	ti.SetCursor(cursor)
	ti.Focus()
	return fmt.Sprintf(" %s%s%s", boldFg, ti.View(), reset)
}

// RenderProgressBar draws a smooth Bubbles progress.Model bar with percentage.
func RenderProgressBar(progStartHex, progEndHex string, done, total, barWidth int) string {
	if barWidth < 8 {
		barWidth = 8
	}
	ratio := 0.0
	if total > 0 {
		clamped := min(max(0, done), total)
		ratio = float64(clamped) / float64(total)
	}
	prog := progress.New(
		progress.WithGradient(progStartHex, progEndHex),
		progress.WithWidth(barWidth+6),
		progress.WithoutPercentage(),
	)
	pct := int(ratio * 100)
	return fmt.Sprintf("%s %3d%%", prog.ViewAs(ratio), pct)
}

// WrapVisibleLine wraps an ANSI-styled line into one or more lines of exact maxCols visible width without truncating text.
func WrapVisibleLine(s string, maxCols int) []string {
	s = SanitizeSingleLine(s)
	if maxCols <= 0 {
		return nil
	}
	if lipgloss.Width(s) <= maxCols {
		return []string{PadVisibleRight(s, maxCols)}
	}
	var lines []string
	rem := s
	activeSGR := ""
	for rem != "" {
		chunk, rest, nextSGR := splitVisibleChunk(rem, maxCols, activeSGR)
		lines = append(lines, PadVisibleRight(activeSGR+chunk+"\x1b[0m", maxCols))
		rem = strings.TrimLeft(rest, " ")
		activeSGR = nextSGR
	}
	return lines
}

func splitVisibleChunk(s string, maxCols int, startSGR string) (string, string, string) {
	cols := 0
	curSGR := startSGR
	lastSpaceIdx := -1
	lastSpaceSGR := curSGR
	for i := 0; i < len(s); {
		if seqLen := ansiEscapeLen(s[i:]); seqLen > 0 {
			seq := s[i : i+seqLen]
			if seq == "\x1b[0m" {
				curSGR = ""
			} else {
				curSGR += seq
			}
			i += seqLen
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := max(1, lipgloss.Width(string(r)))
		if cols+rw > maxCols {
			if lastSpaceIdx > 0 {
				return s[:lastSpaceIdx], s[lastSpaceIdx+1:], lastSpaceSGR
			}
			return s[:i], s[i:], curSGR
		}
		if r == ' ' && cols > 0 {
			lastSpaceIdx = i
			lastSpaceSGR = curSGR
		}
		cols += rw
		i += size
	}
	return s, "", curSGR
}

// RenderPanelBox wraps content lines in a Lip Gloss rounded enterprise panel frame without line-wrap overflow.
func RenderPanelBox(titleStyle, mutedStyle, cardStyle lipgloss.Style, border, reset, title, rightMeta string, innerWidth int, body []string) []string {
	if innerWidth < 40 {
		innerWidth = 40
	}
	contentW := max(24, innerWidth-4)
	allLines := make([]string, 0, len(body)+2)
	if strings.TrimSpace(title) != "" || strings.TrimSpace(rightMeta) != "" {
		header := titleStyle.Render(strings.TrimSpace(title))
		if strings.TrimSpace(rightMeta) != "" {
			meta := mutedStyle.Render(strings.TrimSpace(rightMeta))
			maxTitleW := max(10, contentW-lipgloss.Width(meta)-2)
			if lipgloss.Width(header) > maxTitleW {
				header = TruncateVisible(header, maxTitleW)
			}
			gap := max(2, contentW-lipgloss.Width(header)-lipgloss.Width(meta))
			header = header + strings.Repeat(" ", gap) + meta
		}
		allLines = append(allLines, PadVisibleRight(header, contentW))
	}
	for _, line := range body {
		allLines = append(allLines, WrapVisibleLine(line, contentW)...)
	}
	rendered := cardStyle.Render(strings.Join(allLines, "\n"))
	rawLines := strings.Split(rendered, "\n")
	out := make([]string, 0, len(rawLines))
	for _, rl := range rawLines {
		out = append(out, "  "+rl)
	}
	return out
}

// RenderErrorBanner draws a Lip Gloss error callout box when s.Error is active.
func RenderErrorBanner(errorBoxStyle lipgloss.Style, critBg, boldRed, reset, errText string, innerWidth int) []string {
	if innerWidth < 40 {
		innerWidth = 40
	}
	contentW := max(24, innerWidth-4)
	allLines := []string{PadVisibleRight(fmt.Sprintf("%s ▲ ERROR / DIAGNOSTIC ALERT %s", critBg, reset), contentW)}
	allLines = append(allLines, WrapVisibleLine(fmt.Sprintf("%s%s%s", boldRed, strings.TrimSpace(errText), reset), contentW)...)
	box := errorBoxStyle.Render(strings.Join(allLines, "\n"))
	var out []string
	for _, line := range strings.Split(box, "\n") {
		out = append(out, "  "+line)
	}
	return out
}

// RenderMetrics shows evenly spaced count cards with explicit surface backgrounds.
func RenderMetrics(mutedStyle lipgloss.Style, surface, foreground string, cards [][2]string, totalWidth int) []string {
	if len(cards) == 0 {
		return nil
	}
	columns := len(cards)
	if totalWidth < 76 {
		columns = min(columns, 2)
	}
	column := max(1, (totalWidth-4-2*(columns-1))/columns)
	rule := mutedStyle.Render(strings.Repeat("─", max(1, column-2)))
	var lines []string
	for start := 0; start < len(cards); start += columns {
		rows := []string{"  ", "  ", "  "}
		for i, item := range cards[start:min(start+columns, len(cards))] {
			cells := []string{
				" " + mutedStyle.Render(item[0]),
				" " + item[1],
				" " + rule,
			}
			for row, content := range cells {
				if i > 0 {
					rows[row] += "  "
				}
				rows[row] += PaintLine(surface, foreground, content, column)
			}
		}
		lines = append(lines, rows...)
	}
	return lines
}
