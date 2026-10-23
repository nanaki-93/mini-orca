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
	lines := renderPageHeader(th, "TASKS", "Plan → Generate → Verify → Review → Integrate")
	items := s.AgentItems()
	if len(items) == 0 {
		return append(lines, "  No active tasks.", "", "  [n] New task · [b] Issues · :features for ideas")
	}
	selected := s.ClampedIdx()
	titleW := max(12, width-29)
	lines = append(lines, th.renderTableHeader(fmt.Sprintf("  %-12s %-10s %s", "STATUS", "STAGE", "TASK"), width)...)
	start, end := windowBounds(len(items), selected, s.adaptiveWindowRows(7, 12))
	for idx := start; idx < end; idx++ {
		item := items[idx]
		lines = append(lines, fmt.Sprintf("  %s%-12s %-10s %s%s%s", th.renderCursorPrefix(idx == selected),
			theme.TruncatePlain(pipelineAgentStatusLabel(item.Status), 12), pipelineStageShortLabel(item.StageNumber),
			rowTitleStyle(th, idx == selected), theme.TruncatePlain(item.Title, titleW), th.reset))
	}
	lines = appendWindowFooter(th, lines, start, end, len(items))
	return append(lines, "", s.renderAgentNextStepLine(th, items[selected]))
}

