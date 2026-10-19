package tui

import (
	"fmt"
	"strings"
)

func (s *State) renderActivePage(th ansiTheme, width int) []string {
	switch s.Page {
	case PageSummary:
		return s.renderSummaryPage(th, width)
	case PageRepo:
		return s.renderRepoPage(th, width)
	case PageProject:
		return s.renderProjectPage(th, width)
	case PageAgents:
		return s.renderAgentsPage(th, width)
	case PageBugs:
		return s.renderBugsPage(th, width)
	case PageIssueDetail:
		return s.renderIssueDetailPage(th, width)
	case PageFeatures:
		return s.renderFeaturesPage(th, width)
	case PageAnalysisRun:
		return s.renderAnalysisRunPage(th, width)
	case PageAnalysisFiles:
		return s.renderAnalysisFilesPage(th, width)
	default:
		return s.renderSecondaryPage(th, width)
	}
}

func (s *State) renderSecondaryPage(th ansiTheme, width int) []string {
	switch s.Page {
	case PageEditor:
		return s.renderEditorPage(th, width)
	case PageGit:
		return s.renderGitPage(th, width)
	case PageHistory:
		return s.renderHistoryPage(th, width)
	case PageDocs:
		return s.renderDocsPage(th, width)
	case PageDocsComponents:
		return s.renderDocsComponentsPage(th, width)
	case PageDocsDecisions:
		return s.renderDocsDecisionsPage(th, width)
	case PageDiagrams:
		return s.renderDiagramsPage(th, width)
	case PageInstructions, PageModels, PageBootstrap, PageLessons:
		return s.renderGovernanceSubPage(th, width)
	default:
		return s.renderSummaryPage(th, width)
	}
}

func adaptivePanelWidth(width int) int {
	return max(42, width-6)
}

func renderPageHeader(th ansiTheme, title, subtitle string) []string {
	if strings.TrimSpace(subtitle) == "" {
		return []string{
			fmt.Sprintf("  %s%s%s", th.bold+th.accent, title, th.reset),
			"",
		}
	}
	return []string{
		fmt.Sprintf("  %s%s%s   %s· %s%s", th.bold+th.accent, title, th.reset, th.muted, subtitle, th.reset),
		"",
	}
}

func rowTitleStyle(th ansiTheme, selected bool) string {
	if selected {
		return th.bold + th.fg
	}
	return th.fg
}

func (s *State) renderSummaryPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	allIssues := s.CollectAllIssues()
	summary := ComputeIssueSummary(allIssues, s.Features)
	cards := [][2]string{
		{"OPEN FINDINGS", fmt.Sprintf("%s%d%s open %s(%d active)%s", th.bold+th.red, summary.TotalOpen, th.reset, th.muted, summary.InProgress, th.reset)},
		{"RESOLVED", fmt.Sprintf("%s%d%s verified", th.bold+th.green, summary.Done, th.reset)},
		{"RISK BREAKDOWN", fmt.Sprintf("%sB:%d%s  %sS:%d%s  %sP:%d%s", th.bold+th.red, summary.Bugs, th.reset, th.bold+th.amber, summary.Security, th.reset, th.cyan, summary.Performance, th.reset)},
		{"FEATURE IDEAS", fmt.Sprintf("%s%d%s ready", th.bold+th.purple, summary.Features, th.reset)},
	}
	lines := renderPageHeader(th, "[1] OVERVIEW · EXECUTIVE DASHBOARD", "Real-time workspace health & risk telemetry")
	lines = append(lines, th.renderKPICards(cards, width)...)
	denom := max(1, summary.TotalOpen+summary.Done)
	distBody := []string{
		fmt.Sprintf("%sBugs & Defects%s      %s  %s%d open%s", th.bold+th.fg, th.reset, th.renderProgressBar(summary.Bugs, denom, 16), th.red, summary.Bugs, th.reset),
		fmt.Sprintf("%sSecurity Risks%s      %s  %s%d open%s", th.bold+th.fg, th.reset, th.renderProgressBar(summary.Security, denom, 16), th.amber, summary.Security, th.reset),
		fmt.Sprintf("%sPerformance%s         %s  %s%d open%s", th.fg, th.reset, th.renderProgressBar(summary.Performance, denom, 16), th.cyan, summary.Performance, th.reset),
		fmt.Sprintf("%sResolved & Verified%s %s  %s%d fixed%s", th.fg, th.reset, th.renderProgressBar(summary.Done, denom, 16), th.green, summary.Done, th.reset),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("CATEGORY BREAKDOWN · HEALTH DISTRIBUTION", "TELEMETRY", panelW, distBody)...)

	focusStatus := fmt.Sprintf("%sAll findings resolved — workspace is clean%s", th.green, th.reset)
	if summary.TotalOpen > 0 {
		focusStatus = fmt.Sprintf("%s%d open findings%s require triage (%s%d bugs%s · %s%d security%s)",
			th.bold+th.amber, summary.TotalOpen, th.reset, th.red, summary.Bugs, th.reset, th.amber, summary.Security, th.reset)
	}
	agentSummary := fmt.Sprintf("%s  No active proposal in flight", th.statusBadge("IDLE"))
	if s.Change != nil {
		agentSummary = fmt.Sprintf("%s  %s%s%s", th.statusBadge(s.Change.State), th.bold+th.fg, truncatePlain(s.Change.Title, max(24, panelW-24)), th.reset)
	}
	scanState := "IDLE"
	if s.Scan != nil && s.Scan.Status != "" {
		scanState = s.Scan.Status
	}
	focusBody := []string{
		fmt.Sprintf("%sPriority Triage:%s    %s", th.bold+th.cyan, th.reset, focusStatus),
		fmt.Sprintf("%sAgent Pipeline:%s     %s", th.bold+th.purple, th.reset, agentSummary),
		fmt.Sprintf("%sVerification Gate:%s  %s  %s(%d feature proposals queued)%s", th.bold+th.green, th.reset, th.statusBadge(scanState), th.muted, summary.Features, th.reset),
	}
	lines = append(lines, "")
	return append(lines, th.renderPanelBox("PRIORITY FOCUS & WORKSPACE READINESS", "STATUS SUMMARY", panelW, focusBody)...)
}

