package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const sidebarMenuWidth = 24

// Render generates a full-screen frame that never exceeds the terminal's dimensions.
func (s *State) Render(width, height int) string {
	width, height = theme.ClampWidth(width), theme.ClampHeight(height)
	th := themeForPalette(s.Palette)
	if s.viewport.width != width || s.viewport.height != height {
		s.viewport.width, s.viewport.height = width, height
		s.viewport.followSelection = true
	}
	if width < 52 || height < 16 {
		return s.renderSmallTerminal(th, width, height)
	}
	header := []string{theme.PaintLine(th.surface, th.fg, s.renderBrandBar(th, width), width), s.renderStatusBar(th, width)}
	sidebar := width >= 100 && height >= 20
	if !sidebar {
		header = append(header, s.renderHubBar(th, width), s.renderSubViewBar(th, width))
	}
	rule := th.border + strings.Repeat("─", width) + th.reset
	header = append(header, rule)
	footer := []string{rule, theme.PaintLine(th.surface, th.fg, s.renderActionBar(th, width), width), s.renderCommandBar(th, width)}
	bodyRows := height - len(header) - len(footer)
	contentWidth := width
	if sidebar {
		contentWidth -= sidebarMenuWidth + 1
	}
	s.viewportBodyRows = bodyRows
	body := s.renderWorkspaceContent(th, contentWidth, bodyRows)
	if sidebar {
		menu := s.renderSidebarMenu(th, sidebarMenuWidth, bodyRows)
		for i := 0; i < bodyRows; i++ {
			if i >= len(body) {
				body = append(body, "")
			}
			body[i] = theme.PaintLine(th.surface, th.fg, menu[i], sidebarMenuWidth) + th.border + "│" + th.reset + body[i]
		}
	}
	lines := append([]string(nil), header...)
	for i := 0; i < bodyRows; i++ {
		row := ""
		if i < len(body) {
			row = body[i]
		}
		lines = append(lines, row)
	}
	return fitFrame(th, append(lines, footer...), width, height)
}

func fitFrame(th ansiTheme, lines []string, width, height int) string {
	out := make([]string, height)
	for i := range out {
		row := ""
		if i < len(lines) {
			row = lines[i]
		}
		out[i] = theme.PaintLine(th.bg, th.fg, row, width)
	}
	return strings.Join(out, "\n")
}

func (s *State) renderSmallTerminal(th ansiTheme, width, height int) string {
	lines := []string{"MINI-ORCA", "", "Enlarge the terminal to 52 × 16.", "Your workspace is preserved.", "", "Ctrl+C: quit"}
	if s.CmdActive {
		lines = []string{"MINI-ORCA", th.renderCommandInput(s.CmdInput, s.commandCursor, width), "Enter: run · Esc: cancel"}
	}
	return fitFrame(th, lines, width, height)
}

func (s *State) renderBrandBar(th ansiTheme, width int) string {
	name := "Open a project to begin"
	if s.Project != nil {
		name = client.FirstNonEmpty(s.Project.Name, s.Project.Path)
	}
	right := th.amber + "○ Offline" + th.reset
	if s.Connected {
		right = th.green + "● Connected" + th.reset
	}
	if count := len(s.AgentItems()); count > 0 && width >= 88 {
		right = fmt.Sprintf("%s%d task(s)%s  %s", th.purple, count, th.reset, right)
	}
	brand := th.accentBg + " ◈ MINI-ORCA " + th.reset
	nameBudget := max(8, width-theme.VisibleRuneCount(right)-26)
	left := brand + th.border + " │ " + th.reset + th.bold + th.fg + theme.TruncatePlain(name, nameBudget) + th.reset
	if width >= 104 {
		active := s.ActiveHub()
		left += th.muted + "  ·  " + th.hubColor(active.Key) + "[" + active.Key + "] " + active.Label + th.reset
	}
	return alignBar(left, right+" ", width)
}

func alignBar(left, right string, width int) string {
	leftWidth := max(0, width-theme.VisibleRuneCount(right)-2)
	return theme.PadVisibleRight(left, leftWidth) + "  " + right
}

func (s *State) renderWorkspaceContent(th ansiTheme, width, rows int) []string {
	s.viewportBodyRows = max(1, rows-1)
	var body []string
	switch {
	case s.CmdActive:
		body = s.renderCommandMenu(th, width, rows-1)
	case s.Confirmation != nil:
		body = s.renderDialog(th, width, rows-1)
	case s.ShowKeybindsModal:
		body = s.renderKeybindsModal(th, width)
	case s.Error != "":
		body = s.renderDialog(th, width, rows-1)
	default:
		s.viewport.dialogKey, s.viewport.dialogScroll = "", 0
		body = s.renderActivePage(th, width)
		if !(s.Page == PageEditor && s.isViewingDiff()) {
			body = wrapWorkspaceLines(body, width)
		}
		body = s.scrollDetails(th, body, rows-1)
	}
	for i, line := range body {
		if strings.HasPrefix(line, "  "+th.renderCursorPrefix(true)) {
			body[i] = theme.PaintLine(th.selection, th.fg, line, width-2)
		}
	}
	return append([]string{""}, body...)
}

