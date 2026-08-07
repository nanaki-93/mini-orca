# Task 1.7: Update main.go - Remove Session Store

## Phase
Phase 1: Remove Session System

## Status
✅ Complete

## Goal
Edit `cmd/daemon/main.go` to remove all session store initialization and session-related route registrations.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `cmd/daemon/main.go` - Remove session store initialization"
- **Plan says**: "Remove endpoints: `POST /api/sessions`, `GET /api/sessions`, `POST /api/sessions/:id/*`"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `cmd/daemon/main.go` |

## Changes Required
### Remove
- `sessionStore := api.NewSessionStore()`
- `gateStore := api.NewGateStore()`
- Phase router initialization
- All session-related route registrations
- `sessionHandler` initialization
- `gateHandler` initialization

### Keep
- `projectStore` (will be removed in Phase 2)
- `htmxRenderHandler`

## Impact
- **High**: Entry point of the application
- Must ensure all remaining routes and handlers work correctly

## Dependencies
- **Before**: Tasks 1.1-1.6 (all session code removed)
- **After**: Phase 1 complete → Proceed to Phase 2

## Risk
🔴 **High** - entry point

## Checklist
- [x] Remove `sessionStore` initialization
- [x] Remove `gateStore` initialization
- [x] Remove all session route registrations
- [x] Remove `sessionHandler` and `gateHandler` initialization
- [x] Keep `projectStore` and `htmxRenderHandler`
- [x] Build and verify no compilation errors
