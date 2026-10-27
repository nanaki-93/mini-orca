package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// tasksSplit is the list share of the Tasks workspace.
const tasksSplit = 0.4

// renderTasksWorkspace lists tasks beside the selected task's brief, review
// gate and live agent run. Opening a task (Enter) shows its stages beside
// the selected stage's evidence.
func (s *State) renderTasksWorkspace(th ansiTheme, width, rows int) []string {
	if s.AgentDetailOpen && s.Change != nil {
		return s.renderTaskStagesWorkspace(th, width, rows)
	}
	items := s.AgentItems()
	listW := listPaneWidth(width, tasksSplit)
	detailW := detailPaneWidth(width, tasksSplit)
	s.viewportBodyRows = listPaneRows(width, rows)
	selected := s.ClampedIdx()
	var list []string
	if len(items) == 0 {
		list = emptyPane(th, "No active tasks", "n  plan a new task", "4  pick an issue to fix", ":features  feature ideas")
	} else {
		list = s.taskListRows(th, items, selected, listW)
	}
	scroll := s.listScroll(s.taskListCursorRow(items, selected), len(list), s.viewportBodyRows)
	left := theme.Pane{Title: s.viewTabsFit(th, listW-14), Meta: fmt.Sprintf("%d active", len(items)), Body: list, Focused: true, Scroll: scroll}
	right := theme.Pane{Title: "Task", Body: emptyPane(th, "Select a task", "Its brief, review gate and agent run appear here.")}
	if len(items) > 0 {
		item := items[selected]
		right.Title = "Task"
		right.Body = s.taskDetailBody(th, item, detailW)
		right.Meta = s.toneForTask(th, item.Status).label
		s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(right.Body), detailPaneRows(width, rows))
		right.Scroll = s.DetailScroll
	}
	return s.splitPanes(th, width, rows, tasksSplit, left, right)
}

// taskListRows renders one row per task in engine order (j/k follow it), each
// with a status glyph, the title and a right-aligned status and diff stat.
func (s *State) taskListRows(th ansiTheme, items []AgentItem, selected, width int) []string {
	rows := make([]string, 0, len(items)+2)
	_, running, review, failed, _ := s.agentFleetBreakdown()
	rows = append(rows, th.muted+fmt.Sprintf("%d need review · %d running · %d need attention", review, running, failed)+th.reset, "")
	for i, item := range items {
		tone := s.toneForTask(th, item.Status)
		right := tone.color + strings.ToLower(tone.label) + th.reset
		left := tone.color + tone.glyph + th.reset + " " + rowTitleStyle(th, i == selected) + theme.SanitizeSingleLine(item.Title) + th.reset
		rows = append(rows, th.listRow(i == selected, rightAlign(left, right, width-1), width))
	}
	return rows
}

// taskListCursorRow is the body row of the selected task.
func (s *State) taskListCursorRow(items []AgentItem, selected int) int {
	if len(items) == 0 {
		return 0
	}
	return selected + 2
}

// taskDetailBody is the right pane for a task: what it is, whether it can be
// applied, how the agents are doing and what to press next.
func (s *State) taskDetailBody(th ansiTheme, item AgentItem, width int) []string {
	chg := s.sessionFor(item.ID)
	if chg == nil {
		return append(wrapText(th.bold+th.fg+theme.SanitizeSingleLine(item.Title)+th.reset, width), "", th.muted+"Loading task details…"+th.reset)
	}
	body := s.taskHeading(th, item, chg, width)
	body = append(body, s.taskGateRows(th, chg, width)...)
	body = append(body, s.taskRunLines(th, chg, width)...)
	if len(chg.Checks) > 0 {
		passed, total := domain.CountPassedChecks(chg.Checks)
		body = append(body, "", rightAlign(th.sectionHeader("Checks", -1), th.muted+fmt.Sprintf("%d/%d required passed", passed, total)+th.reset, width))
		body = append(body, s.checkChips(th, chg.Checks, width)...)
	}
	body = append(body, s.taskFileLines(th, chg, width)...)
	if len(chg.AcceptanceCriteria) > 0 {
		body = append(body, "", th.sectionHeader("Done when", len(chg.AcceptanceCriteria)))
		for i, criterion := range chg.AcceptanceCriteria {
			body = append(body, wrapText(th.muted+fmt.Sprintf("%d.", i+1)+th.reset+" "+theme.SanitizeSingleLine(criterion), width)...)
		}
	}
	body = append(body, "", th.sectionHeader("Next", -1))
	return append(body, wrapText(s.taskNextStep(th, item), width)...)
}

