package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func newDraftID() string {
	return newOpaqueID("draft")
}

func newChatSessionID() string {
	return newOpaqueID("chat")
}

func newOpaqueID(prefix string) string {
	var token [16]byte
	if _, err := rand.Read(token[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(token[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

// projectSnapshot identifies exactly one indexed project state.
type projectSnapshot struct {
	id       string
	revision string
}

func newProjectSnapshot(id, revision string) (projectSnapshot, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(revision) == "" {
		return projectSnapshot{}, fmt.Errorf("project_id and project_revision are required")
	}
	return projectSnapshot{id: id, revision: revision}, nil
}

func (snapshot projectSnapshot) matches(id, revision string) bool {
	return snapshot.id == id && snapshot.revision == revision
}

// selectedFile identifies the one source file and bytes a draft may change.
type selectedFile struct {
	path     string
	baseHash string
}

func newSelectedFile(path, baseHash string) (selectedFile, error) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(baseHash) == "" {
		return selectedFile{}, fmt.Errorf("target_path and base_file_hash are required")
	}
	return selectedFile{path: path, baseHash: baseHash}, nil
}

func (file selectedFile) matches(path, baseHash string) bool {
	return file.path == path && file.baseHash == baseHash
}

// taskTarget combines a project snapshot, selected file, and immutable Go
// declaration target. Its comparison is the full stale-write rule.
type taskTarget struct {
	project projectSnapshot
	file    selectedFile
	mode    project.DeclarationEditMode
	symbol  string
}

func newTaskTarget(id, revision, path, baseHash string, mode project.DeclarationEditMode, symbol string) (taskTarget, error) {
	snapshot, err := newProjectSnapshot(id, revision)
	if err != nil {
		return taskTarget{}, err
	}
	file, err := newSelectedFile(path, baseHash)
	if err != nil {
		return taskTarget{}, err
	}
	if mode == "" || strings.TrimSpace(symbol) == "" {
		return taskTarget{}, fmt.Errorf("draft mode and target_symbol are required")
	}
	return taskTarget{project: snapshot, file: file, mode: mode, symbol: symbol}, nil
}

func taskTargetForDraft(draft Draft) (taskTarget, error) {
	return newTaskTarget(draft.ProjectID, draft.ProjectRevision, draft.TargetPath, draft.BaseFileHash, draft.Mode, draft.TargetSymbol)
}

func (target taskTarget) matchesDraft(draft Draft) bool {
	return target.project.matches(draft.ProjectID, draft.ProjectRevision) &&
		target.file.matches(draft.TargetPath, draft.BaseFileHash) &&
		target.mode == draft.Mode && target.symbol == draft.TargetSymbol
}

// draftRevisionIdentity binds a reviewer-visible draft revision and hash.
type draftRevisionIdentity struct {
	id       string
	revision int64
	hash     string
}

func newDraftRevisionIdentity(id string, revision int64, hash string) (draftRevisionIdentity, error) {
	if strings.TrimSpace(id) == "" || revision < 1 || strings.TrimSpace(hash) == "" {
		return draftRevisionIdentity{}, fmt.Errorf("draft_id, draft_revision, and draft_hash are required")
	}
	return draftRevisionIdentity{id: id, revision: revision, hash: hash}, nil
}

func (identity draftRevisionIdentity) matches(draft Draft) bool {
	return identity.id == draft.ID && identity.revision == draft.Revision && identity.hash == draft.Hash
}

// applyIdentity is the validated identity supplied by the stable Apply wire
// DTO. The declaration target is verified against the stored draft.
type applyIdentity struct {
	project      projectSnapshot
	baseFileHash string
	draft        draftRevisionIdentity
}

func newApplyIdentity(request ApplyRequest) (applyIdentity, error) {
	project, err := newProjectSnapshot(request.ProjectID, request.ProjectRevision)
	if err != nil {
		return applyIdentity{}, err
	}
	if strings.TrimSpace(request.BaseFileHash) == "" {
		return applyIdentity{}, fmt.Errorf("base_file_hash is required")
	}
	draft, err := newDraftRevisionIdentity(request.DraftID, request.DraftRevision, request.DraftHash)
	if err != nil {
		return applyIdentity{}, err
	}
	return applyIdentity{project: project, baseFileHash: request.BaseFileHash, draft: draft}, nil
}

func (identity applyIdentity) matchesDraft(draft Draft) bool {
	return identity.project.matches(draft.ProjectID, draft.ProjectRevision) &&
		identity.baseFileHash == draft.BaseFileHash && identity.draft.matches(draft)
}
