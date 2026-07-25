# Task 4.3.1 — Define Phase Transition Rules

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Define valid phase transitions.

## Checklist
- [ ] Create `internal/orchestrator/phase_router.go`
- [ ] Define valid transitions:
  - Planning → PlanningReview (human gate)
  - PlanningReview → Planning (reject)
  - PlanningReview → Coding (approve)
  - Coding → Testing
  - Testing → Coding (fail, retry)
  - Testing → Review (pass)
  - Review → Coding (reject)
  - Review → HumanReview (pass)
  - HumanReview → Coding (request edits)
  - HumanReview → Completed (approve)
- [ ] Define `Transition` struct: `{From Phase, To Phase, RequiresHuman bool}`
- [ ] Implement `GetValidTransitions(current Phase) []Phase`
- [ ] Implement `ValidateTransition(from, to Phase) error`

## Dependencies
- Task 4.2.5

## Deliverables
- Phase transition rules and validation

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
