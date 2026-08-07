# Task 5.2: Remove Complex Phase Router

## Phase
Phase 5: Simplify Orchestrator

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/orchestrator/phase_router.go` and `internal/orchestrator/phase_router_test.go` to remove dynamic phase routing.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/orchestrator/phase_router.go` - Remove complex routing"
- **Plan says**: "Replace with simple linear flow"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/orchestrator/phase_router.go` |
| DELETE | `internal/orchestrator/phase_router_test.go` |

## Impact
- **Low**: Replaced by simple linear flow (Task 5.1)

## Dependencies
- **Before**: Task 5.1 (orchestrator simplified to RunOnce)
- **After**: Proceed to Task 5.3

## Risk
🟢 **Low** - replaced by simple linear flow

## Checklist
- [ ] Delete `phase_router.go`
- [ ] Delete `phase_router_test.go`
- [ ] Remove any imports of phase router
- [ ] Ensure orchestrator uses linear flow instead
