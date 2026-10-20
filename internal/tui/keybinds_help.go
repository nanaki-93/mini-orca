package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type keyDesc struct {
	key  string
	desc string
}

type pageHelpSpec struct {
	title     string
	primary   []keyDesc
	secondary []keyDesc
	commands  []keyDesc
}

var allColonCommands = []keyDesc{
	{":analyze / :run", "Run or resume analysis"},
	{":reanalyze", "Fresh project analysis"},
	{":pause / :resume", "Pause or resume run"},
	{":retry / :repair", "Retry failed/stale run"},
	{":reindex", "Reindex repo & symbols"},
	{":test / :scan", "Run project checks"},
	{":analysis-config", "Set run config params"},
	{":batch <n>", "Set files per batch"},
	{":budget <s>", "Set time budget (sec)"},
	{":attempts <n>", "Set max stage attempts"},
	{":agent <prompt>", "Dispatch agent mission"},
	{":fix / :dismiss", "Fix or dismiss finding"},
	{":diff [mode]", "Open diff inspector"},
	{":hunk [dir]", "Next/prev diff hunk"},
	{":apply / :commit", "Apply & commit change"},
	{":undo", "Guarded undo last edit"},
	{":merge / :cancel", "Merge or cancel agent"},
	{":debug / :logs", "Agent & check logs"},
	{":bugs/:sec/:perf", "Filter issue category"},
	{":status <state>", "Filter issue status"},
	{":category <cat>", "Set finding category"},
	{":open <file>", "Open file in inspector"},
	{":scope / :files", "File scope inclusion"},
	{":features", "Feature suggestions"},
	{":git/:proposals", "Git or proposals view"},
	{":docs / :arch", "Handbook & arch views"},
	{":project [path]", "Project view/open repo"},
	{":models", "Model routing & presets"},
	{":model <s> <id>", "Assign model to scope"},
	{":preset <id>", "Apply/save/del preset"},
	{":trust/:bootstrap", "Trust or bootstrap"},
	{":palette/:theme", "Cycle TUI color theme"},
	{":keys / :help", "Open keys & cmd help"},
	{":reconnect / :q", "Reconnect or quit TUI"},
}

