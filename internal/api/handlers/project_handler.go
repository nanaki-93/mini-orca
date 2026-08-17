package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ProjectHandler struct {
	manager  *project.Manager
	analyzer *project.Analyzer
}

type projectImportRequest struct {
	ProjectPath string `json:"project_path"`
}

func NewProjectHandler(manager *project.Manager, analyzer *project.Analyzer) *ProjectHandler {
	return &ProjectHandler{manager: manager, analyzer: analyzer}
}

func (h *ProjectHandler) Import(w http.ResponseWriter, r *http.Request) {
	var request projectImportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid request", "A JSON project_path is required.", err))
		return
	}
	analysis, err := h.analyzer.Analyze(r.Context(), request.ProjectPath)
	if err != nil {
		api.WriteAppError(w, apperrors.BadRequest("project import failed", err.Error(), err))
		return
	}
	if err := h.manager.Set(analysis.Path, analysis); err != nil {
		api.WriteAppError(w, apperrors.Internal("project activation failed", "The analysis was created but the project could not be activated.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

func (h *ProjectHandler) Current(w http.ResponseWriter, _ *http.Request) {
	analysis, err := h.manager.Analysis()
	if err != nil {
		api.WriteAppError(w, apperrors.NotFound("project analysis not found", "Import a project to create its analysis.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

func (h *ProjectHandler) FileInfo(w http.ResponseWriter, r *http.Request) {
	info, err := project.GetFileInfo(h.manager.Root(), r.URL.Query().Get("path"))
	if err != nil {
		api.WriteAppError(w, apperrors.BadRequest("file information failed", err.Error(), err))
		return
	}
	api.WriteJSON(w, http.StatusOK, info)
}
