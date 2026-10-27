package tui

import (
	"sort"
	"strings"
	"unicode"
)

type commandItem struct {
	command, title, hint string
	argument             bool
	page                 Page
}

const requiredCommandArgumentHint = "Argument required. Type a value, then Enter."

func commandCatalog() []commandItem {
	items := []commandItem{
		{command: "project", title: "Open or switch project", hint: "project /absolute/path · Restore local project data and index.", argument: true},
		{command: "agent", title: "New task", hint: "agent <request> · Prepare an editable plan before generation.", argument: true},
		{command: "analyze", title: "Run or resume analysis", hint: "Analyze the selected scope. Remote providers require consent."},
		{command: "test", title: "Run project checks", hint: "Execute configured checks in an isolated workspace. Requires project trust."},
		{command: "reindex", title: "Update project index", hint: "Read current files and invalidate outdated checks and workflows."},
		{command: "reconnect", title: "Reconnect to daemon", hint: "Reconnect and reload local workspace information."},
		{command: "messages", title: "Show recent messages", hint: "Status updates and errors from this session, newest first."},
		{command: "back", title: "Go back", hint: "Return to the previous view and its selection (Backspace or Ctrl+O)."},
		{command: "web", title: "Open Web UI & Diff Viewer", hint: "Open the local browser UI for side-by-side diffs, files, issues and guarded Apply/Undo."},
	}
	for _, hub := range HubTabs() {
		for _, view := range hub.SubPages {
			if view.Page == PageIssueDetail || view.Page == PagePlan {
				continue
			}
			command := string(view.Page)
			if view.Page == PageProject {
				command = "checks"
			}
			if view.Page == PageBootstrap {
				command = "setup"
			}
			items = append(items, commandItem{command: command, title: hub.Label + " / " + view.Label, hint: "Open this view. No work starts until you choose an action.", page: view.Page})
		}
	}
	return append(items,
		commandItem{command: "missions", title: "Work / Mission plans", hint: "Browse local mission history; opening or editing never calls a provider."},
		commandItem{command: "mission-scope", title: "Select mission checkout", hint: "mission-scope main or task · Read local history, settings and indexed paths.", argument: true},
		commandItem{command: "mission-refresh", title: "Refresh mission history", hint: "Read current catalog, settings and index; retain the editable draft."},
		commandItem{command: "mission-open", title: "Inspect a saved mission", hint: "mission-open <ID> · Read the plan and check its captured identities.", argument: true},
		commandItem{command: "mission-draft", title: "Return to mission draft", hint: "Show the retained local draft without starting work."},
		commandItem{command: "mission-new", title: "Draft a mission goal", hint: "mission-new <goal> · Create a local draft, then choose files and budgets.", argument: true},
		commandItem{command: "mission-edit", title: "Edit mission fields locally", hint: "mission-edit goal|files|order|title|approach|criteria|writes|reads|depends <value> · Lists use semicolons.", argument: true},
		commandItem{command: "mission-budget", title: "Edit a mission allocation", hint: "mission-budget mission|planning|child <field> <integer> · Fits the displayed ceiling.", argument: true},
		commandItem{command: "mission-child", title: "Select or add a child", hint: "mission-child <ID> · Local editing only; a new child needs scope, approach, criteria and limits.", argument: true},
		commandItem{command: "mission-remove", title: "Remove a child locally", hint: "mission-remove <ID> · Review remaining dependencies before saving.", argument: true},
		commandItem{command: "mission-plan", title: "Decompose the mission", hint: "Confirm captured source, Code provider and planning budget before model work."},
		commandItem{command: "mission-save", title: "Save mission revision", hint: "Save the complete draft; any revision clears child reviews and final approval."},
		commandItem{command: "mission-review", title: "Review selected child", hint: "Inspect and confirm the concrete child against the exact saved plan hash."},
		commandItem{command: "mission-approve", title: "Approve reviewed mission", hint: "Confirm the exact hash after every child is reviewed. Starts no execution."},
		commandItem{command: "mission-cancel", title: "Cancel mission request", hint: "Retain the draft; refresh local history before retrying a possibly completed save."},
		commandItem{command: "mission-execution", title: "Inspect mission execution", hint: "Read its owner, current base, child progress and recovery receipts without starting work."},
		commandItem{command: "mission-prepare", title: "Prepare an approved mission", hint: "Confirm the fixed plan and clean Git base. Runs no models or checks."},
		commandItem{command: "mission-trust", title: "Trust mission check commands", hint: "Confirm the displayed commands for the exact owner checkout and revision."},
		commandItem{command: "mission-start", title: "Start ready mission children", hint: "Confirm models, context, worktrees, checks and fixed allocations for unstarted children."},
		commandItem{command: "mission-pause", title: "Pause the active mission wave", hint: "Interrupt active children; their allocations remain consumed."},
		commandItem{command: "mission-stop", title: "Stop the mission", hint: "Cancel execution permanently while retaining proposals and evidence."},
		commandItem{command: "mission-proposal", title: "Review the selected child proposal", hint: "Open its exact owned candidate in the ordinary diff inspector."},
		commandItem{command: "mission-candidate", title: "Inspect a retained candidate generation", hint: "mission-candidate 0|1|2|3 · 0 is the original proposal; failed candidates remain read-only until ordinary review guards pass.", argument: true},
		commandItem{command: "mission-model-history", title: "Inspect original model execution", hint: "Read the original child's model history as historical evidence without rerunning it."},
		commandItem{command: "mission-revalidate", title: "Revalidate against the current base", hint: "Confirm fresh checks of retained content in a new candidate; no model calls or Apply."},
		commandItem{command: "mission-integrate", title: "Integrate the committed mission child", hint: "Confirm its exact proposal, commit and approved destination/order."},
		commandItem{command: "mission-reconcile", title: "Recover an interrupted mission wave", hint: "Inspect expired ownership and record interruption without restarting children."},
		commandItem{command: "mission-recover-integration", title: "Recover mission integration", hint: "Inspect the retained Git outcome without repeating a merge."},
		commandItem{command: "mission-recover-revalidation", title: "Recover interrupted revalidation", hint: "Retain candidates and record interruption without replaying checks."},
		commandItem{command: "mission-cleanup", title: "Clean a proven integrated child", hint: "Retry only the retained integrated checkout and branch cleanup."},
		commandItem{command: "media", title: "Work / Media and live advice", hint: "Inspect imported media, advisory outputs and owned live sessions."},
		commandItem{command: "media-scope", title: "Select media checkout", hint: "media-scope main or task \u00b7 Read local availability without connecting.", argument: true},
		commandItem{command: "media-view", title: "Select media section", hint: "media-view attachments, sessions or outputs.", argument: true},
		commandItem{command: "media-refresh", title: "Refresh media metadata", hint: "Read local history; never connect, send or replay input."},
		commandItem{command: "media-import", title: "Review a local attachment for import", hint: "media-import <path> \u00b7 Inspect bounded bytes inside the selected checkout before local import.", argument: true},
		commandItem{command: "media-select", title: "Toggle selected attachment", hint: "Choose up to four exact imported versions; no provider request."},
		commandItem{command: "media-ask", title: "Ask about selected attachments", hint: "media-ask <question> \u00b7 Confirm exact versions, settings, destination and sharing.", argument: true},
		commandItem{command: "media-inspect", title: "Inspect selected media or session", hint: "Disclose private artifact text or inspect live metadata; no provider request."},
		commandItem{command: "media-hide", title: "Hide private media text", hint: "Close the explicit transcript or advisory preview."},
		commandItem{command: "media-export", title: "Export selected media privately", hint: "Confirm exact artifact version and hash; no automatic audio playback."},
		commandItem{command: "media-cancel", title: "Cancel pending media request", hint: "Stop the local request; refresh to establish any retained outcome."},
		commandItem{command: "live-open", title: "Connect live advice", hint: "Confirm a bounded live session using the configured Code destination."},
		commandItem{command: "live-send", title: "Share text with the selected live session", hint: "live-send <text> \u00b7 Review exact session and sharing consent before dispatch.", argument: true},
		commandItem{command: "live-send-media", title: "Share selected JPEG or PCM", hint: "Confirm exactly one imported version up to 64 KiB."},
		commandItem{command: "live-interrupt", title: "Interrupt selected live turn", hint: "Explicit control; no automatic replay."},
		commandItem{command: "live-close", title: "Close selected live session", hint: "Close and join its daemon owner."},
		commandItem{command: "live-reconnect", title: "Review a fresh live connection", hint: "Confirm remaining lifetime and budget; previous input is never replayed."},
		commandItem{command: "library-scope", title: "Select artifact and memory checkout", hint: "library-scope main or task · Read the selected checkout's local library.", argument: true},
		commandItem{command: "library-refresh", title: "Refresh artifact and memory metadata", hint: "Read local records; private text requires separate inspection."},
		commandItem{command: "library-inspect", title: "Inspect selected library record", hint: "Explicitly disclose bounded retained text and source freshness."},
		commandItem{command: "library-hide", title: "Hide private library previews", hint: "Close artifact, advice and source disclosures."},
		commandItem{command: "library-cancel", title: "Cancel library request", hint: "Refresh afterward to establish any local write outcome."},
		commandItem{command: "artifact-next", title: "Next artifact page", hint: "Continue the same catalog snapshot; changed catalogs require refresh."},
		commandItem{command: "artifact-first", title: "First artifact page", hint: "Read the current catalog from its beginning."},
		commandItem{command: "artifact-export", title: "Export selected artifact", hint: "Confirm a new private copy of the exact version and hash."},
		commandItem{command: "artifact-delete", title: "Delete selected artifact version", hint: "Confirm exact deletion; recovery evidence and active references remain protected."},
		commandItem{command: "artifact-report", title: "Open artifact's research report", hint: "Inspect the research artifact first; open its cited local report in your browser."},
		commandItem{command: "memory-create", title: "Retain candidate advice", hint: "memory-create decision|lesson <days> <title> | <text> · Capture current plan or open-file sources.", argument: true},
		commandItem{command: "memory-edit", title: "Revise retained advice", hint: "memory-edit decision|lesson <days> <title> | <text> · Recapture sources and reset acceptance.", argument: true},
		commandItem{command: "memory-retain", title: "Retain a proposal lesson or check outcome", hint: "memory-retain outcome|lesson:<number> <days> <title> · The host derives text from the exact proposal receipt.", argument: true},
		commandItem{command: "memory-accept", title: "Accept inspected candidate advice", hint: "Confirm current source scope and retention before future retrieval."},
		commandItem{command: "memory-reject", title: "Reject candidate advice", hint: "Confirm removal of a candidate through guarded forget."},
		commandItem{command: "memory-forget", title: "Forget accepted advice", hint: "Confirm removal from future retrieval; prior invocation history remains separate."},
		commandItem{command: "memory-source", title: "Read retained advice's source", hint: "memory-source <number> · Check current scope and exact file hash before a read-only preview.", argument: true},
		commandItem{command: "external-scope", title: "Select external work checkout", hint: "external-scope main or task · Read local receipts for this checkout.", argument: true},
		commandItem{command: "external-refresh", title: "Refresh local external history", hint: "Read local API, research and remote-task receipts without external requests."},
		commandItem{command: "external-inspect", title: "Inspect selected external receipt", hint: "Disclose private API or remote detail; research opens the retained local report in your browser."},
		commandItem{command: "external-cancel", title: "Stop local external request", hint: "Cancel owned transport; refresh local history to establish any external outcome."},
		commandItem{command: "api-approve", title: "Approve exact external API request", hint: "Inspect the request first, then confirm one dispatch of its retained version/hash."},
		commandItem{command: "api-reconcile", title: "Reconcile uncertain API outcome", hint: "api-reconcile applied|not_applied|other <observation> · Record your external check without replay.", argument: true},
		commandItem{command: "research", title: "Research with Google Search", hint: "research [--question-only] <question> · Capture plan source hashes, then confirm provider, search and sharing.", argument: true},
		commandItem{command: "delegate", title: "Delegate bounded advisory work", hint: "delegate <remote-id> <analysis|review> <skill> [--question-only] <question> · Confirm destination and source sharing.", argument: true},
		commandItem{command: "report-open", title: "Open retained research report", hint: "Open the selected local report with citations and search suggestions in your system browser."},
		commandItem{command: "remote-refresh", title: "Refresh retained remote task", hint: "Confirm a remote status request for the selected receipt; never resend prior messages."},
		commandItem{command: "remote-reconnect", title: "Reconnect retained remote stream", hint: "Confirm reconnection to an advertised stream for the existing task."},
		commandItem{command: "remote-cancel", title: "Cancel retained remote task", hint: "Confirm remote cleanup; inspect the receipt for the resulting outcome."},
		commandItem{command: "remote-input", title: "Reply to remote input request", hint: "remote-input <reply> · Confirm sharing the bounded reply once.", argument: true},
		commandItem{command: "remote-reconcile", title: "Reconcile uncertain remote task", hint: "remote-reconcile <observation> · Record your external check locally without a remote request.", argument: true},
		commandItem{command: "connection-scope", title: "Select connections checkout", hint: "connection-scope main or task · Read local inventory for this checkout.", argument: true},
		commandItem{command: "connections-refresh", title: "Refresh local connection inventory", hint: "Read configured destinations and grant receipts; does not discover or connect."},
		commandItem{command: "connection-discover", title: "Discover selected connection", hint: "Confirm discovery or incoming access for the selected destination and checkout."},
		commandItem{command: "connection-revoke", title: "Revoke selected connection grant", hint: "Confirm revocation and cancellation of operations owned by this grant."},
		commandItem{command: "connection-cancel", title: "Cancel connection request", hint: "Stop the request; refresh local inventory before reusing its grant."},
		commandItem{command: "connection-use", title: "Select connection for plan investigation", hint: "Select at most two granted MCP/API connections; source sharing is confirmed when investigating."},
		commandItem{command: "connections-clear", title: "Remove plan connection selections", hint: "Clear ephemeral tool choices from the current draft without revoking existing grants."},
		commandItem{command: "agent-settings", title: "Choose settings checkout", hint: "main: project checkout (default). task: selected task worktree. Scope for future runs. Open :roles or :skills to edit.", argument: true},
		commandItem{command: "settings-refresh", title: "Refresh agent settings", hint: "Read local saved settings; no provider discovery."},
		commandItem{command: "cloud", title: "Inspect cloud copies", hint: "Read local destinations and retained receipts. An inspected main-checkout artifact or task can prefill the source."},
		commandItem{command: "cloud-refresh", title: "Refresh local cloud receipts", hint: "Read local state only; remote availability is checked separately."},
		commandItem{command: "cloud-preview", title: "Preview a private cloud copy", hint: "cloud-preview <destination> artifact|session <source-id> · Read the exact local archive size, hash and destination.", argument: true},
		commandItem{command: "cloud-copy", title: "Copy selected private data", hint: "Review the preview and explicitly confirm sharing with its exact cloud destination."},
		commandItem{command: "cloud-check", title: "Check a retained remote copy", hint: "Confirm contacting the selected copy's destination. Never repeats an upload."},
		commandItem{command: "cloud-delete", title: "Delete a retained remote copy", hint: "Confirm removal of the live cloud copy. Local originals are retained."},
		commandItem{command: "cloud-cancel", title: "Cancel the local cloud request", hint: "Stop local transport; refresh before recovery because a remote outcome may be retained."},
		commandItem{command: "ambient", title: "Inspect ambient analysis", hint: "Read main-checkout watch, grant, budget and last attempt."},
		commandItem{command: "ambient-refresh", title: "Refresh ambient state", hint: "Read the latest main index and local receipt; never calls a provider."},
		commandItem{command: "ambient-watch", title: "Set an ambient watch", hint: "ambient-watch changes <path; path> or interval <seconds> <path; path> · Confirm watch-only detection.", argument: true},
		commandItem{command: "ambient-grant", title: "Grant automatic semantic analysis", hint: "ambient-grant [seconds] · Review provider, filtered source sharing, expiry and fixed budgets."},
		commandItem{command: "ambient-revoke", title: "Revoke automatic permission", hint: "Confirm cancellation of provider work; retain watch-only detection and charges."},
		commandItem{command: "ambient-disable", title: "Disable ambient analysis", hint: "Confirm stopping the owner and clearing its selection; preserve receipts and charges."},
		commandItem{command: "ambient-cancel", title: "Cancel local ambient request", hint: "Stop local transport; refresh before another control because its outcome may be retained."},
		commandItem{command: "settings-cancel", title: "Cancel settings request", hint: "Refresh afterward to confirm whether a save completed."},
		commandItem{command: "settings-reset", title: "Restore configured agent defaults", hint: "Confirm reset for future runs in the selected checkout."},
		commandItem{command: "agent-role", title: "Assign a workflow role", hint: "agent-role creation|testing|review analyze|bug|code · Confirm future-run scope assignment.", argument: true},
		commandItem{command: "agent-budget", title: "Edit an agent budget", hint: "agent-budget <name> <integer> · Must fit the configured ceiling.", argument: true},
		commandItem{command: "skill-validate", title: "Validate selected skill", hint: "Read captured package metadata and hashes; grants no tools or execution."},
		commandItem{command: "skill-toggle", title: "Enable or disable selected skill", hint: "Confirm a future-run change in the selected checkout."},
		commandItem{command: "execution", title: "Inspect task execution", hint: "Open recorded stages and execution detail. No model work starts."},
		commandItem{command: "run-output", title: "Disclose private invocation output", hint: "run-output <ID> · Read bounded model text and redacted tool records from Timeline.", argument: true},
		commandItem{command: "hide-output", title: "Hide private invocation output", hint: "Close the private disclosure and stop its refresh."},
		commandItem{command: "run-cancel", title: "Cancel current native run", hint: "Cancel the active root; the worker owns cleanup."},
		commandItem{command: "run-export", title: "Export a trace snapshot", hint: "Review the destination and retained source-free snapshot; explicit confirmation sends once."},
		commandItem{command: "run-input", title: "Respond to pending task input", hint: "run-input approve, reject or cancel · Exact proposal identity is required; Apply remains separate.", argument: true},
		commandItem{command: "stream-output", title: "Configure task output streaming", hint: "stream-output on or off · Applies to new workflows and retries through provider capability checks.", argument: true},
		commandItem{command: "plan", title: "Review task plan", hint: "Inspect the approach, evidence, scope and acceptance criteria."},
		commandItem{command: "investigate", title: "Investigate task plan", hint: "Read admitted scope and propose evidence, approach and criteria for review."},
		commandItem{command: "accept-investigation", title: "Accept planning suggestions", hint: "Update the draft; scope suggestions remain advisory and generation requires approval."},
		commandItem{command: "cancel-investigation", title: "Cancel plan investigation", hint: "Stop the current request and retain the editable draft."},
		commandItem{command: "pause", title: "Pause analysis", hint: "Stop dispatching new analysis work and retain completed results."},
		commandItem{command: "reanalyze", title: "Start fresh analysis", hint: "Run a new analysis, preserving work already in progress."},
		commandItem{command: "retry", title: "Retry a task stage", hint: "retry test or retry review · Reuse implementation or passing checks.", argument: true},
		commandItem{command: "open", title: "Read a source file", hint: "open <project-relative path> · Read-only source viewer.", argument: true},
		commandItem{command: "trust", title: "Manage execution trust", hint: "Grant or revoke permission to execute this project's checks."},
		commandItem{command: "apply", title: "Apply reviewed proposal", hint: "Write the checked proposal after every changed file has been reviewed."},
		commandItem{command: "undo", title: "Undo last applied change", hint: "Restore the previous source only when the current hashes still match."},
		commandItem{command: "merge", title: "Merge selected worktree", hint: "Integrate the reviewed task into its destination branch."},
		commandItem{command: "reject", title: "Reject selected task or worktree", hint: "Discard the fix and return its linked issues to TODO."},
		commandItem{command: "model", title: "Assign a model", hint: "model <analyze|bug|code> <model id>", argument: true},
		commandItem{command: "palette", title: "Change color theme", hint: "Cycle Tokyo, CRT, Amber and Paper."},
		commandItem{command: "keys", title: "Keyboard reference", hint: "Browse shortcuts for the current view and all colon commands."},
		commandItem{command: "quit", title: "Quit Mini-Orca", hint: "Close the terminal interface."},
	)
}

