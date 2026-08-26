package handlers

import "github.com/nanaki-93/mini-orca/v2/internal/project"

// ChatSessionRequest creates one conversation pinned to an open Go file and
// one immutable declaration target.
type ChatSessionRequest struct {
	ProjectID       string                      `json:"project_id"`
	ProjectRevision string                      `json:"project_revision"`
	BaseFileHash    string                      `json:"base_file_hash"`
	OpenPath        string                      `json:"open_path"`
	Mode            project.DeclarationEditMode `json:"mode"`
	TargetSymbol    string                      `json:"target_symbol"`
}

// ChatMessageRequest cannot contain a file, symbol, mode, or project identity.
// Those values are immutable properties of the session.
type ChatMessageRequest struct {
	Message               string `json:"message"`
	ParentDraftID         string `json:"parent_draft_id,omitempty"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}