// taskHeading is the task's title and a one-line summary of its status, kind,
// branch and version.
func (s *State) taskHeading(th ansiTheme, item AgentItem, chg *ChangeSession, width int) []string {
	tone := s.toneForTask(th, item.Status)
	body := wrapText(th.bold+th.fg+theme.SanitizeSingleLine(chg.Title)+th.reset, width)
	meta := []string{tone.color + tone.glyph + " " + tone.label + th.reset}
	if chg.Kind != "" {
		meta = append(meta, th.muted+chg.Kind+th.reset)
	}
	meta = append(meta, th.purple+"⎇ "+theme.TruncatePlain(item.Branch, 28)+th.reset)
	if chg.Revision > 0 {
		meta = append(meta, th.muted+fmt.Sprintf("v%d · %s", chg.Revision, shortID(chg.Hash))+th.reset)
	}
	return append(body, strings.Join(meta, th.muted+"  ·  "+th.reset), "")
}

// taskRunLines is the agent run: each stage with its state and time, then
// recovery and the run's outcome on one line.
func (s *State) taskRunLines(th ansiTheme, chg *ChangeSession, width int) []string {
	stages := s.stagesFor(chg)
	if chg.Workflow == nil || len(stages) == 0 {
		return nil
	}
	body := []string{"", rightAlign(th.sectionHeader("Agent run", -1), th.muted+workflowShortTiming(chg)+th.reset, width)}
	for _, stage := range stages {
		body = append(body, s.stageRow(th, stage, width))
	}
	if restartNeedsConsent(chg.Workflow.Status) {
		body = append(body, wrapText(th.amber+"▲ "+th.reset+th.fg+"Restarting requires fresh consent and checks."+th.reset, width)...)
	}
	if reason := strings.TrimSpace(chg.Workflow.Reason); reason != "" && chg.Workflow.Status != "running" {
		// One line here; Enter opens the stages with the full diagnostics.
		body = append(body, "", th.amber+"▲ "+th.reset+th.fg+theme.TruncatePlain(theme.SanitizeSingleLine(reason), max(8, width-2))+th.reset)
	}
	return body
}

// taskFileLines lists the changed files with their review marks, or the
// captured scope before any change exists.
func (s *State) taskFileLines(th ansiTheme, chg *ChangeSession, width int) []string {
	changes := domain.EffectiveSessionChanges(chg)
	if len(changes) == 0 {
		if len(chg.Targets) == 0 {
			return nil
		}
		body := []string{"", th.sectionHeader("Scope", len(chg.Targets))}
		for _, target := range chg.Targets {
			body = append(body, th.muted+"·"+th.reset+" "+th.fg+theme.SanitizeSingleLine(target.Path)+th.reset)
		}
		return body
	}
	added, removed, _ := domain.ClassifyDiffScope(changes)
	body := []string{"", rightAlign(th.sectionHeader("Changed files", len(changes)), th.diffStat(added, removed), width)}
	reviewedAtHash := chg.FileReviewHash == chg.Hash
	for _, fc := range changes {
		mark := th.muted + "○" + th.reset
		if reviewedAtHash && domain.ContainsString(chg.ReviewedFiles, fc.Path) {
			mark = th.green + "✔" + th.reset
		}
		a, r := diffLineCounts(fc)
		body = append(body, rightAlign(mark+" "+th.fg+theme.SanitizeSingleLine(fc.Path)+th.reset, th.diffStat(a, r), width))
	}
	return body
}

// sessionFor finds the loaded session of a task: the current one or an
// active one.
func (s *State) sessionFor(id string) *ChangeSession {
	if s.Change != nil && s.Change.ID == id {
		return s.Change
	}
	for _, chg := range s.ActiveChanges {
		if chg != nil && chg.ID == id {
			return chg
		}
	}
	return nil
}

// stagesFor returns a session's stages, with native progress for the
// current session.
func (s *State) stagesFor(chg *ChangeSession) []domain.AgentStage {
	if chg == s.Change {
		return s.currentAgentStages()
	}
	return domain.WorkflowStages(chg, nil)
}

