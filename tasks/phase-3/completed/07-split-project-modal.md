# Task 3.7: Split internal/api/templates/components/project-modal.html

## Goal
Split 364-line file into 2 files: modal overlay + form.

## Files to CREATE
- `internal/api/templates/components/project-modal.html` (~150 lines) — Modal overlay
- `internal/api/templates/components/project-form.html` (~210 lines) — Form fields

## Files to DELETE
- `internal/api/templates/components/project-modal.html` (original)

---

## File 1: project-modal.html (~150 lines)

**Keep:**
- Modal overlay structure
- Backdrop
- Modal container
- Header (title + close button)
- Footer (cancel + submit buttons)
- Modal open/close JavaScript
- Modal CSS

```html
{{ define "project-modal" }}
<div id="project-modal" class="modal-overlay hidden">
    <div class="modal-backdrop" onclick="closeProjectModal()"></div>
    <div class="modal-container">
        <div class="modal-header">
            <h2>Open Project</h2>
            <button onclick="closeProjectModal()">×</button>
        </div>
        {{ template "project-form" . }}
        <div class="modal-footer">
            <button onclick="closeProjectModal()">Cancel</button>
            <button onclick="submitProjectForm()">Open</button>
        </div>
    </div>
</div>
{{ end }}
```

---

## File 2: project-form.html (~210 lines)

**Move here:**
- Form fields:
  - Project path input
  - Project type selector
  - Feature request textarea
  - Validation messages
- Form validation JavaScript
- Form submit JavaScript
- Form CSS

```html
{{ define "project-form" }}
<form id="project-form" class="project-form">
    <div class="form-group">
        <label>Project Path</label>
        <input type="text" id="project-path" value="{{ .ProjectPath }}" />
        <span class="form-error" id="path-error"></span>
    </div>
    <div class="form-group">
        <label>Project Type</label>
        <select id="project-type">
            <option value="go">Go</option>
            <option value="python">Python</option>
            ...
        </select>
    </div>
    <div class="form-group">
        <label>Feature Request</label>
        <textarea id="feature-request" rows="4">{{ .FeatureRequest }}</textarea>
    </div>
</form>
{{ end }}
```

---

## Implementation Steps
1. Create `project-form.html` — extract form
2. Rewrite `project-modal.html` — keep overlay structure
3. Delete original `project-modal.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 210 lines
- Modal opens/closes correctly
- Form validates correctly
