# Task 4.6: Simplify IDE Template

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Completed
- **Date**: 2025-08-07
- **Removed right sidebar** (info-panel with phase tracker + activity log) and dashboard polling trigger
- **Simplified main area** — editor now takes full width (no right panel split)
- **Added chat input area** — bottom panel with text input, send button, and response display
- **Footer simplified** — shows connected status and current phase (already simplified)
- **Build verified: ✅ passes**
- **No dangling references to removed elements remain**

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
- [x] Remove/simplify right sidebar (phase info panel)
- [x] Keep file tree (left) and editor (center)
- [x] Add chat input area
- [x] Simplify footer to show connected status and current phase
- [x] Verify IDE layout is functional
