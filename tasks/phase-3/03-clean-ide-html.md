# Task 3.3: Clean internal/api/templates/ide.html

## Goal
Remove any model/provider references from ide.html. Keep as-is otherwise.

## Files to Modify
- `internal/api/templates/ide.html`

## Current State
```html
{{ define "content" }}
{{ template "base" . }}
{{ template "ide-content" . }}
{{ end }}

{{ define "ide-content" }}
<div class="ide-container">
    {{ template "header-bar" . }}
    <div class="ide-body">
        {{ template "sidebar" . }}
        <div class="ide-main">
            {{ template "workspace" . }}
        </div>
        {{ template "footer" . }}
    </div>
</div>
{{ end }}
```

## Implementation Steps

### Step 1: Check for model/provider references
Search for any of:
- `{{ .ModelName }}`
- `{{ .ProviderName }}`
- `{{ .Temperature }}`
- `{{ .MaxTokens }}`
- `{{ .TokenUsage }}`

### Step 2: Remove any found references
If any are found, remove them. If none found, no changes needed.

### Step 3: The footer section
If the footer (status bar) shows model/provider info, remove it:
```html
<!-- Remove any such section from footer -->
<div class="...">
    <span>{{ .ModelName }}</span>
    <span>{{ .ProviderName }}</span>
</div>
```

## Verification
- No model/provider references remain
- `go build ./...` succeeds
- File remains small (< 100 lines)
