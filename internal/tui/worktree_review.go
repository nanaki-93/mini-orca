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

func (s *State) cleanupSelectedWorktree() {
	wt := s.selectedWorktree()
	if wt == nil || wt.Current || wt.Integration != "merged" {
		s.CmdOutput = "Cleanup requires the worktree commit to be included in the current branch. Refresh after completing the merge."
		return
	}
	intent := &WorktreeIntent{ProjectIdentity: s.GitWorkspace.ProjectIdentity, GitIdentity: s.GitWorkspace.GitIdentity,
		Path: wt.Path, SourceHead: wt.Head, SourceBranch: wt.Branch, Confirm: true, ConfirmExecution: true}
	s.enqueueAction(ActionRequest{Kind: "cleanup-merged", Worktree: intent})
	s.CmdOutput = "Verifying integration and cleaning up the worktree; its branch will be retained..."
}

func (s *State) dispatchCleanupMerged(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if act.Worktree == nil || !s.matchesPlanSource(act.Worktree.ProjectIdentity) {
		return fmt.Errorf("project changed; refresh Worktrees before cleanup")
	}
	if err := s.ensureExecutionTrust(ctx, cl, reqID+"-trust"); err != nil {
		return fmt.Errorf("verify cleanup trust: %w; after external changes, use [R] Update index and grant trust again", err)
	}
	var result mergeWorktreeResponse
	err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/git/cleanup-merged", act.Worktree, &result)
	s.applyMergeWorktreeResponse(act.Worktree.Path, result)
	if err != nil {
		return err
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
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
	if wt.Integration == "merged" {
		rows = append(rows, "This commit is included in the current branch. [c] verifies and removes the clean worktree, retaining its branch.")
		rows = append(rows, "After external changes: [R] updates the project index, then :trust grants execution trust for that revision before cleanup.")
	}
	if wt.Integration == "behind" {
		rows = append(rows, "The destination advanced since this task branched. Overlapping paths need review; they do not prove a conflict.")
		rows = append(rows, renderDiagnosticBulletRows(th, th.amber, "Changed on both branches:", th.amber, th.fg, wt.OverlapPaths, contentW)...)
	}
	if s.Repository != nil && s.Repository.MergeHead != "" {
		rows = append(rows, "A merge is in progress in the current checkout. Resolve conflicts with your external Git tool and commit the merge. Then [R] Update index, :trust, and [c] Cleanup.")
		rows = append(rows, renderDiagnosticBulletRows(th, th.red, "Unresolved files:", th.red, th.fg, s.Repository.Conflicts, contentW)...)
	}
	rows = append(rows, "[r] only reads status. [R] reindexes local facts and stops outdated checks and workflows in the destination checkout.")
	return append(lines, th.renderPanelBox("INTEGRATION", "", adaptivePanelWidth(width), rows)...)
}

func (s *State) worktreeRecoveryHint() string {
	if s.Repository != nil && s.Repository.MergeHead != "" {
		return "Merge in progress · resolve + commit externally, then [R] Update index · [Enter] Recovery details"
	}
	wt := s.selectedWorktree()
	if wt != nil && wt.Integration == "behind" {
		return fmt.Sprintf("Destination advanced · %d overlapping paths · [Enter] Inspect before merging", len(wt.OverlapPaths))
	}
	return "[Enter] Inspect · [m] Merge · [c] Clean up integrated · [r] Refresh"
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
	s.WorkspaceTrust = false
	if err := s.refreshWorktrees(ctx, cl, reqID+"-refresh", act); err != nil {
		return fmt.Errorf("index updated, but Worktrees refresh failed: %w", err)
	}
	s.CmdOutput = "Index updated. Use :trust to grant execution trust for this revision, then [c] to clean up the integrated worktree."
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
	if s.projectRevision() != project.ProjectRevision {
		s.WorkspaceTrust = false
	}
	s.Project, s.Index, s.Repository, s.GitWorkspace = &project, &index, &workspace.GitRepository, &workspace
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
