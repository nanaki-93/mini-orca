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
		return s.renderEditorPage(th)
	case PageGit:
		return s.renderGitPage(th)
	case PageHistory:
		return s.renderHistoryPage(th)
	case PageDocs:
		return s.renderDocsPage(th)
	case PageDiagrams:
		return s.renderDiagramsPage(th)
	case PageInstructions:
		return s.renderInstructionsPage(th)
	default:
		return s.renderSummaryPage(th, width)
	}
}

func (s *State) renderSummaryPage(th ansiTheme, width int) []string {
	allIssues := s.CollectAllIssues()
	summary := ComputeIssueSummary(allIssues, s.Features)
	cards := [][2]string{
		{"OPEN FINDINGS", fmt.Sprintf("%s%d%s open (%d active)", th.bold, summary.TotalOpen, th.reset, summary.InProgress)},
		{"RESOLVED", fmt.Sprintf("%s%d%s verified", th.green, summary.Done, th.reset)},
		{"RISK PROFILE", fmt.Sprintf("B:%d  S:%d  P:%d", summary.Bugs, summary.Security, summary.Performance)},
		{"FEATURE IDEAS", fmt.Sprintf("%s%d%s queued", th.cyan, summary.Features, th.reset)},
	}
	lines := []string{
		fmt.Sprintf("  %s[1] OVERVIEW · DASHBOARD%s   %s([j/k] Move · [Enter/a] Dispatch Agent · [r] Run Analysis · [t] Run Checks)%s", th.accent, th.reset, th.muted, th.reset),
	}
	lines = append(lines, th.renderKPICards(cards, width)...)
	lines = append(lines,
		fmt.Sprintf(
			"  %sSummary:%s Open Issues: %d  │  In Progress: %d  │  Resolved: %d  │  Bugs: %d  Sec: %d  Perf: %d  Features: %d",
			th.muted, th.reset, summary.TotalOpen, summary.InProgress, summary.Done, summary.Bugs, summary.Security, summary.Performance, summary.Features,
		),
		"",
	)
	lines = append(lines, th.renderTableHeader("   SEVERITY    CATEGORY    FINDING                              LOCATION                     STATUS", width)...)
	open := s.openIssues()
	if len(open) > 8 {
		open = open[:8]
	}
	if len(open) == 0 {
		return append(lines, "  (No open issues — press [r] or :analyze to run project analysis)")
	}
	selected := s.ClampedIdx()
	for idx, row := range open {
		cursor := th.renderCursorPrefix(idx == selected)
		loc := truncatePlain(fmt.Sprintf("%s:%d", row.Path, row.Line), 26)
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %-10s  %-35s  %-27s  %s",
			cursor, th.severityBadge(row.Severity), strings.ToUpper(row.Category), truncatePlain(row.Name, 35), loc, th.statusBadge(row.Status),
		))
	}
	if cur := s.SelectedIssue(); cur != nil {
		lines = append(lines, "")
		lines = append(lines, th.renderIssueInspectorBox(th, width, *cur)...)
	}
	return lines
}

func (th ansiTheme) renderIssueInspectorBox(_ ansiTheme, width int, cur IssueRow) []string {
	body := []string{
		fmt.Sprintf("%sSelected:%s %s (%s:%d)", th.bold, th.reset, cur.Title, cur.Path, cur.Line),
		fmt.Sprintf("%sCause:%s    %s", th.amber, th.reset, truncatePlain(cur.Cause, max(40, width-18))),
		fmt.Sprintf("%sSolution:%s %s", th.green, th.reset, truncatePlain(cur.Solution, max(40, width-18))),
	}
	return th.renderPanelBox("FINDING DIAGNOSTIC INSPECTOR", strings.ToUpper(cur.Category), min(104, width-6), body)
}

func (s *State) renderProjectPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[1] OVERVIEW · PROJECT & TESTS%s   %s([t/Enter] Run Checks · :reindex · :project <path>)%s", th.accent, th.reset, th.muted, th.reset),
	}
	if s.Project != nil {
		meta := []string{
			fmt.Sprintf("Repository: %s%s%s (%s) · %d files · %d lines", th.bold, s.Project.Name, th.reset, s.Project.Path, s.Project.FileCount, s.Project.TotalLines),
		}
		if s.Project.Type != "" || s.Project.BuildFile != "" {
			meta = append(meta, fmt.Sprintf("Stack Type: %s · Build Manifest: %s", firstNonEmpty(s.Project.Type, "polyglot"), firstNonEmpty(s.Project.BuildFile, "auto")))
		}
		lines = append(lines, th.renderPanelBox("REPOSITORY PROFILE", "WORKSPACE", min(104, width-6), meta)...)
	} else {
		lines = append(lines, "  No project loaded. Use :project <path> to open a repository.")
	}
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return append(lines, "  No scan phases recorded yet. Press [t] or :test to run verification checks.")
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   PHASE             STATUS        COMMAND", width)...)
	selected := s.ClampedIdx()
	for idx, ph := range s.Scan.Phases {
		cursor := th.renderCursorPrefix(idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%-16s  %-12s  state:%-10s  cmd:%s",
			cursor, ph.Name, th.statusBadge(ph.State), ph.State, strings.Join(ph.Command, " "),
		))
	}
	return lines
}

