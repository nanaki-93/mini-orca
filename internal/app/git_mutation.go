package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type GitMutationRequest struct {
	GitIdentity
	ProjectID        string `json:"project_id"`
	ProjectRevision  string `json:"project_revision"`
	Confirm          bool   `json:"confirm"`
	ConfirmExecution bool   `json:"confirm_execution"`
	TargetBranch     string `json:"target_branch,omitempty"`
	Path             string `json:"path,omitempty"`
	SourceHead       string `json:"source_head,omitempty"`
	SourceBranch     string `json:"source_branch,omitempty"`
	MergeHead        string `json:"merge_head,omitempty"`
}

type GitMutationResult struct {
	State           string                `json:"state"`
	Path            string                `json:"path,omitempty"`
	RemovedWorktree string                `json:"removed_worktree,omitempty"`
	Repository      *GitRepository        `json:"repository,omitempty"`
	Index           *project.ProjectIndex `json:"index,omitempty"`
	Error           string                `json:"error,omitempty"`
	Warnings        []string              `json:"warnings"`
}

func (s *Service) gitMutationPreflight(ctx context.Context, request GitMutationRequest, abort bool) (*GitRepository, error) {
	if !request.Confirm || !request.ConfirmExecution {
		return nil, fmt.Errorf("confirm the Git action and execution of repository filters or merge drivers")
	}
	repository, err := s.GitRepository(ctx, request.ProjectID, request.ProjectRevision)
	if err != nil {
		return nil, err
	}
	expected := *repository
	if abort {
		expected.MergeHead = ""
		expected.Conflicts = nil
	}
	if err := requireGitIdentity(&expected, request.GitIdentity); err != nil {
		return nil, err
	}
	if err := s.requireProjectExecutionTrust(request.ProjectRevision); err != nil {
		return nil, err
	}
	return repository, nil
}

func (s *Service) CreateGitWorktree(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, false)
	if err != nil {
		return nil, err
	}
	if err := validateNewGitBranch(ctx, repository.Root, request.TargetBranch); err != nil {
		return nil, err
	}
	path, err := validateNewWorktreePath(ctx, repository.Root, request.Path)
	if err != nil {
		return nil, err
	}
	if err := prepareWorktreeDirectory(ctx, repository.Root); err != nil {
		return nil, err
	}
	_, err = gitRead(ctx, repository.Root, "worktree", "add", "--no-track", "-b", request.TargetBranch, "--", path, repository.Head)
	result := &GitMutationResult{State: "created", Path: path, Repository: repository, Warnings: []string{}}
	if err != nil {
		result.State, result.Error = "failed", err.Error()
	} else if err := s.initializeWorktreeInventory(ctx, path); err != nil {
		result.Error = "Worktree created, but local initialization failed: " + err.Error()
	}
	return result, nil
}

func (s *Service) initializeWorktreeInventory(ctx context.Context, path string) error {
	runtime := s.runtimes.analyze
	analyzer := project.NewAnalyzerWithProvenance(nil, runtime.profile.Model, runtime.effective.Scope, runtime.effective.ProviderOrigin, runtime.effective.ReasoningEffort)
	return analyzer.StoreLocalInventory(ctx, path)
}

func validateNewGitBranch(ctx context.Context, root, branch string) error {
	if branch == "" || strings.HasPrefix(branch, "-") || len(branch) > 240 {
		return fmt.Errorf("provide a new local branch name of at most 240 bytes")
	}
	if _, err := gitRead(ctx, root, "check-ref-format", "refs/heads/"+branch); err != nil {
		return err
	}
	_, err := gitRead(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return fmt.Errorf("branch %s already exists; choose a new branch name", branch)
	}
	if !gitExitCode(err, 1) {
		return err
	}
	return nil
}

