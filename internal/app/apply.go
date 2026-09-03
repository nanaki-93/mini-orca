package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	applyStateRelativePath = ".mini-orca/sessions/apply-state.json"
	auditRelativePath      = ".mini-orca/sessions/audit.json"
)

type ApplyRequest struct {
	DraftID         string `json:"draft_id"`
	DraftRevision   int64  `json:"draft_revision"`
	DraftHash       string `json:"draft_hash"`
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	BaseFileHash    string `json:"base_file_hash"`
	Confirm         bool   `json:"confirm"`
}

type UndoRequest struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	PostApplyHash   string `json:"post_apply_hash"`
	Confirm         bool   `json:"confirm"`
}

// AuditEntry is durable source-free evidence of one requested mutation.
type AuditEntry struct {
	ID              string       `json:"id"`
	Action          string       `json:"action"`
	TargetPath      string       `json:"target_path"`
	GenerationID    string       `json:"generation_id,omitempty"`
	BeforeHash      string       `json:"before_hash"`
	AfterHash       string       `json:"after_hash"`
	ProjectID       string       `json:"project_id"`
	ProjectRevision string       `json:"project_revision"`
	Model           string       `json:"model,omitempty"`
	Profile         string       `json:"profile,omitempty"`
	Validation      bool         `json:"validation_passed"`
	Checks          []CheckAudit `json:"checks"`
	Outcome         string       `json:"outcome"`
	Timestamp       time.Time    `json:"timestamp"`
}

// CheckAudit omits command output because it can contain source.
type CheckAudit struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	State    string `json:"state"`
	ExitCode int    `json:"exit_code,omitempty"`
}

type applyState struct {
	ProjectID  string    `json:"project_id"`
	TargetPath string    `json:"target_path"`
	BeforeHash string    `json:"before_hash"`
	AfterHash  string    `json:"after_hash"`
	BackupPath string    `json:"backup_path"`
	AuditID    string    `json:"audit_id"`
	AppliedAt  time.Time `json:"applied_at"`
}

type ApplyResult struct {
	Audit           AuditEntry            `json:"audit"`
	ProjectRevision string                `json:"project_revision"`
	PostApplyHash   string                `json:"post_apply_hash"`
	UndoAvailable   bool                  `json:"undo_available"`
	Index           *project.ProjectIndex `json:"index"`
}

// ApplyDraft is the only retained source-write path. It accepts identity and
// confirmation, never source text, and rechecks the source immediately before
// the atomic replacement.
func (s *Service) ApplyDraft(ctx context.Context, request ApplyRequest) (*ApplyResult, error) {
	identity, err := validateApplyRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	current, err := s.loadCurrentApplyProject(identity)
	if err != nil {
		return nil, err
	}
	state, err := s.loadStoredDraftForApply(identity)
	if err != nil {
		return nil, err
	}
	current, err = s.loadCurrentApplyFile(current, state.target)
	if err != nil {
		return nil, err
	}
	composition, err := validateDraftForApply(state, current.source)
	if err != nil {
		return nil, err
	}
	original, err := s.verifyApplyBeforeWrite(ctx, current, state)
	if err != nil {
		return nil, err
	}
	backupPath, err := writeBackup(current.root, state.target.file.path, original)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(current.path, []byte(composition.Source)); err != nil {
		return nil, err
	}
	return s.persistDraftApply(current, state, composition, backupPath)
}

type currentApplyProject struct {
	root         string
	path         string
	source       []byte
	baseFileHash string
}

type applyDraftState struct {
	draft    Draft
	target   taskTarget
	checks   DraftCheckReport
	identity applyIdentity
}

func validateApplyRequest(ctx context.Context, request ApplyRequest) (applyIdentity, error) {
	if !request.Confirm {
		return applyIdentity{}, fmt.Errorf("explicit apply confirmation is required")
	}
	if err := ctx.Err(); err != nil {
		return applyIdentity{}, err
	}
	return newApplyIdentity(request)
}

func (s *Service) loadCurrentApplyProject(identity applyIdentity) (currentApplyProject, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return currentApplyProject{}, err
	}
	if !identity.project.matches(analysis.ProjectID, analysis.ProjectRevision) {
		return currentApplyProject{}, project.ErrRevisionConflict
	}
	return currentApplyProject{root: s.manager.Root()}, nil
}

