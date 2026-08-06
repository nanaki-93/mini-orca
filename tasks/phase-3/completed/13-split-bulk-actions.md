# Task 3.13: Split internal/api/templates/components/bulk-actions.html

## Goal
Split 337-line file into 2 files: container + action item rendering.

## Files to CREATE
- `internal/api/templates/components/bulk-actions.html` (~130 lines) — Container
- `internal/api/templates/components/bulk-action-item.html` (~207 lines) — Action item

## Files to DELETE
- `internal/api/templates/components/bulk-actions.html` (original)

---

## File 1: bulk-actions.html (~130 lines)

**Keep:**
- Actions container div
- Action category headers
- Loop over actions: `{{ range .Actions }}`

```html
{{ define "bulk-actions" }}
<div id="bulk-actions" class="bulk-actions">
    {{ range .Actions }}
    {{ template "bulk-action-item" . }}
    {{ end }}
</div>
{{ end }}
```

---

## File 2: bulk-action-item.html (~207 lines)

**Move here:**
- `{{ define "bulk-action-item" }}` — Individual action item
- Item content: icon, name, description, execute button
- Action-specific rendering
- Item CSS

```html
{{ define "bulk-action-item" }}
<div class="action-item" data-action="{{ .Name }}">
    <div class="action-header">
        <span class="action-icon">{{ .Icon }}</span>
        <span class="action-name">{{ .Name }}</span>
    </div>
    <p class="action-description">{{ .Description }}</p>
    <button class="action-execute" onclick="executeAction('{{ .Name }}')">Execute</button>
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `bulk-action-item.html` — extract item template
2. Rewrite `bulk-actions.html` — keep container + range loop
3. Delete original `bulk-actions.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 207 lines
- Bulk actions render correctly