func (s *State) renderRepoPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[1] OVERVIEW · REPOSITORY PROFILE", "Indexed codebase footprint & language inventory")
	if s.Project == nil {
		return append(lines, fmt.Sprintf("  %sNo project loaded.%s", th.muted, th.reset))
	}
	branch := "detached"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = s.Repository.Branch
	}
	meta := []string{
		fmt.Sprintf("%sRepository:%s     %s%s%s", th.muted, th.reset, th.bold+th.fg, s.Project.Name, th.reset),
		fmt.Sprintf("%sWorkspace Path:%s %s%s%s", th.muted, th.reset, th.cyan, s.Project.Path, th.reset),
		fmt.Sprintf("%sStack & Build:%s  %s%s%s · Manifest: %s · Branch: %s%s%s",
			th.muted, th.reset, th.bold+th.fg, firstNonEmpty(s.Project.Type, "polyglot"), th.reset,
			firstNonEmpty(s.Project.BuildFile, "auto"), th.purple, branch, th.reset),
		fmt.Sprintf("%sCode Footprint:%s %s%d%s indexed files · %s%d%s total lines",
			th.muted, th.reset, th.bold+th.green, s.Project.FileCount, th.reset, th.bold+th.fg, s.Project.TotalLines, th.reset),
	}
	lines = append(lines, th.renderPanelBox("WORKSPACE IDENTITY & FOOTPRINT", "REPOSITORY", panelW, meta)...)
	if s.Index != nil && len(s.Index.Files) > 0 {
		lines = append(lines, "", fmt.Sprintf("  %sINDEXED FILES SAMPLE (%d TOTAL)%s", th.bold+th.cyan, len(s.Index.Files), th.reset))
		limit := min(8, len(s.Index.Files))
		for i := 0; i < limit; i++ {
			f := s.Index.Files[i]
			lines = append(lines, fmt.Sprintf("  %s•%s %-42s %s%s · %d lines%s",
				th.accent, th.reset, truncatePlain(f.Path, 42), th.muted, firstNonEmpty(f.Language, "text"), f.LineCount, th.reset))
		}
	}
	return lines
}

