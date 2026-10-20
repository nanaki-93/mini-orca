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
	report, err := h.service.StartGoScan(request.ProjectRevision)
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
	trust, err := h.service.ExecutionTrust(revision, r.URL.Query().Get("task_test_name"))
	if err != nil {
		writeProjectError(w, "execution trust lookup failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, trust)
}

// TrustProjectExecution records a deliberate, current-session authorization.
func (h *ProjectHandler) TrustProjectExecution(w http.ResponseWriter, r *http.Request) {
	var request executionTrustRequest
	if !decodeStrictJSON(w, r, &request, "invalid execution trust request", "Provide project_revision and confirm: true.") || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
		return
	}
	trust, err := h.service.TrustProjectExecution(request.ProjectRevision, request.Confirm)
	if err != nil {
		api.WriteAppError(w, api.BadRequest("trusted local execution confirmation required", "Review the command scope and explicitly trust local execution for this project.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, trust)
}

// BootstrapExec executes `mise trust && mise install` and `lefthook install`
// in the active project root under explicit local execution trust.
func (h *ProjectHandler) BootstrapExec(w http.ResponseWriter, r *http.Request) {
	var request executionTrustRequest
	if !decodeStrictJSON(w, r, &request, "invalid bootstrap exec request", "Provide project_revision and confirm: true.") || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
		return
	}
	result, err := h.service.ExecuteBootstrapToolchain(r.Context(), request.ProjectRevision, request.Confirm)
	if err != nil {
		api.WriteAppError(w, api.BadRequest("bootstrap toolchain execution failed", "Confirm local execution trust before running mise and lefthook.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
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
