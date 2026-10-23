package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type WorktreeIntent struct {
	ProjectIdentity
	GitIdentity
	Path             string `json:"path"`
	SourceHead       string `json:"source_head"`
	SourceBranch     string `json:"source_branch"`
	Confirm          bool   `json:"confirm"`
	ConfirmExecution bool   `json:"confirm_execution"`
}

func (s *State) selectedWorktree() *GitWorktree {
	if s.GitWorkspace == nil || len(s.GitWorkspace.Worktrees) == 0 {
		return nil
	}
	return &s.GitWorkspace.Worktrees[s.ClampedIdx()]
}

func worktreeStatus(wt GitWorktree) string {
	switch {
	case wt.Current:
		return "CURRENT"
	case wt.Error != "":
		return "UNAVAILABLE"
	case wt.Locked:
		return "LOCKED"
	case wt.Dirty:
		return "DIRTY"
	case worktreeHasUnfinishedTasks(&wt):
		return worktreeTaskStatus(wt)
	case wt.Integration == "merged":
		return "INTEGRATED"
	case wt.Integration == "behind":
		return "BEHIND"
	case wt.Mergeable:
		return "READY"
	default:
		return "WAITING"
	}
}

func worktreeTaskStatus(wt GitWorktree) string {
	status := "DRAFT"
	for _, entry := range wt.Sessions {
		switch entry.WorkflowStatus {
		case "failed":
			return "FAILED"
		case "running", "queued", "canceling":
			status = "WORKING"
		case "awaiting_human_review":
			status = "REVIEW"
		case "stale", "canceled":
			status = "NEEDS_ATTENTION"
		}
	}
	return status
}

func (s *State) cleanupSelectedWorktree() {
	wt := s.selectedWorktree()
	if wt == nil || wt.Current {
		s.CmdOutput = "Select another worktree to clean up; the current checkout is retained."
		return
	}
	if wt.Dirty || wt.Locked || wt.Prunable || wt.Error != "" {
		s.CmdOutput = "Worktree retained. Resolve local changes, locks or unavailable status, then refresh before cleanup."
		return
	}
	intent := &WorktreeIntent{ProjectIdentity: s.GitWorkspace.ProjectIdentity, GitIdentity: s.GitWorkspace.GitIdentity,
		Path: wt.Path, SourceHead: wt.Head, SourceBranch: wt.Branch, Confirm: true, ConfirmExecution: true}
	if wt.Integration != "merged" || worktreeHasUnfinishedTasks(wt) {
		s.confirmWorktreeDiscard(wt, intent)
		return
	}
	s.enqueueAction(ActionRequest{Kind: "cleanup-merged", Worktree: intent})
	s.CmdOutput = "Verifying integration and cleaning up the worktree; its branch will be retained..."
}

func worktreeHasUnfinishedTasks(wt *GitWorktree) bool {
	for _, entry := range wt.Sessions {
		if entry.State != "applied" || entry.Commit == nil || entry.Commit.State != "committed" {
			return true
		}
	}
	return false
}

func (s *State) confirmWorktreeDiscard(wt *GitWorktree, intent *WorktreeIntent) {
	seed := &ChangeSeed{WorktreePath: wt.Path, Kind: wt.Branch, Message: wt.Head}
	for _, entry := range wt.Sessions {
		if entry.State != "applied" {
			seed.FindingTargets = append(seed.FindingTargets, entry.FindingTargets...)
		}
	}
	s.Confirmation = &Confirmation{
		Title: "Discard worktree and saved tasks?", Destructive: true, AcceptLabel: "Discard worktree",
		Message: fmt.Sprintf("Remove %s and its saved plans and proposals? Running tasks will be stopped. Commits remain on branch %s; uncommitted source changes block removal.", wt.Path, wt.Branch),
		OnConfirm: func(state *State) {
			state.enqueueAction(ActionRequest{Kind: "reject-change", Worktree: intent, Seed: seed})
			state.CmdOutput = "Stopping tasks and discarding the worktree; waiting for removal confirmation..."
		},
	}
}

func (s *State) postCapturedWorktreeRemoval(ctx context.Context, cl *Client, reqID, endpoint string, intent *WorktreeIntent) (mergeWorktreeResponse, error) {
	if intent == nil || !s.matchesPlanSource(intent.ProjectIdentity) {
		return mergeWorktreeResponse{}, fmt.Errorf("project changed; refresh Worktrees before cleanup")
	}
	if err := s.ensureExecutionTrust(ctx, cl, reqID+"-trust"); err != nil {
		return mergeWorktreeResponse{}, fmt.Errorf("verify cleanup trust: %w; after external changes, use [R] Update index to refresh the trusted workspace", err)
	}
	var result mergeWorktreeResponse
	err := cl.DoJSON(ctx, reqID, http.MethodPost, endpoint, intent, &result)
	if err == nil && result.Error != "" {
		err = fmt.Errorf("%s", result.Error)
	}
	return result, err
}

