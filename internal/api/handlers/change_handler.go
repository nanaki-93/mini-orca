package handlers

import (
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"net/http"
	"path/filepath"
)

type ChangeHandler struct {
	service *app.Service
	manager *project.Manager
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
		writeChatError(w, "AI workflow request failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, value)
}
func guardedWorkflowRead(w http.ResponseWriter, r *http.Request, manager *project.Manager) bool {
	index, err := manager.Index()
	if err != nil {
		writeChatError(w, "project unavailable", err)
		return false
	}
	if r.URL.Query().Get("project_id") != index.ProjectID || r.URL.Query().Get("project_revision") != index.ProjectRevision {
		writeChatError(w, "project changed", project.ErrRevisionConflict)
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
	var request app.ChangeIdentity
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.ReviewChange(r.Context(), r.PathValue("sessionID"), request)
	respondWorkflow(w, value, err)
}
func (h *ChangeHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeApplyRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.ApplyChange(r.Context(), r.PathValue("sessionID"), request)
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

type InstructionPreset struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
}
type InstructionPreview struct {
	ProjectID       string                        `json:"project_id"`
	ProjectRevision string                        `json:"project_revision"`
	Path            string                        `json:"path"`
	ExistingContent string                        `json:"existing_content"`
	Exists          bool                          `json:"exists"`
	Effective       project.EffectiveInstructions `json:"effective"`
	Presets         []InstructionPreset           `json:"presets"`
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
	current, err := h.manager.Index()
	if err != nil {
		respondWorkflow(w, nil, err)
		return
	}
	if h.manager.Root() != root || current.ProjectID != index.ProjectID || current.ProjectRevision != index.ProjectRevision {
		respondWorkflow(w, nil, project.ErrRevisionConflict)
		return
	}
	preview := InstructionPreview{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Path: path, Effective: effective, Presets: []InstructionPreset{
		{"focused", "Keep changes focused", "Keep changes limited to the requested behavior. Preserve unrelated work and public contracts."},
		{"tests", "Test meaningful behavior", "Cover changed behavior and meaningful error paths with deterministic tests. Report checks actually run."},
		{"style", "Follow project conventions", "Read nearby implementations before editing. Reuse existing patterns and avoid unnecessary dependencies."},
		{"security", "Protect sensitive data", "Keep credentials and source-bearing diagnostics out of logs and Git. Validate external input at its boundary."},
		{"review", "Review every change", "Prepare a diff and explain behavior and verification before explicitly approved source changes."},
		{"cancellation", "Preserve cancellation", "Propagate cancellation and deadlines, reject stale asynchronous results, and own background work lifetimes."},
	}}
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
