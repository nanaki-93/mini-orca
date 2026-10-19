package tui

import (
	"fmt"
	"strings"
)

const sidebarMenuWidth = 26

// Render generates a complete, full-screen adaptive ANSI terminal frame for the current TUI state.
func (s *State) Render(width, height int) string {
	frameWidth := clampWidth(width)
	frameHeight := clampHeight(height)
	th := themeForPalette(s.Palette)

	if frameWidth < 96 {
		return s.renderCompactFrame(th, frameWidth, frameHeight)
	}
	return s.renderSplitFrame(th, frameWidth, frameHeight)
}

func (s *State) renderSplitFrame(th ansiTheme, frameWidth, frameHeight int) string {
	contentWidth := max(44, frameWidth-2-sidebarMenuWidth-1)

	header := []string{
		s.renderTopBorder(th, frameWidth, frameHeight),
		th.wrapFrameLine(s.renderStatusBar(th, frameWidth), frameWidth),
	}
	if prog := s.renderProgressStrip(th, frameWidth); prog != "" {
		header = append(header, th.wrapFrameLine(prog, frameWidth))
	}
	header = append(header, s.renderSplitRule(th, frameWidth, sidebarMenuWidth, "┣", "┳", "┫", "━"))

	sidebar := s.renderSidebarMenu(th, sidebarMenuWidth)
	rightBody := s.renderWorkspaceContent(th, contentWidth)

	footer := []string{
		s.renderSplitRule(th, frameWidth, sidebarMenuWidth, "┣", "┻", "┫", "━"),
		th.wrapFrameLine(s.renderCommandBar(th, frameWidth), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}

	targetBodyRows := max(1, frameHeight-len(header)-len(footer))
	body := make([]string, 0, targetBodyRows)
	for i := 0; i < targetBodyRows; i++ {
		var leftLine, rightLine string
		if i < len(sidebar) {
			leftLine = sidebar[i]
		}
		if i < len(rightBody) {
			rightLine = rightBody[i]
		}
		body = append(body, padVisibleRight(leftLine, sidebarMenuWidth)+
			th.border+"│"+th.reset+
			padVisibleRight(rightLine, contentWidth))
	}
	return assembleFrame(th, frameWidth, frameHeight, header, body, footer)
}

func (s *State) renderCompactFrame(th ansiTheme, frameWidth, frameHeight int) string {
	header := []string{
		s.renderTopBorder(th, frameWidth, frameHeight),
		th.wrapFrameLine(s.renderStatusBar(th, frameWidth), frameWidth),
		s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"),
		th.wrapFrameLine(s.renderHubBar(th, frameWidth), frameWidth),
	}
	if sub := s.renderSubViewBar(th, frameWidth); sub != "" {
		header = append(header, th.wrapFrameLine(sub, frameWidth))
	}
	if prog := s.renderProgressStrip(th, frameWidth); prog != "" {
		header = append(header, th.wrapFrameLine(prog, frameWidth))
	}
	header = append(header, s.renderHorizontalRule(th, frameWidth, "┠", "┨", "─"))

	body := s.renderWorkspaceContent(th, frameWidth-2)

	footer := []string{
		s.renderHorizontalRule(th, frameWidth, "┣", "┫", "━"),
		th.wrapFrameLine(s.renderCommandBar(th, frameWidth), frameWidth),
		s.renderBottomBorder(th, frameWidth),
	}
	return assembleFrame(th, frameWidth, frameHeight, header, body, footer)
}

func assembleFrame(th ansiTheme, frameWidth, frameHeight int, header, body, footer []string) string {
	targetBodyRows := max(1, frameHeight-len(header)-len(footer))
	lines := make([]string, 0, len(header)+targetBodyRows+len(footer))
	lines = append(lines, header...)
	for i := 0; i < targetBodyRows; i++ {
		var row string
		if i < len(body) {
			row = body[i]
		}
		lines = append(lines, th.wrapFrameLine(row, frameWidth))
	}
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func (s *State) renderWorkspaceContent(th ansiTheme, contentWidth int) []string {
	var body []string
	body = append(body, "")
	if s.Error != "" {
		body = append(body, th.renderErrorBanner(s.Error, contentWidth-4)...)
		body = append(body, "")
	}
	switch {
	case s.Confirmation != nil:
		body = append(body, s.renderConfirmationModal(th, contentWidth)...)
	case s.ShowKeybindsModal:
		body = append(body, s.renderKeybindsModal(th, contentWidth)...)
	default:
		body = append(body, s.renderActivePage(th, contentWidth)...)
	}
	return body
}

func (s *State) renderSidebarMenu(th ansiTheme, width int) []string {
	active := s.ActiveHub()
	lines := []string{
		"",
		fmt.Sprintf("  %sNAVIGATION%s", th.bold+th.cyan, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
	}
	for _, tab := range HubTabs() {
		isActiveHub := tab.Key == active.Key
		if isActiveHub {
			badge := fmt.Sprintf(" ▸ %s. %-18s", tab.Key, strings.ToUpper(tab.Label))
			lines = append(lines, " "+th.accentBg+truncatePlain(badge, width-2)+th.reset)
			lines = append(lines, s.renderSidebarSubPages(th, tab, width)...)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("   %s%s.%s %s%s%s",
				th.muted, tab.Key, th.reset, th.fg, truncatePlain(tab.Label, width-7), th.reset))
		}
	}
	lines = append(lines, s.renderSidebarPulse(th, width)...)
	return lines
}

func (s *State) renderSidebarSubPages(th ansiTheme, tab HubTab, width int) []string {
	var visible []HubSubPage
	for _, sp := range tab.SubPages {
		if sp.Page != PageIssueDetail {
			visible = append(visible, sp)
		}
	}
	curPage := s.EffectivePage()
	if curPage == PageIssueDetail {
		curPage = PageBugs
	}
	var out []string
	for idx, sp := range visible {
		branch := "├─"
		if idx == len(visible)-1 {
			branch = "└─"
		}
		label := truncatePlain(sp.Label, width-9)
		if sp.Page == curPage {
			out = append(out, fmt.Sprintf("  %s%s%s %s◆ %s%s", th.border, branch, th.reset, th.bold+th.accent, label, th.reset))
		} else {
			out = append(out, fmt.Sprintf("  %s%s%s %s◇ %s%s", th.border, branch, th.reset, th.muted, label, th.reset))
		}
	}
	return out
}

func (s *State) renderSidebarPulse(th ansiTheme, width int) []string {
	summary := ComputeIssueSummary(s.CollectAllIssues(), s.Features)
	openVal := fmt.Sprintf("%s0%s", th.green, th.reset)
	if summary.TotalOpen > 0 {
		openVal = fmt.Sprintf("%s%d open%s", th.bold+th.amber, summary.TotalOpen, th.reset)
	}
	agentState := "IDLE"
	if s.Change != nil && s.Change.State != "" {
		agentState = s.Change.State
	}
	return []string{
		fmt.Sprintf("  %sWORKSPACE PULSE%s", th.bold+th.cyan, th.reset),
		fmt.Sprintf("  %s%s%s", th.border, strings.Repeat("─", max(8, width-4)), th.reset),
		fmt.Sprintf("  %sFindings%s   %s", th.muted, th.reset, openVal),
		fmt.Sprintf("  %sResolved%s   %s%d%s", th.muted, th.reset, th.green, summary.Done, th.reset),
		fmt.Sprintf("  %sAgent%s      %s", th.muted, th.reset, th.statusBadge(agentState)),
		fmt.Sprintf("  %sFiles%s      %s%d%s", th.muted, th.reset, th.fg, s.indexedFileCount(), th.reset),
	}
}

func (s *State) renderTopBorder(th ansiTheme, width, height int) string {
	tag := fmt.Sprintf(" ◆ MINI-ORCA // TUI · %s · %dx%d ", th.name, width, height)
	fill := max(4, width-visibleRuneCount(tag)-2)
	return fmt.Sprintf("%s┏%s%s%s%s┓%s", th.border, th.accent, tag, th.border, strings.Repeat("━", fill), th.reset)
}

func (s *State) renderBottomBorder(th ansiTheme, width int) string {
	return fmt.Sprintf("%s┗%s┛%s", th.border, strings.Repeat("━", max(10, width-2)), th.reset)
}

func (s *State) renderHorizontalRule(th ansiTheme, width int, left, right, ch string) string {
	return fmt.Sprintf("%s%s%s%s%s", th.border, left, strings.Repeat(ch, max(10, width-2)), right, th.reset)
}

func (s *State) renderSplitRule(th ansiTheme, width, leftPaneWidth int, left, mid, right, ch string) string {
	inner := max(10, width-2)
	rightPaneWidth := max(4, inner-leftPaneWidth-1)
	return fmt.Sprintf("%s%s%s%s%s%s%s",
		th.border,
		left,
		strings.Repeat(ch, leftPaneWidth),
		mid,
		strings.Repeat(ch, rightPaneWidth),
		right,
		th.reset,
	)
}

func (s *State) renderStatusBar(th ansiTheme, width int) string {
	connBadge := th.okBg + " ● ONLINE " + th.reset
	if !s.Connected {
		connBadge = th.warnBg + " ○ OFFLINE " + th.reset
	}
	projName := "no-project"
	if s.Project != nil && s.Project.Name != "" {
		projName = truncatePlain(s.Project.Name, 26)
	}
	branch := "main"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = truncatePlain(s.Repository.Branch, 20)
	}
	dirty := ""
	if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
		dirty = " " + th.statusBadge("DIRTY")
	}
	trust := th.statusBadge("TRUSTED")
	if !s.WorkspaceTrust {
		trust = th.statusBadge("UNTRUSTED")
	}
	ver := ""
	if s.Version != "" {
		ver = th.muted + "v" + s.Version + " " + th.reset
	}
	content := fmt.Sprintf(
		"  %sMINI-ORCA // TUI%s %s %s  %s%s%s   %sgit:%s %s%s%s   %strust:%s %s   %spalette:%s %s%s%s",
		th.bold+th.accent, th.reset, ver, connBadge,
		th.bold+th.fg, projName, th.reset,
		th.muted, th.reset, th.cyan, branch, dirty,
		th.muted, th.reset, trust,
		th.muted, th.reset, th.purple, s.Palette, th.reset,
	)
	return padVisibleRight(content, width-2)
}

