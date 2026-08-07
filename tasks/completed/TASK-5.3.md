# Task 5.3: Simplify History Tracking

## Phase
Phase 5: Simplify Orchestrator

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/orchestrator/history.go` to replace complex history graph with simple phase log.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/orchestrator/history.go` - Simplify history tracking"
- Part of simplifying orchestrator
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/orchestrator/history.go` |
| EDIT | `internal/orchestrator/orchestrator.go` |
| EDIT | `internal/orchestrator/history_test.go` |
| EDIT | `internal/orchestrator/orchestrator_test.go` |

## Changes Required
- Remove complex history graph
- Keep simple phase log: array of `{phase, status, timestamp, output}`

## Impact
- **Medium**: History used in UI
- UI must adapt to new simple history format

## Dependencies
- **Before**: Task 5.2 (phase router deleted)
- **After**: Proceed to Task 5.4

## Risk
🟡 **Medium** - history used in UI

## Checklist
- [x] Remove complex history graph data structures
- [x] Implement simple phase log as array
- [x] Each entry: `{phase, status, timestamp, output}`
- [x] Update any code that reads history
- [x] Ensure history displays correctly in UI
