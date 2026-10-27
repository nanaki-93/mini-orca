package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// splitMinWidth is the narrowest body that shows a list and its detail side
// by side; narrower terminals stack them.
const splitMinWidth = 96

func (th ansiTheme) paneStyle() theme.PaneStyle {
	return theme.PaneStyle{
		Background: th.bg,
		Foreground: th.fg,
		Border:     th.border,
		Focus:      th.accent,
		Title:      th.bold + th.fg,
		Muted:      th.muted,
		Reset:      th.reset,
	}
}

func (th ansiTheme) modalStyle(tone string) theme.PaneStyle {
	st := th.paneStyle()
	st.Background = th.surface
	st.Focus = tone
	st.Border = tone
	return st
}

// renderBody lays out the active workspace as panes filling width × rows.
func (s *State) renderBody(th ansiTheme, width, rows int) []string {
	var body []string
	switch s.EffectivePage() {
	case PageSummary:
		body = s.renderInboxWorkspace(th, width, rows)
	case PageAgents:
		body = s.renderTasksWorkspace(th, width, rows)
	case PageEditor:
		body = s.renderCodeWorkspace(th, width, rows)
	case PageBugs, PageIssueDetail:
		body = s.renderIssuesWorkspace(th, width, rows)
	case PageGit, PageWorktreeDetail:
		body = s.renderBranchesWorkspace(th, width, rows)
	case PageDocs, PageDocsComponents, PageDocsDecisions, PageDiagrams:
		body = s.renderDocsWorkspace(th, width, rows)
	default:
		body = s.renderContentPane(th, width, rows)
	}
	for len(body) < rows {
		body = append(body, "")
	}
	return body[:rows]
}

// viewTabsFit is viewTabs within budget columns: when every view does not
// fit, it names the current view and its position instead.
func (s *State) viewTabsFit(th ansiTheme, budget int) string {
	hub := s.ActiveHub()
	current := parentPage(s.EffectivePage())
	var labels []string
	active := 0
	for _, sp := range hub.SubPages {
		if sp.Page == PageIssueDetail {
			continue
		}
		if sp.Page == current {
			active = len(labels)
		}
		labels = append(labels, sp.Label)
	}
	if len(labels) == 1 {
		return labels[0]
	}
	tabs := theme.Tabs(labels, active, th.bold+th.accent, th.muted, th.reset)
	if theme.VisibleRuneCount(tabs) <= budget {
		return tabs
	}
	return th.bold + th.accent + labels[active] + th.reset + th.muted + fmt.Sprintf("  %d/%d · Tab", active+1, len(labels)) + th.reset
}

// renderContentPane shows a page's full content in one pane titled with the
// workspace's views; the selected row stays visible and PgUp/PgDn scroll.
func (s *State) renderContentPane(th ansiTheme, width, rows int) []string {
	inner := width - 4
	s.viewportBodyRows = max(1, rows-2)
	// Page renderers indent everything by two columns for full-width
	// layouts; render two columns wider and drop that margin in the pane.
	body := s.renderActivePage(th, inner+2)
	body = wrapWorkspaceLines(body, inner+2)
	body = tidyContent(th, body, inner)
	scroll := s.followContentScroll(th, body, rows-2)
	meta := s.pageMeta()
	return theme.RenderPane(th.paneStyle(), theme.Pane{Title: s.viewTabsFit(th, width-12-theme.VisibleRuneCount(meta)), Meta: meta, Body: body, Focused: true, Scroll: scroll}, width, rows)
}

// tidyContent removes the two-column left margin page renderers add for
// full-width layouts (the pane has its own padding) and paints the cursor row.
func tidyContent(th ansiTheme, lines []string, width int) []string {
	marker := "  " + th.renderCursorPrefix(true)
	out := make([]string, len(lines))
	for i, line := range lines {
		selected := strings.HasPrefix(line, marker)
		line = strings.TrimPrefix(strings.TrimPrefix(line, " "), " ")
		if selected {
			line = theme.PaintLine(th.selection, th.fg, line, width)
		}
		out[i] = line
	}
	return out
}

// followContentScroll keeps the cursor row in view after navigation and
// otherwise honors the reader's PgUp/PgDn position.
func (s *State) followContentScroll(th ansiTheme, body []string, rows int) int {
	if s.viewport.followSelection {
		s.viewport.followSelection = false
		s.viewport.manualScroll = false
		for i, line := range body {
			if strings.HasPrefix(line, th.selection) {
				s.DetailScroll = theme.FollowScroll(s.DetailScroll, i, len(body), rows)
				break
			}
		}
	}
	s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(body), rows)
	return s.DetailScroll
}

