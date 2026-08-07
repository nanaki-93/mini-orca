package handlers

import "time"

// ChatRequest represents a chat message request from the client.
type ChatRequest struct {
	Message    string `json:"message"`
	FilePath   string `json:"file_path,omitempty"`   // optional: file being edited
	LineNumber int    `json:"line_number,omitempty"` // optional: cursor position
}

// ChatResponse represents a chat message response from the system.
type ChatResponse struct {
	Role      string    `json:"role"` // "assistant" or "system"
	Content   string    `json:"content"`
	Phase     string    `json:"phase"` // "coding", "testing", "review"
	Timestamp time.Time `json:"timestamp"`
}
