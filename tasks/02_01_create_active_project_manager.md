# Task 2.1 — Create the thread-safe active-project manager

**Phase:** 2 — Project analysis backend  
**Status:** Complete  
**Priority:** High  
**Risk:** Medium

## Goal

Provide one synchronized source of truth for the project currently selected by either UI.

## Plan coverage

- Add a small, thread-safe active-project store shared by API, file tree, and generation handlers.

## Dependencies

- Task 1.2

## Files

- `internal/project/manager.go`
- `cmd/daemon/main.go`
- `internal/api/handlers/htmx_render.go`
- `internal/api/handlers/htmx_filetree.go`

## Implementation steps

1. Create a manager containing the canonical root and latest `Analysis` under `sync.RWMutex`.
2. Validate the configured initial project during daemon startup.
3. Add atomic `Root`, `Set`, and `Analysis` operations.
4. Return defensive copies of maps and file slices from `Analysis`.
5. Inject the manager into HTMX, project, and chat handlers rather than copying path strings.
6. Make the file tree read the root at request time so imports take effect without restarting the daemon.

## Acceptance criteria

- Concurrent readers and imports are race-free.
- Activating a project updates every handler's project root.
- Callers cannot mutate manager-owned maps or slices through a returned analysis object.
- An invalid initial configured path fails startup with a clear error.

## Verification

```bash
go test -race ./internal/project ./internal/api/handlers
go test ./cmd/daemon
```
