package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mini-orca/internal/state"
)

// runPlanningPhase executes the planning phase.
func (o *Orchestrator) runPlanningPhase(ctx context.Context) error {
	o.session.Phase = state.PhasePlanning
	o.session.AddEvent(state.NewEvent("", "phase_change", state.PhasePlanning, "Starting planning phase"))

	// Run planner
	_, err := o.ExecuteAgent(ctx, "planner", o.session.Description)
	if err != nil {
		return fmt.Errorf("planner execution failed: %w", err)
	}

	// Parse plan (simplified - in production, use proper parsing)
	o.plan = state.NewPlan(
		"plan-001",
		o.session.ID,
		"Development plan created by planner",
	)

	// Add atomic units from plan output (simplified)
	o.plan.AtomicUnits = []state.AtomicUnit{
		{
			ID:          "AU-001",
			Name:        "Example Function",
			Type:        "function",
			File:        "main.go",
			Description: "Example atomic unit",
			Status:      state.UnitStatusPending,
			CreatedAt:   time.Now(),
		},
	}

	o.session.Plan = o.plan

	// Transition to planning review
	if err := o.session.TransitionTo(state.PhasePlanningReview); err != nil {
		return err
	}

	// Save state
	if err := o.stateStore.UpdateSession(o.session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	if err := o.stateStore.SavePlan(o.plan); err != nil {
		return fmt.Errorf("failed to save plan: %w", err)
	}

	o.session.AddEvent(state.NewEvent("", "phase_complete", state.PhasePlanning,
		fmt.Sprintf("Planning complete. Plan has %d atomic units", len(o.plan.AtomicUnits))))

	return nil
}

// runCodingPhase executes the coding phase for the next pending unit.
func (o *Orchestrator) runCodingPhase(ctx context.Context) error {
	o.session.Phase = state.PhaseCoding
	o.session.AddEvent(state.NewEvent("", "phase_change", state.PhaseCoding, "Starting coding phase"))

	// Find next pending unit
	pendingUnits := o.plan.GetPendingUnits()
	if len(pendingUnits) == 0 {
		// All units complete, move to human review
		o.session.TransitionTo(state.PhaseHumanReview)
		return nil
	}

	unit := pendingUnits[0]
	unit.Status = state.UnitStatusInProgress
	o.currentUnit = unit.ID
	o.session.AddEvent(state.NewEvent("", "unit_start", state.PhaseCoding,
		fmt.Sprintf("Starting unit %s: %s", unit.ID, unit.Name)))

	// Prepare input
	input := fmt.Sprintf(`Atomic Unit: %s
Type: %s
File: %s
Description: %s
Dependencies: %v`,
		unit.Name, unit.Type, unit.File, unit.Description, unit.Dependencies)

	// Execute coder with retry
	coderResult, err := o.ExecuteAgent(ctx, "coder", input)
	if err != nil {
		unit.Status = state.UnitStatusPending
		unit.Errors = append(unit.Errors, err.Error())
		return fmt.Errorf("coder execution failed: %w", err)
	}

	// Store generated code
	unit.Code = coderResult.Output
	unit.Status = state.UnitStatusTesting
	o.currentUnit = ""

	// Write code to file
	if err := o.fileOps.AppendFunctionToFile(unit.File, unit.Name, unit.Code); err != nil {
		return fmt.Errorf("failed to write code: %w", err)
	}

	// Format code
	if err := o.formatter.FormatFile(unit.File); err != nil {
		// Log but don't fail
		o.session.AddEvent(state.NewEvent("", "warning", state.PhaseCoding,
			fmt.Sprintf("Formatting failed for %s: %v", unit.File, err)))
	}

	// Commit changes
	if err := o.gitOps.CommitAtomicUnit(unit.ID, fmt.Sprintf("Implement %s", unit.Name)); err != nil {
		o.session.AddEvent(state.NewEvent("", "warning", state.PhaseCoding,
			fmt.Sprintf("Git commit failed: %v", err)))
	}

	o.session.AddEvent(state.NewEvent("", "unit_complete", state.PhaseCoding,
		fmt.Sprintf("Unit %s code generated", unit.ID)))

	return nil
}

// runTestingPhase executes the testing phase.
func (o *Orchestrator) runTestingPhase(ctx context.Context) error {
	o.session.Phase = state.PhaseTesting
	o.session.AddEvent(state.NewEvent("", "phase_change", state.PhaseTesting, "Starting testing phase"))

	// Run tests
	output, err := o.tools.RunTest()
	if err != nil {
		o.session.AddEvent(state.NewEvent("", "error", state.PhaseTesting,
			fmt.Sprintf("Tests failed: %v", err)))

		// Transition back to planning review for major issues
		if o.session.CanTransitionTo(state.PhasePlanningReview) {
			o.session.TransitionTo(state.PhasePlanningReview)
		}
		return fmt.Errorf("tests failed: %w", err)
	}

	o.session.AddEvent(state.NewEvent("", "phase_complete", state.PhaseTesting,
		fmt.Sprintf("Tests passed. Output: %s", strings.TrimSpace(output))))

	// Transition to review
	if err := o.session.TransitionTo(state.PhaseReview); err != nil {
		return err
	}

	return nil
}

// runReviewPhase executes the code review phase.
func (o *Orchestrator) runReviewPhase(ctx context.Context) error {
	o.session.Phase = state.PhaseReview
	o.session.AddEvent(state.NewEvent("", "phase_change", state.PhaseReview, "Starting review phase"))

	// Prepare input (current unit code)
	currentUnit := ""
	for _, u := range o.plan.AtomicUnits {
		if u.ID == o.currentUnit || u.Status == state.UnitStatusInProgress {
			currentUnit = u.Code
			break
		}
	}

	if currentUnit == "" {
		// Use last completed unit
		completed := o.plan.GetCompletedUnits()
		if len(completed) > 0 {
			currentUnit = completed[len(completed)-1].Code
		}
	}

	input := fmt.Sprintf("Code to review:\n%s", currentUnit)

	// Execute reviewer
	reviewResult, err := o.ExecuteAgent(ctx, "reviewer", input)
	if err != nil {
		return fmt.Errorf("reviewer execution failed: %w", err)
	}

	// Parse review verdict (simplified)
	verdict := "approve"
	if strings.Contains(strings.ToLower(reviewResult.Output), "reject") {
		verdict = "reject"
	}

	if verdict == "reject" {
		// Transition back to planning review
		if o.session.CanTransitionTo(state.PhasePlanningReview) {
			o.session.TransitionTo(state.PhasePlanningReview)
		}
		o.session.AddEvent(state.NewEvent("", "review_reject", state.PhaseReview, "Code review rejected"))
		return fmt.Errorf("code review rejected")
	}

	// Mark unit as complete
	for i := range o.plan.AtomicUnits {
		if o.plan.AtomicUnits[i].ID == o.currentUnit {
			o.plan.AtomicUnits[i].Status = state.UnitStatusComplete
			o.plan.AtomicUnits[i].UpdatedAt = time.Now()
			break
		}
	}

	o.session.AddEvent(state.NewEvent("", "phase_complete", state.PhaseReview, "Code review approved"))

	return nil
}
