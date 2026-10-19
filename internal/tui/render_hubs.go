package tui

import (
	"fmt"
	"strings"
)

func (s *State) renderActivePage(th ansiTheme, width int) []string {
	switch s.Page {
	case PageSummary:
		return s.renderSummaryPage(th, width)
	case PageProject:
		return s.renderProjectPage(th, width)
	case PageAgents:
		return s.renderAgentsPage(th, width)
	case PageBugs:
		return s.renderBugsPage(th, width)
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
	case PageDiagrams:
		return s.renderDiagramsPage(th, width)
	case PageInstructions:
		return s.renderInstructionsPage(th, width)
	default:
		return s.renderSummaryPage(th, width)
	}
}

func adaptivePanelWidth(width int) int {
	return max(60, width-6)
}

func (s *State) renderSummaryPage(th ansiTheme, width int) []string {
	allIssues := s.CollectAllIssues()
	summary := ComputeIssueSummary(allIssues, s.Features)
	cards := [][2]string{
		{"OPEN FINDINGS", fmt.Sprintf("%s%d%s open (%s%d%s active)", th.red, summary.TotalOpen, th.reset, th.amber, summary.InProgress, th.reset)},
		{"RESOLVED", fmt.Sprintf("%s%d%s verified", th.green, summary.Done, th.reset)},
		{"RISK BREAKDOWN", fmt.Sprintf("%sB:%d%s  %sS:%d%s  %sP:%d%s", th.red, summary.Bugs, th.reset, th.amber, summary.Security, th.reset, th.cyan, summary.Performance, th.reset)},
		{"FEATURE IDEAS", fmt.Sprintf("%s%d%s ready", th.purple, summary.Features, th.reset)},
	}
	lines := []string{
		fmt.Sprintf(
			"  %s[1] OVERVIEW · PROJECT HEALTH & BUG TRIAGE%s   %s(Select a bug with [j/k] and press [Enter] or [a] to start AI Agent Fix)%s",
			th.accent, th.reset, th.cyan, th.reset,
		),
	}
	lines = append(lines, th.renderKPICards(cards, width)...)
	lines = append(lines,
		fmt.Sprintf(
			"  %sMETRICS:%s Open Issues: %d  │  In Progress: %d  │  Resolved: %d  │  Bugs: %d  Sec: %d  Perf: %d  Features: %d",
			th.bold+th.muted, th.reset, summary.TotalOpen, summary.InProgress, summary.Done, summary.Bugs, summary.Security, summary.Performance, summary.Features,
		),
		"",
	)
	lines = append(lines, th.renderTableHeader("   SEVERITY    CATEGORY      FINDING TITLE                            LOCATION                     AGENT STATUS", width)...)
	open := s.openIssues()
	if len(open) > 8 {
		open = open[:8]
	}
	if len(open) == 0 {
		emptyBox := []string{
			fmt.Sprintf("%sNo open issues detected.%s", th.green, th.reset),
			fmt.Sprintf("▸ Press %s[r]%s or type %s:analyze%s to scan the project for bugs, security risks, and feature ideas.", th.cyan, th.reset, th.cyan, th.reset),
			fmt.Sprintf("▸ Type %s:agent <objective>%s to start an AI Agent for a custom bug fix or new feature.", th.green, th.reset),
		}
		return append(lines, th.renderPanelBox("GETTING STARTED · ANALYZE OR START AGENT", "READY", adaptivePanelWidth(width), emptyBox)...)
	}
	nameCol := max(35, width-68)
	selected := s.ClampedIdx()
	for idx, row := range open {
		cursor := th.renderCursorPrefix(idx == selected)
		loc := truncatePlain(fmt.Sprintf("%s:%d", row.Path, row.Line), 26)
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %-12s  %-*s  %-27s  %s",
			cursor, th.severityBadge(row.Severity), strings.ToUpper(row.Category), nameCol, truncatePlain(row.Name, nameCol), loc, th.statusBadge(row.Status),
		))
	}
	return s.appendSelectedIssueInspector(th, width, lines)
}