func (s *State) renderProjectPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[1] OVERVIEW · VERIFICATION TESTS & CHECKS", "Automated quality gates & test execution")
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		empty := []string{
			"No verification check phases recorded for the current session.",
			fmt.Sprintf("%sStatus:%s Waiting for verification run.", th.muted, th.reset),
		}
		return append(lines, th.renderPanelBox("VERIFICATION GATE PIPELINE", "IDLE", panelW, empty)...)
	}
	lines = append(lines, th.renderTableHeader("   PHASE             STATUS        STATE         COMMAND", width)...)
	selected := s.ClampedIdx()
	for idx, ph := range s.Scan.Phases {
		cursor := th.renderCursorPrefix(idx == selected)
		nameStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-16s%s  %-12s  %sstate:%-10s  cmd:%s%s",
			cursor, nameStyle, ph.Name, th.reset, th.statusBadge(ph.State), th.muted, ph.State, strings.Join(ph.Command, " "), th.reset,
		))
		if strings.EqualFold(ph.State, "failed") && strings.TrimSpace(ph.Output) != "" {
			lines = append(lines, fmt.Sprintf("    %s▲ Check Failure:%s %s%s%s",
				th.critBg, th.reset, th.bold+th.red, truncatePlain(strings.TrimSpace(ph.Output), max(36, width-24)), th.reset))
		}
	}
	return lines
}

func windowBounds(total, selected, maxRows int) (int, int) {
	if total <= maxRows || maxRows <= 0 {
		return 0, total
	}
	start := max(0, selected-maxRows/2)
	end := start + maxRows
	if end > total {
		end = total
		start = max(0, end-maxRows)
	}
	return start, end
}

func appendWindowFooter(th ansiTheme, lines []string, start, end, total int) []string {
	if total <= end-start {
		return lines
	}
	return append(lines, fmt.Sprintf("  %sShowing %d–%d of %d items%s", th.muted, start+1, end, total, th.reset))
}

