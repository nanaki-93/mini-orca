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
	{":pause/:resume", "Analysis controls"},
	{":retry/:repair", "Retry task stages"},
	{":reindex", "Reindex repository"},
	{":test / :scan", "Run project checks"},
	{":batch <n>", "Set files per batch"},
	{":budget <s>", "Set time budget (s)"},
	{":attempts <n>", "Set max attempts"},
	{":agent <prompt>", "Plan a coding task"},
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
	spec := pageHelpSpec{title: "Tasks · Missions", primary: []keyDesc{{"Enter", "Open plan"}, {"n", "New draft"}, {"r", "Refresh"}}, secondary: []keyDesc{{"←/→", "Section"}, {"PgUp/PgDn", "Details"}, {"Esc", "History / back"}}, commands: []keyDesc{{":mission-scope main|task", "Select checkout"}, {":mission-child ID", "Select / add child"}, {":mission-edit", "Edit local fields"}, {":mission-budget", "Edit explicit limits"}, {":mission-draft", "Return to draft"}, {":plan", "Ordinary task plan"}}}
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
		title:     "Inbox",
		primary:   []keyDesc{{"Enter", "Open selected area"}, {"n", "New task"}, {"o", "Switch project"}},
		secondary: []keyDesc{{"r / t", "Analysis / checks"}},
		commands:  []keyDesc{{":reanalyze", "Fresh analysis"}, {":test", "Run checks"}},
	},
	PageRepo: {
		title:     "Inbox · Repository",
		primary:   []keyDesc{{"Enter / r", "Reindex repo"}, {"t", "Run checks/tests"}},
		secondary: []keyDesc{{"n", "Fresh reanalyze"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":reindex", "Reindex repo"}, {":project <path>", "Switch repo"}},
	},
	PageProject: {
		title:     "Inbox · Checks",
		primary:   []keyDesc{{"Enter / t", "Run checks/tests"}, {"r", "Run/resume scan"}},
		secondary: []keyDesc{{"n", "Fresh reanalyze"}, {"R", "Retry failed run"}},
		commands:  []keyDesc{{":test", "Run checks"}, {":analyze", "Run analysis"}},
	},
	PageAgents: {
		title:     "Tasks",
		primary:   []keyDesc{{"i / Enter", "Agent detail"}, {"d", "Inspect diff"}},
		secondary: []keyDesc{{"←/→", "Execution section"}, {"a / c", "Apply & commit"}, {"r / x", "Repair / reject"}},
		commands:  []keyDesc{{":execution", "Run detail"}, {":run-output ID", "Private text"}, {":run-input", "Decision"}, {":run-export", "Export trace"}},
	},
	PagePlan: {
		title:     "Tasks · Plan",
		primary:   []keyDesc{{"g", "Approve & generate"}, {"s", "Save draft"}, {"i", "Investigate scope"}},
		secondary: []keyDesc{{"a / x", "Accept / cancel"}, {"e / f / t", "Edit plan fields"}, {"←/→", "Change section"}},
		commands:  []keyDesc{{":investigate", "Investigate scope"}, {":replan", "Use current base"}, {":repairs on|off", "Automatic repair"}},
	},
	PageBugs: {
		title:     "Issues",
		primary:   []keyDesc{{"Space / x", "Select batch"}, {"Enter / a", "Review fix plan"}},
		secondary: []keyDesc{{"s / t", "Status / type"}, {"i / d", "Inspect / delete"}},
		commands:  []keyDesc{{":fix", "Fix finding"}, {":delete", "Delete finding"}},
	},
	PageIssueDetail: {
		title:     "Issues · Finding",
		primary:   []keyDesc{{"Enter / f", "Review fix plan"}, {"i / Esc", "Back to issues"}},
		secondary: []keyDesc{{"←/→", "Change section"}, {"j/k", "Prev/next issue"}},
		commands:  []keyDesc{{":fix", "Fix finding"}, {":delete", "Delete finding"}},
	},
	PageFeatures: {
		title:     "Tasks · Feature ideas",
		primary:   []keyDesc{{"Enter", "Inspect feature"}, {"f / a", "Review feature plan"}},
		secondary: []keyDesc{{"r", "Run/resume scan"}, {"n", "Fresh reanalyze"}},
		commands:  []keyDesc{{":agent <msg>", "Custom feature"}, {":analyze", "Run analysis"}},
	},
	PageFeatureDetail: {
		title:     "Tasks · Feature idea",
		primary:   []keyDesc{{"f / Enter", "Review feature plan"}, {"Esc", "Back"}},
		secondary: []keyDesc{{"←/→", "Change section"}, {"PgUp/PgDn", "Scroll"}},
		commands:  []keyDesc{{":agent <goal>", "Custom feature"}},
	},
	PageAnalysisRun: {
		title:     "Inbox · Analysis",
		primary:   []keyDesc{{"r", "Run/resume scan"}, {"P / R", "Pause / retry"}},
		secondary: []keyDesc{{"x", "Reset config"}, {"n", "Fresh reanalyze"}},
		commands:  []keyDesc{{":batch <n>", "Set batch size"}, {":budget <s>", "Set time budget"}},
	},
	PageAnalysisFiles: {
		title:     "Inbox · Analysis scope",
		primary:   []keyDesc{{"Enter / Space", "Toggle scope"}, {"r", "Run scoped scan"}},
		secondary: []keyDesc{{"n", "Fresh reanalyze"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":open <file>", "Open file"}, {":analyze", "Run analysis"}},
	},
}

