package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ChangeVerification struct {
	SchemaVersion   int          `json:"schema_version"`
	SessionID       string       `json:"session_id"`
	ProjectID       string       `json:"project_id"`
	ProjectRevision string       `json:"project_revision"`
	ProposalHash    string       `json:"proposal_hash"`
	WorkspaceHash   string       `json:"workspace_hash"`
	Status          string       `json:"status"`
	Reason          string       `json:"reason,omitempty"`
	Checks          []DraftCheck `json:"checks"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

func (s *Service) VerifyChange(ctx context.Context, id string, identity ChangeIdentity) (*ChangeVerification, error) {
	return s.verifyChangeWithChecks(ctx, id, identity, s.verifyAppliedWorkspace)
}

// Execution happens outside workflow locks. Publication repeats the complete
// applied identity check, so Undo, another Apply or unrelated edits discard it.
func (s *Service) verifyChangeWithChecks(ctx context.Context, id string, identity ChangeIdentity, run func(context.Context, string, *changeJournal, string) ([]DraftCheck, error)) (*ChangeVerification, error) {
	root := s.manager.Root()
	journal, err := s.appliedChangeForVerification(ctx, root, id, identity)
	if err != nil {
		return nil, err
	}
	if changeHasGo(journal.Session.Changes) {
		if err := s.requireProjectExecutionTrust(identity.ProjectRevision); err != nil {
			return nil, err
		}
	}
	checks, checkErr := run(ctx, root, journal, identity.ProjectRevision)
	evidence := &ChangeVerification{SchemaVersion: 1, SessionID: id, ProjectID: identity.ProjectID, ProjectRevision: identity.ProjectRevision, ProposalHash: identity.Hash, WorkspaceHash: journal.PostWorkspaceHash, Checks: append([]DraftCheck{}, checks...), UpdatedAt: time.Now().UTC()}
	evidence.Status, evidence.Reason = verificationOutcome(checks, checkErr)
	// Record interrupted checks as canceled without inheriting canceled source
	// identity work. This bounded read/write cleanup never executes project code.
	guard, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	current, err := s.appliedChangeForVerification(guard, root, id, identity)
	if err != nil {
		return nil, err
	}
	current.Verification = evidence
	if err := writeChangeJournal(root, current); err != nil {
		return nil, fmt.Errorf("persist verification: %w", err)
	}
	return evidence, nil
}

func (s *Service) appliedChangeForVerification(ctx context.Context, root, id string, identity ChangeIdentity) (*changeJournal, error) {
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if s.manager.Root() != root || index.ProjectID != identity.ProjectID || index.ProjectRevision != identity.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	journal, err := readChangeJournal(root)
	if err != nil {
		return nil, err
	}
	if !matchesAppliedChange(journal, id, identity) {
		return nil, project.ErrRevisionConflict
	}
	if err := validateChangeRestore(root, journal); err != nil {
		return nil, err
	}
	if err := (benchmarkFixture{fingerprint: journal.PostWorkspaceHash}).verifyCurrent(ctx, root); err != nil {
		return nil, err
	}
	return journal, nil
}

func matchesAppliedChange(journal *changeJournal, id string, identity ChangeIdentity) bool {
	return journal != nil && journal.State == "applied" && journal.Session.ID == id && journal.Session.ProjectID == identity.ProjectID && journal.Session.Hash == identity.Hash && journal.Session.Revision == identity.Revision && journal.PostWorkspaceHash != ""
}

func changeHasGo(edits []ChangeEdit) bool {
	for _, edit := range edits {
		if filepath.Ext(edit.Path) == ".go" {
			return true
		}
	}
	return false
}

func (s *Service) verifyAppliedWorkspace(ctx context.Context, root string, journal *changeJournal, revision string) ([]DraftCheck, error) {
	checks := changeSourceChecks(journal.Session.Changes)
	if !changeHasGo(journal.Session.Changes) || !requiredChecksPassed(checks) {
		return checks, ctx.Err()
	}
	workspace, err := os.MkdirTemp("", "mini-orca-change-verify-")
	if err != nil {
		return checks, err
	}
	defer os.RemoveAll(workspace)
	if err := copyCheckWorkspace(ctx, root, workspace); err != nil {
		return checks, err
	}
	fingerprint, err := fingerprintCheckWorkspace(ctx, workspace)
	if err != nil {
		return checks, err
	}
	if fingerprint != journal.PostWorkspaceHash {
		return checks, project.ErrRevisionConflict
	}
	checks = append(checks, s.runCheck(ctx, workspace, revision, "post-Apply tests", true, []string{"go", "test", "./..."}, false, true))
	checks = append(checks, s.runCheck(ctx, workspace, revision, "post-Apply vet", true, []string{"go", "vet", "./..."}, false, false))
	return checks, ctx.Err()
}

func verificationOutcome(checks []DraftCheck, err error) (string, string) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "canceled", "Verification was interrupted before completion."
	}
	if err != nil {
		return "unavailable", "Verification could not complete. Refresh project facts and retry."
	}
	if len(checks) == 0 {
		return "unavailable", "No verification checks completed."
	}
	for _, check := range checks {
		if check.State == CheckCanceled {
			return "canceled", "A verification check was interrupted."
		}
	}
	if !requiredChecksPassed(checks) {
		return "failed", "Applied files remain in place; one or more verification checks failed."
	}
	return "verified", ""
}

func (s *Service) currentChangeVerification(ctx context.Context, root string, journal *changeJournal, index *project.ProjectIndex) (*ChangeVerification, error) {
	if journal.Verification == nil {
		return nil, nil
	}
	evidence := *journal.Verification
	if evidence.SchemaVersion != 1 || evidence.SessionID != journal.Session.ID || evidence.ProposalHash != journal.Session.Hash || evidence.WorkspaceHash != journal.PostWorkspaceHash {
		return nil, fmt.Errorf("invalid stored verification identity")
	}
	if evidence.ProjectID != index.ProjectID || evidence.ProjectRevision != index.ProjectRevision || journal.State != "applied" {
		evidence.Status, evidence.Reason = "stale", "Project identity changed after verification."
		return &evidence, nil
	}
	if err := (benchmarkFixture{fingerprint: evidence.WorkspaceHash}).verifyCurrent(ctx, root); err != nil {
		if !errors.Is(err, project.ErrRevisionConflict) {
			return nil, err
		}
		evidence.Status, evidence.Reason = "stale", "Source changed after verification."
	}
	return &evidence, nil
}
