# Task 3.14: Split internal/api/templates/components/skill-form.html

## Goal
Split 195-line file into 2 files: form container + field rendering.

## Files to CREATE
- `internal/api/templates/components/skill-form.html` (~90 lines) — Form container
- `internal/api/templates/components/skill-field.html` (~105 lines) — Field rendering

## Files to DELETE
- `internal/api/templates/components/skill-form.html` (original)

---

## File 1: skill-form.html (~90 lines)

**Keep:**
- Form container
- Form header
- Loop over fields: `{{ range .Fields }}`
- Submit button
- Form validation JavaScript

```html
{{ define "skill-form" }}
<form id="skill-form" class="skill-form">
    <h3>{{ .Title }}</h3>
    {{ range .Fields }}
    {{ template "skill-field" . }}
    {{ end }}
    <div class="form-actions">
        <button type="submit">Save</button>
        <button type="button" onclick="closeSkillForm()">Cancel</button>
    </div>
</form>
{{ end }}
```

---

## File 2: skill-field.html (~105 lines)

**Move here:**
- `{{ define "skill-field" }}` — Individual field
- Field content: label, input/textarea/select, validation
- Field type-specific rendering
- Field CSS

```html
{{ define "skill-field" }}
<div class="form-field field-{{ .Type }}">
    <label for="{{ .ID }}">{{ .Label }}</label>
    {{ if eq .Type "text" }}
    <input type="text" id="{{ .ID }}" name="{{ .Name }}" value="{{ .Value }}" />
    {{ else if eq .Type "textarea" }}
    <textarea id="{{ .ID }}" name="{{ .Name }}">{{ .Value }}</textarea>
    {{ else if eq .Type "select" }}
    <select id="{{ .ID }}" name="{{ .Name }}">
        {{ range .Options }}
        <option value="{{ .Value }}" {{ if .Selected }}selected{{ end }}>{{ .Label }}</option>
        {{ end }}
    </select>
    {{ end }}
    {{ if .Error }}<span class="field-error">{{ .Error }}</span>{{ end }}
</div>
{{ end }}
```

---

## Implementation Steps
1. Create `skill-field.html` — extract field template
2. Rewrite `skill-form.html` — keep container + range loop
3. Delete original `skill-form.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 105 lines
- Skill form renders correctly
