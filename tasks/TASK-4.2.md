# Task 4.2: Delete Unused Phase Templates

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete unused phase template files and the completed phase directory.

## CLEANUP_PLAN Verification
- **Plan says**: "Files to DELETE (phases): completed/ directory and 9 phase template files"
- Part of reducing template count from ~60 to ~20
- ✅ This task directly fulfills the plan requirement

## Files to Delete
| File |
|------|
| `internal/api/templates/phases/completed/` (entire directory) |
| `internal/api/templates/phases/coding-content.html` |
| `internal/api/templates/phases/coding-structure.html` |
| `internal/api/templates/phases/testing-content.html` |
| `internal/api/templates/phases/testing-structure.html` |
| `internal/api/templates/phases/review-content.html` |
| `internal/api/templates/phases/review-structure.html` |
| `internal/api/templates/phases/human-review-content.html` |
| `internal/api/templates/phases/human-review-structure.html` |

## Impact
- **Medium**: Phase rendering depends on these
- Phase rendering logic must be simplified to use remaining templates

## Dependencies
- **Before**: Task 4.1 (unused components deleted)
- **After**: Proceed to Task 4.3

## Risk
🟡 **Medium** - phase rendering depends on these

## Checklist
- [ ] Delete `phases/completed/` directory
- [ ] Delete all 9 phase template files
- [ ] Update phase rendering logic to use simplified templates
- [ ] Ensure phase tracker still displays correctly
