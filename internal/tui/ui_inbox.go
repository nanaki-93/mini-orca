package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const inboxSplit = 0.46

// inboxItem is one actionable row of the Inbox.
type inboxItem struct {
	group    string
	glyph    string
	tone     string
	title    string
	right    string
	taskID   string
	worktree int
	page     Page
}

// inboxItems collects what needs attention, most urgent first: tasks to
// review, running agents, failures, branches to merge, then the project's
// issues, analysis, checks and repository.
func (s *State) inboxItems(th ansiTheme) []inboxItem {
	if s.Project == nil {
		return []inboxItem{{group: "Start", glyph: "◈", tone: th.accent, title: "Open a project", right: "o", page: PageSummary}}
	}
	needs, working, failed := s.inboxTaskItems(th)
	var merge []inboxItem
	if s.GitWorkspace != nil {
		for i, wt := range s.GitWorkspace.Worktrees {
			if worktreeStatus(wt) == "TO_MERGE" {
				merge = append(merge, inboxItem{group: "Ready to merge", glyph: "⇡", tone: th.amber, title: client.FirstNonEmpty(wt.Branch, wt.Path), right: th.muted + fmt.Sprintf("%d task(s)", len(wt.Sessions)+len(wt.Completed)) + th.reset, worktree: i})
			}
		}
	}
	summary := domain.ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	issueTone := th.muted
	if summary.TotalOpen > 0 {
		issueTone = th.amber
	}
	project := []inboxItem{{group: "Project", glyph: "!", tone: issueTone, title: "Issues", right: th.muted + fmt.Sprintf("%d open · %d in progress", summary.TotalOpen, summary.InProgress) + th.reset, page: PageBugs, worktree: -1}}
	// Analysis or checks that stopped short need a decision, so they rise
	// into Needs you; otherwise they stay with the project facts.
	for _, item := range []inboxItem{s.inboxAnalysisItem(th), s.inboxChecksItem(th)} {
		if item.group == groupNeedsYou {
			needs = append(needs, item)
		} else {
			project = append(project, item)
		}
	}
	project = append(project, inboxItem{group: "Project", glyph: "◆", tone: th.muted, title: "Repository", right: th.muted + fmt.Sprintf("%d files", s.Project.FileCount) + th.reset, page: PageRepo, worktree: -1})
	var out []inboxItem
	for _, group := range [][]inboxItem{needs, working, failed, merge, project} {
		out = append(out, group...)
	}
	return out
}

// inboxTaskItems sorts tasks into the Inbox groups; drafts and finished tasks
// stay in Tasks.
func (s *State) inboxTaskItems(th ansiTheme) (needs, working, failed []inboxItem) {
	for _, item := range s.AgentItems() {
		tone := s.toneForTask(th, item.Status)
		row := inboxItem{group: tone.group, glyph: tone.glyph, tone: tone.color, title: item.Title, right: tone.color + tone.label + th.reset, taskID: item.ID, worktree: -1}
		switch tone.group {
		case groupNeedsYou:
			needs = append(needs, row)
		case groupWorking:
			row.right = th.muted + item.Stage + th.reset
			working = append(working, row)
		case groupFailed:
			failed = append(failed, row)
		}
	}
	return needs, working, failed
}

func (s *State) inboxAnalysisItem(th ansiTheme) inboxItem {
	item := inboxItem{group: "Project", glyph: "◆", tone: th.muted, title: "Analysis", right: th.muted + "not run yet" + th.reset, page: PageAnalysisRun, worktree: -1}
	if s.Run == nil {
		return item
	}
	right := strings.ReplaceAll(s.Run.Status, "_", " ")
	if done, total := runAggregateCoverage(s.Run); total > 0 {
		right += fmt.Sprintf(" · %d/%d files", done, total)
	}
	item.right, item.tone = th.muted+right+th.reset, th.toneFor(s.Run.Status)
	if analysisNeedsAttention(s.Run.Status) {
		item.group, item.glyph = groupNeedsYou, "▲"
	}
	return item
}

