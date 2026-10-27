package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) renderAgentsPage(th ansiTheme, width int) []string {
	if s.AgentDetailOpen && s.Change != nil {
		return s.renderAgentDebugPage(th, width)
	}
	lines := renderPageHeader(th, "Tasks", "Plans, checked proposals and human review")
	lines = append(lines, s.renderAgentFleetStrip(th))
	items := s.AgentItems()
	if len(items) == 0 {
		lines = append(lines, "", "  No active tasks.", "  [n] New task · [b] Issues · :features for ideas")
		return lines
	}
	selected := s.ClampedIdx()
	titleW := max(12, width-33)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader(fmt.Sprintf("  %-12s %-14s %s", "STATUS", "STAGE", "TASK"), width)...)
	start, end := windowBounds(len(items), selected, s.adaptiveWindowRows(13, 10))
	for idx := start; idx < end; idx++ {
		item := items[idx]
		stColor := agentStatusTextStyle(th, item.Status)
		lines = append(lines, fmt.Sprintf("  %s%s%-12s%s %s%-14s%s %s%s%s",
			th.renderCursorPrefix(idx == selected),
			stColor, theme.TruncatePlain(pipelineAgentStatusLabel(item.Status), 12), th.reset,
			th.cyan, theme.TruncatePlain(item.Stage, 14), th.reset,
			rowTitleStyle(th, idx == selected), theme.TruncatePlain(item.Title, titleW), th.reset))
	}
	lines = appendWindowFooter(th, lines, start, end, len(items))
	if s.viewportBodyRows <= 0 || s.viewportBodyRows >= 20 {
		lines = append(lines, "")
		return append(lines, s.renderSelectedAgentOverviewBox(th, adaptivePanelWidth(width), items[selected])...)
	}
	return append(lines, "", s.renderAgentNextStepLine(th, items[selected]))
}

func (s *State) renderAgentFleetStrip(th ansiTheme) string {
	total, running, review, failed, toMerge := s.agentFleetBreakdown()
	return fmt.Sprintf("  %sTasks:%s %s%d active%s · %s%d running%s · %s%d review%s · %s%d attention%s · %s%d to merge%s",
		th.bold+th.purple, th.reset,
		th.bold+th.fg, total, th.reset,
		th.amber, running, th.reset,
		th.green, review, th.reset,
		th.red, failed, th.reset,
		th.cyan, toMerge, th.reset)
}

func (s *State) renderSelectedAgentOverviewBox(th ansiTheme, panelW int, item AgentItem) []string {
	scopeBadge := client.FirstNonEmpty(item.ScopeBadge, "PENDING")
	targetPath := client.FirstNonEmpty(item.Path, "workspace")
	body := []string{
		"Stage: " + item.Stage,
		fmt.Sprintf("%sBranch:%s %s⎇ %s%s · %sTarget:%s %s%s%s · %sScope:%s %s (%s+%d%s/%s-%d%s) · %sChecks:%s %s%d/%d%s",
			th.muted, th.reset, th.purple, client.FirstNonEmpty(item.Branch, "main"), th.reset,
			th.muted, th.reset, th.cyan, targetPath, th.reset,
			th.muted, th.reset, th.statusBadge(scopeBadge), th.green, item.AddedLines, th.reset, th.red, item.RemovedLines, th.reset,
			th.muted, th.reset, th.bold+th.fg, item.ChecksPassed, item.ChecksTotal, th.reset),
		strings.TrimLeft(s.renderAgentNextStepLine(th, item), " "),
	}
	return th.renderPanelBox("SELECTED TASK", strings.ToUpper(item.Status), panelW, body)
}

func agentStatusTextStyle(th ansiTheme, rawStatus string) string {
	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case "ready", "checked", "done", "applied", "passed", "completed":
		return th.bold + th.green
	case "failed", "error", "changes_requested", "rejected", "interrupted":
		return th.bold + th.red
	case "running", "queued", "working", "in_progress", "canceling", "stale":
		return th.bold + th.amber
	default:
		return th.fg
	}
}