func (s *State) renderHubBar(th ansiTheme, width int) string {
	active := s.ActiveHub()
	var parts []string
	for _, tab := range HubTabs() {
		if tab.Key == active.Key {
			parts = append(parts, th.activeTabStyle.Render("▸ "+tab.Label))
		} else {
			parts = append(parts, th.idleTabStyle.Render(tab.Label))
		}
	}
	row := fmt.Sprintf("  %s %s", th.labelStyle.Render("MENU:"), strings.Join(parts, th.border+"│"+th.reset))
	return padVisibleRight(row, width-2)
}

func (s *State) renderSubViewBar(th ansiTheme, width int) string {
	hub := s.ActiveHub()
	if len(hub.SubPages) == 0 {
		return ""
	}
	curPage := s.EffectivePage()
	var parts []string
	for _, sp := range hub.SubPages {
		if sp.Page == PageIssueDetail && curPage != PageIssueDetail {
			continue
		}
		if sp.Page == curPage {
			parts = append(parts, th.subActiveStyle.Render("◆ "+sp.Label))
		} else {
			parts = append(parts, th.subIdleStyle.Render("◇ "+sp.Label))
		}
	}
	row := fmt.Sprintf("  %s %s", th.labelStyle.Render("VIEW:"), strings.Join(parts, th.border+"·"+th.reset))
	return padVisibleRight(row, width-2)
}

