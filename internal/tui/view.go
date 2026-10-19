package tui

import (
	"fmt"
	"strings"
)

// Render generates a complete, full-screen adaptive ANSI terminal frame for the current TUI state.
func (s *State) Render(width, height int) string {
	frameWidth := clampWidth(width)
	frameHeight := clampHeight(height)
	th := themeForPalette(s.Palette)

	var header []string
	header = append(header, s.renderTopBorder(th, frameWidth, frameHeight))
	header = append(header, th.wrapFrameLine(s.renderStatusBar(th, frameWidth), frameWidth))
	header = append(header, s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"))
	header = append(header, th.wrapFrameLine(s.renderHubBar(th, frameWidth), frameWidth))
	if sub := s.renderSubViewBar(th, frameWidth); sub != "" {
		header = append(header, th.wrapFrameLine(sub, frameWidth))
	}
	header = append(header, th.wrapFrameLine(s.renderActionToolbar(th, frameWidth), frameWidth))
	if prog := s.renderProgressStrip(th, frameWidth); prog != "" {
		header = append(header, th.wrapFrameLine(prog, frameWidth))
	}
	header = append(header, s.renderHorizontalRule(th, frameWidth, "┠", "┨", "─"))

	var body []string
	if s.Error != "" {
		body = append(body, th.renderErrorBanner(s.Error, frameWidth-6)...)
	}
	if s.Confirmation != nil {
		body = append(body, s.renderConfirmationModal(th, frameWidth)...)
	} else if s.ShowKeybindsModal {
		body = append(body, s.renderKeybindsModal(th, frameWidth)...)
	} else {
		body = append(body, s.renderActivePage(th, frameWidth)...)
	}

	footer := []string{
		s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"),
		th.wrapFrameLine(s.renderShortcutHintBar(th), frameWidth),
		th.wrapFrameLine(s.renderCommandBar(th), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}

	targetBodyRows := max(len(body), frameHeight-len(header)-len(footer))
	var lines []string
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
		"  %sMINI-ORCA // TUI%s %s [%s %s%s%s]  %sgit:%s%s%s  %strust:%s%s  %spalette:%s%s%s  %s[?] Keys  [:] Cmd%s",
		th.accent, th.reset, ver, connBadge, th.bold, projName, th.reset,
		th.muted, th.fg, branch, dirty,
		th.muted, trust, th.reset,
		th.muted, th.purple, s.Palette, th.reset,
		th.cyan, th.reset,
	)
	return padVisibleRight(content, width-2)
}

func (s *State) renderHubBar(th ansiTheme, width int) string {
	active := s.ActiveHub()
	var parts []string
	for _, tab := range HubTabs() {
		label := fmt.Sprintf("[%s] %s", tab.Key, tab.Label)
		if tab.Key == active.Key {
			parts = append(parts, th.infoBg+" ▸ "+label+" "+th.reset)
		} else {
			parts = append(parts, th.muted+" "+label+" "+th.reset)
		}
	}
	row := fmt.Sprintf("  %sHUBS:%s %s", th.bold+th.muted, th.reset, strings.Join(parts, th.border+" │ "+th.reset))
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
			parts = append(parts, th.selBg+th.accent+" ◆ ["+sp.Label+"] "+th.reset)
		} else {
			parts = append(parts, th.muted+" ◇ "+sp.Label+" "+th.reset)
		}
	}
	row := fmt.Sprintf("  %sVIEW [Tab]:%s %s", th.bold+th.muted, th.reset, strings.Join(parts, th.border+" · "+th.reset))
	return padVisibleRight(row, width-2)
}

