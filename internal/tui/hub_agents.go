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
	panelW := adaptivePanelWidth(width)
	items := s.AgentItems()
	activeStage := 0
	if len(items) > 0 {
		activeStage = items[s.ClampedIdx()].StageNumber
	}
	pipelineLines := []string{renderPipelineStepper(th, activeStage, s.Change)}
	if len(items) > 0 {
		if stageSummary := s.renderWorkflowStagesLine(th); stageSummary != "" {
			pipelineLines = append(pipelineLines, stageSummary)
		}
	}
	lines := renderPageHeader(th, "[2] AGENT STUDIO · 4-STAGE AUTONOMOUS PIPELINE", "Guarded code generation & verification")
	lines = append(lines, th.renderPanelBox("GUARDED EXECUTION PIPELINE", "4-STAGE WORKFLOW", panelW, pipelineLines)...)
	if len(items) == 0 {
		guide := []string{
			fmt.Sprintf("%sStatus:%s No active autonomous missions running.", th.bold+th.cyan, th.reset),
			fmt.Sprintf("• %sIssue Remediation:%s Press %s[b]%s or open %sIssues%s and press %s[Enter/a]%s on a finding.", th.bold+th.fg, th.reset, th.cyan, th.reset, th.accent, th.reset, th.cyan, th.reset),
			fmt.Sprintf("• %sFeature Delivery:%s  Select a proposal in %sFeatures%s or run %s:agent <goal>%s.", th.bold+th.fg, th.reset, th.accent, th.reset, th.purple, th.reset),
		}
		lines = append(lines, "")
		return append(lines, th.renderPanelBox("AUTONOMOUS MISSION STANDBY", "IDLE", panelW, guide)...)
	}
	lines = append(lines, "")
	selected := s.ClampedIdx()
	titleCol := max(16, width-60)
	headerCols := fmt.Sprintf("    %-16s  %-*s  %-12s  %-10s  %s", "STAGE", titleCol, "MISSION TITLE", "SCOPE", "DELTA", "CHECKS")
	lines = append(lines, th.renderTableHeader(headerCols, width)...)
	start, end := windowBounds(len(items), selected, 5)
	for idx := start; idx < end; idx++ {
		item := items[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		stageLabel := theme.TruncatePlain(fmt.Sprintf("%d. %s", item.StageNumber, item.Stage), 16)
		tStyle := rowTitleStyle(th, idx == selected)
		deltaCol := theme.PadVisibleRight(fmt.Sprintf("%s+%d%s/%s-%d%s", th.green, item.AddedLines, th.reset, th.red, item.RemovedLines, th.reset), 10)
		lines = append(lines, fmt.Sprintf(
			"  %s%-16s  %s%-*s%s  %s  %s  %schecks:%d/%d%s",
			cursor, stageLabel, tStyle, titleCol, theme.TruncatePlain(item.Title, titleCol), th.reset,
			th.padBadge(item.ScopeBadge, 12),
			deltaCol,
			th.muted, item.ChecksPassed, item.ChecksTotal, th.reset,
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(items))
	curItem := items[selected]
	detail := []string{
		fmt.Sprintf("%sMission:%s   %s%s%s", th.muted, th.reset, th.bold+th.fg, curItem.Title, th.reset),
		fmt.Sprintf("%sScope:%s     %s%s%s   %sBranch:%s %s%s%s   %sStatus:%s %s   %sChecks:%s %d/%d",
			th.muted, th.reset, th.cyan, client.FirstNonEmpty(curItem.Path, "workspace"), th.reset,
			th.muted, th.reset, th.purple, curItem.Branch, th.reset,
			th.muted, th.reset, th.statusBadge(curItem.Status),
			th.muted, th.reset, curItem.ChecksPassed, curItem.ChecksTotal),
	}
	if errLines := s.renderAgentErrorDiagnosticsLines(th); len(errLines) > 0 {
		detail = append(detail, errLines...)
	} else {
		detail = append(detail, s.renderAgentTelemetryLine(th, curItem, max(24, panelW-18)))
	}
	detail = append(detail, s.renderAgentNextStepLine(th, curItem))
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("ACTIVE AGENT MISSION CONTROL", strings.ToUpper(curItem.Stage), panelW, detail)...)
	return lines
}

func (s *State) hasWorkflowErrorOrRejection() bool {
	if strings.TrimSpace(s.Error) != "" {
		return true
	}
	chg := s.Change
	if chg == nil {
		return false
	}
	if chg.State == "failed" || chg.Freshness == "stale" {
		return true
	}
	if chg.Workflow != nil {
		st := chg.Workflow.Status
		if st == "failed" || st == "changes_requested" || st == "interrupted" || st == "stale" || strings.TrimSpace(chg.Workflow.Reason) != "" {
			return true
		}
	}
	for _, chk := range chg.Checks {
		if !strings.EqualFold(chk.State, "passed") {
			return true
		}
	}
	return false
}

func (s *State) renderAgentErrorDiagnosticsLines(th ansiTheme) []string {
	if !s.hasWorkflowErrorOrRejection() {
		return nil
	}
	var out []string
	if strings.TrimSpace(s.Error) != "" {
		out = append(out, fmt.Sprintf("%sError:%s     %s▲ %s%s", th.muted, th.reset, th.bold+th.red, strings.TrimSpace(s.Error), th.reset))
	}
	chg := s.Change
	if chg == nil {
		return out
	}
	out = append(out, s.collectWorkflowStageErrorLines(th, chg)...)
	out = append(out, s.collectFailedCheckSummaryLines(th, chg, 2)...)
	if chg.Workflow != nil && chg.Workflow.Review != nil && chg.Workflow.Review.Verdict == "changes_requested" {
		out = append(out, fmt.Sprintf("%sCritic:%s    %s  %s%s%s",
			th.muted, th.reset, th.statusBadge(chg.Workflow.Review.Verdict), th.fg, chg.Workflow.Review.Summary, th.reset))
		for _, f := range chg.Workflow.Review.Findings {
			out = append(out, fmt.Sprintf("  %s•%s %s", th.amber, th.reset, f))
		}
	}
	if chg.Workflow != nil && strings.TrimSpace(chg.Workflow.ProbeSummary) != "" {
		out = append(out, fmt.Sprintf("%sProbe:%s     %s%s%s", th.muted, th.reset, th.cyan, strings.TrimSpace(chg.Workflow.ProbeSummary), th.reset))
	}
	return out
}

func (s *State) collectWorkflowStageErrorLines(th ansiTheme, chg *ChangeSession) []string {
	if chg.Workflow == nil {
		return nil
	}
	var out []string
	hasStageReason := false
	for idx, st := range chg.Workflow.Stages {
		if st.Status != "failed" && st.Status != "interrupted" && st.Status != "stale" && strings.TrimSpace(st.Reason) == "" {
			continue
		}
		stageLine := fmt.Sprintf("%sStage:%s     %s%d. %s%s %s",
			th.muted, th.reset, th.bold+th.fg, idx+1, domain.CanonicalStageLabel(idx, st.Name), th.reset,
			th.statusBadge(st.Status))
		out = append(out, stageLine)
		if strings.TrimSpace(st.Reason) != "" {
			hasStageReason = true
			out = append(out, fmt.Sprintf("  %s└─ Error:%s %s%s%s", th.red, th.reset, th.amber, strings.TrimSpace(st.Reason), th.reset))
		}
	}
	if !hasStageReason && strings.TrimSpace(chg.Workflow.Reason) != "" {
		out = append(out, fmt.Sprintf("%sReason:%s    %s%s%s", th.muted, th.reset, th.amber, theme.TruncatePlain(strings.TrimSpace(chg.Workflow.Reason), 74), th.reset))
	}
	return out
}

func (s *State) collectFailedCheckSummaryLines(th ansiTheme, chg *ChangeSession, maxLines int) []string {
	var out []string
	for _, chk := range chg.Checks {
		if strings.EqualFold(chk.State, "passed") {
			continue
		}
		cmdStr := strings.Join(chk.Command, " ")
		out = append(out, fmt.Sprintf("%sCheck:%s     %s  %s%s%s (exit %d)  %s%s%s",
			th.muted, th.reset, th.statusBadge(chk.State), th.bold+th.fg, chk.Name, th.reset, chk.ExitCode, th.muted, cmdStr, th.reset))
		out = append(out, formatCheckOutputLines(th, chk.Output, maxLines)...)
	}
	return out
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
	panelW := adaptivePanelWidth(width)
	chg := s.Change
	lines := renderPageHeader(th, "[2] AGENT STUDIO · WORKFLOW DEBUG & CHECK INSPECTOR", "[Esc/g] Back to Pipeline · [r] Repair/Refresh · [d] Diff")
	wfStatus := chg.State
	wfReason := ""
	repairs := chg.RepairAttempts
	if chg.Workflow != nil {
		wfStatus = client.FirstNonEmpty(chg.Workflow.Status, wfStatus)
		wfReason = chg.Workflow.Reason
		if chg.Workflow.AutoRepairAttempts > repairs {
			repairs = chg.Workflow.AutoRepairAttempts
		}
	}
	stageBox := []string{
		fmt.Sprintf("%sSession:%s %s%s%s (%s) · %sStatus:%s %s · %sFreshness:%s %s · %sRepairs:%s %d",
			th.muted, th.reset, th.bold+th.fg, chg.Title, th.reset, chg.ID,
			th.muted, th.reset, th.statusBadge(wfStatus),
			th.muted, th.reset, client.FirstNonEmpty(chg.Freshness, "current"),
			th.muted, th.reset, repairs),
	}
	if strings.TrimSpace(s.Error) != "" {
		stageBox = append(stageBox, fmt.Sprintf("%sError:%s   %s▲ %s%s", th.muted, th.reset, th.bold+th.red, strings.TrimSpace(s.Error), th.reset))
	}
	if strings.TrimSpace(wfReason) != "" {
		stageBox = append(stageBox, fmt.Sprintf("%sReason:%s  %s%s%s", th.muted, th.reset, th.amber, wfReason, th.reset))
	}
	stageBox = append(stageBox, renderAgentDebugStageLines(th, chg, wfReason)...)
	if chg.Workflow != nil && strings.TrimSpace(chg.Workflow.ProbeSummary) != "" {
		stageBox = append(stageBox, fmt.Sprintf("%sProbe:%s   %s%s%s", th.muted, th.reset, th.cyan, strings.TrimSpace(chg.Workflow.ProbeSummary), th.reset))
	}
	lines = append(lines, th.renderPanelBox("4-STAGE WORKFLOW & ASSIGNED MODELS", strings.ToUpper(wfStatus), panelW, stageBox)...)
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("VERIFICATION CHECKS & DIAGNOSTIC OUTPUT", fmt.Sprintf("%d CHECKS", len(chg.Checks)), panelW, s.renderAgentCheckDebugLines(th, panelW))...)
	if chg.Workflow != nil && chg.Workflow.Review != nil {
		rev := chg.Workflow.Review
		revLines := []string{
			fmt.Sprintf("%sVerdict:%s %s  %s%s%s", th.muted, th.reset, th.statusBadge(rev.Verdict), th.fg, rev.Summary, th.reset),
		}
		for _, f := range rev.Findings {
			revLines = append(revLines, fmt.Sprintf("  %s•%s %s", th.amber, th.reset, f))
		}
		lines = append(lines, "")
		lines = append(lines, th.renderPanelBox("STAGE 3 · CRITIC REVIEW VERDICT", strings.ToUpper(rev.Verdict), panelW, revLines)...)
	}
	return lines
}

