package project

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitBranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
}

type GitWorktreeInfo struct {
	Path       string `json:"path"`
	Head       string `json:"head"`
	Branch     string `json:"branch"`
	IsBare     bool   `json:"is_bare"`
	IsDetached bool   `json:"is_detached"`
	IsMain     bool   `json:"is_main"`
}

type GitMergeResult struct {
	Success  bool   `json:"success"`
	Conflict bool   `json:"conflict"`
	Message  string `json:"message"`
}

// ListGitBranches returns all local branches in the repository.
func ListGitBranches(root string) ([]GitBranchInfo, error) {
	output, err := runGit(root, "branch", "--format=%(refname:short)|%(HEAD)")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var branches []GitBranchInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		name := parts[0]
		isCurrent := len(parts) > 1 && parts[1] == "*"
		branches = append(branches, GitBranchInfo{
			Name:      name,
			IsCurrent: isCurrent,
		})
	}
	return branches, nil
}

// SwitchGitBranch checks out or switches to the specified branch.
func SwitchGitBranch(root, branchName string) error {
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	_, err := runGit(root, "checkout", branchName)
	if err != nil {
		return fmt.Errorf("failed to switch branch: %w", err)
	}
	return nil
}

// CommitGitChanges stages all modified/tracked changes and commits with the given message.
func CommitGitChanges(root, message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}
	// Stage changes
	_, err := runGit(root, "add", "-A")
	if err != nil {
		return "", fmt.Errorf("failed to stage changes: %w", err)
	}
	// Commit
	out, err := runGit(root, "commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ListGitWorktrees returns all worktrees for the repository.
func ListGitWorktrees(root string) ([]GitWorktreeInfo, error) {
	output, err := runGit(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []GitWorktreeInfo
	blocks := strings.Split(output, "\n\n")
	for i, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		wt := GitWorktreeInfo{IsMain: (i == 0)}
		lines := strings.Split(block, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "worktree ") {
				wt.Path = strings.TrimPrefix(line, "worktree ")
			} else if strings.HasPrefix(line, "HEAD ") {
				wt.Head = strings.TrimPrefix(line, "HEAD ")
			} else if strings.HasPrefix(line, "branch ") {
				branchRef := strings.TrimPrefix(line, "branch ")
				wt.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
			} else if line == "bare" {
				wt.IsBare = true
			} else if line == "detached" {
				wt.IsDetached = true
			}
		}
		if wt.Path != "" {
			worktrees = append(worktrees, wt)
		}
	}
	return worktrees, nil
}

// CreateGitWorktree creates a new worktree at path with a new branch branchName based on startPoint (or HEAD).
func CreateGitWorktree(root, path, branchName string) error {
	path = strings.TrimSpace(path)
	branchName = strings.TrimSpace(branchName)
	if path == "" || branchName == "" {
		return fmt.Errorf("path and branch_name are required")
	}

	// Run git worktree add -b branchName path
	_, err := runGit(root, "worktree", "add", "-b", branchName, path)
	if err != nil {
		// Try without -b if branch already exists
		_, err2 := runGit(root, "worktree", "add", path, branchName)
		if err2 != nil {
			return fmt.Errorf("failed to add worktree: %w", err)
		}
	}
	return nil
}

// MergeGitWorktree merges the given source branch into the repository at root.
func MergeGitWorktree(root, sourceBranch string) (GitMergeResult, error) {
	sourceBranch = strings.TrimSpace(sourceBranch)
	if sourceBranch == "" {
		return GitMergeResult{}, fmt.Errorf("source branch is required")
	}

	cmd := exec.Command("git", "-C", root, "merge", sourceBranch, "--no-ff", "-m", fmt.Sprintf("Merge branch '%s'", sourceBranch))
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)

	if err == nil {
		return GitMergeResult{
			Success:  true,
			Conflict: false,
			Message:  outStr,
		}, nil
	}

	// Check if conflict occurred
	if strings.Contains(outStr, "CONFLICT") || strings.Contains(outStr, "Automatic merge failed") {
		return GitMergeResult{
			Success:  false,
			Conflict: true,
			Message:  outStr,
		}, nil
	}

	return GitMergeResult{
		Success:  false,
		Conflict: false,
		Message:  outStr,
	}, fmt.Errorf("merge failed: %w", err)
}
