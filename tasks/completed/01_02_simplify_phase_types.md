# Task 1.2: Simplify Phase Types and Transitions

## Goal
Remove planning phases from the phase system and update valid transitions.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/state/session.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/orchestrator/phase_router.go`

## Detailed Steps

### Step 1: Remove Planning Phase Constants from session.go
**Remove these constants:**
```go
PhasePlanning       Phase = "planning"
PhasePlanningReview Phase = "planning_review"
```

**Keep:**
```go
PhaseCoding         Phase = "coding"
PhaseTesting        Phase = "testing"
PhaseReview         Phase = "review"
PhaseHumanReview    Phase = "human_review"
```

### Step 2: Remove Planning Phase Constants from phase_router.go
**Remove the same two constants from phase_router.go.**

### Step 3: Update Valid Transitions in phase_router.go
**Replace ALL existing transitions with:**
```go
var validTransitions = []Transition{
	{From: PhaseCoding, To: PhaseTesting, RequiresHuman: false},
	{From: PhaseTesting, To: PhaseCoding, RequiresHuman: false},  // retry loop
	{From: PhaseTesting, To: PhaseReview, RequiresHuman: false},
	{From: PhaseReview, To: PhaseCoding, RequiresHuman: false},   // retry loop
	{From: PhaseReview, To: PhaseHumanReview, RequiresHuman: false},
	{From: PhaseHumanReview, To: PhaseCoding, RequiresHuman: true}, // edit requested
	{From: PhaseHumanReview, To: "completed", RequiresHuman: true},
}
```

### Step 4: Verify Session Default Phase
In `InitStateStore()`, ensure `CurrentPhase: PhaseCoding` (not PhasePlanning).

## Verification
- `go build ./internal/orchestrator/` — compiles
- `go build ./internal/state/` — compiles
- No references to `PhasePlanning` or `PhasePlanningReview` in the two modified files
