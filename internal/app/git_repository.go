package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type GitIdentity struct {
	Head   string `json:"head"`
	Branch string `json:"branch"`
}

type GitRepository struct {
	GitIdentity
	ProjectID       string   `json:"project_id"`
	ProjectRevision string   `json:"project_revision"`
	Available       bool     `json:"available"`
	Root            string   `json:"root"`
	AtRoot          bool     `json:"at_root"`
	Dirty           bool     `json:"dirty"`
	MergeHead       string   `json:"merge_head,omitempty"`
	Operation       string   `json:"operation,omitempty"`
	Conflicts       []string `json:"conflicts"`
}

func (s *Service) GitRepository(ctx context.Context, id, revision string) (*GitRepository, error) {
	root := s.manager.Root()
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != id || index.ProjectRevision != revision {
		return nil, project.ErrRevisionConflict
	}
	repository, err := readGitRepository(ctx, root)
	if err != nil {
		return nil, err
	}
	current, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if s.manager.Root() != root || current.ProjectID != id || current.ProjectRevision != revision {
		return nil, project.ErrRevisionConflict
	}
	repository.ProjectID, repository.ProjectRevision = id, revision
	return repository, nil
}

func readGitRepository(ctx context.Context, root string) (*GitRepository, error) {
	repository := &GitRepository{Conflicts: []string{}}
	top, err := gitRead(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		var exit *exec.ExitError
		if errors.Is(err, exec.ErrNotFound) || (errors.As(err, &exit) && strings.Contains(err.Error(), "not a git repository")) {
			return repository, nil
		}
		return nil, err
	}
	repository.Root, err = project.CanonicalRoot(strings.TrimSpace(top))
	if err != nil {
		return nil, err
	}
	repository.Available, repository.AtRoot = true, repository.Root == filepath.Clean(root)
	branch, err := gitRead(ctx, root, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	repository.Branch = strings.TrimSpace(branch)
	head, err := gitRead(ctx, root, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil && !gitExitCode(err, 1) {
		return nil, err
	}
	repository.Head = strings.TrimSpace(head)
	status, err := gitRead(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	repository.Dirty = gitStatusDirty(status)
	unmerged, err := gitRead(ctx, root, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	if unmerged != "" {
		repository.Conflicts = strings.Split(strings.TrimSuffix(unmerged, "\x00"), "\x00")
	}
	if err := readGitOperation(ctx, root, repository); err != nil {
		return nil, err
	}
	return repository, nil
}

func readGitOperation(ctx context.Context, root string, repository *GitRepository) error {
	merge, err := gitRead(ctx, root, "rev-parse", "--verify", "-q", "MERGE_HEAD")
	if err == nil {
		repository.MergeHead = strings.TrimSpace(merge)
	} else if !gitExitCode(err, 1) {
		return err
	}
	gitDir, err := gitRead(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	for _, name := range []string{"rebase-merge", "rebase-apply", "CHERRY_PICK_HEAD", "REVERT_HEAD", "sequencer"} {
		_, err := os.Stat(filepath.Join(strings.TrimSpace(gitDir), name))
		if err == nil {
			repository.Operation = name
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func gitStatusDirty(status string) bool {
	for _, entry := range strings.Split(status, "\x00") {
		// Untracked private workflow metadata is not a user's source change.
		if entry != "" && entry != "?? .mini-orca/" {
			return true
		}
	}
	return false
}

func gitExitCode(err error, code int) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == code
}

func requireGitIdentity(repository *GitRepository, expected GitIdentity) error {
	if !repository.Available || !repository.AtRoot || repository.Branch == "" {
		return fmt.Errorf("open a Git worktree root on a local branch")
	}
	if expected.Head == "" || expected.Branch == "" || repository.GitIdentity != expected {
		return fmt.Errorf("%w: Git branch or HEAD changed; refresh and review again", project.ErrRevisionConflict)
	}
	if repository.MergeHead != "" || repository.Operation != "" || len(repository.Conflicts) != 0 {
		return fmt.Errorf("finish or abort the current Git operation first")
	}
	return nil
}

func captureChangeGit(ctx context.Context, root string) (*GitIdentity, error) {
	repository, err := readGitRepository(ctx, root)
	if err != nil {
		return nil, err
	}
	if !repository.Available {
		return nil, nil
	}
	return &repository.GitIdentity, nil
}

func verifyChangeGit(ctx context.Context, root string, expected *GitIdentity) error {
	if expected == nil {
		return nil
	}
	actual, err := captureChangeGit(ctx, root)
	if err != nil {
		return err
	}
	if actual == nil || *actual != *expected {
		return project.ErrRevisionConflict
	}
	return nil
}