func pipelineStageShortLabel(stageNum int) string {
	switch stageNum {
	case 1:
		return "Code"
	case 2:
		return "Test"
	case 3:
		return "AI Review"
	case 4:
		return "Hu review"
	default:
		return "Code"
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

func (s *State) activeAgentStageNumber() int {
	items := s.AgentItems()
	if len(items) > 0 {
		idx := s.ClampedIdx()
		if idx >= 0 && idx < len(items) && items[idx].StageNumber >= 1 && items[idx].StageNumber <= 4 {
			if items[idx].StageNumber == 1 && s.Change != nil && len(s.Change.Checks) > 0 && !isStageOneFailedOrInterrupted(s.Change.Workflow) {
				return 2
			}
			return items[idx].StageNumber
		}
	}
	return 1
}

func isStageOneFailedOrInterrupted(wf *ChangeWorkflow) bool {
	if wf == nil || len(wf.Stages) == 0 {
		return false
	}
	st := wf.Stages[0].Status
	return st == "failed" || st == "interrupted"
}

func (s *State) effectiveAgentDetailStage() int {
	if s.AgentStage >= 1 && s.AgentStage <= 4 {
		return s.AgentStage
	}
	return s.activeAgentStageNumber()
}

func (s *State) renderAgentDebugPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	chg := s.Change
	actions := "[↑/↓] Stage · [d] Diff · [r] Retry · [x] Cancel/reject · [Esc] Back"
	if s.hasRunningWorkflow() {
		actions = "[↑/↓] Stage · [d] Diff · [x] Cancel · [Esc] Back"
	}
	lines := renderPageHeader(th, "TASK · STAGES", actions)
	activeStage := s.activeAgentStageNumber()
	selectedStage := s.effectiveAgentDetailStage()
	for _, row := range theme.WrapVisibleLine(chg.Title, max(10, width-4)) {
		lines = append(lines, "  "+row)
	}
	lines = append(lines, renderPipelineStepper(th, activeStage, chg))
	lines = append(lines, "  "+workflowTimingSummary(chg), "")
	lines = append(lines, fmt.Sprintf("  Selected: %d/4 · %s", selectedStage, resolvePipelineStageStatus(activeStage, chg, selectedStage)))
	lines = append(lines, selectedStageMetadata(th, chg, selectedStage, width)...)
	lines = append(lines, s.renderSelectedAgentStageDetailPanel(th, panelW, chg, activeStage, selectedStage)...)
	return lines
}

func selectedStageMetadata(th ansiTheme, chg *ChangeSession, stage int, width int) []string {
	if chg.Workflow == nil {
		return nil
	}
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	var rows []string
	if stage <= len(chg.Workflow.Stages) {
		entry := chg.Workflow.Stages[stage-1]
		model := "Not recorded"
		if entry.Model != nil {
			model = entry.Model.Model
		}
		rows = renderDiagnosticRow(th, th.cyan, "Model:", th.fg, model+" · elapsed "+workflowElapsed(entry.StartedAt, entry.FinishedAt), contentW)
		if entry.Model != nil && entry.Model.Timeout != "" {
			rows = append(rows, renderDiagnosticRow(th, th.cyan, "Budget:", th.fg, entry.Model.Timeout+" for the model and retries; check deadlines are separate.", contentW)...)
		}
		if entry.Reason != "" {
			rows = append(rows, renderDiagnosticRow(th, th.amber, "Stage reason:", th.fg, entry.Reason, contentW)...)
		}
	}
	if chg.Workflow.Reason != "" {
		rows = append(rows, renderDiagnosticRow(th, th.amber, "Workflow:", th.fg, chg.Workflow.Reason, contentW)...)
	}
	if chg.Workflow.ProbeSummary != "" {
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Probe:", th.fg, chg.Workflow.ProbeSummary, contentW)...)
	}
	return rows
}

func (s *State) renderSelectedAgentStageDetailPanel(th ansiTheme, panelW int, chg *ChangeSession, activeStage, selectedStage int) []string {
	touched := domain.EffectiveSessionChanges(chg)
	switch selectedStage {
	case 1:
		return th.renderPanelBox("STAGE 1 · ARCHITECT & CODE", fmt.Sprintf("%d FILES", len(touched)), panelW, s.renderStage1CodeDetailLines(th, chg))
	case 2:
		return th.renderPanelBox("VERIFICATION CHECKS & DIAGNOSTIC OUTPUT", fmt.Sprintf("%d CHECKS", len(chg.Checks)), panelW, s.renderAgentCheckDebugLines(th, panelW))
	case 3:
		meta := strings.ToUpper(resolvePipelineStageStatus(activeStage, chg, 3))
		if chg.Workflow != nil && chg.Workflow.Review != nil && chg.Workflow.Review.Verdict != "" {
			meta = strings.ToUpper(chg.Workflow.Review.Verdict)
		}
		return th.renderPanelBox("STAGE 3 · CRITIC REVIEW VERDICT", meta, panelW, s.renderStage3CriticDetailLines(th, chg, activeStage))
	default:
		meta := normalizePipelineStageBadge(4, resolvePipelineStageStatus(activeStage, chg, 4), chg)
		return th.renderPanelBox("STAGE 4 · HUMAN REVIEW & APPLY GATE", meta, panelW, s.renderStage4HumanReviewDetailLines(th, chg, activeStage))
	}
}

func (s *State) renderStage1CodeDetailLines(th ansiTheme, chg *ChangeSession) []string {
	touched := domain.EffectiveSessionChanges(chg)
	if len(touched) == 0 {
		if s.hasRunningWorkflow() {
			return []string{fmt.Sprintf("%sGenerating code proposal for target files...%s", th.amber, th.reset)}
		}
		return []string{fmt.Sprintf("%sStage 1 (Architect & Code) produced no proposal changes — inspect workflow reason above and press [r] to retry.%s", th.amber, th.reset)}
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

func (s *State) renderStage3CriticDetailLines(th ansiTheme, chg *ChangeSession, activeStage int) []string {
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
	stStatus := resolvePipelineStageStatus(activeStage, chg, 3)
	return []string{
		fmt.Sprintf("%sStatus:%s %s  %sCritic review verdict not recorded yet.%s",
			th.muted, th.reset, th.statusBadge(stStatus), th.muted, th.reset),
	}
}

func (s *State) renderStage4HumanReviewDetailLines(th ansiTheme, chg *ChangeSession, activeStage int) []string {
	stStatus := normalizePipelineStageBadge(4, resolvePipelineStageStatus(activeStage, chg, 4), chg)
	touched := domain.EffectiveSessionChanges(chg)
	added, removed, badge := domain.ClassifyDiffScope(touched)
	return []string{
		fmt.Sprintf("%sGate:%s    %s   %sScope:%s %s   %sDelta:%s %s+%d%s/%s-%d%s (%d files)",
			th.muted, th.reset, th.statusBadge(stStatus),
			th.muted, th.reset, th.statusBadge(badge),
			th.muted, th.reset, th.green, added, th.reset, th.red, removed, th.reset, len(touched)),
		fmt.Sprintf("%sActions:%s %s[d]%s Inspect Diff · %s[a/c]%s Apply & Commit · %s[x]%s Reject Fix · %s[u]%s Guarded Undo",
			th.muted, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset),
	}
}

func (s *State) renderAgentCheckDebugLines(th ansiTheme, _ int) []string {
	if s.Change == nil || len(s.Change.Checks) == 0 {
		if s.hasRunningWorkflow() {
			return []string{fmt.Sprintf("%sStage 2 generates complete test files, then runs verification. No check results recorded yet.%s", th.muted, th.reset)}
		}
		if s.Change != nil && len(s.Change.Changes) == 0 {
			return []string{fmt.Sprintf("%sStage 1 (Architect & Code) produced no proposal changes — inspect workflow reason above and press [r] to retry.%s", th.amber, th.reset)}
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

func normalizePipelineStageBadge(stageNum int, rawStatus string, chg *ChangeSession) string {
	st := strings.ToLower(strings.TrimSpace(rawStatus))
	if stageNum == 4 && (st == "waiting" || st == "awaiting_human_review") {
		if chg != nil && (chg.State == "ready" || chg.State == "checked" || (chg.Workflow != nil && chg.Workflow.Status == "awaiting_human_review")) {
			return "READY"
		}
	}
	if st == "" {
		return "PENDING"
	}
	return strings.ToUpper(st)
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

func renderPipelineStepper(th ansiTheme, activeStage int, chg *ChangeSession) string {
	stages := []struct {
		num   int
		label string
	}{
		{1, "1. Architect"},
		{2, "2. Verify"},
		{3, "3. Critic"},
		{4, "4. Review"},
	}
	var parts []string
	for _, st := range stages {
		stStatus := resolvePipelineStageStatus(activeStage, chg, st.num)
		parts = append(parts, formatPipelineStepperNode(th, st.label, stStatus, st.num == activeStage))
	}
	return strings.Join(parts, th.border+" ──▶ "+th.reset)
}

func resolvePipelineStageStatus(activeStage int, chg *ChangeSession, stageNum int) string {
	if activeStage <= 0 {
		return "idle"
	}
	if fromWF := stageStatusFromWorkflow(chg, stageNum); fromWF != "" {
		return fromWF
	}
	return stageStatusFallback(activeStage, chg, stageNum)
}

func stageStatusFromWorkflow(chg *ChangeSession, stageNum int) string {
	if chg == nil || chg.Workflow == nil || stageNum-1 >= len(chg.Workflow.Stages) {
		return ""
	}
	raw := strings.ToLower(strings.TrimSpace(chg.Workflow.Stages[stageNum-1].Status))
	if stageNum == 4 && isChangeReadyForReview(chg, raw) {
		return "ready"
	}
	return raw
}

func stageStatusFallback(activeStage int, chg *ChangeSession, stageNum int) string {
	if stageNum < activeStage {
		return "completed"
	}
	if stageNum > activeStage {
		return "pending"
	}
	if isChangeFailedOrRejected(chg) {
		return "failed"
	}
	if stageNum == 4 || isChangeReadyForReview(chg, "") {
		return "ready"
	}
	return "running"
}

func isChangeReadyForReview(chg *ChangeSession, raw string) bool {
	if raw == "waiting" || raw == "awaiting_human_review" {
		return true
	}
	if chg == nil {
		return false
	}
	if chg.State == "ready" || chg.State == "checked" {
		return true
	}
	return chg.Workflow != nil && chg.Workflow.Status == "awaiting_human_review"
}

func isChangeFailedOrRejected(chg *ChangeSession) bool {
	if chg == nil {
		return false
	}
	if chg.State == "failed" {
		return true
	}
	return chg.Workflow != nil && (chg.Workflow.Status == "failed" || chg.Workflow.Status == "changes_requested")
}

func formatPipelineStepperNode(th ansiTheme, label, stStatus string, isActive bool) string {
	switch strings.ToLower(strings.TrimSpace(stStatus)) {
	case "completed", "passed", "approve", "approved", "applied":
		if isActive {
			return fmt.Sprintf("%s %s %s", th.okBg, label, th.reset)
		}
		return fmt.Sprintf("%s[%s]%s", th.green, label, th.reset)
	case "ready", "checked", "awaiting_human_review":
		return fmt.Sprintf("%s %s %s", th.okBg, label, th.reset)
	case "failed", "error", "changes_requested", "rejected", "stale", "interrupted":
		return fmt.Sprintf("%s %s %s", th.critBg, label, th.reset)
	case "running", "working", "in_progress", "waiting", "canceling":
		return fmt.Sprintf("%s %s %s", th.warnBg, label, th.reset)
	default:
		return fmt.Sprintf("%s[%s]%s", th.muted, label, th.reset)
	}
}

func (s *State) renderEditorPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	subRight := "Read-only repository inspector"
	if s.isViewingDiff() {
		subRight = diffModeDisplayName(s.EffectiveDiffMode())
	} else if !s.BrowseFiles && s.File != nil {
		subRight = theme.SanitizeCodeText(s.File.Path)
	}
	lines := renderPageHeader(th, "FILES & DIFF", subRight)
	if s.isViewingDiff() {
		lines = append(lines, s.renderChangeDiffLines(th, width)...)
		return lines
	}
	if !s.BrowseFiles && s.File != nil {
		header := fmt.Sprintf("%sFile:%s %s%s%s %s(%s · %d lines)%s",
			th.muted, th.reset, th.bold+th.fg, theme.SanitizeCodeText(s.File.Path), th.reset,
			th.muted, client.FirstNonEmpty(s.File.Language, "text"), s.File.LineCount, th.reset)
		lines = append(lines, th.renderPanelBox("READ-ONLY SOURCE VIEWER", "SOURCE", panelW, []string{header})...)
		if s.File.Content != "" {
			rawLines := strings.Split(strings.ReplaceAll(s.File.Content, "\r\n", "\n"), "\n")
			codeCol := max(16, width-14)
			start, end := windowBounds(len(rawLines), s.ClampedIdx(), s.adaptiveWindowRows(10, 14))
			for idx := start; idx < end; idx++ {
				codeText := theme.TruncatePlain(shiftedCode(rawLines[idx], s.DiffOffset), codeCol)
				lines = append(lines, fmt.Sprintf("  %s%s%4d │%s %s", th.renderCursorPrefix(idx == s.ClampedIdx()), th.muted, idx+1, th.reset, codeText))
			}
			lines = appendWindowFooter(th, lines, start, end, len(rawLines))
		}
		return lines
	}
	return s.renderIndexedFilesInEditor(th, width, lines)
}

func (s *State) renderIndexedFilesInEditor(th ansiTheme, width int, lines []string) []string {
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, fmt.Sprintf("  %sNo file or diff loaded.%s", th.muted, th.reset))
	}
	lines = append(lines, th.renderTableHeader("   REPOSITORY FILE PATH                        LANGUAGE      LINES", width)...)
	selected := s.ClampedIdx()
	total := len(s.Index.Files)
	start, end := windowBounds(total, selected, s.adaptiveWindowRows(8, 14))
	for idx := start; idx < end; idx++ {
		f := s.Index.Files[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%s%-40s%s %s(%s)  %d lines%s",
			cursor, tStyle, f.Path, th.reset, th.muted, client.FirstNonEmpty(f.Language, "text"), f.LineCount, th.reset))
	}
	return appendWindowFooter(th, lines, start, end, total)
}

func (s *State) renderGitPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "WORKTREES", s.worktreeRecoveryHint())
	if s.GitWorkspace == nil || len(s.GitWorkspace.Worktrees) == 0 {
		return append(lines, "  No Git worktrees reported.")
	}
	lines = append(lines, th.renderTableHeader("    STATUS        BRANCH", width)...)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(s.GitWorkspace.Worktrees), selected, s.adaptiveWindowRows(5, 14))
	for idx := start; idx < end; idx++ {
		wt := s.GitWorkspace.Worktrees[idx]
		lines = append(lines, fmt.Sprintf("  %s%-13s %s%s%s", th.renderCursorPrefix(idx == selected), worktreeStatus(wt),
			rowTitleStyle(th, idx == selected), theme.TruncatePlain(wt.Branch, max(10, width-20)), th.reset))
	}
	return appendWindowFooter(th, lines, start, end, len(s.GitWorkspace.Worktrees))
}

func (s *State) renderHistoryPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "PROPOSAL HISTORY", "Guarded change history & audit trail")
	if len(s.ChangeHistory) == 0 {
		return append(lines, fmt.Sprintf("  %sNo saved proposals in history.%s", th.muted, th.reset))
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
