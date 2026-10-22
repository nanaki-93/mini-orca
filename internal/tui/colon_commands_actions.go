package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
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
			if len(s.SelectedIssueIDs) == 0 {
				s.CmdOutput = fmt.Sprintf("Deselected %s — batch selection cleared", issue.Name)
			} else {
				s.CmdOutput = fmt.Sprintf("Selected %d issue(s) for batch fix — press [Enter] to run batch or [f] to fix current row", len(s.SelectedIssueIDs))
			}
			return
		}
	}
	s.SelectedIssueIDs = append(s.SelectedIssueIDs, issue.ID)
	s.CmdOutput = fmt.Sprintf("Selected %d issue(s) for batch fix — press [Enter] to run batch or [f] to fix current row", len(s.SelectedIssueIDs))
}

func (s *State) dismissSelectedIssue() {
	issue := s.SelectedIssue()
	if issue == nil {
		s.CmdOutput = "No issue selected to delete."
		return
	}
	for idx, id := range s.SelectedIssueIDs {
		if id == issue.ID {
			s.SelectedIssueIDs = append(s.SelectedIssueIDs[:idx], s.SelectedIssueIDs[idx+1:]...)
			break
		}
	}
	target := issue.Target
	s.markFindingDismissed(target)
	s.SelectedIdx = s.ClampedIdx()
	if s.Page == PageIssueDetail && len(s.FilteredIssues()) == 0 {
		s.Navigate(PageBugs)
	}
	s.CmdOutput = fmt.Sprintf("Deleted issue: %s", issue.Name)
	s.enqueueAction(ActionRequest{Kind: "dismiss-finding", Target: issue.ID, Issue: &target})
}

func (s *State) dispatchSelectedIssues() {
	if len(s.SelectedIssueIDs) > 0 {
		s.dispatchBatchIssues()
		return
	}
	s.dispatchSingleSelectedIssue()
}

func (s *State) dispatchSingleSelectedIssue() {
	issue := s.SelectedIssue()
	if issue == nil || issue.Status == "done" {
		return
	}
	for idx, id := range s.SelectedIssueIDs {
		if id == issue.ID {
			s.SelectedIssueIDs = append(s.SelectedIssueIDs[:idx], s.SelectedIssueIDs[idx+1:]...)
			break
		}
	}
	s.dispatchSingleIssue(*issue)
}

func (s *State) dispatchSingleIssue(issue IssueRow) {
	seed := buildSingleIssueSeed(issue)
	if len(seed.AcceptanceCriteria) == 0 {
		seed.AcceptanceCriteria = []string{domain.SynthesizeIssueVerification(issue, s.Index)}
	}
	s.openTaskPlan(seed)
}

func buildSingleIssueSeed(issue IssueRow) *ChangeSeed {
	kind := "fix"
	if issue.Category == "security" || issue.Category == "performance" {
		kind = issue.Category
	}
	return &ChangeSeed{
		Plan:               &ChangePlanInput{Problem: client.FirstNonEmpty(issue.Cause, issue.Title, issue.Name), Evidence: issue.Evidence, Approach: client.FirstNonEmpty(issue.Solution, "Investigate the reported behavior and implement a focused fix with regression coverage."), NonGoals: issue.NonGoals},
		FindingTargets:     []FindingTarget{issue.Target},
		Title:              issue.Name,
		Paths:              []string{issue.Path},
		Kind:               kind,
		Message:            fmt.Sprintf("Address this finding: %s\nLocation: %s:%d\nCause: %s\nProposed solution: %s", issue.Title, issue.Path, issue.Line, issue.Cause, issue.Solution),
		AcceptanceCriteria: issue.AcceptanceCriteria,
	}
}

