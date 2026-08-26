package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ChatHandler serves file-scoped conversations and source-free activity.
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
	if err := decodeChatJSON(r, &request); err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	session, err := h.service.OpenChatSession(app.ChatSessionCreateRequest{
		ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision,
		BaseFileHash: request.BaseFileHash, OpenPath: request.OpenPath,
		Mode: request.Mode, TargetSymbol: request.TargetSymbol,
	})
	if err != nil {
		writeChatError(w, "open chat session failed", err)
		return
	}
	_ = h.service.RecordActivity(session.ProjectID, session.ProjectRevision, project.Activity{
		Role: "user", Content: "Opened file-scoped chat session", Phase: "coding", TargetFile: session.OpenPath, TargetSymbol: session.TargetSymbol,
	})
	api.WriteJSON(w, http.StatusCreated, session)
}

// Session returns messages only for their owning file-scoped session.
func (h *ChatHandler) Session(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.ChatSession(r.PathValue("sessionID"))
	if err != nil {
		writeChatError(w, "chat session unavailable", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, session)
}

// SendSessionMessage asks for one declaration-only proposal. The request has
// no mutable target fields, so it cannot retarget the active session.
func (h *ChatHandler) SendSessionMessage(w http.ResponseWriter, r *http.Request) {
	var request ChatMessageRequest
	if err := decodeChatJSON(r, &request); err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
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
	_ = h.service.RecordActivity(session.ProjectID, session.ProjectRevision, project.Activity{
		Role: "user", Content: "Requested declaration proposal", Phase: "coding", TargetFile: session.OpenPath, TargetSymbol: session.TargetSymbol,
	})
	proposal, err := h.service.SendChatSessionMessage(r.Context(), app.ChatSessionMessageRequest{
		SessionID: r.PathValue("sessionID"), Message: request.Message, ParentDraftID: request.ParentDraftID,
		ConfirmRemoteProvider: request.ConfirmRemoteProvider,
	})
	if err != nil {
		_ = h.service.RecordActivity(session.ProjectID, session.ProjectRevision, project.Activity{
			Role: "assistant", Content: "Declaration proposal failed", Phase: "error", TargetFile: session.OpenPath, TargetSymbol: session.TargetSymbol,
		})
		writeChatError(w, "generate declaration failed", err)
		return
	}
	_ = h.service.RecordActivity(session.ProjectID, session.ProjectRevision, project.Activity{
		Role: "assistant", Content: "Generated declaration draft", Phase: "coding", TargetFile: session.OpenPath, TargetSymbol: session.TargetSymbol,
	})
	api.WriteJSON(w, http.StatusOK, proposal)
}

// Activity returns project-scoped, source-free activity. It is intentionally
// distinct from file-scoped conversation messages.
func (h *ChatHandler) Activity(w http.ResponseWriter, r *http.Request) {
	history, err := h.service.Activity()
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "project activity not found")
		return
	}
	api.WriteJSON(w, http.StatusOK, history)
}

// RemovedMessageEndpoint keeps the previous route discoverable while ensuring
// it cannot bypass file-scoped session identity. Desktop migration follows the
// typed session contract instead of receiving a compatibility full-file draft.
func (h *ChatHandler) RemovedMessageEndpoint(w http.ResponseWriter, r *http.Request) {
	api.WriteError(w, http.StatusGone, "chat message endpoint was replaced by file-scoped sessions")
}

func decodeChatJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
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
	api.WriteError(w, http.StatusBadRequest, operation+": "+err.Error())
}