func (s *State) forgetDiscardedWorktree(path string) {
	s.removeActiveSession("", path)
	kept := s.ChangeHistory[:0]
	for _, entry := range s.ChangeHistory {
		if entry.WorktreePath != path {
			kept = append(kept, entry)
		}
	}
	s.ChangeHistory = kept
	if s.Change != nil && s.Change.WorktreePath == path {
		s.Change = nil
	}
	if s.PlanDraft != nil && s.PlanDraft.Seed.WorktreePath == path {
		s.PlanDraft = nil
	}
}

func (s *State) dispatchCleanupMerged(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	result, err := s.postCapturedWorktreeRemoval(ctx, cl, reqID, "/api/projects/current/git/cleanup-merged", act.Worktree)
	if err != nil {
		return err
	}
	s.applyMergeWorktreeResponse(act.Worktree.Path, result)
	if result.RemovedWorktree == act.Worktree.Path {
		s.Navigate(PageGit)
		s.CmdOutput = "Merged worktree removed; linked findings are completed and the Git branch is retained."
	} else if len(result.Warnings) == 0 {
		return fmt.Errorf("cleanup was not confirmed; refresh Worktrees")
	}
	return nil
}

func (s *State) renderWorktreeDetail(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "WORKTREE", "[r] Refresh · [R] Update index · [m] Merge · [c] Cleanup · [Esc] Back")
	wt := s.selectedWorktree()
	if wt == nil {
		return append(lines, "  No worktree selected.")
	}
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	rows := renderDiagnosticRow(th, th.cyan, "Branch:", th.fg, wt.Branch, contentW)
	rows = append(rows, renderDiagnosticRow(th, th.cyan, "Path:", th.fg, wt.Path, contentW)...)
	rows = append(rows, renderDiagnosticRow(th, th.cyan, "Commit:", th.fg, wt.Head, contentW)...)
	rows = append(rows, renderDiagnosticRow(th, th.amber, "Status:", th.fg, worktreeStatus(*wt), contentW)...)
	rows = append(rows, renderDiagnosticRow(th, th.red, "Error:", th.fg, wt.Error, contentW)...)
	rows = append(rows, renderDiagnosticRow(th, th.green, "Next:", th.fg, s.worktreeNextAction(wt), contentW)...)
	for _, entry := range wt.Sessions {
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Task:", th.fg, entry.Title, contentW)...)
		rows = append(rows, renderDiagnosticRow(th, th.amber, "Result:", th.fg, entry.WorkflowReason, contentW)...)
	}
	if s.Repository != nil && s.Repository.MergeHead != "" {
		rows = append(rows, renderDiagnosticBulletRows(th, th.red, "Unresolved files:", th.red, th.fg, s.Repository.Conflicts, contentW)...)
	} else if wt.Integration == "behind" {
		rows = append(rows, renderDiagnosticBulletRows(th, th.amber, "Changed on both branches:", th.amber, th.fg, wt.OverlapPaths, contentW)...)
	}
	return append(lines, th.renderPanelBox("INTEGRATION", "", adaptivePanelWidth(width), rows)...)
}

func (s *State) worktreeNextAction(wt *GitWorktree) string {
	switch {
	case s.Repository != nil && s.Repository.MergeHead != "":
		return "Resolve conflicts with your external Git tool and commit. Then [R] update the index and [c] clean up."
	case wt.Current:
		return "This is the current checkout. Select another worktree to merge or remove."
	case wt.Dirty:
		return "Commit or move local changes before cleanup; they are protected from deletion."
	case wt.Locked || wt.Prunable || wt.Error != "":
		return "Resolve the worktree's lock or unavailable state in Git, then [r] refresh."
	case worktreeHasUnfinishedTasks(wt):
		return "Saved tasks are unfinished. Open Work > Tasks to inspect checks and retry, or [c] explicitly discard them; the Git branch is retained."
	case wt.Integration == "merged":
		return "Integrated. [c] removes the clean checkout and completes linked findings; the Git branch is retained."
	case wt.Integration == "behind":
		return "The destination advanced. Review overlapping changes before merging; overlap alone does not prove a conflict."
	default:
		return "[m] Merge committed work, or [c] discard the checkout while retaining its Git branch."
	}
}

func (s *State) worktreeRecoveryHint() string {
	if s.Repository != nil && s.Repository.MergeHead != "" {
		return "Merge in progress · resolve + commit externally, then [R] Update index · [Enter] Recovery details"
	}
	wt := s.selectedWorktree()
	if wt != nil && wt.Integration == "behind" {
		return fmt.Sprintf("Destination advanced · %d overlapping paths · [Enter] Inspect before merging", len(wt.OverlapPaths))
	}
	return "[Enter] Inspect · [m] Merge · [c] Clean up / discard · [r] Refresh"
}

