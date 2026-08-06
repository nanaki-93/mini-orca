# Task 3.9: Split internal/api/templates/components/activity-log.html

## Goal
Split 378-line file into 2 files: log container + entry rendering.

## Files to CREATE
- `internal/api/templates/components/activity-log.html` (~150 lines) — Log container
- `internal/api/templates/components/activity-entry.html` (~230 lines) — Entry rendering

## Files to DELETE
- `internal/api/templates/components/activity-log.html` (original)

---

## File 1: activity-log.html (~150 lines)

**Keep:**
- Log container div
- Filter controls (all, errors, warnings, info)
- Clear button
- Log level selector
- Loop over entries: `{{ range .Entries }}`

```html
{{ define "activity-log" }}
<div id="activity-log" class="activity-log">
    <div class="log-toolbar">
        <button class="filter-btn active" data-level="all">All</button>
        <button class="filter-btn" data-level="error">Errors</button>
        <button class="filter-btn" data-level="warning">Warnings</button>
        <button class="filter-btn" data-level="info">Info</button>
        <button onclick="clearActivityLog()">Clear</button>
    </div>
    <div class="log-entries">
        {{ range .Entries }}
        {{ template "activity-entry" . }}
        {{ end }}
    </div>
</div>
{{ end }}
```

---

## File 2: activity-entry.html (~230 lines)

**Move here:**
- `{{ define "activity-entry" }}` — Individual entry
- Entry content: timestamp, level, message, source
- Level-specific styling (error=red, warning=yellow, info=blue)
- Entry CSS

```html
{{ define "activity-entry" }}
<div class="log-entry log-{{ .Level }}" data-level="{{ .Level }}">
    <span class="entry-time">{{ .Timestamp }}</span>
    <span class="entry-level">{{ .Level | upper }}</span>
    <span class="entry-source">{{ .Source }}</span>
    <span class="entry-message">{{ .Message }}</span>
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `activity-entry.html` — extract entry template
2. Rewrite `activity-log.html` — keep container + range loop
3. Delete original `activity-log.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 230 lines
- Activity log renders correctly