func (s *State) appendSelectedIssueInspector(th ansiTheme, width int, lines []string) []string {
	cur := s.SelectedIssue()
	if cur == nil {
		return lines
	}
	lines = append(lines, "")
	return append(lines, th.renderIssueInspectorBox(width, *cur)...)
}

func (th ansiTheme) renderIssueInspectorBox(width int, cur IssueRow) []string {
	panelW := adaptivePanelWidth(width)
	textW := max(44, panelW-18)
	actionHint := fmt.Sprintf(
		"%s ▶ FIX THIS BUG: Press [Enter] or [a] to start AI Agent %s   %s[x] Batch Select   [d] Dismiss   [:open %s] Inspect Code%s",
		th.okBg, th.reset, th.cyan, cur.Path, th.reset,
	)
	if cur.Status == "in_progress" || cur.AgentPath != "" {
		actionHint = fmt.Sprintf(
			"%s ⟳ AGENT WORKING ON THIS ISSUE %s   Press %s[Enter]%s or %s[2]%s to open the live 4-Stage Agent Pipeline",
			th.warnBg, th.reset, th.cyan, th.reset, th.cyan, th.reset,
		)
	}
	body := []string{
		fmt.Sprintf("%sSelected:%s %s %s (%s:%d)", th.bold, th.reset, th.severityBadge(cur.Severity), cur.Title, cur.Path, cur.Line),
		fmt.Sprintf("%s▲ Cause:%s    %s%s%s", th.bold+th.red, th.reset, th.amber, truncatePlain(cur.Cause, textW), th.reset),
		fmt.Sprintf("%s✔ Solution:%s %s%s%s", th.bold+th.green, th.reset, th.fg, truncatePlain(cur.Solution, textW), th.reset),
		actionHint,
	}
	return th.renderPanelBox("BUG DIAGNOSTIC & AI AGENT DISPATCH", strings.ToUpper(cur.Category), panelW, body)
}

func (s *State) renderProjectPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := []string{
		fmt.Sprintf("  %s[1] OVERVIEW · PROJECT & VERIFICATION TESTS%s   %s([t/Enter] Run Checks · :reindex · :project <path>)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	if s.Project != nil {
		meta := []string{
			fmt.Sprintf("Repository: %s%s%s (%s) · %d files · %d lines", th.bold, s.Project.Name, th.reset, s.Project.Path, s.Project.FileCount, s.Project.TotalLines),
		}
		if s.Project.Type != "" || s.Project.BuildFile != "" {
			meta = append(meta, fmt.Sprintf("Stack Type: %s · Build Manifest: %s", firstNonEmpty(s.Project.Type, "polyglot"), firstNonEmpty(s.Project.BuildFile, "auto")))
		}
		lines = append(lines, th.renderPanelBox("REPOSITORY PROFILE", "WORKSPACE", panelW, meta)...)
	} else {
		lines = append(lines, "  No project loaded. Use :project <path> to open a repository.")
	}
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return append(lines, "  No scan phases recorded yet. Press [t] or :test to run verification checks.")
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   PHASE             STATUS        STATE         COMMAND", width)...)
	selected := s.ClampedIdx()
	for idx, ph := range s.Scan.Phases {
		cursor := th.renderCursorPrefix(idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-16s  %-12s  state:%-10s  cmd:%s",
			cursor, ph.Name, th.statusBadge(ph.State), ph.State, strings.Join(ph.Command, " "),
		))
		if strings.EqualFold(ph.State, "failed") && strings.TrimSpace(ph.Output) != "" {
			lines = append(lines, fmt.Sprintf("    %s▲ Check Failure:%s %s%s%s", th.critBg, th.reset, th.red, truncatePlain(strings.TrimSpace(ph.Output), max(40, width-24)), th.reset))
		}
	}
	return lines
}

