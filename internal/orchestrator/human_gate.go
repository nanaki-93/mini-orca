package orchestrator

import (
	"fmt"
	"sync"
)

// GateResponse represents a human response to an approval request.
type GateResponse struct {
	// Action is the response action: "approve", "edit", or "refuse".
	Action string
	// Feedback is optional feedback from the human reviewer.
	Feedback string
}

// HumanGate manages human-in-the-loop approval gates within the pipeline.
type HumanGate struct {
	// sessionID uniquely identifies the session this gate belongs to.
	sessionID string
	// phase is the current pipeline phase requiring approval.
	phase Phase
	// output is the content presented to the human for review.
	output string
	// approved indicates whether the gate has been approved.
	approved bool
	// feedback stores the human reviewer's feedback.
	feedback string
	// responseChan is used to receive the human's response.
	responseChan chan GateResponse
	// mu protects concurrent access to the gate state.
	mu sync.Mutex
}

// NewHumanGate creates a new HumanGate instance with the given session ID and phase.
func NewHumanGate(sessionID string, phase Phase) *HumanGate {
	return &HumanGate{
		sessionID:    sessionID,
		phase:        phase,
		responseChan: make(chan GateResponse, 1),
	}
}

// RequestApproval requests human approval for the given output.
// It blocks until a response is received via Respond().
// Returns nil if approved, or the GateResponse if edit/refuse.
func (g *HumanGate) RequestApproval(output string) (*GateResponse, error) {
	g.mu.Lock()
	if g.approved {
		g.mu.Unlock()
		return nil, fmt.Errorf("human gate: already approved for session %s", g.sessionID)
	}
	g.output = output
	g.mu.Unlock()

	// Wait for response
	select {
	case response := <-g.responseChan:
		g.mu.Lock()
		g.approved = response.Action == "approve"
		g.feedback = response.Feedback
		g.mu.Unlock()

		if response.Action == "approve" {
			return nil, nil
		}
		return &response, nil
	}
}

// Respond sends a response to the approval gate.
// The action must be one of: "approve", "edit", or "refuse".
func (g *HumanGate) Respond(action string, feedback string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch action {
	case "approve", "edit", "refuse":
		// Valid action
	default:
		return fmt.Errorf("human gate: invalid action %q, must be 'approve', 'edit', or 'refuse'", action)
	}

	response := GateResponse{
		Action:   action,
		Feedback: feedback,
	}

	select {
	case g.responseChan <- response:
		return nil
	default:
		return fmt.Errorf("human gate: no pending approval request for session %s", g.sessionID)
	}
}

// SessionID returns the session ID associated with this gate.
func (g *HumanGate) SessionID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.sessionID
}

// Phase returns the phase requiring approval.
func (g *HumanGate) Phase() Phase {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.phase
}

// Output returns the content presented for review.
func (g *HumanGate) Output() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.output
}

// IsApproved returns whether the gate has been approved.
func (g *HumanGate) IsApproved() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.approved
}

// Feedback returns the reviewer's feedback.
func (g *HumanGate) Feedback() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.feedback
}
