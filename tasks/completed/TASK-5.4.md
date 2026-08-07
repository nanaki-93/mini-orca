# Task 5.4: Simplify Retry Logic

## Phase
Phase 5: Simplify Orchestrator

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/orchestrator/retry.go` to keep basic retry with exponential backoff but remove complex retry strategies.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/orchestrator/retry.go` - Simplify or remove"
- Part of simplifying orchestrator
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/orchestrator/retry.go` |
| EDIT | `internal/orchestrator/retry_test.go` |

## Changes Required
- Keep basic retry with exponential backoff
- Remove complex retry strategies
- Simplify `WithRetry` function

## Impact
- **Low**: Retry logic is utility code

## Dependencies
- **Before**: Task 5.3 (history simplified)
- **After**: Proceed to Task 5.5

## Risk
🟢 **Low**

## Checklist
- [x] Keep basic retry with exponential backoff
- [x] Remove complex retry strategies
- [x] Simplify `WithRetry` function
- [x] Ensure retry still works for failed operations
