package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ChangeHandler struct {
	service *app.Service
	manager *project.Manager
}

func (h *ChangeHandler) InvestigatePlan(w http.ResponseWriter, r *http.Request) {
	var request app.PlanInvestigationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.InvestigatePlan(r.Context(), request)
	respondWorkflow(w, value, err)
}

func NewChangeHandler(service *app.Service, manager *project.Manager) *ChangeHandler {
	return &ChangeHandler{service, manager}
}

func decodeWorkflowBody(w http.ResponseWriter, r *http.Request, body any) bool {
	if err := api.DecodeJSON(w, r, body); err != nil {
		api.WriteRequestError(w, err, "invalid AI workflow request", "Provide the displayed task identity and supported fields.")
		return false
	}
	return true
}

func respondWorkflow(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeWorkflowError(w, "AI workflow request failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, value)
}

func guardedWorkflowRead(w http.ResponseWriter, r *http.Request, manager *project.Manager) bool {
	index, err := manager.Index()
	if err != nil {
		writeWorkflowError(w, "project unavailable", err)
		return false
	}
	if r.URL.Query().Get("project_id") != index.ProjectID || r.URL.Query().Get("project_revision") != index.ProjectRevision {
		writeWorkflowError(w, "project changed", project.ErrRevisionConflict)
		return false
	}
	return true
}

func (h *ChangeHandler) List(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	value, err := h.service.ChangeHistory(r.Context())
	respondWorkflow(w, value, err)
}

// Usage totals what agents spent per task in the selected checkout.
func (h *ChangeHandler) Usage(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	value, err := h.service.ChangeUsage(r.Context())
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	value, err := h.service.ChangeSession(r.Context(), r.PathValue("sessionID"))
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Open(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeCreateRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.OpenChangeSession(r.Context(), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Resume(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeIdentity
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	index, err := h.manager.Index()
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	if request.ProjectID != index.ProjectID || request.ProjectRevision != index.ProjectRevision {
		respondWorkflow(w, nil, project.ErrRevisionConflict)
		return
	}
	value, err := h.service.ResumeChange(r.Context(), r.PathValue("sessionID"))
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Message(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeMessageRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	var value *app.ChangeSession
	var err error
	if request.Repair {
		value, err = h.service.RepairChangeMessage(r.Context(), r.PathValue("sessionID"), request)
	} else {
		value, err = h.service.SendChangeMessage(r.Context(), r.PathValue("sessionID"), request)
	}
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Checks(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeCheckRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.CheckChange(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Review(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeReviewRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.ReviewChange(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Comment(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeCommentRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.CommentChangeLine(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeCommentDeleteRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.DeleteChangeComment(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeApplyRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.ApplyChange(r.Context(), r.PathValue("sessionID"), request)
	if request.CommitMessage != "" || request.Head != "" {
		respondGit(w, value, err)
		return
	}
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Undo(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeApplyRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.UndoChange(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Commit(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeApplyRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.RetryChangeCommit(r.Context(), r.PathValue("sessionID"), request)
	respondGit(w, value, err)
}

func (h *ChangeHandler) Recovery(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	value, err := h.service.ChangeRecovery()
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeIdentity
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.VerifyChange(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) ReviewFile(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeFileReviewRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.ReviewChangeFile(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}

func (h *ChangeHandler) Plan(w http.ResponseWriter, r *http.Request) {
	var request app.ChangePlanRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.UpdateChangePlan(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}