func (s *State) commandMatches() []commandItem {
	if s.commandArgumentContext() != nil {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(s.CmdInput))
	switch query {
	case "q", "exit":
		query = "quit"
	case "help":
		query = "keys"
	}
	catalog := commandCatalog()
	recent := s.recentCommandRanks()
	type ranked struct {
		item  commandItem
		score int
		order int
	}
	var matches []ranked
	for order, item := range catalog {
		if score, ok := commandScore(item, query); ok {
			matches = append(matches, ranked{item: item, score: score + recent[item.command], order: order})
		}
	}
	if len(matches) == 0 && query != "" {
		// Tolerate typos and abbreviations ("aply", "wt") with a subsequence match.
		for order, item := range catalog {
			if isSubsequence(strings.ReplaceAll(query, " ", ""), item.command) {
				matches = append(matches, ranked{item: item, score: recent[item.command], order: order})
			}
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].order < matches[j].order
	})
	items := make([]commandItem, len(matches))
	for i, match := range matches {
		items[i] = match.item
	}
	return items
}

// commandScore keeps the existing every-word-matches rule and ranks exact
// commands, command prefixes and word starts ahead of mid-word hits.
func commandScore(item commandItem, query string) (int, bool) {
	if query == "" {
		return 0, true
	}
	command := strings.ToLower(item.command)
	title := strings.ToLower(item.title)
	text := command + " " + title
	score := 0
	for _, word := range strings.Fields(query) {
		index := strings.Index(text, word)
		if index < 0 {
			return 0, false
		}
		if index == 0 || text[index-1] == ' ' || text[index-1] == '-' {
			score += 40
		}
	}
	switch {
	case command == query:
		score += 1000
	case strings.HasPrefix(command, query):
		score += 500
	case strings.HasPrefix(title, query):
		score += 200
	}
	return score, true
}

