package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type commandItem struct {
	command, title, hint string
	argument             bool
	page                 Page
}

func commandCatalog() []commandItem {
	items := []commandItem{
		{command: "project", title: "Open or switch project", hint: "project /absolute/path · Restore local project data and index.", argument: true},
		{command: "agent", title: "New task", hint: "agent <request> · Prepare an editable plan before generation.", argument: true},
		{command: "analyze", title: "Run or resume analysis", hint: "Analyze the selected scope. Remote providers require consent."},
		{command: "test", title: "Run project checks", hint: "Execute configured checks in an isolated workspace. Requires project trust."},
		{command: "reindex", title: "Update project index", hint: "Read current files and invalidate outdated checks and workflows."},
		{command: "reconnect", title: "Reconnect to daemon", hint: "Reconnect and reload local workspace information."},
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
		commandItem{command: "mission-plan", title: "Decompose the mission", hint: "Confirm captured source, Function provider and planning budget before model work."},
		commandItem{command: "mission-save", title: "Save mission revision", hint: "Save the complete draft; any revision clears child reviews and final approval."},
		commandItem{command: "mission-review", title: "Review selected child", hint: "Inspect and confirm the concrete child against the exact saved plan hash."},
		commandItem{command: "mission-approve", title: "Approve reviewed mission", hint: "Confirm the exact hash after every child is reviewed. Starts no execution."},
		commandItem{command: "mission-cancel", title: "Cancel mission request", hint: "Retain the draft; refresh local history before retrying a possibly completed save."},
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
		commandItem{command: "agent-settings", title: "Select settings checkout", hint: "agent-settings main or task · Load local settings for the main checkout or selected task worktree.", argument: true},
		commandItem{command: "settings-refresh", title: "Refresh agent settings", hint: "Read local saved settings; no provider discovery."},
		commandItem{command: "settings-cancel", title: "Cancel settings request", hint: "Refresh afterward to confirm whether a save completed."},
		commandItem{command: "settings-reset", title: "Restore configured agent defaults", hint: "Confirm reset for future runs in the selected checkout."},
		commandItem{command: "agent-role", title: "Assign a workflow role", hint: "agent-role creation|testing|review analyze|bug|function · Confirm future-run scope assignment.", argument: true},
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
		commandItem{command: "model", title: "Assign a model", hint: "model <analyze|bug|function> <model id>", argument: true},
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
	var items []commandItem
	for _, item := range commandCatalog() {
		text := strings.ToLower(item.command + " " + item.title)
		matches := true
		for _, word := range strings.Fields(query) {
			if !strings.Contains(text, word) {
				matches = false
				break
			}
		}
		if matches {
			if item.command == query {
				items = append([]commandItem{item}, items...)
			} else {
				items = append(items, item)
			}
		}
	}
	return items
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
		delta := 1
		if ev.Key == "ArrowUp" {
			delta = -1
		}
		s.commandSelection = min(max(0, s.commandSelection+delta), max(0, len(s.commandMatches())-1))
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
	for _, item := range commandCatalog() {
		if item.argument && strings.TrimSpace(s.CmdInput) == item.command {
			s.openCommandInput(item.command + " ")
			return
		}
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

func (s *State) renderCommandMenu(th ansiTheme, width, rows int) []string {
	lines := renderPageHeader(th, "COMMANDS", "")
	input := th.renderCommandInput(s.CmdInput, s.commandCursor, width-4)
	lines[1] = "  " + theme.PaintLine(th.surface, th.fg, input, width-4)
	if rows >= 12 {
		lines = append(lines, "")
	}
	if item := s.commandArgumentContext(); item != nil {
		lines = append(lines, "  "+th.cyan+item.title+th.reset)
		return appendWrappedText(lines, item.hint, width)
	}
	items := s.commandMatches()
	if len(items) == 0 {
		return append(lines, "  No matching suggestions.", "  Enter runs the command you typed.")
	}
	selected := min(max(0, s.commandSelection), len(items)-1)
	hint := theme.WrapVisibleLine(items[selected].hint, width-4)
	start, end := windowBounds(len(items), selected, max(1, rows-len(lines)-len(hint)-1))
	commandWidth := min(20, width/3)
	for i := start; i < end; i++ {
		item := items[i]
		line := th.renderCursorPrefix(i == selected) + theme.PadVisibleRight(th.cyan+":"+item.command+th.reset, commandWidth) + " " + rowTitleStyle(th, i == selected) + item.title + th.reset
		lines = append(lines, "  "+theme.TruncateVisible(line, width-4))
	}
	lines = append(lines, "  "+th.muted+fmt.Sprintf("%d–%d of %d commands", start+1, end, len(items))+th.reset)
	for _, line := range hint {
		lines = append(lines, "  "+th.cyan+line+th.reset)
	}
	return lines
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
