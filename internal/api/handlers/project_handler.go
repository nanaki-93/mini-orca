package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ProjectHandler struct {
	manager *project.Manager
	service *app.Service
}

type projectImportRequest struct {
	ProjectPath           string `json:"project_path"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}

type projectRestoreRequest struct {
	ProjectPath string `json:"project_path"`
}

type reindexRequest struct {
	ProjectRevision string `json:"project_revision,omitempty"`
}

type symbolsResponse struct {
	ProjectID       string               `json:"project_id"`
	ProjectRevision string               `json:"project_revision"`
	Path            string               `json:"path"`
	Language        string               `json:"language"`
	Symbols         []project.SymbolInfo `json:"symbols"`
}

type fileAnalysisRequest struct {
	Path                  string `json:"path"`
	ProjectRevision       string `json:"project_revision"`
	Refresh               bool   `json:"refresh,omitempty"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}

type analyzeAllRequest struct {
	ProjectRevision       string `json:"project_revision"`
	MaxFiles              int    `json:"max_files,omitempty"`
	MaxRetries            int    `json:"max_retries,omitempty"`
	ConfirmRemoteProvider bool   `json:"confirm_remote_provider,omitempty"`
}

type findingStatusRequest struct {
	ProjectRevision string `json:"project_revision"`
	Status          string `json:"status"`
}

type goScanRequest struct {
	ProjectRevision string `json:"project_revision"`
}

func NewProjectHandler(manager *project.Manager, service *app.Service) *ProjectHandler {
	return &ProjectHandler{manager: manager, service: service}
}

