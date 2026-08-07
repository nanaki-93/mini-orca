# Task 8.3: Rewrite Remaining Tests

## Phase
Phase 8: Final Cleanup & Testing

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Rewrite test files to match the simplified codebase.

## CLEANUP_PLAN Verification
- **Plan says**: "Rewrite remaining tests to match simplified code"
- Part of final cleanup and validation
- ✅ This task directly fulfills the plan requirement

## Files to Rewrite
| File | Reason |
|------|--------|
| `internal/state/store_test.go` | Simplified store |
| `internal/orchestrator/retry_test.go` | Simplified retry |
| `internal/orchestrator/human_gate_test.go` | Simplified gate |
| `internal/orchestrator/history_test.go` | Simplified history |

## Files to Review
| File | Reason |
|------|--------|
| `internal/agent/coder_test.go` | Agent unchanged |
| `internal/agent/tester_test.go` | Agent unchanged |
| `internal/agent/reviewer_test.go` | Agent unchanged |
| `internal/agent/client_test.go` | Client unchanged |
| `internal/agent/orchestrator_test.go` | Orchestrator simplified |
| `internal/tools/*_test.go` | Tools unchanged |

## Impact
- **Medium**: Tests must pass

## Dependencies
- **Before**: Task 8.2 (test files cleaned up)
- **After**: Proceed to Task 8.4

## Risk
🟡 **Medium** - tests must pass

## Checklist
- [ ] Rewrite `store_test.go` for simplified store
- [ ] Rewrite `retry_test.go` for simplified retry
- [ ] Rewrite `human_gate_test.go` for simplified gate
- [ ] Rewrite `history_test.go` for simplified history
- [ ] Review and update `orchestrator_test.go`
- [ ] Review all agent and tool tests
- [ ] Ensure all tests compile
