package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
)

// ProjectHandler manages HTTP handlers for project lifecycle operations.
type ProjectHandler struct {
	// projectStore provides access to project data.
	projectStore *ProjectStore
}

// NewProjectHandler creates a new ProjectHandler instance.
func NewProjectHandler(projectStore *ProjectStore) *ProjectHandler {
	return &ProjectHandler{
		projectStore: projectStore,
	}
}

// ListProjects handles GET /api/projects
// Returns a list of all projects with their summaries.
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := h.projectStore.ListProjects()

	summaries := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		summaries = append(summaries, ProjectSummary{
			ID:        project.ID,
			Name:      project.Name,
			Type:      project.Type,
			FileCount: project.FileCount,
			CreatedAt: project.CreatedAt,
		})
	}

	api.WriteJSON(w, http.StatusOK, ProjectListResponse{
		Projects: summaries,
		Total:    len(summaries),
	})
}

// CreateProject handles POST /api/projects
// Creates or opens a project from the given path.
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req ProjectCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if req.Path == "" {
		api.WriteAppError(w, apperrors.BadRequest("path is required", "The project path must be provided.", nil))
		return
	}

	project, err := h.projectStore.CreateProject(req.Name, req.Path, req.Type)
	if err != nil {
		api.WriteAppError(w, apperrors.BadRequest("project creation failed", "Failed to create or open project: "+err.Error(), err))
		return
	}

	api.WriteJSON(w, http.StatusCreated, ProjectResponse{
		ID:        project.ID,
		Name:      project.Name,
		Path:      project.Path,
		Type:      project.Type,
		FileCount: project.FileCount,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	})
}

// handleProjectRoute dispatches project-related requests based on the URL path.
func (h *ProjectHandler) handleProjectRoute(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// GET /api/projects - list projects
	if path == "/api/projects" || path == "/api/projects/" {
		switch r.Method {
		case http.MethodGet:
			h.ListProjects(w, r)
			return
		case http.MethodPost:
			h.CreateProject(w, r)
			return
		}
	}

	// /api/projects/{id}/files - file operations
	if strings.HasPrefix(path, "/api/projects/") && strings.Contains(path, "/files") {
		projectID := api.ExtractProjectID(path)
		if projectID != "" {
			if r.Method == http.MethodGet {
				h.ListFiles(w, r)
				return
			}
		}
	}

	// Route not found
	api.WriteAppError(w, apperrors.NotFound("route not found", "No handler for this project route.", nil))
}

// ─── Helper Types ─────────────────────────────────────────────────────────────

// directoryEntry represents a single file or directory entry.
type directoryEntry struct {
	name       string
	relPath    string
	isDir      bool
	size       int64
	modifiedAt time.Time
}

// ─── Helper Functions ─────────────────────────────────────────────────────────

// detectProjectTypeFromDir detects the project type by examining directory contents.
func detectProjectTypeFromDir(dirPath string) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "unknown"
	}

	for _, entry := range entries {
		name := entry.Name()
		switch {
		case name == "go.mod":
			return "go"
		case name == "requirements.txt" || name == "setup.py" || name == "pyproject.toml":
			return "python"
		case name == "package.json" || name == "tsconfig.json":
			return "typescript"
		case name == "Cargo.toml":
			return "rust"
		case name == "pom.xml" || (strings.HasSuffix(name, ".java") && !entry.IsDir()):
			return "java"
		case name == "build.gradle" || name == "build.gradle.kts":
			return "kotlin"
		}
	}

	return "unknown"
}

// countFiles counts the number of files in a directory recursively.
func countFiles(dirPath string) (int, error) {
	count := 0
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// scanDirectory scans a directory and returns file entries.
func scanDirectory(dirPath string) ([]directoryEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	result := make([]directoryEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, directoryEntry{
			name:       entry.Name(),
			relPath:    entry.Name(),
			isDir:      entry.IsDir(),
			size:       info.Size(),
			modifiedAt: info.ModTime(),
		})
	}

	return result, nil
}

// detectContentType determines the MIME type of a file based on extension and content.
func detectContentType(filePath string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".js", ".mjs":
		return "text/javascript"
	case ".ts", ".tsx":
		return "text/typescript"
	case ".rs":
		return "text/x-rust"
	case ".java":
		return "text/x-java"
	case ".kt", ".kts":
		return "text/x-kotlin"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".toml":
		return "text/toml"
	case ".md":
		return "text/markdown"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".xml":
		return "application/xml"
	case ".sh":
		return "text/x-sh"
	case ".bash":
		return "text/x-sh"
	case ".sql":
		return "text/x-sql"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		// Check if content is binary
		if isBinary(content) {
			return "application/octet-stream"
		}
		return "text/plain"
	}
}

// isBinary checks if content appears to be binary.
func isBinary(content []byte) bool {
	// Check first 800 bytes for null bytes (common in binary files)
	checkLen := len(content)
	if checkLen > 800 {
		checkLen = 800
	}
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// resolveProjectPath attempts to resolve a project path using multiple strategies.
// It handles absolute paths, relative paths, and common shorthand notations.
func resolveProjectPath(projectPath string) (string, error) {
	// Strategy 1: If it's already an absolute path, use it directly
	if filepath.IsAbs(projectPath) {
		absPath, err := filepath.Abs(projectPath)
		if err != nil {
			return "", fmt.Errorf("failed to make path absolute: %w", err)
		}
		return absPath, nil
	}

	// Strategy 2: Try as-is (relative to current directory)
	absPath, err := filepath.Abs(projectPath)
	if err == nil {
		if info, statErr := os.Stat(absPath); statErr == nil && info.IsDir() {
			return absPath, nil
		}
	}

	// Strategy 3: Try common home directory shorthands
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// Replace ~ with home directory
		if strings.HasPrefix(projectPath, "~/") {
			tildePath := filepath.Join(homeDir, projectPath[2:])
			if info, statErr := os.Stat(tildePath); statErr == nil && info.IsDir() {
				return tildePath, nil
			}
		}
		// Also try ~ alone (home directory)
		if projectPath == "~" {
			return homeDir, nil
		}
	}

	// Strategy 4: Try with ./ prefix
	if info, statErr := os.Stat("./" + projectPath); statErr == nil && info.IsDir() {
		absPath, _ := filepath.Abs("./" + projectPath)
		return absPath, nil
	}

	return "", fmt.Errorf("path %q could not be resolved to an existing directory", projectPath)
}
