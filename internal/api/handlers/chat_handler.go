package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ChatHandler serves file-scoped conversations.
type ChatHandler struct {
	service *app.Service
}

func NewChatHandler(service *app.Service) *ChatHandler {
	return &ChatHandler{service: service}
}

// OpenSession creates a session whose file, revision, hash, mode, and symbol
// cannot be changed by later messages.
func (h *ChatHandler) OpenSession(w http.ResponseWriter, r *http.Request) {
	var request ChatSessionRequest
	if err := api.DecodeJSON(w, r, &request); err != nil {
		api.WriteRequestError(w, err, "invalid chat session request", "Provide the selected project, file, and symbol identity.")
		return
	}
	session, err := h.service.OpenChatSession(app.ChatSessionCreateRequest{
		ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
		BaseFileHash: request.BaseFileHash, OpenPath: request.OpenPath,
		Mode: request.Mode, TargetSymbol: request.TargetSymbol,
		TaskSpec: request.TaskSpec,
	})
	if err != nil {
		writeChatError(w, "open chat session failed", err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, session)
}

// SendSessionMessage asks for one declaration-only proposal. The request has
// no mutable target fields, so it cannot retarget the active session.
func (h *ChatHandler) SendSessionMessage(w http.ResponseWriter, r *http.Request) {
	var request ChatMessageRequest
	if err := api.DecodeJSON(w, r, &request); err != nil {
		api.WriteRequestError(w, err, "invalid chat message request", "Provide one message for the active file-scoped session.")
		return
	}
	if strings.TrimSpace(request.Message) == "" {
		api.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}
	session, err := h.service.ChatSession(r.PathValue("sessionID"))
	if err != nil {
		writeChatError(w, "chat session unavailable", err)
		return
	}
	if session.State != "active" {
		api.WriteError(w, http.StatusConflict, "project or file changed; open a new chat session")
		return
	}
	proposal, err := h.service.SendChatSessionMessage(r.Context(), app.ChatSessionMessageRequest{
		SessionID: r.PathValue("sessionID"), Message: request.Message, ParentDraftID: request.ParentDraftID,
		ConfirmRemoteProvider: request.ConfirmRemoteProvider, Repair: request.Repair,
	})
	if err != nil {
		writeChatError(w, "generate declaration failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, proposal)
}

func writeChatError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, project.ErrRevisionConflict) {
		api.WriteError(w, http.StatusConflict, "project or file changed; open a new chat session")
		return
	}
	if errors.Is(err, context.Canceled) {
		api.WriteError(w, http.StatusRequestTimeout, "generation canceled")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		api.WriteError(w, http.StatusGatewayTimeout, "generation timed out")
		return
	}
	api.WriteAppError(w, api.BadRequest(operation, "Review the request and try again.", err))
}