var hubThreeToFiveHelpSpecs = map[Page]pageHelpSpec{
	PageEditor: {
		title:     "Code · Files & review",
		primary:   []keyDesc{{"w / W", "Reviewed / next"}, {"Enter / a", "Apply & commit"}},
		secondary: []keyDesc{{"x", "Reject hunk"}, {"n / f / v", "Hunk/file/layout"}},
		commands:  []keyDesc{{":apply", "Apply & commit"}, {":undo", "Guarded undo"}},
	},
	PageGit: {
		title:     "Branches",
		primary:   []keyDesc{{"Enter", "Inspect worktree"}, {"m", "Merge worktree"}},
		secondary: []keyDesc{{"x / c", "Reject / clean up"}, {"r / R", "Refresh / reindex"}},
		commands:  []keyDesc{{":merge / :reject", "Decide integration"}, {":reindex", "Update index"}},
	},
	PageWorktreeDetail: {
		title:     "Branches · Worktree",
		primary:   []keyDesc{{"m", "Merge"}, {"x / c", "Reject / clean up"}},
		secondary: []keyDesc{{"r / R", "Refresh / reindex"}, {"Esc", "Back"}},
		commands:  []keyDesc{{":merge", "Merge worktree"}},
	},
	PageHistory: {
		title:     "Tasks · Proposals",
		primary:   []keyDesc{{"Enter", "Load proposal"}, {"a / c", "Apply & commit"}},
		secondary: []keyDesc{{"u", "Guarded undo"}, {"d", "Inspect diff"}},
		commands:  []keyDesc{{":apply", "Apply & commit"}, {":undo", "Guarded undo"}},
	},
	PageDocs: {
		title:     "Docs · Context",
		primary:   []keyDesc{{"r", "Refresh handbook"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageDocsComponents: {
		title:     "Docs · Components",
		primary:   []keyDesc{{"r", "Refresh handbook"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageDocsDecisions: {
		title:     "Docs · Scenarios & decisions",
		primary:   []keyDesc{{"r", "Refresh handbook"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageDiagrams: {
		title:     "Docs · Architecture",
		primary:   []keyDesc{{"r", "Refresh diagram"}, {"n", "Fresh reanalyze"}},
		secondary: []keyDesc{{"R", "Retry failed run"}, {"P", "Pause scan"}},
		commands:  []keyDesc{{":analyze", "Run analysis"}, {":reanalyze", "Fresh analysis"}},
	},
	PageInstructions: {
		title:     "Settings · Trust",
		primary:   []keyDesc{{"Enter / t", "Toggle trust"}, {"b", "Bootstrap rules"}},
		secondary: []keyDesc{{"g", "Graduate lesson"}, {"u", "Guarded undo"}},
		commands:  []keyDesc{{":trust", "Toggle trust"}, {":bootstrap", "Bootstrap rules"}},
	},
	PageModels: {
		title:     "Settings · Models",
		primary:   []keyDesc{{"s / n", "Save new preset"}, {"d", "Delete preset"}},
		secondary: []keyDesc{{"x", "Reset scope"}, {"r", "Refresh models"}},
		commands:  []keyDesc{{":model <s> <id>", "Set model"}, {":preset <id>", "Apply preset"}},
	},
	PageRoles: {
		title:     "Settings · Roles & budgets",
		primary:   []keyDesc{{"Enter / e", "Edit selected setting"}, {"r", "Refresh local settings"}},
		secondary: []keyDesc{{"o", "Select checkout"}, {"x", "Cancel request; refresh outcome"}},
		commands:  []keyDesc{{":agent-role", "Assign model scope"}, {":agent-budget", "Reduce a budget"}, {":settings-reset", "Confirm restore defaults"}},
	},
	PageCloud: {
		title:     "Settings · Cloud copies",
		primary:   []keyDesc{{"e", "Preview private data"}, {"c", "Confirm copy"}, {"i", "Check remote copy"}, {"d", "Delete remote copy"}},
		secondary: []keyDesc{{"r", "Refresh local receipts"}, {"Enter", "Show or hide details"}, {"x", "Cancel local request"}},
		commands:  []keyDesc{{":cloud-preview", "Choose destination and source"}, {":cloud-check", "Reconcile uncertain outcome"}, {":cloud-delete", "Confirm remote cleanup"}},
	},
	PageAmbient: {
		title:     "Settings · Ambient analysis",
		primary:   []keyDesc{{"w", "Set watch"}, {"g", "Grant provider permission"}, {"v", "Revoke permission"}, {"d", "Disable watch"}},
		secondary: []keyDesc{{"r", "Refresh local state"}, {"Enter", "Show or hide details"}, {"x", "Cancel local request"}},
		commands:  []keyDesc{{":ambient-watch", "Choose paths and timing"}, {":ambient-grant", "Confirm expiry and limits"}, {":ambient-disable", "Stop automatic work"}},
	},
	PageSkills: {
		title:     "Settings · Skills",
		primary:   []keyDesc{{"Enter / v", "Validate selected skill"}, {"e", "Confirm enable or disable"}},
		secondary: []keyDesc{{"r", "Refresh local settings"}, {"o", "Select checkout"}, {"x", "Cancel request"}},
		commands:  []keyDesc{{":skill-validate", "Validate captured files"}, {":skill-toggle", "Enable or disable"}, {":agent-settings", "Select main or task checkout"}},
	},
	PageConnections: {
		title:     "Settings · Connections",
		primary:   []keyDesc{{"Enter / c", "Confirm discovery or incoming grant"}, {"d", "Confirm revoke"}},
		secondary: []keyDesc{{"s", "Select for plan investigation"}, {"w", "External Work"}, {"r", "Refresh local inventory"}, {"o", "Select checkout"}, {"x", "Cancel request"}},
		commands:  []keyDesc{{":connection-scope", "Select main or task checkout"}, {":connection-use", "Toggle selected plan tool grant"}, {":connections-clear", "Clear plan tool selections"}},
	},
	PageMedia: {
		title:     "Tasks · Media",
		primary:   []keyDesc{{"Enter", "Inspect selected record"}, {"Space", "Select imported attachment"}, {"n / q", "Import / ask with confirmation"}},
		secondary: []keyDesc{{"a / l / o", "Attachments / live / outputs"}, {"s", "Edit live input"}, {"e", "Confirm private export"}, {"r", "Refresh metadata"}, {"x", "Cancel local request"}, {"Esc", "Hide private text"}},
		commands:  []keyDesc{{":live-open", "Confirm a new connection"}, {":live-close", "Close the selected session"}, {":live-reconnect", "Confirm fresh connection without replay"}, {":media-scope", "Choose main or task checkout"}},
	},
	PageArtifacts: {
		title:     "Tasks · Artifacts",
		primary:   []keyDesc{{"Enter", "Preview retained text"}, {"e", "Confirm private export"}, {"d", "Confirm exact version deletion"}},
		secondary: []keyDesc{{"n / N", "Next / first page"}, {"r", "Refresh metadata"}, {"o", "Select checkout"}, {"w", "Open inspected research report"}, {"x", "Cancel request"}, {"Esc", "Hide private previews"}},
		commands:  []keyDesc{{":artifact-export", "Create private byte-for-byte copy"}, {":library-scope", "Select main or task checkout"}},
	},
	PageMemory: {
		title:     "Settings · Memory",
		primary:   []keyDesc{{"Enter", "Inspect advice and source freshness"}, {"a", "Confirm candidate acceptance"}},
		secondary: []keyDesc{{"n / e", "Create / edit advice"}, {"d / f", "Reject candidate / forget accepted"}, {"s", "Read selected source"}, {"t", "Retain proposal lesson or outcome"}, {"r", "Refresh metadata"}, {"o", "Select checkout"}, {"x", "Cancel request"}, {"Esc", "Hide previews"}},
		commands:  []keyDesc{{":memory-create", "kind days title | text"}, {":memory-retain", "outcome or lesson:number days title"}, {":library-scope", "Select main or task checkout"}},
	},
	PageExternalWork: {
		title:     "Settings · External work",
		primary:   []keyDesc{{"Enter", "Inspect receipt / open research report"}, {"a", "Confirm reviewed API dispatch"}},
		secondary: []keyDesc{{"r", "Refresh local history"}, {"n", "Research question"}, {"d", "Delegate advisory work"}, {"i", "Reply when input is required"}, {"u", "Reconcile uncertain outcome"}, {"x", "Stop local request"}, {"o", "Select checkout"}},
		commands:  []keyDesc{{":remote-refresh", "Confirm remote task status request"}, {":remote-reconnect", "Confirm retained stream connection"}, {":remote-cancel", "Confirm remote cancellation"}, {":api-reconcile", "Record externally checked outcome"}},
	},
	PageBootstrap: {
		title:     "Settings · Bootstrap",
		primary:   []keyDesc{{"Enter / b", "Bootstrap pillar"}, {"t", "Toggle trust"}},
		secondary: []keyDesc{{"g", "Graduate lesson"}, {"u", "Guarded undo"}},
		commands:  []keyDesc{{":bootstrap", "Bootstrap rules"}, {":trust", "Toggle trust"}},
	},
	PageLessons: {
		title:     "Settings · Lessons",
		primary:   []keyDesc{{"Enter / g", "Graduate lesson"}, {"a / c", "Apply & commit"}},
		secondary: []keyDesc{{"d", "Inspect diff"}, {"u", "Guarded undo"}},
		commands:  []keyDesc{{":apply", "Apply AGENTS.md"}, {":undo", "Guarded undo"}},
	},
}
