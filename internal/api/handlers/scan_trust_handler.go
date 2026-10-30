package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
)

type goScanRequest struct {
	ProjectRevision string `json:"project_revision"`
}

type executionTrustRequest struct {
	ProjectRevision string `json:"project_revision"`
	Confirm         bool   `json:"confirm"`
}

// StartGoScan explicitly starts parser, vet, and test verification in a
// temporary copied workspace; no imported source is modified.
func (h *ProjectHandler) StartGoScan(w http.ResponseWriter, r *http.Request) {
	var request goScanRequest
	if !decodeStrictJSON(w, r, &request, "invalid verified scan request", "Provide project_revision.") || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
		return
	}
	report, err := h.service.StartGoScan(r.Context(), request.ProjectRevision)
	if err != nil {
		writeProjectError(w, "start verified scan failed", err)
		return
	}
	api.WriteJSON(w, http.StatusAccepted, report)
}

// ExecutionTrust reports the command scope that can execute imported project
// code and whether this active project revision has been explicitly trusted.
func (h *ProjectHandler) ExecutionTrust(w http.ResponseWriter, r *http.Request) {
	revision := r.URL.Query().Get("project_revision")
	if !requireCurrentRevision(w, h.manager, revision) {
		return
	}
	trust, err := h.service.ExecutionTrust(r.Context(), revision, r.URL.Query().Get("task_test_name"))
	if err != nil {
		writeProjectError(w, "execution trust lookup failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, trust)
}

// TrustProjectExecution saves or revokes deliberate consent for this project path.
func (h *ProjectHandler) TrustProjectExecution(w http.ResponseWriter, r *http.Request) {
	var request executionTrustRequest
	if !decodeStrictJSON(w, r, &request, "invalid execution trust request", "Provide project_revision and confirm: true.") || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
		return
	}
	trust, err := h.service.TrustProjectExecution(r.Context(), request.ProjectRevision, request.Confirm)
	if err != nil {
		api.WriteAppError(w, api.BadRequest("trusted local execution confirmation required", "Review the command scope and explicitly trust local execution for this project.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, trust)
}

// GoScanProgress reads source-free persisted progress for the active revision.
func (h *ProjectHandler) GoScanProgress(w http.ResponseWriter, r *http.Request) {
	revision := r.URL.Query().Get("project_revision")
	if !requireCurrentRevision(w, h.manager, revision) {
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

// CancelGoScan requests cancellation for the active temporary-workspace scan.
func (h *ProjectHandler) CancelGoScan(w http.ResponseWriter, r *http.Request) {
	revision := r.URL.Query().Get("project_revision")
	if !requireCurrentRevision(w, h.manager, revision) {
		return
	}
	report, err := h.service.CancelGoScan(revision)
	if err != nil {
		writeProjectError(w, "cancel verified scan failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, report)
}
