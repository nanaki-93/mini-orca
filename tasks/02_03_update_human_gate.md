# Task 2.3: Verify Human Gate Integration

## Goal
Ensure the human gate integrates properly with the new 4-phase pipeline.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/orchestrator/human_gate.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/orchestrator/orchestrator.go`

## Detailed Steps

### Step 1: Verify human_gate.go
No changes needed to `HumanGate` struct or methods. It's generic and works with any phase.

### Step 2: Add Run() method to Orchestrator
**Add this method to orchestrator.go:**
```go
// Run executes the full pipeline: coding → testing → review → human_gate
func (o *Orchestrator) Run() error {
	for {
		switch o.currentPhase {
		case PhaseCoding:
			if err := o.runCoding(); err != nil {
				return fmt.Errorf("pipeline: coding failed: %w", err)
			}
		case PhaseTesting:
			if err := o.runTesting(); err != nil {
				return fmt.Errorf("pipeline: testing failed: %w", err)
			}
		case PhaseReview:
			if err := o.runReview(); err != nil {
				return fmt.Errorf("pipeline: review failed: %w", err)
			}
		case PhaseHumanReview:
			if err := o.runHumanReview(); err != nil {
				return fmt.Errorf("pipeline: human review failed: %w", err)
			}
			if o.humanGate != nil && o.humanGate.IsApproved() {
				if err := o.transitionToCompleted(); err != nil {
					return fmt.Errorf("pipeline: completed failed: %w", err)
				}
				return nil
			}
			if o.humanGate != nil && !o.humanGate.IsApproved() {
				fmt.Printf("Human requested edits: %s\n", o.humanGate.Feedback())
				if err := o.transitionTo(PhaseCoding); err != nil {
					return fmt.Errorf("pipeline: failed to loop back to coding: %w", err)
				}
			}
		case "completed":
			return nil
		default:
			return fmt.Errorf("pipeline: unknown phase %s", o.currentPhase)
		}
	}
}
```

## Verification
- `go build ./internal/orchestrator/` — compiles
- `Run()` method exists and handles all 4 phases
- Human gate properly checked after `runHumanReview()`
- Loop-back to coding on edit request works
