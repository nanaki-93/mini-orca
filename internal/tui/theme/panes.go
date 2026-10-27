package theme

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PaneStyle holds the SGR sequences a pane is drawn with. Background is
// re-applied after every reset inside content so nested styles never punch
// holes into the pane surface.
type PaneStyle struct {
	Background string
	Foreground string
	Border     string
	Focus      string
	Title      string
	Muted      string
	Reset      string
}

// Pane is one bordered region of the frame: a title (which may carry
// view tabs), right-aligned metadata, content rows and a scroll offset.
type Pane struct {
	Title   string
	Meta    string
	Body    []string
	Focused bool
	// Scroll is the first visible body row; it is clamped while rendering.
	Scroll int
	// Footer replaces the default scroll indicator in the bottom border.
	Footer string
}

// RenderPane draws p as exactly height rows of exactly width columns with a
// rounded border. Content wider than the pane is truncated with an ellipsis;
// content taller than the pane is windowed at p.Scroll with a position
// indicator in the bottom border.
func RenderPane(st PaneStyle, p Pane, width, height int) []string {
	if width < 4 || height < 2 {
		return blankRows(st, width, height)
	}
	border := st.Border
	if p.Focused {
		border = st.Focus
	}
	inner := width - 4
	rows := height - 2
	scroll := ClampScroll(p.Scroll, len(p.Body), rows)
	out := make([]string, 0, height)
	out = append(out, paneTop(st, border, p.Title, p.Meta, width))
	for i := 0; i < rows; i++ {
		line := ""
		if idx := scroll + i; idx < len(p.Body) {
			line = p.Body[idx]
		}
		cell := PaintLine(st.Background, st.Foreground, TruncateVisible(line, inner), inner)
		out = append(out, st.Background+border+"│"+st.Reset+st.Background+" "+cell+st.Background+" "+border+"│"+st.Reset)
	}
	footer := p.Footer
	if footer == "" && len(p.Body) > rows {
		footer = fmt.Sprintf("%d–%d of %d", scroll+1, min(len(p.Body), scroll+rows), len(p.Body))
	}
	out = append(out, paneBottom(st, border, footer, width))
	return out
}

// ClampScroll keeps a window of rows inside total lines.
func ClampScroll(scroll, total, rows int) int {
	if rows <= 0 || total <= rows {
		return 0
	}
	return min(max(0, scroll), total-rows)
}

// FollowScroll returns a scroll offset that keeps row visible, moving the
// window as little as possible from scroll.
func FollowScroll(scroll, row, total, rows int) int {
	if rows <= 0 {
		return 0
	}
	if row < scroll {
		scroll = row
	} else if row >= scroll+rows {
		scroll = row - rows + 1
	}
	return ClampScroll(scroll, total, rows)
}

func paneTop(st PaneStyle, border, title, meta string, width int) string {
	fill := width - 2
	var b strings.Builder
	b.WriteString(st.Background + border + "╭")
	used := 0
	if title != "" {
		budget := fill - 4
		if meta != "" {
			budget -= lipgloss.Width(meta) + 3
		}
		if budget >= 6 {
			t := TruncateVisible(title, budget)
			b.WriteString("─ " + st.Reset + st.Background + st.Title + strings.ReplaceAll(t, "\x1b[0m", "\x1b[0m"+st.Background+st.Title) + st.Reset + st.Background + border + " ")
			used = 3 + lipgloss.Width(t)
		}
	}
	metaW := 0
	if meta != "" {
		metaW = lipgloss.Width(meta) + 3
		if used+metaW > fill {
			metaW = 0
		}
	}
	b.WriteString(strings.Repeat("─", max(0, fill-used-metaW)))
	if metaW > 0 {
		b.WriteString(" " + st.Reset + st.Background + st.Muted + strings.ReplaceAll(meta, "\x1b[0m", "\x1b[0m"+st.Background+st.Muted) + st.Reset + st.Background + border + " ─")
	}
	b.WriteString("╮" + st.Reset)
	return b.String()
}

func paneBottom(st PaneStyle, border, footer string, width int) string {
	fill := width - 2
	var b strings.Builder
	b.WriteString(st.Background + border + "╰")
	footerW := 0
	if footer != "" {
		footerW = lipgloss.Width(footer) + 4
		if footerW > fill-2 {
			footerW = 0
		}
	}
	b.WriteString(strings.Repeat("─", max(0, fill-footerW)))
	if footerW > 0 {
		b.WriteString(" " + st.Reset + st.Background + st.Muted + footer + st.Reset + st.Background + border + " ─")
	}
	b.WriteString("╯" + st.Reset)
	return b.String()
}

func blankRows(st PaneStyle, width, height int) []string {
	out := make([]string, max(0, height))
	for i := range out {
		out[i] = PaintLine(st.Background, st.Foreground, "", max(0, width))
	}
	return out
}

// HStack joins columns of rows side by side. Every column must already be
// padded to its own width; shorter columns are padded with blank rows of
// widths[i] columns.
func HStack(columns [][]string, widths []int) []string {
	height := 0
	for _, col := range columns {
		height = max(height, len(col))
	}
	out := make([]string, height)
	for row := 0; row < height; row++ {
		var b strings.Builder
		for i, col := range columns {
			if row < len(col) {
				b.WriteString(col[row])
			} else {
				b.WriteString(strings.Repeat(" ", widths[i]))
			}
		}
		out[row] = b.String()
	}
	return out
}

// SplitWidths divides total columns between a left pane of about ratio
// (0–1) and the rest, keeping each at least minLeft/minRight when possible.
func SplitWidths(total int, ratio float64, minLeft, minRight int) (int, int) {
	left := int(float64(total) * ratio)
	left = max(minLeft, left)
	if total-left < minRight {
		left = max(minLeft, total-minRight)
	}
	left = min(left, total)
	return left, total - left
}

// Overlay draws box over base with its top-left corner at column x, row y.
// The base becomes a faint backdrop, and the rows the box covers show only
// the box, so nothing behind it reads as part of the dialog.
func Overlay(base, box []string, x, y int, background string) []string {
	out := make([]string, len(base))
	for i, row := range base {
		out[i] = "\x1b[2m" + strings.ReplaceAll(row, "\x1b[0m", "\x1b[0m\x1b[2m") + "\x1b[0m"
	}
	for i, line := range box {
		row := y + i
		if row < 0 || row >= len(out) {
			continue
		}
		width := lipgloss.Width(base[row])
		w := lipgloss.Width(line)
		left := background + strings.Repeat(" ", max(0, x))
		right := background + strings.Repeat(" ", max(0, width-x-w))
		out[row] = left + "\x1b[0m" + line + "\x1b[0m" + right + "\x1b[0m"
	}
	return out
}

// Tabs renders view labels for a pane title: the active one emphasized, the
// rest muted, separated by " · ".
func Tabs(labels []string, active int, activeStyle, mutedStyle, reset string) string {
	parts := make([]string, len(labels))
	for i, label := range labels {
		if i == active {
			parts[i] = activeStyle + label + reset
		} else {
			parts[i] = mutedStyle + label + reset
		}
	}
	return strings.Join(parts, mutedStyle+" · "+reset)
}