func (s *State) renderProgressStrip(th ansiTheme, width int) string {
	var parts []string
	if s.Run != nil && s.Run.Status != "" {
		done, total := runAggregateCoverage(s.Run)
		bar := th.renderProgressBar(done, total, 12)
		parts = append(parts, fmt.Sprintf("Analysis %s %s (%ds)", th.statusBadge(s.Run.Status), bar, s.Run.ElapsedSeconds))
	}
	if s.Change != nil {
		added, removed, badge := ClassifyDiffScope(s.Change.Changes)
		parts = append(parts, fmt.Sprintf("Active Proposal: %s%s%s %s (%s+%d%s/%s-%d%s)",
			th.bold, truncatePlain(s.Change.Title, 28), th.reset,
			th.statusBadge(badge), th.green, added, th.reset, th.red, removed, th.reset,
		))
	}
	if s.Busy != "" {
		parts = append(parts, th.warnBg+" ⟳ BUSY: "+s.Busy+" "+th.reset)
	}
	if len(parts) == 0 {
		return ""
	}
	row := "  " + strings.Join(parts, th.border+"  │  "+th.reset)
	return padVisibleRight(row, width-2)
}

func runAggregateCoverage(run *AnalysisRun) (int, int) {
	if run == nil {
		return 0, 0
	}
	var done, total int
	for _, sec := range run.Sections {
		done += sec.Coverage.Succeeded + sec.Coverage.Partial
		total += sec.Coverage.Total
	}
	if total == 0 && strings.EqualFold(run.Status, "completed") {
		return 1, 1
	}
	return done, total
}

