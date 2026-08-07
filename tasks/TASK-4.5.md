# Task 4.5: Simplify Base Template

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/api/templates/base.html` to remove unused UI elements and simplify the layout.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/api/templates/base.html` - Simplify header/sidebar/main/footer"
- Part of reducing UI complexity
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/api/templates/base.html` |

## Changes Required
### Remove
- Mobile sidebar toggle and overlay
- Settings from header block
- Global loading overlay (simplify)
- Toast notification container (keep but simplify)

### Keep
- Sidebar (file tree)
- Main (editor + chat)
- Footer (status bar)

## Impact
- **High**: Affects all pages
- Must ensure essential UI elements remain functional

## Dependencies
- **Before**: Task 4.4 (unused CSS deleted)
- **After**: Proceed to Task 4.6

## Risk
🔴 **High** - affects all pages

## Checklist
- [ ] Remove mobile sidebar toggle and overlay
- [ ] Simplify header block (remove settings)
- [ ] Keep sidebar (file tree), main (editor + chat), footer (status bar)
- [ ] Simplify global loading overlay
- [ ] Simplify toast notification container
- [ ] Verify base template renders correctly