func (s *State) renderAgentsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	items := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	activeStage := 1
	if len(items) > 0 {
		activeStage = items[s.ClampedIdx()].StageNumber
	}
	stepper := renderPipelineStepper(th, activeStage)
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · 4-STAGE AUTONOMOUS PIPELINE%s   %s(Fix Bugs & Build Features with Guarded Verification)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	lines = append(lines, th.renderPanelBox("GUARDED EXECUTION PIPELINE", "4-STAGE WORKFLOW", panelW, []string{stepper})...)
	if len(items) == 0 {
		guide := []string{
			"No active agent sessions running right now.",
			fmt.Sprintf("1. %sFix a Bug:%s Press %s[Tab]%s to open %sIssues%s, select a bug with %s[j/k]%s, and press %s[Enter]%s or %s[a]%s.", th.bold+th.green, th.reset, th.cyan, th.reset, th.accent, th.reset, th.cyan, th.reset, th.green, th.reset, th.green, th.reset),
			fmt.Sprintf("2. %sBuild a Feature:%s Press %s[Tab]%s twice to open %sFeatures%s, select an idea, and press %s[Enter]%s or %s[a]%s.", th.bold+th.purple, th.reset, th.cyan, th.reset, th.accent, th.reset, th.green, th.reset, th.green, th.reset),
			fmt.Sprintf("3. %sCustom Mission:%s Type %s:agent <describe bug fix or feature>%s and press %s[Enter]%s.", th.bold+th.cyan, th.reset, th.green, th.reset, th.cyan, th.reset),
		}
		return append(lines, th.renderPanelBox("HOW TO START AN AI AGENT MISSION", "QUICK GUIDE", panelW, guide)...)
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   STAGE                     MISSION TITLE                    SCOPE        DELTA        VERIFICATION", width)...)
	selected := s.ClampedIdx()
	titleCol := max(30, width-78)
	for idx, item := range items {
		cursor := th.renderCursorPrefix(idx == selected)
		stageLabel := fmt.Sprintf("Stage %d (%s)", item.StageNumber, item.Stage)
		lines = append(lines, fmt.Sprintf(
			"  %s%-24s · %-*s %s (+%d/-%d) checks:%d/%d %s",
			cursor, stageLabel, titleCol, truncatePlain(item.Title, titleCol), th.statusBadge(item.ScopeBadge),
			item.AddedLines, item.RemovedLines, item.ChecksPassed, item.ChecksTotal,
			th.renderProgressBar(item.ChecksPassed, max(1, item.ChecksTotal), 8),
		))
	}
	curItem := items[selected]
	detail := []string{
		fmt.Sprintf("Mission: %s%s%s · Target: %s · Branch: %s · Status: %s", th.bold, curItem.Title, th.reset, firstNonEmpty(curItem.Path, "workspace"), curItem.Branch, th.statusBadge(curItem.Status)),
		fmt.Sprintf("Actions: %s[c] Apply & Commit%s   %s[3] Review Code Diff%s   %s[r] Repair/Recheck%s   %s[m] Merge Worktree%s   %s[u] Guarded Undo%s", th.green, th.reset, th.cyan, th.reset, th.amber, th.reset, th.purple, th.reset, th.red, th.reset),
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
		{1, "1. Architect & Code"},
		{2, "2. Verify & Test"},
		{3, "3. Critic Review"},
		{4, "4. Human Review"},
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
	return "Pipeline: " + strings.Join(parts, th.border+" ──▶ "+th.reset)
}

func (s *State) renderBugsPage(th ansiTheme, width int) []string {
	batchBadge := ""
	if len(s.SelectedIssueIDs) > 0 {
		batchBadge = fmt.Sprintf("   %s BATCH SELECTED: %d (Press [a] to fix all) %s", th.warnBg, len(s.SelectedIssueIDs), th.reset)
	}
	lines := []string{
		fmt.Sprintf(
			"  %s[2] AGENT STUDIO · ISSUES & BUGS%s   Category [t/c/f]: %s[%s]%s   Status [s/v]: %s[%s]%s%s",
			th.accent, th.reset, th.purple, strings.ToUpper(s.IssueCategory), th.reset, th.green, strings.ToUpper(s.IssueView), th.reset, batchBadge,
		),
		"",
	}
	lines = append(lines, th.renderTableHeader("   SEL  SEVERITY    CATEGORY    FINDING                            LOCATION                  STATUS", width)...)
	filtered := s.FilteredIssues()
	if len(filtered) == 0 {
		return append(lines, "  No issues matching current filter. Press [t] to cycle category or [r] to run analysis.")
	}
	nameCol := max(33, width-72)
	selected := s.ClampedIdx()
	for idx, row := range filtered {
		cursor := th.renderCursorPrefix(idx == selected)
		mark := "[ ]"
		if containsString(s.SelectedIssueIDs, row.ID) {
			mark = th.amber + "[x]" + th.reset
		}
		loc := truncatePlain(fmt.Sprintf("%s:%d", row.Path, row.Line), 24)
		lines = append(lines, fmt.Sprintf(
			"  %s%s %s  %-10s  %-*s  %-24s  %s",
			cursor, mark, th.severityBadge(row.Severity), row.Category, nameCol, truncatePlain(row.Name, nameCol), loc, th.statusBadge(row.Status),
		))
	}
	return s.appendSelectedIssueInspector(th, width, lines)
}

func (s *State) renderFeaturesPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · FEATURES & ENHANCEMENTS%s   %s([Enter/a] Build Feature with Agent · [:agent <spec>] Custom Feature · [d] Dismiss)%s", th.accent, th.reset, th.cyan, th.reset),
		"",
	}
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, "  No active feature suggestions. Run :analyze to generate goal-aligned feature ideas, or type :agent <feature>.")
	}
	lines = append(lines, th.renderTableHeader("   EFFORT     FEATURE PROPOSAL                        STRATEGIC BENEFIT", width)...)
	selected := s.ClampedIdx()
	for idx, f := range features {
		cursor := th.renderCursorPrefix(idx == selected)
		effort := strings.ToUpper(firstNonEmpty(f.Effort, "med"))
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %-38s  Benefit: %s",
			cursor, th.severityBadge(effort), truncatePlain(f.Title, 38), truncatePlain(f.Benefit, max(28, width-62)),
		))
	}
	cur := features[selected]
	featBox := []string{
		fmt.Sprintf("%sFeature:%s %s   %sEffort:%s %s", th.bold, th.reset, cur.Title, th.muted, th.reset, th.severityBadge(firstNonEmpty(cur.Effort, "med"))),
		fmt.Sprintf("%sBenefit:%s %s", th.green, th.reset, truncatePlain(cur.Benefit, max(40, panelW-14))),
		fmt.Sprintf("%s ▶ BUILD THIS FEATURE: Press [Enter] or [a] to dispatch AI Agent %s   %s[d] Dismiss   [:agent <goal>] Custom Feature%s", th.okBg, th.reset, th.cyan, th.reset),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("FEATURE IMPLEMENTATION WORKBENCH", "AI AGENT BUILDER", panelW, featBox)...)
	return lines
}

