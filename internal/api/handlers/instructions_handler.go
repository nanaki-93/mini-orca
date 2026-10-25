package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type InstructionPreview = app.InstructionPreview

func (h *ChangeHandler) Instructions(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	path := r.URL.Query().Get("path")
	if filepath.Base(path) != "AGENTS.md" {
		api.WriteError(w, http.StatusBadRequest, "choose a project-relative AGENTS.md path")
		return
	}
	preview, err := h.service.PreviewInstructions(path)
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, preview)
}

func (h *ChangeHandler) ProposeInstructions(w http.ResponseWriter, r *http.Request) {
	var request app.InstructionProposalRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.ProposeInstructions(r.Context(), request)
	respondWorkflow(w, value, err)
}