func (s *State) inboxChecksItem(th ansiTheme) inboxItem {
	item := inboxItem{group: "Project", glyph: "◆", tone: th.muted, title: "Checks", right: th.muted + "not run yet" + th.reset, page: PageProject, worktree: -1}
	if s.Scan == nil {
		return item
	}
	item.right, item.tone = th.muted+fmt.Sprintf("%s · %d checks", s.Scan.Status, len(s.Scan.Phases))+th.reset, th.toneFor(s.Scan.Status)
	if phaseTone(s.Scan.Status) == "failed" {
		item.group, item.glyph = groupNeedsYou, "▲"
	}
	return item
}

// analysisNeedsAttention reports analysis outcomes that stopped short.
func analysisNeedsAttention(status string) bool {
	switch status {
	case "partial", "failed", "paused", "interrupted", "canceled":
		return true
	}
	return false
}

// toneFor colors a run or scan status.
func (th ansiTheme) toneFor(status string) string {
	switch phaseTone(status) {
	case "passed":
		return th.green
	case "failed":
		return th.red
	case "running":
		return th.amber
	default:
		return th.muted
	}
}

func (s *State) activateDashboardRow() {
	if s.Project == nil {
		s.openCommandInput("project ")
		return
	}
	items := s.inboxItems(themeForPalette(s.Palette))
	idx := s.ClampedIdx()
	if idx >= len(items) {
		return
	}
	item := items[idx]
	switch {
	case item.taskID != "":
		s.Navigate(PageAgents)
		for i, task := range s.AgentItems() {
			if task.ID == item.taskID {
				s.SelectedIdx = i
				break
			}
		}
		s.syncSelectedAgentSession()
		s.CmdOutput = "Tasks › " + item.title
	case item.worktree >= 0:
		s.Navigate(PageGit)
		s.SelectedIdx = item.worktree
		s.CmdOutput = "Branches › " + item.title
	default:
		s.Navigate(item.page)
	}
}

// renderInboxWorkspace lists what needs attention beside the selected item.
func (s *State) renderInboxWorkspace(th ansiTheme, width, rows int) []string {
	if s.Project == nil {
		return s.renderWelcome(th, width, rows)
	}
	items := s.inboxItems(th)
	listW := listPaneWidth(width, inboxSplit)
	detailW := detailPaneWidth(width, inboxSplit)
	s.viewportBodyRows = listPaneRows(width, rows)
	selected := s.ClampedIdx()
	list := []string{}
	group, cursorRow := "", 0
	for i, item := range items {
		if item.group != group {
			if group != "" {
				list = append(list, "")
			}
			group = item.group
			count := 0
			for _, other := range items {
				if other.group == group {
					count++
				}
			}
			if group == "Project" {
				count = -1
			}
			list = append(list, th.sectionHeader(group, count))
		}
		if i == selected {
			cursorRow = len(list)
		}
		left := item.tone + item.glyph + th.reset + " " + rowTitleStyle(th, i == selected) + theme.SanitizeSingleLine(item.title) + th.reset
		list = append(list, th.listRow(i == selected, rightAlign(left, item.right, listW-1), listW))
	}
	if len(items) > 0 && items[0].group == "Project" {
		list = append([]string{th.green + "✔ " + th.reset + th.fg + "Nothing needs you right now." + th.reset, th.muted + "No task waits for review and no agent is running." + th.reset, ""}, list...)
		cursorRow += 3
	}
	left := theme.Pane{Title: s.viewTabsFit(th, listW-6-min(30, theme.VisibleRuneCount(client.FirstNonEmpty(s.Project.Name, s.Project.Path)))), Meta: theme.TruncatePlain(client.FirstNonEmpty(s.Project.Name, s.Project.Path), 30), Body: list, Focused: true, Scroll: s.listScroll(cursorRow, len(list), s.viewportBodyRows)}
	right := theme.Pane{Title: "Details"}
	if selected < len(items) {
		right = s.inboxDetailPane(th, items[selected], detailW)
	}
	s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(right.Body), detailPaneRows(width, rows))
	right.Scroll = s.DetailScroll
	return s.splitPanes(th, width, rows, inboxSplit, left, right)
}

