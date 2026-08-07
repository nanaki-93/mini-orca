# Task 8.2: Remove Test Files for Deleted Code

## Phase
Phase 8: Final Cleanup & Testing

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete test files for removed functionality and identify tests that need rewriting.

## CLEANUP_PLAN Verification
- **Plan says**: "Remove test files for deleted code"
- Part of final cleanup
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File | Notes |
|--------|------|-------|
| DELETE | `internal/orchestrator/phase_router_test.go` | Already deleted in Task 5.2 |
| DELETE | `internal/agent/skills/*_test.go` | Already deleted with skills in Task 3.1 |
| REWRITE | `internal/state/store_test.go` | For simplified store |
| REWRITE | `internal/orchestrator/retry_test.go` | For simplified retry |
| REWRITE | `internal/orchestrator/human_gate_test.go` | For simplified gate |
| REWRITE | `internal/orchestrator/history_test.go` | For simplified history |
| REVIEW | `internal/orchestrator/errors_test.go` | Review for changes |

## Impact
- **Low**: Test cleanup

## Dependencies
- **Before**: Task 8.1 (base template updated)
- **After**: Proceed to Task 8.3

## Risk
🟢 **Low** - test cleanup

## Checklist
- [ ] Delete `phase_router_test.go` (if not already deleted)
- [ ] Delete `skills/*_test.go` files (if not already deleted)
- [ ] Identify tests needing rewrite for simplified code
- [ ] Remove tests for deleted functionality
