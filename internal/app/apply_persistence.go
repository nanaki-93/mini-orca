package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const pendingApplyRelativePath = ".mini-orca/sessions/pending-apply.json"

// The journal is written before source replacement. Its expected after hash
// makes an interrupted write distinguishable from an applied change on restart.
type applyJournal struct {
	State    applyState `json:"state"`
	Audit    AuditEntry `json:"audit"`
	previous []byte
}

func prepareApplyJournal(current currentApplyProject, state applyDraftState, composition project.GoDeclarationComposition, backupPath string) (applyJournal, error) {
	if _, err := readAudit(current.root); err != nil {
		return applyJournal{}, fmt.Errorf("audit is unreadable; source was not changed: %w", err)
	}
	audit := AuditEntry{ID: "apply-" + shortHash(composition.CompositionHash), Action: "apply", TargetPath: state.target.file.path, GenerationID: state.draft.ID, BeforeHash: state.target.file.baseHash, AfterHash: composition.CompositionHash, ProjectID: state.draft.ProjectID, ProjectRevision: state.draft.ProjectRevision, Model: state.draft.EffectiveModel.Model, Profile: state.draft.EffectiveModel.Profile, Validation: true, Checks: auditChecks(state.checks.Checks), Outcome: "applied", Timestamp: time.Now().UTC()}
	journal := applyJournal{
		Audit: audit,
		State: applyState{ProjectID: audit.ProjectID, TargetPath: audit.TargetPath, BeforeHash: audit.BeforeHash, AfterHash: audit.AfterHash, BackupPath: backupPath, AuditID: audit.ID, AppliedAt: audit.Timestamp},
	}
	previous, err := os.ReadFile(filepath.Join(current.root, pendingApplyRelativePath))
	if err != nil && !os.IsNotExist(err) {
		return applyJournal{}, fmt.Errorf("read previous Apply recovery journal: %w", err)
	}
	journal.previous = previous
	if err := writeJSONAtomic(filepath.Join(current.root, pendingApplyRelativePath), journal); err != nil {
		return applyJournal{}, fmt.Errorf("save Apply recovery journal; source was not changed: %w", err)
	}
	return journal, nil
}

func restorePendingApply(root string, journal applyJournal) error {
	path := filepath.Join(root, pendingApplyRelativePath)
	if journal.previous != nil {
		return storage.WriteFile(path, journal.previous, 0600)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unapplied recovery journal: %w", err)
	}
	return nil
}

// Lifecycle ownership spans source mutation and completion. A metadata failure
// after replacement must keep the receipt and recovery journal available.
func (s *Service) persistDraftApply(root string, journal applyJournal, writeErr error) *ApplyResult {
	result := s.mutationReceipt(journal.Audit, true, writeErr)
	if err := appendAudit(root, result.Audit); err != nil {
		result.Warnings = append(result.Warnings, "Source changed, but the audit could not be saved: "+err.Error())
	}
	if err := writeApplyState(root, journal.State); err != nil {
		result.Warnings = append(result.Warnings, "The normal Undo receipt could not be saved; Undo uses the recovery journal: "+err.Error())
	}
	if len(result.Warnings) == 0 {
		removeMutationRecord(root, pendingApplyRelativePath, result)
	}
	return result
}

func (s *Service) persistDraftUndo(state *applyState, revision string, writeErr error) *ApplyResult {
	audit := AuditEntry{ID: "undo-" + shortHash(state.BeforeHash), Action: "undo", TargetPath: state.TargetPath, BeforeHash: state.AfterHash, AfterHash: state.BeforeHash, ProjectID: state.ProjectID, ProjectRevision: revision, Validation: true, Outcome: "undone", Timestamp: time.Now().UTC()}
	result := s.mutationReceipt(audit, false, writeErr)
	root := s.manager.Root()
	if err := appendAudit(root, result.Audit); err != nil {
		result.Warnings = append(result.Warnings, "Source was restored, but the audit could not be saved: "+err.Error())
	}
	if removeMutationRecord(root, applyStateRelativePath, result) {
		removeMutationRecord(root, pendingApplyRelativePath, result)
	}
	return result
}

func (s *Service) mutationReceipt(audit AuditEntry, undo bool, writeErr error) *ApplyResult {
	result := &ApplyResult{Audit: audit, ProjectRevision: audit.ProjectRevision, PostApplyHash: audit.AfterHash, UndoAvailable: undo}
	if writeErr != nil {
		result.Warnings = append(result.Warnings, writeErr.Error())
	}
	index, err := s.reindexActiveProject()
	if err != nil {
		result.Warnings = append(result.Warnings, "Source changed, but the project index could not be refreshed. Re-index the project: "+err.Error())
	} else {
		result.Index = index
		result.ProjectRevision = index.ProjectRevision
		result.Audit.ProjectRevision = index.ProjectRevision
	}
	return result
}

func removeMutationRecord(root, relative string, result *ApplyResult) bool {
	if err := os.Remove(filepath.Join(root, relative)); err != nil && !os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, "Source changed, but obsolete recovery metadata could not be removed: "+err.Error())
		return false
	}
	return true
}

func readPendingApplyState(root string) (*applyState, error) {
	var journal applyJournal
	if err := readJSON(filepath.Join(root, pendingApplyRelativePath), &journal); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read Apply recovery journal: %w", err)
	}
	state := journal.State
	path, err := project.ResolveFile(root, state.TargetPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if contentHash(data) != state.AfterHash {
		return nil, project.ErrRevisionConflict
	}
	return &state, nil
}
