# Task 2.1: Delete Project Handlers

## Phase
Phase 2: Remove Project System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/api/handlers/project_handler.go` to remove project-related HTTP handlers.

## CLEANUP_PLAN Verification
- **Plan says**: "DELETE: `internal/api/handlers/project_handler.go` - Project HTTP handlers"
- **Plan says**: "Remove endpoints: `GET /api/projects`, `POST /api/projects`, `GET /api/projects/:id/files`"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/api/handlers/project_handler.go` |

## Endpoints to Remove
- `GET /api/projects`
- `POST /api/projects`
- `GET /api/projects/:id/files`

## Impact
- **Medium**: Project store used in render handler
- Route registrations in `main.go` must be removed

## Dependencies
- **Before**: Start here (Phase 2 start)
- **After**: Proceed to Task 2.2

## Risk
🟡 **Medium** - project store used in render handler

## Checklist
- [ ] Delete `internal/api/handlers/project_handler.go`
- [ ] Remove route registrations from `cmd/daemon/main.go`
- [ ] Remove project handler initialization from `main.go`
- [ ] Ensure no dangling references remain
