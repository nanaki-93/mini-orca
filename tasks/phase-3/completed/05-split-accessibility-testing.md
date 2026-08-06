# Task 3.5: Split internal/api/templates/components/accessibility-testing.html

## Goal
Split 786-line file into 2 files: panel + results display.

## Files to CREATE
- `internal/api/templates/components/accessibility-testing.html` (~300 lines) — Panel + tests
- `internal/api/templates/components/accessibility-results.html` (~480 lines) — Results + reports

## Files to DELETE
- `internal/api/templates/components/accessibility-testing.html` (original)

---

## File 1: accessibility-testing.html (~300 lines)

**Keep:**
- Panel container
- Test controls (start/stop)
- Test categories
- Individual test definitions
- Test execution controls
- Test status indicators

---

## File 2: accessibility-results.html (~480 lines)

**Move here:**
- Results display section
- Pass/fail summary
- Detailed results per test
- Report generation
- Report history
- Export functionality

---

## Implementation Steps
1. Create `accessibility-results.html` — extract results sections
2. Rewrite `accessibility-testing.html` — keep panel and test controls
3. Delete original `accessibility-testing.html`

## Verification
- `go build ./...` succeeds
- Both files exist
- No file exceeds 480 lines
- Accessibility panel renders correctly
