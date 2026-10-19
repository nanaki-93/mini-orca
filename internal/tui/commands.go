package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/tuidomain"
)

func (s *State) enqueueAction(act ActionRequest) {
	s.Error = ""
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
	s.startLocalAgentPreview(seed)
	s.Navigate(PageAgents)
	s.CmdOutput = fmt.Sprintf("Dispatched Fix for %s (%s) — Agent Pipeline active", issue.Name, issue.Path)
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
		if row.Path != "" && !containsString(paths, row.Path) && len(paths) < 7 {
			paths = append(paths, row.Path)
		}
		if row.Path != "" && containsString(paths, row.Path) {
			targets = append(targets, row.Target)
		}
	}
	seed := &ChangeSeed{
		FindingTargets: targets,
		Title:          fmt.Sprintf("Batch fix (%d issues)", len(batch)),
		Paths:          paths,
		Kind:           "fix",
		Message:        fmt.Sprintf("Address %d selected findings across %s", len(batch), strings.Join(paths, ", ")),
	}
	s.startLocalAgentPreview(seed)
	s.Navigate(PageAgents)
	s.CmdOutput = fmt.Sprintf("Dispatched grouped agent fix for %d issues", len(batch))
	s.enqueueAction(ActionRequest{Kind: "start-agent", Seed: seed})
}

func (s *State) startLocalAgentPreview(seed *ChangeSeed) {
	if seed == nil {
		return
	}
	var targets []ChangeTarget
	for _, p := range seed.Paths {
		if strings.TrimSpace(p) != "" {
			targets = append(targets, ChangeTarget{Path: p, Exists: true})
		}
	}
	s.Change = &ChangeSession{
		ID:                 "agent-session",
		Kind:               firstNonEmpty(seed.Kind, "fix"),
		Title:              firstNonEmpty(seed.Title, "Agent Mission"),
		State:              "running",
		FindingTargets:     seed.FindingTargets,
		Targets:            targets,
		AcceptanceCriteria: seed.AcceptanceCriteria,
		Workflow: &ChangeWorkflow{
			ID:     "wf-active",
			Status: "running",
			Stages: []ChangeWorkflowStage{
				{Name: "Architect & Code", Status: "running"},
				{Name: "Verify & Test", Status: "queued"},
				{Name: "Critic Review", Status: "queued"},
				{Name: "Human Review", Status: "queued"},
			},
		},
	}
}

func containsString(list []string, target string) bool {
	return tuidomain.ContainsString(list, target)
}

func (s *State) dispatchFeatureIdea(idea FeatureSuggestion) {
	seed := &ChangeSeed{
		Title:              idea.Title,
		Paths:              idea.Paths,
		Kind:               "feature",
		Message:            fmt.Sprintf("Implement feature: %s\nBenefit: %s", idea.Title, idea.Benefit),
		AcceptanceCriteria: idea.AcceptanceCriteria,
	}
	s.startLocalAgentPreview(seed)
	s.Navigate(PageAgents)
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
			s.enqueueAction(ActionRequest{Kind: "save-selection", Target: path})
			return
		}
	}
	s.ExcludedPaths = append(s.ExcludedPaths, path)
	s.CmdOutput = fmt.Sprintf("Excluded %s in analysis scope", path)
	s.enqueueAction(ActionRequest{Kind: "save-selection", Target: path})
}

