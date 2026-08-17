# Task 2.5 — Expose project and file-information APIs

**Phase:** 2 — Project analysis backend  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Medium

## Goal

Expose the import, current-project, and selected-file operations needed by both UIs.

## Plan coverage

- Add `POST /api/projects/import`.
- Add `GET /api/projects/current`.
- Add `GET /api/projects/current/files/info?path=...`.

## Dependencies

- Task 2.1
- Task 2.4

## Files

- `internal/api/handlers/project_handler.go`
- `cmd/daemon/main.go`
- `internal/project/manager.go`

## Implementation steps

1. Decode imports from `{"project_path":"..."}` and reject unknown JSON fields.
2. Analyze the requested directory before atomically activating it.
3. Return the complete `Analysis` payload after import.
4. Return the cached active analysis from the current-project endpoint.
5. Resolve and return safe selected-file metadata from the file-info endpoint.
6. Register method-specific routes in the daemon mux.
7. Return consistent JSON application errors for invalid paths, missing analysis, and unreadable files.

## Acceptance criteria

- Importing a valid directory writes analysis, activates the root, and returns metadata.
- A failed import does not partially activate an invalid project.
- Current-project returns the latest successfully imported analysis.
- File info accepts only paths inside the active project.

## Verification

```bash
go test ./internal/api/handlers ./internal/project
go vet ./cmd/daemon
```
