// Package api provides HTTP handlers for the Mini-Orca REST API.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/orchestrator"
)

// GateResponseRequest represents the request body for responding to a human gate.
type GateResponseRequest struct {
	// Action is the response action: "approve" or "edit".
	Action string `json:"action"`
	// Feedback is optional feedback from the human reviewer.
	Feedback string `json:"feedback,omitempty"`
}

// GateResponse represents the response body for gate status queries.
type GateResponse struct {
	// SessionID is the unique identifier for the session.
	SessionID string `json:"session_id"`
	// Phase is the current pipeline phase requiring approval.
	Phase string `json:"phase"`
	// Output is the content presented to the human for review.
	Output string `json:"output"`
	// Approved indicates whether the gate has been approved.
	Approved bool `json:"approved"`
	// Feedback stores the human reviewer's feedback, if any.
	Feedback string `json:"feedback,omitempty"`
	// Timestamp is when the gate was created or last updated.
	Timestamp time.Time `json:"timestamp"`
}

// GateHandler manages HTTP handlers for human gate interactions.
type GateHandler struct {
	// gateStore provides access to active human gates.
	gateStore *GateStore
	// router provides phase transition capabilities.
	router *orchestrator.PhaseRouter
}

// GateStore provides access to human gates by session ID.
type GateStore struct {
	gates map[string]*orchestrator.HumanGate
}

// NewGateStore creates a new GateStore instance.
func NewGateStore() *GateStore {
	return &GateStore{
		gates: make(map[string]*orchestrator.HumanGate),
	}
}

// RegisterGate registers a human gate for a session.
func (s *GateStore) RegisterGate(sessionID string, gate *orchestrator.HumanGate) {
	s.gates[sessionID] = gate
}

// GetGate retrieves a human gate by session ID.
func (s *GateStore) GetGate(sessionID string) (*orchestrator.HumanGate, bool) {
	gate, ok := s.gates[sessionID]
	return gate, ok
}

// NewGateHandler creates a new GateHandler instance.
func NewGateHandler(gateStore *GateStore, router *orchestrator.PhaseRouter) *GateHandler {
	return &GateHandler{
		gateStore: gateStore,
		router:    router,
	}
}

// GetGateStatus handles GET /api/sessions/:id/gate
// Returns the current status of the human gate for the given session.
func (h *GateHandler) GetGateStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	gate, ok := h.gateStore.GetGate(sessionID)
	if !ok {
		WriteAppError(w, apperrors.NotFound("gate not found", "No active approval gate found for this session.", nil))
		return
	}

	response := GateResponse{
		SessionID: gate.SessionID(),
		Phase:     string(gate.Phase()),
		Output:    gate.Output(),
		Approved:  gate.IsApproved(),
		Feedback:  gate.Feedback(),
		Timestamp: time.Now(),
	}

	WriteJSON(w, http.StatusOK, response)
}

// RespondToGate handles POST /api/sessions/:id/gate
// Processes a human response to a gate approval request and triggers phase transition.
func (h *GateHandler) RespondToGate(w http.ResponseWriter, r *http.Request) {
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	gate, ok := h.gateStore.GetGate(sessionID)
	if !ok {
		WriteAppError(w, apperrors.NotFound("gate not found", "No active approval gate found for this session.", nil))
		return
	}

	var req GateResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if req.Action == "" {
		WriteAppError(w, apperrors.BadRequest("action is required", "A response action (e.g., 'approve', 'edit') is required.", nil))
		return
	}

	if err := gate.Respond(req.Action, req.Feedback); err != nil {
		WriteAppError(w, apperrors.BadRequest("gate response failed", "Failed to process gate response: "+err.Error(), err))
		return
	}

	// On approve → transition to completed
	if req.Action == "approve" {
		if err := h.router.TransitionTo("completed"); err != nil {
			WriteAppError(w, apperrors.Conflict("phase transition failed", "Could not complete the session: "+err.Error(), err))
			return
		}
	}

	// On edit → transition back to coding (handled by orchestrator loop)
	// The orchestrator's Run() method checks gate.IsApproved() after runHumanReview()

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "accepted",
		"action":     req.Action,
		"session_id": sessionID,
	})
}