func (s *State) renderSidebarMenu(th ansiTheme, width, rows int) []string {
	active := s.ActiveHub()
	models := s.renderSidebarModels(th, width)
	subPages := s.renderSidebarSubPages(th, active, width, rows-len(HubTabs())-len(models))
	roomy := rows >= len(HubTabs())+len(subPages)+len(models)+5
	var lines []string
	if roomy {
		lines = append(lines, "", "  "+th.bold+th.cyan+"◈ WORKSPACE"+th.reset, "")
	}
	for _, tab := range HubTabs() {
		style := th.hubColor(tab.Key)
		if tab.Key == active.Key {
			style += th.bold
		}
		lines = append(lines, "  "+style+tab.Key+"  "+tab.Label+th.reset)
		if tab.Key == active.Key {
			lines = append(lines, subPages...)
			if roomy {
				lines = append(lines, "")
			}
		}
	}
	if rows-len(lines) >= len(models)+2 {
		models = append([]string{"  " + th.border + strings.Repeat("─", width-4) + th.reset, "  " + th.muted + "◈ MODELS" + th.reset}, models...)
	}
	for len(lines) < rows-len(models) {
		lines = append(lines, "")
	}
	return append(lines, models...)
}

func (s *State) renderSidebarModels(th ansiTheme, width int) []string {
	var lines []string
	for _, role := range []struct{ scope, label string }{{"analyze", "Analyze"}, {"bug", "Bug"}, {"function", "Code"}} {
		model := s.EffectiveScopeModel(role.scope).Model
		if model == "" || model == role.scope {
			model = "unavailable"
		}
		color := th.cyan
		if role.scope == "bug" {
			color = th.purple
		} else if role.scope == "function" {
			color = th.green
		}
		lines = append(lines, fmt.Sprintf("  %s%-7s%s %s", color, role.label, th.reset, theme.TruncatePlain(theme.SanitizeSingleLine(model), max(1, width-10))))
	}
	return lines
}

func (s *State) renderSidebarSubPages(th ansiTheme, tab HubTab, width, availableRows int) []string {
	curPage := parentPage(s.EffectivePage())
	var pages []HubSubPage
	selected := 0
	for _, sp := range tab.SubPages {
		if sp.Page == PageIssueDetail {
			continue
		}
		if sp.Page == curPage {
			selected = len(pages)
		}
		pages = append(pages, sp)
	}
	limit := max(1, availableRows)
	more := len(pages) > limit && limit > 1
	if more {
		limit--
	}
	start, end := windowBounds(len(pages), selected, limit)
	var out []string
	for _, sp := range pages[start:end] {
		label := theme.TruncatePlain(sp.Label, width-8)
		if sp.Page == curPage {
			out = append(out, theme.PaintLine(th.selection, th.hubColor(tab.Key), "    ▸ "+label, width))
		} else {
			out = append(out, "      "+th.muted+label+th.reset)
		}
	}
	if more {
		out = append(out, "    "+th.muted+"Tab · more views"+th.reset)
	}
	return out
}

func (s *State) renderStatusBar(th ansiTheme, width int) string {
	left := " " + th.muted + "Local workspace" + th.reset
	if s.Project != nil {
		branch := "Git unavailable"
		if s.Repository != nil && s.Repository.Available {
			branch = client.FirstNonEmpty(s.Repository.Branch, "Detached HEAD")
		}
		left = " " + th.purple + "⎇ " + theme.TruncatePlain(branch, 22) + th.reset
		if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
			left += th.amber + " · Modified" + th.reset
		}
		trust := th.amber + "⚠ Checks need trust"
		if s.WorkspaceTrust {
			trust = th.green + "✔ Checks trusted"
		}
		if width >= 80 {
			left += th.muted + "  ·  " + trust + th.reset
		}
	}
	activity := s.statusBarActivity()
	maxRight := max(10, min(width/2-2, width-theme.VisibleRuneCount(left)-4))
	return alignBar(left, th.cyan+theme.TruncatePlain(activity, maxRight)+th.reset+" ", width)
}

func (s *State) statusBarActivity() string {
	if s.Busy != "" {
		return "⟳ Working · " + s.Busy
	}
	if s.Run != nil && s.Run.Status != "" {
		done, total := runAggregateCoverage(s.Run)
		activity := "◈ Analysis " + s.Run.Status
		if total > 0 {
			activity += fmt.Sprintf(" · %d/%d", done, total)
		}
		return activity
	}
	if count := len(s.AgentItems()); count > 0 {
		return fmt.Sprintf("⚡ %d active task(s)", count)
	}
	if s.Project != nil {
		return "● Ready"
	}
	return ""
}