func (s *State) renderAgentsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	items := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	activeStage := 1
	if len(items) > 0 {
		activeStage = items[s.ClampedIdx()].StageNumber
	}
	stepper := renderPipelineStepper(th, activeStage)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · 4-STAGE AUTONOMOUS PIPELINE", "Guarded code generation & verification")
	lines = append(lines, th.renderPanelBox("GUARDED EXECUTION PIPELINE", "4-STAGE WORKFLOW", panelW, []string{stepper})...)
	if len(items) == 0 {
		guide := []string{
			fmt.Sprintf("%sStatus:%s No active autonomous missions running.", th.bold+th.cyan, th.reset),
			fmt.Sprintf("• %sIssue Remediation:%s Select a finding in %sIssues%s to generate a guarded fix.", th.bold+th.fg, th.reset, th.accent, th.reset),
			fmt.Sprintf("• %sFeature Delivery:%s  Select a proposal in %sFeatures%s or run %s:agent <goal>%s.", th.bold+th.fg, th.reset, th.accent, th.reset, th.purple, th.reset),
		}
		lines = append(lines, "")
		return append(lines, th.renderPanelBox("AUTONOMOUS MISSION STANDBY", "IDLE", panelW, guide)...)
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   STAGE              MISSION TITLE               SCOPE      DELTA       CHECKS", width)...)
	selected := s.ClampedIdx()
	titleCol := max(16, width-60)
	start, end := windowBounds(len(items), selected, 8)
	for idx := start; idx < end; idx++ {
		item := items[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		stageLabel := truncatePlain(fmt.Sprintf("%d. %s", item.StageNumber, item.Stage), 16)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-16s  %s%-*s%s  %s  %s+%d%s/%s-%d%s  %schecks:%d/%d%s",
			cursor, stageLabel, tStyle, titleCol, truncatePlain(item.Title, titleCol), th.reset,
			th.statusBadge(item.ScopeBadge),
			th.green, item.AddedLines, th.reset, th.red, item.RemovedLines, th.reset,
			th.muted, item.ChecksPassed, item.ChecksTotal, th.reset,
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(items))
	curItem := items[selected]
	detail := []string{
		fmt.Sprintf("%sMission:%s  %s%s%s", th.muted, th.reset, th.bold+th.fg, curItem.Title, th.reset),
		fmt.Sprintf("%sScope:%s    %s%s%s   %sBranch:%s %s%s%s   %sStatus:%s %s",
			th.muted, th.reset, th.cyan, firstNonEmpty(curItem.Path, "workspace"), th.reset,
			th.muted, th.reset, th.purple, curItem.Branch, th.reset,
			th.muted, th.reset, th.statusBadge(curItem.Status)),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("ACTIVE AGENT MISSION CONTROL", strings.ToUpper(curItem.Stage), panelW, detail)...)
	return lines
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

func (s *State) renderBugsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	batchBadge := ""
	if len(s.SelectedIssueIDs) > 0 {
		batchBadge = fmt.Sprintf("   %s BATCH SELECTED: %d %s", th.warnBg, len(s.SelectedIssueIDs), th.reset)
	}
	lines := []string{
		fmt.Sprintf(
			"  %s[2] AGENT STUDIO · ISSUES & BUGS%s   %sCategory:%s %s[%s]%s   %sStatus:%s %s[%s]%s%s",
			th.bold+th.accent, th.reset,
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(s.IssueCategory), th.reset,
			th.muted, th.reset, th.bold+th.green, strings.ToUpper(s.IssueView), th.reset,
			batchBadge,
		),
		"",
	}
	lines = append(lines, th.renderTableHeader("   SEL  SEVERITY    CATEGORY   FINDING                        LOCATION            STATUS", width)...)
	filtered := s.FilteredIssues()
	if len(filtered) == 0 {
		return append(lines, fmt.Sprintf("  %sNo findings match the active filter.%s", th.muted, th.reset))
	}
	nameCol := max(18, width-62)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(filtered), selected, 9)
	for idx := start; idx < end; idx++ {
		row := filtered[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		mark := th.muted + "[ ]" + th.reset
		if containsString(s.SelectedIssueIDs, row.ID) {
			mark = th.amber + "[x]" + th.reset
		}
		loc := truncatePlain(fmt.Sprintf("%s:%d", row.Path, row.Line), 18)
		cat := truncatePlain(row.Category, 9)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s %s  %s%-9s%s  %s%-*s%s  %s%-18s%s  %s",
			cursor, mark, th.severityBadge(row.Severity),
			th.purple, cat, th.reset,
			tStyle, nameCol, truncatePlain(row.Name, nameCol), th.reset,
			th.muted, loc, th.reset,
			th.statusBadge(row.Status),
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(filtered))
	if cur := s.SelectedIssue(); cur != nil {
		footer := []string{
			fmt.Sprintf("%sFinding:%s   %s  %s%s%s",
				th.muted, th.reset, th.severityBadge(cur.Severity), th.bold+th.fg, truncatePlain(cur.Title, max(20, panelW-22)), th.reset),
			fmt.Sprintf("%sLocation:%s  %s%s:%d%s   %sSummary:%s %s",
				th.muted, th.reset, th.cyan, cur.Path, cur.Line, th.reset,
				th.muted, th.reset, truncatePlain(cur.Cause, max(16, panelW-38))),
		}
		lines = append(lines, "")
		lines = append(lines, th.renderPanelBox("SELECTED FINDING FOCUS", strings.ToUpper(cur.Category), panelW, footer)...)
	}
	return lines
}

func (s *State) renderIssueDetailPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · ISSUE DETAIL & ROOT CAUSE", "Deep diagnostic & remediation plan")
	cur := s.SelectedIssue()
	if cur == nil {
		return append(lines, fmt.Sprintf("  %sNo finding selected.%s", th.muted, th.reset))
	}
	textW := max(28, panelW-22)
	meta := []string{
		fmt.Sprintf("%sFinding:%s  %s%s%s", th.muted, th.reset, th.bold+th.fg, cur.Title, th.reset),
		fmt.Sprintf("%sSeverity:%s %s   %sCategory:%s %s%s%s   %sStatus:%s %s",
			th.muted, th.reset, th.severityBadge(cur.Severity),
			th.muted, th.reset, th.bold+th.purple, strings.ToUpper(cur.Category), th.reset,
			th.muted, th.reset, th.statusBadge(cur.Status)),
		fmt.Sprintf("%sLocation:%s %s%s:%d%s", th.muted, th.reset, th.cyan, cur.Path, cur.Line, th.reset),
	}
	lines = append(lines, th.renderPanelBox("ISSUE METADATA & LOCATION", cur.ID, panelW, meta)...)
	diag := []string{
		fmt.Sprintf("%s▲ Root Cause:%s      %s%s%s", th.bold+th.red, th.reset, th.amber, truncatePlain(cur.Cause, textW), th.reset),
		fmt.Sprintf("%s✔ Recommended Fix:%s %s%s%s", th.bold+th.green, th.reset, th.bold+th.fg, truncatePlain(cur.Solution, textW), th.reset),
	}
	lines = append(lines, "")
	return append(lines, th.renderPanelBox("ROOT CAUSE DIAGNOSTIC & REMEDIATION", strings.ToUpper(cur.Category), panelW, diag)...)
}