func (s *State) renderAnalysisRunPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · LAST ANALYSIS RUN%s   %s([r] Run/Resume · :pause · :repair)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	if s.Run == nil {
		return append(lines, "  No analysis run recorded. Press [r] or :analyze to start.")
	}
	done, total := runAggregateCoverage(s.Run)
	summaryLine := fmt.Sprintf(
		"Run ID: %s%s%s · Status: %s · Elapsed: %ds · Coverage: %s",
		th.bold, s.Run.Identity.ID, th.reset, th.statusBadge(s.Run.Status), s.Run.ElapsedSeconds, th.renderProgressBar(done, total, 16),
	)
	lines = append(lines, th.renderPanelBox("ANALYSIS TELEMETRY", strings.ToUpper(s.Run.Status), panelW, []string{summaryLine})...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   SECTION         STATUS        PROGRESS             SUCCEEDED    FAILED", width)...)
	selected := s.ClampedIdx()
	for idx, sec := range s.Run.Sections {
		cursor := th.renderCursorPrefix(idx == selected)
		bar := th.renderProgressBar(sec.Coverage.Succeeded, sec.Coverage.Total, 12)
		failStr := fmt.Sprintf("failed:%d", sec.Coverage.Failed)
		if sec.Coverage.Failed > 0 {
			failStr = th.critBg + " " + failStr + " " + th.reset
		}
		lines = append(lines, fmt.Sprintf(
			"  %s%-14s  %-12s  %s   succeeded:%d/%d %s",
			cursor, strings.ToUpper(sec.Category), th.statusBadge(sec.Status), bar, sec.Coverage.Succeeded, sec.Coverage.Total, failStr,
		))
	}
	return lines
}