func pipelineAgentStatusLabel(rawStatus string) string {
	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case "running", "queued", "working", "in_progress", "canceling":
		return strings.ToUpper(rawStatus)
	case "changes_requested":
		return "NEEDS REVIEW"
	case "":
		return "UNKNOWN"
	default:
		return strings.ToUpper(rawStatus)
	}
}

func formatCheckOutputLines(th ansiTheme, output string, maxLines int) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	rawLines := strings.Split(strings.TrimSpace(strings.ReplaceAll(output, "\r\n", "\n")), "\n")
	limit := min(maxLines, len(rawLines))
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		clean := strings.TrimSpace(theme.SanitizeCodeText(rawLines[i]))
		out = append(out, fmt.Sprintf("    %s│%s %s%s%s", th.red, th.reset, th.amber, clean, th.reset))
	}
	return out
}

func (s *State) renderAgentDebugPage(th ansiTheme, width int) []string {
	if s.AgentRunSection > 0 && s.AgentRunSection < len(agentRunSections) {
		return s.renderAgentRunPage(th, width)
	}
	panelW := adaptivePanelWidth(width)
	chg := s.Change
	actions := "[←/→] Overview · [↑/↓] Stage · [d] Diff · [r] Retry · [x] Cancel/reject"
	if s.hasRunningWorkflow() {
		actions = "[←/→] Overview · [↑/↓] Stage · [d] Diff · [x] Cancel"
	}
	if chg.MissionOrigin != nil {
		actions = "[←/→] Execution detail · [d] Diff · :mission-execution for owner controls"
	}
	lines := renderPageHeader(th, "Task · stages", actions)
	selectedStage := s.effectiveAgentDetailStage()
	stages := s.currentAgentStages()
	if s.viewportBodyRows > 0 && s.viewportBodyRows < 14 {
		lines = []string{"  " + th.bold + "Task · stages" + th.reset, "  " + theme.TruncatePlain(chg.Title, max(10, width-4))}
		lines = appendWrappedText(lines, workflowTimingSummary(chg), width)
	} else {
		for _, row := range theme.WrapVisibleLine(chg.Title, max(10, width-4)) {
			lines = append(lines, "  "+row)
		}
		lines = append(lines, "  "+workflowTimingSummary(chg), "")
	}
	lines = append(lines, s.renderRecordedStages(th, width, stages, selectedStage)...)
	lines = append(lines, "", "  "+s.agentStageSelectionLabel())
	if len(stages) == 0 {
		return lines
	}
	stage := stages[selectedStage-1]
	lines = append(lines, selectedStageMetadata(th, chg, stage, width)...)
	lines = append(lines, s.renderSelectedAgentStageDetailPanel(th, panelW, chg, stage)...)
	if s.AgentProgress != nil && s.AgentProgress.Status != "current" {
		message := "Native progress unavailable; showing recorded workflow summaries."
		if s.AgentProgress.Status == "not_recorded" {
			message = "No native run in recent history; showing recorded workflow summaries."
		}
		lines = appendWrappedText(append(lines, ""), message, width)
	}
	return lines
}

func (s *State) renderRecordedStages(th ansiTheme, width int, stages []domain.AgentStage, selected int) []string {
	rows := min(5, max(1, s.viewportBodyRows-12))
	if s.viewportBodyRows == 0 {
		rows = 5
	}
	start, end := windowBounds(len(stages), selected-1, rows)
	var lines []string
	for i := start; i < end; i++ {
		stage := stages[i]
		label := stage.Label
		if stage.Parallel {
			label = "↳ " + label + " · parallel"
		}
		lines = append(lines, fmt.Sprintf("  %s%s%-12s%s %s", th.renderCursorPrefix(i == selected-1), agentStatusTextStyle(th, string(stage.State)), strings.ToUpper(string(stage.State)), th.reset, theme.TruncatePlain(label, max(12, width-19))))
	}
	return appendWindowFooter(th, lines, start, end, len(stages))
}