// pageMeta is the short right-aligned fact in a content pane's top border.
func (s *State) pageMeta() string {
	switch s.EffectivePage() {
	case PageInstructions:
		if s.WorkspaceTrust {
			return "trusted"
		}
		return "not trusted"
	case PageHistory:
		return fmt.Sprintf("%d proposals", len(s.ChangeHistory))
	case PageFeatures, PageFeatureDetail:
		return fmt.Sprintf("%d ideas", len(s.activeFeatures()))
	case PageAnalysisRun:
		if s.Run != nil {
			return s.Run.Status
		}
	}
	return ""
}

// splitPanes renders a list pane and a detail pane side by side, or stacked
// on narrow terminals (the list keeps at least a third of the rows).
func (s *State) splitPanes(th ansiTheme, width, rows int, ratio float64, left, right theme.Pane) []string {
	st := th.paneStyle()
	if width < splitMinWidth {
		top := stackedListRows(rows)
		out := theme.RenderPane(st, left, width, top)
		return append(out, theme.RenderPane(st, right, width, rows-top)...)
	}
	lw, rw := theme.SplitWidths(width, ratio, 34, 48)
	return theme.HStack([][]string{theme.RenderPane(st, left, lw, rows), theme.RenderPane(st, right, rw, rows)}, []int{lw, rw})
}

// paneRows is the content height of a pane of the given outer height.
func paneRows(rows int) int { return max(1, rows-2) }

// stackedListRows is the outer height of the list pane when a split view
// stacks on a narrow terminal.
func stackedListRows(rows int) int {
	if rows < 18 {
		return max(4, rows/2)
	}
	return max(5, rows*2/5)
}

// listPaneRows returns the content rows the list pane gets for a split.
func listPaneRows(width, rows int) int {
	if width < splitMinWidth {
		return paneRows(stackedListRows(rows))
	}
	return paneRows(rows)
}

// detailPaneRows returns the content rows the detail pane gets for a split.
func detailPaneRows(width, rows int) int {
	if width < splitMinWidth {
		return paneRows(rows - stackedListRows(rows))
	}
	return paneRows(rows)
}

// listPaneWidth returns the inner width of the list pane for a split.
func listPaneWidth(width int, ratio float64) int {
	if width < splitMinWidth {
		return width - 4
	}
	lw, _ := theme.SplitWidths(width, ratio, 34, 48)
	return lw - 4
}

// detailPaneWidth returns the inner width of the detail pane for a split.
func detailPaneWidth(width int, ratio float64) int {
	if width < splitMinWidth {
		return width - 4
	}
	_, rw := theme.SplitWidths(width, ratio, 34, 48)
	return rw - 4
}

// listRow renders one selectable row: a selection bar and background when
// selected, so the cursor reads at a glance in every palette.
func (th ansiTheme) listRow(selected bool, content string, width int) string {
	if selected {
		return theme.PaintLine(th.selection, th.fg, th.accent+"▌"+th.reset+th.selection+content, width)
	}
	return " " + content
}

// sectionHeader labels a group of rows with a muted uppercase title and count.
func (th ansiTheme) sectionHeader(label string, count int) string {
	if count < 0 {
		return th.muted + strings.ToUpper(label) + th.reset
	}
	return th.muted + strings.ToUpper(label) + th.reset + " " + th.border + fmt.Sprintf("%d", count) + th.reset
}

// field renders a label/value row with aligned values.
func (th ansiTheme) field(label, value string, labelW int) string {
	return th.muted + theme.PadVisibleRight(label, labelW) + th.reset + " " + value
}

// wrapText wraps plain or styled text to width, without a margin.
func wrapText(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		for _, row := range theme.WrapVisibleLine(para, max(1, width)) {
			out = append(out, strings.TrimRight(row, " "))
		}
	}
	return out
}

