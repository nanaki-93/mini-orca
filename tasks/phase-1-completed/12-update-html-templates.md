# Task 1.12: Update HTML Templates — Remove Model/Provider UI

## Goal
Remove all model/provider configuration UI from HTML templates.

## Files to Modify
- `internal/api/templates/components/header.html`
- `internal/api/templates/ide.html`

## Files to DELETE
- `internal/api/templates/components/model-config-panel.html`

---

## Changes for header.html

### Step 1: Remove Model Info section from header bar
**Delete this block:**
```html
<!-- Model Info -->
<div class="hidden lg:flex items-center gap-2 px-3 py-1.5 bg-dark-700 rounded-md">
    <div class="flex flex-col items-start">
        <span class="text-[10px] text-text-secondary uppercase tracking-wider">Model</span>
        <span class="text-xs text-text-primary font-medium">{{ .ModelName }}</span>
    </div>
    <div class="h-4 w-px bg-dark-600"></div>
    <div class="flex flex-col items-start">
        <span class="text-[10px] text-text-secondary uppercase tracking-wider">Provider</span>
        <span class="text-xs text-text-primary font-medium">{{ .ProviderName }}</span>
    </div>
</div>
```

### Step 2: Remove Token Usage section
**Delete this block:**
```html
<!-- Token Usage -->
{{ if .TokenUsage }}
<div class="hidden xl:flex items-center gap-1.5 px-2 py-1 bg-dark-700 rounded-md">
    ...
</div>
{{ end }}
```

### Step 3: Remove Settings panel model configuration
**In the settings panel section, delete:**
```html
<!-- Model Settings -->
<div class="mb-6">
    <h3 class="text-sm font-medium text-text-primary mb-3">Model Configuration</h3>
    <div class="space-y-3">
        <div>
            <label class="block text-xs text-text-secondary mb-1">Active Provider</label>
            <select class="...">
                {{ range .Providers }}
                <option value="{{ .Name }}" ...>{{ .Name }}</option>
                {{ end }}
            </select>
        </div>
        <div>
            <label class="block text-xs text-text-secondary mb-1">Model</label>
            <input type="text" value="{{ .ModelName }}" class="..." />
        </div>
    </div>
</div>

<!-- Temperature Settings -->
<div class="mb-6">
    <h3 class="text-sm font-medium text-text-primary mb-3">Generation Settings</h3>
    <div class="space-y-3">
        <div>
            <label class="text-xs text-text-secondary">Temperature</label>
            <input type="range" ... value="{{ .Temperature }}" />
        </div>
        <div>
            <label class="text-xs text-text-secondary">Max Tokens</label>
            <input type="number" value="{{ .MaxTokens }}" class="..." />
        </div>
    </div>
</div>
```

---

## Changes for ide.html

### Step 1: Remove model references from footer (status bar)
The footer currently shows model/provider. Remove any such references.

---

## Changes for model-config-panel.html

### Step 1: DELETE the file
```bash
rm internal/api/templates/components/model-config-panel.html
```

This entire component is for model configuration — no longer needed.

---

## Verification
- No `{{ .ModelName }}`, `{{ .ProviderName }}`, `{{ .Temperature }}`, `{{ .MaxTokens }}` in any template
- `model-config-panel.html` is deleted
- `go build ./...` succeeds (templates are loaded at runtime, no compile errors)
