# Task 5.1: Simplify Orchestrator Run() Method

## Phase
Phase 5: Simplify Orchestrator

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/orchestrator/orchestrator.go` to replace infinite loop `Run()` with single-pass `RunOnce()`.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/orchestrator/orchestrator.go` - Simplify Run() to single request flow"
- **Plan says**: "New Flow: User opens file → Coder → Tester → Reviewer → Show result to user"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/orchestrator/orchestrator.go` |

## Changes Required
- Replace infinite loop `Run()` with single-pass `RunOnce()`
- Flow: coding → testing → review → human_gate → return
- Remove auto-retry loops (let caller handle retries)
- Keep phase transition validation

## Impact
- **High**: Core pipeline logic
- Must maintain correct single-pass flow

## Dependencies
- **Before**: Start here (Phase 5 start)
- **After**: Proceed to Task 5.2

## Risk
🔴 **High** - core pipeline logic

## Checklist
- [ ] Replace `Run()` infinite loop with `RunOnce()` single-pass
- [ ] Implement linear flow: coding → testing → review → human_gate
- [ ] Remove auto-retry loops
- [ ] Keep phase transition validation
- [ ] Ensure orchestrator handles single request correctly
