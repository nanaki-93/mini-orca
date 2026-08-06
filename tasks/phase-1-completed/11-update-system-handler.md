# Task 1.11: Update internal/api/handlers/system.go

## Goal
No model/provider info in system info endpoint. Currently system.go has no model references.

## Files to Modify
- `internal/api/handlers/system.go`

## Current State
```go
type SystemInfoResponse struct {
    CWD     string `json:"cwd"`
    HomeDir string `json:"home_dir"`
    Version string `json:"version"`
}
```

## Implementation
**No changes needed.** This file has no model/provider references.

## Verification
- File remains unchanged
- `go build ./internal/api/handlers/...` succeeds