// taskGateRows mirrors what Apply requires: finished agents, reviewer
// verdict, every changed file reviewed at this version and passing checks.
func (s *State) taskGateRows(th ansiTheme, chg *ChangeSession, width int) []string {
	rows := []string{th.sectionHeader("Ready to apply?", -1)}
	agentsDone, agentsActive, agentDetail := agentGate(chg)
	rows = append(rows, th.gateRow(agentsDone, agentsActive, "Agents finished", agentDetail, width))
	verdict, verdictDetail := reviewerVerdict(chg)
	rows = append(rows, th.gateRow(verdict == "approve", false, "Reviewers", verdictDetail, width))
	reviewed, total := s.reviewCountFor(chg)
	filesDone := total > 0 && reviewed == total
	rows = append(rows, th.gateRow(filesDone, agentsDone && !filesDone, "Files reviewed", fmt.Sprintf("%d of %d · d review", reviewed, total), width))
	passed, checks := domain.CountPassedChecks(chg.Checks)
	checkDetail := "not run"
	if checks > 0 {
		checkDetail = fmt.Sprintf("%d/%d passed", passed, checks)
	}
	checksDone := checks > 0 && passed == checks
	rows = append(rows, th.gateRow(checksDone, filesDone && !checksDone, "Checks", checkDetail, width))
	applied := chg.State == "applied"
	applyDetail := "a apply & commit"
	if applied {
		applyDetail = "applied · u undo"
	}
	return append(rows, th.gateRow(applied, agentsDone && filesDone && checksDone && !applied, "Apply", applyDetail, width))
}

// agentGate reports whether the agents produced a finished proposal, whether
// that step is the current one and how to describe it.
func agentGate(chg *ChangeSession) (done, active bool, detail string) {
	changes := len(domain.EffectiveSessionChanges(chg))
	running := chg.Workflow != nil && (chg.Workflow.Status == "running" || chg.Workflow.Status == "canceling")
	switch {
	case running:
		detail = "working…"
	case chg.Workflow != nil:
		detail = strings.ReplaceAll(chg.Workflow.Status, "_", " ")
	case changes > 0:
		detail = "manual proposal"
	default:
		detail = "no changes yet"
	}
	return changes > 0 && !running, running || changes == 0, detail
}

// reviewCountFor counts reviewed files of chg at its current hash.
func (s *State) reviewCountFor(chg *ChangeSession) (int, int) {
	changes := domain.EffectiveSessionChanges(chg)
	if chg.FileReviewHash != chg.Hash {
		return 0, len(changes)
	}
	reviewed := 0
	for _, fc := range changes {
		if domain.ContainsString(chg.ReviewedFiles, fc.Path) {
			reviewed++
		}
	}
	return reviewed, len(changes)
}

// reviewerVerdict is the reviewers' overall verdict: the recorded aggregate,
// else derived from the specialists.
func reviewerVerdict(chg *ChangeSession) (string, string) {
	if chg.Workflow == nil {
		return "none", "not part of a manual proposal"
	}
	if rev := chg.Workflow.Review; rev != nil && rev.Verdict != "" {
		return rev.Verdict, strings.ReplaceAll(rev.Verdict, "_", " ")
	}
	specialists := chg.Workflow.Specialists
	approved := 0
	for _, sp := range specialists {
		if sp.Review == nil {
			continue
		}
		switch sp.Review.Verdict {
		case "changes_requested", "reject":
			return "changes_requested", "changes requested by " + sp.ID
		case "approve":
			approved++
		}
	}
	if len(specialists) > 0 && approved == len(specialists) {
		return "approve", fmt.Sprintf("%d approved", approved)
	}
	return "pending", "pending"
}

func (s *State) stageRow(th ansiTheme, stage domain.AgentStage, width int) string {
	label := stage.Label
	if stage.Parallel {
		label = "  ↳ " + label
	}
	right := ""
	switch {
	case stage.StartedAt != "" && stage.State != domain.StagePending:
		right += th.muted + workflowElapsed(stage.StartedAt, stage.FinishedAt) + th.reset
	case stage.State == domain.StageWaiting:
		right += th.accent + "waiting for you" + th.reset
	default:
		right += th.muted + strings.ReplaceAll(string(stage.State), "_", " ") + th.reset
	}
	return rightAlign(s.stageGlyph(th, stage.State)+" "+th.fg+label+th.reset, right, width)
}

