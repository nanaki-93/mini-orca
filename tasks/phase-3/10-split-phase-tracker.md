# Task 3.10: Split internal/api/templates/components/phase-tracker.html

## Goal
Split 275-line file into 2 files: tracker container + node rendering.

## Files to CREATE
- `internal/api/templates/components/phase-tracker.html` (~120 lines) — Tracker container
- `internal/api/templates/components/phase-node.html` (~155 lines) — Node rendering

## Files to DELETE
- `internal/api/templates/components/phase-tracker.html` (original)

---

## File 1: phase-tracker.html (~120 lines)

**Keep:**
- Tracker container div
- Navigation controls (previous/next)
- Phase summary
- Loop over phases: `{{ range .Phases }}`

```html
{{ define "phase-tracker" }}
<div id="phase-tracker" class="phase-tracker">
    <div class="tracker-nav">
        <button onclick="goToPhase('previous')">← Previous</button>
        <span>Phase {{ .CurrentPhase }} of {{ .TotalPhases }}</span>
        <button onclick="goToPhase('next')">Next →</button>
    </div>
    <div class="phase-list">
        {{ range .Phases }}
        {{ template "phase-node" . }}
        {{ end }}
    </div>
</div>
{{ end }}
```

---

## File 2: phase-node.html (~155 lines)

**Move here:**
- `{{ define "phase-node" }}` — Individual phase node
- Node content: phase name, status, progress bar, step indicator
- Status-specific styling (pending, in-progress, completed, failed)
- Node CSS

```html
{{ define "phase-node" }}
<div class="phase-node phase-{{ .Status }}" data-phase="{{ .Name }}">
    <div class="node-header">
        <span class="node-step">{{ .Step }}</span>
        <span class="node-name">{{ .Name | title }}</span>
        <span class="node-status">{{ .Status | title }}</span>
    </div>
    <div class="node-progress">
        <div class="progress-bar" style="width: {{ .Progress }}%"></div>
    </div>
    <span class="node-progress-text">{{ .Progress }}%</span>
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `phase-node.html` — extract node template
2. Rewrite `phase-tracker.html` — keep container + range loop
3. Delete original `phase-tracker.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 155 lines
- Phase tracker renders correctly
