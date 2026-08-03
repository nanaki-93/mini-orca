# Task 4.2: Update Gate API

## Goal
Update the gate API to work with the simplified flow. The gate still exists for the human review phase but the transition logic is simpler.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/gate.go`

## Detailed Steps

### Step 1: Update RespondToGate Handler
**Replace the entire `RespondToGate` method:**

**From:**
```go
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
		WriteAppError(w, apperrors.BadRequest("action is required", "A response action (e.g., 'approve', 'reject') is required.", nil))
		return
	}

	if err := gate.Respond(req.Action, req.Feedback); err != nil {
		WriteAppError(w, apperrors.BadRequest("gate response failed", "Failed to process gate response: "+err.Error(), err))
		return
	}

	// Trigger phase transition on approval
	if req.Action == "approve" {
		validTransitions := orchestrator.GetValidTransitions(gate.Phase())
		if len(validTransitions) > 0 {
			nextPhase := validTransitions[0]
			if err := h.router.TransitionTo(nextPhase); err != nil {
				WriteAppError(w, apperrors.Conflict("phase transition failed", "Could not transition to the next phase: "+err.Error(), err))
				return
			}
		}
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "accepted",
		"action":     req.Action,
		"session_id": sessionID,
	})
}
```

**To:**
```go
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
```

### Step 2: Keep GetGateStatus Handler Unchanged
The `GetGateStatus` handler doesn't need changes — it just returns gate state.

### Step 3: Verify GateStore
The `GateStore` struct and methods don't need changes:
```go
type GateStore struct {
	gates map[string]*orchestrator.HumanGate
}
```

### Step 4: Verify GateResponseRequest
The request struct stays the same:
```go
type GateResponseRequest struct {
	Action   string `json:"action"`
	Feedback string `json:"feedback,omitempty"`
}
```

Valid actions: `"approve"`, `"edit"` (no more `reject` needed since review handles that).

## Verification
- `go build ./internal/api/` — compiles
- `RespondToGate` handles `approve` → completed, `edit` → triggers orchestrator loop
- Gate store and request structs unchanged
