package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) renderActivePage(th ansiTheme, width int) []string {
	switch s.Page {
	case PageSummary:
		return s.renderSummaryPage(th, width)
	case PageRepo:
		return s.renderRepoPage(th, width)
	case PageProject:
		return s.renderProjectPage(th, width)
	case PagePlan:
		return s.renderPlanPage(th, width)
	case PageAgents:
		return s.renderAgentsPage(th, width)
	case PageBugs:
		return s.renderBugsPage(th, width)
	case PageIssueDetail:
		return s.renderIssueDetailPage(th, width)
	case PageFeatures:
		return s.renderFeaturesPage(th, width)
	case PageFeatureDetail:
		return s.renderFeatureDetailPage(th, width)
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
	case PageWorktreeDetail:
		return s.renderWorktreeDetail(th, width)
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
			fmt.Sprintf("  %s%s%s", th.bold+th.fg, title, th.reset),
			"",
		}
	}
	return []string{
		fmt.Sprintf("  %s%s%s", th.bold+th.fg, title, th.reset),
		fmt.Sprintf("  %s%s%s", th.muted, subtitle, th.reset),
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

func (s *State) renderRepoPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "REPOSITORY", "Project identity and indexed files")
	if s.Project == nil {
		return append(lines, "", "  No project loaded. [o] Open project")
	}
	branch := "detached"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = s.Repository.Branch
	}
	meta := []string{
		fmt.Sprintf("%s%-14s%s %s%s%s", th.muted, "Repository:", th.reset, th.bold+th.fg, s.Project.Name, th.reset),
		fmt.Sprintf("%s%-14s%s %s%s%s", th.muted, "Path:", th.reset, th.cyan, s.Project.Path, th.reset),
		fmt.Sprintf("%s%-14s%s %s%s%s · Manifest: %s · Branch: %s%s%s",
			th.muted, "Stack:", th.reset, th.bold+th.fg, client.FirstNonEmpty(s.Project.Type, "polyglot"), th.reset,
			client.FirstNonEmpty(s.Project.BuildFile, "auto"), th.purple, branch, th.reset),
		fmt.Sprintf("%s%-14s%s %s%d%s indexed files · %s%d%s lines",
			th.muted, "Footprint:", th.reset, th.bold+th.green, s.Project.FileCount, th.reset, th.bold+th.fg, s.Project.TotalLines, th.reset),
	}
	lines = append(lines, th.renderPanelBox("", "", panelW, meta)...)
	if s.Index != nil && len(s.Index.Files) > 0 {
		lines = append(lines, "")
		for _, f := range s.Index.Files {
			lines = append(lines, fmt.Sprintf("  %s•%s %-42s %s%s · %d lines%s",
				th.accent, th.reset, f.Path, th.muted, client.FirstNonEmpty(f.Language, "text"), f.LineCount, th.reset))
		}
	}
	return lines
}

func (s *State) renderProjectPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "CHECKS", "Verification results and diagnostic output")
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return append(lines, "", "  No verification checks recorded.", "  [t] Run checks · :trust to manage execution permission")
	}
	lines = append(lines, "", "  "+th.statusBadge(s.Scan.Status), "")
	selected := s.ClampedIdx()
	start, end := windowBounds(len(s.Scan.Phases), selected, s.adaptiveWindowRows(12, 6))
	for idx := start; idx < end; idx++ {
		ph := s.Scan.Phases[idx]
		name := theme.TruncatePlain(ph.Name, max(12, width-26))
		lines = append(lines, "  "+th.renderCursorPrefix(idx == selected)+rowTitleStyle(th, idx == selected)+theme.PadVisibleRight(name, max(12, width-26))+th.reset+"  "+th.statusBadge(ph.State))
	}
	lines = appendWindowFooter(th, lines, start, end, len(s.Scan.Phases))
	return append(append(lines, ""), s.renderCheckDetails(th, width)...)
}
