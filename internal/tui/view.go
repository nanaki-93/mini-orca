package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// The frame is four fixed bars around a body of bordered panes:
//
//	header      project · branch · checkout            activity · trust · connection
//	workspaces  1 Inbox  2 Tasks  3 Code  4 Issues  5 Docs  6 Branches  7 Settings
//	body        one or two panes for the active workspace, overlays on top
//	hints       the keys that act on what is selected
//	messages    the latest status message, busy work or error
const frameChromeRows = 4

// minFrameWidth and minFrameHeight are the smallest terminal the frame
// renders in; below it a resize prompt keeps the session intact.
const (
	minFrameWidth  = 52
	minFrameHeight = 16
)

// Render generates a full-screen frame that never exceeds the terminal's dimensions.
func (s *State) Render(width, height int) string {
	width, height = theme.ClampWidth(width), theme.ClampHeight(height)
	th := themeForPalette(s.Palette)
	if s.viewport.width != width || s.viewport.height != height {
		s.viewport.width, s.viewport.height = width, height
		// A resize must not discard an explicit reading position. Navigation or
		// a selection change independently restores focus tracking.
		if !s.viewport.manualScroll {
			s.viewport.followSelection = true
		}
	}
	if width < minFrameWidth || height < minFrameHeight {
		return s.renderSmallTerminal(th, width, height)
	}
	bodyRows := height - frameChromeRows
	body := s.renderBody(th, width, bodyRows)
	body = s.renderOverlay(th, body, width, bodyRows)
	lines := make([]string, 0, height)
	lines = append(lines, s.renderHeaderBar(th, width), s.renderWorkspaceTabs(th, width))
	lines = append(lines, body...)
	lines = append(lines, s.renderKeyHints(th, width), s.renderMessageLine(th, width))
	return fitFrame(th, lines, width, height)
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
	lines := []string{th.bold + th.fg + "◈ mini-orca" + th.reset, "", fmt.Sprintf("Enlarge the terminal to %d × %d.", minFrameWidth, minFrameHeight), th.muted + "Your workspace is preserved." + th.reset, "", th.muted + "Ctrl+C quit" + th.reset}
	if s.CmdActive {
		lines = []string{"◈ mini-orca", th.renderCommandInput(s.CmdInput, s.commandCursor, width)}
		if s.commandMissingArguments() != nil {
			lines = append(lines, theme.WrapVisibleLine(requiredCommandArgumentHint, width)...)
			lines = append(lines, "Esc cancel")
		} else {
			lines = append(lines, "Enter run · Esc cancel")
		}
	}
	return fitFrame(th, lines, width, height)
}

func alignBar(left, right string, width int) string {
	leftWidth := max(0, width-theme.VisibleRuneCount(right)-2)
	return theme.PadVisibleRight(left, leftWidth) + "  " + right
}

// renderHeaderBar names the project and checkout on the left and shows live
// activity, execution trust and the daemon connection on the right. When
// space is short the project name shrinks first, then the branch.
func (s *State) renderHeaderBar(th ansiTheme, width int) string {
	brand := th.accentBg + " ◈ mini-orca " + th.reset
	var right []string
	if activity := s.headerActivity(th); activity != "" {
		right = append(right, activity)
	}
	if s.Project != nil && width >= 96 {
		if s.WorkspaceTrust {
			right = append(right, th.green+"✔"+th.reset+th.muted+" checks trusted"+th.reset)
		} else {
			right = append(right, th.amber+"⚠"+th.reset+th.muted+" checks need trust"+th.reset)
		}
	}
	if s.Connected {
		right = append(right, th.green+"●"+th.reset+th.muted+" live"+th.reset)
	} else {
		right = append(right, th.red+"○"+th.reset+th.muted+" offline"+th.reset)
	}
	rightText := strings.Join(right, th.border+"  │  "+th.reset) + " "
	if s.Project == nil {
		return theme.PaintLine(th.surface, th.fg, alignBar(brand+" "+th.muted+"No project open"+th.reset, rightText, width), width)
	}
	name := client.FirstNonEmpty(s.Project.Name, s.Project.Path)
	branch := "no Git"
	if s.Repository != nil && s.Repository.Available {
		branch = client.FirstNonEmpty(s.Repository.Branch, "detached HEAD")
	}
	modified := ""
	if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
		modified = th.amber + " ●" + th.reset + th.muted + " modified" + th.reset
	}
	budget := width - theme.VisibleRuneCount(rightText) - 2 - theme.VisibleRuneCount(brand) - theme.VisibleRuneCount(modified) - 5
	branchW := min(theme.VisibleRuneCount(branch), max(8, budget/2))
	nameW := max(6, budget-branchW)
	left := brand + " " + th.bold + th.fg + theme.TruncatePlain(name, nameW) + th.reset +
		th.muted + "  ⎇ " + th.reset + th.purple + theme.TruncatePlain(branch, branchW) + th.reset + modified
	return theme.PaintLine(th.surface, th.fg, alignBar(left, rightText, width), width)
}