func (s *State) renderFeaturesPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · FEATURES & ENHANCEMENTS", "Architecture-aligned capability proposals")
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, fmt.Sprintf("  %sNo active feature proposals queued.%s", th.muted, th.reset))
	}
	lines = append(lines, th.renderTableHeader("   EFFORT      FEATURE PROPOSAL                    STRATEGIC BENEFIT", width)...)
	selected := s.ClampedIdx()
	start, end := windowBounds(len(features), selected, 9)
	for idx := start; idx < end; idx++ {
		f := features[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		effort := strings.ToUpper(firstNonEmpty(f.Effort, "med"))
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %s%-32s%s  %s%s%s",
			cursor, th.severityBadge(effort), tStyle, truncatePlain(f.Title, 32), th.reset,
			th.muted, truncatePlain(f.Benefit, max(18, width-52)), th.reset,
		))
	}
	lines = appendWindowFooter(th, lines, start, end, len(features))
	cur := features[selected]
	featBox := []string{
		fmt.Sprintf("%sProposal:%s %s%s%s   %sEffort:%s %s",
			th.muted, th.reset, th.bold+th.fg, cur.Title, th.reset,
			th.muted, th.reset, th.severityBadge(firstNonEmpty(cur.Effort, "med"))),
		fmt.Sprintf("%sBenefit:%s  %s%s%s", th.muted, th.reset, th.green, truncatePlain(cur.Benefit, max(28, panelW-14)), th.reset),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("FEATURE IMPLEMENTATION WORKBENCH", "CAPABILITY SPEC", panelW, featBox)...)
	return lines
}

func (s *State) renderAnalysisRunPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · LAST ANALYSIS RUN", "Section coverage & execution telemetry")
	if s.Run == nil {
		return append(lines, fmt.Sprintf("  %sNo analysis run recorded yet.%s", th.muted, th.reset))
	}
	done, total := runAggregateCoverage(s.Run)
	summaryLine := fmt.Sprintf(
		"%sRun ID:%s %s%s%s · %sStatus:%s %s · %sElapsed:%s %ds · %sCoverage:%s %s",
		th.muted, th.reset, th.bold+th.fg, s.Run.Identity.ID, th.reset,
		th.muted, th.reset, th.statusBadge(s.Run.Status),
		th.muted, th.reset, s.Run.ElapsedSeconds,
		th.muted, th.reset, th.renderProgressBar(done, total, 12),
	)
	lines = append(lines, th.renderPanelBox("ANALYSIS TELEMETRY", strings.ToUpper(s.Run.Status), panelW, []string{summaryLine})...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   SECTION         STATUS        PROGRESS             SUCCEEDED    FAILED", width)...)
	selected := s.ClampedIdx()
	for idx, sec := range s.Run.Sections {
		cursor := th.renderCursorPrefix(idx == selected)
		bar := th.renderProgressBar(sec.Coverage.Succeeded, sec.Coverage.Total, 10)
		failStr := fmt.Sprintf("%sfailed:%d%s", th.muted, sec.Coverage.Failed, th.reset)
		if sec.Coverage.Failed > 0 {
			failStr = th.critBg + fmt.Sprintf(" failed:%d ", sec.Coverage.Failed) + th.reset
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-14s%s  %-12s  %s   %ssucceeded:%d/%d%s %s",
			cursor, tStyle, strings.ToUpper(sec.Category), th.reset,
			th.statusBadge(sec.Status), bar,
			th.green, sec.Coverage.Succeeded, sec.Coverage.Total, th.reset, failStr,
		))
	}
	return lines
}

func (s *State) renderAnalysisFilesPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "[2] AGENT STUDIO · FILE SCOPE", "Captured path inclusion & analysis scope")
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, fmt.Sprintf("  %sNo indexed files available.%s", th.muted, th.reset))
	}
	total := len(s.Index.Files)
	excl := len(s.ExcludedPaths)
	incl := max(0, total-excl)
	lines = append(lines, fmt.Sprintf("  %sScope Summary:%s %s%d Included%s · %s%d Excluded%s · %d Total Indexed Files",
		th.muted, th.reset, th.bold+th.green, incl, th.reset, th.amber, excl, th.reset, total), "")
	lines = append(lines, th.renderTableHeader("   SCOPE     REPOSITORY PATH                             LANGUAGE      LINES", width)...)
	selected := s.ClampedIdx()
	pathCol := max(24, width-34)
	start, end := windowBounds(total, selected, 14)
	for idx := start; idx < end; idx++ {
		f := s.Index.Files[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		scopeTag := th.statusBadge("INCL")
		if containsString(s.ExcludedPaths, f.Path) {
			scopeTag = th.statusBadge("EXCL")
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s   %s%-*s%s  %s%s · %d lines%s",
			cursor, scopeTag, tStyle, pathCol, truncatePlain(f.Path, pathCol), th.reset,
			th.muted, firstNonEmpty(f.Language, "text"), f.LineCount, th.reset,
		))
	}
	return appendWindowFooter(th, lines, start, end, total)
}