func selectedStageMetadata(th ansiTheme, chg *ChangeSession, stage domain.AgentStage, width int) []string {
	if chg.Workflow == nil {
		return nil
	}
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	model := "Not recorded"
	if stage.Model != nil {
		model = stage.Model.Model
	}
	rows := renderDiagnosticRow(th, th.cyan, "Model:", th.fg, model+" · elapsed "+workflowElapsed(stage.StartedAt, stage.FinishedAt), contentW)
	if stage.Model != nil && stage.Model.Timeout != "" {
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Budget:", th.fg, stage.Model.Timeout+" for the model and retries; check deadlines are separate.", contentW)...)
	}
	if stage.Reason != "" {
		rows = append(rows, renderDiagnosticRow(th, th.amber, "Stage reason:", th.fg, stage.Reason, contentW)...)
	}
	if chg.Workflow.Reason != "" {
		rows = append(rows, renderDiagnosticRow(th, th.amber, "Workflow:", th.fg, chg.Workflow.Reason, contentW)...)
	}
	if chg.Workflow.ProbeSummary != "" {
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Probe:", th.fg, chg.Workflow.ProbeSummary, contentW)...)
	}
	return rows
}

func (s *State) renderSelectedAgentStageDetailPanel(th ansiTheme, panelW int, chg *ChangeSession, stage domain.AgentStage) []string {
	touched := domain.EffectiveSessionChanges(chg)
	switch stage.ID {
	case "create":
		return th.renderPanelBox("PROPOSAL", fmt.Sprintf("%d FILES", len(touched)), panelW, s.renderStage1CodeDetailLines(th, chg))
	case "generate_tests":
		return th.renderPanelBox(strings.ToUpper(stage.Label), strings.ToUpper(string(stage.State)), panelW, []string{"Test generation stays within the captured scope. Inspect Verify for check results.", s.testingRecoveryText(stage)})
	case "verify":
		return th.renderPanelBox("VERIFICATION CHECKS", fmt.Sprintf("%d CHECKS", len(chg.Checks)), panelW, s.renderAgentCheckDebugLines(th, panelW))
	case "review":
		meta := strings.ToUpper(string(stage.State))
		if chg.Workflow != nil && chg.Workflow.Review != nil && chg.Workflow.Review.Verdict != "" {
			meta = strings.ToUpper(chg.Workflow.Review.Verdict)
		}
		return th.renderPanelBox("MODEL REVIEW", meta, panelW, s.renderStage3CriticDetailLines(th, chg, string(stage.State)))
	case "human_review":
		return th.renderPanelBox("HUMAN REVIEW & APPLY", strings.ToUpper(string(stage.State)), panelW, s.renderStage4HumanReviewDetailLines(th, chg, string(stage.State)))
	default:
		return s.renderSpecialistStage(th, panelW, chg, stage)
	}
}

func (s *State) testingRecoveryText(stage domain.AgentStage) string {
	if stage.State == domain.StageRunning {
		return "Testing is in progress; no completed verification is implied."
	}
	if !s.hasRunningWorkflow() && stage.State == domain.StageFailed {
		return "Use :retry test to reuse the saved proposal."
	}
	return "Generation and verification have separate recorded outcomes."
}

func (s *State) renderSpecialistStage(th ansiTheme, width int, chg *ChangeSession, stage domain.AgentStage) []string {
	lines := []string{"No additional evidence recorded."}
	if chg.Workflow != nil {
		for _, report := range chg.Workflow.Specialists {
			if "review_"+report.ID != stage.ID {
				continue
			}
			lines = []string{"Model advice · " + report.Evidence}
			if report.Review != nil {
				lines = append(lines, report.Review.Summary)
				lines = append(lines, report.Review.Findings...)
			}
		}
	}
	return th.renderPanelBox(strings.ToUpper(stage.Label), strings.ToUpper(string(stage.State)), width, lines)
}

