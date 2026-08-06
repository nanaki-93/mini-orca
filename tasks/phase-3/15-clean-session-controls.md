# Task 3.15: Clean session-controls.html and mobile-sidebar.html

## Goal
Remove any model/provider references from these files. Keep as-is otherwise.

## Files to Modify
- `internal/api/templates/components/session-controls.html`
- `internal/api/templates/components/mobile-sidebar.html`

## Implementation Steps

### Step 1: Check session-controls.html
Search for:
- `{{ .ModelName }}`
- `{{ .ProviderName }}`
- `{{ .Temperature }}`
- `{{ .MaxTokens }}`

If any found, remove them.

### Step 2: Check mobile-sidebar.html
Search for same patterns. Remove if found.

### Step 3: Both files are already small (< 100 lines)
No splitting needed, just cleanup.

## Verification
- No model/provider references remain
- `go build ./...` succeeds
- Files remain small