func (s *State) renderAgentsPage(th ansiTheme, width int) []string {
	items := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	activeStage := 1
	if len(items) > 0 {
		activeStage = items[s.ClampedIdx()].StageNumber
	}
	stepper := renderPipelineStepper(th, activeStage)
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · 4-STAGE PIPELINE%s", th.accent, th.reset),
	}
	lines = append(lines, th.renderPanelBox("GUARDED EXECUTION PIPELINE", "4-STAGE WORKFLOW", min(104, width-6), []string{stepper})...)
	if len(items) == 0 {
		return append(lines, "  No active agent sessions. Press [Tab] to view Issues or use :agent <objective>.")
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   STAGE                     MISSION TITLE                    SCOPE        DELTA        VERIFICATION", width)...)
	selected := s.ClampedIdx()
	for idx, item := range items {
		cursor := th.renderCursorPrefix(idx == selected)
		stageLabel := fmt.Sprintf("Stage %d (%s)", item.StageNumber, item.Stage)
		lines = append(lines, fmt.Sprintf(
			"  %s%-24s · %-30s %s (+%d/-%d) checks:%d/%d %s",
			cursor, stageLabel, truncatePlain(item.Title, 30), th.statusBadge(item.ScopeBadge),
			item.AddedLines, item.RemovedLines, item.ChecksPassed, item.ChecksTotal,
			th.renderProgressBar(item.ChecksPassed, max(1, item.ChecksTotal), 8),
		))
	}
	lines = append(lines, "", fmt.Sprintf("  %sActions:%s [c] Apply & Commit   [m] Merge Worktree   [u] Guarded Undo   [r] Repair/Recheck", th.cyan, th.reset))
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
			parts = append(parts, fmt.Sprintf("%s[%s]%s", th.accent, st.label, th.reset))
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
		batchBadge = fmt.Sprintf("   Batch Selected: %s%d%s", th.amber, len(s.SelectedIssueIDs), th.reset)
	}
	lines := []string{
		fmt.Sprintf(
			"  %s[2] AGENT STUDIO · ISSUES%s   Category [t/c/f]: %s[%s]%s   Status [s/v]: %s[%s]%s%s",
			th.accent, th.reset, th.purple, strings.ToUpper(s.IssueCategory), th.reset, th.green, strings.ToUpper(s.IssueView), th.reset, batchBadge,
		),
		"",
	}
	lines = append(lines, th.renderTableHeader("   SEL  SEVERITY    CATEGORY    FINDING                            LOCATION                  STATUS", width)...)
	filtered := s.FilteredIssues()
	if len(filtered) == 0 {
		return append(lines, "  No issues matching current filter.")
	}
	selected := s.ClampedIdx()
	for idx, row := range filtered {
		cursor := th.renderCursorPrefix(idx == selected)
		mark := "[ ]"
		if containsString(s.SelectedIssueIDs, row.ID) {
			mark = th.amber + "[x]" + th.reset
		}
		loc := truncatePlain(fmt.Sprintf("%s:%d", row.Path, row.Line), 24)
		lines = append(lines, fmt.Sprintf(
			"  %s%s %s  %-10s  %-33s  %-24s  %s",
			cursor, mark, th.severityBadge(row.Severity), row.Category, truncatePlain(row.Name, 33), loc, th.statusBadge(row.Status),
		))
	}
	if cur := s.SelectedIssue(); cur != nil {
		lines = append(lines, "")
		lines = append(lines, th.renderIssueInspectorBox(th, width, *cur)...)
	}
	return lines
}

func (s *State) renderFeaturesPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · FEATURES%s   %s([a/Enter] Dispatch to Agent · [d] Dismiss · [r] Refresh)%s", th.accent, th.reset, th.muted, th.reset),
		"",
	}
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, "  No active feature suggestions. Run :analyze to generate goal-aligned feature ideas.")
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
	return lines
}

