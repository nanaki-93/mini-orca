package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentAttachmentHandler struct{ service *app.Service }

func NewAgentAttachmentHandler(service *app.Service) *AgentAttachmentHandler {
	return &AgentAttachmentHandler{service: service}
}

func (h *AgentAttachmentHandler) Overview(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.AttachmentOverview(r.Context(), query)
	respondArtifact(w, value, err)
}

func (h *AgentAttachmentHandler) Import(w http.ResponseWriter, r *http.Request) {
	var request app.AttachmentImport
	if !decodeStrictJSON(w, r, &request, "invalid attachment import", "Provide the current project identity, name, MIME, base64 data and explicit local import confirmation.") {
		return
	}
	value, err := h.service.ImportAttachment(r.Context(), request)
	respondArtifact(w, value, err)
}

func (h *AgentAttachmentHandler) Advice(w http.ResponseWriter, r *http.Request) {
	var request app.AttachmentAdviceRequest
	if !decodeStrictJSON(w, r, &request, "invalid attachment advice request", "Select exact imported versions, current settings and destination, a question, and media-sharing/provider confirmations.") {
		return
	}
	value, err := h.service.RunAttachmentAdvice(r.Context(), request)
	if errors.Is(err, app.ErrAttachmentAdviceUnavailable) {
		w.Header().Set("Cache-Control", "no-store")
		api.WriteAppError(w, api.NewAppError(api.ErrorInternal, "attachment advice unavailable", "The provider exchange failed. Imported media remains in the private library; no advice was published.", http.StatusBadGateway, err))
		return
	}
	respondArtifact(w, value, err)
}
