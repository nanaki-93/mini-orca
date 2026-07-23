package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitOps provides git operations for version control integration.
type GitOps struct {
	repoDir string
}

// NewGitOps creates a new git operations handler.
func NewGitOps(repoDir string) *GitOps {
	return &GitOps{
		repoDir: repoDir,
	}
}

// GitAdd adds files to git staging.
func (g *GitOps) GitAdd(paths ...string) error {
	args := []string{"add"}
	if len(paths) > 0 {
		args = append(args, paths...)
	} else {
		args = append(args, ".")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GitCommit creates a commit with the given message.
func (g *GitOps) GitCommit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GitDiff shows the diff for a file or all changes.
func (g *GitOps) GitDiff(paths ...string) (string, error) {
	args := []string{"diff"}
	if len(paths) > 0 {
		args = append(args, paths...)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	return string(output), nil
}

// GitStatus shows the current git status.
func (g *GitOps) GitStatus() (string, error) {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	return string(output), nil
}

// GitLog returns the last n commits.
func (g *GitOps) GitLog(n int) (string, error) {
	cmd := exec.Command("git", "log", "--oneline", fmt.Sprintf("-n%d", n))
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git log failed: %w", err)
	}

	return string(output), nil
}

// GitBranch returns the current branch name.
func (g *GitOps) GitBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git branch failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GitInit initializes a new git repository.
func (g *GitOps) GitInit() error {
	cmd := exec.Command("git", "init")
	cmd.Dir = g.repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsGitRepo checks if the directory is a git repository.
func (g *GitOps) IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = g.repoDir
	return cmd.Run() == nil
}

// CommitAtomicUnit commits changes from an atomic unit implementation.
func (g *GitOps) CommitAtomicUnit(unitID string, message string) error {
	// Add all changed files
	if err := g.GitAdd(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Create a descriptive commit message
	fullMessage := fmt.Sprintf("[%s] %s", unitID, message)

	return g.GitCommit(fullMessage)
}