func isSubsequence(query, text string) bool {
	if query == "" {
		return false
	}
	runes := []rune(query)
	index := 0
	for _, r := range strings.ToLower(text) {
		if index < len(runes) && r == runes[index] {
			index++
		}
	}
	return index == len(runes)
}

// recentCommandRanks boosts commands from this session's history, newest first.
func (s *State) recentCommandRanks() map[string]int {
	ranks := map[string]int{}
	for i := len(s.CmdHistory) - 1; i >= 0 && len(ranks) < 8; i-- {
		name, _, _ := strings.Cut(strings.TrimSpace(s.CmdHistory[i]), " ")
		name = strings.ToLower(strings.TrimPrefix(name, ":"))
		if _, seen := ranks[name]; !seen && name != "" {
			ranks[name] = 30 - len(ranks)*3
		}
	}
	return ranks
}

func (s *State) commandArgumentContext() *commandItem {
	command, _, hasSpace := strings.Cut(strings.TrimLeft(s.CmdInput, " "), " ")
	if !hasSpace {
		return nil
	}
	for _, item := range commandCatalog() {
		if item.argument && strings.EqualFold(command, item.command) {
			return &item
		}
	}
	return nil
}

func (s *State) commandMissingArguments() *commandItem {
	for _, item := range commandCatalog() {
		if item.argument && strings.EqualFold(strings.TrimSpace(s.CmdInput), item.command) {
			return &item
		}
	}
	return nil
}

