package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type GitBranch struct {
	Name         string `json:"name"`
	Head         string `json:"head"`
	WorktreePath string `json:"worktree_path,omitempty"`
}

type GitWorktree struct {
	Path string `json:"path"`
	GitIdentity
	Current   bool                 `json:"current"`
	Locked    bool                 `json:"locked"`
	Prunable  bool                 `json:"prunable"`
	Dirty     *bool                `json:"dirty,omitempty"`
	Completed []ChangeHistoryEntry `json:"completed"`
	Error     string               `json:"error,omitempty"`
}

type GitWorkspace struct {
	*GitRepository
	Branches  []GitBranch   `json:"branches"`
	Worktrees []GitWorktree `json:"worktrees"`
}

func (s *Service) GitWorkspace(ctx context.Context, id, revision string) (*GitWorkspace, error) {
	repository, err := s.GitRepository(ctx, id, revision)
	if err != nil {
		return nil, err
	}
	result := &GitWorkspace{GitRepository: repository, Branches: []GitBranch{}, Worktrees: []GitWorktree{}}
	if !repository.Available {
		return result, nil
	}
	result.Worktrees, err = readGitWorktrees(ctx, repository.Root)
	if err != nil {
		return nil, err
	}
	for i := range result.Worktrees {
		populateGitWorktree(ctx, repository.Root, &result.Worktrees[i])
	}
	result.Branches, err = readGitBranches(ctx, repository.Root, result.Worktrees)
	if err != nil {
		return nil, err
	}
	current, err := s.GitRepository(ctx, id, revision)
	if err != nil {
		return nil, err
	}
	if current.GitIdentity != repository.GitIdentity {
		return nil, project.ErrRevisionConflict
	}
	return result, nil
}

func readGitWorktrees(ctx context.Context, root string) ([]GitWorktree, error) {
	output, err := gitRead(ctx, root, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	result := []GitWorktree{}
	for _, record := range strings.Split(output, "\x00\x00") {
		if record == "" {
			continue
		}
		tree := GitWorktree{Completed: []ChangeHistoryEntry{}}
		for _, field := range strings.Split(record, "\x00") {
			key, value, _ := strings.Cut(field, " ")
			switch key {
			case "worktree":
				tree.Path = value
			case "HEAD":
				tree.Head = value
			case "branch":
				tree.Branch = strings.TrimPrefix(value, "refs/heads/")
			case "locked":
				tree.Locked = true
			case "prunable":
				tree.Prunable = true
			}
		}
		if tree.Path == "" {
			return nil, fmt.Errorf("invalid Git worktree record")
		}
		tree.Current = filepath.Clean(tree.Path) == filepath.Clean(root)
		result = append(result, tree)
		if len(result) > 100 {
			return nil, fmt.Errorf("more than 100 worktrees; use Git directly to manage this repository")
		}
	}
	return result, nil
}

func populateGitWorktree(ctx context.Context, root string, tree *GitWorktree) {
	canonical, err := project.CanonicalRoot(tree.Path)
	if err != nil {
		tree.Error = err.Error()
		return
	}
	tree.Path, tree.Current = canonical, canonical == root
	repository, err := readGitRepository(ctx, canonical)
	if err != nil {
		tree.Error = err.Error()
		return
	}
	if !repository.Available || !repository.AtRoot || repository.GitIdentity != tree.GitIdentity {
		tree.Error = "Worktree changed; refresh Git status."
		return
	}
	tree.Dirty = &repository.Dirty
	history, err := listChangeSessions(canonical)
	if err != nil {
		tree.Error = "Fix history unavailable: " + err.Error()
		return
	}
	for _, entry := range history {
		if entry.State == "applied" && entry.Commit != nil && entry.Commit.State == "committed" {
			tree.Completed = append(tree.Completed, entry)
		}
	}
}

func readGitBranches(ctx context.Context, root string, trees []GitWorktree) ([]GitBranch, error) {
	output, err := gitRead(ctx, root, "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	result := []GitBranch{}
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		name, head, ok := strings.Cut(line, "\x00")
		if !ok {
			return nil, fmt.Errorf("invalid Git branch record")
		}
		branch := GitBranch{Name: strings.TrimPrefix(name, "refs/heads/"), Head: head}
		for _, tree := range trees {
			if tree.Branch == branch.Name {
				branch.WorktreePath = tree.Path
			}
		}
		result = append(result, branch)
	}
	return result, nil
}
