package tui

import (
	"fmt"
	"strings"
)

const sidebarMenuWidth = 26

// Render generates a complete, full-screen adaptive ANSI terminal frame for the current TUI state.
func (s *State) Render(width, height int) string {
	frameWidth := clampWidth(width)
	frameHeight := clampHeight(height)
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

	sidebar := s.renderSidebarMenu(th, sidebarMenuWidth)
	rightBody := s.renderWorkspaceContent(th, contentWidth)

	footer := []string{
		s.renderSplitRule(th, frameWidth, sidebarMenuWidth, "┣", "┻", "┫", "━"),
		th.wrapFrameLine(s.renderCommandBar(th, frameWidth), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}

	targetBodyRows := max(max(len(sidebar), len(rightBody)), frameHeight-len(header)-len(footer))
	lines := make([]string, 0, len(header)+targetBodyRows+len(footer))
	lines = append(lines, header...)
	for i := 0; i < targetBodyRows; i++ {
		var leftLine, rightLine string
		if i < len(sidebar) {
			leftLine = sidebar[i]
		}
		if i < len(rightBody) {
			rightLine = rightBody[i]
		}
		row := padVisibleRight(leftLine, sidebarMenuWidth) +
			th.border + "│" + th.reset +
			padVisibleRight(rightLine, contentWidth)
		lines = append(lines, th.wrapFrameLine(row, frameWidth))
	}
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
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

	body := s.renderWorkspaceContent(th, frameWidth-2)

	footer := []string{
		s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"),
		th.wrapFrameLine(s.renderCommandBar(th, frameWidth), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}

	targetBodyRows := max(len(body), frameHeight-len(header)-len(footer))
	lines := make([]string, 0, len(header)+targetBodyRows+len(footer))
	lines = append(lines, header...)
	for i := 0; i < targetBodyRows; i++ {
		if i < len(body) {
			lines = append(lines, th.wrapFrameLine(body[i], frameWidth))
		} else {
			lines = append(lines, th.wrapFrameLine("", frameWidth))
		}
	}
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func (s *State) renderWorkspaceContent(th ansiTheme, contentWidth int) []string {
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
		body = append(body, s.renderKeybindsModal(th, contentWidth)...)
	default:
		body = append(body, s.renderActivePage(th, contentWidth)...)
	}
	return body
}

func (s *State) renderSidebarMenu(th ansiTheme, width int) []string {
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
			lines = append(lines, " "+th.accentBg+truncatePlain(badge, width-2)+th.reset)
			lines = append(lines, s.renderSidebarSubPages(th, tab, width)...)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("   %s%s.%s %s%s%s",
				th.muted, tab.Key, th.reset, th.fg, truncatePlain(tab.Label, width-7), th.reset))
		}
	}
	lines = append(lines, s.renderSidebarPulse(th, width)...)
	return lines
}

func (s *State) renderSidebarSubPages(th ansiTheme, tab HubTab, width int) []string {
	var visible []HubSubPage
	for _, sp := range tab.SubPages {
		if sp.Page == PageIssueDetail && s.Page != PageIssueDetail {
			continue
		}
		visible = append(visible, sp)
	}
	var out []string
	for idx, sp := range visible {
		branch := "├─"
		if idx == len(visible)-1 {
			branch = "└─"
		}
		label := truncatePlain(sp.Label, width-9)
		if sp.Page == s.Page {
			out = append(out, fmt.Sprintf("  %s%s%s %s◆ %s%s", th.border, branch, th.reset, th.bold+th.accent, label, th.reset))
		} else {
			out = append(out, fmt.Sprintf("  %s%s%s %s◇ %s%s", th.border, branch, th.reset, th.muted, label, th.reset))
		}
	}
	return out
}

func (s *State) renderSidebarPulse(th ansiTheme, width int) []string {
	summary := ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	openVal := fmt.Sprintf("%s0%s", th.green, th.reset)
	if summary.TotalOpen > 0 {
		openVal = fmt.Sprintf("%s%d open%s", th.bold+th.amber, summary.TotalOpen, th.reset)
	}
	agentState := "IDLE"
	if s.Change != nil && s.Change.State != "" {
		agentState = s.Change.State
	}
	return []string{
		fmt.Sprintf("  %sWORKSPACE PULSE%s", th.bold+th.cyan, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
		fmt.Sprintf("  %sFindings%s   %s", th.muted, th.reset, openVal),
		fmt.Sprintf("  %sResolved%s   %s%d%s", th.muted, th.reset, th.green, summary.Done, th.reset),
		fmt.Sprintf("  %sAgent%s      %s", th.muted, th.reset, th.statusBadge(agentState)),
		fmt.Sprintf("  %sFiles%s      %s%d%s", th.muted, th.reset, th.fg, s.indexedFileCount(), th.reset),
	}
}

func (s *State) renderTopBorder(th ansiTheme, width, height int) string {
	tag := fmt.Sprintf(" ◆ MINI-ORCA // TUI · %s · %dx%d ", th.name, width, height)
	fill := max(4, width-visibleRuneCount(tag)-2)
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
		projName = truncatePlain(s.Project.Name, 26)
	}
	branch := "main"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = truncatePlain(s.Repository.Branch, 20)
	}
	dirty := ""
	if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
		dirty = " " + th.statusBadge("DIRTY")
	}
	trust := th.statusBadge("TRUSTED")
	if !s.WorkspaceTrust {
		trust = th.statusBadge("UNTRUSTED")
	}
	ver := ""
	if s.Version != "" {
		ver = th.muted + "v" + s.Version + " " + th.reset
	}
	content := fmt.Sprintf(
		"  %sMINI-ORCA // TUI%s %s %s  %s%s%s   %sgit:%s %s%s%s   %strust:%s %s   %spalette:%s %s%s%s",
		th.bold+th.accent, th.reset, ver, connBadge,
		th.bold+th.fg, projName, th.reset,
		th.muted, th.reset, th.cyan, branch, dirty,
		th.muted, th.reset, trust,
		th.muted, th.reset, th.purple, s.Palette, th.reset,
	)
	return padVisibleRight(content, width-2)
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
	return padVisibleRight(row, width-2)
}