func (s *State) openCommandInput(value string) {
	s.CmdActive, s.CmdInput = true, value
	s.commandCursor = len([]rune(value))
	s.commandSelection, s.commandHistoryIndex = -1, -1
	s.commandHistoryDraft = ""
}

func (s *State) handleCommandInputKey(ev KeyEvent) {
	if ev.Ctrl {
		s.handleCommandControlKey(ev.Key)
		return
	}
	switch ev.Key {
	case "Escape":
		s.CmdActive, s.CmdInput = false, ""
	case "Enter":
		s.submitCommandInput()
	case "Tab":
		s.completeCommand(false)
	case "ArrowUp", "ArrowDown":
		count := len(s.commandMatches())
		if count == 0 {
			s.commandSelection = -1
			return
		}
		delta := 1
		if ev.Key == "ArrowUp" {
			delta = -1
		}
		s.commandSelection = min(max(0, s.commandSelection+delta), count-1)
	case "ArrowLeft", "ArrowRight", "Home", "End", "Backspace", "Delete":
		s.editCommandInput(ev.Key)
	default:
		text := ev.Text
		if text == "" && len([]rune(ev.Key)) == 1 {
			text = ev.Key
		}
		s.insertCommandText(text)
	}
}

func (s *State) handleCommandControlKey(key string) {
	switch strings.ToLower(key) {
	case "a":
		s.commandCursor = 0
	case "e":
		s.commandCursor = len([]rune(s.CmdInput))
	case "u":
		s.CmdInput, s.commandCursor = "", 0
		s.commandSelection = -1
	case "p":
		s.recallCommand(1)
	case "n":
		s.recallCommand(-1)
	}
}

