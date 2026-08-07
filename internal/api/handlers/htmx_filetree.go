package handlers

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
)

// RenderFileTree handles GET /api/render/file-tree
// Renders the HTML partial for the file tree explorer.
func (h *HTMXRenderHandler) RenderFileTree(w http.ResponseWriter, r *http.Request) {
	projectPath := h.getProjectPath()

	// Read root directory
	items, err := h.readDir(projectPath)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("file listing failed", "Failed to list project files.", err))
		return
	}

	data := FileTreeRenderData{
		RootItems:   items,
		CurrentPath: "",
		ProjectPath: projectPath,
	}

	rendered, err := h.templateEngine.RenderComponentPartial("file-tree", data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the file tree component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// ExpandFolder handles GET /api/tree/expand
// Returns HTML for the children of a directory (lazy-loaded).
func (h *HTMXRenderHandler) ExpandFolder(w http.ResponseWriter, r *http.Request) {
	// HTMX with hx-get sends hx-vals as query parameters
	path := r.URL.Query().Get("path")
	depthStr := r.URL.Query().Get("depth")

	if path == "" {
		// Try form-encoded body (HTMX POST)
		if err := r.ParseForm(); err == nil {
			path = r.Form.Get("path")
			if depthStr == "" {
				depthStr = r.Form.Get("depth")
			}
		}
	}
	if path == "" {
		// Try JSON body
		var vals map[string]string
		if json.NewDecoder(r.Body).Decode(&vals) == nil {
			path = vals["path"]
			if depthStr == "" {
				depthStr = vals["depth"]
			}
		}
	}
	if path == "" {
		api.WriteAppError(w, apperrors.BadRequest("path is required", "A directory path must be provided.", nil))
		return
	}

	// Default depth to 1 if not provided, otherwise parse integer
	depth := 1
	if depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			depth = d
		}
	}

	projectPath := h.getProjectPath()
	fullPath := filepath.Join(projectPath, path)

	// Verify the path is within the project directory
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(projectPath)) {
		api.WriteAppError(w, apperrors.BadRequest("invalid path", "Path escapes project directory.", nil))
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("directory listing failed", "Failed to list directory contents.", err))
		return
	}

	var sb strings.Builder
	nextDepth := strconv.Itoa(depth + 1)
	paddingLeft := fmt.Sprintf("calc(0.75rem + %d * 1rem)", depth)

	for _, entry := range entries {
		if entry.IsDir() {
			// 1. Calculate the MD5 ID upfront
			folderID := fmt.Sprintf("folder-children-%x", md5.Sum([]byte(entry.Name())))

			sb.WriteString(`<div class="tree-folder" data-path="` + filepath.Join(path, entry.Name()) + `" data-loaded="false">`)

			// 2. Use folderID in hx-target, update hx-vals with current path and next depth, and dynamic padding
			sb.WriteString(`<div class="tree-item tree-folder-toggle flex items-center gap-1.5 px-3 py-2 cursor-pointer hover:bg-dark-700 rounded-sm transition-colors select-none" style="padding-left: ` + paddingLeft + `" hx-get="/api/tree/expand" hx-vals='{"path": "` + filepath.Join(path, entry.Name()) + `", "depth": "` + nextDepth + `"}' hx-target="#` + folderID + `" hx-swap="innerHTML" hx-trigger="click once">`)

			sb.WriteString(`<svg class="tree-chevron w-4 h-4 text-text-secondary shrink-0 transition-transform" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/></svg>`)
			sb.WriteString(`<svg class="tree-folder-icon w-4 h-4 text-yellow-500 shrink-0" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>`)
			sb.WriteString(`<span class="text-text-primary">` + template.HTMLEscapeString(entry.Name()) + `</span></div>`)

			// 3. The container ID matches hx-target perfectly
			sb.WriteString(`<div id="` + folderID + `" class="tree-folder-children overflow-hidden transition-all duration-150"></div>`)
			sb.WriteString(`</div>`)
		} else {
			ext := fileExtension(entry.Name())
			icon := fileIcon(ext)
			sb.WriteString(`<div class="tree-file" data-path="` + filepath.Join(path, entry.Name()) + `">`)

			sb.WriteString(`<div class="tree-item tree-file-toggle flex items-center gap-1.5 px-3 py-1 cursor-pointer hover:bg-dark-700 rounded-sm transition-colors select-none text-text-primary" style="padding-left: ` + paddingLeft + `" hx-get="/api/files/view" hx-vals='{"path": "` + filepath.Join(path, entry.Name()) + `"}' hx-target="#file-content" hx-indicator="#loading-indicator">`)

			sb.WriteString(`<svg class="tree-file-icon w-4 h-4 shrink-0" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">`)
			if icon == "folder" {
				sb.WriteString(`<path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/>`)
			} else {
				sb.WriteString(`<path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd"/>`)
			}
			sb.WriteString(`</svg><span class="truncate">` + template.HTMLEscapeString(entry.Name()) + `</span></div></div>`)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sb.String()))
}

