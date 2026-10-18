package tui

import (
	"fmt"
	"strings"
)

func (s *State) enqueueAction(act ActionRequest) {
	s.PendingActions = append(s.PendingActions, act)
}

// SelectedIssue returns the currently highlighted IssueRow on Overview or Issues.
func (s *State) SelectedIssue() *IssueRow {
	var list []IssueRow
	if s.Page == PageSummary {
		open := s.openIssues()
		if len(open) > 8 {
			open = open[:8]
		}
		list = open
	} else {
		list = s.FilteredIssues()
	}
	idx := s.ClampedIdx()
	if idx < 0 || idx >= len(list) {
		return nil
	}
	row := list[idx]
	return &row
}

func (s *State) toggleSelectedIssue() {
	issue := s.SelectedIssue()
	if issue == nil || issue.Status == "done" {
		return
	}
	for idx, id := range s.SelectedIssueIDs {
		if id == issue.ID {
			s.SelectedIssueIDs = append(s.SelectedIssueIDs[:idx], s.SelectedIssueIDs[idx+1:]...)
			return
		}
	}
	s.SelectedIssueIDs = append(s.SelectedIssueIDs, issue.ID)
}

func (s *State) dismissSelectedIssue() {
	issue := s.SelectedIssue()
	if issue == nil || issue.Status == "done" {
		return
	}
	s.CmdOutput = fmt.Sprintf("Dismissed finding: %s", issue.Name)
	target := issue.Target
	s.enqueueAction(ActionRequest{Kind: "dismiss-finding", Target: issue.ID, Issue: &target})
}

func (s *State) dispatchSelectedIssues() {
	if len(s.SelectedIssueIDs) > 0 {
		s.dispatchBatchIssues()
		return
	}
	issue := s.SelectedIssue()
	if issue == nil || issue.Status == "done" {
		return
	}
	s.dispatchSingleIssue(*issue)
}

func (s *State) dispatchSingleIssue(issue IssueRow) {
	kind := "fix"
	if issue.Category == "security" || issue.Category == "performance" {
		kind = issue.Category
	}
	seed := &ChangeSeed{
		FindingTargets:     []FindingTarget{issue.Target},
		Title:              issue.Name,
		Paths:              []string{issue.Path},
		Kind:               kind,
		Message:            fmt.Sprintf("Address this finding: %s\nLocation: %s:%d\nCause: %s\nProposed solution: %s", issue.Title, issue.Path, issue.Line, issue.Cause, issue.Solution),
		AcceptanceCriteria: issue.AcceptanceCriteria,
	}
	s.CmdOutput = fmt.Sprintf("Dispatched Fix for %s (%s)", issue.Name, issue.Path)
	s.enqueueAction(ActionRequest{Kind: "start-agent", Target: issue.Path, Seed: seed})
}

