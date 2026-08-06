# Task 4.1: Split internal/api/static/css/main.css

## Goal
Split 2742-line CSS file into 12 focused files.

## Files to CREATE
- `internal/api/static/css/main.css` (~500 lines) — Base styles, utilities, variables
- `internal/api/static/css/header.css` (~200 lines) — Header styles
- `internal/api/static/css/sidebar.css` (~300 lines) — Sidebar, file tree styles
- `internal/api/static/css/editor.css` (~400 lines) — Editor, code display styles
- `internal/api/static/css/dashboard.css` (~300 lines) — Dashboard, phase tracker styles
- `internal/api/static/css/activity-log.css` (~200 lines) — Activity log styles
- `internal/api/static/css/phase-tracker.css` (~250 lines) — Phase tracker styles
- `internal/api/static/css/modal.css` (~200 lines) — Modal styles
- `internal/api/static/css/toast.css` (~150 lines) — Toast/notification styles
- `internal/api/static/css/accessibility.css` (~200 lines) — Accessibility panel styles
- `internal/api/static/css/skills.css` (~150 lines) — Skills library styles
- `internal/api/static/css/responsive.css` (~900 lines) — Responsive breakpoints

## Files to DELETE
- `internal/api/static/css/main.css` (original)

---

## File 1: main.css (~500 lines)

**Keep:**
- CSS custom properties (variables)
- `@import` statements for other CSS files
- `*` universal selector
- `html`, `body` base styles
- Typography
- Flexbox/Grid utilities
- Common button styles
- Common input styles
- Scrollbar styles
- Color classes
- Spacing utilities
- Animation keyframes

```css
/* main.css */
@import 'header.css';
@import 'sidebar.css';
@import 'editor.css';
@import 'dashboard.css';
@import 'activity-log.css';
@import 'phase-tracker.css';
@import 'modal.css';
@import 'toast.css';
@import 'accessibility.css';
@import 'skills.css';
@import 'responsive.css';

:root {
    /* CSS variables */
    --bg-primary: #1a1b26;
    --bg-secondary: #24283b;
    --bg-tertiary: #292e42;
    --text-primary: #c0caf5;
    --text-secondary: #565f89;
    --accent: #7aa2f7;
    --accent-hover: #5d8af4;
    --success: #9ece6a;
    --warning: #e0af68;
    --error: #f7768e;
    /* ... more variables */
}

* { margin: 0; padding: 0; box-sizing: border-box; }
html { font-size: 14px; }
body { ... }

/* Utilities */
.flex { display: flex; }
.flex-col { flex-direction: column; }
.items-center { align-items: center; }
.justify-between { justify-content: space-between; }
.gap-1 { gap: 4px; }
.gap-2 { gap: 8px; }
.hidden { display: none; }
/* ... more utilities */
```

---

## File 2: header.css (~200 lines)

**Extract:**
- `#header-bar` styles
- `.phase-indicator` styles
- `.phase-badge` styles
- `.phase-progress` styles
- `.phase-dot` styles
- Session control button styles
- Settings button styles

---

## File 3: sidebar.css (~300 lines)

**Extract:**
- `#sidebar` styles
- `.file-tree` styles
- `.tree-item` styles
- `.tree-icon` styles
- `.tree-name` styles
- Sidebar toggle styles
- File tree search styles

---

## File 4: editor.css (~400 lines)

**Extract:**
- `.editor-container` styles
- `.code-display` styles
- `.code-block` styles
- `.file-ops` styles
- `.file-op` styles
- Syntax highlighting classes
- Editor toolbar styles

---

## File 5: dashboard.css (~300 lines)

**Extract:**
- `.dashboard` styles
- `.dashboard-panel` styles
- `.dashboard-header` styles
- `.dashboard-content` styles
- Agent status styles
- Test results styles

---

## File 6: activity-log.css (~200 lines)

**Extract:**
- `.activity-log` styles
- `.log-toolbar` styles
- `.log-entries` styles
- `.log-entry` styles
- `.entry-time`, `.entry-level`, `.entry-source`, `.entry-message` styles
- Filter button styles

---

## File 7: phase-tracker.css (~250 lines)

**Extract:**
- `.phase-tracker` styles
- `.tracker-nav` styles
- `.phase-list` styles
- `.phase-node` styles
- `.node-header`, `.node-step`, `.node-name`, `.node-status` styles
- `.node-progress` styles
- `.progress-bar` styles

---

## File 8: modal.css (~200 lines)

**Extract:**
- `.modal-overlay` styles
- `.modal-backdrop` styles
- `.modal-container` styles
- `.modal-header`, `.modal-footer` styles
- `.project-form` styles
- Form field styles

---

## File 9: toast.css (~150 lines)

**Extract:**
- `.toast-container` styles
- `.toast` styles
- `.toast-success`, `.toast-error`, `.toast-warning`, `.toast-info` styles
- Toast animation styles

---

## File 10: accessibility.css (~200 lines)

**Extract:**
- `.accessibility-panel` styles
- `.test-category` styles
- `.test-item` styles
- `.test-result` styles
- `.pass`, `.fail` styles

---

## File 11: skills.css (~150 lines)

**Extract:**
- `.skills-library` styles
- `.skills-grid` styles
- `.skill-card` styles
- `.skill-badge` styles
- `.skill-tags` styles

---

## File 12: responsive.css (~900 lines)

**Extract:**
- All `@media` queries
- Responsive breakpoints (mobile, tablet, desktop)
- Responsive layout adjustments

---

## Implementation Steps
1. Create `main.css` — extract variables, utilities, imports
2. Create each component CSS file — extract component-specific styles
3. Create `responsive.css` — extract all media queries
4. Delete original `main.css`
5. Update HTML templates to include individual CSS files:
   ```html
   <link rel="stylesheet" href="/static/css/main.css">
   ```
   (main.css now imports all other CSS files)

## Verification
- `go build ./...` succeeds
- All 12 files exist
- No file exceeds 500 lines (responsive.css can be up to 900)
- Page renders correctly with same styling
- `grep -r "\.phase-indicator\|\.file-tree\|\.modal-overlay" internal/api/static/css/` — verify styles are in correct files
