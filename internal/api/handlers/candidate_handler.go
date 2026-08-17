package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// CandidateHandler exposes explicit check, Apply, Undo, and source-free audit
// operations for already-generated candidates.
type CandidateHandler struct {
	service *app.Service
	manager *project.Manager
}

func NewCandidateHandler(service *app.Service, manager *project.Manager) *CandidateHandler {
	return &CandidateHandler{service: service, manager: manager}
}

type candidateCheckRequest struct {
	GenerationID    string `json:"generation_id"`
	ProjectRevision string `json:"project_revision"`
	RunLint         bool   `json:"run_lint,omitempty"`
	RunTests        bool   `json:"run_tests,omitempty"`
}

func (h *CandidateHandler) Check(w http.ResponseWriter, r *http.Request) {
	var request candidateCheckRequest
	if !decodeCandidateRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	report, err := h.service.CheckCandidate(r.Context(), request.GenerationID, app.CandidateCheckOptions{RunLint: request.RunLint, RunTests: request.RunTests})
	if err != nil {
		writeCandidateError(w, "candidate checks failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, report)
}

func (h *CandidateHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var request app.ApplyRequest
	if !decodeCandidateRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	result, err := h.service.ApplyCandidate(r.Context(), request)
	if err != nil {
		writeCandidateError(w, "apply candidate failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

func (h *CandidateHandler) Undo(w http.ResponseWriter, r *http.Request) {
	var request app.UndoRequest
	if !decodeCandidateRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	result, err := h.service.UndoCandidate(r.Context(), request)
	if err != nil {
		writeCandidateError(w, "undo candidate failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

func (h *CandidateHandler) Audit(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	entries, err := h.service.AuditHistory()
	if err != nil {
		writeCandidateError(w, "audit history unavailable", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, entries)
}

func (h *CandidateHandler) requireRevision(w http.ResponseWriter, revision string) bool {
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

func decodeCandidateRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	if r.Method != http.MethodPost {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid candidate request")
		return false
	}
	return true
}

func writeCandidateError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, project.ErrRevisionConflict) {
		writeProjectError(w, action, err)
		return
	}
	api.WriteError(w, http.StatusBadRequest, err.Error())
}
