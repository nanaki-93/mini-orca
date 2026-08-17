package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ChatHandler manages chat message and history endpoints.
type ChatHandler struct {
	service *app.Service
}

// NewChatHandler creates a new ChatHandler instance.
func NewChatHandler(service *app.Service) *ChatHandler {
	return &ChatHandler{
		service: service,
	}
}

// SendMessage handles POST /api/chat/message
// Sends a message to the chat, triggers the pipeline, and returns the response.
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ChatRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Message == "" {
		api.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}

	if req.FilePath == "" {
		api.WriteError(w, http.StatusBadRequest, "a file must be open to send a message")
		return
	}
	if strings.TrimSpace(req.TargetSymbol) == "" {
		api.WriteError(w, http.StatusBadRequest, "target_symbol is required (function or class name)")
		return
	}
	if len(req.TargetSymbol) > 200 || strings.ContainsAny(req.TargetSymbol, "\r\n") {
		api.WriteError(w, http.StatusBadRequest, "target_symbol is invalid")
		return
	}
	if err := h.service.ValidateMutableRequest(req.ProjectID, req.ProjectRevision, req.FilePath, req.BaseFileHash); err != nil {
		if errors.Is(err, project.ErrRevisionConflict) {
			api.WriteError(w, http.StatusConflict, "project or file changed; reload before generating")
			return
		}
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = h.service.RecordActivity(req.ProjectID, req.ProjectRevision, project.Activity{
		Role: "user", Content: "Requested focused generation", Phase: "coding", TargetFile: req.FilePath, TargetSymbol: req.TargetSymbol,
	})

	result, err := h.service.Generate(r.Context(), req.Message, req.FilePath, req.TargetSymbol, req.ConfirmRemoteProvider)
	if err != nil {
		_ = h.service.RecordActivity(req.ProjectID, req.ProjectRevision, project.Activity{
			Role: "assistant", Content: "Generation failed", Phase: "error", TargetFile: req.FilePath, TargetSymbol: req.TargetSymbol,
		})
		if errors.Is(err, context.Canceled) {
			api.WriteError(w, http.StatusRequestTimeout, "generation canceled")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			api.WriteError(w, http.StatusGatewayTimeout, "generation timed out")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = h.service.RecordActivity(req.ProjectID, req.ProjectRevision, project.Activity{
		Role: "assistant", Content: "Generated preview", Phase: "coding", TargetFile: req.FilePath, TargetSymbol: req.TargetSymbol,
	})

	api.WriteJSON(w, http.StatusOK, result)
}

// GetHistory handles GET /api/chat/history
// Returns the conversation history.
func (h *ChatHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	history, err := h.service.Activity()
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "project activity not found")
		return
	}
	api.WriteJSON(w, http.StatusOK, history)
}
