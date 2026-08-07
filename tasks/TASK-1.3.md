# Task 1.3: Delete Session API Types

## Phase
Phase 1: Remove Session System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/api/session_types.go` to remove session-related API type definitions.

## CLEANUP_PLAN Verification
- **Plan says**: "DELETE: `internal/api/session_types.go` - Session API types"
- Part of "Remove multi-session management with a single 'active session' concept"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/api/session_types.go` |

## Types to Remove
- `SessionStore`
- `SessionCreateRequest`
- `SessionResponse`
- Any other session-related API types

## Impact
- **Medium**: `SessionStore` used in `main.go` and render handlers
- All references must be removed

## Dependencies
- **Before**: Task 1.2 (session handlers deleted)
- **After**: Proceed to Task 1.4

## Risk
🟡 **Medium** - `SessionStore` used in main.go and render handlers

## Checklist
- [ ] Delete `internal/api/session_types.go`
- [ ] Remove `SessionStore` references from `main.go`
- [ ] Remove `SessionStore` references from render handlers
- [ ] Ensure no dangling type references remain