// checkChips lists checks compactly, failed ones first with their exit code.
func (s *State) checkChips(th ansiTheme, checks []client.Check, width int) []string {
	var failed, rest []string
	for _, chk := range checks {
		chip := th.checkGlyph(chk.State) + " " + th.fg + chk.Name + th.reset
		if !chk.Required {
			chip += th.muted + " (optional)" + th.reset
		}
		if phaseTone(chk.State) == "failed" {
			failed = append(failed, chip+th.muted+fmt.Sprintf(" exit %d", chk.ExitCode)+th.reset)
		} else {
			rest = append(rest, chip)
		}
	}
	var out []string
	out = append(out, failed...)
	line := ""
	for _, chip := range rest {
		if line != "" && theme.VisibleRuneCount(line+"   "+chip) > width {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += "   "
		}
		line += chip
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func (s *State) taskNextStep(th ansiTheme, item AgentItem) string {
	switch strings.ToLower(item.Status) {
	case "stale":
		return th.keyHint("r", "plan it again on the current source", "X", "discard")
	case "running", "queued", "canceling":
		return th.keyHint("Enter", "follow the stages", "x", "cancel")
	case "failed", "changes_requested", "interrupted":
		return th.keyHint("Enter", "inspect failure", "r", "retry", "X", "discard")
	case "ready", "checked", "reviewed":
		return th.keyHint("d", "review changed files", "a", "apply & commit", "x", "reject")
	case "applied":
		return th.keyHint("u", "guarded undo", "m", "merge branch")
	default:
		return th.keyHint("Enter", "open", "d", "review", ":agent", "plan another task")
	}
}

// restartNeedsConsent reports workflow states that cannot resume: a restart
// asks for consent again and re-runs checks.
func restartNeedsConsent(status string) bool {
	return status == "interrupted" || status == "outdated" || status == "stale"
}

func workflowShortTiming(chg *ChangeSession) string {
	if chg == nil || chg.Workflow == nil {
		return ""
	}
	w := chg.Workflow
	end := w.UpdatedAt
	if w.Status == "running" || w.Status == "canceling" {
		end = ""
	}
	parts := []string{fmt.Sprintf("attempt %d", max(1, w.Attempt)+w.AutoRepairAttempts)}
	if elapsed := workflowElapsed(w.StartedAt, end); elapsed != "not timed" {
		parts = append(parts, elapsed)
	}
	if w.AutoRepair {
		parts = append(parts, "auto-repair")
	}
	return strings.Join(parts, " · ")
}

func shortID(hash string) string {
	hash = strings.TrimPrefix(hash, "sha256:")
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// renderTaskStagesWorkspace is an opened task: its stages beside the
// selected stage's evidence, or a run section (Timeline, Evidence, Usage,
// Recovery) across the full width.
func (s *State) renderTaskStagesWorkspace(th ansiTheme, width, rows int) []string {
	chg := s.Change
	title := theme.TruncatePlain(client.FirstNonEmpty(chg.Title, chg.ID), max(20, width/2))
	if s.AgentRunSection > 0 && s.AgentRunSection < len(agentRunSections) {
		inner := width - 4
		s.viewportBodyRows = paneRows(rows)
		body := tidyContent(th, wrapWorkspaceLines(s.renderAgentRunPage(th, inner+2), inner+2), inner)
		s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(body), paneRows(rows))
		tabs := theme.Tabs(agentRunSections, s.AgentRunSection, th.bold+th.accent, th.muted, th.reset)
		return theme.RenderPane(th.paneStyle(), theme.Pane{Title: title + th.muted + "  ›  " + th.reset + tabs, Body: body, Focused: true, Scroll: s.DetailScroll}, width, rows)
	}
	stages := s.currentAgentStages()
	selected := s.effectiveAgentDetailStage()
	listW := listPaneWidth(width, tasksSplit)
	detailW := detailPaneWidth(width, tasksSplit)
	s.viewportBodyRows = detailPaneRows(width, rows)
	list := wrapText(th.bold+th.fg+theme.SanitizeSingleLine(client.FirstNonEmpty(chg.Title, chg.ID))+th.reset, listW)
	list = append(list, wrapText(th.muted+workflowTimingSummary(chg)+th.reset, listW)...)
	list = append(list, "")
	stageOffset := len(list)
	for i, stage := range stages {
		list = append(list, th.listRow(i == selected-1, s.stageRow(th, stage, listW-1), listW))
	}
	if len(stages) == 0 {
		list = emptyPane(th, "No recorded stages", "Start the agents to see their progress here.")
	}
	tabs := theme.Tabs(agentRunSections, 0, th.bold+th.accent, th.muted, th.reset)
	left := theme.Pane{Title: "Task · stages", Meta: fmt.Sprintf("%d stages", len(stages)), Body: list, Focused: true, Scroll: s.listScroll(stageOffset+selected-1, len(list), listPaneRows(width, rows))}
	right := theme.Pane{Title: tabs, Meta: s.agentStageSelectionLabel(), Body: []string{}}
	if len(stages) > 0 {
		stage := stages[selected-1]
		right.Title = stage.Label + th.muted + "  ·  " + th.reset + tabs
		right.Body = s.stageDetailBody(th, chg, stage, detailW)
		s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(right.Body), detailPaneRows(width, rows))
		right.Scroll = s.DetailScroll
	}
	return s.splitPanes(th, width, rows, tasksSplit, left, right)
}

// stageDetailBody shows what one stage recorded: model, timing, reason and
// the stage's own evidence.
func (s *State) stageDetailBody(th ansiTheme, chg *ChangeSession, stage domain.AgentStage, width int) []string {
	body := []string{s.stageGlyph(th, stage.State) + " " + th.bold + th.fg + stage.Label + th.reset + th.muted + "  " + strings.ReplaceAll(string(stage.State), "_", " ") + th.reset}
	if chg.Workflow != nil && restartNeedsConsent(chg.Workflow.Status) {
		body = append(body, wrapText(th.amber+"▲ "+th.reset+th.fg+"This run cannot resume: restart requires fresh consent and checks."+th.reset, width)...)
	}
	body = append(body, "")
	body = append(body, s.stageFacts(th, chg, stage, width)...)
	section, evidence := s.stageEvidence(th, chg, stage, width)
	body = append(body, "", th.sectionHeader(section, -1))
	for _, line := range evidence {
		body = append(body, wrapText(line, width)...)
	}
	if s.AgentProgress != nil && s.AgentProgress.Status != "current" {
		message := "Native progress unavailable; showing recorded workflow summaries."
		if s.AgentProgress.Status == "not_recorded" {
			message = "No native run in recent history; showing recorded workflow summaries."
		}
		body = append(body, "", th.muted+message+th.reset)
	}
	return body
}

// stageFacts lists the stage's model and budget, timing and recorded reasons.
func (s *State) stageFacts(th ansiTheme, chg *ChangeSession, stage domain.AgentStage, width int) []string {
	const labelW = 10
	var body []string
	if stage.Model != nil && stage.Model.Model != "" {
		body = append(body, th.field("Model", th.fg+stage.Model.Model+th.reset, labelW))
		if stage.Model.Timeout != "" {
			body = append(body, th.field("Budget", th.fg+stage.Model.Timeout+" for the model and retries"+th.reset, labelW))
		}
	}
	if stage.StartedAt != "" {
		body = append(body, th.field("Elapsed", th.fg+workflowElapsed(stage.StartedAt, stage.FinishedAt)+th.reset, labelW))
	}
	if stage.Reason != "" {
		body = append(body, wrapText(th.field("Reason", th.amber+theme.SanitizeSingleLine(stage.Reason)+th.reset, labelW), width)...)
	}
	if chg.Workflow == nil {
		return body
	}
	if reason := strings.TrimSpace(chg.Workflow.Reason); reason != "" {
		body = append(body, wrapText(th.field("Workflow", th.fg+theme.SanitizeSingleLine(reason)+th.reset, labelW), width)...)
	}
	if chg.Workflow.ProbeSummary != "" {
		body = append(body, wrapText(th.field("Probe", th.fg+theme.SanitizeSingleLine(chg.Workflow.ProbeSummary)+th.reset, labelW), width)...)
	}
	return body
}

// stageEvidence names the stage's evidence section and its lines.
func (s *State) stageEvidence(th ansiTheme, chg *ChangeSession, stage domain.AgentStage, width int) (string, []string) {
	switch stage.ID {
	case "create":
		return "Proposal", s.renderStage1CodeDetailLines(th, chg)
	case "generate_tests":
		return stage.Label, []string{"Test generation stays within the captured scope. Verify shows the check results.", s.testingRecoveryText(stage)}
	case "verify":
		return "Verification checks", s.renderAgentCheckDebugLines(th, width)
	case "review":
		return "Model review", s.renderStage3CriticDetailLines(th, chg, string(stage.State))
	case "human_review":
		return "Human review & apply", []string{th.keyHint("d", "review the diff", "a", "apply & commit", "x", "reject", "u", "guarded undo")}
	default:
		return stage.Label, s.specialistEvidence(chg, stage)
	}
}

func (s *State) specialistEvidence(chg *ChangeSession, stage domain.AgentStage) []string {
	if chg.Workflow == nil {
		return []string{"No additional evidence recorded."}
	}
	for _, report := range chg.Workflow.Specialists {
		if "review_"+report.ID != stage.ID {
			continue
		}
		lines := []string{"Model advice · " + report.Evidence}
		if report.Review != nil {
			lines = append(lines, report.Review.Summary)
			for _, finding := range report.Review.Findings {
				lines = append(lines, "• "+finding)
			}
		}
		return lines
	}
	return []string{"No additional evidence recorded."}
}