func (h *ProjectHandler) Import(w http.ResponseWriter, r *http.Request) {
	var request projectImportRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid request", "A JSON project_path is required.", err))
		return
	}
	if err := h.service.RequireRemoteConfirmation(config.AnalyzeModelScope, request.ConfirmRemoteProvider); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("remote provider confirmation required", err.Error(), err))
		return
	}
	analysis, err := h.service.AnalyzeProject(r.Context(), request.ProjectPath)
	if err != nil {
		if writeContextError(w, "project import", err) {
			return
		}
		api.WriteAppError(w, apperrors.BadRequest("project import failed", err.Error(), err))
		return
	}
	h.service.ProjectChanged()
	if err := h.manager.Set(analysis.Path, analysis); err != nil {
		api.WriteAppError(w, apperrors.Internal("project activation failed", "The analysis was created but the project could not be activated.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

func (h *ProjectHandler) Restore(w http.ResponseWriter, r *http.Request) {
	var request projectRestoreRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid request", "A JSON project_path is required.", err))
		return
	}
	analysis, err := h.service.RestoreProject(request.ProjectPath)
	if err != nil {
		api.WriteAppError(w, apperrors.BadRequest("project restore failed", err.Error(), err))
		return
	}
	h.service.ProjectChanged()
	if err := h.manager.Restore(analysis.Path, analysis); err != nil {
		api.WriteAppError(w, apperrors.Internal("project restoration failed", "The stored analysis was loaded but the project could not be activated.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

func writeContextError(w http.ResponseWriter, action string, err error) bool {
	switch {
	case errors.Is(err, context.Canceled):
		api.WriteError(w, http.StatusRequestTimeout, action+" canceled")
	case errors.Is(err, context.DeadlineExceeded):
		api.WriteError(w, http.StatusGatewayTimeout, action+" timed out")
	default:
		return false
	}
	return true
}

func (h *ProjectHandler) Current(w http.ResponseWriter, _ *http.Request) {
	analysis, err := h.manager.Analysis()
	if err != nil {
		api.WriteAppError(w, apperrors.NotFound("project analysis not found", "Import a project to create its analysis.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

// Overview combines source-free deterministic metrics with optional model and
// scan state for the project-level workspaces.
func (h *ProjectHandler) Overview(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	overview, err := h.service.ProjectOverview()
	if err != nil {
		writeProjectError(w, "project overview failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, overview)
}

// Findings lists source-free verified reports and AI suggestions with their
// provenance, confidence, lifecycle, location, and freshness intact.
func (h *ProjectHandler) Findings(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	findings, err := h.service.ListFindings(app.FindingFilter{
		Source: r.URL.Query().Get("source"), Confidence: r.URL.Query().Get("confidence"),
		Severity: r.URL.Query().Get("severity"), Status: r.URL.Query().Get("status"), Freshness: r.URL.Query().Get("freshness"),
	})
	if err != nil {
		writeProjectError(w, "finding lookup failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct {
		ProjectID       string                   `json:"project_id"`
		ProjectRevision string                   `json:"project_revision"`
		Findings        []project.UnifiedFinding `json:"findings"`
	}{ProjectID: findingsProjectID(h.manager), ProjectRevision: r.URL.Query().Get("project_revision"), Findings: findings})
}

// UpdateFindingStatus changes only user triage for one deterministic finding.
func (h *ProjectHandler) UpdateFindingStatus(w http.ResponseWriter, r *http.Request) {
	var request findingStatusRequest
	if !decodeStrictJSON(w, r, &request, "invalid finding triage request", "Provide project_revision and status.") || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	if err := h.service.UpdateFindingStatus(request.ProjectRevision, r.PathValue("findingID"), request.Status); err != nil {
		writeProjectError(w, "update finding triage failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StartGoScan explicitly starts parser, vet, and test verification in an
// isolated copy; no imported source is modified.
func (h *ProjectHandler) StartGoScan(w http.ResponseWriter, r *http.Request) {
	var request goScanRequest
	if !decodeStrictJSON(w, r, &request, "invalid verified scan request", "Provide project_revision.") || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	report, err := h.service.StartGoScan(request.ProjectRevision)
	if err != nil {
		writeProjectError(w, "start verified scan failed", err)
		return
	}
	api.WriteJSON(w, http.StatusAccepted, report)
}

// GoScanProgress reads source-free persisted progress for the active revision.
func (h *ProjectHandler) GoScanProgress(w http.ResponseWriter, r *http.Request) {
	revision := r.URL.Query().Get("project_revision")
	if !h.requireRevision(w, revision) {
		return
	}
	report, err := h.service.GoScanProgress(revision)
	if err != nil {
		writeProjectError(w, "verified scan lookup failed", err)
		return
	}
	if report == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	api.WriteJSON(w, http.StatusOK, report)
}

// CancelGoScan requests cancellation for the active isolated scan.
func (h *ProjectHandler) CancelGoScan(w http.ResponseWriter, r *http.Request) {
	revision := r.URL.Query().Get("project_revision")
	if !h.requireRevision(w, revision) {
		return
	}
	report, err := h.service.CancelGoScan(revision)
	if err != nil {
		writeProjectError(w, "cancel verified scan failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, report)
}

func (h *ProjectHandler) FileInfo(w http.ResponseWriter, r *http.Request) {
	info, err := project.GetFileInfo(h.manager.Root(), r.URL.Query().Get("path"))
	if err != nil {
		api.WriteAppError(w, apperrors.BadRequest("file information failed", err.Error(), err))
		return
	}
	api.WriteJSON(w, http.StatusOK, info)
}

// Index returns deterministic facts for the active project. The index contains
// only context-policy-eligible, project-relative paths and never source text.
func (h *ProjectHandler) Index(w http.ResponseWriter, _ *http.Request) {
	index, err := h.manager.Index()
	if err != nil {
		writeProjectError(w, "project index unavailable", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, index)
}

// Symbols returns valid atomic targets for one policy-eligible project file.
func (h *ProjectHandler) Symbols(w http.ResponseWriter, r *http.Request) {
	file, err := h.manager.IndexedFile(r.URL.Query().Get("path"))
	if err != nil {
		writeProjectError(w, "symbol lookup failed", err)
		return
	}
	if file.Binary {
		writeProjectError(w, "symbol lookup failed", project.ErrUnsupportedFile)
		return
	}
	index, err := h.manager.Index()
	if err != nil {
		writeProjectError(w, "symbol lookup failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, symbolsResponse{
		ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision,
		Path: file.Path, Language: file.Language, Symbols: file.Symbols,
	})
}

func (h *ProjectHandler) Impact(w http.ResponseWriter, r *http.Request) {
	preview, err := h.manager.ImpactPreview(r.URL.Query().Get("path"), r.URL.Query().Get("symbol"))
	if err != nil {
		writeProjectError(w, "impact preview failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, preview)
}

func (h *ProjectHandler) GitStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.manager.GitStatus(r.URL.Query().Get("path"))
	if err != nil {
		writeProjectError(w, "Git status failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, status)
}

// Reindex refreshes deterministic facts without contacting the model. A client
// may supply its current revision to avoid refreshing state it no longer owns.
func (h *ProjectHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	var request reindexRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		api.WriteAppError(w, apperrors.BadRequest("invalid reindex request", "Provide an optional project_revision JSON field.", err))
		return
	}
	if request.ProjectRevision != "" {
		index, err := h.manager.Index()
		if err != nil {
			writeProjectError(w, "project reindex failed", err)
			return
		}
		if request.ProjectRevision != index.ProjectRevision {
			writeProjectError(w, "project reindex failed", project.ErrRevisionConflict)
			return
		}
	}
	var (
		index *project.ProjectIndex
		err   error
	)
	if h.service != nil {
		index, err = h.service.Reindex()
	} else {
		index, err = h.manager.Reindex()
	}
	if err != nil {
		writeProjectError(w, "project reindex failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, index)
}

// AnalyzeAllJob returns persisted sequential cache-warming progress.
func (h *ProjectHandler) AnalyzeAllJob(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	job, err := h.service.AnalyzeAllJob()
	if err != nil {
		writeProjectError(w, "analyze-all job lookup failed", err)
		return
	}
	if job == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	api.WriteJSON(w, http.StatusOK, job)
}

// StartAnalyzeAll explicitly starts a bounded sequential analysis job.
func (h *ProjectHandler) StartAnalyzeAll(w http.ResponseWriter, r *http.Request) {
	request, ok := h.decodeAnalyzeAllRequest(w, r)
	if !ok || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	job, err := h.service.StartAnalyzeAll(r.Context(), app.AnalyzeAllOptions{MaxFiles: request.MaxFiles, MaxRetries: request.MaxRetries}, request.ConfirmRemoteProvider)
	if err != nil {
		writeProjectError(w, "start analyze-all failed", err)
		return
	}
	api.WriteJSON(w, http.StatusAccepted, job)
}

// PauseAnalyzeAll pauses after any current one-file request completes.
func (h *ProjectHandler) PauseAnalyzeAll(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	job, err := h.service.PauseAnalyzeAll()
	if err != nil {
		writeProjectError(w, "pause analyze-all failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, job)
}

// ResumeAnalyzeAll restarts a paused job for the same active project revision.
func (h *ProjectHandler) ResumeAnalyzeAll(w http.ResponseWriter, r *http.Request) {
	request, ok := h.decodeAnalyzeAllRequest(w, r)
	if !ok || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	job, err := h.service.ResumeAnalyzeAll(r.Context(), request.ConfirmRemoteProvider)
	if err != nil {
		writeProjectError(w, "resume analyze-all failed", err)
		return
	}
	api.WriteJSON(w, http.StatusAccepted, job)
}

// CancelAnalyzeAll cancels an in-flight request and prevents later files starting.
func (h *ProjectHandler) CancelAnalyzeAll(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	job, err := h.service.CancelAnalyzeAll()
	if err != nil {
		writeProjectError(w, "cancel analyze-all failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, job)
}

// FileAnalysis returns cached semantic state only; source is never returned.
func (h *ProjectHandler) FileAnalysis(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	analysis, err := h.service.CachedFileAnalysis(r.URL.Query().Get("path"))
	if err != nil {
		writeProjectError(w, "file analysis lookup failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

// AnalyzeFile performs at most one selected-file model analysis.
func (h *ProjectHandler) AnalyzeFile(w http.ResponseWriter, r *http.Request) {
	request, ok := h.decodeFileAnalysisRequest(w, r)
	if !ok || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	analysis, err := h.service.AnalyzeFile(r.Context(), request.Path, request.Refresh, request.ConfirmRemoteProvider)
	if err != nil {
		if writeContextError(w, "file analysis", err) {
			return
		}
		writeProjectError(w, "file analysis failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, analysis)
}

// DeleteFileAnalysis clears one selected file's cache after a revision check.
func (h *ProjectHandler) DeleteFileAnalysis(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	if err := h.service.ClearFileAnalysis(r.URL.Query().Get("path")); err != nil {
		writeProjectError(w, "clear file analysis failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) decodeFileAnalysisRequest(w http.ResponseWriter, r *http.Request) (fileAnalysisRequest, bool) {
	var request fileAnalysisRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid file analysis request", "Provide path and project_revision.", err))
		return request, false
	}
	return request, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, value any, message, userMessage string) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		api.WriteAppError(w, apperrors.BadRequest(message, userMessage, err))
		return false
	}
	return true
}

func findingsProjectID(manager *project.Manager) string {
	analysis, err := manager.Analysis()
	if err != nil {
		return ""
	}
	return analysis.ProjectID
}

func (h *ProjectHandler) decodeAnalyzeAllRequest(w http.ResponseWriter, r *http.Request) (analyzeAllRequest, bool) {
	var request analyzeAllRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid analyze-all request", "Provide project_revision and optional job limits.", err))
		return request, false
	}
	return request, true
}

func (h *ProjectHandler) requireRevision(w http.ResponseWriter, revision string) bool {
	index, err := h.manager.Index()
	if err != nil {
		writeProjectError(w, "project revision check failed", err)
		return false
	}
	if revision == "" || revision != index.ProjectRevision {
		writeProjectError(w, "project revision check failed", project.ErrRevisionConflict)
		return false
	}
	return true
}

func writeProjectError(w http.ResponseWriter, action string, err error) {
	switch {
	case errors.Is(err, project.ErrNoActiveProject):
		api.WriteAppError(w, apperrors.NotFound(action, "Import a project before using this endpoint.", err))
	case errors.Is(err, project.ErrExcludedFile):
		api.WriteAppError(w, apperrors.Forbidden(action, "This file is excluded by the project context policy.", err))
	case errors.Is(err, project.ErrRevisionConflict):
		api.WriteAppError(w, apperrors.Conflict(action, "The active project changed. Refresh and try again.", err))
	case errors.Is(err, project.ErrUnsupportedFile):
		api.WriteAppError(w, apperrors.New(apperrors.TypeBadRequest, action, "This file does not support symbol extraction.", http.StatusUnprocessableEntity, err))
	default:
		api.WriteAppError(w, apperrors.BadRequest(action, err.Error(), err))
	}
}
