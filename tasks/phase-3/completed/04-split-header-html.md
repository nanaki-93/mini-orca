# Task 3.4: Split internal/api/templates/components/header.html

## Goal
Split 348-line file into 2 files: header bar + settings panel.

## Files to CREATE
- `internal/api/templates/components/header.html` (~150 lines) — Header bar only
- `internal/api/templates/components/header_settings.html` (~200 lines) — Settings panel

## Files to DELETE
- `internal/api/templates/components/header.html` (original)

---

## File 1: header.html (~150 lines)

**Keep only:**
- The `{{ define "header-bar" }}` block
- Project info section
- Phase indicator section
- Session controls (pause/resume/stop buttons)
- Settings button (triggers settings panel toggle)
- New Session button
- All JavaScript for settings panel toggle
- All CSS for phase indicators

```html
{{ define "header-bar" }}
<div id="header-bar" class="...">
    <!-- Left: Project Info -->
    <div class="...">
        <!-- Logo -->
        <!-- Open Project Button -->
        <!-- Session ID -->
    </div>

    <!-- Center: Phase Indicator -->
    <div class="...">
        <!-- Phase Badge -->
        <!-- Phase Progress Bar -->
    </div>

    <!-- Right: Controls -->
    <div class="...">
        <!-- Pause/Resume/Start buttons -->
        <!-- Stop button -->
        <!-- Settings button -->
        <!-- New Session button -->
    </div>
</div>
{{ end }}
```

**Delete from this file:**
- `{{ define "settings-panel" }}` block (move to header_settings.html)
- Settings panel JavaScript (move to header_settings.html)
- Settings panel CSS (move to header_settings.html)

---

## File 2: header_settings.html (~200 lines)

**Move here:**
- `{{ define "settings-panel" }}` block
- Settings panel JavaScript (toggle, Escape key)
- Settings panel CSS

```html
{{ define "settings-panel" }}
<div id="settings-panel" class="...">
    <!-- Backdrop -->
    <div class="..." onclick="toggleSettingsPanel()"></div>
    <!-- Panel -->
    <div class="...">
        <!-- Panel Header -->
        <!-- Settings Content -->
        <!-- Save Button -->
    </div>
</div>
{{ end }}
```

**Note:** Remove any model/provider settings from the settings panel content (already done in task 1.12).

---

## Implementation Steps
1. Create `header_settings.html` — extract settings panel
2. Rewrite `header.html` — keep only header bar
3. Delete original `header.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 200 lines
- Settings panel toggles correctly
- Header renders correctly
