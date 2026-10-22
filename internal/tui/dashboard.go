package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

var dashboardPages = []Page{PageBugs, PageAgents, PageAnalysisRun, PageProject}

func (s *State) activateDashboardRow() {
	if s.Project == nil {
		s.openCommandInput("project ")
		return
	}
	s.Navigate(dashboardPages[s.ClampedIdx()])
}

func (s *State) renderSummaryPage(th ansiTheme, width int) []string {
	if s.Project == nil && s.Run == nil {
		return s.renderWelcomePage(th, width)
	}
	lines := renderPageHeader(th, "DASHBOARD", "Project activity and next steps")
	if s.Project != nil {
		lines = append(lines, "  "+th.muted+theme.TruncatePlain(s.Project.Path, width-4)+th.reset)
	}
	summary := domain.ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	cards := [][2]string{
		{"Open issues", fmt.Sprintf("%s%d%s", th.bold+th.amber, summary.TotalOpen, th.reset)},
		{"Resolved", fmt.Sprintf("%s%d%s", th.bold+th.green, summary.Done, th.reset)},
		{"Tasks", fmt.Sprintf("%s%d%s", th.bold+th.fg, len(s.AgentItems()), th.reset)},
		{"Features", fmt.Sprintf("%s%d%s", th.bold+th.fg, summary.Features, th.reset)},
	}
	lines = append(lines, "")
	lines = append(lines, th.renderMetrics(cards, width)...)
	lines = append(lines, "", "  "+th.muted+"ACTIVITY"+th.reset)
	activity := [][2]string{
		{"Issues", fmt.Sprintf("%d open · %d in progress", summary.TotalOpen, summary.InProgress)},
		{"Tasks", fmt.Sprintf("%d tasks · Plan, verify and review changes", len(s.AgentItems()))},
		{"Analysis", "Not run yet"},
		{"Checks", "Not run yet"},
	}
	if s.Run != nil {
		done, total := runAggregateCoverage(s.Run)
		activity[2][1] = th.statusBadge(s.Run.Status)
		if total > 0 {
			activity[2][1] += fmt.Sprintf(" %d/%d files", done, total)
		}
	}
	if s.Scan != nil {
		activity[3][1] = th.statusBadge(s.Scan.Status) + fmt.Sprintf(" %d checks", len(s.Scan.Phases))
	}
	for i, item := range activity {
		lines = append(lines, "  "+th.renderCursorPrefix(i == s.ClampedIdx())+rowTitleStyle(th, i == s.ClampedIdx())+fmt.Sprintf("%-11s", item[0])+th.reset+item[1])
	}
	lines = append(lines, "", "  "+th.muted+"NEXT STEP"+th.reset)
	lines = appendWrappedText(lines, th.cyan+s.dashboardNextStep()+th.reset, width)
	if s.Run != nil && strings.TrimSpace(s.Run.Reason) != "" {
		lines = append(lines, "")
		lines = appendWrappedText(lines, th.muted+"Analysis: "+s.Run.Reason+th.reset, width)
	}
	return lines
}

func (s *State) dashboardNextStep() string {
	switch {
	case !s.Connected:
		return ":reconnect to refresh the connection and workspace data."
	case s.isAnalysisPausedOrInterrupted():
		return "[r] Resume analysis from its saved progress."
	case s.hasRunningWorkflow():
		return "[2] Inspect running tasks and their stage diagnostics."
	case s.HasActiveProposal():
		return ":diff to inspect the proposal. Review each changed file before Apply."
	case s.Run == nil:
		return "[r] Analyze the project to discover issues and feature ideas."
	case s.Run.Status == "failed":
		return "Open Analysis to inspect failures; [R] retries failed stages."
	default:
		return "[Enter] Open an activity · [o] Switch project · :agent <request> to plan a task."
	}
}

func (s *State) renderWelcomePage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "WELCOME", "A clear workspace for your next change")
	lines = append(lines, "", "  "+th.bold+th.fg+"Open a local project"+th.reset)
	lines = appendWrappedText(lines, "Browse its code, investigate findings and review changes before applying them.", width)
	lines = append(lines, "", "  "+th.renderCursorPrefix(true)+th.keyPill("Enter / o", "Open project"), "", "  "+th.muted+"HOW IT WORKS"+th.reset)
	for _, step := range []string{
		"1  Open       Restore the project and its saved results.",
		"2  Analyze    Find issues and explore feature ideas.",
		"3  Plan       Choose scope and approve the approach.",
		"4  Review     Inspect checked diffs, then Apply or Undo.",
	} {
		lines = appendWrappedText(lines, step, width)
	}
	if !s.Connected {
		lines = append(lines, "")
		lines = appendWrappedText(lines, th.amber+"Daemon offline. Start the daemon, then use :reconnect."+th.reset, width)
	}
	return lines
}

func appendWrappedText(lines []string, text string, width int) []string {
	for _, row := range theme.WrapVisibleLine(text, max(1, width-4)) {
		lines = append(lines, "  "+row)
	}
	return lines
}

func (s *State) selectedCheck() *Check {
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return nil
	}
	return &s.Scan.Phases[s.ClampedIdx()]
}

func (s *State) renderCheckDetails(th ansiTheme, width int) []string {
	check := s.selectedCheck()
	if check == nil {
		return nil
	}
	rows := []string{th.statusBadge(check.State) + " " + client.FirstNonEmpty(check.Name, "Check")}
	rows = append(rows, "Command: "+strings.Join(check.Command, " "))
	if check.DurationMS > 0 {
		rows = append(rows, fmt.Sprintf("Duration: %dms", check.DurationMS))
	}
	if check.Output == "" {
		rows = append(rows, "No diagnostic output recorded.")
	} else {
		for _, line := range strings.Split(check.Output, "\n") {
			rows = append(rows, theme.SanitizeCodeText(line))
		}
	}
	return th.renderPanelBox("SELECTED CHECK", "PgUp/PgDn: details", adaptivePanelWidth(width), rows)
}
