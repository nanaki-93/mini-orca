package tui

import (
	"fmt"
	"strings"
)

func (s *State) renderActivePage(th ansiTheme) []string {
	switch s.Page {
	case PageSummary:
		return s.renderSummaryPage(th)
	case PageProject:
		return s.renderProjectPage(th)
	case PageAgents:
		return s.renderAgentsPage(th)
	case PageBugs:
		return s.renderBugsPage(th)
	case PageFeatures:
		return s.renderFeaturesPage(th)
	case PageAnalysisRun:
		return s.renderAnalysisRunPage(th)
	case PageAnalysisFiles:
		return s.renderAnalysisFilesPage(th)
	default:
		return s.renderSecondaryPage(th)
	}
}

func (s *State) renderSecondaryPage(th ansiTheme) []string {
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
		return s.renderSummaryPage(th)
	}
}

func (s *State) renderSummaryPage(th ansiTheme) []string {
	allIssues := s.CollectAllIssues()
	summary := ComputeIssueSummary(allIssues, s.Features)
	lines := []string{
		fmt.Sprintf("%s[1] OVERVIEW · DASHBOARD%s", th.accent, th.reset),
		fmt.Sprintf(
			"  Open Issues: %d  │  In Progress: %d  │  Resolved: %d  │  Bugs: %d  Sec: %d  Perf: %d  Features: %d",
			summary.TotalOpen, summary.InProgress, summary.Done, summary.Bugs, summary.Security, summary.Performance, summary.Features,
		),
		"",
		"  Top Open Issues ([j/k] move, [Enter/a] dispatch to Agent, [r] Run Analysis):",
	}
	open := s.openIssues()
	if len(open) > 8 {
		open = open[:8]
	}
	if len(open) == 0 {
		return append(lines, "  (No open issues — press [r] or :analyze to run project analysis)")
	}
	selected := s.ClampedIdx()
	for idx, row := range open {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf("%s[%-8s] %-36s  %s:%d", cursor, strings.ToUpper(row.Severity), row.Name, row.Path, row.Line))
	}
	return lines
}

func (s *State) renderProjectPage(th ansiTheme) []string {
	lines := []string{fmt.Sprintf("%s[1] OVERVIEW · PROJECT & TESTS%s  ([t/Enter] Run Checks)", th.accent, th.reset)}
	if s.Project != nil {
		lines = append(lines, fmt.Sprintf("  Repository: %s (%s) · %d files · %d lines", s.Project.Name, s.Project.Path, s.Project.FileCount, s.Project.TotalLines))
	} else {
		lines = append(lines, "  No project loaded. Use :project <path> to open a repository.")
	}
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return append(lines, "  No scan phases recorded yet. Press [t] or :test to run verification checks.")
	}
	selected := s.ClampedIdx()
	for idx, ph := range s.Scan.Phases {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf("%s%-16s  state:%-10s  cmd:%s", cursor, ph.Name, ph.State, strings.Join(ph.Command, " ")))
	}
	return lines
}

func (s *State) renderAgentsPage(th ansiTheme) []string {
	lines := []string{
		fmt.Sprintf("%s[2] AGENT STUDIO · 4-STAGE PIPELINE%s", th.accent, th.reset),
		"  Pipeline: [1. Architect & Code] ──▶ [2. Verify & Test] ──▶ [3. Critic Review] ──▶ [4. Human Review]",
		"",
	}
	items := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	if len(items) == 0 {
		return append(lines, "  No active agent sessions. Press [Tab] to view Issues or use :agent <objective>.")
	}
	selected := s.ClampedIdx()
	for idx, item := range items {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf(
			"%sStage %d (%s) · %s [%s] (+%d/-%d) checks:%d/%d",
			cursor, item.StageNumber, item.Stage, item.Title, item.ScopeBadge, item.AddedLines, item.RemovedLines, item.ChecksPassed, item.ChecksTotal,
		))
	}
	lines = append(lines, "", "  Actions: [c] Apply & Commit   [m] Merge Worktree   [u] Guarded Undo   [r] Repair/Recheck")
	return lines
}