func (s *Service) loadStoredDraftForApply(identity applyIdentity) (applyDraftState, error) {
	if _, err := s.requireCurrentDraft(identity.draft.id); err != nil {
		return applyDraftState{}, err
	}
	draft, checks, err := s.drafts.applyState(identity)
	if err != nil {
		return applyDraftState{}, err
	}
	target, err := taskTargetForDraft(draft)
	if err != nil {
		return applyDraftState{}, err
	}
	if !target.project.matches(identity.project.id, identity.project.revision) || target.file.baseHash != identity.baseFileHash {
		return applyDraftState{}, project.ErrRevisionConflict
	}
	return applyDraftState{draft: draft, target: target, checks: checks, identity: identity}, nil
}

func (s *Service) loadCurrentApplyFile(current currentApplyProject, target taskTarget) (currentApplyProject, error) {
	if err := s.ValidateMutableRequest(target.project.id, target.project.revision, target.file.path, target.file.baseHash); err != nil {
		return currentApplyProject{}, err
	}
	path, err := project.ResolveFile(current.root, target.file.path)
	if err != nil {
		return currentApplyProject{}, err
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return currentApplyProject{}, fmt.Errorf("read apply target: %w", err)
	}
	if contentHash(source) != target.file.baseHash {
		return currentApplyProject{}, project.ErrRevisionConflict
	}
	current.path = path
	current.source = source
	current.baseFileHash = target.file.baseHash
	return current, nil
}

func validateDraftForApply(state applyDraftState, source []byte) (project.GoDeclarationComposition, error) {
	composition := project.ComposeGoDeclaration(state.target.file.path, string(source), project.GoDeclarationEdit{
		Mode: state.target.mode, TargetSymbol: state.target.symbol, Declaration: state.draft.Declaration, Imports: state.draft.Imports,
	})
	if !composition.Validation.Applicable || composition.CompositionHash != state.draft.CompositionHash || composition.CompositionHash != state.checks.CompositionHash {
		return project.GoDeclarationComposition{}, project.ErrRevisionConflict
	}
	return composition, nil
}

func (s *Service) verifyApplyBeforeWrite(ctx context.Context, current currentApplyProject, state applyDraftState) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := s.loadCurrentApplyProject(state.identity); err != nil {
		return nil, err
	}
	latest, err := s.loadStoredDraftForApply(state.identity)
	if err != nil {
		return nil, err
	}
	if !state.target.matchesDraft(latest.draft) || latest.draft.CompositionHash != state.draft.CompositionHash || latest.checks.CompositionHash != state.checks.CompositionHash {
		return nil, project.ErrRevisionConflict
	}
	if err := s.ValidateMutableRequest(state.target.project.id, state.target.project.revision, state.target.file.path, state.target.file.baseHash); err != nil {
		return nil, err
	}
	source, err := os.ReadFile(current.path)
	if err != nil {
		return nil, fmt.Errorf("re-read apply target: %w", err)
	}
	if contentHash(source) != current.baseFileHash {
		return nil, project.ErrRevisionConflict
	}
	return source, nil
}

func (s *Service) persistDraftApply(current currentApplyProject, state applyDraftState, composition project.GoDeclarationComposition, backupPath string) (*ApplyResult, error) {
	postHash := composition.CompositionHash
	index, err := s.Reindex()
	if err != nil {
		return nil, err
	}
	audit := AuditEntry{ID: "apply-" + shortHash(postHash), Action: "apply", TargetPath: state.target.file.path, GenerationID: state.draft.ID, BeforeHash: state.target.file.baseHash, AfterHash: postHash, ProjectID: state.draft.ProjectID, ProjectRevision: index.ProjectRevision, Model: state.draft.EffectiveModel.Model, Profile: state.draft.EffectiveModel.Profile, Validation: true, Checks: auditChecks(state.checks.Checks), Outcome: "applied", Timestamp: time.Now().UTC()}
	if err := appendAudit(current.root, audit); err != nil {
		return nil, err
	}
	if err := writeApplyState(current.root, applyState{ProjectID: state.draft.ProjectID, TargetPath: state.target.file.path, BeforeHash: state.target.file.baseHash, AfterHash: postHash, BackupPath: backupPath, AuditID: audit.ID, AppliedAt: audit.Timestamp}); err != nil {
		return nil, err
	}
	return &ApplyResult{Audit: audit, ProjectRevision: index.ProjectRevision, PostApplyHash: postHash, UndoAvailable: true, Index: index}, nil
}