func renderAgentDebugStageLines(th ansiTheme, chg *ChangeSession, wfReason string) []string {
	if chg.Workflow == nil || len(chg.Workflow.Stages) == 0 {
		return nil
	}
	var out []string
	for idx, st := range chg.Workflow.Stages {
		modelName := "default"
		if st.Model != nil && st.Model.Model != "" {
			modelName = st.Model.Model
		}
		out = append(out, fmt.Sprintf("  %s%d. %-18s%s  %s  %smodel:%s%s",
			th.bold+th.fg, idx+1, domain.CanonicalStageLabel(idx, st.Name), th.reset,
			th.padBadge(st.Status, 14), th.cyan, modelName, th.reset))
		stErr := strings.TrimSpace(st.Reason)
		if stErr == "" && (st.Status == "failed" || st.Status == "interrupted") && strings.TrimSpace(wfReason) != "" {
			stErr = strings.TrimSpace(wfReason)
		}
		if stErr != "" {
			out = append(out, fmt.Sprintf("      %s└─ Error:%s %s%s%s", th.red, th.reset, th.amber, stErr, th.reset))
		}
	}
	return out
}

func (s *State) renderAgentCheckDebugLines(th ansiTheme, _ int) []string {
	if s.Change == nil || len(s.Change.Checks) == 0 {
		if s.Change != nil && len(s.Change.Changes) == 0 && !s.hasRunningWorkflow() {
			return []string{fmt.Sprintf("%sStage 1 (Architect & Code) produced no proposal changes — inspect workflow reason above and press [r] to retry.%s", th.amber, th.reset)}
		}
		return []string{fmt.Sprintf("%sNo deterministic verification checks recorded yet — press [r] to run checks.%s", th.muted, th.reset)}
	}
	var out []string
	for _, chk := range s.Change.Checks {
		cmdStr := strings.Join(chk.Command, " ")
		out = append(out, fmt.Sprintf("%s  %s%-18s%s (exit %d)  %s%s%s",
			th.statusBadge(chk.State), th.bold+th.fg, chk.Name, th.reset, chk.ExitCode, th.muted, cmdStr, th.reset))
		if !strings.EqualFold(chk.State, "passed") {
			out = append(out, formatCheckOutputLines(th, chk.Output, 10)...)
		}
	}
	return out
}

