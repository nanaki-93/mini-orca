package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

var commitOIDPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func validateChangeFindingTargets(session *ChangeSession) error {
	if len(session.FindingTargets) > 16 {
		return fmt.Errorf("at most sixteen finding targets are supported")
	}
	seen := make(map[project.FindingTarget]bool)
	for _, target := range session.FindingTargets {
		if err := target.Validate(); err != nil {
			return err
		}
		if changeTarget(session, target.Path) == nil || seen[target] {
			return fmt.Errorf("finding targets must be distinct and within the captured paths")
		}
		seen[target] = true
	}
	return nil
}

// Capture before merging: merge drivers and later work must not change which
// findings the reviewed commits address. Legacy sessions have no target link.
func mergedFindingTargets(ctx context.Context, repository *GitRepository, source *GitWorktree) ([]project.FindingCompletion, error) {
	history, err := listChangeSessions(source.Path)
	if err != nil {
		return nil, fmt.Errorf("read worktree finding targets: %w", err)
	}
	completed := []project.FindingCompletion{}
	for _, entry := range history {
		if len(entry.FindingTargets) == 0 || entry.State != "applied" || entry.Commit == nil || entry.Commit.State != "committed" {
			continue
		}
		if !commitOIDPattern.MatchString(entry.Commit.OID) {
			return nil, fmt.Errorf("invalid committed fix identity in %s", entry.ID)
		}
		_, err := gitRead(ctx, repository.Root, "merge-base", "--is-ancestor", entry.Commit.OID, source.Head)
		if gitExitCode(err, 1) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("verify finding commit %s: %w", entry.Commit.OID, err)
		}
		for _, target := range entry.FindingTargets {
			if target.ProjectID == repository.ProjectID {
				completed = append(completed, project.FindingCompletion{FindingTarget: target, CommitOID: entry.Commit.OID})
			}
		}
	}
	return completed, nil
}

func completeMergedFindings(ctx context.Context, root, head string, completions []project.FindingCompletion) error {
	if len(completions) == 0 {
		return nil
	}
	if _, err := gitRead(ctx, root, "merge-base", "--is-ancestor", head, "HEAD"); err != nil {
		return fmt.Errorf("verify the reviewed commit was merged: %w", err)
	}
	store, err := project.NewFindingStore(root)
	if err != nil {
		return err
	}
	return store.Complete(completions)
}

func completeAppliedChangeFindings(root string, session *ChangeSession) {
	if !isCommittedChangeSession(session) {
		return
	}
	store, err := project.NewFindingStore(root)
	if err != nil {
		return
	}
	completions := linkedChangeFindingCompletions(session)
	if len(completions) == 0 {
		completions = titleMatchedChangeFindingCompletions(store, session)
	}
	_ = store.Complete(completions)
}

func isCommittedChangeSession(session *ChangeSession) bool {
	return session != nil && session.State == "applied" && session.Commit != nil &&
		session.Commit.State == "committed" && commitOIDPattern.MatchString(session.Commit.OID)
}

func linkedChangeFindingCompletions(session *ChangeSession) []project.FindingCompletion {
	completions := make([]project.FindingCompletion, 0, len(session.FindingTargets))
	for _, target := range session.FindingTargets {
		if target.ProjectID == session.ProjectID {
			completions = append(completions, project.FindingCompletion{FindingTarget: target, CommitOID: session.Commit.OID})
		}
	}
	return completions
}

func titleMatchedChangeFindingCompletions(store *project.FindingStore, session *ChangeSession) []project.FindingCompletion {
	title := strings.TrimSpace(session.Title)
	if title == "" {
		return nil
	}
	findings, err := store.Load(project.FindingInput{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision})
	if err != nil {
		return nil
	}
	var completions []project.FindingCompletion
	for _, finding := range findings {
		if finding.ProjectID == session.ProjectID && finding.Location.Path != "" && changeTarget(session, finding.Location.Path) != nil && strings.EqualFold(strings.TrimSpace(finding.Title), title) {
			completions = append(completions, project.FindingCompletion{
				FindingTarget: project.FindingTarget{ProjectID: finding.ProjectID, Kind: "semantic", ID: finding.ID, Path: finding.Location.Path},
				CommitOID:     session.Commit.OID,
			})
		}
	}
	return completions
}

func (s *Service) FindingCompletions() ([]project.FindingCompletion, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	store, err := project.NewFindingStore(s.manager.Root())
	if err != nil {
		return nil, err
	}
	return store.Completions(index.ProjectID)
}
