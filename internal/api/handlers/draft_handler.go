package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// DraftHandler exposes the explicit draft mutation, validation, check, Apply,
// and Undo operations.
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

// UpdateDraft creates a new revision for a manual declaration/import edit.
func (h *DraftHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	var request draftUpdateRequest
	if !decodeDraftRequest(w, r, &request) || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
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
	if !decodeDraftRequest(w, r, &request) || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
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
	if !decodeDraftRequest(w, r, &request) || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
		return
	}
	report, err := h.service.CheckDraft(r.Context(), app.DraftCheckRequest{ID: r.PathValue("draftID"), ExpectedRevision: request.ExpectedRevision, ExpectedHash: request.ExpectedHash, Options: app.DraftCheckOptions{RunLint: request.RunLint, RunTests: request.RunTests}})
	if err != nil {
		writeDraftError(w, "draft checks failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, report)
}

func (h *DraftHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var request app.ApplyRequest
	if !decodeDraftRequest(w, r, &request) || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
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
	if !decodeDraftRequest(w, r, &request) || !requireCurrentRevision(w, h.manager, request.ProjectRevision) {
		return
	}
	result, err := h.service.UndoDraft(r.Context(), request)
	if err != nil {
		writeDraftError(w, "undo draft failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

func decodeDraftRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	if err := api.DecodeJSON(w, r, value); err != nil {
		api.WriteRequestError(w, err, "invalid draft request", "Provide one complete draft request object.")
		return false
	}
	return true
}

func writeDraftError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, project.ErrRevisionConflict) {
		writeProjectError(w, action, err)
		return
	}
	api.WriteAppError(w, api.BadRequest(action, "Review the draft and try again.", err))
}
