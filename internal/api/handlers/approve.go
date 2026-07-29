// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/internal/orchestrator"
)

// ─── Request/Response Types ──────────────────────────────────────────────────

// GateResponseRequest represents the request body for responding to a human gate.
type GateResponseRequest struct {
	// Action is the response action: "approve", "reject", or "edit".
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

// GateTransitionResponse represents the response body for gate response with phase transition.
type GateTransitionResponse struct {
	// Status is the operation status.
	Status string `json:"status"`
	// Action is the action taken.
	Action string `json:"action"`
	// SessionID is the unique identifier for the session.
	SessionID string `json:"session_id"`
	// NextPhase is the phase that was transitioned to (only on approval).
	NextPhase string `json:"next_phase,omitempty"`
}

// ─── Store ────────────────────────────────────────────────────────────────────

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

// ─── Handler ──────────────────────────────────────────────────────────────────

// ApprovalHandler manages HTTP handlers for human gate interactions.
type ApprovalHandler struct {
	// gateStore provides access to active human gates.
	gateStore *GateStore
	// router provides phase transition capabilities.
	router *orchestrator.PhaseRouter
}

// NewApprovalHandler creates a new ApprovalHandler instance.
func NewApprovalHandler(gateStore *GateStore, router *orchestrator.PhaseRouter) *ApprovalHandler {
	return &ApprovalHandler{
		gateStore: gateStore,
		router:    router,
	}
}

// GetGateStatus handles GET /api/sessions/:id/gate
// Returns the current status of the human gate for the given session.
func (h *ApprovalHandler) GetGateStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	gate, ok := h.gateStore.GetGate(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "gate not found for session")
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

	writeJSON(w, http.StatusOK, response)
}

// RespondToGate handles POST /api/sessions/:id/gate
// Processes a human response to a gate approval request and triggers phase transition.
func (h *ApprovalHandler) RespondToGate(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	gate, ok := h.gateStore.GetGate(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "gate not found for session")
		return
	}

	var req GateResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateGateResponse(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := gate.Respond(req.Action, req.Feedback); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := GateTransitionResponse{
		Status:    "accepted",
		Action:    req.Action,
		SessionID: sessionID,
	}

	// Trigger phase transition on approval
	if req.Action == "approve" {
		validTransitions := orchestrator.GetValidTransitions(gate.Phase())
		if len(validTransitions) > 0 {
			nextPhase := validTransitions[0]
			if err := h.router.TransitionTo(nextPhase); err != nil {
				writeError(w, http.StatusConflict, "phase transition failed: "+err.Error())
				return
			}
			response.NextPhase = string(nextPhase)
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// ─── Path Extraction ──────────────────────────────────────────────────────────

// extractSessionID extracts the session ID from the URL path.
// Expected format: /api/sessions/{id}/gate
func extractSessionID(path string) string {
	parts := splitPath(path)
	if len(parts) < 4 {
		return ""
	}
	// parts: ["", "api", "sessions", "{id}", "gate"]
	return parts[3]
}

// ─── Validation ───────────────────────────────────────────────────────────────

// validActions contains the allowed gate response actions.
var validActions = map[string]struct{}{
	"approve": {},
	"reject":  {},
	"edit":    {},
}

// validateGateResponse validates the gate response request.
// It checks that the action is valid and provides clear error messages.
func validateGateResponse(req GateResponseRequest) error {
	if req.Action == "" {
		return &ValidationError{Field: "action", Message: "action is required"}
	}

	if _, exists := validActions[req.Action]; !exists {
		return &ValidationError{
			Field:   "action",
			Message: "invalid action: must be 'approve', 'reject', or 'edit'",
		}
	}

	return nil
}

// ValidationError represents a validation error for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