func (s *State) renderAnalysisRunPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · LAST ANALYSIS RUN%s   %s([r] Run/Resume · :pause · :repair)%s", th.accent, th.reset, th.muted, th.reset),
	}
	if s.Run == nil {
		return append(lines, "  No analysis run recorded. Press [r] or :analyze to start.")
	}
	done, total := runAggregateCoverage(s.Run)
	summaryLine := fmt.Sprintf(
		"Run ID: %s%s%s · Status: %s · Elapsed: %ds · Coverage: %s",
		th.bold, s.Run.Identity.ID, th.reset, th.statusBadge(s.Run.Status), s.Run.ElapsedSeconds, th.renderProgressBar(done, total, 14),
	)
	lines = append(lines, th.renderPanelBox("ANALYSIS TELEMETRY", strings.ToUpper(s.Run.Status), min(104, width-6), []string{summaryLine})...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   SECTION         STATUS        PROGRESS             SUCCEEDED    FAILED", width)...)
	selected := s.ClampedIdx()
	for idx, sec := range s.Run.Sections {
		cursor := th.renderCursorPrefix(idx == selected)
		bar := th.renderProgressBar(sec.Coverage.Succeeded, sec.Coverage.Total, 12)
		lines = append(lines, fmt.Sprintf(
			"  %s%-14s  %-12s  %s   succeeded:%d/%d failed:%d",
			cursor, strings.ToUpper(sec.Category), th.statusBadge(sec.Status), bar, sec.Coverage.Succeeded, sec.Coverage.Total, sec.Coverage.Failed,
		))
	}
	return lines
}

func (s *State) renderAnalysisFilesPage(th ansiTheme, width int) []string {
	lines := []string{
		fmt.Sprintf("  %s[2] AGENT STUDIO · FILE SCOPE%s   %s([x/Space/Enter] Toggle Include/Exclude)%s", th.accent, th.reset, th.muted, th.reset),
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
	for idx, f := range s.Index.Files {
		cursor := th.renderCursorPrefix(idx == selected)
		scopeTag := th.statusBadge("INCL")
		if containsString(s.ExcludedPaths, f.Path) {
			scopeTag = th.statusBadge("EXCL")
		}
		lines = append(lines, fmt.Sprintf(
			"  %s%s   %-42s  (%s, %d lines)",
			cursor, scopeTag, truncatePlain(f.Path, 42), firstNonEmpty(f.Language, "text"), f.LineCount,
		))
	}
	return lines
}

func (s *State) renderEditorPage(th ansiTheme) []string {
	lines := []string{
		fmt.Sprintf("%s[3] CODE & DIFF · INSPECTOR%s  ([e] Inline Why: %v · [x] Reject Hunk · [c] Apply · [u] Undo)", th.accent, th.reset, s.ShowInlineWhy),
	}
	if s.Change != nil && len(s.Change.Changes) > 0 {
		lines = append(lines, s.renderChangeDiffLines(th)...)
		return lines
	}
	if s.File != nil {
		lines = append(lines, fmt.Sprintf("  File: %s (%s)", s.File.Path, s.File.Language))
		if s.File.Content != "" {
			for idx, l := range strings.Split(s.File.Content, "\n") {
				if idx >= 12 {
					break
				}
				lines = append(lines, fmt.Sprintf("  %4d │ %s", idx+1, l))
			}
		}
		return lines
	}
	return s.renderIndexedFilesInEditor(th, lines)
}

func (s *State) renderChangeDiffLines(th ansiTheme) []string {
	fc := s.Change.Changes[0]
	added, removed, badge := ClassifyDiffScope(s.Change.Changes)
	lines := []string{
		fmt.Sprintf("  Proposal: %s · File: %s [%s +%d/-%d]", s.Change.Title, fc.Path, badge, added, removed),
	}
	if s.ShowInlineWhy && fc.Summary != "" {
		lines = append(lines, fmt.Sprintf("  %sWhy:%s %s", th.accent, th.reset, fc.Summary))
	}
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

func (s *State) renderIndexedFilesInEditor(th ansiTheme, lines []string) []string {
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, "  No file or diff loaded. Use :open <path> or select an indexed file.")
	}
	selected := s.ClampedIdx()
	for idx, f := range s.Index.Files {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf("%s%-44s (%s)", cursor, f.Path, f.Language))
	}
	return lines
}

func (s *State) renderGitPage(th ansiTheme) []string {
	lines := []string{fmt.Sprintf("%s[3] CODE & DIFF · GIT & WORKTREES%s  ([m] Merge Worktree)", th.accent, th.reset)}
	if s.GitWorkspace == nil || len(s.GitWorkspace.Worktrees) == 0 {
		return append(lines, "  No Git worktrees reported.")
	}
	selected := s.ClampedIdx()
	for idx, wt := range s.GitWorkspace.Worktrees {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		role := "WORKTREE"
		if wt.Current {
			role = "CURRENT"
		} else if wt.Mergeable {
			role = "MERGEABLE"
		}
		lines = append(lines, fmt.Sprintf("%s[%-9s] branch:%-18s path:%s", cursor, role, wt.Branch, wt.Path))
	}
	return lines
}