func (s *State) renderEditorPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	whyLabel := "Inline Rationale: ON"
	if !s.ShowInlineWhy {
		whyLabel = "Inline Rationale: OFF"
	}
	lines := renderPageHeader(th, "[3] CODE & DIFF · INSPECTOR", whyLabel)
	if s.Change != nil && len(s.Change.Changes) > 0 {
		lines = append(lines, s.renderChangeDiffLines(th, width)...)
		return lines
	}
	if s.File != nil {
		header := fmt.Sprintf("%sFile:%s %s%s%s %s(%s · %d lines)%s",
			th.muted, th.reset, th.bold+th.fg, s.File.Path, th.reset,
			th.muted, firstNonEmpty(s.File.Language, "text"), s.File.LineCount, th.reset)
		lines = append(lines, th.renderPanelBox("READ-ONLY SOURCE VIEWER", "SOURCE", panelW, []string{header})...)
		if s.File.Content != "" {
			rawLines := strings.Split(s.File.Content, "\n")
			start, end := windowBounds(len(rawLines), s.ClampedIdx(), 14)
			for idx := start; idx < end; idx++ {
				lines = append(lines, fmt.Sprintf("  %s%4d │%s %s", th.muted, idx+1, th.reset, rawLines[idx]))
			}
			lines = appendWindowFooter(th, lines, start, end, len(rawLines))
		}
		return lines
	}
	return s.renderIndexedFilesInEditor(th, width, lines)
}

func (s *State) renderChangeDiffLines(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	fc := s.Change.Changes[0]
	added, removed, badge := ClassifyDiffScope(s.Change.Changes)
	hunks := SplitDiffIntoHunks(fc.Diff.Lines)
	meta := []string{
		fmt.Sprintf(
			"%sProposal:%s %s%s%s · %sFile:%s %s%s%s %s (%s+%d%s/%s-%d%s) · %sHunks:%s %d",
			th.muted, th.reset, th.bold+th.fg, s.Change.Title, th.reset,
			th.muted, th.reset, th.cyan, fc.Path, th.reset,
			th.statusBadge(badge),
			th.green, added, th.reset, th.red, removed, th.reset,
			th.muted, th.reset, len(hunks),
		),
	}
	if s.ShowInlineWhy && fc.Summary != "" {
		meta = append(meta, fmt.Sprintf("%sRationale:%s %s%s%s", th.bold+th.accent, th.reset, th.fg, fc.Summary, th.reset))
	}
	lines := th.renderPanelBox("GUARDED DIFF REVIEW", badge, panelW, meta)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("  SIGN  LINE │ UNIFIED DIFF CONTENT", width)...)
	start, end := windowBounds(len(fc.Diff.Lines), s.ClampedIdx(), 14)
	for idx := start; idx < end; idx++ {
		dl := fc.Diff.Lines[idx]
		switch {
		case IsAddedLine(dl.Kind):
			lines = append(lines, fmt.Sprintf("  %s+ %4d │ %s%s", th.green, dl.NewLine, dl.Text, th.reset))
		case IsRemovedLine(dl.Kind):
			lines = append(lines, fmt.Sprintf("  %s- %4d │ %s%s", th.red, dl.OldLine, dl.Text, th.reset))
		default:
			lines = append(lines, fmt.Sprintf("  %s  %4d │%s %s", th.muted, dl.NewLine, th.reset, dl.Text))
		}
	}
	return appendWindowFooter(th, lines, start, end, len(fc.Diff.Lines))
}

