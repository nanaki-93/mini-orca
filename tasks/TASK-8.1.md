# Task 8.1: Update Base Template Functions

## Phase
Phase 8: Final Cleanup & Testing

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/api/templates/base.html` to remove unnecessary JavaScript utilities.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/api/templates/base.html` - Simplify header/sidebar/main/footer"
- Part of final cleanup
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/api/templates/base.html` |

## Changes Required
### Remove
- HTMX batcher
- Debounce utilities

### Keep
- HTMX config
- Tailwind config

## Impact
- **Low**: JS utilities

## Dependencies
- **Before**: Start here (Phase 8 start)
- **After**: Proceed to Task 8.2

## Risk
🟢 **Low** - JS utilities

## Checklist
- [ ] Remove HTMX batcher
- [ ] Remove debounce utilities
- [ ] Keep HTMX config
- [ ] Keep Tailwind config
- [ ] Verify base template still works