func (s *State) renderWorkflowStagesLine(th ansiTheme) string {
	if s.Change == nil || s.Change.Workflow == nil || len(s.Change.Workflow.Stages) == 0 {
		return ""
	}
	shortNames := []string{"1.Code", "2.Test", "3.Critic", "4.Review"}
	var parts []string
	for idx, st := range s.Change.Workflow.Stages {
		if idx >= len(shortNames) {
			break
		}
		label := shortNames[idx]
		stStatus := normalizePipelineStageBadge(idx+1, st.Status, s.Change)
		parts = append(parts, fmt.Sprintf("%s%s%s %s", th.muted, label, th.reset, th.statusBadge(stStatus)))
	}
	if s.Change.Workflow.AutoRepairAttempts > 0 {
		parts = append(parts, fmt.Sprintf("%srepair:#%d%s", th.amber, s.Change.Workflow.AutoRepairAttempts, th.reset))
	}
	return strings.Join(parts, "  ")
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

func (s *State) renderAgentTelemetryLine(th ansiTheme, item AgentItem, _ int) string {
	if s.Change != nil {
		if s.Change.Workflow != nil && s.Change.Workflow.Review != nil {
			rev := s.Change.Workflow.Review
			return fmt.Sprintf("%sCritic:%s    %s  %s%s%s",
				th.muted, th.reset, th.statusBadge(rev.Verdict),
				th.fg, rev.Summary, th.reset)
		}
		for _, chk := range s.Change.Checks {
			if strings.EqualFold(chk.State, "failed") && strings.TrimSpace(chk.Output) != "" {
				return fmt.Sprintf("%sFailure:%s   %s▲ %s:%s %s",
					th.muted, th.reset, th.bold+th.red, chk.Name, th.reset,
					strings.TrimSpace(chk.Output))
			}
		}
		if s.Change.Workflow != nil && strings.TrimSpace(s.Change.Workflow.Reason) != "" {
			return fmt.Sprintf("%sReason:%s    %s%s%s",
				th.muted, th.reset, th.amber, s.Change.Workflow.Reason, th.reset)
		}
	}
	return fmt.Sprintf("%sProgress:%s  Stage %d/4 (%s) · Delta: %s+%d%s/%s-%d%s (%s)",
		th.muted, th.reset, item.StageNumber, item.Stage,
		th.green, item.AddedLines, th.reset, th.red, item.RemovedLines, th.reset, item.ScopeBadge)
}

func (s *State) renderAgentNextStepLine(th ansiTheme, item AgentItem) string {
	st := strings.ToLower(strings.TrimSpace(item.Status))
	switch {
	case st == "stale":
		return fmt.Sprintf("%sNext Step:%s %s▲ Proposal is stale (files changed)%s · %s[r]%s Refresh & Rerun Agent · %s[b]%s Issues",
			th.bold+th.cyan, th.reset, th.amber, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	case st == "applied":
		return fmt.Sprintf("%sNext Step:%s %s✔ Fix applied & committed%s · %s[u]%s Guarded Undo · %s[b]%s Back to Issues",
			th.bold+th.cyan, th.reset, th.green, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	case st == "running" || st == "queued" || st == "canceling":
		return fmt.Sprintf("%sNext Step:%s %s⟳ Workflow running%s · %s[g]%s Debug & Logs · %s[r]%s Refresh · %s[x]%s Cancel",
			th.bold+th.cyan, th.reset, th.amber, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	case st == "failed" || st == "changes_requested" || (item.ChecksTotal > 0 && item.ChecksPassed < item.ChecksTotal):
		return fmt.Sprintf("%sNext Step:%s %s▲ Verification needs repair%s · %s[r]%s Self-Repair · %s[g]%s Debug & Logs · %s[d]%s Diff",
			th.bold+th.cyan, th.reset, th.red, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	case st == "ready" || st == "checked" || (item.ChecksTotal > 0 && item.ChecksPassed == item.ChecksTotal):
		return fmt.Sprintf("%sNext Step:%s %s✔ Ready to Review & Apply%s · %s[d/Enter]%s Inspect Diff · %s[c]%s Apply & Commit · %s[g]%s Debug",
			th.bold+th.cyan, th.reset, th.green, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	default:
		return fmt.Sprintf("%sNext Step:%s %s[d/Enter]%s Inspect Diff · %s[g]%s Debug & Logs · %s[r]%s Run Checks · %s[c]%s Apply",
			th.bold+th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
	}
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
	lines := renderPageHeader(th, "[3] CODE & DIFF · INSPECTOR", subRight)
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
			codeCol := max(16, width-12)
			start, end := windowBounds(len(rawLines), s.ClampedIdx(), s.adaptiveWindowRows(10, 14))
			for idx := start; idx < end; idx++ {
				codeText := theme.TruncatePlain(theme.SanitizeCodeText(rawLines[idx]), codeCol)
				lines = append(lines, fmt.Sprintf("  %s%4d │%s %s", th.muted, idx+1, th.reset, codeText))
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
	lines := renderPageHeader(th, "[3] CODE & DIFF · GIT & WORKTREES", "Isolated worktrees & branch status")
	if s.GitWorkspace == nil || len(s.GitWorkspace.Worktrees) == 0 {
		return append(lines, fmt.Sprintf("  %sNo Git worktrees reported.%s", th.muted, th.reset))
	}
	lines = append(lines, th.renderTableHeader("    ROLE          BRANCH                     WORKTREE PATH", width)...)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(s.GitWorkspace.Worktrees), selected, s.adaptiveWindowRows(12, 12))
	for idx := start; idx < end; idx++ {
		wt := s.GitWorkspace.Worktrees[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		role := "WORKTREE"
		if wt.Current {
			role = "CURRENT"
		} else if wt.Mergeable {
			role = "MERGEABLE"
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%s  %sbranch:%-18s%s  %spath:%s%s",
			cursor, th.padBadge(role, 12), tStyle, wt.Branch, th.reset, th.muted, wt.Path, th.reset))
	}
	return appendWindowFooter(th, lines, start, end, len(s.GitWorkspace.Worktrees))
}

func (s *State) renderHistoryPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[3] CODE & DIFF · SAVED PROPOSALS", "Guarded change history & audit trail")
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