func (s *State) renderIndexedFilesInEditor(th ansiTheme, width int, lines []string) []string {
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, fmt.Sprintf("  %sNo file or diff loaded.%s", th.muted, th.reset))
	}
	lines = append(lines, th.renderTableHeader("   REPOSITORY FILE PATH                        LANGUAGE      LINES", width)...)
	selected := s.ClampedIdx()
	total := len(s.Index.Files)
	start, end := windowBounds(total, selected, 14)
	for idx := start; idx < end; idx++ {
		f := s.Index.Files[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%s%-40s%s %s(%s)  %d lines%s",
			cursor, tStyle, truncatePlain(f.Path, 40), th.reset, th.muted, firstNonEmpty(f.Language, "text"), f.LineCount, th.reset))
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
	start, end := windowBounds(len(s.GitWorkspace.Worktrees), selected, 12)
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
	start, end := windowBounds(len(s.ChangeHistory), selected, 12)
	for idx := start; idx < end; idx++ {
		entry := s.ChangeHistory[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-12s %s%-32s%s %srev:%d (%s)%s",
			cursor, th.statusBadge(entry.State), tStyle, truncatePlain(entry.Title, 32), th.reset,
			th.muted, entry.Revision, firstNonEmpty(entry.Freshness, "current"), th.reset,
		))
	}
	return appendWindowFooter(th, lines, start, end, len(s.ChangeHistory))
}

func (s *State) renderDocsPage(th ansiTheme, width int) []string {
	if s.DocsSection == "components" {
		return s.renderDocsComponentsPage(th, width)
	}
	if s.DocsSection == "scenarios" || s.DocsSection == "decisions" {
		return s.renderDocsDecisionsPage(th, width)
	}
	panelW := adaptivePanelWidth(width)
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "[4] HANDBOOK & ARCH · SYSTEM CONTEXT & DEPENDENCIES", "Specification & subsystem boundaries")
	ctxLines := []string{
		fmt.Sprintf("%sPurpose:%s  %s%s%s", th.bold+th.cyan, th.reset, th.fg, docs.Context.Purpose, th.reset),
		fmt.Sprintf("%sTopology:%s %s%s%s", th.bold+th.purple, th.reset, th.fg, docs.Context.Topology, th.reset),
	}
	lines = append(lines, th.renderPanelBox("SYSTEM CONTEXT & TOPOLOGY", strings.ToUpper(docs.Status), panelW, ctxLines)...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   DEPENDENCY & SUBSYSTEM CATALOG", width)...)
	for _, dep := range docs.Dependencies {
		lines = append(lines, fmt.Sprintf("  %s•%s %s%s%s %s(%s)%s — %s",
			th.accent, th.reset, th.bold+th.fg, dep.Name, th.reset, th.muted, dep.Kind, th.reset, dep.Purpose))
	}
	return lines
}

func (s *State) renderDocsComponentsPage(th ansiTheme, width int) []string {
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "[4] HANDBOOK & ARCH · ARCHITECTURAL COMPONENTS", "Package ownership & responsibility layers")
	lines = append(lines, th.renderTableHeader("   LAYER          COMPONENT                 PATH                        SUMMARY", width)...)
	for _, c := range docs.Components {
		lines = append(lines, fmt.Sprintf("  %s•%s %s[%s]%s %s%s%s %s(%s)%s — %s",
			th.accent, th.reset, th.purple, c.Layer, th.reset, th.bold+th.fg, c.Name, th.reset, th.muted, c.Path, th.reset, c.Summary))
	}
	return lines
}

func (s *State) renderDocsDecisionsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "[4] HANDBOOK & ARCH · SCENARIOS & ARCHITECTURAL DECISIONS", "Runtime invariants & ADRs")
	var scenBody []string
	for _, sc := range docs.Scenarios {
		scenBody = append(scenBody, fmt.Sprintf("%s• %s:%s %s", th.bold+th.cyan, sc.Title, th.reset, sc.Summary))
	}
	lines = append(lines, th.renderPanelBox("RUNTIME SCENARIOS & INVARIANTS", "BEHAVIOR", panelW, scenBody)...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   CATEGORY       ARCHITECTURAL DECISION RECORD (ADR)", width)...)
	for _, d := range docs.Decisions {
		lines = append(lines, fmt.Sprintf("  %s•%s %s[%s]%s %s%s%s — %s",
			th.accent, th.reset, th.purple, d.Category, th.reset, th.bold+th.fg, d.Title, th.reset, d.Decision))
	}
	return lines
}