func (s *State) handleWorktreeKey(key string) bool {
	switch key {
	case "c":
		s.cleanupSelectedWorktree()
	case "r", "R":
		path := ""
		if wt := s.selectedWorktree(); wt != nil {
			path = wt.Path
		}
		kind := "refresh-worktrees"
		if key == "R" {
			kind = "reindex-worktrees"
		}
		s.enqueueAction(ActionRequest{Kind: kind, Target: path, Source: ProjectIdentity{ProjectID: s.projectID(), ProjectRevision: s.projectRevision()}})
	case "Enter", "i":
		s.Navigate(PageWorktreeDetail)
	case "Escape", "Backspace":
		if s.Page != PageWorktreeDetail {
			return false
		}
		s.Navigate(PageGit)
	default:
		return false
	}
	return true
}

func (s *State) reindexWorktrees(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if !s.matchesPlanSource(act.Source) {
		return fmt.Errorf("project changed; refresh Worktrees before updating the index")
	}
	if err := s.dispatchReindex(ctx, cl, reqID); err != nil {
		return err
	}
	if err := s.refreshWorktrees(ctx, cl, reqID+"-refresh", act); err != nil {
		return fmt.Errorf("index updated, but Worktrees refresh failed: %w", err)
	}
	s.CmdOutput = "Index updated. [c] Clean up the selected worktree."
	if !s.WorkspaceTrust {
		s.CmdOutput = "Index updated. Use :trust to grant project execution trust, then [c] to clean up the worktree."
	}
	return nil
}

func (s *State) refreshWorktrees(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	var project Project
	if err := cl.DoJSON(ctx, reqID+"-project", http.MethodGet, "/api/projects/current", nil, &project); err != nil {
		return err
	}
	if project.ProjectID == "" || project.ProjectID != act.Source.ProjectID {
		return fmt.Errorf("project changed; reopen the project before refreshing Worktrees")
	}
	query := "?project_id=" + url.QueryEscape(project.ProjectID) + "&project_revision=" + url.QueryEscape(project.ProjectRevision)
	var workspace GitWorkspace
	if err := cl.DoJSON(ctx, reqID+"-workspace", http.MethodGet, "/api/projects/current/git/workspace"+query, nil, &workspace); err != nil {
		return err
	}
	var index ProjectIndex
	if err := cl.DoJSON(ctx, reqID+"-index", http.MethodGet, "/api/projects/current/index", nil, &index); err != nil {
		return err
	}
	if s.projectID() != act.Source.ProjectID || workspace.ProjectIdentity != project.ProjectIdentity || index.ProjectIdentity != project.ProjectIdentity {
		return fmt.Errorf("project changed during refresh; retry Worktrees refresh")
	}
	revisionChanged := s.projectRevision() != project.ProjectRevision
	if revisionChanged {
		s.WorkspaceTrust = false
	}
	s.Project, s.Index, s.Repository, s.GitWorkspace = &project, &index, &workspace.GitRepository, &workspace
	if revisionChanged {
		revQuery, _ := s.resourceQueries()
		if err := s.syncExecutionTrust(ctx, cl, revQuery); err != nil {
			return fmt.Errorf("worktrees refreshed, but %w", err)
		}
	}
	for i, wt := range workspace.Worktrees {
		if wt.Path == act.Target {
			s.SelectedIdx = i
			break
		}
	}
	s.SelectedIdx = s.ClampedIdx()
	s.Error = ""
	s.CmdOutput = "Worktrees refreshed. [Enter] Inspect · [c] Clean up integrated worktree"
	return nil
}

func (s *State) overlappingTask(paths []string, savedID string) string {
	for _, chg := range s.ActiveChanges {
		if chg == nil || chg.ID == savedID || chg.State == "applied" || chg.State == "undone" {
			continue
		}
		if len(chg.Changes) == 0 && chg.Workflow == nil {
			continue
		}
		for _, target := range chg.Targets {
			for _, path := range paths {
				if target.Path == path {
					return fmt.Sprintf("%s is already in task %q. Finish or discard that task before starting another fix in the same file.", path, chg.Title)
				}
			}
		}
	}
	return ""
}

// A saved plan may live in a checkout created before another fix was merged.
// Do not regenerate against that base without another explicit plan review.
func validateSavedWorktreeBase(ctx context.Context, cl *Client, task startAgentTask) error {
	if task.act.Kind != "start-agent" || task.act.Seed.WorktreePath == "" || task.head == "" {
		return nil
	}
	var ws GitWorkspace
	if err := cl.DoJSON(ctx, task.reqID+"-base", http.MethodGet, "/api/projects/current/git/workspace?project_id="+url.QueryEscape(task.projectID)+"&project_revision="+url.QueryEscape(task.projectRevision), nil, &ws); err != nil {
		return err
	}
	for _, wt := range ws.Worktrees {
		if wt.Path != task.act.Seed.WorktreePath {
			continue
		}
		if wt.Error != "" {
			return fmt.Errorf("inspect task worktree: %s", wt.Error)
		}
		if wt.Head == ws.Head || wt.Integration == "up_to_date" {
			return nil
		}
		return fmt.Errorf("task worktree is behind the destination; refresh, then :replan to review a new plan on the current branch. The old worktree is retained")
	}
	return fmt.Errorf("task worktree is no longer registered; refresh and :replan")
}
