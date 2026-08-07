# Task 4.1: Delete Unused Component Templates

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Completed
- **Date**: 2025-08-07
- **All 16 files deleted successfully**
- **Removed dangling reference in base_modals.html**
- **Build verified: ✅ passes**

## Goal
Delete 16 unused component template files to reduce template bloat.

## CLEANUP_PLAN Verification
- **Plan says**: "Files to DELETE (components): 16 component files listed"
- **Plan says**: "Components to KEEP: file-tree, phase-tracker, activity, feedback, header"
- ✅ This task directly fulfills the plan requirement

## Files to Delete
| File |
|------|
| `internal/api/templates/components/accessibility-results.html` |
| `internal/api/templates/components/accessibility-testing.html` |
| `internal/api/templates/components/agent-skill-badge.html` |
| `internal/api/templates/components/agent-skills.html` |
| `internal/api/templates/components/bulk-action-item.html` |
| `internal/api/templates/components/bulk-actions.html` |
| `internal/api/templates/components/dashboard.html` |
| `internal/api/templates/components/header_settings.html` |
| `internal/api/templates/components/mobile-sidebar.html` |
| `internal/api/templates/components/project-form.html` |
| `internal/api/templates/components/project-modal.html` |
| `internal/api/templates/components/session-controls.html` |
| `internal/api/templates/components/skill-card.html` |
| `internal/api/templates/components/skill-field.html` |
| `internal/api/templates/components/skill-form.html` |
| `internal/api/templates/components/skills-library.html` |

## Files to Keep (for reference)
- `file-tree.html`, `file-tree-item.html`, `file-tree-search.html`
- `phase-tracker.html`, `phase-node.html`
- `activity-entry.html`, `activity-log.html`
- `feedback-input.html`
- `header.html`

## Impact
- **Low**: These components are unused
- ~16 files removed, ~100KB

## Dependencies
- **Before**: Start here (Phase 4 start)
- **After**: Proceed to Task 4.2

## Risk
🟢 **Low** - these components are unused

## Checklist
- [x] Delete all 16 component template files
- [x] Verify no templates reference deleted components
- [x] Ensure remaining components still render correctly
