# Task 1.6: Update Orchestrator to Use Simplified State

## Phase
Phase 1: Remove Session System

## Status
✅ Complete

## Goal
Edit `internal/orchestrator/orchestrator.go` to replace `state.Store` method calls with direct field access on `Orchestrator.currentSession`.

## CLEANUP_PLAN Verification
- **Plan says**: "Replace multi-session management with a single 'active session' concept"
- **Plan says**: "Simplify Run() to single request flow" (also covered in Phase 5)
- ✅ This task fulfills the plan requirement for simplified state usage

## Files
| Action | File |
|--------|------|
| EDIT | `internal/orchestrator/orchestrator.go` |

## Changes Required
- Replace `state.Store` method calls with direct field access on `Orchestrator.currentSession`
- Remove calls to:
  - `stateStore.SaveSession`
  - `stateStore.SavePhaseHistory`
  - `stateStore.SaveTestResult`
  - `stateStore.SaveReviewReport`
  - Any other session-related store calls
- Keep phase transitions and human gate logic

## Impact
- **High**: Core orchestrator logic
- Must maintain correct phase flow: coding → testing → review → human_review

## Dependencies
- **Before**: Task 1.5 (state store simplified)
- **After**: Proceed to Task 1.7

## Risk
🔴 **High** - core orchestrator logic

## Checklist
- [x] Replace session store calls with direct field access
- [x] Remove all `SaveSession`, `SavePhaseHistory`, etc. calls
- [x] Keep phase transition validation
- [x] Keep human gate logic intact
- [x] Ensure orchestrator compiles
