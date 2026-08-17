package handlers

import (
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

// ChatRequest represents a chat message request from the client.
type ChatRequest struct {
	Message               string             `json:"message"`
	FilePath              string             `json:"file_path"`     // target file being edited
	TargetSymbol          string             `json:"target_symbol"` // exact function or class to generate
	ScopeMode             workflow.ScopeMode `json:"scope_mode,omitempty"`
	ProjectID             string             `json:"project_id"`
	ProjectRevision       string             `json:"project_revision"`
	BaseFileHash          string             `json:"base_file_hash"`
	LineNumber            int                `json:"line_number,omitempty"` // optional: cursor position
	ConfirmRemoteProvider bool               `json:"confirm_remote_provider,omitempty"`
	Action                workflow.Action    `json:"action,omitempty"`
	TemplateID            string             `json:"template_id,omitempty"`
}

// ChatResponse represents a chat message response from the system.
type ChatResponse struct {
	Role      string    `json:"role"` // "assistant" or "user"
	Content   string    `json:"content"`
	Phase     string    `json:"phase"` // "coding", "testing", "review"
	Timestamp time.Time `json:"timestamp"`
}
