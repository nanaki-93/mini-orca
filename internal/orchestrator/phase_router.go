// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import (
	"fmt"
	"time"

	"github.com/nanaki-93/mini-orca/internal/state"
)

// Phase represents a stage in the development pipeline.
type Phase string

const (
	// PhasePlanning is the initial planning phase.
	PhasePlanning Phase = "planning"
	// PhasePlanningReview is the human review gate for the planning phase.
	PhasePlanningReview Phase = "planning_review"
	// PhaseCoding is the implementation phase.
	PhaseCoding Phase = "coding"
	// PhaseTesting is the testing phase.
	PhaseTesting Phase = "testing"
	// PhaseReview is the automated review phase.
	PhaseReview Phase = "review"
	// PhaseHumanReview is the final human review gate.
	PhaseHumanReview Phase = "human_review"
)

// Transition represents a valid phase transition in the pipeline.
type Transition struct {
	// From is the source phase.
	From Phase
	// To is the destination phase.
	To Phase
	// RequiresHuman indicates whether this transition requires human approval.
	RequiresHuman bool
}

// validTransitions defines all allowed phase transitions in the pipeline.
var validTransitions = []Transition{
	{From: PhasePlanning, To: PhasePlanningReview, RequiresHuman: true},
	{From: PhasePlanningReview, To: PhasePlanning, RequiresHuman: true},
	{From: PhasePlanningReview, To: PhaseCoding, RequiresHuman: true},
	{From: PhaseCoding, To: PhaseTesting, RequiresHuman: false},
	{From: PhaseTesting, To: PhaseCoding, RequiresHuman: false},
	{From: PhaseTesting, To: PhaseReview, RequiresHuman: false},
	{From: PhaseReview, To: PhaseCoding, RequiresHuman: false},
	{From: PhaseReview, To: PhaseHumanReview, RequiresHuman: false},
	{From: PhaseHumanReview, To: PhaseCoding, RequiresHuman: true},
	{From: PhaseHumanReview, To: "completed", RequiresHuman: true},
}

// GetValidTransitions returns the list of valid destination phases for the given current phase.
func GetValidTransitions(current Phase) []Phase {
	var destinations []Phase
	for _, t := range validTransitions {
		if t.From == current {
			destinations = append(destinations, t.To)
		}
	}
	return destinations
}

// ValidateTransition checks whether a transition from 'from' to 'to' is valid.
// It returns nil if the transition is allowed, or an error if it is not.
func ValidateTransition(from, to Phase) error {
	for _, t := range validTransitions {
		if t.From == from && t.To == to {
			return nil
		}
	}
	return fmt.Errorf("invalid phase transition: %s → %s", from, to)
}

// PhaseRouter determines the next phase in the pipeline based on the current state.
type PhaseRouter struct {
	// transitions holds the valid transitions for the router.
	transitions []Transition
	// currentPhase tracks the current phase of the pipeline.
	currentPhase Phase
	// session stores the current session state.
	session *state.Session
	// orchestrator is the orchestrator instance for phase handlers.
	orchestrator *Orchestrator
}

// NewPhaseRouter creates a new PhaseRouter instance with the given session and orchestrator.
func NewPhaseRouter(session *state.Session, orchestrator *Orchestrator) *PhaseRouter {
	return &PhaseRouter{
		transitions:  validTransitions,
		session:      session,
		orchestrator: orchestrator,
	}
}

// NextPhase returns the next phase to execute after the given phase.
func (r *PhaseRouter) NextPhase(current Phase) (Phase, error) {
	destinations := GetValidTransitions(current)
	if len(destinations) == 0 {
		return "", ErrInvalidPhase
	}
	// Return the first valid destination (default behavior).
	// Specific destination selection should be handled by the caller.
	return destinations[0], nil
}

// GetTransitionsForPhase returns all valid transitions from the given phase.
func (r *PhaseRouter) GetTransitionsForPhase(current Phase) []Transition {
	var result []Transition
	for _, t := range r.transitions {
		if t.From == current {
			result = append(result, t)
		}
	}
	return result
}

// ValidateTransition checks whether a transition from 'from' to 'to' is valid.
// It returns nil if the transition is allowed, or an error if it is not.
func (r *PhaseRouter) ValidateTransition(from, to Phase) error {
	for _, t := range r.transitions {
		if t.From == from && t.To == to {
			return nil
		}
	}
	return fmt.Errorf("invalid phase transition: %s → %s", from, to)
}

// TransitionTo transitions the pipeline to the specified phase.
// It validates the transition, updates session state, logs the transition,
// and triggers the appropriate phase handler.
func (r *PhaseRouter) TransitionTo(phase Phase) error {
	// Validate the transition
	if err := r.ValidateTransition(r.currentPhase, phase); err != nil {
		return fmt.Errorf("transition failed: %w", err)
	}

	// Log the transition
	logTransition(r.currentPhase, phase)

	// Update session state
	if err := r.updateSessionState(phase); err != nil {
		return fmt.Errorf("failed to update session state: %w", err)
	}

	// Trigger phase handler
	if err := r.triggerPhaseHandler(phase); err != nil {
		return fmt.Errorf("phase handler failed: %w", err)
	}

	// Update current phase
	r.currentPhase = phase

	return nil
}

// updateSessionState updates the session's current phase and timestamp.
func (r *PhaseRouter) updateSessionState(phase Phase) error {
	if r.session == nil {
		return fmt.Errorf("session is nil")
	}

	r.session.CurrentPhase = state.Phase(phase)
	r.session.UpdatedAt = time.Now()

	return nil
}

// logTransition logs a phase transition for monitoring and debugging.
func logTransition(from, to Phase) {
	// TODO: replace with proper logging framework
	fmt.Printf("[PhaseRouter] Transition: %s → %s\n", from, to)
}

// triggerPhaseHandler triggers the appropriate phase handler based on the target phase.
func (r *PhaseRouter) triggerPhaseHandler(phase Phase) error {
	switch phase {
	case PhasePlanning:
		return r.handlePlanningPhase()
	case PhasePlanningReview:
		return r.handlePlanningReviewPhase()
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

// handlePlanningPhase handles the planning phase execution.
func (r *PhaseRouter) handlePlanningPhase() error {
	// TODO: implement planning phase handler
	return nil
}

// handlePlanningReviewPhase handles the planning review phase execution.
func (r *PhaseRouter) handlePlanningReviewPhase() error {
	// TODO: implement planning review phase handler
	return nil
}

// handleCodingPhase handles the coding phase execution.
func (r *PhaseRouter) handleCodingPhase() error {
	// TODO: implement coding phase handler
	return nil
}

// handleTestingPhase handles the testing phase execution.
func (r *PhaseRouter) handleTestingPhase() error {
	// TODO: implement testing phase handler
	return nil
}

// handleReviewPhase handles the review phase execution.
func (r *PhaseRouter) handleReviewPhase() error {
	// TODO: implement review phase handler
	return nil
}

// handleHumanReviewPhase handles the human review phase execution.
func (r *PhaseRouter) handleHumanReviewPhase() error {
	// TODO: implement human review phase handler
	return nil
}
