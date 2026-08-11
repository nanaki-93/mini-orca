// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
)

// HTMXRenderHandler manages HTTP handlers for HTMX partial rendering.
type HTMXRenderHandler struct {
	// templateEngine provides template rendering capabilities.
	templateEngine *TemplateEngine
	// cache provides caching for rendered responses.
	cache *api.ResponseCache
	// projectPath is the path to the project directory (from config or cwd).
	projectPath string
	// lastRenderedPhaseHistory stores the last rendered phase history for diffing.
	lastRenderedPhaseHistory string
}

// NewHTMXRenderHandler creates a new HTMXRenderHandler instance.
func NewHTMXRenderHandler(
	templateEngine *TemplateEngine,
	cache *api.ResponseCache,
	projectPath string,
) *HTMXRenderHandler {
	return &HTMXRenderHandler{
		templateEngine: templateEngine,
		cache:          cache,
		projectPath:    projectPath,
	}
}

// RenderMainPage renders the main IDE page.
func (h *HTMXRenderHandler) RenderMainPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)

	// Project data from config or cwd
	projectPath := h.projectPath
	if projectPath == "" {
		// Fallback to current working directory
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			projectPath = "."
		}
	}

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