func renderWrappedKeyPillRows(th ansiTheme, labelStyle, label string, items []keyDesc, availW int) []string {
	var lines []string
	curLine := ""
	curW := 0
	for _, it := range items {
		pill := th.keyPill(it.key, it.desc)
		pillW := theme.VisibleRuneCount(pill)
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

func renderCommandCatalogRows(th ansiTheme, items []keyDesc, contentW int) []string {
	if contentW >= 82 {
		colW := (contentW - 2) / 2
		var rows []string
		for i := 0; i < len(items); i += 2 {
			left := formatCommandCatalogCell(th, items[i], colW)
			if i+1 >= len(items) {
				rows = append(rows, left)
				continue
			}
			right := formatCommandCatalogCell(th, items[i+1], colW)
			rows = append(rows, left+"  "+right)
		}
		return rows
	}
	var rows []string
	for _, it := range items {
		rows = append(rows, formatCommandCatalogCell(th, it, contentW))
	}
	return rows
}

func formatCommandCatalogCell(th ansiTheme, it keyDesc, colW int) string {
	cell := fmt.Sprintf("%s%-16s%s %s%s%s", th.cyan, it.key, th.reset, th.fg, it.desc, th.reset)
	return theme.PadVisibleRight(cell, colW)
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
	globalNav := []keyDesc{{"1-5", "Section"}, {"Tab", "Sub-page"}, {"←/→", "Row Option"}, {"j/k", "Select Row"}}
	globalKeys := []keyDesc{{"?", "Close Help"}, {":", "Command"}, {"p", "Cycle Palette"}, {"q", "Quit"}}

	body := []string{
		fmt.Sprintf("%sPAGE SHORTCUTS · %s%s", th.bold+th.accent, spec.title, th.reset),
	}
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.green, "PAGE KEYS", spec.primary, availW)...)
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.amber, "WORKFLOW", spec.secondary, availW)...)
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.purple, "PAGE COMMANDS", spec.commands, availW)...)
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.cyan, "GLOBAL NAV", globalNav, availW)...)
	body = append(body, renderWrappedKeyPillRows(th, th.bold+th.cyan, "GLOBAL KEYS", globalKeys, availW)...)
	body = append(body, fmt.Sprintf("%sGENERIC CMDS · ALL COLON COMMANDS%s", th.bold+th.purple, th.reset))
	body = append(body, renderCommandCatalogRows(th, allColonCommands, contentW)...)

	rightMeta := "[?] / Esc"
	if s.viewportBodyRows > 0 {
		maxInner := max(6, s.viewportBodyRows-4)
		if len(body) > maxInner {
			maxScroll := len(body) - maxInner
			scroll := min(max(0, s.KeybindsScroll), maxScroll)
			body = body[scroll : scroll+maxInner]
			rightMeta = fmt.Sprintf("[j/k] %d-%d/%d · [?] / Esc", scroll+1, scroll+maxInner, len(body)+maxScroll)
		}
	}
	return th.renderPanelBox("KEYBINDINGS & COLON COMMANDS REFERENCE", rightMeta, boxW, body)
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
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":test", "Run checks"}, {":bugs", "Issues view"}},
	},
	PageRepo: {
		title:     "[1] OVERVIEW · REPOSITORY & INDEX",
		primary:   []keyDesc{{"Enter/r", "Reindex Repo"}, {"t", "Run Tests/Scan"}, {"j/k", "Scroll"}},
		secondary: []keyDesc{{"h/l", "Overview Sub-page"}, {"Tab", "Next Sub-page"}, {"1-5", "Switch Hub"}},
		commands:  []keyDesc{{":reindex", "Reindex repo"}, {":project <path>", "Switch repo"}, {":scope", "File scope"}},
	},
	PageProject: {
		title:     "[1] OVERVIEW · PROJECT & VERIFICATION",
		primary:   []keyDesc{{"Enter/t", "Run Scan & Tests"}, {"r", "Run/Resume Analysis"}, {"j/k", "Select Check"}},
		secondary: []keyDesc{{"h/l", "Overview Sub-page"}, {"n", "Reanalyze"}, {"Tab", "Next Sub-page"}},
		commands:  []keyDesc{{":test", "Run checks"}, {":scan", "Verify repo"}, {":analyze", "Run analysis"}},
	},
	PageAgents: {
		title:     "[2] AGENT STUDIO · PIPELINE & WORKTREES",
		primary:   []keyDesc{{"Enter/d", "Inspect Diff"}, {"g", "Debug/Logs"}, {"r", "Repair"}, {"x", "Cancel"}},
		secondary: []keyDesc{{"a/c", "Apply & Commit"}, {"u", "Guarded Undo"}, {"m", "Merge"}, {"b", "Issues"}},
		commands:  []keyDesc{{":agent <msg>", "New mission"}, {":debug", "View logs"}, {":apply", "Apply diff"}},
	},
	PageBugs: {
		title:     "[2] AGENT STUDIO · ISSUES & FINDINGS",
		primary:   []keyDesc{{"Enter/a", "Fix Selected Bug"}, {"i", "Inspect Issue"}, {"d", "Dismiss"}},
		secondary: []keyDesc{{"s/v", "Status Filter"}, {"f/t", "Category Filter"}, {"x/Space", "Batch"}},
		commands:  []keyDesc{{":fix", "Fix issue"}, {":dismiss", "Dismiss issue"}, {":status <s>", "Filter status"}},
	},
	PageIssueDetail: {
		title:     "[2] AGENT STUDIO · ISSUE INSPECTOR",
		primary:   []keyDesc{{"Enter/a", "Dispatch Fix"}, {"w", "Watch Agent Pipeline"}, {"d", "Dismiss"}},
		secondary: []keyDesc{{"b/i/Esc", "Back to Issues"}, {"s/v", "Status Filter"}, {"f/t", "Category"}},
		commands:  []keyDesc{{":fix", "Fix issue"}, {":dismiss", "Dismiss issue"}, {":bugs", "Back to list"}},
	},
	PageFeatures: {
		title:     "[2] AGENT STUDIO · FEATURE SUGGESTIONS",
		primary:   []keyDesc{{"Enter/a", "Build Feature"}, {"d", "Dismiss Idea"}, {"j/k", "Select Idea"}},
		secondary: []keyDesc{{"r", "Run/Resume Analysis"}, {"n", "Reanalyze"}, {"w", "Agent Pipeline"}},
		commands:  []keyDesc{{":features", "Features view"}, {":agent <msg>", "Custom feature"}, {":analyze", "Run analysis"}},
	},
	PageAnalysisRun: {
		title:     "[2] AGENT STUDIO · LAST ANALYSIS RUN & CONFIG",
		primary:   []keyDesc{{"r", "Run/Resume Analysis"}, {"b/t/a", "Batch/Budget/Tries"}, {"f/c", "Features/Cache"}},
		secondary: []keyDesc{{"Enter/←/→", "Step Selected Row"}, {"x", "Reset"}, {"P", "Pause"}, {"R", "Retry"}},
		commands:  []keyDesc{{":analysis-config", "Set config"}, {":batch <n>", "Batch"}, {":budget <s>", "Budget"}},
	},
	PageAnalysisFiles: {
		title:     "[2] AGENT STUDIO · FILE SCOPE",
		primary:   []keyDesc{{"Enter/x/Space", "Toggle File Scope"}, {"j/k", "Select File"}},
		secondary: []keyDesc{{"r", "Run Scoped Analysis"}, {"n", "Reanalyze"}, {"h/l", "Studio View"}},
		commands:  []keyDesc{{":scope", "File scope"}, {":open <file>", "Open file"}, {":analyze", "Run analysis"}},
	},
}

