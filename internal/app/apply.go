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
)

const (
	applyStateRelativePath = ".mini-orca/sessions/apply-state.json"
	auditRelativePath      = ".mini-orca/sessions/audit.json"
)

type ApplyRequest struct {
	GenerationID    string `json:"generation_id"`
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
	ID                string       `json:"id"`
	Action            string       `json:"action"`
	TargetPath        string       `json:"target_path"`
	GenerationID      string       `json:"generation_id,omitempty"`
	BeforeHash        string       `json:"before_hash"`
	AfterHash         string       `json:"after_hash"`
	ProjectID         string       `json:"project_id"`
	ProjectRevision   string       `json:"project_revision"`
	Model             string       `json:"model,omitempty"`
	Profile           string       `json:"profile,omitempty"`
	Validation        bool         `json:"validation_passed"`
	Checks            []CheckAudit `json:"checks"`
	Outcome           string       `json:"outcome"`
	Timestamp         time.Time    `json:"timestamp"`
	TemplateID        string       `json:"template_id,omitempty"`
	ActionTemplate    string       `json:"action_template,omitempty"`
	TemplateInputHash string       `json:"template_input_hash,omitempty"`
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

// ApplyCandidate writes one previously validated and checked candidate after
// re-reading the real file. It never accepts source text from the caller.
func (s *Service) ApplyCandidate(ctx context.Context, request ApplyRequest) (*ApplyResult, error) {
	if !request.Confirm {
		return nil, fmt.Errorf("explicit apply confirmation is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	preview, checks, err := s.checkedCandidate(request.GenerationID)
	if err != nil {
		return nil, err
	}
	if !preview.Validation.Applicable || !checks.Applicable {
		return nil, fmt.Errorf("candidate scope validation or required checks did not pass")
	}
	if preview.ProjectID != request.ProjectID || preview.ProjectRevision != request.ProjectRevision || preview.BaseFileHash != request.BaseFileHash {
		return nil, project.ErrRevisionConflict
	}
	if err := s.ValidateMutableRequest(request.ProjectID, request.ProjectRevision, preview.TargetPath, request.BaseFileHash); err != nil {
		return nil, err
	}
	root := s.manager.Root()
	path, err := project.ResolveFile(root, preview.TargetPath)
	if err != nil {
		return nil, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read apply target: %w", err)
	}
	if contentHash(original) != preview.BaseFileHash {
		return nil, project.ErrRevisionConflict
	}
	backupPath, err := writeBackup(root, preview.TargetPath, original)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(path, []byte(preview.CandidateContent)); err != nil {
		return nil, err
	}
	postHash := contentHash([]byte(preview.CandidateContent))
	index, err := s.Reindex()
	if err != nil {
		return nil, err
	}
	audit := AuditEntry{ID: "apply-" + shortHash(postHash), Action: "apply", TargetPath: preview.TargetPath, GenerationID: preview.GenerationID, BeforeHash: preview.BaseFileHash, AfterHash: postHash, ProjectID: preview.ProjectID, ProjectRevision: index.ProjectRevision, Model: preview.EffectiveModel.Model, Profile: preview.EffectiveModel.Profile, Validation: true, Checks: auditChecks(checks.Checks), Outcome: "applied", Timestamp: time.Now().UTC(), TemplateID: preview.TemplateID, ActionTemplate: preview.Action, TemplateInputHash: preview.TemplateInputHash}
	if err := appendAudit(root, audit); err != nil {
		return nil, err
	}
	if err := writeApplyState(root, applyState{ProjectID: preview.ProjectID, TargetPath: preview.TargetPath, BeforeHash: preview.BaseFileHash, AfterHash: postHash, BackupPath: backupPath, AuditID: audit.ID, AppliedAt: audit.Timestamp}); err != nil {
		return nil, err
	}
	return &ApplyResult{Audit: audit, ProjectRevision: index.ProjectRevision, PostApplyHash: postHash, UndoAvailable: true, Index: index}, nil
}

// UndoCandidate restores only the last unchanged applied file.
func (s *Service) UndoCandidate(ctx context.Context, request UndoRequest) (*ApplyResult, error) {
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

func (s *Service) AuditHistory() ([]AuditEntry, error) { return readAudit(s.manager.Root()) }

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
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return atomicWrite(path, data)
}
func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
func auditChecks(checks []CandidateCheck) []CheckAudit {
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