func (s *State) renderConfirmationModal(th ansiTheme, width int) []string {
	c := s.Confirmation
	body := []string{
		fmt.Sprintf("%s%s%s", th.bold+th.fg, c.Title, th.reset),
		c.Message,
		"",
		fmt.Sprintf("Confirm: %s  ·  Cancel: %s",
			th.keyPill("y/Enter", firstNonEmpty(c.AcceptLabel, "Confirm")),
			th.keyPill("n/Esc", "Cancel")),
	}
	return th.renderPanelBox("CONFIRMATION REQUIRED", "SECURITY & GOVERNANCE GATE", min(82, width-6), body)
}

type keyDesc struct {
	key  string
	desc string
}

type pageHelpSpec struct {
	title     string
	primary   []keyDesc
	secondary []keyDesc
	commands  string
}

func renderWrappedKeyPillRows(th ansiTheme, labelStyle, label string, items []keyDesc, availW int) []string {
	var lines []string
	curLine := ""
	curW := 0
	for _, it := range items {
		pill := th.keyPill(it.key, it.desc)
		pillW := visibleRuneCount(pill)
		if curW > 0 && curW+2+pillW > availW {
			lines = append(lines, formatKeybindRow(th, labelStyle, label, len(lines) == 0, curLine))
			curLine = pill
			curW = pillW
			continue
		}
		if curW > 0 {
			curLine += "  " + pill
			curW += 2 + pillW
		} else {
			curLine = pill
			curW = pillW
		}
	}
	if curLine != "" || len(lines) == 0 {
		lines = append(lines, formatKeybindRow(th, labelStyle, label, len(lines) == 0, curLine))
	}
	return lines
}

func renderWrappedCommandRows(th ansiTheme, labelStyle, label, raw string, availW int) []string {
	sep := "  "
	if !strings.Contains(raw, "  ") {
		sep = " "
	}
	var tokens []string
	for _, part := range strings.Split(raw, sep) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			tokens = append(tokens, trimmed)
		}
	}
	var lines []string
	curLine := ""
	curW := 0
	for _, tok := range tokens {
		tokW := visibleRuneCount(tok)
		if curW > 0 && curW+2+tokW > availW {
			lines = append(lines, formatKeybindRow(th, labelStyle, label, len(lines) == 0, th.fg+curLine+th.reset))
			curLine = tok
			curW = tokW
			continue
		}
		if curW > 0 {
			curLine += "  " + tok
			curW += 2 + tokW
		} else {
			curLine = tok
			curW = tokW
		}
	}
	if curLine != "" || len(lines) == 0 {
		lines = append(lines, formatKeybindRow(th, labelStyle, label, len(lines) == 0, th.fg+curLine+th.reset))
	}
	return lines
}

func formatKeybindRow(th ansiTheme, labelStyle, label string, first bool, content string) string {
	if first {
		return fmt.Sprintf("%s%-16s%s%s", labelStyle, label, th.reset, content)
	}
	return strings.Repeat(" ", 16) + content
}

