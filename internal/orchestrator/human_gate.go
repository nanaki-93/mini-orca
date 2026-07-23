package orchestrator

import (
	"context"
	"fmt"
	"time"

	"mini-orca/internal/state"
)

// HumanGate represents a human approval gate in the workflow.
type HumanGate struct {
	SessionID string
	Phase     state.Phase
	Approved  bool
	Rejected  bool
	Message   string
	ApprovedAt time.Time
}

// WaitForHumanApproval blocks until the user approves or rejects the current phase.
func (o *Orchestrator) waitForHumanApproval(ctx context.Context, gatePhase state.Phase) error {
	o.session.AddEvent(state.NewEvent("", "human_gate", gatePhase,
		fmt.Sprintf("Waiting for human approval for phase: %s", gatePhase)))

	// Create a channel to receive approval signal
	approvalChan := make(chan struct{})

	// In a real implementation, this would:
	// 1. Notify the user via HTTP/WebSocket
	// 2. Wait for user input
	// 3. Update session state based on user decision

	// For now, simulate waiting with a timeout
	timeout := 30 * time.Minute // 30 minute timeout
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for approval")
	case <-approvalChan:
		// User approved
		o.session.AddEvent(state.NewEvent("", "human_approval", gatePhase,
			"Human approval received"))
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("human approval timeout after %v", timeout)
	}
}

// ApprovePhase approves the current human gate.
func (o *Orchestrator) ApprovePhase(phase state.Phase, message string) error {
	if !phase.RequiresHumanApproval() {
		return fmt.Errorf("phase %s does not require human approval", phase)
	}

	o.session.AddEvent(state.NewEvent("", "human_approval", phase,
		fmt.Sprintf("Phase approved: %s", message)))

	// Update session state
	if phase == state.PhasePlanningReview {
		if err := o.session.TransitionTo(state.PhaseCoding); err != nil {
			return err
		}
	} else if phase == state.PhaseHumanReview {
		if err := o.session.TransitionTo(state.PhaseComplete); err != nil {
			return err
		}
	}

	// Save state
	return o.stateStore.UpdateSession(o.session)
}

// RejectPhase rejects the current human gate.
func (o *Orchestrator) RejectPhase(phase state.Phase, message string) error {
	if !phase.RequiresHumanApproval() {
		return fmt.Errorf("phase %s does not require human approval", phase)
	}

	o.session.AddEvent(state.NewEvent("", "human_reject", phase,
		fmt.Sprintf("Phase rejected: %s", message)))

	// Transition back to planning review
	if err := o.session.TransitionTo(state.PhasePlanningReview); err != nil {
		return err
	}

	// Save state
	return o.stateStore.UpdateSession(o.session)
}

// GetPendingApprovals returns phases that need human approval.
func (o *Orchestrator) GetPendingApprovals() []state.Phase {
	var pending []state.Phase

	if o.session.Phase == state.PhasePlanningReview {
		pending = append(pending, state.PhasePlanningReview)
	}

	if o.session.Phase == state.PhaseHumanReview {
		pending = append(pending, state.PhaseHumanReview)
	}

	return pending
}
