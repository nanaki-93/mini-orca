package tui

import "fmt"

// Applying a reviewed proposal commits only in its isolated checkout. The
// destination and finding completion remain unchanged until explicit merge.
func (s *State) queueCommittedWorktree(chg *ChangeSession) {
	chg.State = "applied"
	s.registerCreatedWorktree(chg.WorktreePath, chg.Commit.OID, chg.Commit.Branch)
	s.archiveAndCompleteChange(chg)
	for _, entry := range s.ChangeHistory {
		if entry.ID != chg.ID {
			continue
		}
		for i := range s.GitWorkspace.Worktrees {
			wt := &s.GitWorkspace.Worktrees[i]
			if wt.Path == chg.WorktreePath {
				wt.Sessions = replaceWorktreeSession(wt.Sessions, entry)
				wt.Completed = replaceWorktreeSession(wt.Completed, entry)
				wt.Integration = ""
				wt.Mergeable = !worktreeHasUnfinishedTasks(wt)
			}
		}
	}
	if s.returnCommittedMission(chg) {
		return
	}
	s.Navigate(PageGit)
	for i, wt := range s.GitWorkspace.Worktrees {
		if wt.Path == chg.WorktreePath {
			s.SelectedIdx = i
		}
	}
	s.CmdOutput = fmt.Sprintf("Committed %q. TO_MERGE · [m] Merge · [x] Reject and return linked issues to TODO.", chg.Title)
}

func replaceWorktreeSession(entries []ChangeHistoryEntry, entry ChangeHistoryEntry) []ChangeHistoryEntry {
	for i := range entries {
		if entries[i].ID == entry.ID {
			entries[i] = entry
			return entries
		}
	}
	return append(entries, entry)
}

func (s *State) withoutPendingWorktreeIssues(rows []IssueRow) []IssueRow {
	if s.GitWorkspace == nil {
		return rows
	}
	var targets []FindingTarget
	for _, wt := range s.GitWorkspace.Worktrees {
		if wt.Current {
			continue
		}
		for _, entry := range wt.Sessions {
			if entry.State == "applied" && entry.Commit != nil && entry.Commit.State == "committed" {
				targets = append(targets, entry.FindingTargets...)
			}
		}
	}
	kept := rows[:0]
	for _, row := range rows {
		if row.Status == "done" || !matchesAnyFindingTarget(row.Target, targets) {
			kept = append(kept, row)
		}
	}
	return kept
}