func (s *State) dispatchBatchIssues() {
	all := s.CollectAllIssues()
	selectedSet := make(map[string]bool, len(s.SelectedIssueIDs))
	for _, id := range s.SelectedIssueIDs {
		selectedSet[id] = true
	}
	var batch []IssueRow
	for _, row := range all {
		if row.Status != "done" && selectedSet[row.ID] {
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
	seed := buildBatchIssuesSeed(batch)
	s.openTaskPlan(seed)
}

func buildBatchIssuesSeed(batch []IssueRow) *ChangeSeed {
	if len(batch) == 1 {
		return buildSingleIssueSeed(batch[0])
	}
	paths, included, targets := selectBatchIssueScope(batch)
	itemLines, criteria := formatBatchIssueItems(included)
	msg := fmt.Sprintf(
		"Address these %d findings across %s:\n\n%s\n\nExplain how each changed file addresses the findings. Preserve unrelated behavior.",
		len(included), strings.Join(paths, ", "), strings.Join(itemLines, "\n\n"),
	)
	return &ChangeSeed{
		FindingTargets:     targets,
		Title:              fmt.Sprintf("Batch fix (%d issues)", len(included)),
		Paths:              paths,
		Kind:               batchIssueKind(included),
		Message:            msg,
		AcceptanceCriteria: criteria,
	}
}

func selectBatchIssueScope(batch []IssueRow) ([]string, []IssueRow, []FindingTarget) {
	var paths []string
	var included []IssueRow
	var targets []FindingTarget
	for _, row := range batch {
		if row.Path != "" && !domain.ContainsString(paths, row.Path) {
			paths = append(paths, row.Path)
		}
		if row.Path != "" && domain.ContainsString(paths, row.Path) {
			included = append(included, row)
			targets = append(targets, row.Target)
		}
	}
	if len(included) == 0 {
		included = batch
	}
	return paths, included, targets
}

func formatBatchIssueItems(rows []IssueRow) ([]string, []string) {
	var itemLines []string
	var criteria []string
	for idx, row := range rows {
		loc := row.Path
		if row.Line > 0 {
			loc = fmt.Sprintf("%s:%d", row.Path, row.Line)
		}
		if row.Symbol != "" {
			loc += " · " + row.Symbol
		}
		itemLines = append(itemLines, fmt.Sprintf(
			"%d. %s (%s)\n   Cause: %s\n   Proposed solution: %s",
			idx+1, row.Title, loc, row.Cause, row.Solution,
		))
		for _, ac := range row.AcceptanceCriteria {
			trimmed := strings.TrimSpace(ac)
			if trimmed != "" && !domain.ContainsString(criteria, trimmed) {
				criteria = append(criteria, trimmed)
			}
		}
	}
	return itemLines, criteria
}

func batchIssueKind(rows []IssueRow) string {
	if len(rows) == 0 {
		return "fix"
	}
	first := rows[0].Category
	if first != "security" && first != "performance" {
		return "fix"
	}
	for _, r := range rows[1:] {
		if r.Category != first {
			return "fix"
		}
	}
	return first
}

func (s *State) startLocalAgentPreview(seed *ChangeSeed) string {
	if seed == nil {
		return ""
	}
	if s.Change != nil && s.Change.ID != "" {
		s.upsertActiveChange(s.Change)
	}
	s.nextPreviewSeq++
	previewID := fmt.Sprintf("agent-session-%d", s.nextPreviewSeq)
	var targets []ChangeTarget
	for _, p := range seed.Paths {
		if strings.TrimSpace(p) != "" {
			targets = append(targets, ChangeTarget{Path: p, Exists: true})
		}
	}
	preview := &ChangeSession{
		ID:                 previewID,
		Kind:               client.FirstNonEmpty(seed.Kind, "fix"),
		Title:              client.FirstNonEmpty(seed.Title, "Agent Mission"),
		State:              "running",
		WorktreePath:       seed.WorktreePath,
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
	s.Change = preview
	s.upsertActiveChange(preview)
	return previewID
}

func (s *State) dispatchFeatureIdea(idea FeatureSuggestion) {
	seed := &ChangeSeed{
		Plan:               &ChangePlanInput{Problem: idea.Title, Evidence: idea.Evidence, Approach: "Implement " + idea.Title + ". " + idea.Benefit},
		Title:              idea.Title,
		Paths:              idea.Paths,
		Kind:               "feature",
		Message:            fmt.Sprintf("Implement feature: %s\nBenefit: %s", idea.Title, idea.Benefit),
		AcceptanceCriteria: idea.AcceptanceCriteria,
	}
	s.openTaskPlan(seed)
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
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
	}
	if s.Change == nil {
		s.CmdOutput = "No active proposal diff to apply."
		return
	}
	if s.Change.Plan != nil && !allProposalFilesReviewed(s.Change) {
		s.openAgentDiffInspector()
		s.CmdOutput = "Inspect each changed file and press [w] to mark it reviewed before Apply."
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
	passed, total := domain.CountPassedChecks(s.Change.Checks)
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
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
		if s.Change != nil && s.Change.WorktreePath != "" {
			if s.Change.Commit == nil || s.Change.Commit.State != "committed" {
				s.applyAndCommitProposal("")
				return
			}
			_, mainBranch := s.mainGitIdentity()
			s.CmdOutput = fmt.Sprintf("Merging %s into %s...", s.Change.WorktreePath, client.FirstNonEmpty(mainBranch, "main"))
			s.enqueueAction(ActionRequest{Kind: "merge-worktree", Target: s.Change.WorktreePath})
			return
		}
	}
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
	s.CmdOutput = fmt.Sprintf("Merging %s into %s...", wt.Path, client.FirstNonEmpty(s.GitWorkspace.Branch, "main"))
	s.enqueueAction(ActionRequest{Kind: "merge-worktree", Target: wt.Path})
}

func (s *State) triggerAgentRecheckOrRepair() {
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
	}
	if s.Change == nil {
		return
	}
	if s.hasRunningWorkflow() {
		s.CmdOutput = "Refreshing active agent workflow status..."
		s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
		return
	}
	if s.Change.Plan != nil && len(s.Change.ScopeSuggestions) > 0 {
		s.openCurrentPlan()
		return
	}
	if s.isChangeStale() || len(s.Change.Changes) == 0 {
		s.restartAgentFromCurrentChange()
		return
	}
	passed, total := domain.CountPassedChecks(s.Change.Checks)
	if (total > 0 && passed < total) || s.isWorkflowFailedOrInterrupted() {
		s.CmdOutput = "Triggering agent self-repair..."
		s.enqueueAction(ActionRequest{Kind: "repair-change", Target: s.Change.ID})
		return
	}
	if s.Change.Workflow != nil && s.Change.Workflow.Status == "changes_requested" {
		s.restartAgentFromCurrentChange()
		return
	}
	s.CmdOutput = "Re-running deterministic verification checks..."
	s.enqueueAction(ActionRequest{Kind: "check-change", Target: s.Change.ID})
}

func (s *State) isWorkflowFailedOrInterrupted() bool {
	if s.Change == nil {
		return false
	}
	if s.Change.State == "failed" || s.Change.State == "interrupted" {
		return true
	}
	if s.Change.Workflow == nil {
		return false
	}
	switch s.Change.Workflow.Status {
	case "failed", "interrupted", "outdated":
		return true
	}
	for _, st := range s.Change.Workflow.Stages {
		if st.Status == "failed" || st.Status == "interrupted" {
			return true
		}
	}
	return false
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
	if s.Change.WorktreePath == "" && s.Project != nil && s.Change.ProjectRevision != "" && s.Project.ProjectRevision != "" && s.Change.ProjectRevision != s.Project.ProjectRevision {
		return true
	}
	return false
}

func (s *State) restartAgentFromCurrentChange() {
	stale := s.isChangeStale()
	seed := s.buildSeedFromCurrentChange()
	if seed == nil {
		return
	}
	if s.Change != nil && s.Change.ID != "" {
		s.removeActiveChangeByID(s.Change.ID)
		s.Change = nil
	}
	s.openTaskPlan(seed)
	if stale {
		s.CmdOutput = "Source changed. Review the refreshed plan before generating."
	}
}

func (s *State) buildSeedFromCurrentChange() (seed *ChangeSeed) {
	defer func() {
		if seed != nil && s.Change != nil && s.Change.Plan != nil {
			input := s.Change.Plan.ChangePlanInput
			seed.Plan = &input
		}
	}()
	if s.Change == nil {
		return nil
	}
	paths := collectChangeSessionPaths(s.Change)
	title := client.FirstNonEmpty(s.Change.Title, "Agent Mission")
	if matched := s.matchIssuesForTargets(s.Change.FindingTargets); len(matched) > 0 {
		seed := buildBatchIssuesSeed(matched)
		seed.Title = title
		seed.Paths = paths
		seed.WorktreePath = s.Change.WorktreePath
		seed.FindingTargets = s.Change.FindingTargets
		if len(s.Change.AcceptanceCriteria) > 0 {
			seed.AcceptanceCriteria = s.Change.AcceptanceCriteria
		}
		return seed
	}
	return &ChangeSeed{
		FindingTargets:     s.Change.FindingTargets,
		Title:              title,
		Paths:              paths,
		WorktreePath:       s.Change.WorktreePath,
		Kind:               client.FirstNonEmpty(s.Change.Kind, "fix"),
		Message:            firstUserChangeMessage(s.Change, title),
		AcceptanceCriteria: s.Change.AcceptanceCriteria,
	}
}

func collectChangeSessionPaths(chg *ChangeSession) []string {
	var paths []string
	for _, t := range chg.Targets {
		if t.Path != "" && !domain.ContainsString(paths, t.Path) {
			paths = append(paths, t.Path)
		}
	}
	for _, c := range chg.Changes {
		if c.Path != "" && !domain.ContainsString(paths, c.Path) {
			paths = append(paths, c.Path)
		}
	}
	for _, ft := range chg.FindingTargets {
		if ft.Path != "" && !domain.ContainsString(paths, ft.Path) {
			paths = append(paths, ft.Path)
		}
	}
	return paths
}

func firstUserChangeMessage(chg *ChangeSession, title string) string {
	for _, m := range chg.Messages {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
			return m.Content
		}
	}
	if chg != nil && (len(chg.FindingTargets) > 0 || len(chg.AcceptanceCriteria) > 0) {
		msg := fmt.Sprintf("Resolve issue: %s", title)
		if len(chg.FindingTargets) > 0 && chg.FindingTargets[0].Path != "" {
			msg += fmt.Sprintf("\nLocation: %s", chg.FindingTargets[0].Path)
		}
		if len(chg.AcceptanceCriteria) > 0 {
			msg += "\nProposed solution: " + strings.Join(chg.AcceptanceCriteria, " ")
		}
		return msg
	}
	return fmt.Sprintf("Refresh and complete proposal: %s", title)
}

