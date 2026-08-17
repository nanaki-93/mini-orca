// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
)

// HTMXRenderHandler manages HTTP handlers for HTMX partial rendering.
type HTMXRenderHandler struct {
	// templateEngine provides template rendering capabilities.
	templateEngine *TemplateEngine
	// cache provides caching for rendered responses.
	cache *api.ResponseCache
	// projectManager provides the currently imported project.
	projectManager *project.Manager
	// lastRenderedPhaseHistory stores the last rendered phase history for diffing.
	lastRenderedPhaseHistory string
}

// NewHTMXRenderHandler creates a new HTMXRenderHandler instance.
func NewHTMXRenderHandler(
	templateEngine *TemplateEngine,
	cache *api.ResponseCache,
	projectManager *project.Manager,
) *HTMXRenderHandler {
	return &HTMXRenderHandler{
		templateEngine: templateEngine,
		cache:          cache,
		projectManager: projectManager,
	}
}

// RenderMainPage renders the main IDE page.
func (h *HTMXRenderHandler) RenderMainPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)
	data["Version"] = version.Version

	// Project data from config or cwd
	projectPath := h.getProjectPath()

	projectName := filepath.Base(projectPath)
	data["ProjectName"] = projectName
	data["ProjectPath"] = projectPath

	// Read file tree directly from filesystem
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		data["RootItems"] = []FileSystemItem{}
	} else {
		items := make([]FileSystemItem, 0, len(entries))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			items = append(items, FileSystemItem{
				Name:  entry.Name(),
				Path:  entry.Name(),
				IsDir: entry.IsDir(),
				Size:  info.Size(),
			})
		}
		data["RootItems"] = items
	}

	data["CurrentPath"] = ""

	h.templateEngine.RenderMain(w, "ide", data)
}