func (s *State) renderBugsPage(th ansiTheme) []string {
	lines := []string{
		fmt.Sprintf(
			"%s[2] AGENT STUDIO · ISSUES%s   Category [t/c/f]: %s%s%s   Status [s/v]: %s%s%s",
			th.accent, th.reset, th.purple, strings.ToUpper(s.IssueCategory), th.reset, th.green, strings.ToUpper(s.IssueView), th.reset,
		),
	}
	filtered := s.FilteredIssues()
	if len(filtered) == 0 {
		return append(lines, "  No issues matching current filter.")
	}
	selected := s.ClampedIdx()
	for idx, row := range filtered {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		mark := "[ ]"
		if containsString(s.SelectedIssueIDs, row.ID) {
			mark = "[x]"
		}
		lines = append(lines, fmt.Sprintf("%s%s [%-8s] %-6s %-34s %s:%d", cursor, mark, strings.ToUpper(row.Severity), row.Category, row.Name, row.Path, row.Line))
	}
	if cur := s.SelectedIssue(); cur != nil {
		lines = append(lines,
			"",
			fmt.Sprintf("  Selected: %s (%s:%d)", cur.Title, cur.Path, cur.Line),
			fmt.Sprintf("  Cause:    %s", cur.Cause),
			fmt.Sprintf("  Solution: %s", cur.Solution),
		)
	}
	return lines
}

func (s *State) renderFeaturesPage(th ansiTheme) []string {
	lines := []string{fmt.Sprintf("%s[2] AGENT STUDIO · FEATURES%s  ([a/Enter] Dispatch to Agent, [d] Dismiss)", th.accent, th.reset)}
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, "  No active feature suggestions. Run :analyze to generate goal-aligned feature ideas.")
	}
	selected := s.ClampedIdx()
	for idx, f := range features {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf("%s[%-6s] %-38s  Benefit: %s", cursor, strings.ToUpper(firstNonEmpty(f.Effort, "med")), f.Title, f.Benefit))
	}
	return lines
}

func (s *State) renderAnalysisRunPage(th ansiTheme) []string {
	lines := []string{fmt.Sprintf("%s[2] AGENT STUDIO · LAST ANALYSIS RUN%s  ([r] Run/Resume, :pause, :repair)", th.accent, th.reset)}
	if s.Run == nil {
		return append(lines, "  No analysis run recorded. Press [r] or :analyze to start.")
	}
	lines = append(lines, fmt.Sprintf("  Run ID: %s · Status: %s · Elapsed: %ds", s.Run.Identity.ID, strings.ToUpper(s.Run.Status), s.Run.ElapsedSeconds))
	selected := s.ClampedIdx()
	for idx, sec := range s.Run.Sections {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		lines = append(lines, fmt.Sprintf(
			"%s%-14s status:%-10s succeeded:%d/%d failed:%d",
			cursor, strings.ToUpper(sec.Category), sec.Status, sec.Coverage.Succeeded, sec.Coverage.Total, sec.Coverage.Failed,
		))
	}
	return lines
}

func (s *State) renderAnalysisFilesPage(th ansiTheme) []string {
	lines := []string{fmt.Sprintf("%s[2] AGENT STUDIO · FILE SCOPE%s  ([x/Space/Enter] Toggle Include/Exclude)", th.accent, th.reset)}
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, "  No indexed files available.")
	}
	selected := s.ClampedIdx()
	for idx, f := range s.Index.Files {
		cursor := "  "
		if idx == selected {
			cursor = th.accent + "▸ " + th.reset
		}
		scopeTag := "[INCL]"
		if containsString(s.ExcludedPaths, f.Path) {
			scopeTag = "[EXCL]"
		}
		lines = append(lines, fmt.Sprintf("%s%s %-40s (%s, %d lines)", cursor, scopeTag, f.Path, f.Language, f.LineCount))
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