func (s *State) matchIssuesForTargets(targets []FindingTarget) []IssueRow {
	if len(targets) == 0 {
		return nil
	}
	all := s.CollectAllIssues()
	var matched []IssueRow
	for _, ft := range targets {
		for _, row := range all {
			if sameFindingTarget(row.Target, ft) {
				matched = append(matched, row)
				break
			}
		}
	}
	return matched
}

func (s *State) rejectCurrentDiffHunk() {
	if s.DiffSinceReview {
		s.Error = "Switch to the complete proposal with [z] before rejecting a hunk."
		return
	}
	active, _, ok := s.activeDiffHunk()
	if !ok || s.Change == nil {
		s.CmdOutput = "No modifiable diff hunk selected to reject."
		return
	}
	fc := findChangeFileByPathOrIndex(s.Change.Changes, active.Path, active.FileIndex)
	if fc == nil {
		s.CmdOutput = "No modifiable diff hunk selected to reject."
		return
	}
	fc.Content = domain.RejectHunkFromDiff(fc.Diff.Lines, active.Hunk)
	fc.Diff.Lines = domain.RejectHunkLines(fc.Diff.Lines, active.Hunk)
	s.Change.ReviewedHash, s.Change.FileReviewHash = "", ""
	s.Change.ReviewedFiles = nil
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

func findChangeFileByPathOrIndex(changes []FileChange, path string, idx int) *FileChange {
	for i := range changes {
		if changes[i].Path == path {
			return &changes[i]
		}
	}
	if idx >= 0 && idx < len(changes) {
		return &changes[idx]
	}
	return nil
}

func (s *State) toggleWorkspaceTrust() {
	s.WorkspaceTrust = !s.WorkspaceTrust
	word := "revoked"
	if s.WorkspaceTrust {
		word = "granted"
	}
	s.CmdOutput = "Saving project trust..."
	s.enqueueAction(ActionRequest{Kind: "set-trust", Target: word, Source: ProjectIdentity{ProjectID: s.projectID(), ProjectRevision: s.projectRevision()}})
}

// IsLessonGraduated reports whether a Harness² lesson has been graduated or is already present in AGENTS.md.
func (s *State) IsLessonGraduated(rule string) bool {
	if domain.ContainsString(s.GraduatedLessons, rule) {
		return true
	}
	if s.InstructionDraft != "" && domain.ContainsGuideline(s.InstructionDraft, rule) {
		return true
	}
	if s.Instructions != nil && domain.ContainsGuideline(s.Instructions.ExistingContent, rule) {
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
	s.InstructionDraft = domain.InsertGuidelineSection(base, "Harness² Lessons & Guardrails", rule)
	if !domain.ContainsString(s.GraduatedLessons, rule) {
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

func (s *State) cancelActiveWorkflow() {
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
	}
	if s.Change == nil || !s.hasRunningWorkflow() {
		s.CmdOutput = "No running agent workflow to cancel."
		return
	}
	s.CmdOutput = fmt.Sprintf("Canceling active workflow for %s...", s.Change.Title)
	s.enqueueAction(ActionRequest{Kind: "cancel-workflow", Target: s.Change.ID})
}

func (s *State) cancelOrRejectActiveWorkflow() {
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
	}
	if s.hasRunningWorkflow() {
		s.cancelActiveWorkflow()
		return
	}
	s.rejectSelectedAgentFix()
}

func (s *State) rejectSelectedAgentFix() {
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
	}
	if s.Change == nil {
		s.CmdOutput = "No active agent proposal to reject."
		return
	}
	if s.Change.State == "applied" {
		s.CmdOutput = fmt.Sprintf("Proposal %q is already applied and committed (press [u] for Undo).", s.Change.Title)
		return
	}
	chg := *s.Change
	wtPath := chg.WorktreePath
	targets := s.collectRejectedFindingTargets(&chg)
	_, _, srcHead, srcBranch := s.resolveMergeWorktreeIdentities(wtPath)
	wfID := ""
	if s.hasRunningWorkflow() && chg.Workflow != nil {
		wfID = chg.Workflow.ID
	}
	s.CmdOutput = fmt.Sprintf("Discarding task %q...", chg.Title)
	s.enqueueAction(ActionRequest{
		Kind:      "reject-change",
		Target:    chg.ID,
		PreviewID: wfID,
		Seed: &ChangeSeed{
			Title:          chg.Title,
			WorktreePath:   wtPath,
			FindingTargets: targets,
			Kind:           srcBranch,
			Message:        srcHead,
		},
	})
}

func (s *State) discardSelectedWorktree() {
	if s.GitWorkspace == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx < 0 || idx >= len(s.GitWorkspace.Worktrees) {
		return
	}
	wt := s.GitWorkspace.Worktrees[idx]
	if wt.Current || wt.Path == "" {
		return
	}
	if chg := s.findActiveChangeByWorktree(wt.Path); chg != nil {
		cp := *chg
		s.Change = &cp
		s.rejectSelectedAgentFix()
		return
	}
	s.CmdOutput = fmt.Sprintf("Discarding worktree %s...", wt.Path)
	s.enqueueAction(ActionRequest{
		Kind: "reject-change",
		Seed: &ChangeSeed{WorktreePath: wt.Path, Kind: wt.Branch, Message: wt.Head},
	})
}

func (s *State) stepAnalysisConfigByIndex(cfgIdx int, forward bool) {
	switch cfgIdx {
	case 0:
		s.stepAnalysisBatch(forward)
	case 1:
		s.stepAnalysisBudget(forward)
	case 2:
		s.stepAnalysisAttempts(forward)
	case 3:
		s.toggleAnalysisFeatures()
	case 4:
		s.stepAnalysisCachePolicy(forward)
	}
}

func (s *State) stepAnalysisBatch(forward bool) {
	cfg := s.EffectiveAnalysisConfig()
	cfg.BatchFiles = domain.StepAnalysisBatchFiles(cfg.BatchFiles, forward)
	s.AnalysisConfig = cfg
	s.CmdOutput = fmt.Sprintf("Analysis batch limit set to %d files/batch", cfg.BatchFiles)
}

func (s *State) stepAnalysisBudget(forward bool) {
	cfg := s.EffectiveAnalysisConfig()
	cfg.BudgetSeconds = domain.StepAnalysisBudgetSeconds(cfg.BudgetSeconds, forward)
	s.AnalysisConfig = cfg
	s.CmdOutput = fmt.Sprintf("Analysis execution budget set to %ds (%dm %ds)", cfg.BudgetSeconds, cfg.BudgetSeconds/60, cfg.BudgetSeconds%60)
}

func (s *State) stepAnalysisAttempts(forward bool) {
	cfg := s.EffectiveAnalysisConfig()
	cfg.MaxAttemptsPerStage = domain.StepAnalysisMaxAttempts(cfg.MaxAttemptsPerStage, forward)
	s.AnalysisConfig = cfg
	s.CmdOutput = fmt.Sprintf("Analysis max attempts per stage set to %d", cfg.MaxAttemptsPerStage)
}

func (s *State) toggleAnalysisFeatures() {
	cfg := s.EffectiveAnalysisConfig()
	cfg.IncludeFeatures = !cfg.IncludeFeatures
	s.AnalysisConfig = cfg
	status := "ENABLED"
	if !cfg.IncludeFeatures {
		status = "SKIPPED"
	}
	s.CmdOutput = fmt.Sprintf("Analysis feature discovery %s", status)
}

func (s *State) cycleAnalysisCachePolicy() {
	s.stepAnalysisCachePolicy(true)
}

func (s *State) stepAnalysisCachePolicy(forward bool) {
	cfg := s.EffectiveAnalysisConfig()
	cur := "incremental"
	if cfg.Refresh && cfg.ClearResults {
		cur = "clean"
	} else if cfg.Refresh {
		cur = "refresh"
	}
	next := stepStringSlice([]string{"incremental", "refresh", "clean"}, cur, forward)
	switch next {
	case "refresh":
		cfg.Refresh = true
		cfg.ClearResults = false
		s.CmdOutput = "Analysis cache policy: REFRESH (bypass cache, preserve prior findings)"
	case "clean":
		cfg.Refresh = true
		cfg.ClearResults = true
		s.CmdOutput = "Analysis cache policy: CLEAN REANALYZE (bypass cache & clear prior results)"
	default:
		cfg.Refresh = false
		cfg.ClearResults = false
		s.CmdOutput = "Analysis cache policy: INCREMENTAL (reuse cached unchanged files)"
	}
	s.AnalysisConfig = cfg
}

func (s *State) resetAnalysisConfig() {
	s.AnalysisConfig = domain.DefaultAnalysisRunConfig()
	s.CmdOutput = "Reset analysis run configuration to defaults (64 files · 300s · 2 attempts · features:on · incremental)"
}

func (s *State) selectedModelScope() string {
	scopes := domain.CanonicalModelScopes()
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
	s.saveCustomPresetsToDisk()
	eff := s.EffectiveScopeModel(scope)
	s.CmdOutput = fmt.Sprintf("Assigned scope [%s] -> %s (%s · %s)",
		strings.ToUpper(scope), client.FirstNonEmpty(eff.Model, modelID), eff.ProviderOrigin, domain.FormatModelParams(eff))
}

func (s *State) resetSelectedScopeModel() {
	scope := s.selectedModelScope()
	if s.ScopeModels == nil {
		s.ScopeModels = make(map[string]string)
	}
	s.ScopeModels[scope] = scope
	s.saveCustomPresetsToDisk()
	s.CmdOutput = fmt.Sprintf("Reset scope [%s] routing to default profile (%s)", strings.ToUpper(scope), scope)
}

func (s *State) cycleModelPreset() {
	s.stepModelPreset(true)
}

func (s *State) stepModelPreset(forward bool) {
	presets := s.AllModelPresets()
	if len(presets) == 0 {
		return
	}
	pos := 0
	for i, p := range presets {
		if p.ID == s.ActivePresetID {
			if forward {
				pos = (i + 1) % len(presets)
			} else {
				pos = (i - 1 + len(presets)) % len(presets)
			}
			break
		}
	}
	if s.ClampedIdx() >= len(domain.CanonicalModelScopes()) {
		s.SelectedIdx = len(domain.CanonicalModelScopes()) + pos
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
	s.saveCustomPresetsToDisk()
	s.CmdOutput = fmt.Sprintf("Applied model preset: %s (analyze:%s · bug:%s · function:%s)",
		p.Name, p.Analyze, p.Bug, p.Function)
}

// SaveModelPreset saves the current 3-scope model assignment as a new or updated custom preset.
func (s *State) SaveModelPreset(name string) ModelPreset {
	preset := domain.BuildCustomModelPreset(name, s.ScopeModels, len(s.CustomPresets))
	updatedIdx := -1
	for i, existing := range s.CustomPresets {
		if existing.ID == preset.ID || strings.EqualFold(existing.Name, preset.Name) {
			s.CustomPresets[i] = preset
			updatedIdx = i
			break
		}
	}
	if updatedIdx < 0 {
		s.CustomPresets = append(s.CustomPresets, preset)
		updatedIdx = len(s.CustomPresets) - 1
	}
	s.ActivePresetID = preset.ID
	s.SelectedIdx = len(domain.CanonicalModelScopes()) + len(domain.DefaultModelPresets()) + updatedIdx
	s.saveCustomPresetsToDisk()
	s.CmdOutput = fmt.Sprintf("Saved model preset %q (%s) [analyze:%s · bug:%s · function:%s]",
		preset.Name, preset.ID, preset.Analyze, preset.Bug, preset.Function)
	return preset
}

// DeleteSelectedCustomPreset removes the selected (or active) custom model preset.
func (s *State) DeleteSelectedCustomPreset() bool {
	baseOffset := len(domain.CanonicalModelScopes()) + len(domain.DefaultModelPresets())
	cIdx := s.ClampedIdx() - baseOffset
	if cIdx < 0 || cIdx >= len(s.CustomPresets) {
		for i, p := range s.CustomPresets {
			if p.ID == s.ActivePresetID {
				cIdx = i
				break
			}
		}
	}
	if cIdx < 0 || cIdx >= len(s.CustomPresets) {
		return false
	}
	removed := s.CustomPresets[cIdx]
	s.CustomPresets = append(s.CustomPresets[:cIdx], s.CustomPresets[cIdx+1:]...)
	if s.ActivePresetID == removed.ID {
		s.ActivePresetID = "local-default"
	}
	s.SelectedIdx = min(s.SelectedIdx, max(0, s.ActiveListLength()-1))
	s.saveCustomPresetsToDisk()
	s.CmdOutput = fmt.Sprintf("Deleted custom model preset %q (%s)", removed.Name, removed.ID)
	return true
}

func (s *State) requestRefreshModels() {
	s.CmdOutput = "Refreshing configured & discovered LLM models..."
	s.enqueueAction(ActionRequest{Kind: "refresh-models"})
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
	return s.execAnalysisConfigCommand(cmd, arg)
}

func (s *State) execAnalysisConfigCommand(cmd, arg string) bool {
	switch cmd {
	case "batch", "batch-files":
		if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil && n > 0 {
			cfg := s.EffectiveAnalysisConfig()
			cfg.BatchFiles = n
			s.AnalysisConfig = domain.NormalizeAnalysisRunConfig(cfg)
			s.CmdOutput = fmt.Sprintf("Analysis batch limit set to %d files/batch", s.AnalysisConfig.BatchFiles)
		} else {
			s.stepAnalysisBatch(true)
		}
		return true
	case "budget", "budget-seconds":
		if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(arg), "s"))); err == nil && n > 0 {
			cfg := s.EffectiveAnalysisConfig()
			cfg.BudgetSeconds = n
			s.AnalysisConfig = domain.NormalizeAnalysisRunConfig(cfg)
			s.CmdOutput = fmt.Sprintf("Analysis execution budget set to %ds", s.AnalysisConfig.BudgetSeconds)
		} else {
			s.stepAnalysisBudget(true)
		}
		return true
	case "attempts", "max-attempts":
		if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil && n > 0 {
			cfg := s.EffectiveAnalysisConfig()
			cfg.MaxAttemptsPerStage = n
			s.AnalysisConfig = domain.NormalizeAnalysisRunConfig(cfg)
			s.CmdOutput = fmt.Sprintf("Analysis max attempts per stage set to %d", s.AnalysisConfig.MaxAttemptsPerStage)
		} else {
			s.stepAnalysisAttempts(true)
		}
		return true
	case "analysis-config", "run-config":
		s.Navigate(PageAnalysisRun)
		s.SelectedIdx = s.analysisRunSectionCount()
		s.applyAnalysisConfigArgs(arg)
		return true
	}
	return false
}

