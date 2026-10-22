package handlers

import (
	"context"
	"errors"
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"net/http"
)

type GitHandler struct{ service *app.Service }

func NewGitHandler(service *app.Service) *GitHandler { return &GitHandler{service} }

func respondGit(w http.ResponseWriter, value any, err error) {
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, project.ErrRevisionConflict):
			status = http.StatusConflict
		case errors.Is(err, context.Canceled):
			status = http.StatusRequestTimeout
		case errors.Is(err, context.DeadlineExceeded):
			status = http.StatusGatewayTimeout
		}
		api.WriteError(w, status, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, value)
}

func (h *GitHandler) Repository(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GitRepository(r.Context(), r.URL.Query().Get("project_id"), r.URL.Query().Get("project_revision"))
	respondGit(w, value, err)
}

func (h *GitHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GitWorkspace(r.Context(), r.URL.Query().Get("project_id"), r.URL.Query().Get("project_revision"))
	respondGit(w, value, err)
}
func (h *GitHandler) CreateWorktree(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.CreateGitWorktree(r.Context(), request)
	respondGit(w, value, err)
}
func (h *GitHandler) Switch(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.SwitchGitBranch(r.Context(), request)
	respondGit(w, value, err)
}
func (h *GitHandler) PreviewMerge(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.PreviewGitMerge(r.Context(), request)
	respondGit(w, value, err)
}
func (h *GitHandler) Merge(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.MergeGitWorktree(r.Context(), request)
	respondGit(w, value, err)
}
func (h *GitHandler) AbortMerge(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.AbortGitMerge(r.Context(), request)
	respondGit(w, value, err)
}

func (h *GitHandler) CleanupMergedWorktree(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.CleanupMergedWorktree(r.Context(), request)
	respondGit(w, value, err)
}

func (h *GitHandler) DiscardWorktree(w http.ResponseWriter, r *http.Request) {
	var request app.GitMutationRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.DiscardGitWorktree(r.Context(), request)
	respondGit(w, value, err)
}
