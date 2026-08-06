# Task 1.10: Update internal/api/handlers/htmx_render.go

## Goal
Remove model/provider references from RenderMainPage and template data.

## Files to Modify
- `internal/api/handlers/htmx_render.go`

## Implementation Steps

### Step 1: Update RenderMainPage() — remove model data
Find the `RenderMainPage` function and remove these lines:

**Delete:**
```go
data["ModelName"] = "gpt-4o"
data["ProviderName"] = "OpenAI"
```

### Step 2: Verify no other model references
Search for any remaining `model.` or `ModelName` or `ProviderName` references in this file.

## Verification
- `go build ./internal/api/handlers/...` succeeds
- No `model.` references in this file
- No hardcoded `ModelName` or `ProviderName` assignments
