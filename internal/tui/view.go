package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const sidebarMenuWidth = 26

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
	}
	if prog := s.renderProgressStrip(th, frameWidth); prog != "" {
		header = append(header, th.wrapFrameLine(prog, frameWidth))
	}
	header = append(header, s.renderSplitRule(th, frameWidth, sidebarMenuWidth, "┣", "┳", "┫", "━"))

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
	if prog := s.renderProgressStrip(th, frameWidth); prog != "" {
		header = append(header, th.wrapFrameLine(prog, frameWidth))
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
		body = append(body, s.renderActivePage(th, contentWidth)...)
		body = s.appendBottomFooterDock(th, contentWidth, targetBodyRows, body)
	}
	return body
}

func (s *State) appendBottomFooterDock(th ansiTheme, contentWidth, targetBodyRows int, body []string) []string {
	rem := targetBodyRows - len(body)
	if rem < 3 {
		return body
	}
	panelW := adaptivePanelWidth(contentWidth)
	dockBox := s.renderWorkspaceFooter(th, panelW, rem >= 4)
	if len(dockBox) > rem {
		dockBox = s.renderWorkspaceFooter(th, panelW, false)
	}
	if len(dockBox) > rem {
		return body
	}
	padRows := max(0, targetBodyRows-len(body)-len(dockBox))
	for i := 0; i < padRows; i++ {
		body = append(body, "")
	}
	return append(body, dockBox...)
}

func (s *State) renderWorkspaceFooter(th ansiTheme, panelW int, twoLines bool) []string {
	totalFiles := s.indexedFileCount()
	inclFiles := max(0, totalFiles-len(s.ExcludedPaths))
	runStatus := "IDLE"
	if s.Run != nil && s.Run.Status != "" {
		runStatus = s.Run.Status
	}
	chgStatus := "IDLE"
	if s.HasActiveProposal() && s.Change.State != "" {
		chgStatus = s.Change.State
	}
	trust := th.statusBadge("TRUSTED")
	if !s.WorkspaceTrust {
		trust = th.statusBadge("UNTRUSTED")
	}
	if !twoLines {
		line := fmt.Sprintf("%sAnalysis:%s %s %d/%d Files · %sAgents:%s %s · %strust:%s %s · %spalette:%s %s%s%s",
			th.muted, th.reset, th.statusBadge(runStatus), inclFiles, totalFiles,
			th.muted, th.reset, th.statusBadge(chgStatus),
			th.muted, th.reset, trust,
			th.muted, th.reset, th.purple, s.Palette, th.reset)
		return th.renderPanelBox("", "", panelW, []string{line})
	}
	lines := []string{
		fmt.Sprintf("%sAnalysis:%s %s %d/%d Files · %sAgents:%s %s",
			th.muted, th.reset, th.statusBadge(runStatus), inclFiles, totalFiles,
			th.muted, th.reset, th.statusBadge(chgStatus)),
		fmt.Sprintf("%sProposals:%s %d · %sLessons:%s %d · %strust:%s %s · %spalette:%s %s%s%s",
			th.muted, th.reset, len(s.ChangeHistory),
			th.muted, th.reset, len(s.GraduatedLessons),
			th.muted, th.reset, trust,
			th.muted, th.reset, th.purple, s.Palette, th.reset),
	}
	return th.renderPanelBox("", "", panelW, lines)
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
			badge := fmt.Sprintf(" ▸ %s. %-18s", tab.Key, strings.ToUpper(tab.Label))
			lines = append(lines, " "+th.accentBg+theme.TruncatePlain(badge, width-2)+th.reset)
			lines = append(lines, s.renderSidebarSubPages(th, tab, width)...)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("   %s%s.%s %s%s%s",
				th.muted, tab.Key, th.reset, th.fg, theme.TruncatePlain(tab.Label, width-7), th.reset))
		}
	}
	if active.Key != "5" {
		lines = append(lines, "")
	}
	lines = append(lines, "")
	lines = append(lines, s.renderSidebarPulse(th, width)...)
	footer := s.renderSidebarFooter(th, width)
	if targetBodyRows-len(lines)-len(footer) >= 8 {
		lines = append(lines, "")
		lines = append(lines, s.renderSidebarRouting(th, width)...)
	}
	if rem := targetBodyRows - len(lines); rem >= len(footer) {
		padRows := max(0, targetBodyRows-len(lines)-len(footer))
		for i := 0; i < padRows; i++ {
			lines = append(lines, "")
		}
		lines = append(lines, footer...)
	}
	return lines
}

