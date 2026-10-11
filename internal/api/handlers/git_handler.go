package handlers

import (
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"net/http"
)

type GitHandler struct{ service *app.Service }

func NewGitHandler(service *app.Service) *GitHandler { return &GitHandler{service} }

func respondGit(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeChatError(w, "Git operation failed", err)
		return
	}
	api.WriteJSON(w, http.StatusOK, value)
}

func (h *GitHandler) Repository(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GitRepository(r.Context(), r.URL.Query().Get("project_id"), r.URL.Query().Get("project_revision"))
	respondGit(w, value, err)
}