func (s *State) renderAnalysisFilesPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · FILE SCOPE%s   %s([x/Space/Enter] Toggle Include/Exclude)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, "  No indexed files available.")
	}
	total := len(s.Index.Files)
	excl := len(s.ExcludedPaths)
	incl := max(0, total-excl)
	lines = append(lines, fmt.Sprintf("  %sScope Summary:%s %d Included · %d Excluded · %d Total Indexed Files", th.muted, th.reset, incl, excl, total), "")
	lines = append(lines, th.renderTableHeader("   SCOPE     REPOSITORY PATH                             LANGUAGE      LINES", width)...)
	selected := s.ClampedIdx()
	pathCol := max(42, width-40)
	for idx, f := range s.Index.Files {
		cursor := th.renderCursorPrefix(idx == selected)
		scopeTag := th.statusBadge("INCL")
		if containsString(s.ExcludedPaths, f.Path) {
			scopeTag = th.statusBadge("EXCL")
		}
		lines = append(lines, fmt.Sprintf(
			"  %s%s   %-*s  (%s, %d lines)",
			cursor, scopeTag, pathCol, truncatePlain(f.Path, pathCol), firstNonEmpty(f.Language, "text"), f.LineCount,
		))
	}
	return lines
}

func (s *State) renderEditorPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := []string{
		fmt.Sprintf(
			"  %s[3] CODE & DIFF · INSPECTOR%s   %s([e] Inline Why: %v · [x] Reject Hunk · [c] Apply · [u] Undo)%s",
			th.accent, th.reset, th.cyan, s.ShowInlineWhy, th.reset,
		),
	}
	if s.Change != nil && len(s.Change.Changes) > 0 {
		lines = append(lines, s.renderChangeDiffLines(th, width)...)
		return lines
	}
	if s.File != nil {
		header := fmt.Sprintf("File: %s%s%s (%s) · %d lines", th.bold, s.File.Path, th.reset, firstNonEmpty(s.File.Language, "text"), s.File.LineCount)
		lines = append(lines, th.renderPanelBox("READ-ONLY SOURCE VIEWER", "SOURCE", panelW, []string{header})...)
		if s.File.Content != "" {
			for idx, l := range strings.Split(s.File.Content, "\n") {
				if idx >= 16 {
					break
				}
				lines = append(lines, fmt.Sprintf("  %s%4d │%s %s", th.muted, idx+1, th.reset, l))
			}
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
			"Proposal: %s%s%s · File: %s %s (%s+%d%s/%s-%d%s) · Hunks: %d",
			th.bold, s.Change.Title, th.reset, fc.Path, th.statusBadge(badge),
			th.green, added, th.reset, th.red, removed, th.reset, len(hunks),
		),
	}
	if s.ShowInlineWhy && fc.Summary != "" {
		meta = append(meta, fmt.Sprintf("%sWhy:%s %s", th.accent, th.reset, fc.Summary))
	}
	lines := th.renderPanelBox("GUARDED DIFF REVIEW", badge, panelW, meta)
	lines = append(lines, th.renderTableHeader("  SIGN  LINE │ UNIFIED DIFF CONTENT", width)...)
	for _, dl := range fc.Diff.Lines {
		switch {
		case IsAddedLine(dl.Kind):
			lines = append(lines, fmt.Sprintf("  %s+ %4d │ %s%s", th.green, dl.NewLine, dl.Text, th.reset))
		case IsRemovedLine(dl.Kind):
			lines = append(lines, fmt.Sprintf("  %s- %4d │ %s%s", th.red, dl.OldLine, dl.Text, th.reset))
		default:
			lines = append(lines, fmt.Sprintf("    %4d │ %s", dl.NewLine, dl.Text))
		}
	}
	return lines
}

