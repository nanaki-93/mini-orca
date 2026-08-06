# Task 3.8: Split Phase Templates

## Goal
Split all phase templates into structure/controls + content display.

## Files to CREATE
- `internal/api/templates/phases/coding-structure.html` (~150 lines)
- `internal/api/templates/phases/coding-content.html` (~150 lines)
- `internal/api/templates/phases/testing-structure.html` (~150 lines)
- `internal/api/templates/phases/testing-content.html` (~150 lines)
- `internal/api/templates/phases/review-structure.html` (~150 lines)
- `internal/api/templates/phases/review-content.html` (~150 lines)
- `internal/api/templates/phases/human-review.html` (~250 lines)

## Files to DELETE
- `internal/api/templates/phases/coding.html`
- `internal/api/templates/phases/testing.html`
- `internal/api/templates/phases/review.html`
- `internal/api/templates/phases/human-review.html` (original)

---

## For each phase (coding, testing, review):

### File X: phase-structure.html (~150 lines)
**Keep:**
- Phase container div
- Phase header (title, description)
- Phase controls (start, stop, retry buttons)
- Phase progress indicator
- Phase-specific controls

**Remove:**
- Code/file display content (move to content file)
- Model/provider references

### File X: phase-content.html (~150 lines)
**Move here:**
- Code display area
- File operations list
- Generated code blocks
- Editor/preview sections
- Model/provider display (remove)

---

## For human-review.html (~250 lines):
Split into:
- `human-review-structure.html` (~100 lines) — Container + controls
- `human-review-content.html` (~150 lines) — Review input + approval buttons

---

## Implementation Steps
1. Create coding-structure.html — extract structure from coding.html
2. Create coding-content.html — extract content from coding.html
3. Delete coding.html
4. Repeat for testing, review, human-review
5. Remove any `{{ .ModelName }}`, `{{ .ProviderName }}`, `{{ .Temperature }}` references

## Verification
- `go build ./...` succeeds
- All 8 files exist
- No file exceeds 250 lines
- Phase rendering works correctly
