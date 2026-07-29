// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ─── Request/Response Types ──────────────────────────────────────────────────

// ProjectCreateRequest represents the request body for creating/opening a project.
type ProjectCreateRequest struct {
	// Name is the project name (derived from directory if empty).
	Name string `json:"name,omitempty"`
	// Path is the filesystem path to the project directory.
	Path string `json:"path"`
	// Type is the project type (e.g., "go", "python").
	Type string `json:"type,omitempty"`
}

// ProjectResponse represents the response body for project data.
type ProjectResponse struct {
	// ID is the unique identifier for the project.
	ID string `json:"id"`
	// Name is the project name.
	Name string `json:"name"`
	// Path is the filesystem path to the project directory.
	Path string `json:"path"`
	// Type is the project type.
	Type string `json:"type"`
	// FileCount is the number of files in the project.
	FileCount int `json:"file_count"`
	// CreatedAt is when the project was created/opened.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the project was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectListResponse represents the response body for listing projects.
type ProjectListResponse struct {
	// Projects is the list of project summaries.
	Projects []ProjectSummary `json:"projects"`
	// Total is the total number of projects.
	Total int `json:"total"`
}

// ProjectSummary represents a summary of a project for list responses.
type ProjectSummary struct {
	// ID is the unique identifier for the project.
	ID string `json:"id"`
	// Name is the project name.
	Name string `json:"name"`
	// Type is the project type.
	Type string `json:"type"`
	// FileCount is the number of files in the project.
	FileCount int `json:"file_count"`
	// CreatedAt is when the project was created/opened.
	CreatedAt time.Time `json:"created_at"`
}

// FileEntry represents a file or directory entry in a project.
type FileEntry struct {
	// Name is the file or directory name.
	Name string `json:"name"`
	// Path is the relative path from the project root.
	Path string `json:"path"`
	// IsDir indicates whether this entry is a directory.
	IsDir bool `json:"is_dir"`
	// Size is the file size in bytes (0 for directories).
	Size int64 `json:"size"`
	// ModifiedAt is the last modification time.
	ModifiedAt time.Time `json:"modified_at"`
}

// FileListResponse represents the response body for listing project files.
type FileListResponse struct {
	// Files is the list of file and directory entries.
	Files []FileEntry `json:"files"`
	// Total is the total number of entries.
	Total int `json:"total"`
	// ProjectPath is the root path of the project.
	ProjectPath string `json:"project_path"`
}

// FileContentResponse represents the response body for file content.
type FileContentResponse struct {
	// Path is the relative path of the file.
	Path string `json:"path"`
	// Content is the file content as a string.
	Content string `json:"content"`
	// Size is the file size in bytes.
	Size int `json:"size"`
	// ContentType is the MIME type of the file content.
	ContentType string `json:"content_type"`
}

// ProjectStore provides in-memory storage for projects and their file metadata.
type ProjectStore struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

// Project represents an open project in memory.
type Project struct {
	// ID is the unique identifier.
	ID string `json:"id"`
	// Name is the project name.
	Name string `json:"name"`
	// Path is the filesystem path to the project directory.
	Path string `json:"path"`
	// Type is the detected project type.
	Type string `json:"type"`
	// FileCount is the cached file count.
	FileCount int `json:"file_count"`
	// CreatedAt is when the project was opened.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the project was last refreshed.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewProjectStore creates a new ProjectStore instance.
func NewProjectStore() *ProjectStore {
	return &ProjectStore{
		projects: make(map[string]*Project),
	}
}

// CreateProject creates or opens a project at the given path.
func (s *ProjectStore) CreateProject(name, projectPath, projectType string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate the path exists
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("project: failed to resolve path %q: %w", projectPath, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("project: path %q does not exist: %w", projectPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("project: %q is not a directory", projectPath)
	}

	// Generate or use provided name
	projectName := name
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	// Detect project type if not specified
	detectedType := projectType
	if detectedType == "" {
		detectedType = detectProjectTypeFromDir(absPath)
	}

	// Count files
	fileCount, err := countFiles(absPath)
	if err != nil {
		return nil, fmt.Errorf("project: failed to count files in %q: %w", projectPath, err)
	}

	project := &Project{
		ID:        fmt.Sprintf("proj-%d", time.Now().UnixNano()),
		Name:      projectName,
		Path:      absPath,
		Type:      detectedType,
		FileCount: fileCount,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.projects[project.ID] = project
	return project, nil
}

// GetProject retrieves a project by its ID.
func (s *ProjectStore) GetProject(id string) (*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id == "" {
		return nil, fmt.Errorf("project: ID is required")
	}

	project, ok := s.projects[id]
	if !ok {
		return nil, fmt.Errorf("project: %q not found", id)
	}

	// Return a copy
	return copyProject(project), nil
}

// ListProjects returns all projects.
func (s *ProjectStore) ListProjects() []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Project, 0, len(s.projects))
	for _, project := range s.projects {
		result = append(result, copyProject(project))
	}

	// Sort by creation time, newest first
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// ListProjectFiles lists files in a project's directory.
func (s *ProjectStore) ListProjectFiles(projectID string, relativePath string) ([]FileEntry, error) {
	s.mu.RLock()
	project, ok := s.projects[projectID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("project: %q not found", projectID)
	}

	dirPath := project.Path
	if relativePath != "" {
		dirPath = filepath.Join(project.Path, relativePath)
	}

	entries, err := scanDirectory(dirPath)
	if err != nil {
		return nil, fmt.Errorf("project: failed to list files in %q: %w", dirPath, err)
	}

	// Convert to FileEntry slice
	fileEntries := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		relPath := entry.relPath
		if relativePath != "" {
			relPath = filepath.Join(relativePath, entry.relPath)
		}
		fileEntries = append(fileEntries, FileEntry{
			Name:       entry.name,
			Path:       relPath,
			IsDir:      entry.isDir,
			Size:       entry.size,
			ModifiedAt: entry.modifiedAt,
		})
	}

	// Sort: directories first, then files, both alphabetically
	sort.Slice(fileEntries, func(i, j int) bool {
		if fileEntries[i].IsDir != fileEntries[j].IsDir {
			return fileEntries[i].IsDir
		}
		return fileEntries[i].Name < fileEntries[j].Name
	})

	return fileEntries, nil
}

// GetFileContent retrieves the content of a file in a project.
func (s *ProjectStore) GetFileContent(projectID string, filePath string) (*FileContentResponse, error) {
	s.mu.RLock()
	project, ok := s.projects[projectID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("project: %q not found", projectID)
	}

	// Sanitize the file path to prevent directory traversal
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("project: invalid file path: %q", filePath)
	}

	absPath := filepath.Join(project.Path, cleanPath)

	// Verify the file is within the project directory
	absProjectPath, _ := filepath.Abs(project.Path)
	absFilePath, _ := filepath.Abs(absPath)
	if !strings.HasPrefix(absFilePath, absProjectPath) {
		return nil, fmt.Errorf("project: file path escapes project directory")
	}

	// Check if it's a file (not a directory)
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("project: file not found: %q", filePath)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("project: %q is a directory, not a file", filePath)
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("project: failed to read file %q: %w", filePath, err)
	}

	// Determine content type
	contentType := detectContentType(filePath, content)

	return &FileContentResponse{
		Path:        cleanPath,
		Content:     string(content),
		Size:        len(content),
		ContentType: contentType,
	}, nil
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

// ─── Store Methods ────────────────────────────────────────────────────────────

// copyProject creates a deep copy of a Project.
func copyProject(p *Project) *Project {
	if p == nil {
		return nil
	}
	return &Project{
		ID:        p.ID,
		Name:      p.Name,
		Path:      p.Path,
		Type:      p.Type,
		FileCount: p.FileCount,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

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

// ─── Handler ──────────────────────────────────────────────────────────────────

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

	writeJSON(w, http.StatusOK, ProjectListResponse{
		Projects: summaries,
		Total:    len(summaries),
	})
}

// CreateProject handles POST /api/projects
// Creates or opens a project from the given path.
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req ProjectCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	project, err := h.projectStore.CreateProject(req.Name, req.Path, req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, ProjectResponse{
		ID:        project.ID,
		Name:      project.Name,
		Path:      project.Path,
		Type:      project.Type,
		FileCount: project.FileCount,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	})
}

// ListFiles handles GET /api/projects/:id/files
// Returns a list of files and directories in the project.
func (h *ProjectHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	projectID := extractProjectID(r.URL.Path)
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	// Extract optional subdirectory path from the URL
	// Expected format: /api/projects/{id}/files or /api/projects/{id}/files/{subpath}
	relativePath := extractSubPath(r.URL.Path, "/api/projects/"+projectID+"/files")

	entries, err := h.projectStore.ListProjectFiles(projectID, relativePath)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, FileListResponse{
		Files:       entries,
		Total:       len(entries),
		ProjectPath: projectID,
	})
}