func (s *State) headerActivity(th ansiTheme) string {
	if s.mediaActiveNotice() != "" {
		return th.amber + "● live media session" + th.reset
	}
	if s.Busy != "" {
		return th.cyan + s.spinner() + " " + theme.TruncatePlain(s.Busy, 32) + th.reset
	}
	if s.asyncWork > 0 {
		return th.cyan + s.spinner() + " working" + th.reset
	}
	_, running, _, _, _ := s.agentFleetBreakdown()
	if running > 0 {
		return th.amber + s.spinner() + fmt.Sprintf(" %d agent run", running) + plural(running) + th.reset
	}
	if s.Run != nil && s.Run.Status == "running" {
		done, total := runAggregateCoverage(s.Run)
		return th.cyan + s.spinner() + fmt.Sprintf(" analysis %d/%d", done, total) + th.reset
	}
	return ""
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// renderWorkspaceTabs lists the seven workspaces with their number keys and
// attention counts; the active one is filled. Narrow terminals drop the
// hint, then shorten inactive tabs to their keys.
func (s *State) renderWorkspaceTabs(th ansiTheme, width int) string {
	active := s.ActiveHub()
	badges := s.workspaceBadges()
	build := func(compact bool) string {
		var parts []string
		for _, tab := range HubTabs() {
			badge := badges[tab.Key]
			if tab.Key == active.Key {
				label := tab.Label
				if badge > 0 {
					label += fmt.Sprintf(" %d", badge)
				}
				parts = append(parts, th.accentBg+" "+tab.Key+" "+label+" "+th.reset)
				continue
			}
			count := ""
			if badge > 0 {
				count = th.amber + fmt.Sprintf(" %d", badge) + th.reset
			}
			if compact {
				// Keys only; a dot marks a workspace that needs attention.
				dot := " "
				if badge > 0 {
					dot = th.amber + "•" + th.reset
				}
				parts = append(parts, " "+th.fg+tab.Key+th.reset+dot)
				continue
			}
			parts = append(parts, " "+th.muted+tab.Key+th.reset+" "+th.fg+tab.Label+th.reset+count+" ")
		}
		return " " + strings.Join(parts, " ")
	}
	left := build(false)
	right := th.muted + "Tab views · ? help · Ctrl+K commands" + th.reset + " "
	if theme.VisibleRuneCount(left)+theme.VisibleRuneCount(right)+2 <= width {
		return alignBar(left, right, width)
	}
	if theme.VisibleRuneCount(left)+2 <= width {
		return left
	}
	return build(true)
}

// workspaceBadges counts what needs attention per workspace key.
func (s *State) workspaceBadges() map[string]int {
	_, running, review, failed, toMerge := s.agentFleetBreakdown()
	_ = running
	badges := map[string]int{"1": review + failed, "6": toMerge}
	if s.Project != nil {
		badges["4"] = len(s.issuesInView("open"))
	}
	return badges
}

// issuesInView counts issues with the given status view without changing the
// active filter.
func (s *State) issuesInView(view string) []IssueRow {
	saved, savedCategory := s.IssueView, s.IssueCategory
	s.IssueView, s.IssueCategory = view, "all"
	issues := s.FilteredIssues()
	s.IssueView, s.IssueCategory = saved, savedCategory
	return issues
}

// renderKeyHints shows the keys that act on the current selection, the
// primary one first and highlighted, followed by an overflow count.
func (s *State) renderKeyHints(th ansiTheme, width int) string {
	items, right := s.keyHintItems(th, width)
	available := width - theme.VisibleRuneCount(right) - 3
	var hints []string
	for i, item := range items {
		hint := th.cyan + item.key + th.reset + " " + th.fg + item.desc + th.reset
		if i == 0 {
			hint = th.accentBg + " " + item.key + " " + th.reset + " " + th.bold + th.fg + item.desc + th.reset
		}
		// Reserve room for the overflow marker so hidden actions stay discoverable.
		reserve := 0
		if i < len(items)-1 {
			reserve = len("   +9 more")
		}
		if theme.VisibleRuneCount(strings.Join(append(hints, hint), "   "))+reserve > available {
			if i == 0 {
				hints = append(hints, theme.TruncateVisible(hint, max(4, available)))
			}
			break
		}
		hints = append(hints, hint)
	}
	if hidden := len(items) - len(hints); hidden > 0 && len(hints) > 0 {
		hints = append(hints, th.muted+fmt.Sprintf("+%d more", hidden)+th.reset)
	}
	return theme.PaintLine(th.surface, th.fg, alignBar(" "+strings.Join(hints, "   "), right, width), width)
}

// keyHintItems picks the hints for the page or the open overlay and the
// right-hand reminder that goes with them.
func (s *State) keyHintItems(th ansiTheme, width int) ([]keyDesc, string) {
	switch {
	case s.CmdActive:
		items := []keyDesc{{"Enter", "Run"}, {"Tab", "Complete"}, {"↑/↓", "Select"}}
		if s.commandMissingArguments() != nil {
			items[0].desc = "Needs arguments"
		}
		return items, th.muted + "Esc cancel" + th.reset + " "
	case s.Confirmation != nil:
		// Cancel is pinned on the right so a long accept label never hides it.
		return []keyDesc{{"y/Enter", client.FirstNonEmpty(s.Confirmation.AcceptLabel, "Confirm")}}, th.cyan + "n/Esc" + th.reset + " " + th.fg + "Cancel" + th.reset + " "
	case s.ShowKeybindsModal || s.nav.showLog:
		return []keyDesc{{"↑/↓", "Scroll"}, {"PgUp/PgDn", "Page"}}, th.muted + "Esc close" + th.reset + " "
	}
	right := th.muted + "Esc back · : commands · ? help" + th.reset + " "
	if width < 100 {
		right = th.muted + ": commands · ? help" + th.reset + " "
	}
	if s.Error != "" {
		return []keyDesc{{"Esc", "Dismiss"}, {"PgUp/PgDn", "Details"}}, right
	}
	return s.pageActions(), right
}

// renderMessageLine shows, in priority order, the command input help, an
// error, busy work with its detail, or the latest status message.
func (s *State) renderMessageLine(th ansiTheme, width int) string {
	switch {
	case s.CmdActive:
		return " " + th.muted + theme.TruncatePlain("Type to search commands · Ctrl+P/N history · Ctrl+U clear", width-2) + th.reset
	case s.Error != "":
		return " " + th.red + "✖ " + th.reset + th.fg + theme.TruncatePlain(theme.SanitizeSingleLine(s.Error), max(1, width-18)) + th.reset + th.muted + "  Esc dismiss" + th.reset
	case s.Busy != "":
		message := s.Busy
		if s.busyDetail != "" {
			message += " · " + s.busyDetail
		}
		return " " + th.cyan + s.spinner() + " " + th.reset + th.fg + theme.TruncatePlain(message, width-4) + th.reset
	}
	message := s.CmdOutput
	if message == "" || message == "Workspace ready" {
		message = "1–7 workspaces · Tab views · ↑/↓ select · Enter open · Ctrl+K commands"
	}
	return " " + th.muted + theme.TruncatePlain(theme.SanitizeSingleLine(message), width-2) + th.reset
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (s *State) spinner() string {
	return spinnerFrames[s.spinnerFrame%len(spinnerFrames)]
}

// showsActivity reports whether the frame has live work to animate.
func (s *State) showsActivity() bool {
	return s.Busy != "" || s.asyncWork > 0 || s.hasActiveBackgroundJob()
}

// statusBarActivity summarizes live work in one phrase, for narrow layouts
// and the message log.
func (s *State) statusBarActivity() string {
	if s.Busy != "" {
		return s.spinner() + " " + s.Busy
	}
	if s.asyncWork > 0 {
		return s.spinner() + " Working in the background…"
	}
	if s.Run != nil && s.Run.Status != "" {
		done, total := runAggregateCoverage(s.Run)
		activity := "◈ Analysis " + s.Run.Status
		if s.Run.Status == "running" {
			activity = s.spinner() + " Analysis running"
		}
		if total > 0 {
			activity += fmt.Sprintf(" · %d/%d", done, total)
		}
		return activity
	}
	if count := len(s.AgentItems()); count > 0 {
		if reviewed, total := s.activeProposalReviewCount(); total > 0 {
			return fmt.Sprintf("⚡ %d active task(s) · %d/%d reviewed", count, reviewed, total)
		}
		return fmt.Sprintf("⚡ %d active task(s)", count)
	}
	if s.Project != nil {
		return "● Ready"
	}
	return ""
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