func (s *State) inboxDetailPane(th ansiTheme, item inboxItem, width int) theme.Pane {
	switch {
	case item.taskID != "":
		for _, task := range s.AgentItems() {
			if task.ID == item.taskID {
				return theme.Pane{Title: "Task", Meta: s.toneForTask(th, task.Status).label, Body: s.taskDetailBody(th, task, width)}
			}
		}
	case item.worktree >= 0 && s.GitWorkspace != nil && item.worktree < len(s.GitWorkspace.Worktrees):
		wt := &s.GitWorkspace.Worktrees[item.worktree]
		return theme.Pane{Title: th.purple + "⎇ " + th.reset + client.FirstNonEmpty(wt.Branch, "detached"), Body: s.worktreeDetailBody(th, wt, width)}
	}
	switch item.page {
	case PageBugs:
		return theme.Pane{Title: "Issues", Body: s.inboxIssuesBody(th, width)}
	case PageAnalysisRun:
		return theme.Pane{Title: "Analysis", Body: s.inboxAnalysisBody(th, width)}
	case PageProject:
		return theme.Pane{Title: "Checks", Body: s.inboxChecksBody(th, width)}
	default:
		return theme.Pane{Title: "Repository", Body: s.inboxProjectBody(th, width)}
	}
}

func (s *State) inboxIssuesBody(th ansiTheme, width int) []string {
	summary := domain.ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	body := []string{
		th.field("Open", th.amber+fmt.Sprint(summary.TotalOpen)+th.reset+th.muted+fmt.Sprintf("  ·  %d bugs · %d security · %d performance", summary.Bugs, summary.Security, summary.Performance)+th.reset, 12),
		th.field("In progress", th.fg+fmt.Sprint(summary.InProgress)+th.reset, 12),
		th.field("Fixed", th.green+fmt.Sprint(summary.Done)+th.reset, 12),
		th.field("Ideas", th.fg+fmt.Sprint(summary.Features)+th.reset+th.muted+" feature ideas to triage"+th.reset, 12),
	}
	open := s.issuesInView("open")
	if len(open) > 0 {
		body = append(body, "", th.sectionHeader("Most severe", -1))
		for _, issue := range topIssues(open, 6) {
			body = append(body, rightAlign(th.severityChip(issue.Severity)+" "+th.fg+theme.SanitizeSingleLine(issue.Name)+th.reset, "", width))
		}
	}
	return append(body, "", th.keyHint("Enter", "open Issues", "4", "Issues workspace"))
}

