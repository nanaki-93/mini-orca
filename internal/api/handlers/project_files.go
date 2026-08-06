package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
)

// ListFiles handles GET /api/projects/:id/files
// Returns a list of files and directories in the project.
func (h *ProjectHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	projectID := api.ExtractProjectID(r.URL.Path)
	if projectID == "" {
		api.WriteAppError(w, apperrors.BadRequest("project ID is required", "A project ID must be provided in the URL.", nil))
		return
	}

	// Extract optional subdirectory path from the URL
	// Expected format: /api/projects/{id}/files or /api/projects/{id}/files/{subpath}
	relativePath := api.ExtractSubPath(r.URL.Path, "/api/projects/"+projectID+"/files")

	entries, err := h.projectStore.ListProjectFiles(projectID, relativePath)
	if err != nil {
		api.WriteAppError(w, apperrors.NotFound("project files not found", "Could not list files for the specified project.", err))
		return
	}

	api.WriteJSON(w, http.StatusOK, FileListResponse{
		Files:       entries,
		Total:       len(entries),
		ProjectPath: projectID,
	})
}

// GetFileContent handles GET /api/projects/:id/files/:path
// Returns the content of a file in the project.
func (h *ProjectHandler) GetFileContent(w http.ResponseWriter, r *http.Request) {
	projectID := api.ExtractProjectID(r.URL.Path)
	if projectID == "" {
		api.WriteAppError(w, apperrors.BadRequest("project ID is required", "A project ID must be provided in the URL.", nil))
		return
	}

	// Extract file path from the URL
	// Expected format: /api/projects/{id}/files/{path}
	filePath := api.ExtractSubPath(r.URL.Path, "/api/projects/"+projectID+"/files")
	if filePath == "" {
		api.WriteAppError(w, apperrors.BadRequest("file path is required", "Please specify a file path within the project.", nil))
		return
	}

	content, err := h.projectStore.GetFileContent(projectID, filePath)
	if err != nil {
		status := http.StatusNotFound
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "escapes") {
			status = http.StatusBadRequest
		}

		errType := apperrors.TypeNotFound
		if status == http.StatusBadRequest {
			errType = apperrors.TypeBadRequest
		}

		api.WriteAppError(w, apperrors.New(errType, err.Error(), "Failed to retrieve file content.", status, err))
		return
	}

	api.WriteJSON(w, http.StatusOK, content)
}

// ─── Store Methods ────────────────────────────────────────────────────────────

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