func (s *State) applyAnalysisConfigArgs(arg string) {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		cfg := s.EffectiveAnalysisConfig()
		s.CmdOutput = fmt.Sprintf("Analysis Run Config: batch=%d · budget=%ds · attempts=%d · features=%v · refresh=%v",
			cfg.BatchFiles, cfg.BudgetSeconds, cfg.MaxAttemptsPerStage, cfg.IncludeFeatures, cfg.Refresh)
		return
	}
	if strings.EqualFold(trimmed, "reset") || strings.EqualFold(trimmed, "default") {
		s.resetAnalysisConfig()
		return
	}
	cfg := s.EffectiveAnalysisConfig()
	for _, tok := range strings.Fields(trimmed) {
		if k, v, ok := strings.Cut(strings.ToLower(tok), "="); ok {
			applySingleAnalysisConfigToken(&cfg, k, v)
		}
	}
	s.AnalysisConfig = domain.NormalizeAnalysisRunConfig(cfg)
	s.CmdOutput = fmt.Sprintf("Updated analysis config: batch=%d · budget=%ds · attempts=%d · features=%v · refresh=%v · clear=%v",
		s.AnalysisConfig.BatchFiles, s.AnalysisConfig.BudgetSeconds, s.AnalysisConfig.MaxAttemptsPerStage,
		s.AnalysisConfig.IncludeFeatures, s.AnalysisConfig.Refresh, s.AnalysisConfig.ClearResults)
}

