package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// ─── Project Store ────────────────────────────────────────────────────────────

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

	// Resolve the path - try multiple strategies for robustness
	absPath, err := resolveProjectPath(projectPath)
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