// UndoDraft restores only the last unchanged applied file.
func (s *Service) UndoDraft(ctx context.Context, request UndoRequest) (*ApplyResult, error) {
	if !request.Confirm {
		return nil, fmt.Errorf("explicit undo confirmation is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state, err := readApplyState(s.manager.Root())
	if err != nil {
		return nil, err
	}
	if state == nil || state.ProjectID != request.ProjectID || state.AfterHash != request.PostApplyHash {
		return nil, project.ErrRevisionConflict
	}
	if err := s.ValidateMutableRequest(request.ProjectID, request.ProjectRevision, state.TargetPath, request.PostApplyHash); err != nil {
		return nil, err
	}
	path, err := project.ResolveFile(s.manager.Root(), state.TargetPath)
	if err != nil {
		return nil, err
	}
	backup, err := os.ReadFile(state.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("read apply backup: %w", err)
	}
	if contentHash(backup) != state.BeforeHash {
		return nil, fmt.Errorf("apply backup integrity check failed")
	}
	if err := atomicWrite(path, backup); err != nil {
		return nil, err
	}
	index, err := s.Reindex()
	if err != nil {
		return nil, err
	}
	audit := AuditEntry{ID: "undo-" + shortHash(state.BeforeHash), Action: "undo", TargetPath: state.TargetPath, BeforeHash: state.AfterHash, AfterHash: state.BeforeHash, ProjectID: request.ProjectID, ProjectRevision: index.ProjectRevision, Validation: true, Outcome: "undone", Timestamp: time.Now().UTC()}
	if err := appendAudit(s.manager.Root(), audit); err != nil {
		return nil, err
	}
	if err := os.Remove(filepath.Join(s.manager.Root(), applyStateRelativePath)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &ApplyResult{Audit: audit, ProjectRevision: index.ProjectRevision, PostApplyHash: state.BeforeHash, UndoAvailable: false, Index: index}, nil
}

func writeBackup(root, target string, content []byte) (string, error) {
	directory := filepath.Join(root, ".mini-orca", "backups")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	name := shortHash(contentHash([]byte(target))) + "-" + shortHash(contentHash(content)) + ".bak"
	path := filepath.Join(directory, name)
	if err := atomicWrite(path, content); err != nil {
		return "", err
	}
	return path, nil
}

func atomicWrite(path string, content []byte) error {
	info, err := os.Stat(path)
	mode := os.FileMode(0600)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".mini-orca-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err == nil {
		defer directory.Close()
		if err := directory.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func writeApplyState(root string, state applyState) error {
	return writeJSONAtomic(filepath.Join(root, applyStateRelativePath), state)
}
func readApplyState(root string) (*applyState, error) {
	var state applyState
	if err := readJSON(filepath.Join(root, applyStateRelativePath), &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}
func appendAudit(root string, entry AuditEntry) error {
	entries, err := readAudit(root)
	if err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(root, auditRelativePath), append(entries, entry))
}
func readAudit(root string) ([]AuditEntry, error) {
	var entries []AuditEntry
	if err := readJSON(filepath.Join(root, auditRelativePath), &entries); err != nil {
		if os.IsNotExist(err) {
			return []AuditEntry{}, nil
		}
		return nil, err
	}
	return entries, nil
}
func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return storage.WriteFile(path, data, 0600)
}
func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
func auditChecks(checks []DraftCheck) []CheckAudit {
	result := make([]CheckAudit, len(checks))
	for i, check := range checks {
		result[i] = CheckAudit{Name: check.Name, Required: check.Required, State: check.State, ExitCode: check.ExitCode}
	}
	return result
}
func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func shortHash(hash string) string {
	if len(hash) > 15 {
		return hash[len(hash)-12:]
	}
	return hash
}