func applySingleAnalysisConfigToken(cfg *AnalysisRunConfig, k, v string) {
	switch k {
	case "batch", "batch_files":
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.BatchFiles = n
		}
	case "budget", "budget_seconds":
		if n, err := strconv.Atoi(strings.TrimSuffix(v, "s")); err == nil && n > 0 {
			cfg.BudgetSeconds = n
		}
	case "attempts", "max_attempts", "max_attempts_per_stage":
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxAttemptsPerStage = n
		}
	case "features", "include_features":
		cfg.IncludeFeatures = isTruthyConfigValue(v)
	case "refresh":
		cfg.Refresh = isTruthyConfigValue(v)
		if !cfg.Refresh {
			cfg.ClearResults = false
		}
	case "clear", "clear_results":
		cfg.ClearResults = isTruthyConfigValue(v)
	}
}

func isTruthyConfigValue(v string) bool {
	return v == "true" || v == "on" || v == "1" || v == "yes"
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
	case "preset-save", "save-preset":
		s.Navigate(PageModels)
		s.SaveModelPreset(arg)
		return true
	case "preset-delete", "delete-preset":
		s.Navigate(PageModels)
		s.deletePresetByArg(arg)
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
	scopes := domain.CanonicalModelScopes()
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
	if !domain.ContainsString(scopes, scope) {
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
	trimmed := strings.TrimSpace(arg)
	norm := strings.ToLower(trimmed)
	if norm == "" {
		s.cycleModelPreset()
		return
	}
	if norm == "save" || strings.HasPrefix(norm, "save ") {
		s.SaveModelPreset(strings.TrimSpace(trimmed[4:]))
		return
	}
	if norm == "delete" || strings.HasPrefix(norm, "delete ") {
		s.deletePresetByArg(strings.TrimSpace(trimmed[6:]))
		return
	}
	for idx, p := range s.AllModelPresets() {
		if p.ID == norm || strings.Contains(strings.ToLower(p.ID), norm) || strings.Contains(strings.ToLower(p.Name), norm) {
			s.SelectedIdx = len(domain.CanonicalModelScopes()) + idx
			s.applyModelPreset(p)
			return
		}
	}
	s.CmdOutput = fmt.Sprintf("Unknown model preset %q (use: local-default, hybrid-precision, deep-audit, or :preset-save <name>)", arg)
}

func (s *State) deletePresetByArg(arg string) {
	norm := strings.ToLower(strings.TrimSpace(arg))
	if norm != "" {
		baseOffset := len(domain.CanonicalModelScopes()) + len(domain.DefaultModelPresets())
		for i, p := range s.CustomPresets {
			if strings.ToLower(p.ID) == norm || strings.Contains(strings.ToLower(p.Name), norm) {
				s.SelectedIdx = baseOffset + i
				break
			}
		}
	}
	if !s.DeleteSelectedCustomPreset() {
		s.CmdOutput = "No custom model preset selected to delete (built-in presets cannot be deleted)"
	}
}

func (s *State) execMutationCommand(cmd, arg string) bool {
	switch cmd {
	case "dismiss", "delete", "del", "rm":
		if s.Page == PageAgents {
			s.rejectSelectedAgentFix()
		} else {
			s.dismissSelectedIssue()
		}
		return true
	case "fix":
		s.dispatchSelectedIssues()
		return true
	case "agent", "agents":
		s.execAgentMissionCommand(arg)
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
	case "reject", "discard":
		s.rejectSelectedAgentFix()
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

func (s *State) execAgentMissionCommand(arg string) {
	s.Navigate(PageAgents)
	if arg == "" {
		s.CmdOutput = "Switched to [2] Agent Studio."
		return
	}
	seed := &ChangeSeed{Title: arg, Kind: "fix", Message: arg}
	s.openTaskPlan(seed)
}
