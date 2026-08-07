# Task 4.4: Delete Unused CSS Files

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete 5 unused CSS files to reduce stylesheets.

## CLEANUP_PLAN Verification
- **Plan says**: "Files to DELETE (CSS): 5 CSS files listed"
- **Plan says**: "Keep: main.css, editor.css, sidebar.css, phase-tracker.css, modal.css, toast.css, header.css"
- ✅ This task directly fulfills the plan requirement

## Files to Delete
| File |
|------|
| `internal/api/static/css/accessibility.css` |
| `internal/api/static/css/activity-log.css` |
| `internal/api/static/css/dashboard.css` |
| `internal/api/static/css/skills.css` |
| `internal/api/static/css/responsive.css` |

## Files to Keep
- `main.css`
- `editor.css`
- `sidebar.css`
- `phase-tracker.css`
- `modal.css`
- `toast.css`
- `header.css`

## Impact
- **Low**: 5 CSS files removed, ~25KB

## Dependencies
- **Before**: Task 4.3 (editor template deleted)
- **After**: Proceed to Task 4.5

## Risk
🟢 **Low**

## Checklist
- [ ] Delete all 5 unused CSS files
- [ ] Verify no HTML templates import deleted CSS files
- [ ] Ensure remaining styles are sufficient