// renderOverlay draws the active modal (commands, confirmation, help,
// messages or an error) centered over the body.
func (s *State) renderOverlay(th ansiTheme, body []string, width, rows int) []string {
	var box []string
	y := 0
	// Small terminals give modals the full width and height so nothing of
	// the workspace interleaves with what the modal says.
	fit := func(limit int) int {
		if width < 70 {
			return width
		}
		return min(limit, width-6)
	}
	maxRows := rows - 2
	if rows < 18 {
		maxRows = rows
	}
	switch {
	case s.CmdActive:
		box = s.renderCommandPalette(th, fit(100), min(maxRows, 24))
		y = 1
	case s.Confirmation != nil:
		box = s.renderConfirmationBox(th, fit(76), maxRows)
	case s.nav.showLog:
		box = s.renderMessageLogBox(th, fit(110), maxRows)
		y = 1
	case s.ShowKeybindsModal:
		box = s.renderHelpBox(th, fit(112), maxRows)
		y = 1
	case s.Error != "":
		box = s.renderErrorBox(th, fit(84), maxRows)
	default:
		s.viewport.dialogKey, s.viewport.dialogScroll = "", 0
		return body
	}
	if y == 0 {
		y = max(0, (rows-len(box))/3)
	}
	if rows < 18 || y+len(box) > rows {
		y = max(0, rows-len(box))
		if rows < 18 {
			y = 0
		}
	}
	x := max(0, (width-theme.VisibleRuneCount(box[0]))/2)
	return theme.Overlay(body, box, x, y, th.bg)
}

func (s *State) renderConfirmationBox(th ansiTheme, width, maxRows int) []string {
	c := s.Confirmation
	tone := th.accent
	if c.Destructive {
		tone = th.red
	}
	inner := width - 4
	body := []string{""}
	body = append(body, wrapText(theme.SanitizeSingleLine(c.Message), inner)...)
	body = append(body, "", th.accentBg+" y "+th.reset+" "+th.bold+th.fg+client.FirstNonEmpty(c.AcceptLabel, "Confirm")+th.reset+"    "+th.cyan+"n"+th.reset+" "+th.fg+"Cancel"+th.reset)
	return s.dialogBox(th, tone, c.Title, "y/n", c.Title+"\x00"+c.Message, body, width, maxRows)
}

func (s *State) renderErrorBox(th ansiTheme, width, maxRows int) []string {
	inner := width - 4
	body := []string{""}
	body = append(body, wrapText(theme.SanitizeSingleLine(s.Error), inner)...)
	body = append(body, "")
	if !s.Connected {
		body = append(body, th.muted+"Start the daemon, then run :reconnect."+th.reset)
	}
	body = append(body, th.cyan+"Esc"+th.reset+" dismiss   "+th.cyan+":"+th.reset+" commands   "+th.cyan+":messages"+th.reset+" history")
	return s.dialogBox(th, th.red, th.red+"✖ "+th.reset+"Something went wrong", "", s.Error, body, width, maxRows)
}

// dialogBox sizes a modal to its content, scrolling long messages with
// PgUp/PgDn through the shared dialog scroll position.
func (s *State) dialogBox(th ansiTheme, tone, title, meta, key string, body []string, width, maxRows int) []string {
	if s.viewport.dialogKey != key {
		s.viewport.dialogKey, s.viewport.dialogScroll = key, 0
	}
	height := min(maxRows, len(body)+2)
	s.viewport.dialogScroll = theme.ClampScroll(s.viewport.dialogScroll, len(body), height-2)
	return theme.RenderPane(th.modalStyle(tone), theme.Pane{Title: title, Meta: meta, Body: body, Focused: true, Scroll: s.viewport.dialogScroll}, width, height)
}

// renderCommandPalette shows the command input, ranked matches and the
// selected command's explanation.
func (s *State) renderCommandPalette(th ansiTheme, width, maxRows int) []string {
	inner := width - 4
	input := th.renderCommandInput(s.CmdInput, s.commandCursor, inner)
	body := []string{theme.PaintLine(th.selection, th.fg, input, inner), ""}
	if item := s.commandArgumentContext(); item != nil {
		if s.commandMissingArguments() != nil {
			body = append(body, th.amber+requiredCommandArgumentHint+th.reset)
		}
		body = append(body, th.bold+th.fg+item.title+th.reset)
		body = append(body, wrapText(th.muted+item.hint+th.reset, inner)...)
		return theme.RenderPane(th.modalStyle(th.accent), theme.Pane{Title: "Commands", Meta: "Enter run", Body: body, Focused: true}, width, min(maxRows, len(body)+2))
	}
	items := s.commandMatches()
	if len(items) == 0 {
		body = append(body, th.muted+"No matching commands. Enter runs what you typed."+th.reset)
		return theme.RenderPane(th.modalStyle(th.accent), theme.Pane{Title: "Commands", Body: body, Focused: true}, width, min(maxRows, len(body)+2))
	}
	selected := min(max(0, s.commandSelection), len(items)-1)
	hint := wrapText(items[selected].hint, inner)
	listRows := max(1, maxRows-2-len(body)-len(hint)-1)
	start, end := windowBounds(len(items), selected, listRows)
	commandW := min(22, inner/3)
	for i := start; i < end; i++ {
		item := items[i]
		row := theme.PadVisibleRight(th.cyan+":"+item.command+th.reset, commandW) + " " + th.fg + item.title + th.reset
		body = append(body, th.listRow(i == selected, row, inner))
	}
	body = append(body, "")
	for _, line := range hint {
		body = append(body, th.muted+line+th.reset)
	}
	meta := fmt.Sprintf("%d–%d of %d", start+1, end, len(items))
	return theme.RenderPane(th.modalStyle(th.accent), theme.Pane{Title: "Commands", Meta: meta, Body: body, Focused: true}, width, min(maxRows, len(body)+2))
}

