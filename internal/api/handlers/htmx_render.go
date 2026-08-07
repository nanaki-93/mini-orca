// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
)

// HTMXRenderHandler manages HTTP handlers for HTMX partial rendering.
type HTMXRenderHandler struct {
	// templateEngine provides template rendering capabilities.
	templateEngine *TemplateEngine
	// projectStore provides access to project data.
	projectStore *ProjectStore
	// cache provides caching for rendered responses.
	cache *api.ResponseCache
	// lastRenderedPhaseHistory stores the last rendered phase history for diffing.
	lastRenderedPhaseHistory string
}

// NewHTMXRenderHandler creates a new HTMXRenderHandler instance.
func NewHTMXRenderHandler(
	templateEngine *TemplateEngine,
	projectStore *ProjectStore,
	cache *api.ResponseCache,
) *HTMXRenderHandler {
	return &HTMXRenderHandler{
		templateEngine: templateEngine,
		projectStore:   projectStore,
		cache:          cache,
	}
}

// RenderMainPage renders the main IDE page.
func (h *HTMXRenderHandler) RenderMainPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})

	data["SessionID"] = "no-active-session"
	data["CurrentPhaseName"] = "Idle"
	data["CurrentPhaseStatus"] = "pending"
	data["PhaseProgress"] = 0

	// Project data
	projects := h.projectStore.ListProjects()
	if len(projects) > 0 {
		project := projects[0]
		data["ProjectName"] = project.Name
		data["ProjectPath"] = project.Path

		entries, err := h.projectStore.ListProjectFiles(project.ID, "")
		if err == nil {
			items := make([]FileSystemItem, 0, len(entries))
			for _, entry := range entries {
				items = append(items, FileSystemItem{
					Name:  entry.Name,
					Path:  entry.Path,
					IsDir: entry.IsDir,
					Size:  entry.Size,
				})
			}
			data["RootItems"] = items
		}
	} else {
		data["ProjectName"] = "No Project"
		data["ProjectPath"] = "Please open or create a project"
		data["RootItems"] = []FileSystemItem{}
	}

	data["CurrentPath"] = ""

	h.templateEngine.RenderMain(w, "ide", data)
}