func (s *State) renderHistoryPage(th ansiTheme) []string {
	lines := []string{fmt.Sprintf("%s[3] CODE & DIFF · SAVED PROPOSALS%s  ([Enter] Load Proposal)", th.accent, th.reset)}
	if len(s.ChangeHistory) == 0 {
		return append(lines, "  No saved proposals in history.")
	}
	selected := s.ClampedIdx()
	for idx, entry := range s.ChangeHistory {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf("%s[%-8s] %-36s rev:%d (%s)", cursor, strings.ToUpper(entry.State), entry.Title, entry.Revision, entry.Freshness))
	}
	return lines
}

func (s *State) renderDocsPage(th ansiTheme) []string {
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := []string{
		fmt.Sprintf("%s[4] HANDBOOK & ARCH · ENGINEERING SPECIFICATION%s  Section [h/l]: [%s]", th.accent, th.reset, strings.ToUpper(s.DocsSection)),
		fmt.Sprintf("  Purpose:  %s", docs.Context.Purpose),
		fmt.Sprintf("  Topology: %s", docs.Context.Topology),
		"",
	}
	switch s.DocsSection {
	case "components":
		for _, c := range docs.Components {
			lines = append(lines, fmt.Sprintf("  • [%s] %s (%s) — %s", c.Layer, c.Name, c.Path, c.Summary))
		}
	case "scenarios":
		for _, sc := range docs.Scenarios {
			lines = append(lines, fmt.Sprintf("  • %s: %s", sc.Title, sc.Summary))
		}
	case "decisions":
		for _, d := range docs.Decisions {
			lines = append(lines, fmt.Sprintf("  • [%s] %s — %s", d.Category, d.Title, d.Decision))
		}
	default:
		for _, dep := range docs.Dependencies {
			lines = append(lines, fmt.Sprintf("  • %s (%s): %s", dep.Name, dep.Kind, dep.Purpose))
		}
	}
	return lines
}

func (s *State) renderDiagramsPage(th ansiTheme) []string {
	docs := ResolveProjectDocs(s.Overview, s.Project)
	lines := []string{
		fmt.Sprintf("%s[4] HANDBOOK & ARCH · ARCHITECTURE DIAGRAM%s", th.accent, th.reset),
		"",
	}
	for _, line := range RenderArchitectureASCII(docs) {
		lines = append(lines, "  "+line)
	}
	return lines
}

func (s *State) renderInstructionsPage(th ansiTheme) []string {
	lines := []string{
		fmt.Sprintf(
			"%s[5] GOVERNANCE & RULES%s   Section [Tab/h/l]: [%s]   ([t] Trust · [b] Bootstrap · [g] Graduate Lesson)",
			th.accent, th.reset, strings.ToUpper(s.RulesSection),
		),
	}
	switch s.RulesSection {
	case "models":
		for idx, p := range DefaultModelPresets() {
			cursor := "  "
			if idx == s.ClampedIdx() {
				cursor = th.accent + "▸ " + th.reset
			}
			active := ""
			if p.ID == s.ActivePresetID {
				active = " (ACTIVE)"
			}
			lines = append(lines, fmt.Sprintf("%s%s%s [analyze:%s bug:%s function:%s]", cursor, p.Name, active, p.Analyze, p.Bug, p.Function))
		}
	case "bootstrap":
		for idx, pil := range EvaluateBootstrapChecklist(s.Index, s.Instructions) {
			cursor := "  "
			if idx == s.ClampedIdx() {
				cursor = th.accent + "▸ " + th.reset
			}
			badge := "[MISSING]"
			if pil.Ready {
				badge = "[READY]  "
			}
			lines = append(lines, fmt.Sprintf("%s%s %-28s — %s", cursor, badge, pil.Title, pil.Description))
		}
	case "lessons":
		for idx, rule := range s.AllHarnessLessons() {
			cursor := "  "
			if idx == s.ClampedIdx() {
				cursor = th.accent + "▸ " + th.reset
			}
			lines = append(lines, fmt.Sprintf("%s• %s", cursor, rule))
		}
	default:
		lines = append(lines, fmt.Sprintf("  Execution Trust: %v (Press [t] or [Enter] to toggle)", s.WorkspaceTrust))
	}
	return lines
}
