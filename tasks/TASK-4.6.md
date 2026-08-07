# Task 4.6: Simplify IDE Template

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/api/templates/ide.html` to simplify layout and add chat input area.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/api/templates/ide.html` - Simplify layout"
- Part of the simplified IDE-app design
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/api/templates/ide.html` |

## Changes Required
### Remove/Simplify
- Activity/phase info panel (right sidebar) - or simplify to just phase tracker
- Complex layout elements

### Keep
- File tree (left)
- Editor (center)

### Add
- Chat input area (bottom or right panel)

### Simplify Footer
- Show: connected status, current phase

## Impact
- **High**: Main UI layout
- Must maintain usability with simplified layout

## Dependencies
- **Before**: Task 4.5 (base template simplified)
- **After**: Proceed to Task 4.7

## Risk
🔴 **High** - main UI layout

## Checklist
- [ ] Remove/simplify right sidebar (phase info panel)
- [ ] Keep file tree (left) and editor (center)
- [ ] Add chat input area
- [ ] Simplify footer to show connected status and current phase
- [ ] Verify IDE layout is functional