func (s *State) renderIndexedFilesInEditor(th ansiTheme, width int, lines []string) []string {
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, "  No file or diff loaded. Use :open <path> or select an indexed file.")
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   REPOSITORY FILE PATH                        LANGUAGE      LINES", width)...)
	selected := s.ClampedIdx()
	for idx, f := range s.Index.Files {
		cursor := th.renderCursorPrefix(idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%-44s (%s)  %d lines", cursor, f.Path, firstNonEmpty(f.Language, "text"), f.LineCount))
	}
	return lines
}

func (s *State) renderGitPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[3] CODE & DIFF · GIT & WORKTREES%s   %s([m] Merge Worktree · :git)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	if s.GitWorkspace == nil || len(s.GitWorkspace.Worktrees) == 0 {
		return append(lines, "  No Git worktrees reported.")
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   ROLE          BRANCH               WORKTREE PATH", width)...)
	selected := s.ClampedIdx()
	for idx, wt := range s.GitWorkspace.Worktrees {
		cursor := th.renderCursorPrefix(idx == selected)
		role := "WORKTREE"
		if wt.Current {
			role = "CURRENT"
		} else if wt.Mergeable {
			role = "MERGEABLE"
		}
		lines = append(lines, fmt.Sprintf("  %s%-12s  branch:%-18s path:%s", cursor, th.statusBadge(role), wt.Branch, wt.Path))
	}
	return lines
}

func (s *State) renderHistoryPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[3] CODE & DIFF · SAVED PROPOSALS%s   %s([Enter] Load Proposal · :proposals)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	if len(s.ChangeHistory) == 0 {
		return append(lines, "  No saved proposals in history.")
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   STATE        PROPOSAL TITLE                        REVISION    FRESHNESS", width)...)
	selected := s.ClampedIdx()
	for idx, entry := range s.ChangeHistory {
		cursor := th.renderCursorPrefix(idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-12s %-36s rev:%d (%s)",
			cursor, th.statusBadge(entry.State), truncatePlain(entry.Title, 36), entry.Revision, firstNonEmpty(entry.Freshness, "current"),
		))
	}
	return lines
}

func (s *State) renderDocsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := []string{
		fmt.Sprintf(
			"  %s[4] HANDBOOK & ARCH · ENGINEERING SPECIFICATION%s   Section [h/l]: %s[%s]%s",
			th.accent, th.reset, th.purple, strings.ToUpper(s.DocsSection), th.reset,
		),
	}
	ctxLines := []string{
		fmt.Sprintf("%sPurpose:%s  %s", th.bold, th.reset, docs.Context.Purpose),
		fmt.Sprintf("%sTopology:%s %s", th.bold, th.reset, docs.Context.Topology),
	}
	lines = append(lines, th.renderPanelBox("SYSTEM CONTEXT & TOPOLOGY", strings.ToUpper(docs.Status), panelW, ctxLines)...)
	lines = append(lines, "")
	switch s.DocsSection {
	case "components":
		lines = append(lines, th.renderTableHeader("   LAYER          COMPONENT                 PATH                        SUMMARY", width)...)
		for _, c := range docs.Components {
			lines = append(lines, fmt.Sprintf("  • [%s] %s (%s) — %s", c.Layer, c.Name, c.Path, c.Summary))
		}
	case "scenarios":
		lines = append(lines, th.renderTableHeader("   RUNTIME SCENARIO & INVARIANT SUMMARY", width)...)
		for _, sc := range docs.Scenarios {
			lines = append(lines, fmt.Sprintf("  • %s: %s", sc.Title, sc.Summary))
		}
	case "decisions":
		lines = append(lines, th.renderTableHeader("   CATEGORY       ARCHITECTURAL DECISION RECORD (ADR)", width)...)
		for _, d := range docs.Decisions {
			lines = append(lines, fmt.Sprintf("  • [%s] %s — %s", d.Category, d.Title, d.Decision))
		}
	default:
		lines = append(lines, th.renderTableHeader("   DEPENDENCY & SUBSYSTEM CATALOG", width)...)
		for _, dep := range docs.Dependencies {
			lines = append(lines, fmt.Sprintf("  • %s (%s): %s", dep.Name, dep.Kind, dep.Purpose))
		}
	}
	return lines
}