func (s *State) renderKeybindsModal(th ansiTheme, width int) []string {
	spec := lookupPageHelpSpec(s.EffectivePage())
	boxW := max(44, min(108, width-4))
	contentW := max(24, boxW-4)
	availW := max(18, contentW-16)
	ruleLen := max(16, boxW-6)
	globalNav := []keyDesc{{"1-5", "Section"}, {"Tab", "Sub-page"}, {"h/l", "View"}, {"j/k", "Select Row"}}
	globalKeys := []keyDesc{{"?", "Close Help"}, {":", "Command"}, {"p", "Cycle Palette"}, {"q", "Quit"}}
	genericCmds := ":help  :keys  :analyze  :test  :fix  :agent  :open  :project  :models  :apply  :undo  :palette  :q"

	body := []string{
		fmt.Sprintf("%sPAGE SHORTCUTS · %s%s", th.bold+th.accent, spec.title, th.reset),
	}
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.green, "PAGE KEYS", spec.primary, availW)...)
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.amber, "WORKFLOW", spec.secondary, availW)...)
	body = append(body, renderWrappedCommandRows(th, th.bold+th.purple, "PAGE COMMANDS", spec.commands, availW)...)
	body = append(body, fmt.Sprintf("%s%s%s", th.border, strings.Repeat("─", ruleLen), th.reset))
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.cyan, "GLOBAL NAV", globalNav, availW)...)
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.cyan, "GLOBAL KEYS", globalKeys, availW)...)
	body = append(body, renderWrappedCommandRows(th, th.bold+th.purple, "GENERIC CMDS", genericCmds, availW)...)
	return th.renderPanelBox("KEYBINDINGS & COLON COMMANDS REFERENCE", "[?] / Esc", boxW, body)
}

func lookupPageHelpSpec(page Page) pageHelpSpec {
	if spec, ok := hubOneTwoHelpSpecs[page]; ok {
		return spec
	}
	if spec, ok := hubThreeToFiveHelpSpecs[page]; ok {
		return spec
	}
	return hubOneTwoHelpSpecs[PageSummary]
}

var hubOneTwoHelpSpecs = map[Page]pageHelpSpec{
	PageSummary: {
		title:     "[1] OVERVIEW · EXECUTIVE DASHBOARD",
		primary:   []keyDesc{{"Enter/a", "Fix Top Issue"}, {"b", "Issues List"}, {"f", "Features"}},
		secondary: []keyDesc{{"r", "Run/Resume Analysis"}, {"n", "Reanalyze"}, {"t", "Run Tests/Scan"}},
		commands:  ":analyze  :test  :bugs  :features  :project <path>",
	},
	PageRepo: {
		title:     "[1] OVERVIEW · REPOSITORY & INDEX",
		primary:   []keyDesc{{"Enter/r", "Reindex Repo"}, {"t", "Run Tests/Scan"}, {"j/k", "Scroll"}},
		secondary: []keyDesc{{"h/l", "Overview Sub-page"}, {"Tab", "Next Sub-page"}, {"1-5", "Switch Hub"}},
		commands:  ":reindex  :test  :project <path>  :scope",
	},
	PageProject: {
		title:     "[1] OVERVIEW · PROJECT & VERIFICATION",
		primary:   []keyDesc{{"Enter/t", "Run Scan & Tests"}, {"r", "Run Analysis"}, {"j/k", "Select Check"}},
		secondary: []keyDesc{{"h/l", "Overview Sub-page"}, {"n", "Reanalyze"}, {"Tab", "Next Sub-page"}},
		commands:  ":test  :scan  :project <path>  :analyze",
	},
	PageAgents: {
		title:     "[2] AGENT STUDIO · PIPELINE & WORKTREES",
		primary:   []keyDesc{{"Enter/d/i", "Inspect Diff"}, {"r", "Refresh/Repair"}, {"x", "Cancel Workflow"}},
		secondary: []keyDesc{{"a/c", "Apply & Commit"}, {"u", "Guarded Undo"}, {"m", "Merge"}, {"b", "Issues"}},
		commands:  ":agent <prompt>  :diff  :apply [msg]  :cancel  :merge  :undo",
	},
	PageBugs: {
		title:     "[2] AGENT STUDIO · ISSUES & FINDINGS",
		primary:   []keyDesc{{"Enter/a", "Fix Selected Bug"}, {"i", "Inspect Issue"}, {"d", "Dismiss"}},
		secondary: []keyDesc{{"s/v", "Status Filter"}, {"f/t", "Category Filter"}, {"x/Space", "Batch"}},
		commands:  ":fix  :dismiss  :status <open|in_progress|done|all>  :category <cat>",
	},
	PageIssueDetail: {
		title:     "[2] AGENT STUDIO · ISSUE INSPECTOR",
		primary:   []keyDesc{{"Enter/a", "Dispatch Fix"}, {"w", "Watch Agent Pipeline"}, {"d", "Dismiss"}},
		secondary: []keyDesc{{"b/i/Esc", "Back to Issues"}, {"s/v", "Status Filter"}, {"f/t", "Category"}},
		commands:  ":fix  :dismiss  :agent <prompt>  :diff  :bugs",
	},
	PageFeatures: {
		title:     "[2] AGENT STUDIO · FEATURE SUGGESTIONS",
		primary:   []keyDesc{{"Enter/a", "Build Feature"}, {"d", "Dismiss Idea"}, {"j/k", "Select Idea"}},
		secondary: []keyDesc{{"r", "Run/Resume Analysis"}, {"n", "Reanalyze"}, {"w", "Agent Pipeline"}},
		commands:  ":features  :agent <prompt>  :analyze  :reanalyze",
	},
	PageAnalysisRun: {
		title:     "[2] AGENT STUDIO · LAST ANALYSIS RUN",
		primary:   []keyDesc{{"Enter", "Open Section Issues"}, {"r", "Run/Resume"}, {"P", "Pause Run"}},
		secondary: []keyDesc{{"R", "Retry Failed/Stale"}, {"n", "Reanalyze"}, {"j/k", "Select Section"}},
		commands:  ":analyze  :pause  :resume  :repair [path]  :reanalyze",
	},
	PageAnalysisFiles: {
		title:     "[2] AGENT STUDIO · FILE SCOPE",
		primary:   []keyDesc{{"Enter/x/Space", "Toggle File Scope"}, {"j/k", "Select File"}},
		secondary: []keyDesc{{"r", "Run Scoped Analysis"}, {"n", "Reanalyze"}, {"h/l", "Studio View"}},
		commands:  ":scope  :files  :open <file>  :analyze",
	},
}