func (s *State) dispatchBatchIssues() {
	open := s.openIssues()
	selectedSet := make(map[string]bool, len(s.SelectedIssueIDs))
	for _, id := range s.SelectedIssueIDs {
		selectedSet[id] = true
	}
	var batch []IssueRow
	for _, row := range open {
		if selectedSet[row.ID] {
			batch = append(batch, row)
		}
	}
	s.SelectedIssueIDs = nil
	if len(batch) == 0 {
		return
	}
	if len(batch) == 1 {
		s.dispatchSingleIssue(batch[0])
		return
	}
	var paths []string
	var targets []FindingTarget
	for _, row := range batch {
		targets = append(targets, row.Target)
		if row.Path != "" && !containsString(paths, row.Path) && len(paths) < 7 {
			paths = append(paths, row.Path)
		}
	}
	seed := &ChangeSeed{
		FindingTargets: targets,
		Title:          fmt.Sprintf("Batch fix (%d issues)", len(batch)),
		Paths:          paths,
		Kind:           "fix",
		Message:        fmt.Sprintf("Address %d selected findings across %s", len(batch), strings.Join(paths, ", ")),
	}
	s.CmdOutput = fmt.Sprintf("Dispatched grouped agent fix for %d issues", len(batch))
	s.enqueueAction(ActionRequest{Kind: "start-agent", Seed: seed})
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func (s *State) dispatchFeatureIdea(idea FeatureSuggestion) {
	seed := &ChangeSeed{
		Title:              idea.Title,
		Paths:              idea.Paths,
		Kind:               "feature",
		Message:            fmt.Sprintf("Implement feature: %s\nBenefit: %s", idea.Title, idea.Benefit),
		AcceptanceCriteria: idea.AcceptanceCriteria,
	}
	s.CmdOutput = fmt.Sprintf("Dispatched feature idea %q to isolated Agent", idea.Title)
	s.enqueueAction(ActionRequest{Kind: "start-agent", Target: idea.ID, Seed: seed})
}

func (s *State) toggleCurrentFileExclusion() {
	if s.Index == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx >= len(s.Index.Files) {
		return
	}
	path := s.Index.Files[idx].Path
	for i, p := range s.ExcludedPaths {
		if p == path {
			s.ExcludedPaths = append(s.ExcludedPaths[:i], s.ExcludedPaths[i+1:]...)
			s.CmdOutput = fmt.Sprintf("Included %s in analysis scope", path)
			return
		}
	}
	s.ExcludedPaths = append(s.ExcludedPaths, path)
	s.CmdOutput = fmt.Sprintf("Excluded %s in analysis scope", path)
}

func (s *State) applyAndCommitProposal(commitMsg string) {
	if s.Change == nil || s.Change.State == "applied" {
		return
	}
	passed, total := countPassedChecks(s.Change.Checks)
	if total == 0 || passed < total {
		s.CmdOutput = "Cannot apply proposal before required verification checks pass."
		return
	}
	s.CmdOutput = "Applying and committing verified proposal..."
	s.enqueueAction(ActionRequest{Kind: "apply-commit", Target: s.Change.ID, Message: commitMsg})
}

func (s *State) requestGuardedUndo() {
	if s.ChangeReceipt == nil || !s.ChangeReceipt.UndoAvailable {
		return
	}
	s.CmdOutput = "Requesting guarded Undo..."
	s.enqueueAction(ActionRequest{Kind: "undo", Target: s.ChangeReceipt.SessionID})
}

func (s *State) mergeSelectedWorktree() {
	if s.GitWorkspace == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx >= len(s.GitWorkspace.Worktrees) {
		return
	}
	wt := s.GitWorkspace.Worktrees[idx]
	if wt.Current || !wt.Mergeable {
		return
	}
	s.CmdOutput = fmt.Sprintf("Merging %s into %s...", wt.Path, firstNonEmpty(s.GitWorkspace.Branch, "main"))
	s.enqueueAction(ActionRequest{Kind: "merge-worktree", Target: wt.Path})
}

func (s *State) triggerAgentRecheckOrRepair() {
	if s.Change == nil {
		return
	}
	passed, total := countPassedChecks(s.Change.Checks)
	if total > 0 && passed < total {
		s.CmdOutput = "Triggering agent self-repair..."
		s.enqueueAction(ActionRequest{Kind: "repair-change", Target: s.Change.ID})
		return
	}
	s.CmdOutput = "Re-running deterministic verification checks..."
	s.enqueueAction(ActionRequest{Kind: "check-change", Target: s.Change.ID})
}

func (s *State) rejectCurrentDiffHunk() {
	if s.Change == nil || len(s.Change.Changes) == 0 {
		return
	}
	fc := &s.Change.Changes[0]
	hunks := SplitDiffIntoHunks(fc.Diff.Lines)
	if len(hunks) == 0 {
		return
	}
	idx := min(max(0, s.HunkIdx), len(hunks)-1)
	fc.Content = RejectHunkFromDiff(fc.Diff.Lines, hunks[idx])
	s.CmdOutput = fmt.Sprintf("Rejected hunk #%d in %s", idx+1, fc.Path)
}

func (s *State) toggleWorkspaceTrust() {
	s.WorkspaceTrust = !s.WorkspaceTrust
	word := "revoked"
	if s.WorkspaceTrust {
		word = "granted"
	}
	s.CmdOutput = fmt.Sprintf("Workspace trust %s", word)
	s.enqueueAction(ActionRequest{Kind: "set-trust", Target: word})
}

func (s *State) graduateSelectedLesson() {
	lessons := s.AllHarnessLessons()
	idx := min(max(0, s.ClampedIdx()), len(lessons)-1)
	rule := lessons[idx]
	base := s.InstructionDraft
	if base == "" && s.Instructions != nil {
		base = s.Instructions.ExistingContent
	}
	s.InstructionDraft = InsertGuidelineSection(base, "Harness² Lessons & Guardrails", rule)
	if !containsString(s.GraduatedLessons, rule) {
		s.GraduatedLessons = append(s.GraduatedLessons, rule)
	}
	s.CmdOutput = fmt.Sprintf("Graduated Harness² lesson into AGENTS.md: %s", rule)
}

func (s *State) triggerOrResumeAnalysis() {
	if s.Run != nil && (s.Run.Status == "paused" || s.Run.Status == "interrupted") {
		s.CmdOutput = "Resuming project analysis..."
		s.enqueueAction(ActionRequest{Kind: "resume-analysis"})
		return
	}
	s.CmdOutput = "Starting project analysis..."
	s.enqueueAction(ActionRequest{Kind: "run-analysis"})
}

// ActivateCurrentRow performs the primary Enter action for the currently highlighted row.
func (s *State) ActivateCurrentRow() {
	switch s.Page {
	case PageSummary, PageBugs:
		s.activateIssueRow()
	case PageProject:
		s.CmdOutput = "Running project verification checks..."
		s.enqueueAction(ActionRequest{Kind: "scan"})
	case PageEditor:
		s.activateEditorFileRow()
	case PageFeatures:
		features := s.activeFeatures()
		if idx := s.ClampedIdx(); idx < len(features) {
			s.dispatchFeatureIdea(features[idx])
		}
	case PageAgents:
		s.activateAgentRow()
	case PageAnalysisFiles:
		s.toggleCurrentFileExclusion()
	default:
		s.activateSecondaryRow()
	}
}

func (s *State) activateIssueRow() {
	issue := s.SelectedIssue()
	if issue == nil {
		return
	}
	if issue.AgentPath != "" {
		s.Navigate(PageAgents)
		s.AgentDetailOpen = true
		s.CmdOutput = fmt.Sprintf("Opened agent for %s", issue.Name)
		return
	}
	if issue.Status != "done" {
		s.dispatchSingleIssue(*issue)
		return
	}
	s.openFileInEditor(issue.Path)
}

func (s *State) activateEditorFileRow() {
	if s.Index == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx < len(s.Index.Files) {
		s.openFileInEditor(s.Index.Files[idx].Path)
	}
}

func (s *State) activateAgentRow() {
	items := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	idx := s.ClampedIdx()
	if idx < len(items) {
		s.AgentDetailOpen = true
		s.CmdOutput = fmt.Sprintf("Opened agent %s (%s)", items[idx].Title, items[idx].Branch)
	}
}

func (s *State) activateSecondaryRow() {
	switch s.Page {
	case PageAnalysisRun:
		s.activateAnalysisRunRow()
	case PageHistory:
		if idx := s.ClampedIdx(); idx < len(s.ChangeHistory) {
			entry := s.ChangeHistory[idx]
			s.CmdOutput = fmt.Sprintf("Loaded proposal %s", entry.Title)
			s.Navigate(PageEditor)
		}
	case PageInstructions:
		s.activateInstructionsRow()
	}
}

func (s *State) activateAnalysisRunRow() {
	if s.Run == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx < len(s.Run.Sections) {
		s.IssueCategory = s.Run.Sections[idx].Category
		s.Navigate(PageBugs)
	} else if s.Run.Features != nil {
		s.Navigate(PageFeatures)
	}
}

func (s *State) activateInstructionsRow() {
	switch s.RulesSection {
	case "models":
		presets := DefaultModelPresets()
		if idx := s.ClampedIdx(); idx < len(presets) {
			s.ActivePresetID = presets[idx].ID
			s.CmdOutput = fmt.Sprintf("Applied model preset: %s", presets[idx].Name)
		}
	case "bootstrap":
		pillars := EvaluateBootstrapChecklist(s.Index, s.Instructions)
		if idx := s.ClampedIdx(); idx < len(pillars) && !pillars[idx].Ready {
			s.CmdOutput = fmt.Sprintf("Bootstrapping %s...", pillars[idx].Title)
			s.enqueueAction(ActionRequest{Kind: "bootstrap", Target: pillars[idx].ID})
		}
	case "lessons":
		s.graduateSelectedLesson()
	case "trust":
		s.toggleWorkspaceTrust()
	}
}

func (s *State) openFileInEditor(path string) {
	if path == "" {
		return
	}
	s.File = &FileInfo{Path: path, Name: path}
	s.Navigate(PageEditor)
	s.CmdOutput = fmt.Sprintf("Opened %s", path)
	s.enqueueAction(ActionRequest{Kind: "open-file", Target: path})
}

// ExecuteCommand parses and executes a TUI colon command (e.g., ":analyze", ":bugs", ":q").
func (s *State) ExecuteCommand(raw string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), ":"))
	if trimmed == "" {
		return
	}
	s.recordCommandHistory(raw)
	fields := strings.Fields(trimmed)
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))

	if s.execSystemCommand(cmd, arg) || s.execAnalysisCommand(cmd, arg) || s.execNavigationCommand(cmd, arg) || s.execMutationCommand(cmd, arg) {
		return
	}
	s.CmdOutput = fmt.Sprintf("Unknown command %q. Type \":keys\" or \"help\" for available commands.", trimmed)
}

