# Task 2.4: Split internal/api/handlers/project.go

## Goal
Split 679-line file into 3 focused files.

## Files to CREATE
- `internal/api/handlers/project_types.go`
- `internal/api/handlers/project_handler.go`
- `internal/api/handlers/project_files.go`

## Files to DELETE
- `internal/api/handlers/project.go`

---

## File 1: project_types.go (~150 lines)

**Move here:**
- `Project` struct
- `FileEntry` struct
- `ProjectStore` struct
- `NewProjectStore()`
- `CreateProject()`
- `GetProject()`
- `ListProjects()`
- `DeleteProject()`
- Helper functions for projects

```go
package handlers

type Project struct { ... }
type FileEntry struct { ... }

type ProjectStore struct { ... }
func NewProjectStore() *ProjectStore { ... }
func (s *ProjectStore) CreateProject(...) (*Project, error) { ... }
func (s *ProjectStore) GetProject(id string) (*Project, error) { ... }
func (s *ProjectStore) ListProjects() ([]*Project, error) { ... }
func (s *ProjectStore) DeleteProject(id string) error { ... }
```

---

## File 2: project_handler.go (~350 lines)

**Move here:**
- `ProjectHandler` struct
- `NewProjectHandler()`
- HTTP handlers:
  - `ListProjects()`
  - `CreateProject()`
  - Route dispatcher (the `switch action` block)

```go
package handlers

type ProjectHandler struct { ... }
func NewProjectHandler(projectStore *ProjectStore) *ProjectHandler { ... }
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) { ... }
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) { ... }
func (h *ProjectHandler) handleProjectRoute(w http.ResponseWriter, r *http.Request) { ... }
```

---

## File 3: project_files.go (~180 lines)

**Move here:**
- `ListFiles()` handler
- `GetFileContent()` handler
- File-related helper functions

```go
package handlers

func (h *ProjectHandler) ListFiles(w http.ResponseWriter, r *http.Request) { ... }
func (h *ProjectHandler) GetFileContent(w http.ResponseWriter, r *http.Request) { ... }
```

## Verification
- `go build ./internal/api/handlers/...` succeeds
- `go test ./internal/api/handlers/...` passes
- No file exceeds 350 lines