func (s *State) renderActionToolbar(th ansiTheme, width int) string {
	var actions []string
	switch s.Page {
	case PageSummary, PageBugs:
		actions = []string{
			th.okBg + " [Enter / a] FIX SELECTED BUG WITH AGENT " + th.reset,
			th.keyPill("x/Space", "Batch Select"),
			th.keyPill("r", "Run Analysis"),
			th.keyPill("t", "Filter/Test"),
			th.keyPill(":agent <goal>", "New Feature/Fix"),
		}
	case PageFeatures:
		actions = []string{
			th.okBg + " [Enter / a] BUILD FEATURE WITH AGENT " + th.reset,
			th.keyPill(":agent <spec>", "Custom Feature"),
			th.keyPill("d", "Dismiss Idea"),
			th.keyPill("r", "Generate Features"),
		}
	case PageAgents:
		actions = []string{
			th.okBg + " [c] APPLY & COMMIT FIX " + th.reset,
			th.keyPill("r", "Repair / Re-check"),
			th.keyPill("m", "Merge Worktree"),
			th.keyPill("u", "Guarded Undo"),
			th.keyPill("Tab", "Browse Bugs & Features"),
		}
	case PageEditor:
		actions = []string{
			th.okBg + " [c] APPLY & COMMIT DIFF " + th.reset,
			th.keyPill("x", "Reject Hunk"),
			th.keyPill("e", "Toggle Why"),
			th.keyPill("u", "Guarded Undo"),
			th.keyPill(":open <file>", "Open File"),
		}
	default:
		actions = []string{
			th.keyPill("Enter", "Execute Primary Action"),
			th.keyPill("r", "Run/Refresh Analysis"),
			th.keyPill("h/l", "Switch Section"),
			th.keyPill(":agent <goal>", "Start Agent"),
		}
	}
	row := fmt.Sprintf("  %sACTIONS:%s %s", th.bold+th.amber, th.reset, strings.Join(actions, "   "))
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
		parts = append(parts, fmt.Sprintf("Active Agent Proposal: %s%s%s %s (%s+%d%s/%s-%d%s)",
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
		fmt.Sprintf("%s%s%s", th.bold, c.Title, th.reset),
		c.Message,
		"",
		fmt.Sprintf("Press %s[y / Enter] %s%s   or   %s[n / Esc] Cancel%s",
			th.green, firstNonEmpty(c.AcceptLabel, "Confirm"), th.reset, th.amber, th.reset),
	}
	return th.renderPanelBox("CONFIRMATION REQUIRED", "SECURITY & GOVERNANCE GATE", min(88, width-6), body)
}

func (s *State) renderKeybindsModal(th ansiTheme, width int) []string {
	body := []string{
		fmt.Sprintf("%sNAVIGATION%s      [1-5] Switch Hub      [Tab / Shift+Tab] Sub-view   [j/k] Move Row", th.cyan, th.reset),
		fmt.Sprintf("%sBUG FIXING%s      [Enter / a] Start Agent Fix on Selected Bug        [x/Space] Batch Select", th.green, th.reset),
		fmt.Sprintf("%sNEW FEATURES%s    [2] -> [Tab] Features -> [Enter / a] Build         [:agent <prompt>] Custom", th.green, th.reset),
		fmt.Sprintf("%sGUARDED CHANGE%s  [c] Apply & Commit    [u] Guarded Undo             [m] Merge Worktree", th.amber, th.reset),
		fmt.Sprintf("%sINSPECTION%s      [r] Run/Repair        [e] Toggle Diff Why          [p] Cycle Palette", th.cyan, th.reset),
		fmt.Sprintf("%sCOLON COMMANDS%s  :analyze :test :fix :agent :open :project :apply :undo :palette :q", th.purple, th.reset),
	}
	return th.renderPanelBox("KEYBINDINGS & COLON COMMANDS REFERENCE", "[?] or Esc to close", min(94, width-6), body)
}

func (s *State) renderShortcutHintBar(th ansiTheme) string {
	return fmt.Sprintf(
		"  %sWORKFLOW%s  %s  │  %s  │  %s  │  %s  │  %s  │  %s",
		th.bold+th.cyan, th.reset,
		th.keyPill("1-5", "Hub"),
		th.keyPill("Tab", "Sub-view"),
		th.keyPill("j/k", "Select"),
		th.keyPill("Enter/a", "Fix Bug / Build Feature"),
		th.keyPill("r", "Analyze"),
		th.keyPill(":q", "Quit"),
	)
}

func (s *State) renderCommandBar(th ansiTheme) string {
	if s.CmdActive {
		return th.renderCommandInput(s.CmdInput)
	}
	if s.Error != "" {
		return fmt.Sprintf("  %s ▲ ERROR %s %s%s%s   %s([Esc] Clear Error · [:reconnect] Retry)%s",
			th.critBg, th.reset, th.red, s.Error, th.reset, th.muted, th.reset)
	}
	return fmt.Sprintf("  %sSTATUS ▸%s %s", th.green, th.reset, s.CmdOutput)
}
