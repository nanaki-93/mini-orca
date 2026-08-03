// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/model"
	"github.com/nanaki-93/mini-orca/v2/internal/state"
)

// Session represents a single execution session within the pipeline.
type Session struct {
	// ID is the unique identifier for this session.
	ID string
	// Phase is the current phase of the session.
	Phase Phase
	// CreatedAt is when the session was created.
	CreatedAt string
}

// Orchestrator manages the execution flow of the multi-phase development pipeline.
type Orchestrator struct {
	router         *model.Router
	stateStore     *state.Store
	config         *config.Config
	currentPhase   Phase
	currentSession *Session
	historyTracker *HistoryTracker
	ctx            context.Context
	cancel         context.CancelFunc
	humanGate      *HumanGate
}

// New creates a new Orchestrator instance with the given dependencies.
func New(router *model.Router, stateStore *state.Store, config *config.Config) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	return &Orchestrator{
		router:     router,
		stateStore: stateStore,
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run executes the pipeline from start to finish.
func (o *Orchestrator) Run() error {
	// TODO: implement pipeline execution
	return nil
}

// runCoding executes the coding phase of the pipeline.
func (o *Orchestrator) runCoding() error {
	// TODO: implement coding phase execution
	return fmt.Errorf("coding phase not implemented")
}

// transitionTo transitions the pipeline to the specified phase.
func (o *Orchestrator) transitionTo(phase Phase) error {
	if err := ValidateTransition(o.currentPhase, phase); err != nil {
		return fmt.Errorf("transition: %w", err)
	}

	logTransition(o.currentPhase, phase)
	o.currentPhase = phase

	// Update session phase
	if o.currentSession != nil {
		o.currentSession.Phase = phase
	}

	// Update state store
	if o.stateStore != nil {
		_ = o.stateStore.UpdatePhaseStatus(state.Phase(phase), state.SessionStatusRunning)
	}

	return nil
}

// runTesting executes the testing phase of the pipeline.
func (o *Orchestrator) runTesting() error {
	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhaseTesting, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("testing phase: failed to update session status: %w", err)
	}

	// TODO: implement test execution

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseTesting),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      "Testing phase completed (stub)",
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("testing phase: failed to add phase history: %w", err)
	}

	// Transition to Review phase
	if err := o.transitionTo(PhaseReview); err != nil {
		o.historyTracker.LogPhaseTransition(PhaseTesting, PhaseReview)
		return fmt.Errorf("testing phase: failed to transition to review: %w", err)
	}
	o.historyTracker.LogPhaseTransition(PhaseTesting, PhaseReview)

	return nil
}

// runReview executes the review phase of the pipeline.
func (o *Orchestrator) runReview() error {
	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhaseReview, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("review phase: failed to update session status: %w", err)
	}

	// TODO: implement review phase

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseReview),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      "Review phase completed (stub)",
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("review phase: failed to add phase history: %w", err)
	}

	// Transition to HumanReview phase
	if err := o.transitionTo(PhaseHumanReview); err != nil {
		o.historyTracker.LogPhaseTransition(PhaseReview, PhaseHumanReview)
		return fmt.Errorf("review phase: failed to transition to human review: %w", err)
	}
	o.historyTracker.LogPhaseTransition(PhaseReview, PhaseHumanReview)

	return nil
}

// runHumanReview executes the human review phase of the pipeline.
func (o *Orchestrator) runHumanReview() error {
	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhaseHumanReview, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("human review phase: failed to update session status: %w", err)
	}

	// TODO: implement human review phase

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseHumanReview),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      "Human review phase completed (stub)",
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("human review phase: failed to add phase history: %w", err)
	}

	// Transition to Completed
	if err := o.transitionToCompleted(); err != nil {
		o.historyTracker.LogPhaseTransition(PhaseHumanReview, "completed")
		return fmt.Errorf("human review phase: failed to transition to completed: %w", err)
	}
	o.historyTracker.LogPhaseTransition(PhaseHumanReview, "completed")

	return nil
}

// transitionToCompleted transitions the pipeline to the completed state.
func (o *Orchestrator) transitionToCompleted() error {
	// Validate the transition to completed
	if err := ValidateTransition(o.currentPhase, "completed"); err != nil {
		return fmt.Errorf("transition: %w", err)
	}

	logTransition(o.currentPhase, "completed")

	// Update session phase
	if o.currentSession != nil {
		o.currentSession.Phase = "completed"
	}

	// Update state store
	if o.stateStore != nil {
		if err := o.stateStore.UpdatePhaseStatus(state.PhaseHumanReview, state.SessionStatusCompleted); err != nil {
			return fmt.Errorf("human review phase: failed to update session status to completed: %w", err)
		}
	}

	// Mark session as completed
	o.currentPhase = "completed"

	// Persist history
	if o.historyTracker != nil {
		if err := o.historyTracker.PersistToStore(); err != nil {
			return fmt.Errorf("human review phase: failed to persist history: %w", err)
		}
	}

	return nil
}
