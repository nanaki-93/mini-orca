package tui

import (
	"fmt"
	"strings"
)

// Render generates a complete ANSI terminal frame for the current TUI state.
func (s *State) Render(width, height int) string {
	frameWidth := clampWidth(width)
	if height <= 0 {
		height = 32
	}
	th := themeForPalette(s.Palette)
	var lines []string
	lines = append(lines, s.renderTopBorder(th, frameWidth))
	lines = append(lines, s.renderStatusBar(th, frameWidth))
	lines = append(lines, s.renderHubBar(th, frameWidth))
	if sub := s.renderSubViewBar(th, frameWidth); sub != "" {
		lines = append(lines, sub)
	}
	if prog := s.renderProgressStrip(th, frameWidth); prog != "" {
		lines = append(lines, prog)
	}
	lines = append(lines, s.renderHorizontalRule(th, frameWidth, "├", "┤", "─"))

	if s.Confirmation != nil {
		lines = append(lines, s.renderConfirmationModal(th, frameWidth)...)
	} else if s.ShowKeybindsModal {
		lines = append(lines, s.renderKeybindsModal(th, frameWidth)...)
	} else {
		lines = append(lines, s.renderActivePage(th, frameWidth)...)
	}

	lines = append(lines, s.renderHorizontalRule(th, frameWidth, "├", "┤", "─"))
	lines = append(lines, s.renderShortcutHintBar(th))
	lines = append(lines, s.renderCommandBar(th))
	lines = append(lines, s.renderBottomBorder(th, frameWidth))
	return strings.Join(lines, "\n")
}

func (s *State) renderTopBorder(th ansiTheme, width int) string {
	tag := fmt.Sprintf(" ◆ MINI-ORCA // TUI · %s ", th.name)
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
	connBadge := th.green + "● ONLINE" + th.reset
	if !s.Connected {
		connBadge = th.amber + "○ OFFLINE" + th.reset
	}
	projName := "no-project"
	if s.Project != nil && s.Project.Name != "" {
		projName = truncatePlain(s.Project.Name, 24)
	}
	branch := "main"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = truncatePlain(s.Repository.Branch, 20)
	}
	dirty := ""
	if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
		dirty = th.amber + "*dirty" + th.reset
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
		"  %sMINI-ORCA // TUI%s %s [%s %s%s%s]  %sgit:%s%s%s  %strust:%s%s  %spalette:%s%s%s%s  %s[?] Keys  [:] Cmd%s",
		th.accent, th.reset, ver, connBadge, th.bold, projName, th.reset,
		th.muted, th.fg, branch, dirty,
		th.muted, trust, th.reset,
		th.muted, th.purple, s.Palette, th.reset, "",
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
			parts = append(parts, th.selBg+th.accent+" ▸ "+label+" "+th.reset)
		} else {
			parts = append(parts, th.muted+" "+label+" "+th.reset)
		}
	}
	row := "  " + strings.Join(parts, th.border+"│"+th.reset)
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
			parts = append(parts, th.accent+"◆ ["+sp.Label+"]"+th.reset)
		} else {
			parts = append(parts, th.muted+"◇ "+sp.Label+th.reset)
		}
	}
	row := fmt.Sprintf("  %sSub-views (Tab / Shift+Tab):%s  %s", th.muted, th.reset, strings.Join(parts, th.border+"  ·  "+th.reset))
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
		parts = append(parts, fmt.Sprintf("Proposal: %s%s%s %s (%s+%d%s/%s-%d%s)",
			th.bold, truncatePlain(s.Change.Title, 28), th.reset,
			th.statusBadge(badge), th.green, added, th.reset, th.red, removed, th.reset,
		))
	}
	if s.Busy != "" {
		parts = append(parts, th.amber+"⟳ Busy: "+s.Busy+th.reset)
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
	return th.renderPanelBox("CONFIRMATION REQUIRED", "SECURITY & GOVERNANCE GATE", min(84, width-6), body)
}

func (s *State) renderKeybindsModal(th ansiTheme, width int) []string {
	body := []string{
		fmt.Sprintf("%sNAVIGATION%s      [1-5] Switch Hub      [Tab / Shift+Tab] Sub-view   [j/k] Move Row", th.cyan, th.reset),
		fmt.Sprintf("%sTRIAGE & SCOPE%s  [Enter] Select/Open   [t/c/f] Issue Category       [s/v] Issue Status", th.cyan, th.reset),
		fmt.Sprintf("%sAGENT MISSIONS%s  [x/Space] Select/Excl [a] Dispatch to Agent        [d] Dismiss Item", th.cyan, th.reset),
		fmt.Sprintf("%sGUARDED CHANGE%s  [c] Apply & Commit    [u] Guarded Undo             [m] Merge Worktree", th.cyan, th.reset),
		fmt.Sprintf("%sINSPECTION%s      [r] Run/Repair        [e] Toggle Diff Why          [p] Cycle Palette", th.cyan, th.reset),
		fmt.Sprintf("%sCOLON COMMANDS%s  :analyze :test :fix :agent :open :project :apply :undo :palette :q", th.purple, th.reset),
	}
	return th.renderPanelBox("KEYBINDINGS & COLON COMMANDS REFERENCE", "[?] or Esc to close", min(92, width-6), body)
}

func (s *State) renderShortcutHintBar(th ansiTheme) string {
	return fmt.Sprintf(
		"  %sKEYS%s  [1-5] Hub  [Tab] Sub-view  [j/k] Navigate  [Enter] Action  [a] Agent  [r] Analyze  [p] Theme  [?] Help  [:q] Quit",
		th.cyan, th.reset,
	)
}

func (s *State) renderCommandBar(th ansiTheme) string {
	if s.CmdActive {
		return fmt.Sprintf("  %sCOMMAND ▸ :%s%s█   %s(Enter to execute · Esc to cancel)%s", th.green, s.CmdInput, th.reset, th.muted, th.reset)
	}
	if s.Error != "" {
		return fmt.Sprintf("  %s▲ ERROR:%s %s%s%s", th.bold+th.red, th.reset, th.red, s.Error, th.reset)
	}
	return fmt.Sprintf("  %s▸%s %s", th.green, th.reset, s.CmdOutput)
}
