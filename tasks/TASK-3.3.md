# Task 3.3: Delete Skills API Types

## Phase
Phase 3: Remove Skills System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/api/skills_types.go` to remove skills-related API type definitions.

## CLEANUP_PLAN Verification
- **Plan says**: "DELETE: `internal/api/skills_types.go`"
- Part of removing skills system
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/api/skills_types.go` |

## Types to Remove
- `SkillResponse`
- `SkillListResponse`
- Any other skills-related API types

## Impact
- **Low**: Types no longer needed

## Dependencies
- **Before**: Task 3.2 (skills handler deleted)
- **After**: Proceed to Task 3.4

## Risk
🟢 **Low**

## Checklist
- [ ] Delete `internal/api/skills_types.go`
- [ ] Remove any references to deleted types
- [ ] Ensure no dangling type references remain