// GetFileContent handles GET /api/projects/:id/files/:path
// Returns the content of a file in the project.
func (h *ProjectHandler) GetFileContent(w http.ResponseWriter, r *http.Request) {
	projectID := extractProjectID(r.URL.Path)
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	// Extract file path from the URL
	// Expected format: /api/projects/{id}/files/{path}
	filePath := extractSubPath(r.URL.Path, "/api/projects/"+projectID+"/files")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "file path is required")
		return
	}

	content, err := h.projectStore.GetFileContent(projectID, filePath)
	if err != nil {
		status := http.StatusNotFound
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "escapes") {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, content)
}

// ─── Path Extraction ──────────────────────────────────────────────────────────

// extractProjectID extracts the project ID from the URL path.
// Expected format: /api/projects/{id}...
func extractProjectID(path string) string {
	parts := splitPath(path)
	if len(parts) < 3 {
		return ""
	}
	// parts: ["", "api", "projects", "{id}", ...]
	return parts[3]
}

// extractSubPath extracts the sub-path after a known prefix.
// For example, given path="/api/projects/123/files/src/main.go" and
// prefix="/api/projects/123/files", it returns "src/main.go".
func extractSubPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	subPath := strings.TrimPrefix(path, prefix)
	subPath = strings.TrimPrefix(subPath, "/")
	return subPath
}

// splitPath splits a URL path into its components.
func splitPath(path string) []string {
	if path == "/" {
		return []string{""}
	}
	path = cleanPath(path)
	if path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return []string{""}
	}
	return split(path, '/')
}

// cleanPath removes redundant slashes from the path.
func cleanPath(path string) string {
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		path = "/" + path
	}
	n := len(path)
	for i := 1; i < n-1; {
		if path[i] == '/' && path[i+1] == '/' {
			path = path[:i+1] + path[i+2:]
			n--
		} else {
			i++
		}
	}
	return path
}

// split splits a string by a separator into a slice of substrings.
func split(s string, sep rune) []string {
	var result []string
	var current []rune
	for _, r := range s {
		if r == sep {
			result = append(result, string(current))
			current = nil
		} else {
			current = append(current, r)
		}
	}
	result = append(result, string(current))
	return result
}

// ─── HTTP Helpers ─────────────────────────────────────────────────────────────

// writeJSON writes a JSON response to the HTTP writer.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error JSON response to the HTTP writer.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// ErrorResponse represents an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}