func (s *State) renderDiagramsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := []string{
		fmt.Sprintf("  %s[4] HANDBOOK & ARCH · ARCHITECTURE DIAGRAM%s   %s([Tab] Specification · [r] Refresh)%s", th.accent, th.reset, th.cyan, th.reset),
	}
	diagramLines := RenderArchitectureASCII(docs)
	lines = append(lines, th.renderPanelBox("COMPONENT TOPOLOGY BLUEPRINT", docs.Context.Topology, panelW, diagramLines)...)
	return lines
}

func (s *State) renderInstructionsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := []string{
		fmt.Sprintf(
			"  %s[5] GOVERNANCE & RULES%s   Section [Tab/h/l]: %s[%s]%s   %s([t] Trust · [b] Bootstrap · [g] Graduate Lesson)%s",
			th.accent, th.reset, th.purple, strings.ToUpper(s.RulesSection), th.reset, th.cyan, th.reset,
		),
		"",
	}
	switch s.RulesSection {
	case "models":
		lines = append(lines, th.renderTableHeader("   PRESET PROFILE                 STATE       ANALYZE        BUG            FUNCTION", width)...)
		for idx, p := range DefaultModelPresets() {
			cursor := th.renderCursorPrefix(idx == s.ClampedIdx())
			active := "         "
			if p.ID == s.ActivePresetID {
				active = th.green + "(ACTIVE) " + th.reset
			}
			lines = append(lines, fmt.Sprintf(
				"  %s%-28s %s [analyze:%s bug:%s function:%s]",
				cursor, p.Name, active, p.Analyze, p.Bug, p.Function,
			))
		}
	case "bootstrap":
		pillars := EvaluateBootstrapChecklist(s.Index, s.Instructions)
		readyCount := countReadyPillars(pillars)
		lines = append(lines, fmt.Sprintf("  %sGovernance Readiness:%s %s (%d/%d pillars active)", th.muted, th.reset, th.renderProgressBar(readyCount, len(pillars), 14), readyCount, len(pillars)), "")
		lines = append(lines, th.renderTableHeader("   STATUS      GOVERNANCE PILLAR              DESCRIPTION", width)...)
		for idx, pil := range pillars {
			cursor := th.renderCursorPrefix(idx == s.ClampedIdx())
			badge := th.statusBadge("MISSING")
			if pil.Ready {
				badge = th.statusBadge("READY")
			}
			lines = append(lines, fmt.Sprintf("  %s%-10s %-28s — %s", cursor, badge, pil.Title, pil.Description))
		}
	case "lessons":
		lines = append(lines, th.renderTableHeader("   HARNESS² GUARDRAIL LESSONS (Press [g/Enter] to graduate into AGENTS.md)", width)...)
		for idx, rule := range s.AllHarnessLessons() {
			cursor := th.renderCursorPrefix(idx == s.ClampedIdx())
			lines = append(lines, fmt.Sprintf("  %s• %s", cursor, rule))
		}
	default:
		trustBadge := th.statusBadge("TRUSTED")
		if !s.WorkspaceTrust {
			trustBadge = th.statusBadge("UNTRUSTED")
		}
		body := []string{
			fmt.Sprintf("Execution Trust: %v %s (Press [t] or [Enter] to toggle)", s.WorkspaceTrust, trustBadge),
			"Policy: Project-code checks run in isolated worktrees with controlled argv and bounded output.",
		}
		lines = append(lines, th.renderPanelBox("EXECUTION TRUST GOVERNANCE", "SANDBOX POLICY", panelW, body)...)
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
