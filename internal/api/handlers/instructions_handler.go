package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type InstructionPreview struct {
	ProjectID       string                        `json:"project_id"`
	ProjectRevision string                        `json:"project_revision"`
	Path            string                        `json:"path"`
	ExistingContent string                        `json:"existing_content"`
	Exists          bool                          `json:"exists"`
	Effective       project.EffectiveInstructions `json:"effective"`
	Presets         []project.InstructionPreset   `json:"presets"`
}

func (h *ChangeHandler) Instructions(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	path := r.URL.Query().Get("path")
	if filepath.Base(path) != "AGENTS.md" {
		api.WriteError(w, http.StatusBadRequest, "choose a project-relative AGENTS.md path")
		return
	}
	index, err := h.manager.Index()
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	root := h.manager.Root()
	effective, err := project.ResolveInstructions(root, path)
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	presets, err := project.SuggestInstructionPresets(root, path, index.Files)
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	current, err := h.manager.Index()
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	if h.manager.Root() != root || current.ProjectID != index.ProjectID || current.ProjectRevision != index.ProjectRevision {
		respondWorkflow(w, nil, project.ErrRevisionConflict)
		return
	}
	preview := InstructionPreview{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Path: path, Effective: effective, Presets: presets}
	for _, file := range effective.Files {
		if file.Path == path {
			preview.Exists, preview.ExistingContent = true, file.Content
		}
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
