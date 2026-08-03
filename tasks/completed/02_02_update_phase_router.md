# Task 2.2: Simplify Phase Router

## Goal
Simplify the phase router to handle only 4 phases.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/orchestrator/phase_router.go`

## Detailed Steps

### Step 1: Delete these methods:
- `handlePlanningPhase()`
- `handlePlanningReviewPhase()`

### Step 2: Update triggerPhaseHandler()
**Replace the switch with:**
```go
func (r *PhaseRouter) triggerPhaseHandler(phase Phase) error {
	switch phase {
	case PhaseCoding:
		return r.handleCodingPhase()
	case PhaseTesting:
		return r.handleTestingPhase()
	case PhaseReview:
		return r.handleReviewPhase()
	case PhaseHumanReview:
		return r.handleHumanReviewPhase()
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}
}
```

### Step 3: Simplify all handler methods
**Replace each handler to just delegate to orchestrator:**
```go
func (r *PhaseRouter) handleCodingPhase() error {
	return r.orchestrator.RunCoding()
}
func (r *PhaseRouter) handleTestingPhase() error {
	return r.orchestrator.RunTesting()
}
func (r *PhaseRouter) handleReviewPhase() error {
	return r.orchestrator.RunReview()
}
func (r *PhaseRouter) handleHumanReviewPhase() error {
	return r.orchestrator.RunHumanReview()
}
```

### Step 4: Verify GetTransitionsForPhase() and ValidateTransition()
These read from `r.transitions` which is set to `validTransitions`. Since we updated `validTransitions` in Task 1.2, no changes needed.

## Verification
- `go build ./internal/orchestrator/` — compiles
- No references to `PhasePlanning` or `PhasePlanningReview`
- `triggerPhaseHandler()` only handles 4 phases