func (s *State) recordCommandHistory(raw string) {
	entry := strings.TrimSpace(raw)
	var next []string
	next = append(next, entry)
	for _, item := range s.CmdHistory {
		if item != entry && len(next) < 30 {
			next = append(next, item)
		}
	}
	s.CmdHistory = next
}

func (s *State) execSystemCommand(cmd, arg string) bool {
	switch cmd {
	case "q", "quit", "exit", "gui":
		s.Quitting = true
		return true
	case "help", "?", "keys", "keybinds", "keybind", "shortcuts":
		s.ShowKeybindsModal = true
		s.CmdOutput = "Opened Keybindings Reference ([?] or Esc to close). Commands: keys | analyze | test | fix | bootstrap | dismiss | agent <prompt> | open <file> | project <path> | scope | proposals | apply | merge | undo | palette | quit"
		return true
	case "palette", "theme":
		s.CyclePalette()
		return true
	case "trust":
		s.toggleWorkspaceTrust()
		return true
	case "bootstrap":
		s.CmdOutput = "Bootstrapping missing repository workflow pillars..."
		s.enqueueAction(ActionRequest{Kind: "bootstrap"})
		return true
	case "reconnect", "connect":
		s.CmdOutput = "Reconnecting to local daemon..."
		s.enqueueAction(ActionRequest{Kind: "reconnect"})
		return true
	case "project":
		return s.execProjectCommand(arg)
	}
	return false
}

