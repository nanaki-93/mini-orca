package orchestrator

import (
	"fmt"
	"sync"
	"time"
)

// GateResponse represents a human response to an approval request.
type GateResponse struct {
	// Action is the response action: "approve", "reject", or "edit".
	Action string
	// Feedback is optional feedback from the human reviewer.
	Feedback string
	// Timestamp records when the response was made.
	Timestamp time.Time
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
// It blocks until a response is received via Respond() or a timeout occurs.
// Returns the GateResponse or an error if the gate is already approved or timed out.
func (g *HumanGate) RequestApproval(output string) (*GateResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.approved {
		return nil, fmt.Errorf("human gate: already approved for session %s", g.sessionID)
	}

	g.output = output

	// Wait for response or timeout in a goroutine
	done := make(chan struct{})
	go func() {
		select {
		case response := <-g.responseChan:
			g.mu.Lock()
			g.approved = response.Action == "approve"
			g.feedback = response.Feedback
			g.mu.Unlock()
			close(done)
		}
	}()

	// Wait for the response to be processed
	<-done

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.approved {
		return nil, nil
	}

	// Find the response that was sent
	for {
		select {
		case response := <-g.responseChan:
			return &response, nil
		default:
			return nil, fmt.Errorf("human gate: gate not approved for session %s", g.sessionID)
		}
	}
}

// Respond sends a response to the approval gate.
// The action must be one of: "approve", "reject", or "edit".
func (g *HumanGate) Respond(action string, feedback string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch action {
	case "approve", "reject", "edit":
		// Valid action
	default:
		return fmt.Errorf("human gate: invalid action %q, must be 'approve', 'reject', or 'edit'", action)
	}

	response := GateResponse{
		Action:    action,
		Feedback:  feedback,
		Timestamp: time.Now(),
	}

	select {
	case g.responseChan <- response:
		return nil
	default:
		return fmt.Errorf("human gate: no pending approval request for session %s", g.sessionID)
	}
}

// Timeout forces the gate into a timed-out state.
// It returns an error indicating the timeout occurred.
func (g *HumanGate) Timeout(timeout time.Duration) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Wait for the specified duration for a response
	select {
	case response := <-g.responseChan:
		g.approved = response.Action == "approve"
		g.feedback = response.Feedback
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("human gate: approval timed out after %v for session %s", timeout, g.sessionID)
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