var hubThreeToFiveHelpSpecs = map[Page]pageHelpSpec{
	PageEditor: {
		title:     "[3] CODE & DIFF · INSPECTOR",
		primary:   []keyDesc{{"a/c/Enter", "Apply & Commit"}, {"n/N", "Next/Prev Hunk"}, {"f/F", "Next/Prev File"}, {"v", "Diff View"}},
		secondary: []keyDesc{{"x", "Reject Hunk"}, {"e", "Toggle Why"}, {"o", "Browse Files"}, {"u", "Guarded Undo"}},
		commands:  ":diff [hunk|split|unified]  :hunk [next|prev]  :apply [msg]  :undo  :open <file>",
	},
	PageGit: {
		title:     "[3] CODE & DIFF · GIT & WORKTREES",
		primary:   []keyDesc{{"m", "Merge Worktree"}, {"a/c", "Apply & Commit"}, {"u", "Guarded Undo"}},
		secondary: []keyDesc{{"j/k", "Select Worktree"}, {"h/l", "Code & Diff View"}, {"Tab", "Next View"}},
		commands:  ":git  :merge  :apply [msg]  :undo  :proposals",
	},
	PageHistory: {
		title:     "[3] CODE & DIFF · SAVED PROPOSALS",
		primary:   []keyDesc{{"Enter", "Load Proposal Diff"}, {"u", "Guarded Undo"}, {"j/k", "Select Proposal"}},
		secondary: []keyDesc{{"a/c", "Apply & Commit"}, {"h/l", "Code & Diff View"}, {"Tab", "Next View"}},
		commands:  ":proposals  :history  :apply [msg]  :undo",
	},
	PageDocs: {
		title:     "[4] HANDBOOK & ARCH · CONTEXT & DEPENDENCIES",
		primary:   []keyDesc{{"1/c", "Context & Deps"}, {"2/m", "Components"}, {"3/s", "Scenarios & ADRs"}},
		secondary: []keyDesc{{"4/d", "Architecture Diagram"}, {"h/l/Tab", "Switch Handbook View"}},
		commands:  ":docs  :handbook  :arch  :analyze",
	},
	PageDocsComponents: {
		title:     "[4] HANDBOOK & ARCH · ARCHITECTURAL COMPONENTS",
		primary:   []keyDesc{{"2/m", "Components"}, {"1/c", "Context & Deps"}, {"3/s", "Scenarios & ADRs"}},
		secondary: []keyDesc{{"4/d", "Architecture Diagram"}, {"h/l/Tab", "Switch Handbook View"}},
		commands:  ":docs  :arch  :analyze",
	},
	PageDocsDecisions: {
		title:     "[4] HANDBOOK & ARCH · SCENARIOS & DECISIONS",
		primary:   []keyDesc{{"3/s", "Scenarios & ADRs"}, {"1/c", "Context & Deps"}, {"2/m", "Components"}},
		secondary: []keyDesc{{"4/d", "Architecture Diagram"}, {"h/l/Tab", "Switch Handbook View"}},
		commands:  ":docs  :arch  :analyze",
	},
	PageDiagrams: {
		title:     "[4] HANDBOOK & ARCH · ARCHITECTURE DIAGRAM",
		primary:   []keyDesc{{"4/d", "Architecture Diagram"}, {"1/c", "Context"}, {"2/m", "Components"}},
		secondary: []keyDesc{{"3/s", "Scenarios & ADRs"}, {"h/l/Tab", "Switch Handbook View"}},
		commands:  ":arch  :diagrams  :docs  :analyze",
	},
	PageInstructions: {
		title:     "[5] GOVERNANCE & RULES · TRUST POLICY",
		primary:   []keyDesc{{"Enter/t", "Toggle Execution Trust"}, {"b", "Bootstrap Pillars"}},
		secondary: []keyDesc{{"m", "Model Routing"}, {"g", "Graduate Lesson"}, {"h/l/Tab", "Governance View"}},
		commands:  ":trust  :models  :bootstrap",
	},
	PageModels: {
		title:     "[5] GOVERNANCE & RULES · MODEL ROUTING",
		primary:   []keyDesc{{"Enter/m/M", "Cycle Scope Model"}, {"v", "Cycle Preset"}, {"x", "Reset Scope"}},
		secondary: []keyDesc{{"r", "Refresh Model Catalog"}, {"j/k", "Select Scope or Preset"}},
		commands:  ":models  :model <scope> <id>  :preset <id>",
	},
	PageBootstrap: {
		title:     "[5] GOVERNANCE & RULES · BOOTSTRAP CHECKLIST",
		primary:   []keyDesc{{"Enter/b", "Bootstrap Missing Pillar"}, {"j/k", "Select Pillar"}},
		secondary: []keyDesc{{"t", "Toggle Trust"}, {"m", "Model Routing"}, {"h/l/Tab", "Governance View"}},
		commands:  ":bootstrap  :trust  :models",
	},
	PageLessons: {
		title:     "[5] GOVERNANCE & RULES · HARNESS² LESSONS",
		primary:   []keyDesc{{"Enter/g", "Graduate Lesson to AGENTS.md"}, {"j/k", "Select Lesson"}},
		secondary: []keyDesc{{"b", "Bootstrap Pillars"}, {"t", "Toggle Trust"}, {"h/l/Tab", "Governance View"}},
		commands:  ":bootstrap  :trust  :models",
	},
}