func (s *State) renderDiagramsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "[4] HANDBOOK & ARCH · ARCHITECTURE DIAGRAM", "Component topology blueprint")
	diagramLines := RenderArchitectureASCII(docs)
	lines = append(lines, th.renderPanelBox("COMPONENT TOPOLOGY BLUEPRINT", docs.Context.Topology, panelW, diagramLines)...)
	return lines
}

func (s *State) renderGovernanceSubPage(th ansiTheme, width int) []string {
	section := s.RulesSection
	switch s.Page {
	case PageModels:
		section = "models"
	case PageBootstrap:
		section = "bootstrap"
	case PageLessons:
		section = "lessons"
	}
	switch section {
	case "models":
		return s.renderModelsPage(th, width)
	case "bootstrap":
		return s.renderBootstrapPage(th, width)
	case "lessons":
		return s.renderLessonsPage(th, width)
	default:
		return s.renderTrustPage(th, width)
	}
}

func (s *State) renderTrustPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	trustBadge := th.statusBadge("TRUSTED")
	if !s.WorkspaceTrust {
		trustBadge = th.statusBadge("UNTRUSTED")
	}
	lines := renderPageHeader(th, "[5] GOVERNANCE & RULES · TRUST POLICY", "Execution sandbox & repository consent")
	body := []string{
		fmt.Sprintf("%sExecution Trust:%s %s  %s(active workspace consent: %v)%s",
			th.bold+th.fg, th.reset, trustBadge, th.muted, s.WorkspaceTrust, th.reset),
		fmt.Sprintf("%sSandbox Policy:%s  Project-code checks run in isolated worktrees with controlled argv and bounded output.",
			th.muted, th.reset),
	}
	return append(lines, th.renderPanelBox("EXECUTION TRUST GOVERNANCE", "SANDBOX POLICY", panelW, body)...)
}

func (s *State) renderModelsPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "[5] GOVERNANCE & RULES · MODEL ROUTING", "Task-specific LLM preset profiles")
	lines = append(lines, th.renderTableHeader("   PRESET PROFILE                 STATE       ANALYZE        BUG            FUNCTION", width)...)
	selected := s.ClampedIdx()
	for idx, p := range DefaultModelPresets() {
		cursor := th.renderCursorPrefix(idx == selected)
		active := "         "
		if p.ID == s.ActivePresetID {
			active = th.green + "(ACTIVE) " + th.reset
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-28s%s %s %s[analyze:%s bug:%s function:%s]%s",
			cursor, tStyle, p.Name, th.reset, active, th.muted, p.Analyze, p.Bug, p.Function, th.reset,
		))
	}
	return lines
}

func (s *State) renderBootstrapPage(th ansiTheme, width int) []string {
	pillars := EvaluateBootstrapChecklist(s.Index, s.Instructions)
	readyCount := countReadyPillars(pillars)
	lines := renderPageHeader(th, "[5] GOVERNANCE & RULES · BOOTSTRAP CHECKLIST", "Repository readiness & AGENTS.md pillars")
	lines = append(lines, fmt.Sprintf("  %sGovernance Readiness:%s %s %s(%d/%d pillars active)%s",
		th.muted, th.reset, th.renderProgressBar(readyCount, len(pillars), 14), th.bold+th.fg, readyCount, len(pillars), th.reset), "")
	lines = append(lines, th.renderTableHeader("   STATUS      GOVERNANCE PILLAR              DESCRIPTION", width)...)
	selected := s.ClampedIdx()
	for idx, pil := range pillars {
		cursor := th.renderCursorPrefix(idx == selected)
		badge := th.statusBadge("MISSING")
		if pil.Ready {
			badge = th.statusBadge("READY")
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%-10s %s%-28s%s %s— %s%s",
			cursor, badge, tStyle, pil.Title, th.reset, th.muted, pil.Description, th.reset))
	}
	return lines
}

func (s *State) renderLessonsPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "[5] GOVERNANCE & RULES · HARNESS² LESSONS", "Graduated engineering guardrails")
	lines = append(lines, th.renderTableHeader("   HARNESS² GUARDRAIL LESSONS", width)...)
	selected := s.ClampedIdx()
	for idx, rule := range s.AllHarnessLessons() {
		cursor := th.renderCursorPrefix(idx == selected)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s• %s%s%s", cursor, tStyle, rule, th.reset))
	}
	return lines
}

func countReadyPillars(pillars []BootstrapPillar) int {
	ready := 0
	for _, p := range pillars {
		if p.Ready {
			ready++
		}
	}
	return ready
}