func (s *State) editCommandInput(key string) {
	runes := []rune(s.CmdInput)
	pos := min(max(0, s.commandCursor), len(runes))
	switch key {
	case "Home":
		pos = 0
	case "End":
		pos = len(runes)
	case "ArrowLeft":
		pos = max(0, pos-1)
	case "ArrowRight":
		pos = min(len(runes), pos+1)
	case "Backspace":
		if pos > 0 {
			runes = append(runes[:pos-1], runes[pos:]...)
			pos--
		}
	case "Delete":
		if pos < len(runes) {
			runes = append(runes[:pos], runes[pos+1:]...)
		}
	}
	s.CmdInput, s.commandCursor = string(runes), pos
	s.commandSelection = -1
}

func (s *State) insertCommandText(text string) {
	text = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	if text == "" {
		return
	}
	runes := []rune(s.CmdInput)
	pos := min(max(0, s.commandCursor), len(runes))
	s.CmdInput = string(runes[:pos]) + text + string(runes[pos:])
	s.commandCursor = pos + len([]rune(text))
	s.commandSelection = -1
}

func (s *State) recallCommand(delta int) {
	if s.commandHistoryIndex < 0 {
		s.commandHistoryDraft = s.CmdInput
	}
	s.commandHistoryIndex = min(max(-1, s.commandHistoryIndex+delta), len(s.CmdHistory)-1)
	s.CmdInput = s.commandHistoryDraft
	if s.commandHistoryIndex >= 0 {
		s.CmdInput = strings.TrimPrefix(s.CmdHistory[s.commandHistoryIndex], ":")
	}
	s.commandCursor = len([]rune(s.CmdInput))
	s.commandSelection = -1
}

