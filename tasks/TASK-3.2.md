# Task 3.2: Delete Skills API Handler

## Phase
Phase 3: Remove Skills System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/api/skills_handler.go` to remove skills-related HTTP endpoints.

## CLEANUP_PLAN Verification
- **Plan says**: "DELETE: `internal/api/skills_handler.go`"
- Part of removing skills system
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/api/skills_handler.go` |

## Endpoints to Remove
- `/api/skills/*` endpoints

## Impact
- **Low**: Skills are config-only now
- Route registrations in `main.go` must be removed

## Dependencies
- **Before**: Task 3.1 (skills directory deleted)
- **After**: Proceed to Task 3.3

## Risk
🟢 **Low** - skills are config-only now

## Checklist
- [ ] Delete `internal/api/skills_handler.go`
- [ ] Remove route registrations from `cmd/daemon/main.go`
- [ ] Remove skills handler initialization from `main.go`
- [ ] Ensure no dangling references remain