var hubThreeToFiveHelpSpecs = map[Page]pageHelpSpec{
	PageEditor: {
		title:     "[3] CODE & DIFF · INSPECTOR",
		primary:   []keyDesc{{"a/c/Enter", "Apply"}, {"n/N", "Next/Prev Hunk"}, {"f/F", "Next/Prev File"}, {"v", "View"}},
		secondary: []keyDesc{{"x", "Reject Hunk"}, {"e", "Toggle Why"}, {"o", "Browse Files"}, {"u", "Guarded Undo"}},
		commands:  []keyDesc{{":diff [mode]", "Switch view"}, {":hunk [dir]", "Step hunk"}, {":apply", "Apply diff"}},
	},
	PageGit: {
		title:     "[3] CODE & DIFF · GIT & WORKTREES",
		primary:   []keyDesc{{"m", "Merge Worktree"}, {"a/c", "Apply & Commit"}, {"u", "Guarded Undo"}},
		secondary: []keyDesc{{"j/k", "Select Worktree"}, {"h/l", "Code & Diff View"}, {"Tab", "Next View"}},
		commands:  []keyDesc{{":git", "Worktrees view"}, {":merge", "Merge branch"}, {":undo", "Guarded undo"}},
	},
	PageHistory: {
		title:     "[3] CODE & DIFF · SAVED PROPOSALS",
		primary:   []keyDesc{{"Enter", "Load Proposal Diff"}, {"u", "Guarded Undo"}, {"j/k", "Select Proposal"}},
		secondary: []keyDesc{{"a/c", "Apply & Commit"}, {"h/l", "Code & Diff View"}, {"Tab", "Next View"}},
		commands:  []keyDesc{{":proposals", "Proposals list"}, {":apply", "Apply diff"}, {":undo", "Guarded undo"}},
	},
	PageDocs: {
		title:     "[4] HANDBOOK & ARCH · CONTEXT & DEPENDENCIES",
		primary:   []keyDesc{{"c", "Context & Deps"}, {"m", "Components"}, {"s", "Scenarios & ADRs"}},
		secondary: []keyDesc{{"d", "Architecture Diagram"}, {"h/l/Tab", "Switch Handbook View"}, {"r", "Refresh"}},
		commands:  []keyDesc{{":docs", "Handbook view"}, {":arch", "Architecture"}, {":analyze", "Run analysis"}},
	},
	PageDocsComponents: {
		title:     "[4] HANDBOOK & ARCH · ARCHITECTURAL COMPONENTS",
		primary:   []keyDesc{{"m", "Components"}, {"c", "Context & Deps"}, {"s", "Scenarios & ADRs"}},
		secondary: []keyDesc{{"d", "Architecture Diagram"}, {"h/l/Tab", "Switch Handbook View"}, {"r", "Refresh"}},
		commands:  []keyDesc{{":docs", "Handbook view"}, {":arch", "Architecture"}, {":analyze", "Run analysis"}},
	},
	PageDocsDecisions: {
		title:     "[4] HANDBOOK & ARCH · SCENARIOS & DECISIONS",
		primary:   []keyDesc{{"s", "Scenarios & ADRs"}, {"c", "Context & Deps"}, {"m", "Components"}},
		secondary: []keyDesc{{"d", "Architecture Diagram"}, {"h/l/Tab", "Switch Handbook View"}, {"r", "Refresh"}},
		commands:  []keyDesc{{":docs", "Handbook view"}, {":arch", "Architecture"}, {":analyze", "Run analysis"}},
	},
	PageDiagrams: {
		title:     "[4] HANDBOOK & ARCH · ARCHITECTURE DIAGRAM",
		primary:   []keyDesc{{"d", "Architecture Diagram"}, {"c", "Context"}, {"m", "Components"}},
		secondary: []keyDesc{{"s", "Scenarios & ADRs"}, {"h/l/Tab", "Switch Handbook View"}, {"r", "Refresh"}},
		commands:  []keyDesc{{":arch", "Architecture"}, {":docs", "Handbook view"}, {":analyze", "Run analysis"}},
	},
	PageInstructions: {
		title:     "[5] GOVERNANCE & RULES · TRUST POLICY",
		primary:   []keyDesc{{"Enter/t", "Toggle Execution Trust"}, {"b", "Bootstrap Pillars"}},
		secondary: []keyDesc{{"m", "Model Routing"}, {"g", "Graduate Lesson"}, {"h/l/Tab", "Governance View"}},
		commands:  []keyDesc{{":trust", "Toggle trust"}, {":models", "Model routing"}, {":bootstrap", "Bootstrap rules"}},
	},
	PageModels: {
		title:     "[5] GOVERNANCE & RULES · MODEL ROUTING",
		primary:   []keyDesc{{"Enter/m/M", "Cycle Scope Model"}, {"s/n", "Save New Preset"}, {"v", "Cycle Preset"}},
		secondary: []keyDesc{{"d", "Delete Custom Preset"}, {"x", "Reset Scope"}, {"r", "Refresh Models"}},
		commands:  []keyDesc{{":model <s> <id>", "Set model"}, {":preset <id>", "Preset"}, {":preset-save", "Save"}},
	},
	PageBootstrap: {
		title:     "[5] GOVERNANCE & RULES · BOOTSTRAP CHECKLIST",
		primary:   []keyDesc{{"Enter/b", "Bootstrap Missing Pillar"}, {"j/k", "Select Pillar"}},
		secondary: []keyDesc{{"t", "Toggle Trust"}, {"m", "Model Routing"}, {"h/l/Tab", "Governance View"}},
		commands:  []keyDesc{{":bootstrap", "Bootstrap rules"}, {":trust", "Toggle trust"}, {":models", "Model routing"}},
	},
	PageLessons: {
		title:     "[5] GOVERNANCE & RULES · HARNESS² LESSONS",
		primary:   []keyDesc{{"Enter/g", "Graduate Lesson"}, {"d", "Inspect Diff"}, {"a/c", "Apply & Commit"}},
		secondary: []keyDesc{{"u", "Guarded Undo"}, {"b", "Bootstrap Pillars"}, {"t", "Toggle Trust"}},
		commands:  []keyDesc{{":apply", "Apply AGENTS.md"}, {":diff", "Inspect diff"}, {":undo", "Guarded undo"}},
	},
}
