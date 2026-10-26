package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) renderActivePage(th ansiTheme, width int) []string {
	lines := s.renderActiveContent(th, width)
	if notice := s.mediaActiveNotice(); notice != "" && s.Page != PageMedia {
		lines = append([]string{"  " + th.cyan + theme.TruncatePlain(notice, max(1, width-4)) + th.reset, ""}, lines...)
	}
	return lines
}

func (s *State) renderActiveContent(th ansiTheme, width int) []string {
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
	case PageMedia:
		return s.renderMedia(th, width)
	case PageArtifacts:
		return s.renderLibrary(th, width)
	case PageDocs:
		return s.renderDocsPage(th, width)
	case PageDocsComponents:
		return s.renderDocsComponentsPage(th, width)
	case PageDocsDecisions:
		return s.renderDocsDecisionsPage(th, width)
	case PageDiagrams:
		return s.renderDiagramsPage(th, width)
	case PageInstructions, PageModels, PageRoles, PageSkills, PageConnections, PageExternalWork, PageBootstrap, PageLessons, PageMemory, PageAmbient, PageCloud:
		return s.renderGovernanceSubPage(th, width)
	default:
		return s.renderSummaryPage(th, width)
	}
}

func adaptivePanelWidth(width int) int {
	return max(42, width-6)
}

// pageHeaderInlineLimit keeps title and subtitle on one line when they fit the
// narrowest supported layout; longer subtitles get their own line.
const pageHeaderInlineLimit = 68

// renderPageHeader draws a page title in the accent color with its purpose in
// a quieter tone. The breadcrumb above already says where the page lives, so
// the header stays short and leaves the rows to content.
func renderPageHeader(th ansiTheme, title, subtitle string) []string {
	head := fmt.Sprintf("  %s%s◆ %s%s", th.bold, th.accent, title, th.reset)
	subtitle = strings.TrimSpace(subtitle)
	switch {
	case subtitle == "":
		return []string{head, ""}
	case utf8.RuneCountInString(title)+utf8.RuneCountInString(subtitle) <= pageHeaderInlineLimit:
		return []string{head + "   " + th.muted + subtitle + th.reset}
	default:
		return []string{head, fmt.Sprintf("  %s%s%s", th.muted, subtitle, th.reset)}
	}
}

// renderEmptyState explains why a view is empty and what to do next, instead
// of leaving a bare "nothing here" line above blank rows.
func renderEmptyState(th ansiTheme, title string, hints ...string) []string {
	lines := []string{"", "  " + th.bold + th.fg + "○ " + title + th.reset}
	for _, hint := range hints {
		lines = append(lines, "    "+th.muted+hint+th.reset)
	}
	return lines
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
	lines := renderPageHeader(th, "Repository", "Project identity and indexed files")
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
	lines = append(lines, th.renderPanelBox("REPOSITORY TOPOLOGY & FOOTPRINT", "INDEXED", panelW, meta)...)
	if s.Index != nil && len(s.Index.Files) > 0 {
		lines = append(lines, "")
		lines = append(lines, th.renderTableHeader("   INDEXED FILE PATH                            LANGUAGE & SIZE", width)...)
		for _, f := range s.Index.Files {
			lines = append(lines, fmt.Sprintf("  %s•%s %-42s %s%s · %d lines%s",
				th.accent, th.reset, f.Path, th.muted, client.FirstNonEmpty(f.Language, "text"), f.LineCount, th.reset))
		}
	}
	return lines
}

func (s *State) renderProjectPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "Tests & checks", "Verification results and diagnostics")
	if s.Scan == nil || len(s.Scan.Phases) == 0 {
		return append(lines, "", "  No verification checks recorded.", "  [t] Run checks · :trust to manage execution permission")
	}
	lines = append(lines, "", fmt.Sprintf("  %s◈ Verification Suite:%s %s  %s(%d phases)%s",
		th.bold+th.cyan, th.reset, th.statusBadge(s.Scan.Status), th.muted, len(s.Scan.Phases), th.reset), "")
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