func (s *State) renderStage1CodeDetailLines(th ansiTheme, chg *ChangeSession) []string {
	touched := domain.EffectiveSessionChanges(chg)
	if len(touched) == 0 {
		if s.hasRunningWorkflow() {
			return []string{fmt.Sprintf("%sGenerating code proposal for target files...%s", th.amber, th.reset)}
		}
		return []string{fmt.Sprintf("%sNo proposal changes recorded. Inspect the workflow reason before retrying.%s", th.amber, th.reset)}
	}
	added, removed, badge := domain.ClassifyDiffScope(touched)
	out := []string{
		fmt.Sprintf("%sScope:%s %s   %sDelta:%s %s+%d%s/%s-%d%s across %d file(s)",
			th.muted, th.reset, th.statusBadge(badge),
			th.muted, th.reset, th.green, added, th.reset, th.red, removed, th.reset, len(touched)),
	}
	for _, fc := range touched {
		if strings.TrimSpace(fc.Summary) != "" {
			out = append(out, fmt.Sprintf("  %s•%s %s%s%s — %s", th.cyan, th.reset, th.bold+th.fg, fc.Path, th.reset, strings.TrimSpace(fc.Summary)))
		} else {
			out = append(out, fmt.Sprintf("  %s•%s %s%s%s", th.cyan, th.reset, th.bold+th.fg, fc.Path, th.reset))
		}
	}
	return out
}

func (s *State) renderStage3CriticDetailLines(th ansiTheme, chg *ChangeSession, status string) []string {
	if chg != nil && chg.Workflow != nil && chg.Workflow.Review != nil {
		rev := chg.Workflow.Review
		revLines := []string{
			fmt.Sprintf("%sVerdict:%s %s  %s%s%s", th.muted, th.reset, th.statusBadge(rev.Verdict), th.fg, rev.Summary, th.reset),
		}
		for _, f := range rev.Findings {
			revLines = append(revLines, fmt.Sprintf("  %s•%s %s", th.amber, th.reset, f))
		}
		return revLines
	}
	return []string{
		fmt.Sprintf("%sStatus:%s %s  %sCritic review verdict not recorded yet.%s",
			th.muted, th.reset, th.statusBadge(status), th.muted, th.reset),
	}
}

func (s *State) renderStage4HumanReviewDetailLines(th ansiTheme, chg *ChangeSession, status string) []string {
	touched := domain.EffectiveSessionChanges(chg)
	added, removed, badge := domain.ClassifyDiffScope(touched)
	return []string{
		fmt.Sprintf("%sGate:%s    %s   %sScope:%s %s   %sDelta:%s %s+%d%s/%s-%d%s (%d files)",
			th.muted, th.reset, th.statusBadge(status),
			th.muted, th.reset, th.statusBadge(badge),
			th.muted, th.reset, th.green, added, th.reset, th.red, removed, th.reset, len(touched)),
		fmt.Sprintf("%sActions:%s %s[d]%s Inspect Diff · %s[a/c]%s Apply & Commit · %s[x]%s Reject Fix · %s[u]%s Guarded Undo",
			th.muted, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset),
	}
}

func (s *State) renderAgentCheckDebugLines(th ansiTheme, _ int) []string {
	if s.Change == nil || len(s.Change.Checks) == 0 {
		if s.hasRunningWorkflow() {
			return []string{fmt.Sprintf("%sNo check results recorded yet. Test generation and verification are separate stages.%s", th.muted, th.reset)}
		}
		if s.Change != nil && len(s.Change.Changes) == 0 {
			return []string{fmt.Sprintf("%sNo proposal changes recorded. Inspect the workflow reason before retrying.%s", th.amber, th.reset)}
		}
		if s.Change != nil && resumableWorkflowStage(s.Change) == "test" {
			return []string{fmt.Sprintf("%sNo check results recorded. Press [r] to retry Testing using the saved Creation proposal.%s", th.amber, th.reset)}
		}
		return []string{fmt.Sprintf("%sNo deterministic verification checks recorded yet — press [r] to run checks.%s", th.muted, th.reset)}
	}
	var out []string
	for _, chk := range s.Change.Checks {
		cmdStr := strings.Join(chk.Command, " ")
		out = append(out, fmt.Sprintf("%s  %s%-18s%s (exit %d)  %s%s%s",
			th.statusBadge(chk.State), th.bold+th.fg, chk.Name, th.reset, chk.ExitCode, th.muted, cmdStr, th.reset))
		if note := baselineOutcomeLabel(chk); note != "" {
			out = append(out, note)
		}
		elapsed := "not recorded"
		if chk.DurationMS > 0 {
			elapsed = fmt.Sprintf("%dms", chk.DurationMS)
		}
		out = append(out, "    Duration: "+elapsed)
		out = append(out, formatCheckOutputLines(th, chk.Output, len(strings.Split(chk.Output, "\n")))...)
	}
	return out
}