func (s *State) execProjectCommand(arg string) bool {
	if arg != "" {
		s.CmdOutput = fmt.Sprintf("Opening project at %s...", arg)
		s.enqueueAction(ActionRequest{Kind: "open-project", Target: arg})
		return true
	}
	s.Navigate(PageProject)
	s.CmdOutput = "Switched to [1] Overview · Project & Tests."
	return true
}

func (s *State) execAnalysisCommand(cmd, arg string) bool {
	switch cmd {
	case "reindex":
		s.CmdOutput = "Reindexing repository..."
		s.enqueueAction(ActionRequest{Kind: "reindex"})
		return true
	case "analyze", "run":
		s.CmdOutput = "Starting project analysis..."
		s.enqueueAction(ActionRequest{Kind: "run-analysis"})
		return true
	case "new-analysis", "reanalyze":
		s.CmdOutput = "Starting fresh project analysis (preserving in-progress issues)..."
		s.enqueueAction(ActionRequest{Kind: "reanalyze"})
		return true
	case "resume":
		s.CmdOutput = "Resuming paused analysis..."
		s.enqueueAction(ActionRequest{Kind: "resume-analysis"})
		return true
	case "repair", "retry":
		s.CmdOutput = "Retrying failed/stale analysis files..."
		s.enqueueAction(ActionRequest{Kind: "repair-analysis", Target: arg})
		return true
	case "pause":
		s.CmdOutput = "Pausing active analysis..."
		s.enqueueAction(ActionRequest{Kind: "pause-analysis"})
		return true
	case "test", "scan":
		s.CmdOutput = "Running project tests and checks..."
		s.enqueueAction(ActionRequest{Kind: "scan"})
		return true
	}
	return false
}

