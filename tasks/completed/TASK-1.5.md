# Task 1.5: Simplify State Store - Remove Session CRUD

## Phase
Phase 1: Remove Session System

## Status
✅ Complete

## Goal
Edit `internal/state/store.go` to remove all session-related methods and data, keeping only phase tracking functionality.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/state/store.go` - Remove session-related methods, keep only phase tracking"
- **Plan says**: "Keep: phase tracking (coding → testing → review → human_review)"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/state/store.go` |

## Methods to Remove
- `SaveSession`
- `GetSession`
- `ListSessions`
- `GetCurrentSession`
- `SavePhaseHistory`
- `GetSessionHistory`
- `SaveTestResult`
- `GetTestResults`
- `SaveReviewReport`
- `GetReviewReports`

## Methods to Keep
- `UpdatePhaseStatus`
- `SetError`
- `InitStateStore` (simplified)

## Data to Remove
- `Session` map
- Session-related fields

## Data to Keep
- `currentPhase`
- `status`

## Impact
- **High**: Orchestrator depends on these methods
- All calls to removed methods must be eliminated

## Dependencies
- **Before**: Task 1.4 (gate store deleted)
- **After**: Proceed to Task 1.6

## Risk
🔴 **High** - orchestrator depends on these methods

## Checklist
- [x] Remove all session-related methods from `Store`
- [x] Remove `Session` map from store struct
- [x] Keep only `currentPhase` and `status` fields
- [x] Simplify `InitStateStore` for single-session use
- [x] Ensure store compiles independently