func (s *State) renderAgentNextStepLine(th ansiTheme, item AgentItem) string {
	next := "[Enter] Inspect task · [d] Review diff"
	switch strings.ToLower(item.Status) {
	case "stale":
		next = "Source changed · [r] Review a fresh plan"
	case "running", "queued", "canceling":
		next = "[Enter] Inspect progress · [x] Cancel"
	case "failed", "changes_requested", "interrupted":
		next = "[Enter] Inspect failure · :retry test or :retry review"
	case "ready", "checked", "reviewed":
		next = "[d] Review changed files before Apply · [x] Reject"
	}
	return "  " + th.cyan + next + th.reset
}

func (s *State) renderHistoryPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "Proposal history", "Applied and rejected proposals with their audit trail")
	if len(s.ChangeHistory) == 0 {
		return append(lines, renderEmptyState(th, "No proposals in history",
			"Applied and rejected proposals are kept here with their commit and audit trail.",
			"Start one with [2] Tasks or :agent <request>.")...)
	}
	lines = append(lines, th.renderTableHeader("    STATE         COMMIT CODE    PROPOSAL TITLE                   REVISION", width)...)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(s.ChangeHistory), selected, s.adaptiveWindowRows(12, 10))
	for idx := start; idx < end; idx++ {
		entry := s.ChangeHistory[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		tStyle := rowTitleStyle(th, idx == selected)
		code := domain.ResolveCommitCode(entry.ID, entry.Hash, entry.FindingTargets, entry.Commit)
		if code == "" {
			code = "—"
		}
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %s%-13s%s  %s%-32s%s %srev:%d (%s)%s",
			cursor, th.padBadge(entry.State, 12),
			th.purple, code, th.reset,
			tStyle, theme.TruncatePlain(entry.Title, 32), th.reset,
			th.muted, entry.Revision, client.FirstNonEmpty(entry.Freshness, "current"), th.reset,
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(s.ChangeHistory))
	if selected >= 0 && selected < len(s.ChangeHistory) {
		entry := s.ChangeHistory[selected]
		code := client.FirstNonEmpty(domain.ResolveCommitCode(entry.ID, entry.Hash, entry.FindingTargets, entry.Commit), "—")
		oid := "uncommitted"
		if entry.Commit != nil && entry.Commit.OID != "" {
			oid = theme.TruncatePlain(entry.Commit.OID, 12)
		}
		var targetLabels []string
		for _, ft := range entry.FindingTargets {
			targetLabels = append(targetLabels, fmt.Sprintf("%s:%s (%s)", ft.Kind, ft.ID, ft.Path))
		}
		linked := "None"
		if len(targetLabels) > 0 {
			linked = strings.Join(targetLabels, ", ")
		}
		detail := []string{
			fmt.Sprintf("%sProposal:%s      %s%s%s · %sCode:%s %s%s%s · %sCommit:%s %s%s%s",
				th.muted, th.reset, th.bold+th.fg, entry.Title, th.reset,
				th.muted, th.reset, th.bold+th.purple, code, th.reset,
				th.muted, th.reset, th.cyan, oid, th.reset),
			fmt.Sprintf("%sLinked Issues:%s %s%s%s",
				th.muted, th.reset, th.green, linked, th.reset),
		}
		lines = append(lines, "")
		lines = append(lines, th.renderPanelBox("SELECTED PROPOSAL & ISSUE AUDIT TRAIL", strings.ToUpper(entry.State), panelW, detail)...)
	}
	return lines
}
