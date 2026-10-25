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
	{":analyze / :run", "Run/resume analysis"},
	{":reanalyze", "Fresh full analysis"},
	{":pause/:resume", "Pause or resume run"},
	{":retry/:repair", "Retry failed run"},
	{":reindex", "Reindex repository"},
	{":test / :scan", "Run project checks"},
	{":batch <n>", "Set files per batch"},
	{":budget <s>", "Set time budget (s)"},
	{":attempts <n>", "Set max attempts"},
	{":agent <prompt>", "New agent mission"},
	{":fix / :delete", "Fix/delete finding"},
	{":apply/:commit", "Apply & commit diff"},
	{":undo", "Guarded undo edit"},
	{":merge/:reject", "Merge/reject agent"},
	{":open <file>", "Open file path"},
	{":project <path>", "Switch repo path"},
	{":model <s> <id>", "Set scope model"},
	{":preset <id>", "Apply/save preset"},
	{":trust/:boot", "Trust or bootstrap"},
	{":palette / :q", "Cycle theme or quit"},
}

func renderCommandCatalogRows(th ansiTheme, items []keyDesc, contentW int) []string {
	if contentW >= 68 {
		gap := 4
		if contentW < 76 {
			gap = 2
		}
		colW := (contentW - gap) / 2
		var rows []string
		for i := 0; i < len(items); i += 2 {
			left := formatCommandCatalogCell(th, items[i], colW)
			if i+1 >= len(items) {
				rows = append(rows, left)
				continue
			}
			right := formatCommandCatalogCell(th, items[i+1], colW)
			rows = append(rows, left+strings.Repeat(" ", gap)+right)
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
	keyW := max(15, min(20, colW-20))
	cell := fmt.Sprintf("%s%-*s%s %s%s%s", th.cyan, keyW, it.key, th.reset, th.fg, it.desc, th.reset)
	return theme.PadVisibleRight(cell, colW)
}

func (s *State) renderKeybindsModal(th ansiTheme, width int) []string {
	spec := lookupPageHelpSpec(s.EffectivePage())
	if s.EffectivePage() == PagePlan && s.PlanView == "mission" {
		spec = s.missionHelpSpec()
	}
	boxW := max(44, width-4)
	contentW := max(24, boxW-4)
	globalItems := []keyDesc{
		{"↑/↓", "Move selection"},
		{"←/→", "Step option"},
		{"1-5 / Tab", "Switch hub / tab"},
		{"Enter / Esc", "Activate / back"},
		{"? / : / Ctrl+K", "Help / commands"},
		{"p / Ctrl+C", "Theme / quit"},
	}
	var pageItems []keyDesc
	pageItems = append(pageItems, spec.primary...)
	pageItems = append(pageItems, spec.secondary...)
	pageItems = append(pageItems, spec.commands...)

	rule := fmt.Sprintf("%s%s%s", th.border, strings.Repeat("─", contentW), th.reset)
	gapLines := []string{""}

	body := []string{
		fmt.Sprintf("%sPAGE SHORTCUTS · %s%s", th.bold+th.green, spec.title, th.reset),
		rule,
	}
	body = append(body, renderCommandCatalogRows(th, pageItems, contentW)...)
	body = append(body, gapLines...)
	body = append(body,
		fmt.Sprintf("%sGLOBAL NAV · ARROWS & GLOBAL KEYS%s", th.bold+th.amber, th.reset),
		rule,
	)
	body = append(body, renderCommandCatalogRows(th, globalItems, contentW)...)
	body = append(body, gapLines...)
	body = append(body,
		fmt.Sprintf("%sGENERIC CMDS · ACTION COLON COMMANDS%s", th.bold+th.purple, th.reset),
		rule,
	)
	body = append(body, renderCommandCatalogRows(th, allColonCommands, contentW)...)

	rightMeta := "[?] / Esc"
	if s.viewportBodyRows > 0 {
		maxInner := max(1, s.viewportBodyRows-3)
		if len(body) > maxInner {
			maxScroll := len(body) - maxInner
			scroll := min(max(0, s.KeybindsScroll), maxScroll)
			s.KeybindsScroll = scroll
			body = body[scroll : scroll+maxInner]
			rightMeta = fmt.Sprintf("[j/k] %d-%d/%d · [?] / Esc", scroll+1, scroll+maxInner, len(body)+maxScroll)
		} else {
			for len(body) < maxInner {
				body = append(body, "")
			}
		}
	}
	return th.renderPanelBox("KEYBOARD REFERENCE", rightMeta, boxW, body)
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

func (s *State) missionHelpSpec() pageHelpSpec {
	spec := pageHelpSpec{title: "[2] WORK · MISSIONS", primary: []keyDesc{{"Enter", "Open plan"}, {"n", "New draft"}, {"r", "Refresh"}}, secondary: []keyDesc{{"←/→", "Section"}, {"PgUp/PgDn", "Details"}, {"Esc", "History / back"}}, commands: []keyDesc{{":mission-scope main|task", "Select checkout"}, {":mission-child ID", "Select / add child"}, {":mission-edit", "Edit local fields"}, {":mission-budget", "Edit explicit limits"}, {":mission-draft", "Return to draft"}, {":plan", "Ordinary task plan"}}}
	if s.Mission != nil && s.Mission.Draft != nil && !s.Mission.History {
		spec.primary = s.missionDraftActions()
		if s.Mission.ExecutionOpen {
			spec.primary = s.missionExecutionKeys()
			spec.commands = []keyDesc{{":mission-execution", "Refresh execution"}, {":mission-trust", "Confirm check commands"}, {":mission-start", "Start ready allocations"}, {":mission-proposal", "Inspect selected candidate"}, {":mission-revalidate", "Fresh current-base checks"}, {":mission-integrate", "Confirm integration"}, {":mission-stop", "Stop mission"}, {":mission-cleanup", "Retry proven cleanup"}}
		}
	}
	if s.Mission != nil && s.Mission.Pending != nil {
		spec.primary = []keyDesc{{"x", "Cancel request"}, {"←/→", "Section"}, {"PgUp/PgDn", "Details"}}
	}
	return spec
}

func (s *State) missionDraftActions() []keyDesc {
	v := s.Mission
	if v.Saved != nil && v.Saved.Mission.Approved && missionDigest(v.Draft) == missionDigest(v.Saved.Mission.Plan) {
		return []keyDesc{{"Enter", "Execution"}, {"n", "New draft"}, {"Esc", "History"}}
	}
	if v.Saved != nil && v.Saved.Stale {
		return []keyDesc{{"n", "New draft"}, {"Esc", "History"}}
	}
	actions := []keyDesc{{"g", "Decompose"}, {"s", "Save"}, {"e", "Edit"}}
	if v.Saved == nil || missionDigest(v.Draft) != missionDigest(v.Saved.Mission.Plan) || v.Saved.Mission.Approved {
		return actions
	}
	if len(v.Saved.Mission.Reviewed) == len(v.Draft.Children) {
		return append(actions, keyDesc{"a", "Approve plan"})
	}
	return append(actions, keyDesc{"r", "Review child"})
}

var hubOneTwoHelpSpecs = map[Page]pageHelpSpec{
	PageSummary: {
		title:     "[1] PROJECT · DASHBOARD",
		primary:   []keyDesc{{"Enter", "Open selected area"}, {"n", "New task"}, {"o", "Switch project"}},
		secondary: []keyDesc{{"r / t", "Analysis / checks"}},
		commands:  []keyDesc{{":reanalyze", "Fresh analysis"}, {":test", "Run checks"}},
	},
	PageRepo: {
		title:     "[1] OVERVIEW · REPOSITORY & INDEX",
		primary:   []keyDesc{{"Enter / r", "Reindex repo"}, {"t", "Run checks/tests"}},
		secondary: []keyDesc{{"n", "Fresh reanalyze"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":reindex", "Reindex repo"}, {":project <path>", "Switch repo"}},
	},
	PageProject: {
		title:     "[1] OVERVIEW · PROJECT & VERIFICATION",
		primary:   []keyDesc{{"Enter / t", "Run checks/tests"}, {"r", "Run/resume scan"}},
		secondary: []keyDesc{{"n", "Fresh reanalyze"}, {"R", "Retry failed run"}},
		commands:  []keyDesc{{":test", "Run checks"}, {":analyze", "Run analysis"}},
	},
	PageAgents: {
		title:     "[2] WORK · TASKS & EXECUTION",
		primary:   []keyDesc{{"i / Enter", "Agent detail"}, {"d", "Inspect diff"}},
		secondary: []keyDesc{{"←/→", "Execution section"}, {"a / c", "Apply & commit"}, {"r / x", "Repair / reject"}},
		commands:  []keyDesc{{":execution", "Run detail"}, {":run-output ID", "Private text"}, {":run-input", "Decision"}, {":run-export", "Export trace"}},
	},
	PagePlan: {
		title:     "[2] AGENT STUDIO · TASK PLAN",
		primary:   []keyDesc{{"g", "Approve & generate"}, {"s", "Save draft"}, {"i", "Investigate scope"}},
		secondary: []keyDesc{{"a / x", "Accept / cancel"}, {"e / f / t", "Edit plan fields"}, {"←/→", "Change section"}},
		commands:  []keyDesc{{":investigate", "Investigate scope"}, {":replan", "Use current base"}, {":repairs on|off", "Automatic repair"}},
	},
	PageBugs: {
		title:     "[2] AGENT STUDIO · ISSUES & FINDINGS",
		primary:   []keyDesc{{"Space / x", "Select batch"}, {"Enter / a", "Review fix plan"}},
		secondary: []keyDesc{{"s / t", "Status / type"}, {"i / d", "Inspect / delete"}},
		commands:  []keyDesc{{":fix", "Fix finding"}, {":delete", "Delete finding"}},
	},
	PageIssueDetail: {
		title:     "[2] AGENT STUDIO · ISSUE INSPECTOR",
		primary:   []keyDesc{{"Enter / f", "Review fix plan"}, {"i / Esc", "Back to issues"}},
		secondary: []keyDesc{{"←/→", "Change section"}, {"j/k", "Prev/next issue"}},
		commands:  []keyDesc{{":fix", "Fix finding"}, {":delete", "Delete finding"}},
	},
	PageFeatures: {
		title:     "[2] AGENT STUDIO · FEATURE SUGGESTIONS",
		primary:   []keyDesc{{"Enter", "Inspect feature"}, {"f / a", "Review feature plan"}},
		secondary: []keyDesc{{"r", "Run/resume scan"}, {"n", "Fresh reanalyze"}},
		commands:  []keyDesc{{":agent <msg>", "Custom feature"}, {":analyze", "Run analysis"}},
	},
	PageFeatureDetail: {
		title:     "FEATURE",
		primary:   []keyDesc{{"f / Enter", "Review feature plan"}, {"Esc", "Back"}},
		secondary: []keyDesc{{"←/→", "Change section"}, {"PgUp/PgDn", "Scroll"}},
		commands:  []keyDesc{{":agent <goal>", "Custom feature"}},
	},
	PageAnalysisRun: {
		title:     "ANALYSIS",
		primary:   []keyDesc{{"r", "Run/resume scan"}, {"P / R", "Pause / retry"}},
		secondary: []keyDesc{{"x", "Reset config"}, {"n", "Fresh reanalyze"}},
		commands:  []keyDesc{{":batch <n>", "Set batch size"}, {":budget <s>", "Set time budget"}},
	},
	PageAnalysisFiles: {
		title:     "FILE SCOPE",
		primary:   []keyDesc{{"Enter / Space", "Toggle scope"}, {"r", "Run scoped scan"}},
		secondary: []keyDesc{{"n", "Fresh reanalyze"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":open <file>", "Open file"}, {":analyze", "Run analysis"}},
	},
}

var hubThreeToFiveHelpSpecs = map[Page]pageHelpSpec{
	PageEditor: {
		title:     "FILES & DIFF",
		primary:   []keyDesc{{"Enter / a", "Apply & commit"}, {"x", "Reject hunk"}},
		secondary: []keyDesc{{"h/l", "Scroll long lines"}, {"w / z", "Reviewed / compare"}},
		commands:  []keyDesc{{":apply", "Apply & commit"}, {":undo", "Guarded undo"}},
	},
	PageGit: {
		title:     "[3] CODE & DIFF · GIT & WORKTREES",
		primary:   []keyDesc{{"Enter", "Inspect worktree"}, {"m", "Merge worktree"}},
		secondary: []keyDesc{{"x / c", "Reject / clean up"}, {"r / R", "Refresh / reindex"}},
		commands:  []keyDesc{{":merge / :reject", "Decide integration"}, {":reindex", "Update index"}},
	},
	PageWorktreeDetail: {
		title:     "WORKTREE · INTEGRATION",
		primary:   []keyDesc{{"m", "Merge"}, {"x / c", "Reject / clean up"}},
		secondary: []keyDesc{{"r / R", "Refresh / reindex"}, {"Esc", "Back"}},
		commands:  []keyDesc{{":merge", "Merge worktree"}},
	},
	PageHistory: {
		title:     "PROPOSAL HISTORY",
		primary:   []keyDesc{{"Enter", "Load proposal"}, {"a / c", "Apply & commit"}},
		secondary: []keyDesc{{"u", "Guarded undo"}, {"d", "Inspect diff"}},
		commands:  []keyDesc{{":apply", "Apply & commit"}, {":undo", "Guarded undo"}},
	},
	PageDocs: {
		title:     "[4] HANDBOOK & ARCH · CONTEXT & DEPENDENCIES",
		primary:   []keyDesc{{"r", "Refresh handbook"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageDocsComponents: {
		title:     "COMPONENTS",
		primary:   []keyDesc{{"r", "Refresh handbook"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageDocsDecisions: {
		title:     "[4] HANDBOOK & ARCH · SCENARIOS & DECISIONS",
		primary:   []keyDesc{{"r", "Refresh handbook"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageDiagrams: {
		title:     "ARCHITECTURE DIAGRAM",
		primary:   []keyDesc{{"r", "Refresh diagram"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageInstructions: {
		title:     "[5] GOVERNANCE & RULES · TRUST POLICY",
		primary:   []keyDesc{{"Enter / t", "Toggle trust"}, {"b", "Bootstrap rules"}},
		secondary: []keyDesc{{"g", "Graduate lesson"}, {"u", "Guarded undo"}},
		commands:  []keyDesc{{":trust", "Toggle trust"}, {":bootstrap", "Bootstrap rules"}},
	},
	PageModels: {
		title:     "[5] SETTINGS · MODELS",
		primary:   []keyDesc{{"s / n", "Save new preset"}, {"d", "Delete preset"}},
		secondary: []keyDesc{{"x", "Reset scope"}, {"r", "Refresh models"}},
		commands:  []keyDesc{{":model <s> <id>", "Set model"}, {":preset <id>", "Apply preset"}},
	},
	PageRoles: {
		title:     "[5] SETTINGS · ROLES & BUDGETS",
		primary:   []keyDesc{{"Enter / e", "Edit selected setting"}, {"r", "Refresh local settings"}},
		secondary: []keyDesc{{"o", "Select checkout"}, {"x", "Cancel request; refresh outcome"}},
		commands:  []keyDesc{{":agent-role", "Assign model scope"}, {":agent-budget", "Reduce a budget"}, {":settings-reset", "Confirm restore defaults"}},
	},
	PageCloud: {
		title:     "[5] SETTINGS · CLOUD COPIES",
		primary:   []keyDesc{{"e", "Preview private data"}, {"c", "Confirm copy"}, {"i", "Check remote copy"}, {"d", "Delete remote copy"}},
		secondary: []keyDesc{{"r", "Refresh local receipts"}, {"Enter", "Show or hide details"}, {"x", "Cancel local request"}},
		commands:  []keyDesc{{":cloud-preview", "Choose destination and source"}, {":cloud-check", "Reconcile uncertain outcome"}, {":cloud-delete", "Confirm remote cleanup"}},
	},
	PageAmbient: {
		title:     "[5] SETTINGS · AMBIENT ANALYSIS",
		primary:   []keyDesc{{"w", "Set watch"}, {"g", "Grant provider permission"}, {"v", "Revoke permission"}, {"d", "Disable watch"}},
		secondary: []keyDesc{{"r", "Refresh local state"}, {"Enter", "Show or hide details"}, {"x", "Cancel local request"}},
		commands:  []keyDesc{{":ambient-watch", "Choose paths and timing"}, {":ambient-grant", "Confirm expiry and limits"}, {":ambient-disable", "Stop automatic work"}},
	},
	PageSkills: {
		title:     "[5] SETTINGS · SKILLS",
		primary:   []keyDesc{{"Enter / v", "Validate selected skill"}, {"e", "Confirm enable or disable"}},
		secondary: []keyDesc{{"r", "Refresh local settings"}, {"o", "Select checkout"}, {"x", "Cancel request"}},
		commands:  []keyDesc{{":skill-validate", "Validate captured files"}, {":skill-toggle", "Enable or disable"}, {":agent-settings", "Select main or task checkout"}},
	},
	PageConnections: {
		title:     "[5] SETTINGS · CONNECTIONS",
		primary:   []keyDesc{{"Enter / c", "Confirm discovery or incoming grant"}, {"d", "Confirm revoke"}},
		secondary: []keyDesc{{"s", "Select for plan investigation"}, {"w", "External Work"}, {"r", "Refresh local inventory"}, {"o", "Select checkout"}, {"x", "Cancel request"}},
		commands:  []keyDesc{{":connection-scope", "Select main or task checkout"}, {":connection-use", "Toggle selected plan tool grant"}, {":connections-clear", "Clear plan tool selections"}},
	},
	PageMedia: {
		title:     "[2] WORK · MEDIA & LIVE ADVICE",
		primary:   []keyDesc{{"Enter", "Inspect selected record"}, {"Space", "Select imported attachment"}, {"n / q", "Import / ask with confirmation"}},
		secondary: []keyDesc{{"a / l / o", "Attachments / live / outputs"}, {"s", "Edit live input"}, {"e", "Confirm private export"}, {"r", "Refresh metadata"}, {"x", "Cancel local request"}, {"Esc", "Hide private text"}},
		commands:  []keyDesc{{":live-open", "Confirm a new connection"}, {":live-close", "Close the selected session"}, {":live-reconnect", "Confirm fresh connection without replay"}, {":media-scope", "Choose main or task checkout"}},
	},
	PageArtifacts: {
		title:     "[3] CODE · ARTIFACTS & EVIDENCE",
		primary:   []keyDesc{{"Enter", "Preview retained text"}, {"e", "Confirm private export"}, {"d", "Confirm exact version deletion"}},
		secondary: []keyDesc{{"n / N", "Next / first page"}, {"r", "Refresh metadata"}, {"o", "Select checkout"}, {"w", "Open inspected research report"}, {"x", "Cancel request"}, {"Esc", "Hide private previews"}},
		commands:  []keyDesc{{":artifact-export", "Create private byte-for-byte copy"}, {":library-scope", "Select main or task checkout"}},
	},
	PageMemory: {
		title:     "[5] SETTINGS · MEMORY",
		primary:   []keyDesc{{"Enter", "Inspect advice and source freshness"}, {"a", "Confirm candidate acceptance"}},
		secondary: []keyDesc{{"n / e", "Create / edit advice"}, {"d / f", "Reject candidate / forget accepted"}, {"s", "Read selected source"}, {"t", "Retain proposal lesson or outcome"}, {"r", "Refresh metadata"}, {"o", "Select checkout"}, {"x", "Cancel request"}, {"Esc", "Hide previews"}},
		commands:  []keyDesc{{":memory-create", "kind days title | text"}, {":memory-retain", "outcome or lesson:number days title"}, {":library-scope", "Select main or task checkout"}},
	},
	PageExternalWork: {
		title:     "[5] SETTINGS · EXTERNAL WORK",
		primary:   []keyDesc{{"Enter", "Inspect receipt / open research report"}, {"a", "Confirm reviewed API dispatch"}},
		secondary: []keyDesc{{"r", "Refresh local history"}, {"n", "Research question"}, {"d", "Delegate advisory work"}, {"i", "Reply when input is required"}, {"u", "Reconcile uncertain outcome"}, {"x", "Stop local request"}, {"o", "Select checkout"}},
		commands:  []keyDesc{{":remote-refresh", "Confirm remote task status request"}, {":remote-reconnect", "Confirm retained stream connection"}, {":remote-cancel", "Confirm remote cancellation"}, {":api-reconcile", "Record externally checked outcome"}},
	},
	PageBootstrap: {
		title:     "PROJECT SETUP",
		primary:   []keyDesc{{"Enter / b", "Bootstrap pillar"}, {"t", "Toggle trust"}},
		secondary: []keyDesc{{"g", "Graduate lesson"}, {"u", "Guarded undo"}},
		commands:  []keyDesc{{":bootstrap", "Bootstrap rules"}, {":trust", "Toggle trust"}},
	},
	PageLessons: {
		title:     "LESSONS",
		primary:   []keyDesc{{"Enter / g", "Graduate lesson"}, {"a / c", "Apply & commit"}},
		secondary: []keyDesc{{"d", "Inspect diff"}, {"u", "Guarded undo"}},
		commands:  []keyDesc{{":apply", "Apply AGENTS.md"}, {":undo", "Guarded undo"}},
	},
}