func (s *State) renderSubViewBar(th ansiTheme, width int) string {
	hub := s.ActiveHub()
	if len(hub.SubPages) == 0 {
		return ""
	}
	var parts []string
	for _, sp := range hub.SubPages {
		if sp.Page == s.Page {
			parts = append(parts, th.subActiveStyle.Render("◆ "+sp.Label))
		} else {
			parts = append(parts, th.subIdleStyle.Render("◇ "+sp.Label))
		}
	}
	row := fmt.Sprintf("  %s %s", th.labelStyle.Render("VIEW:"), strings.Join(parts, th.border+"·"+th.reset))
	return padVisibleRight(row, width-2)
}

func (s *State) renderProgressStrip(th ansiTheme, width int) string {
	var parts []string
	if s.Run != nil && s.Run.Status != "" {
		done, total := runAggregateCoverage(s.Run)
		bar := th.renderProgressBar(done, total, 12)
		parts = append(parts, fmt.Sprintf("Analysis %s %s (%ds)", th.statusBadge(s.Run.Status), bar, s.Run.ElapsedSeconds))
	}
	if s.Change != nil {
		added, removed, badge := ClassifyDiffScope(s.Change.Changes)
		parts = append(parts, fmt.Sprintf("Active Proposal: %s%s%s %s (%s+%d%s/%s-%d%s)",
			th.bold, truncatePlain(s.Change.Title, 28), th.reset,
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
	return padVisibleRight(row, width-2)
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
	if total == 0 && strings.EqualFold(run.Status, "completed") {
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
			th.keyPill("y/Enter", firstNonEmpty(c.AcceptLabel, "Confirm")),
			th.keyPill("n/Esc", "Cancel")),
	}
	return th.renderPanelBox("CONFIRMATION REQUIRED", "SECURITY & GOVERNANCE GATE", min(82, width-6), body)
}

func (s *State) renderKeybindsModal(th ansiTheme, width int) []string {
	body := []string{
		fmt.Sprintf("%sNAVIGATION%s      %s  %s  %s",
			th.bold+th.cyan, th.reset,
			th.keyPill("1-5", "Section"),
			th.keyPill("Tab", "Sub-page"),
			th.keyPill("j/k", "Select Row")),
		fmt.Sprintf("%sBUG FIXING%s      %s  %s  %s",
			th.bold+th.green, th.reset,
			th.keyPill("Enter/a", "Fix Selected Bug"),
			th.keyPill("i", "Inspect"),
			th.keyPill("x/Space", "Batch")),
		fmt.Sprintf("%sNEW FEATURES%s    %s  %s  %s",
			th.bold+th.green, th.reset,
			th.keyPill("f", "Features"),
			th.keyPill("Enter/a", "Build Feature"),
			th.keyPill("d", "Dismiss")),
		fmt.Sprintf("%sGUARDED CHANGE%s  %s  %s  %s",
			th.bold+th.amber, th.reset,
			th.keyPill("c", "Apply & Commit"),
			th.keyPill("u", "Guarded Undo"),
			th.keyPill("m", "Merge Worktree")),
		fmt.Sprintf("%sINSPECTION%s      %s  %s  %s",
			th.bold+th.cyan, th.reset,
			th.keyPill("r", "Run/Repair"),
			th.keyPill("e", "Toggle Why"),
			th.keyPill("p", "Cycle Palette")),
		fmt.Sprintf("%sCOLON COMMANDS%s  %s:analyze :test :fix :agent :open :project :apply :undo :palette :q%s",
			th.bold+th.purple, th.reset, th.fg, th.reset),
	}
	return th.renderPanelBox("KEYBINDINGS & COLON COMMANDS REFERENCE", "HELP OVERLAY", min(88, width-6), body)
}

func (s *State) activeBreadcrumb() string {
	hub := s.ActiveHub()
	for _, sp := range hub.SubPages {
		if sp.Page == s.Page {
			return fmt.Sprintf("%s / %s", strings.ToUpper(hub.Label), sp.Label)
		}
	}
	return strings.ToUpper(hub.Label)
}

func (s *State) renderCommandBar(th ansiTheme, width int) string {
	if s.CmdActive {
		return th.renderCommandInput(s.CmdInput)
	}
	if s.Error != "" {
		return fmt.Sprintf(" %s ▲ ERROR %s  %s%s%s",
			th.critBg, th.reset, th.bold+th.red, truncatePlain(s.Error, max(20, width-16)), th.reset)
	}
	badge := th.okBg + " ● READY " + th.reset
	if s.Busy != "" {
		badge = th.warnBg + " ⟳ BUSY " + th.reset
	}
	left := fmt.Sprintf(" %s  %s%s%s", badge, th.fg, truncatePlain(s.CmdOutput, max(20, width-38)), th.reset)
	right := fmt.Sprintf("%s%s%s ", th.muted, s.activeBreadcrumb(), th.reset)
	gap := max(2, width-2-visibleRuneCount(left)-visibleRuneCount(right))
	return left + strings.Repeat(" ", gap) + right
}