// ViewFile handles GET /api/files/view
// Returns the content of a file for display.
func (h *HTMXRenderHandler) ViewFile(w http.ResponseWriter, r *http.Request) {
	// HTMX with hx-get sends hx-vals as query parameters
	path := r.URL.Query().Get("path")
	if path == "" {
		// Try form-encoded body (HTMX POST)
		if err := r.ParseForm(); err == nil {
			path = r.Form.Get("path")
		}
	}
	if path == "" {
		// Try JSON body
		var vals map[string]string
		if json.NewDecoder(r.Body).Decode(&vals) == nil {
			path = vals["path"]
		}
	}
	if path == "" {
		api.WriteAppError(w, apperrors.BadRequest("path is required", "A file path must be provided.", nil))
		return
	}

	projectPath := h.getProjectPath()
	fullPath := filepath.Join(projectPath, path)

	// Verify the path is within the project directory
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(projectPath)) {
		api.WriteAppError(w, apperrors.BadRequest("invalid path", "Path escapes project directory.", nil))
		return
	}

	// Check if it's a file (not a directory)
	info, err := os.Stat(fullPath)
	if err != nil {
		api.WriteAppError(w, apperrors.NotFound("file not found", "File not found: "+path, err))
		return
	}
	if info.IsDir() {
		api.WriteAppError(w, apperrors.BadRequest("not a file", path+" is a directory, not a file.", nil))
		return
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("file read failed", "Failed to read file content.", err))
		return
	}

	// Escape HTML for safe display
	escapedContent := template.HTMLEscapeString(string(content))

	// Format as code block with line numbers
	lines := strings.Split(escapedContent, "\n")
	var sb strings.Builder
	sb.WriteString(`<div class="p-4 text-sm font-mono overflow-auto" style="max-height: calc(100vh - 12rem);">`)
	sb.WriteString(`<div class="flex items-center justify-between mb-2 pb-2 border-b border-dark-700">`)
	sb.WriteString(`<span class="text-text-secondary">` + template.HTMLEscapeString(path) + `</span>`)
	sb.WriteString(`<span class="text-text-secondary text-xs">` + fmt.Sprintf("%d lines", len(lines)) + `</span>`)
	sb.WriteString(`</div>`)
	sb.WriteString(`<pre class="m-0"><code>`)
	for i, line := range lines {
		if line == "" && i == len(lines)-1 {
			continue // Skip trailing empty line
		}
		sb.WriteString(`<span class="line-number text-dark-600 select-none mr-4" style="min-width:2rem;display:inline-block;text-align:right;">` + fmt.Sprintf("%d", i+1) + `</span>`)
		sb.WriteString(line)
		sb.WriteString(`
`)
	}
	sb.WriteString(`</code></pre></div>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sb.String()))
}

// getProjectPath returns the configured project path or falls back to cwd.
func (h *HTMXRenderHandler) getProjectPath() string {
	if h.projectPath != "" {
		return h.projectPath
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// readDir reads a directory and returns a slice of FileSystemItem.
func (h *HTMXRenderHandler) readDir(dirPath string) ([]FileSystemItem, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

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

	return items, nil
}