func (s *State) execNavigationCommand(cmd, arg string) bool {
	switch cmd {
	case "scope", "files":
		s.Navigate(PageAnalysisFiles)
		s.CmdOutput = "Switched to [2] Agent Studio · File Scope."
		return true
	case "proposals", "history":
		s.Navigate(PageHistory)
		s.CmdOutput = "Switched to [3] Code & Diff · Saved Proposals."
		return true
	case "issues", "all", "bugs", "security", "performance", "perf":
		s.navigateToIssuesCategory(cmd)
		return true
	case "features":
		s.Navigate(PageFeatures)
		s.CmdOutput = "Switched to [2] Agent Studio · Features."
		return true
	case "arch", "diagrams":
		s.Navigate(PageDiagrams)
		s.CmdOutput = "Switched to [4] Handbook & Arch · Architecture."
		return true
	case "docs", "handbook":
		s.Navigate(PageDocs)
		s.CmdOutput = "Switched to [4] Handbook & Arch · Handbook / Docs."
		return true
	case "git":
		s.Navigate(PageGit)
		s.CmdOutput = "Switched to [3] Code & Diff · Git & Worktrees."
		return true
	case "open":
		if arg != "" {
			s.openFileInEditor(arg)
			return true
		}
	}
	return s.execHubMatchCommand(cmd)
}

func (s *State) navigateToIssuesCategory(cmd string) {
	s.Navigate(PageBugs)
	if cmd == "issues" {
		s.CmdOutput = "Switched to [2] Agent Studio · Issues."
		return
	}
	cat := cmd
	if cmd == "perf" {
		cat = "performance"
	}
	s.IssueCategory = cat
	s.SelectedIdx = 0
	s.CmdOutput = fmt.Sprintf("Switched to [2] Agent Studio · Issues (%s).", strings.ToUpper(cat))
}

func (s *State) execHubMatchCommand(cmd string) bool {
	for _, tab := range HubTabs() {
		if tab.Key == cmd || string(tab.Page) == cmd || strings.Contains(strings.ToLower(tab.Label), cmd) {
			s.Navigate(tab.Page)
			s.CmdOutput = fmt.Sprintf("Switched to [%s] %s.", tab.Key, tab.Label)
			return true
		}
	}
	return false
}

func (s *State) execMutationCommand(cmd, arg string) bool {
	switch cmd {
	case "dismiss":
		s.dismissSelectedIssue()
		return true
	case "fix":
		s.dispatchSelectedIssues()
		return true
	case "agent", "agents":
		s.Navigate(PageAgents)
		if arg != "" {
			seed := &ChangeSeed{Title: "Clean-code agent mission", Kind: "fix", Message: arg}
			s.enqueueAction(ActionRequest{Kind: "start-agent", Message: arg, Seed: seed})
			s.CmdOutput = fmt.Sprintf("Dispatched custom agent mission: %s", arg)
		} else {
			s.CmdOutput = "Switched to [2] Agent Studio."
		}
		return true
	case "apply", "commit":
		s.applyAndCommitProposal(arg)
		return true
	case "merge":
		s.mergeSelectedWorktree()
		return true
	case "undo":
		s.requestGuardedUndo()
		return true
	}
	return false
}