func validateNewWorktreePath(ctx context.Context, root, path string) (string, error) {
	if strings.TrimSpace(path) == "" || strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("choose an absolute path for the new worktree")
	}
	if !filepath.IsAbs(path) {
		cleaned := filepath.Clean(path)
		if filepath.Dir(cleaned) == ".worktrees" {
			path = filepath.Join(root, cleaned)
		} else {
			return "", fmt.Errorf("choose an absolute path for the new worktree")
		}
	}
	path = filepath.Clean(path)
	parent := filepath.Join(root, ".worktrees")
	if filepath.Dir(path) != parent {
		return "", fmt.Errorf("create the worktree directly inside %s", parent)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("worktree path must not already exist")
	}
	if info, err := os.Lstat(parent); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf(".worktrees must be a directory, not a symlink")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := validateWorktreeOverlap(ctx, root, path); err != nil {
		return "", err
	}
	return path, nil
}

func validateWorktreeOverlap(ctx context.Context, root, path string) error {
	trees, err := readGitWorktrees(ctx, root)
	if err != nil {
		return err
	}
	for _, tree := range trees {
		if tree.Current {
			continue // The dedicated .worktrees directory belongs to this checkout.
		}
		canonical, err := filepath.EvalSymlinks(tree.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(canonical, path)
		if err != nil {
			return err
		}
		if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			// A selected checkout may itself live in an ancestor's ignored
			// .worktrees directory. Its own .worktrees remains isolated too.
			selected, err := filepath.Rel(canonical, root)
			if err == nil && strings.HasPrefix(selected, ".worktrees"+string(filepath.Separator)) {
				continue
			}
			return fmt.Errorf("create the worktree outside existing worktrees")
		}
	}
	return nil
}

func prepareWorktreeDirectory(ctx context.Context, root string) error {
	if err := addGitLocalExclude(ctx, root, "/.worktrees/"); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(root, ".worktrees"), 0755)
}

func addGitLocalExclude(ctx context.Context, root, pattern string) error {
	common, err := gitRead(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	exclude := filepath.Join(strings.TrimSpace(common), "info", "exclude")
	if info, err := os.Lstat(filepath.Dir(exclude)); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("git info must be a directory, not a symlink")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if info, err := os.Lstat(exclude); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("git info/exclude must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(exclude)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !strings.Contains("\n"+string(data)+"\n", "\n"+pattern+"\n") {
		if err := os.MkdirAll(filepath.Dir(exclude), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(exclude, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString("\n" + pattern + "\n")
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func (s *Service) SwitchGitBranch(ctx context.Context, request GitMutationRequest) (*GitMutationResult, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	repository, err := s.gitMutationPreflight(ctx, request, false)
	if err != nil {
		return nil, err
	}
	if repository.Dirty {
		return nil, fmt.Errorf("commit or move local changes before switching branches")
	}
	branches, err := readGitBranches(ctx, repository.Root, nil)
	if err != nil {
		return nil, err
	}
	found := false
	for _, branch := range branches {
		if branch.Name == request.TargetBranch {
			found = true
		}
	}
	if !found || request.TargetBranch == repository.Branch {
		return nil, fmt.Errorf("choose a different existing local branch")
	}
	s.invalidateJobsForProjectChange()
	_, err = gitRead(ctx, repository.Root, "switch", "--no-guess", "--", request.TargetBranch)
	return s.completeGitMutation(repository.Root, "switched", err), nil
}

// Even a failed or canceled Git command may have written source. Reindex and
// return the observed outcome instead of presenting the attempt as a no-op.
func (s *Service) completeGitMutation(root, state string, cause error) *GitMutationResult {
	result := &GitMutationResult{State: state, Warnings: []string{}}
	if cause != nil {
		result.State, result.Error = "failed", cause.Error()
	}
	index, err := s.reindexActiveProject()
	if err != nil {
		result.Warnings = append(result.Warnings, "Refresh project facts: "+err.Error())
	} else {
		result.Index = index
	}
	repository, err := readGitRepository(context.Background(), root)
	if err != nil {
		result.Warnings = append(result.Warnings, "Refresh Git status: "+err.Error())
		return result
	}
	if index != nil {
		repository.ProjectID, repository.ProjectRevision = index.ProjectID, index.ProjectRevision
	}
	result.Repository = repository
	if len(repository.Conflicts) > 0 {
		result.State = "conflicts"
	}
	return result
}
