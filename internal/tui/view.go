package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const sidebarMenuWidth = 24

// Render generates a complete, full-screen adaptive ANSI terminal frame for the current TUI state.
func (s *State) Render(width, height int) string {
	frameWidth := theme.ClampWidth(width)
	frameHeight := theme.ClampHeight(height)
	th := themeForPalette(s.Palette)

	if frameWidth < 96 {
		return s.renderCompactFrame(th, frameWidth, frameHeight)
	}
	return s.renderSplitFrame(th, frameWidth, frameHeight)
}

func (s *State) renderSplitFrame(th ansiTheme, frameWidth, frameHeight int) string {
	contentWidth := max(44, frameWidth-2-sidebarMenuWidth-1)

	header := []string{
		s.renderTopBorder(th, frameWidth, frameHeight),
		th.wrapFrameLine(s.renderStatusBar(th, frameWidth), frameWidth),
		s.renderSplitRule(th, frameWidth, sidebarMenuWidth, "┣", "┳", "┫", "━"),
	}

	footer := []string{
		s.renderSplitRule(th, frameWidth, sidebarMenuWidth, "┣", "┻", "┫", "━"),
		th.wrapFrameLine(s.renderCommandBar(th, frameWidth), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}

	targetBodyRows := max(1, frameHeight-len(header)-len(footer))
	s.viewportBodyRows = targetBodyRows
	sidebar := s.renderSidebarMenu(th, sidebarMenuWidth, targetBodyRows)
	rightBody := s.renderWorkspaceContent(th, contentWidth, targetBodyRows)

	body := make([]string, 0, targetBodyRows)
	for i := 0; i < targetBodyRows; i++ {
		var leftLine, rightLine string
		if i < len(sidebar) {
			leftLine = sidebar[i]
		}
		if i < len(rightBody) {
			rightLine = rightBody[i]
		}
		body = append(body, theme.PadVisibleRight(leftLine, sidebarMenuWidth)+
			th.border+"│"+th.reset+
			theme.PadVisibleRight(rightLine, contentWidth))
	}
	return assembleFrame(th, frameWidth, frameHeight, header, body, footer)
}

func (s *State) renderCompactFrame(th ansiTheme, frameWidth, frameHeight int) string {
	header := []string{
		s.renderTopBorder(th, frameWidth, frameHeight),
		th.wrapFrameLine(s.renderStatusBar(th, frameWidth), frameWidth),
		s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"),
		th.wrapFrameLine(s.renderHubBar(th, frameWidth), frameWidth),
	}
	if sub := s.renderSubViewBar(th, frameWidth); sub != "" {
		header = append(header, th.wrapFrameLine(sub, frameWidth))
	}
	header = append(header, s.renderHorizontalRule(th, frameWidth, "┠", "┨", "─"))

	footer := []string{
		s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"),
		th.wrapFrameLine(s.renderCommandBar(th, frameWidth), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}
	targetBodyRows := max(1, frameHeight-len(header)-len(footer))
	s.viewportBodyRows = targetBodyRows
	body := s.renderWorkspaceContent(th, frameWidth-2, targetBodyRows)
	return assembleFrame(th, frameWidth, frameHeight, header, body, footer)
}

func assembleFrame(th ansiTheme, frameWidth, frameHeight int, header, body, footer []string) string {
	targetBodyRows := max(1, frameHeight-len(header)-len(footer))
	lines := make([]string, 0, len(header)+targetBodyRows+len(footer))
	lines = append(lines, header...)
	for i := 0; i < targetBodyRows; i++ {
		var row string
		if i < len(body) {
			row = body[i]
		}
		lines = append(lines, th.wrapFrameLine(row, frameWidth))
	}
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func (s *State) renderWorkspaceContent(th ansiTheme, contentWidth, targetBodyRows int) []string {
	var body []string
	body = append(body, "")
	if s.Error != "" {
		body = append(body, th.renderErrorBanner(s.Error, contentWidth-4)...)
		body = append(body, "")
	}
	switch {
	case s.Confirmation != nil:
		body = append(body, s.renderConfirmationModal(th, contentWidth)...)
	case s.ShowKeybindsModal:
		s.viewportBodyRows = max(1, targetBodyRows-len(body))
		body = append(body, s.renderKeybindsModal(th, contentWidth)...)
	default:
		s.viewportBodyRows = max(1, targetBodyRows-4)
		page := s.renderActivePage(th, contentWidth)
		body = append(body, s.scrollDetails(th, page, max(1, targetBodyRows-len(body)))...)
	}
	return body
}

func (s *State) renderSidebarMenu(th ansiTheme, width, targetBodyRows int) []string {
	active := s.ActiveHub()
	lines := []string{
		"",
		fmt.Sprintf("  %sNAVIGATION%s", th.bold+th.cyan, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
	}
	for _, tab := range HubTabs() {
		isActiveHub := tab.Key == active.Key
		if isActiveHub {
			badge := fmt.Sprintf(" ▸ %s. %-*s", tab.Key, max(1, width-8), theme.TruncatePlain(strings.ToUpper(tab.Label), width-8))
			lines = append(lines, " "+th.accentBg+theme.TruncatePlain(badge, width-2)+th.reset)
			lines = append(lines, s.renderSidebarSubPages(th, tab, width)...)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("   %s%s.%s %s%s%s",
				th.muted, tab.Key, th.reset, th.fg, theme.TruncatePlain(tab.Label, width-7), th.reset))
		}
	}
	models := s.renderSidebarModels(th, width)
	if targetBodyRows-len(lines) >= len(models)+2 {
		models = append([]string{"  " + th.border + strings.Repeat("─", width-4) + th.reset, "  " + th.bold + th.cyan + "MODELS" + th.reset}, models...)
	}
	for len(lines) < targetBodyRows-len(models) {
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
		lines = append(lines, fmt.Sprintf("  %s%-7s%s %s", th.muted, role.label, th.reset, theme.TruncatePlain(theme.SanitizeSingleLine(model), max(1, width-10))))
	}
	return lines
}

func (s *State) renderSidebarSubPages(th ansiTheme, tab HubTab, width int) []string {
	var visible []HubSubPage
	for _, sp := range tab.SubPages {
		if sp.Page != PageIssueDetail {
			visible = append(visible, sp)
		}
	}
	curPage := parentPage(s.EffectivePage())
	var out []string
	for idx, sp := range visible {
		branch := "├─"
		if idx == len(visible)-1 {
			branch = "└─"
		}
		label := theme.TruncatePlain(sp.Label, width-9)
		if sp.Page == curPage {
			out = append(out, fmt.Sprintf("  %s%s%s %s◆ %s%s", th.border, branch, th.reset, th.bold+th.accent, label, th.reset))
		} else {
			out = append(out, fmt.Sprintf("  %s%s%s %s◇ %s%s", th.border, branch, th.reset, th.muted, label, th.reset))
		}
	}
	return out
}

func (s *State) renderTopBorder(th ansiTheme, width, height int) string {
	tag := fmt.Sprintf(" ◆ MINI-ORCA // TUI · %s · %dx%d ", th.name, width, height)
	fill := max(4, width-theme.VisibleRuneCount(tag)-2)
	return fmt.Sprintf("%s┏%s%s%s%s┓%s", th.border, th.accent, tag, th.border, strings.Repeat("━", fill), th.reset)
}

func (s *State) renderBottomBorder(th ansiTheme, width int) string {
	return fmt.Sprintf("%s┗%s┛%s", th.border, strings.Repeat("━", max(10, width-2)), th.reset)
}

func (s *State) renderHorizontalRule(th ansiTheme, width int, left, right, ch string) string {
	return fmt.Sprintf("%s%s%s%s%s", th.border, left, strings.Repeat(ch, max(10, width-2)), right, th.reset)
}

func (s *State) renderSplitRule(th ansiTheme, width, leftPaneWidth int, left, mid, right, ch string) string {
	inner := max(10, width-2)
	rightPaneWidth := max(4, inner-leftPaneWidth-1)
	return fmt.Sprintf("%s%s%s%s%s%s%s",
		th.border,
		left,
		strings.Repeat(ch, leftPaneWidth),
		mid,
		strings.Repeat(ch, rightPaneWidth),
		right,
		th.reset,
	)
}

func (s *State) renderStatusBar(th ansiTheme, width int) string {
	connBadge := th.okBg + " ● ONLINE " + th.reset
	if !s.Connected {
		connBadge = th.warnBg + " ○ OFFLINE " + th.reset
	}
	projName := "no-project"
	if s.Project != nil && s.Project.Name != "" {
		projName = theme.TruncatePlain(s.Project.Name, 26)
	}
	branch := "no branch"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = theme.TruncatePlain(s.Repository.Branch, 20)
	}
	dirty := ""
	if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
		dirty = " " + th.statusBadge("DIRTY")
	}
	ver := ""
	if s.Version != "" {
		ver = "  " + th.muted + "v" + s.Version + th.reset
	}
	content := fmt.Sprintf(
		"  %s%s   %s%s%s   %sgit:%s %s%s%s",
		connBadge, ver,
		th.bold+th.fg, projName, th.reset,
		th.muted, th.reset, th.cyan, branch, dirty,
	)
	if parts := s.renderProgressParts(th); len(parts) > 0 {
		content += th.border + "  │  " + th.reset + strings.Join(parts, th.border+"  │  "+th.reset)
	}
	return theme.PadVisibleRight(content, width-2)
}

func (s *State) renderHubBar(th ansiTheme, width int) string {
	active := s.ActiveHub()
	var parts []string
	for _, tab := range HubTabs() {
		if tab.Key == active.Key {
			parts = append(parts, th.activeTabStyle.Render("▸ "+tab.Label))
		} else {
			parts = append(parts, th.idleTabStyle.Render(tab.Label))
		}
	}
	row := fmt.Sprintf("  %s %s", th.labelStyle.Render("MENU:"), strings.Join(parts, th.border+"│"+th.reset))
	return theme.PadVisibleRight(row, width-2)
}

func (s *State) renderSubViewBar(th ansiTheme, width int) string {
	hub := s.ActiveHub()
	if len(hub.SubPages) == 0 {
		return ""
	}
	curPage := parentPage(s.EffectivePage())
	var parts []string
	for _, sp := range hub.SubPages {
		if sp.Page == PageIssueDetail && curPage != PageIssueDetail {
			continue
		}
		if sp.Page == curPage {
			parts = append(parts, th.subActiveStyle.Render("◆ "+sp.Label))
		} else {
			parts = append(parts, th.subIdleStyle.Render("◇ "+sp.Label))
		}
	}
	row := fmt.Sprintf("  %s %s", th.labelStyle.Render("VIEW:"), strings.Join(parts, th.border+"·"+th.reset))
	return theme.PadVisibleRight(row, width-2)
}

func (s *State) renderProgressParts(th ansiTheme) []string {
	var parts []string
	if s.Run != nil && s.Run.Status != "" {
		done, total := runAggregateCoverage(s.Run)
		bar := th.renderProgressBar(done, total, 8)
		runText := fmt.Sprintf("Analysis %s %s (%ds)", th.statusBadge(s.Run.Status), bar, s.Run.ElapsedSeconds)
		if s.isAnalysisPausedOrInterrupted() {
			reason := theme.TruncatePlain(client.FirstNonEmpty(s.Run.Reason, "window paused"), 28)
			runText += fmt.Sprintf(" %s%s · [r] Resume%s", th.amber, reason, th.reset)
		}
		parts = append(parts, runText)
	}
	if s.Busy != "" {
		parts = append(parts, th.warnBg+" ⟳ BUSY: "+theme.TruncatePlain(s.Busy, 22)+" "+th.reset)
	}
	return parts
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
	return th.renderPanelBox("CONFIRMATION REQUIRED", "SECURITY & GOVERNANCE GATE", min(82, width-6), body)
}

func (s *State) renderCommandBar(th ansiTheme, width int) string {
	if s.CmdActive {
		return th.renderCommandInput(s.CmdInput)
	}
	if s.Error != "" {
		return fmt.Sprintf(" %s ▲ ERROR %s  %s%s%s",
			th.critBg, th.reset, th.bold+th.red, s.Error, th.reset)
	}
	badge := th.okBg + " ● READY " + th.reset
	if s.Busy != "" {
		badge = th.warnBg + " ⟳ BUSY " + th.reset
	}
	left := fmt.Sprintf(" %s  %s%s%s", badge, th.fg, s.CmdOutput, th.reset)
	right := fmt.Sprintf("%s[?] Keys  [:] Cmd%s ", th.muted, th.reset)
	gap := max(2, width-2-theme.VisibleRuneCount(left)-theme.VisibleRuneCount(right))
	return left + strings.Repeat(" ", gap) + right
}
