# Task 1.2: Delete internal/model/ package

## Goal
Remove the entire `internal/model/` package — no longer needed after config simplification.

## Files to DELETE
- `internal/model/provider.go`
- `internal/model/types.go`
- `internal/model/router.go`
- `internal/model/lm_studio.go`
- `internal/model/router_test.go`
- `internal/model/lm_studio_test.go`
- `internal/model/full_flow_test.go`

## Prerequisites
- Task 1.1 must be completed first
- All files importing `github.com/nanaki-93/mini-orca/v2/internal/model` must be updated first

## Verification Before Deleting
Run: `grep -r "internal/model" --include="*.go" .`
Ensure zero results (all imports removed by dependent tasks).

## Command to Delete
```bash
rm -rf internal/model/
```

## Verification After Deleting
- `go build ./...` succeeds (no import errors)
- `go test ./...` passes
