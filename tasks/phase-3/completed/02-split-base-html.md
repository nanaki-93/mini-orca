# Task 3.2: Split internal/api/templates/base.html

## Goal
Split 1072-line file into 4 focused files.

## Files to CREATE
- `internal/api/templates/base.html` (~150 lines) — Main structure
- `internal/api/templates/base_styles.html` (~200 lines) — CSS
- `internal/api/templates/base_scripts.html` (~400 lines) — JavaScript
- `internal/api/templates/base_modals.html` (~320 lines) — Modals

## Files to DELETE
- `internal/api/templates/base.html` (original)

---

## File 1: base.html (~150 lines)

**Keep only:**
- `<!DOCTYPE html>` through `</html>` structure
- `<head>` section with meta tags, title, CSS includes
- `<body>` section with block definitions
- `{{ template }}` includes for styles, scripts, modals

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Mini Orca</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
</head>
<body>
    {{ template "base_styles" . }}
    {{ template "base_scripts" . }}
    {{ template "base_modals" . }}
    {{ block "content" . }}{{ end }}
</body>
</html>
```

---

## File 2: base_styles.html (~200 lines)

**Move here:**
- All `<style>` content from base.html
- CSS variables
- Global styles
- Scrollbar styles
- Utility classes

---

## File 3: base_scripts.html (~400 lines)

**Move here:**
- All `<script>` content from base.html
- HTMX configuration
- ScreenReader class
- FocusManager class
- Toast notification system
- Loading indicator management
- Error handling
- Keyboard shortcuts
- Utility functions (debounce, throttle, etc.)

---

## File 4: base_modals.html (~320 lines)

**Move here:**
- `{{ template "project-modal" . }}`
- Any other modal definitions
- Modal-related JavaScript

---

## Implementation Steps
1. Create `base_styles.html` — extract all `<style>` blocks
2. Create `base_scripts.html` — extract all `<script>` blocks
3. Create `base_modals.html` — extract modal templates
4. Rewrite `base.html` — keep only HTML structure + block definitions + template includes
5. Delete original `base.html`

## Verification
- `go build ./...` succeeds
- All 4 files exist
- No file exceeds 400 lines
- Page renders correctly in browser
