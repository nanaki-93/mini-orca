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
	activeStage := 1
	if len(items) > 0 {
		activeStage = items[s.ClampedIdx()].StageNumber
	}
	pipelineLines := []string{renderPipelineStepper(th, activeStage)}
	if stageSummary := s.renderWorkflowStagesLine(th); stageSummary != "" {
		pipelineLines = append(pipelineLines, stageSummary)
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
	lines = append(lines, th.renderTableHeader("   STAGE              MISSION TITLE               SCOPE      DELTA       CHECKS", width)...)
	selected := s.ClampedIdx()
	titleCol := max(16, width-60)
	start, end := windowBounds(len(items), selected, 5)
	for idx := start; idx < end; idx++ {
		item := items[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		stageLabel := theme.TruncatePlain(fmt.Sprintf("%d. %s", item.StageNumber, item.Stage), 16)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-16s  %s%-*s%s  %s  %s+%d%s/%s-%d%s  %schecks:%d/%d%s",
			cursor, stageLabel, tStyle, titleCol, theme.TruncatePlain(item.Title, titleCol), th.reset,
			th.statusBadge(item.ScopeBadge),
			th.green, item.AddedLines, th.reset, th.red, item.RemovedLines, th.reset,
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
		s.renderAgentTelemetryLine(th, curItem, max(24, panelW-18)),
		s.renderAgentNextStepLine(th, curItem),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("ACTIVE AGENT MISSION CONTROL", strings.ToUpper(curItem.Stage), panelW, detail)...)
	return lines
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
	if strings.TrimSpace(wfReason) != "" {
		stageBox = append(stageBox, fmt.Sprintf("%sReason:%s  %s%s%s", th.muted, th.reset, th.amber, wfReason, th.reset))
	}
	if chg.Workflow != nil && len(chg.Workflow.Stages) > 0 {
		for idx, st := range chg.Workflow.Stages {
			modelName := "default"
			if st.Model != nil && st.Model.Model != "" {
				modelName = st.Model.Model
			}
			stageBox = append(stageBox, fmt.Sprintf("  %s%d. %-18s%s  %-12s  %smodel:%s%s",
				th.bold+th.fg, idx+1, domain.CanonicalStageLabel(idx, st.Name), th.reset,
				th.statusBadge(st.Status), th.cyan, modelName, th.reset))
		}
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

func (s *State) renderAgentCheckDebugLines(th ansiTheme, _ int) []string {
	if s.Change == nil || len(s.Change.Checks) == 0 {
		if s.Change != nil && len(s.Change.Changes) == 0 && !s.hasRunningWorkflow() {
			return []string{fmt.Sprintf("%sNo proposal changes generated yet — press [r] to retry the agent workflow.%s", th.muted, th.reset)}
		}
		return []string{fmt.Sprintf("%sNo deterministic verification checks recorded yet — press [r] to run checks.%s", th.muted, th.reset)}
	}
	var out []string
	for _, chk := range s.Change.Checks {
		cmdStr := strings.Join(chk.Command, " ")
		out = append(out, fmt.Sprintf("%s  %s%-18s%s  %s%s%s",
			th.statusBadge(chk.State), th.bold+th.fg, chk.Name, th.reset, th.muted, cmdStr, th.reset))
		if strings.TrimSpace(chk.Output) != "" && !strings.EqualFold(chk.State, "passed") {
			rawLines := strings.Split(strings.TrimSpace(strings.ReplaceAll(chk.Output, "\r\n", "\n")), "\n")
			limit := min(4, len(rawLines))
			for i := 0; i < limit; i++ {
				clean := strings.TrimSpace(theme.SanitizeCodeText(rawLines[i]))
				out = append(out, fmt.Sprintf("    %s│%s %s%s%s", th.red, th.reset, th.amber, clean, th.reset))
			}
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
		if st.Model != nil && st.Model.Model != "" {
			label += "(" + st.Model.Model + ")"
		}
		parts = append(parts, fmt.Sprintf("%s%s%s %s", th.muted, label, th.reset, th.statusBadge(st.Status)))
	}
	if s.Change.Workflow.AutoRepairAttempts > 0 {
		parts = append(parts, fmt.Sprintf("%srepair:#%d%s", th.amber, s.Change.Workflow.AutoRepairAttempts, th.reset))
	}
	return strings.Join(parts, "  ")
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

func renderPipelineStepper(th ansiTheme, activeStage int) string {
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
		switch {
		case st.num == activeStage:
			parts = append(parts, fmt.Sprintf("%s %s %s", th.infoBg, st.label, th.reset))
		case st.num < activeStage:
			parts = append(parts, fmt.Sprintf("%s[%s]%s", th.green, st.label, th.reset))
		default:
			parts = append(parts, fmt.Sprintf("%s[%s]%s", th.muted, st.label, th.reset))
		}
	}
	return strings.Join(parts, th.border+" ──▶ "+th.reset)
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
	lines = append(lines, th.renderTableHeader("   ROLE          BRANCH               WORKTREE PATH", width)...)
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
		lines = append(lines, fmt.Sprintf("  %s%-12s  %sbranch:%-18s%s %spath:%s%s",
			cursor, th.statusBadge(role), tStyle, wt.Branch, th.reset, th.muted, wt.Path, th.reset))
	}
	return appendWindowFooter(th, lines, start, end, len(s.GitWorkspace.Worktrees))
}

func (s *State) renderHistoryPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "[3] CODE & DIFF · SAVED PROPOSALS", "Guarded change history & audit trail")
	if len(s.ChangeHistory) == 0 {
		return append(lines, fmt.Sprintf("  %sNo saved proposals in history.%s", th.muted, th.reset))
	}
	lines = append(lines, th.renderTableHeader("   STATE        PROPOSAL TITLE                        REVISION    FRESHNESS", width)...)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(s.ChangeHistory), selected, s.adaptiveWindowRows(12, 12))
	for idx := start; idx < end; idx++ {
		entry := s.ChangeHistory[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-12s %s%-32s%s %srev:%d (%s)%s",
			cursor, th.statusBadge(entry.State), tStyle, entry.Title, th.reset,
			th.muted, entry.Revision, client.FirstNonEmpty(entry.Freshness, "current"), th.reset,
		))
	}
	return appendWindowFooter(th, lines, start, end, len(s.ChangeHistory))
}
