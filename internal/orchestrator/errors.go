// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import "fmt"

// OrchestratorError represents a generic orchestrator error.
type OrchestratorError struct {
	Phase   string
	Message string
	Cause   error
}

func (e *OrchestratorError) Error() string {
	return fmt.Sprintf("orchestrator error in phase %s: %s", e.Phase, e.Message)
}

func (e *OrchestratorError) Unwrap() error {
	return e.Cause
}

// ErrInvalidPhase is returned when an invalid phase is requested.
var ErrInvalidPhase = fmt.Errorf("invalid phase")

// ErrAgentNotFound is returned when a required agent is not configured.
var ErrAgentNotFound = fmt.Errorf("agent not found")

// ErrPhaseTransition is returned when a phase transition is not allowed.
var ErrPhaseTransition = fmt.Errorf("invalid phase transition")

// ErrHumanApprovalRequired is returned when human approval is required but not yet given.
var ErrHumanApprovalRequired = fmt.Errorf("human approval required")