// renderHelpBox lists the keys of the current view, global navigation and
// the commands, in two columns when there is room.
func (s *State) renderHelpBox(th ansiTheme, width, maxRows int) []string {
	spec := lookupPageHelpSpec(s.EffectivePage())
	if s.EffectivePage() == PagePlan && s.PlanView == "mission" {
		spec = s.missionHelpSpec()
	}
	inner := width - 4
	global := []keyDesc{
		{"1–7", "Switch workspace"},
		{"Tab / Shift+Tab", "Next / previous view"},
		{"[ / ]", "Previous / next workspace"},
		{"↑/↓ or j/k", "Move selection"},
		{"Enter", "Open or run the primary action"},
		{"Esc / Backspace", "Back"},
		{"Ctrl+O", "Previous view"},
		{"PgUp / PgDn", "Scroll the detail"},
		{"Ctrl+K or :", "Commands"},
		{":web", "Open this view in the browser"},
		{":messages", "Recent messages and errors"},
		{"p", "Next color palette"},
		{"Ctrl+C", "Quit"},
	}
	var page []keyDesc
	page = append(page, spec.primary...)
	page = append(page, spec.secondary...)
	page = append(page, spec.commands...)
	section := func(title string, items []keyDesc) []string {
		out := []string{th.bold + th.accent + title + th.reset}
		return append(out, renderCommandCatalogRows(th, items, inner)...)
	}
	body := section(strings.ToUpper(spec.title), page)
	body = append(body, "")
	body = append(body, section("NAVIGATION", global)...)
	body = append(body, "")
	body = append(body, section("COMMANDS", allColonCommands)...)
	height := min(maxRows, len(body)+2)
	s.KeybindsScroll = theme.ClampScroll(s.KeybindsScroll, len(body), height-2)
	return theme.RenderPane(th.modalStyle(th.accent), theme.Pane{Title: "Keyboard shortcuts", Meta: "? or Esc close", Body: body, Focused: true, Scroll: s.KeybindsScroll}, width, height)
}

// renderMessageLogBox lists recent messages, newest first.
func (s *State) renderMessageLogBox(th ansiTheme, width, maxRows int) []string {
	inner := width - 4
	var body []string
	if len(s.nav.log) == 0 {
		body = append(body, th.muted+"No messages yet. Status updates and errors appear here as you work."+th.reset)
	}
	for i := len(s.nav.log) - 1; i >= 0; i-- {
		entry := s.nav.log[i]
		tone, mark := th.fg, th.muted+"·"+th.reset
		if entry.isError {
			tone, mark = th.red, th.red+"✖"+th.reset
		}
		prefix := th.muted + entry.at.Format("15:04:05") + th.reset + " " + mark + " "
		for j, line := range wrapPlain(entry.text, max(20, inner-12)) {
			if j > 0 {
				prefix = strings.Repeat(" ", 11)
			}
			body = append(body, prefix+tone+line+th.reset)
		}
	}
	height := min(maxRows, max(6, len(body)+2))
	s.nav.logScroll = theme.ClampScroll(s.nav.logScroll, len(body), height-2)
	return theme.RenderPane(th.modalStyle(th.accent), theme.Pane{Title: "Messages", Meta: fmt.Sprintf("%d · newest first", len(s.nav.log)), Body: body, Focused: true, Scroll: s.nav.logScroll}, width, height)
}
