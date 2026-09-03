package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// DraftHandler exposes the explicit draft review, Apply, Undo, and audit
// operations.
type DraftHandler struct {
	service *app.Service
	manager *project.Manager
}

func NewDraftHandler(service *app.Service, manager *project.Manager) *DraftHandler {
	return &DraftHandler{service: service, manager: manager}
}

type draftUpdateRequest struct {
	ProjectRevision  string   `json:"project_revision"`
	ExpectedRevision int64    `json:"expected_revision"`
	Declaration      string   `json:"declaration"`
	Imports          []string `json:"imports"`
}

type draftValidationRequest struct {
	ProjectRevision  string `json:"project_revision"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type draftCheckRequest struct {
	ProjectRevision  string `json:"project_revision"`
	ExpectedRevision int64  `json:"expected_revision"`
	ExpectedHash     string `json:"expected_hash"`
	RunLint          bool   `json:"run_lint,omitempty"`
	RunTests         bool   `json:"run_tests,omitempty"`
}

// Draft returns the isolated editable declaration, never a complete source file.
func (h *DraftHandler) Draft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	draft, err := h.service.Draft(r.PathValue("draftID"))
	if err != nil {
		writeDraftError(w, "draft unavailable", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, draft)
}

// UpdateDraft creates a new revision for a manual declaration/import edit.
func (h *DraftHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	var request draftUpdateRequest
	if !decodeDraftRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	draft, err := h.service.UpdateDraft(app.DraftUpdateRequest{ID: r.PathValue("draftID"), ExpectedRevision: request.ExpectedRevision, Declaration: request.Declaration, Imports: request.Imports})
	if err != nil {
		writeDraftError(w, "update draft failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, draft)
}

// ValidateDraft performs focused declaration composition for one exact draft revision.
func (h *DraftHandler) ValidateDraft(w http.ResponseWriter, r *http.Request) {
	var request draftValidationRequest
	if !decodeDraftRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	draft, err := h.service.ValidateDraft(r.PathValue("draftID"), request.ExpectedRevision)
	if err != nil {
		writeDraftError(w, "validate draft failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, draft)
}

// CheckDraft runs isolated checks for the exact validated draft revision and hash.
func (h *DraftHandler) CheckDraft(w http.ResponseWriter, r *http.Request) {
	var request draftCheckRequest
	if !decodeDraftRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	report, err := h.service.CheckDraft(r.Context(), app.DraftCheckRequest{ID: r.PathValue("draftID"), ExpectedRevision: request.ExpectedRevision, ExpectedHash: request.ExpectedHash, Options: app.DraftCheckOptions{RunLint: request.RunLint, RunTests: request.RunTests}})
	if err != nil {
		writeDraftError(w, "draft checks failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, report)
}

// ReviewDraft returns validation, check evidence, and explicit Apply eligibility.
func (h *DraftHandler) ReviewDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	review, err := h.service.ReviewDraft(r.PathValue("draftID"))
	if err != nil {
		writeDraftError(w, "draft review unavailable", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, review)
}

func (h *DraftHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var request app.ApplyRequest
	if !decodeDraftRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	result, err := h.service.ApplyDraft(r.Context(), request)
	if err != nil {
		writeDraftError(w, "apply draft failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

func (h *DraftHandler) Undo(w http.ResponseWriter, r *http.Request) {
	var request app.UndoRequest
	if !decodeDraftRequest(w, r, &request) || !h.requireRevision(w, request.ProjectRevision) {
		return
	}
	result, err := h.service.UndoDraft(r.Context(), request)
	if err != nil {
		writeDraftError(w, "undo draft failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

func (h *DraftHandler) Audit(w http.ResponseWriter, r *http.Request) {
	if !h.requireRevision(w, r.URL.Query().Get("project_revision")) {
		return
	}
	entries, err := h.service.AuditHistory()
	if err != nil {
		writeDraftError(w, "audit history unavailable", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, entries)
}

func (h *DraftHandler) requireRevision(w http.ResponseWriter, revision string) bool {
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

func decodeDraftRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid draft request")
		return false
	}
	return true
}

func writeDraftError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, project.ErrRevisionConflict) {
		writeProjectError(w, action, err)
		return
	}
	api.WriteError(w, http.StatusBadRequest, err.Error())
}