func (s *State) applyAndCommitProposal(commitMsg string) {
	if s.Change == nil {
		s.CmdOutput = "No active proposal diff to apply."
		return
	}
	if s.Change.State == "applied" {
		s.CmdOutput = fmt.Sprintf("Proposal %q is already applied and committed (press [u] for Undo).", s.Change.Title)
		return
	}
	if s.hasRunningWorkflow() {
		s.CmdOutput = "Wait for the active agent workflow to finish verification before applying."
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
		s.CmdOutput = "No applied proposal receipt available for guarded Undo."
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
	if s.hasRunningWorkflow() {
		s.CmdOutput = "Refreshing active agent workflow status..."
		s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
		return
	}
	if s.isChangeStale() || (s.Change.Workflow != nil && s.Change.Workflow.Status == "failed" && len(EffectiveSessionChanges(s.Change)) == 0) {
		s.restartAgentFromCurrentChange()
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

func (s *State) isChangeStale() bool {
	if s.Change == nil {
		return false
	}
	if s.Change.Freshness == "stale" {
		return true
	}
	if s.Change.Workflow != nil && s.Change.Workflow.Status == "stale" {
		return true
	}
	if s.Project != nil && s.Change.ProjectRevision != "" && s.Project.ProjectRevision != "" && s.Change.ProjectRevision != s.Project.ProjectRevision {
		return true
	}
	return false
}

func (s *State) restartAgentFromCurrentChange() {
	seed := s.buildSeedFromCurrentChange()
	if seed == nil {
		return
	}
	s.startLocalAgentPreview(seed)
	s.CmdOutput = fmt.Sprintf("Refreshing stale proposal %q on current project revision...", seed.Title)
	s.enqueueAction(ActionRequest{Kind: "start-agent", Seed: seed})
}

func (s *State) buildSeedFromCurrentChange() *ChangeSeed {
	if s.Change == nil {
		return nil
	}
	var paths []string
	for _, t := range s.Change.Targets {
		if t.Path != "" && !containsString(paths, t.Path) {
			paths = append(paths, t.Path)
		}
	}
	for _, c := range s.Change.Changes {
		if c.Path != "" && !containsString(paths, c.Path) {
			paths = append(paths, c.Path)
		}
	}
	for _, ft := range s.Change.FindingTargets {
		if ft.Path != "" && !containsString(paths, ft.Path) {
			paths = append(paths, ft.Path)
		}
	}
	title := firstNonEmpty(s.Change.Title, "Agent Mission")
	return &ChangeSeed{
		FindingTargets:     s.Change.FindingTargets,
		Title:              title,
		Paths:              paths,
		Kind:               firstNonEmpty(s.Change.Kind, "fix"),
		Message:            fmt.Sprintf("Refresh and complete proposal: %s", title),
		AcceptanceCriteria: s.Change.AcceptanceCriteria,
	}
}

func (s *State) rejectCurrentDiffHunk() {
	active, _, ok := s.activeDiffHunk()
	if !ok || active.FileIndex < 0 || active.FileIndex >= len(s.Change.Changes) {
		s.CmdOutput = "No modifiable diff hunk selected to reject."
		return
	}
	fc := &s.Change.Changes[active.FileIndex]
	fc.Content = RejectHunkFromDiff(fc.Diff.Lines, active.Hunk)
	fc.Diff.Lines = RejectHunkLines(fc.Diff.Lines, active.Hunk)
	s.Change.ReviewedHash = ""
	s.Change.Checks = nil
	if _, all, hasNext := s.activeDiffHunk(); hasNext {
		s.syncDiffSelectionToHunk(min(active.GlobalIndex, len(all)-1))
	} else {
		s.HunkIdx = 0
		s.SelectedIdx = 0
	}
	msg := fmt.Sprintf("Rejected hunk #%d in %s", active.GlobalIndex+1, fc.Path)
	s.CmdOutput = msg
	if n := len(s.PendingActions); n > 0 && s.PendingActions[n-1].Kind == "update-proposal" {
		s.PendingActions[n-1].Message = msg
		return
	}
	s.enqueueAction(ActionRequest{Kind: "update-proposal", Target: s.Change.ID, Message: msg})
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

// IsLessonGraduated reports whether a Harness² lesson has been graduated or is already present in AGENTS.md.
func (s *State) IsLessonGraduated(rule string) bool {
	if containsString(s.GraduatedLessons, rule) {
		return true
	}
	if s.InstructionDraft != "" && ContainsGuideline(s.InstructionDraft, rule) {
		return true
	}
	if s.Instructions != nil && ContainsGuideline(s.Instructions.ExistingContent, rule) {
		return true
	}
	return false
}

func (s *State) graduateSelectedLesson() {
	lessons := s.AllHarnessLessons()
	if len(lessons) == 0 {
		return
	}
	idx := min(max(0, s.ClampedIdx()), len(lessons)-1)
	rule := lessons[idx]
	if s.IsLessonGraduated(rule) {
		s.CmdOutput = fmt.Sprintf("Harness² lesson already graduated into AGENTS.md: %s", rule)
		return
	}
	base := s.InstructionDraft
	if base == "" && s.Instructions != nil {
		base = s.Instructions.ExistingContent
	}
	s.InstructionDraft = InsertGuidelineSection(base, "Harness² Lessons & Guardrails", rule)
	if !containsString(s.GraduatedLessons, rule) {
		s.GraduatedLessons = append(s.GraduatedLessons, rule)
	}
	s.CmdOutput = fmt.Sprintf("Graduated Harness² lesson into AGENTS.md: %s", rule)
	s.enqueueAction(ActionRequest{Kind: "propose-instructions", Target: "AGENTS.md", Message: s.InstructionDraft})
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
	case PageSummary, PageBugs, PageIssueDetail:
		s.activateIssueRow()
	case PageRepo:
		s.CmdOutput = "Reindexing repository..."
		s.enqueueAction(ActionRequest{Kind: "reindex"})
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
		if s.Page == PageSummary {
			s.Navigate(PageBugs)
			s.CmdOutput = "Switched to [2] Agent Studio · Issues"
		}
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
	if s.isViewingDiff() {
		s.applyAndCommitProposal("")
		return
	}
	if s.Index == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx < len(s.Index.Files) {
		s.openFileInEditor(s.Index.Files[idx].Path)
	}
}

func (s *State) openAgentDiffInspector() {
	s.File = nil
	s.BrowseFiles = false
	if s.Change != nil && len(EffectiveSessionChanges(s.Change)) > 0 {
		s.Navigate(PageEditor)
		s.syncDiffSelectionToHunk(0)
		s.CmdOutput = fmt.Sprintf("Inspecting diff for %s", s.Change.Title)
		return
	}
	if s.Change != nil && s.Change.ID != "" && s.Change.ID != "agent-session" {
		s.Navigate(PageEditor)
		s.CmdOutput = fmt.Sprintf("Loading diff for %s...", s.Change.Title)
		s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
		return
	}
	if s.Change != nil && len(s.Change.Targets) > 0 && s.Change.Targets[0].Path != "" {
		s.openFileInEditor(s.Change.Targets[0].Path)
		return
	}
	s.Navigate(PageEditor)
	s.CmdOutput = "Switched to [3] Code & Diff · Inspector"
}

func (s *State) cancelActiveWorkflow() {
	if s.Change == nil || !s.hasRunningWorkflow() {
		s.CmdOutput = "No running agent workflow to cancel."
		return
	}
	s.CmdOutput = fmt.Sprintf("Canceling active workflow for %s...", s.Change.Title)
	s.enqueueAction(ActionRequest{Kind: "cancel-workflow", Target: s.Change.ID})
}

func (s *State) activateAgentRow() {
	items := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	idx := s.ClampedIdx()
	if idx >= len(items) {
		return
	}
	if s.Change != nil && len(EffectiveSessionChanges(s.Change)) > 0 {
		s.openAgentDiffInspector()
		return
	}
	s.AgentDetailOpen = true
	if s.Change != nil && s.Change.ID != "" && s.Change.ID != "agent-session" {
		s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
	}
	s.CmdOutput = fmt.Sprintf("Inspecting agent %s (%s)", items[idx].Title, items[idx].Branch)
}

func (s *State) activateSecondaryRow() {
	switch s.Page {
	case PageAnalysisRun:
		s.activateAnalysisRunRow()
	case PageHistory:
		if idx := s.ClampedIdx(); idx < len(s.ChangeHistory) {
			entry := s.ChangeHistory[idx]
			s.File = nil
			s.BrowseFiles = false
			s.CmdOutput = fmt.Sprintf("Loaded proposal %s", entry.Title)
			s.Navigate(PageEditor)
			s.enqueueAction(ActionRequest{Kind: "load-change", Target: entry.ID})
		}
	case PageInstructions, PageModels, PageBootstrap, PageLessons:
		s.activateInstructionsRow()
	}
}

func (s *State) activateAnalysisRunRow() {
	if s.Run == nil || s.Run.Status == "paused" || s.Run.Status == "interrupted" {
		s.triggerOrResumeAnalysis()
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
	switch s.EffectivePage() {
	case PageModels:
		scopes := CanonicalModelScopes()
		presets := DefaultModelPresets()
		idx := s.ClampedIdx()
		if idx < len(scopes) {
			s.cycleSelectedScopeModel(true)
			return
		}
		if pIdx := idx - len(scopes); pIdx >= 0 && pIdx < len(presets) {
			s.applyModelPreset(presets[pIdx])
		}
	case PageBootstrap:
		pillars := EvaluateBootstrapChecklist(s.Index, s.Instructions)
		if idx := s.ClampedIdx(); idx < len(pillars) && !pillars[idx].Ready {
			s.CmdOutput = fmt.Sprintf("Bootstrapping %s...", pillars[idx].Title)
			s.enqueueAction(ActionRequest{Kind: "bootstrap", Target: pillars[idx].ID})
		}
	case PageLessons:
		s.graduateSelectedLesson()
	default:
		s.toggleWorkspaceTrust()
	}
}

func (s *State) selectedModelScope() string {
	scopes := CanonicalModelScopes()
	idx := s.ClampedIdx()
	if idx >= 0 && idx < len(scopes) {
		return scopes[idx]
	}
	return "analyze"
}

func (s *State) cycleSelectedScopeModel(forward bool) {
	scope := s.selectedModelScope()
	choices := s.AvailableModelIDs()
	if len(choices) == 0 {
		return
	}
	cur := s.AssignedScopeModelID(scope)
	pos := 0
	for i, c := range choices {
		if c == cur {
			pos = i
			break
		}
	}
	if forward {
		pos = (pos + 1) % len(choices)
	} else {
		pos = (pos - 1 + len(choices)) % len(choices)
	}
	s.setScopeModel(scope, choices[pos])
}

func (s *State) setScopeModel(scope, modelID string) {
	if s.ScopeModels == nil {
		s.ScopeModels = make(map[string]string)
	}
	s.ScopeModels[scope] = modelID
	eff := s.EffectiveScopeModel(scope)
	s.CmdOutput = fmt.Sprintf("Assigned scope [%s] -> %s (%s · %s)",
		strings.ToUpper(scope), firstNonEmpty(eff.Model, modelID), eff.ProviderOrigin, FormatModelParams(eff))
}

func (s *State) resetSelectedScopeModel() {
	scope := s.selectedModelScope()
	if s.ScopeModels == nil {
		s.ScopeModels = make(map[string]string)
	}
	s.ScopeModels[scope] = scope
	s.CmdOutput = fmt.Sprintf("Reset scope [%s] routing to default profile (%s)", strings.ToUpper(scope), scope)
}

func (s *State) cycleModelPreset() {
	presets := DefaultModelPresets()
	if len(presets) == 0 {
		return
	}
	pos := 0
	for i, p := range presets {
		if p.ID == s.ActivePresetID {
			pos = (i + 1) % len(presets)
			break
		}
	}
	s.applyModelPreset(presets[pos])
}

func (s *State) applyModelPreset(p ModelPreset) {
	s.ActivePresetID = p.ID
	if s.ScopeModels == nil {
		s.ScopeModels = make(map[string]string)
	}
	s.ScopeModels["analyze"] = p.Analyze
	s.ScopeModels["bug"] = p.Bug
	s.ScopeModels["function"] = p.Function
	s.CmdOutput = fmt.Sprintf("Applied model preset: %s (analyze:%s · bug:%s · function:%s)",
		p.Name, p.Analyze, p.Bug, p.Function)
}

func (s *State) requestRefreshModels() {
	s.CmdOutput = "Refreshing configured & discovered LLM models..."
	s.enqueueAction(ActionRequest{Kind: "refresh-models"})
}

func (s *State) openFileInEditor(path string) {
	if path == "" {
		return
	}
	s.BrowseFiles = false
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
	s.Error = ""
	s.recordCommandHistory(raw)
	fields := strings.Fields(trimmed)
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))

	if s.execSystemCommand(cmd, arg) || s.execAnalysisCommand(cmd, arg) || s.execMutationCommand(cmd, arg) || s.execNavigationCommand(cmd, arg) {
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
		s.CmdOutput = "Opened Keybindings Reference ([?] or Esc to close). Commands: keys | analyze | resume | reanalyze | test | fix | debug | agent <prompt> | open <file> | project <path> | models | apply | undo | quit"
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
		s.triggerOrResumeAnalysis()
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
	if s.execIssueFilterCommand(cmd, arg) || s.execModelCommand(cmd, arg) {
		return true
	}
	switch cmd {
	case "debug", "logs":
		s.Navigate(PageAgents)
		s.AgentDetailOpen = true
		s.CmdOutput = "Opened Agent Workflow Debug & Check Inspector ([Esc/g] to return)."
		return true
	case "scope", "files":
		s.Navigate(PageAnalysisFiles)
		s.CmdOutput = "Switched to [2] Agent Studio · File Scope."
		return true
	case "proposals", "history":
		s.Navigate(PageHistory)
		s.CmdOutput = "Switched to [3] Code & Diff · Saved Proposals."
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
		s.setIssueStatusFilter("open")
		return true
	}
	return s.execHubMatchCommand(cmd)
}

func (s *State) execIssueFilterCommand(cmd, arg string) bool {
	switch cmd {
	case "issues", "all", "bugs", "security", "performance", "perf":
		s.navigateToIssuesCategory(cmd)
		return true
	case "in-progress", "in_progress", "wip":
		s.setIssueStatusFilter("in_progress")
		return true
	case "done", "resolved", "fixed":
		s.setIssueStatusFilter("done")
		return true
	case "status":
		return s.applyStatusFilterArg(arg)
	case "category", "cat":
		return s.applyCategoryFilterArg(arg)
	case "filter", "filters":
		return s.applyCombinedFilterArg(arg)
	}
	return false
}

func (s *State) setIssueStatusFilter(view string) {
	s.Navigate(PageBugs)
	s.IssueView = view
	s.SelectedIdx = 0
	s.CmdOutput = fmt.Sprintf("Issue status filter: %s (Category: %s)",
		strings.ToUpper(s.IssueView), strings.ToUpper(s.IssueCategory))
}

func (s *State) applyStatusFilterArg(arg string) bool {
	s.Navigate(PageBugs)
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.cycleIssueStatusView(true)
	case "open":
		s.setIssueStatusFilter("open")
	case "in_progress", "in-progress", "progress", "wip":
		s.setIssueStatusFilter("in_progress")
	case "done", "fixed", "resolved", "closed":
		s.setIssueStatusFilter("done")
	case "all":
		s.setIssueStatusFilter("all")
	default:
		s.CmdOutput = fmt.Sprintf("Unknown status filter %q (use: open, in_progress, done, all)", arg)
	}
	return true
}

func (s *State) applyCategoryFilterArg(arg string) bool {
	s.Navigate(PageBugs)
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.cycleIssueCategory(true)
	case "all":
		s.navigateToIssuesCategory("all")
	case "bugs", "bug":
		s.navigateToIssuesCategory("bugs")
	case "security", "sec":
		s.navigateToIssuesCategory("security")
	case "performance", "perf":
		s.navigateToIssuesCategory("performance")
	default:
		s.CmdOutput = fmt.Sprintf("Unknown category filter %q (use: all, bugs, security, perf)", arg)
	}
	return true
}

func (s *State) applyCombinedFilterArg(arg string) bool {
	s.Navigate(PageBugs)
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.cycleIssueStatusView(true)
		return true
	case "open", "in_progress", "in-progress", "progress", "wip", "done", "fixed", "resolved", "closed":
		return s.applyStatusFilterArg(norm)
	case "all-status", "all_status":
		return s.applyStatusFilterArg("all")
	case "all", "bugs", "bug", "security", "sec", "performance", "perf":
		return s.applyCategoryFilterArg(norm)
	default:
		s.CmdOutput = fmt.Sprintf("Unknown filter %q (use: open, in_progress, done, all, bugs, security, perf)", arg)
		return true
	}
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

func (s *State) execModelCommand(cmd, arg string) bool {
	switch cmd {
	case "models":
		s.Navigate(PageModels)
		s.CmdOutput = "Switched to [5] Governance & Rules · Model Routing."
		return true
	case "model":
		s.Navigate(PageModels)
		s.execSetModelArg(arg)
		return true
	case "preset", "presets", "profile":
		s.Navigate(PageModels)
		s.execSetPresetArg(arg)
		return true
	}
	return false
}

func (s *State) execSetModelArg(arg string) {
	fields := strings.Fields(strings.TrimSpace(arg))
	if len(fields) == 0 {
		s.cycleSelectedScopeModel(true)
		return
	}
	scopes := CanonicalModelScopes()
	if len(fields) == 1 {
		target := strings.ToLower(fields[0])
		for idx, sc := range scopes {
			if sc == target {
				s.SelectedIdx = idx
				s.cycleSelectedScopeModel(true)
				return
			}
		}
		s.setScopeModel(s.selectedModelScope(), fields[0])
		return
	}
	scope := strings.ToLower(fields[0])
	if !containsString(scopes, scope) {
		s.CmdOutput = fmt.Sprintf("Unknown model scope %q (use: analyze, bug, function)", fields[0])
		return
	}
	for idx, sc := range scopes {
		if sc == scope {
			s.SelectedIdx = idx
			break
		}
	}
	s.setScopeModel(scope, fields[1])
}

func (s *State) execSetPresetArg(arg string) {
	norm := strings.ToLower(strings.TrimSpace(arg))
	if norm == "" {
		s.cycleModelPreset()
		return
	}
	for idx, p := range DefaultModelPresets() {
		if p.ID == norm || strings.Contains(strings.ToLower(p.ID), norm) || strings.Contains(strings.ToLower(p.Name), norm) {
			s.SelectedIdx = len(CanonicalModelScopes()) + idx
			s.applyModelPreset(p)
			return
		}
	}
	s.CmdOutput = fmt.Sprintf("Unknown model preset %q (use: local-default, hybrid-precision, deep-audit)", arg)
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
			seed := &ChangeSeed{Title: truncatePlain(arg, 48), Kind: "fix", Message: arg}
			s.startLocalAgentPreview(seed)
			s.enqueueAction(ActionRequest{Kind: "start-agent", Message: arg, Seed: seed})
			s.CmdOutput = fmt.Sprintf("Dispatched custom agent mission: %s", arg)
		} else {
			s.CmdOutput = "Switched to [2] Agent Studio."
		}
		return true
	case "accept", "apply", "commit":
		s.applyAndCommitProposal(arg)
		return true
	case "diff", "inspect":
		s.execDiffInspectorCommand(arg)
		return true
	case "hunk", "next-hunk", "prev-hunk":
		s.Navigate(PageEditor)
		s.stepDiffHunk(cmd != "prev-hunk" && !strings.EqualFold(strings.TrimSpace(arg), "prev"))
		return true
	case "cancel":
		s.cancelActiveWorkflow()
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

func (s *State) execDiffInspectorCommand(arg string) {
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.openAgentDiffInspector()
	case "hunk", "focused", "split", "side", "side-by-side", "unified", "all", "stream":
		s.Navigate(PageEditor)
		s.setDiffViewMode(norm)
	case "next":
		s.Navigate(PageEditor)
		s.stepDiffHunk(true)
	case "prev":
		s.Navigate(PageEditor)
		s.stepDiffHunk(false)
	default:
		s.openAgentDiffInspector()
	}
}
