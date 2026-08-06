# Task 3.16: Remove model references from phase templates

## Goal
Remove any remaining model/provider references from all phase templates.

## Files to Modify
- `internal/api/templates/phases/coding.html`
- `internal/api/templates/phases/testing.html`
- `internal/api/templates/phases/review.html`
- `internal/api/templates/phases/human-review.html`

## Implementation Steps

### Step 1: Search for model references
```bash
grep -r "ModelName\|ProviderName\|Temperature\|MaxTokens\|TokenUsage" internal/api/templates/phases/
```

### Step 2: Remove any found references
Replace/remove:
- `{{ .ModelName }}` → remove entirely
- `{{ .ProviderName }}` → remove entirely
- `{{ .Temperature }}` → remove entirely
- `{{ .MaxTokens }}` → remove entirely
- `{{ .TokenUsage }}` → remove entirely

### Step 3: Remove model display from phase headers
If phases have a model/provider badge in the header, remove it:
```html
<!-- Remove this pattern -->
<div class="phase-model-info">
    <span>{{ .ModelName }}</span>
    <span>{{ .ProviderName }}</span>
</div>
```

## Verification
- `grep -r "ModelName\|ProviderName" internal/api/templates/` returns no results
- `go build ./...` succeeds
- Phase rendering works correctly
