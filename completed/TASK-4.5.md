# Task 4.5: Simplify Base Template

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Completed
- **Date**: 2025-08-07
- **Removed mobile sidebar toggle button** and **overlay** from base.html
- **Simplified sidebar** — removed transform/position classes (no longer a mobile drawer)
- **Simplified global loading overlay** — removed secondary spinner ring
- **Simplified toast notification container** — removed aria attributes
- **Removed screen reader live regions** (sr-announcements, sr-alerts) — replaced with console fallback in ScreenReader utility
- **Removed toggleMobileSidebar() function** and **Escape key handler** from base_scripts_error.html
- **Removed ScreenReader DOM element references** from base_scripts.html
- **Build verified: ✅ passes**
- **No dangling references to removed elements remain**

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
- [x] Remove mobile sidebar toggle and overlay
- [x] Simplify header block (remove settings)
- [x] Keep sidebar (file tree), main (editor + chat), footer (status bar)
- [x] Simplify global loading overlay
- [x] Simplify toast notification container
- [x] Verify base template renders correctly