func (s *State) activeBreadcrumb() string {
	hub := s.ActiveHub()
	curPage := s.EffectivePage()
	for _, sp := range hub.SubPages {
		if sp.Page == curPage {
			return fmt.Sprintf("%s / %s", strings.ToUpper(hub.Label), sp.Label)
		}
	}
	return strings.ToUpper(hub.Label)
}

func (s *State) renderCommandBar(th ansiTheme, width int) string {
	if s.CmdActive {
		return th.renderCommandInput(s.CmdInput)
	}
	if s.Error != "" {
		return fmt.Sprintf(" %s ▲ ERROR %s  %s%s%s",
			th.critBg, th.reset, th.bold+th.red, truncatePlain(s.Error, max(20, width-16)), th.reset)
	}
	badge := th.okBg + " ● READY " + th.reset
	if s.Busy != "" {
		badge = th.warnBg + " ⟳ BUSY " + th.reset
	}
	left := fmt.Sprintf(" %s  %s%s%s", badge, th.fg, truncatePlain(s.CmdOutput, max(20, width-38)), th.reset)
	right := fmt.Sprintf("%s%s%s ", th.muted, s.activeBreadcrumb(), th.reset)
	gap := max(2, width-2-visibleRuneCount(left)-visibleRuneCount(right))
	return left + strings.Repeat(" ", gap) + right
}
