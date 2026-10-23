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
	summary := domain.ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	lines := renderPageHeader(th, "DASHBOARD · PROJECT OVERVIEW", "↑/↓ Select · Enter Open · n New task · r Analyze")
	if s.viewportBodyRows >= 16 {
		if s.Project != nil {
			meta := s.Project.Path
			if s.Project.FileCount > 0 {
				meta = fmt.Sprintf("%s  ·  %s  ·  %d files", s.Project.Path, client.FirstNonEmpty(s.Project.Type, "polyglot"), s.Project.FileCount)
			}
			lines[1] = "  " + th.muted + theme.TruncatePlain(meta, width-4) + th.reset
		}
		cards := [][2]string{
			{"Open issues", fmt.Sprintf("%s%d%s %s(%d active)%s", th.amber, summary.TotalOpen, th.reset, th.muted, summary.InProgress, th.reset)},
			{"Resolved", fmt.Sprintf("%s%d%s %sverified%s", th.green, summary.Done, th.reset, th.muted, th.reset)},
			{"Tasks", fmt.Sprintf("%s%d%s %sactive%s", th.purple, len(s.AgentItems()), th.reset, th.muted, th.reset)},
			{"Features", fmt.Sprintf("%s%d%s %sideas%s", th.cyan, summary.Features, th.reset, th.muted, th.reset)},
		}
		lines = append(lines, "")
		lines = append(lines, th.renderMetrics(cards, width)...)
		lines = append(lines, "")
	}
	if width >= 88 && s.viewportBodyRows >= 22 {
		lines = append(lines, s.renderDashboardColumns(th, summary, width)...)
	} else {
		lines = append(lines, s.renderDashboardActivities(th, summary, width, false)...)
		lines = append(lines, "", "  "+th.cyan+"NEXT STEP"+th.reset)
		lines = appendWrappedText(lines, s.dashboardNextStep(), width)
	}
	if s.Run != nil && strings.TrimSpace(s.Run.Reason) != "" {
		lines = append(lines, "")
		lines = appendWrappedText(lines, th.amber+"Analysis: "+s.Run.Reason+th.reset, width)
	}
	return lines
}

type dashboardActivity struct {
	label, summary, detail, color string
}

func (s *State) dashboardActivities(th ansiTheme, summary domain.IssueSummary) []dashboardActivity {
	taskSummary := fmt.Sprintf("%d tasks", len(s.AgentItems()))
	if items := s.AgentItems(); len(items) > 0 {
		taskSummary += " · " + th.statusBadge(items[0].Status)
	}
	activity := []dashboardActivity{
		{"Issues", fmt.Sprintf("%d open · %d in progress", summary.TotalOpen, summary.InProgress), "Inspect evidence, select batches and plan fixes.", th.amber},
		{"Tasks", taskSummary, "Inspect progress, evidence and pending reviews.", th.purple},
		{"Analysis", "Not run yet", "Semantic, security and performance discovery.", th.cyan},
		{"Checks", "Not run yet", "Run sandboxed verification checks and logs.", th.green},
	}
	if s.Run != nil {
		done, total := runAggregateCoverage(s.Run)
		activity[2].summary = th.statusBadge(s.Run.Status)
		if total > 0 {
			activity[2].summary += fmt.Sprintf(" %d/%d files", done, total)
		}
	}
	if s.Scan != nil {
		activity[3].summary = th.statusBadge(s.Scan.Status) + fmt.Sprintf(" %d checks", len(s.Scan.Phases))
	}
	return activity
}

func (s *State) renderDashboardActivities(th ansiTheme, summary domain.IssueSummary, width int, detailed bool) []string {
	var lines []string
	for i, item := range s.dashboardActivities(th, summary) {
		row := "  " + th.renderCursorPrefix(i == s.ClampedIdx()) + item.color + fmt.Sprintf("%-10s", item.label) + th.reset + item.summary
		lines = append(lines, wrapWorkspaceLines([]string{row}, width)...)
		if detailed {
			for _, line := range theme.WrapVisibleLine(item.detail, width-6) {
				lines = append(lines, "    "+th.muted+line+th.reset)
			}
			lines = append(lines, "")
		}
	}
	return lines
}

func (s *State) renderDashboardColumns(th ansiTheme, summary domain.IssueSummary, width int) []string {
	rightWidth := max(30, (width-2)/3)
	leftWidth := width - rightWidth - 5
	left := []string{"  " + th.bold + th.purple + "ACTIVITY" + th.reset, ""}
	left = append(left, s.renderDashboardActivities(th, summary, leftWidth, true)...)
	if s.viewportBodyRows >= 26 {
		left = append(left, s.renderDashboardPipelineSection(th, leftWidth)...)
	} else {
		left = append(left, "  "+th.muted+"↑/↓ Select · Enter Open"+th.reset)
	}
	right := s.renderDashboardRightColumn(th, summary, rightWidth)
	var lines []string
	for i := 0; i < max(len(left), len(right)); i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, theme.PadVisibleRight(l, leftWidth)+"   "+theme.PaintLine(th.surface, th.fg, r, rightWidth))
	}
	return lines
}