func (s *State) submitCommandInput() {
	if s.commandSelection >= 0 || strings.TrimSpace(s.CmdInput) == "" {
		s.completeCommand(true)
		return
	}
	matches := s.commandMatches()
	if len(matches) > 0 && !strings.EqualFold(strings.TrimSpace(s.CmdInput), matches[0].command) {
		s.completeCommand(true)
		return
	}
	// A bare command needing arguments opens an editor instead of using an alias.
	if item := s.commandMissingArguments(); item != nil {
		s.openCommandInput(item.command + " ")
		return
	}
	raw := s.CmdInput
	s.CmdActive, s.CmdInput = false, ""
	s.ExecuteCommand(raw)
}

func (s *State) completeCommand(execute bool) {
	items := s.commandMatches()
	if len(items) == 0 {
		return
	}
	item := items[min(max(0, s.commandSelection), len(items)-1)]
	if item.argument || !execute {
		value := item.command
		if item.argument {
			value += " "
		}
		s.openCommandInput(value)
		return
	}
	s.CmdActive, s.CmdInput = false, ""
	s.ExecuteCommand(item.command)
}

func (s *State) execViewCommand(cmd string) bool {
	for _, item := range commandCatalog() {
		if item.page != "" && cmd == item.command {
			s.Navigate(item.page)
			s.CmdOutput = item.title
			return true
		}
	}
	return false
}
