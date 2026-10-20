package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
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

func (s *State) renderSummaryPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	allIssues := s.CollectAllIssues()
	summary := domain.ComputeIssueSummary(allIssues, s.Features)
	cards := [][2]string{
		{"OPEN FINDINGS", fmt.Sprintf("%s%d%s open %s(%d active)%s", th.bold+th.red, summary.TotalOpen, th.reset, th.muted, summary.InProgress, th.reset)},
		{"RESOLVED", fmt.Sprintf("%s%d%s verified", th.bold+th.green, summary.Done, th.reset)},
		{"RISK BREAKDOWN", fmt.Sprintf("%sB:%d%s  %sS:%d%s  %sP:%d%s", th.bold+th.red, summary.Bugs, th.reset, th.bold+th.amber, summary.Security, th.reset, th.cyan, summary.Performance, th.reset)},
		{"FEATURE IDEAS", fmt.Sprintf("%s%d%s ready", th.bold+th.purple, summary.Features, th.reset)},
	}
	lines := renderPageHeader(th, "[1] OVERVIEW · EXECUTIVE DASHBOARD", "Workspace health & risk telemetry")
	lines = append(lines, th.renderKPICards(cards, width)...)
	denom := max(1, summary.TotalOpen+summary.Done)
	distBody := []string{
		fmt.Sprintf("%sBugs%s            %s  %s%d open%s", th.bold+th.fg, th.reset, th.renderProgressBar(summary.Bugs, denom, 16), th.red, summary.Bugs, th.reset),
		fmt.Sprintf("%sSecurity%s        %s  %s%d open%s", th.bold+th.fg, th.reset, th.renderProgressBar(summary.Security, denom, 16), th.amber, summary.Security, th.reset),
		fmt.Sprintf("%sPerformance%s     %s  %s%d open%s", th.fg, th.reset, th.renderProgressBar(summary.Performance, denom, 16), th.cyan, summary.Performance, th.reset),
		fmt.Sprintf("%sResolved%s        %s  %s%d fixed%s", th.fg, th.reset, th.renderProgressBar(summary.Done, denom, 16), th.green, summary.Done, th.reset),
	}
	lines = append(lines, "")
	lines = append(lines, th.renderPanelBox("", "", panelW, distBody)...)

	focusStatus := fmt.Sprintf("%sWorkspace clean%s", th.green, th.reset)
	if summary.TotalOpen > 0 {
		focusStatus = fmt.Sprintf("%s%d open%s (%s%d bugs%s · %s%d security%s)",
			th.bold+th.amber, summary.TotalOpen, th.reset, th.red, summary.Bugs, th.reset, th.amber, summary.Security, th.reset)
	}
	agentSummary := fmt.Sprintf("%s  None", th.statusBadge("IDLE"))
	if s.HasActiveProposal() {
		agentSummary = fmt.Sprintf("%s  %s%s%s", th.statusBadge(s.Change.State), th.bold+th.fg, s.Change.Title, th.reset)
	}
	scanState := "IDLE"
	if s.Scan != nil && s.Scan.Status != "" {
		scanState = s.Scan.Status
	}
	focusBody := []string{
		fmt.Sprintf("%sTriage:%s        %s", th.bold+th.cyan, th.reset, focusStatus),
		fmt.Sprintf("%sAgent:%s         %s", th.bold+th.purple, th.reset, agentSummary),
		fmt.Sprintf("%sVerification:%s  %s  %s(%d features)%s", th.bold+th.green, th.reset, th.statusBadge(scanState), th.muted, summary.Features, th.reset),
	}
	if s.Run != nil && s.Run.Status != "" {
		runNote := fmt.Sprintf("%s  (%ds)", th.statusBadge(s.Run.Status), s.Run.ElapsedSeconds)
		if s.isAnalysisPausedOrInterrupted() {
			reason := client.FirstNonEmpty(s.Run.Reason, "Window allowance reached")
			runNote = fmt.Sprintf("%s  %s%s%s · %s[r] Resume%s",
				th.statusBadge(s.Run.Status), th.amber, reason, th.reset, th.bold+th.cyan, th.reset)
		}
		focusBody = append(focusBody, fmt.Sprintf("%sAnalysis:%s      %s", th.bold+th.amber, th.reset, runNote))
	}
	lines = append(lines, "")
	return append(lines, th.renderPanelBox("", "", panelW, focusBody)...)
}

func (s *State) renderRepoPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[1] OVERVIEW · REPOSITORY PROFILE", "Indexed codebase & language inventory")
	if s.Project == nil {
		return append(lines, fmt.Sprintf("  %sNo project loaded.%s", th.muted, th.reset))
	}
	branch := "detached"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = s.Repository.Branch
	}
	meta := []string{
		fmt.Sprintf("%sRepository:%s %s%s%s", th.muted, th.reset, th.bold+th.fg, s.Project.Name, th.reset),
		fmt.Sprintf("%sPath:%s       %s%s%s", th.muted, th.reset, th.cyan, s.Project.Path, th.reset),
		fmt.Sprintf("%sStack:%s      %s%s%s · Manifest: %s · Branch: %s%s%s",
			th.muted, th.reset, th.bold+th.fg, client.FirstNonEmpty(s.Project.Type, "polyglot"), th.reset,
			client.FirstNonEmpty(s.Project.BuildFile, "auto"), th.purple, branch, th.reset),
		fmt.Sprintf("%sFootprint:%s  %s%d%s indexed files · %s%d%s lines",
			th.muted, th.reset, th.bold+th.green, s.Project.FileCount, th.reset, th.bold+th.fg, s.Project.TotalLines, th.reset),
	}
	lines = append(lines, th.renderPanelBox("", "", panelW, meta)...)
	if s.Index != nil && len(s.Index.Files) > 0 {
		lines = append(lines, "")
		limit := min(s.adaptiveWindowRows(14, 8), len(s.Index.Files))
		for i := 0; i < limit; i++ {
			f := s.Index.Files[i]
			lines = append(lines, fmt.Sprintf("  %s•%s %-42s %s%s · %d lines%s",
				th.accent, th.reset, f.Path, th.muted, client.FirstNonEmpty(f.Language, "text"), f.LineCount, th.reset))
		}
	}
	return lines
}

func (s *State) renderProjectPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[1] OVERVIEW · VERIFICATION TESTS & CHECKS", "Automated quality gates & tests")
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		empty := []string{
			"No verification checks recorded.",
		}
		return append(lines, th.renderPanelBox("", "", panelW, empty)...)
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
				th.critBg, th.reset, th.bold+th.red, strings.TrimSpace(ph.Output), th.reset))
		}
	}
	return lines
}