func (s *State) renderSidebarRouting(th ansiTheme, width int) []string {
	preset := theme.TruncatePlain(client.FirstNonEmpty(s.ActivePresetID, "local-default"), max(8, width-13))
	aMod := theme.TruncatePlain(client.FirstNonEmpty(s.EffectiveScopeModel("analyze").Model, s.AssignedScopeModelID("analyze")), max(8, width-13))
	bMod := theme.TruncatePlain(client.FirstNonEmpty(s.EffectiveScopeModel("bug").Model, s.AssignedScopeModelID("bug")), max(8, width-13))
	fMod := theme.TruncatePlain(client.FirstNonEmpty(s.EffectiveScopeModel("function").Model, s.AssignedScopeModelID("function")), max(8, width-13))
	return []string{
		fmt.Sprintf("  %sMODEL ROUTING%s", th.bold+th.cyan, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
		fmt.Sprintf("  %s%-10s%s %s%s%s", th.muted, "Preset", th.reset, th.green, preset, th.reset),
		fmt.Sprintf("  %s%-10s%s %s%s%s", th.muted, "Analyze", th.reset, th.fg, aMod, th.reset),
		fmt.Sprintf("  %s%-10s%s %s%s%s", th.muted, "Bug", th.reset, th.fg, bMod, th.reset),
		fmt.Sprintf("  %s%-10s%s %s%s%s", th.muted, "Func", th.reset, th.fg, fMod, th.reset),
	}
}

func (s *State) renderSidebarFooter(th ansiTheme, width int) []string {
	cfg := s.EffectiveAnalysisConfig()
	mins := max(1, (cfg.BudgetSeconds+59)/60)
	return []string{
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
		fmt.Sprintf("  %s%-10s%s %s%d files · %dm%s", th.muted, "Batch", th.reset, th.cyan, cfg.BatchFiles, mins, th.reset),
		fmt.Sprintf("  %s%-10s%s %s%d / stage%s", th.muted, "Attempts", th.reset, th.cyan, cfg.MaxAttemptsPerStage, th.reset),
	}
}

func (s *State) renderSidebarSubPages(th ansiTheme, tab HubTab, width int) []string {
	var visible []HubSubPage
	for _, sp := range tab.SubPages {
		if sp.Page != PageIssueDetail {
			visible = append(visible, sp)
		}
	}
	curPage := s.EffectivePage()
	if curPage == PageIssueDetail {
		curPage = PageBugs
	}
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

func (s *State) renderSidebarPulse(th ansiTheme, width int) []string {
	summary := domain.ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	openVal := fmt.Sprintf("%s0%s", th.green, th.reset)
	if summary.TotalOpen > 0 {
		openVal = fmt.Sprintf("%s%d open%s", th.bold+th.amber, summary.TotalOpen, th.reset)
	}
	agentState := "IDLE"
	if s.HasActiveProposal() && s.Change.State != "" {
		agentState = s.Change.State
	}
	return []string{
		fmt.Sprintf("  %sWORKSPACE PULSE%s", th.bold+th.cyan, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
		fmt.Sprintf("  %s%-10s%s %s", th.muted, "Findings", th.reset, openVal),
		fmt.Sprintf("  %s%-10s%s %s%d%s", th.muted, "Resolved", th.reset, th.green, summary.Done, th.reset),
		fmt.Sprintf("  %s%-10s%s %s", th.muted, "Agent", th.reset, th.statusBadge(agentState)),
		fmt.Sprintf("  %s%-10s%s %s%d%s", th.muted, "Files", th.reset, th.fg, s.indexedFileCount(), th.reset),
	}
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
	branch := "main"
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
	curPage := s.EffectivePage()
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

func (s *State) renderProgressStrip(th ansiTheme, width int) string {
	var parts []string
	if s.Run != nil && s.Run.Status != "" {
		done, total := runAggregateCoverage(s.Run)
		bar := th.renderProgressBar(done, total, 12)
		runText := fmt.Sprintf("Analysis %s %s (%ds)", th.statusBadge(s.Run.Status), bar, s.Run.ElapsedSeconds)
		if s.isAnalysisPausedOrInterrupted() {
			reason := client.FirstNonEmpty(s.Run.Reason, "window paused")
			runText += fmt.Sprintf(" %s%s · [r] Resume%s", th.amber, reason, th.reset)
		}
		parts = append(parts, runText)
	}
	if s.HasActiveProposal() {
		added, removed, badge := domain.ClassifyDiffScope(s.Change.Changes)
		parts = append(parts, fmt.Sprintf("Active Proposal: %s%s%s %s (%s+%d%s/%s-%d%s)",
			th.bold, s.Change.Title, th.reset,
			th.statusBadge(badge), th.green, added, th.reset, th.red, removed, th.reset,
		))
	}
	if s.Busy != "" {
		parts = append(parts, th.warnBg+" ⟳ BUSY: "+s.Busy+" "+th.reset)
	}
	if len(parts) == 0 {
		return ""
	}
	row := "  " + strings.Join(parts, th.border+"  │  "+th.reset)
	return theme.PadVisibleRight(row, width-2)
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