func (s *State) renderDashboardPipelineSection(th ansiTheme, width int) []string {
	out := []string{"  " + th.bold + th.cyan + "CURRENT TASK" + th.reset}
	if items := s.AgentItems(); len(items) > 0 {
		top := items[0]
		meta := fmt.Sprintf("  %s %s · %s", th.statusBadge(top.Status), th.bold+th.fg+theme.TruncatePlain(top.Title, max(12, width-28))+th.reset, th.purple+top.Stage+th.reset)
		out = append(out, theme.TruncateVisible(meta, max(20, width-2)))
	} else {
		out = append(out, "  "+th.muted+"No active tasks · [n] Create a plan"+th.reset)
	}
	return out
}

func (s *State) renderDashboardRightColumn(th ansiTheme, summary domain.IssueSummary, rightWidth int) []string {
	_, running, review, failed, toMerge := s.agentFleetBreakdown()
	fleetLine := fmt.Sprintf("%s%d running%s · %s%d review%s", th.amber, running, th.reset, th.green, review, th.reset)
	if failed > 0 {
		fleetLine += fmt.Sprintf(" · %s%d failed%s", th.red, failed, th.reset)
	}
	wtTotal := 0
	if s.GitWorkspace != nil {
		wtTotal = len(s.GitWorkspace.Worktrees)
	}
	trustBadge := th.statusBadge("UNTRUSTED")
	if s.WorkspaceTrust {
		trustBadge = th.statusBadge("TRUSTED")
	}
	right := []string{
		"  " + th.bold + th.cyan + "WORKSPACE" + th.reset,
		fmt.Sprintf("  %sTasks:%s     %s", th.muted, th.reset, fleetLine),
		fmt.Sprintf("  %sWorktrees:%s %s%d%s total · %s%d%s to merge", th.muted, th.reset, th.bold+th.fg, wtTotal, th.reset, th.purple, toMerge, th.reset),
		fmt.Sprintf("  %sFindings:%s  %s%d%s bug · %s%d%s sec · %s%d%s perf", th.muted, th.reset, th.amber, summary.Bugs, th.reset, th.red, summary.Security, th.reset, th.cyan, summary.Performance, th.reset),
		fmt.Sprintf("  %sSandbox:%s   %s", th.muted, th.reset, trustBadge),
		"  " + th.border + strings.Repeat("─", max(8, rightWidth-4)) + th.reset,
		"  " + th.bold + th.cyan + "NEXT STEP" + th.reset,
	}
	right = appendWrappedText(right, s.dashboardNextStep(), rightWidth)
	right = append(right,
		"",
		"  "+th.keyPill("n", "New task"),
		"  "+th.keyPill("r", "Analyze"),
		"  "+th.keyPill("o", "Switch project"),
		"  "+th.keyPill("Ctrl+K", "Commands"),
	)
	return right
}

func (s *State) agentFleetBreakdown() (total, running, review, failed, toMerge int) {
	items := s.AgentItems()
	total = len(items)
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "running", "queued", "working", "in_progress", "canceling":
			running++
		case "ready", "checked", "reviewed", "awaiting_human_review":
			review++
		case "failed", "changes_requested", "interrupted", "stale":
			failed++
		}
	}
	if s.GitWorkspace != nil {
		for _, wt := range s.GitWorkspace.Worktrees {
			if worktreeStatus(wt) == "TO_MERGE" {
				toMerge++
			}
		}
	}
	return total, running, review, failed, toMerge
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
		return "Choose an activity to inspect its results, or press [n] to plan a new task."
	}
}

func (s *State) renderWelcomePage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "WELCOME · AGENT ORCHESTRATOR", "Your code. A clearer next step.")
	lines = append(lines, "", "  "+th.renderCursorPrefix(true)+th.keyPill("Enter / o", "Open a local project"))
	lines = appendWrappedText(lines, "Browse code, investigate findings and review every change before Apply.", width)
	lines = append(lines, "", "  "+th.bold+th.cyan+"◈ FROM IDEA TO REVIEWED CHANGE"+th.reset, "")
	for i, step := range [][2]string{
		{"Open", "Restore a project and its saved results."},
		{"Analyze", "Explore issues and feature ideas."},
		{"Plan", "Choose scope and approve the approach."},
		{"Review", "Inspect checked diffs, then Apply or Undo."},
	} {
		color := []string{th.accent, th.cyan, th.purple, th.green}[i]
		lines = appendWrappedText(lines, fmt.Sprintf("%s%02d  %-9s%s %s", color, i+1, step[0], th.reset, step[1]), width)
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