func topIssues(issues []IssueRow, limit int) []IssueRow {
	rank := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	sorted := append([]IssueRow(nil), issues...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a, okA := rank[strings.ToLower(sorted[j].Severity)]
			b, okB := rank[strings.ToLower(sorted[j-1].Severity)]
			if !okA {
				a = 4
			}
			if !okB {
				b = 4
			}
			if a >= b {
				break
			}
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

func (s *State) inboxAnalysisBody(th ansiTheme, width int) []string {
	if s.Run == nil {
		return []string{th.fg + "Analysis has not run on this project." + th.reset, "", wrapText(th.muted+"It finds bugs, performance and security issues and writes the project handbook. Remote models ask for consent first."+th.reset, width)[0], "", th.keyHint("r", "analyze", "Enter", "configure scope")}
	}
	body := []string{th.field("Status", th.toneFor(s.Run.Status)+s.Run.Status+th.reset, 9)}
	done, total := runAggregateCoverage(s.Run)
	if total > 0 {
		body = append(body, th.field("Coverage", th.renderProgressBar(done, total, max(10, min(30, width-24))), 9))
	}
	if len(s.Run.Sections) > 0 {
		body = append(body, "", th.sectionHeader("Sections", len(s.Run.Sections)))
		for _, sec := range s.Run.Sections {
			right := th.muted + fmt.Sprintf("%d/%d", sec.Coverage.Succeeded+sec.Coverage.Partial, sec.Coverage.Total) + th.reset
			body = append(body, rightAlign(th.toneFor(sec.Status)+"●"+th.reset+" "+th.fg+sec.Category+th.reset+th.muted+"  "+sec.Status+th.reset, right, width))
		}
	}
	if reason := strings.TrimSpace(s.Run.Reason); reason != "" {
		body = append(body, "")
		body = append(body, wrapText(th.amber+"▲ "+th.reset+theme.SanitizeSingleLine(reason), width)...)
	}
	return append(body, "", th.keyHint("r", "run or resume", "Enter", "open Analysis"))
}

func (s *State) inboxChecksBody(th ansiTheme, width int) []string {
	trust := th.amber + "not trusted" + th.reset + th.muted + " · checks need trust to run project code" + th.reset
	if s.WorkspaceTrust {
		trust = th.green + "trusted" + th.reset
	}
	body := []string{th.field("Trust", trust, 8)}
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		body = append(body, "", th.fg+"No checks have run yet."+th.reset)
		return append(body, "", th.keyHint("Enter", "open Checks", "t", "run checks", "7", "trust in Settings"))
	}
	body = append(body, th.field("Status", th.toneFor(s.Scan.Status)+s.Scan.Status+th.reset, 8), "")
	for _, phase := range s.Scan.Phases {
		body = append(body, th.checkGlyph(phase.State)+" "+th.fg+phase.Name+th.reset+th.muted+"  "+strings.Join(phase.Command, " ")+th.reset)
	}
	return append(body, "", th.keyHint("Enter", "open Checks", "t", "run checks"))
}

func (s *State) inboxProjectBody(th ansiTheme, width int) []string {
	p := s.Project
	branch := "no Git"
	if s.Repository != nil && s.Repository.Available {
		branch = client.FirstNonEmpty(s.Repository.Branch, "detached HEAD")
	}
	body := []string{
		th.field("Name", th.fg+p.Name+th.reset, 9),
		th.field("Path", th.fg+theme.TruncatePlain(p.Path, max(10, width-10))+th.reset, 9),
		th.field("Type", th.fg+client.FirstNonEmpty(p.Type, "polyglot")+th.reset, 9),
		th.field("Files", th.fg+fmt.Sprint(p.FileCount)+th.reset, 9),
		th.field("Branch", th.purple+branch+th.reset, 9),
	}
	body = append(body, "", th.sectionHeader("Models", -1))
	for _, role := range []struct{ scope, label string }{{"analyze", "Analyze"}, {"bug", "Fix"}, {"code", "Code"}} {
		model := s.EffectiveScopeModel(role.scope).Model
		if model == "" || model == role.scope {
			model = th.muted + "not configured" + th.reset
		}
		body = append(body, th.field(role.label, th.fg+theme.TruncatePlain(theme.SanitizeSingleLine(model), max(10, width-10))+th.reset, 9))
	}
	return append(body, "", th.keyHint("o", "switch project", "r", "reindex", ":web", "browser workspace"))
}

// renderWelcome is the first screen before a project is open.
func (s *State) renderWelcome(th ansiTheme, width, rows int) []string {
	body := []string{
		"",
		th.bold + th.fg + "Your code. A clear next step." + th.reset,
		"",
		th.muted + "Mini-Orca plans changes with agents, checks them and asks you to review every file before it touches your source." + th.reset,
		"",
		th.accentBg + " o " + th.reset + " " + th.bold + th.fg + "Open a local project" + th.reset + th.muted + "   or run mini-orca --project <path>" + th.reset,
		"",
		th.sectionHeader("From idea to reviewed change", -1),
	}
	for i, step := range [][2]string{
		{"Open", "Index the project and restore its saved results."},
		{"Analyze", "Find issues and write the project handbook."},
		{"Plan", "Describe the task, its scope and when it is done."},
		{"Review", "Read each changed file, run checks, then Apply or Undo."},
	} {
		body = append(body, th.accent+fmt.Sprintf("%d", i+1)+th.reset+"  "+th.bold+th.fg+theme.PadVisibleRight(step[0], 9)+th.reset+th.muted+step[1]+th.reset)
	}
	if !s.Connected {
		body = append(body, "", th.amber+"▲ "+th.reset+th.fg+"The daemon is offline. Start it, then run :reconnect."+th.reset)
	}
	if s.Run != nil && strings.TrimSpace(s.Run.Reason) != "" {
		body = append(body, "", th.amber+"Analysis: "+th.reset+th.fg+theme.SanitizeSingleLine(s.Run.Reason)+th.reset)
		if s.isAnalysisPausedOrInterrupted() {
			body = append(body, th.keyHint("r", "Resume the analysis from its saved progress"))
		}
	}
	var wrapped []string
	for _, line := range body {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		wrapped = append(wrapped, wrapText(line, width-8)...)
	}
	return theme.RenderPane(th.paneStyle(), theme.Pane{Title: "Welcome to mini-orca", Body: wrapped, Focused: true}, width, rows)
}

// humanTime renders an RFC 3339 timestamp as a relative age.
func humanTime(value string) string {
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	d := time.Since(at)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