func (s *State) renderHubBar(th ansiTheme, width int) string {
	active := s.ActiveHub()
	var parts []string
	for _, tab := range HubTabs() {
		label := " " + tab.Key + " " + tab.Label + " "
		if tab.Key == active.Key {
			parts = append(parts, theme.PaintLine(th.selection, th.hubColor(tab.Key), label, theme.VisibleRuneCount(label)))
		} else {
			parts = append(parts, th.hubColor(tab.Key)+label+th.reset)
		}
	}
	return theme.PadVisibleRight(" "+strings.Join(parts, " "), width)
}

func (s *State) renderSubViewBar(th ansiTheme, width int) string {
	hub := s.ActiveHub()
	page := parentPage(s.EffectivePage())
	var labels []string
	active := 0
	for _, sp := range hub.SubPages {
		if sp.Page == PageIssueDetail {
			continue
		}
		label := sp.Label
		if sp.Page == page {
			active = len(labels)
			label = th.hubColor(hub.Key) + "▸ " + label + th.reset
		} else {
			label = th.muted + label + th.reset
		}
		labels = append(labels, label)
	}
	// Center the visible tabs on the current view; Tab always reaches the rest.
	start, end := active, active+1
	for start > 0 && theme.VisibleRuneCount(strings.Join(labels[start-1:end], "  ")) <= width-12 {
		start--
	}
	for end < len(labels) && theme.VisibleRuneCount(strings.Join(labels[start:end+1], "  ")) <= width-12 {
		end++
	}
	prefix, suffix := "  ", ""
	if start > 0 {
		prefix = "‹ "
	}
	if end < len(labels) {
		suffix = " ›"
	}
	return prefix + strings.Join(labels[start:end], "  ") + suffix
}

func (s *State) renderActionBar(th ansiTheme, width int) string {
	if s.Confirmation != nil {
		confirm := th.keyPill("y/Enter", client.FirstNonEmpty(s.Confirmation.AcceptLabel, "Confirm"))
		return alignBar(" "+confirm, th.keyPill("n/Esc", "Cancel")+" ", width)
	}
	items := s.pageActions()
	right := th.muted + "[?] Help  [:] Commands" + th.reset + " "
	if s.CmdActive {
		items = []keyDesc{{"Enter", "Run"}, {"Tab", "Complete"}}
		right = th.keyPill("Esc", "Cancel") + " "
	} else if s.Error != "" && !s.ShowKeybindsModal {
		items = []keyDesc{{"Esc", "Dismiss"}, {"PgUp/PgDn", "Details"}}
	} else if s.ShowKeybindsModal {
		items = []keyDesc{{"↑/↓", "Scroll"}, {"PgUp/PgDn", "Page"}}
		right = th.keyPill("Esc", "Close") + " "
	}
	available := width - theme.VisibleRuneCount(right) - 3
	var hints []string
	for i, item := range items {
		hint := th.keyPill(item.key, item.desc)
		if i == 0 {
			hint = th.accentBg + "[" + item.key + "]" + th.reset + " " + th.fg + item.desc + th.reset
		}
		if theme.VisibleRuneCount(strings.Join(append(hints, hint), "  ")) > available {
			break
		}
		hints = append(hints, hint)
	}
	return alignBar(" "+strings.Join(hints, "  "), right, width)
}

func runAggregateCoverage(run *AnalysisRun) (int, int) {
	if run == nil {
		return 0, 0
	}
	var done, total int
	for _, sec := range run.Sections {
		done += sec.Coverage.Succeeded + sec.Coverage.Partial
		total += sec.Coverage.Total
	}
	if total == 0 && (strings.EqualFold(run.Status, "completed") || strings.EqualFold(run.Status, "completed_empty")) {
		return 1, 1
	}
	return done, total
}

func (s *State) renderConfirmationModal(th ansiTheme, width int) []string {
	c := s.Confirmation
	body := []string{
		fmt.Sprintf("%s%s%s", th.bold+th.fg, c.Title, th.reset),
		c.Message,
		"",
		fmt.Sprintf("Confirm: %s  ·  Cancel: %s",
			th.keyPill("y/Enter", client.FirstNonEmpty(c.AcceptLabel, "Confirm")),
			th.keyPill("n/Esc", "Cancel")),
	}
	return th.renderPanelBox("CONFIRMATION REQUIRED", "PgUp/PgDn: details", min(82, width-6), body)
}

func (s *State) renderCommandBar(th ansiTheme, width int) string {
	if s.CmdActive {
		return " " + th.muted + theme.TruncatePlain("Type to search · Ctrl+P/N: history · Ctrl+U: clear", width-2) + th.reset
	}
	if s.Error != "" {
		return " " + th.red + "Error · " + th.reset + theme.TruncatePlain(s.Error, max(1, width-24)) + th.muted + "  [Esc] Dismiss" + th.reset
	}
	message := client.FirstNonEmpty(s.Busy, s.CmdOutput)
	if message == "Workspace ready" {
		message = "1–5: workspace · Tab: view · Ctrl+K: commands"
		if width >= 76 {
			message = "1–5: workspace · Tab: view · ↑/↓: select · Ctrl+K: commands"
		}
	}
	return " " + th.muted + theme.TruncatePlain(message, width-2) + th.reset
}
