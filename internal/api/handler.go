package api

import (
	"encoding/json"
	"net/http"

	"github.com/nanaki-93/mini-orca/internal/state"
)

// GET /api/session - Returns the full active project context to the frontend
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, err := s.SM.Store.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ctx)
}

// POST /api/session/edit - Allows the human to override code plans or task arrays before running
func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var incomingUpdate state.ProjectContext
	if err := json.NewDecoder(r.Body).Decode(&incomingUpdate); err != nil {
		http.Error(w, "Invalid payload context", http.StatusBadRequest)
		return
	}

	// Load existing file to merge updates safely
	currentCtx, err := s.SM.Store.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Apply micro-edits safely based on the layout state
	if currentCtx.CurrentState == state.StateWaitingForDesign {
		currentCtx.Analysis = incomingUpdate.Analysis
	} else if currentCtx.CurrentState == state.StateWaitingForTasks {
		currentCtx.Tasks = incomingUpdate.Tasks
	} else if currentCtx.CurrentState == state.StateAnalysis {
		currentCtx.ProjectGoal = incomingUpdate.ProjectGoal
		currentCtx.ProjectPath = incomingUpdate.ProjectPath
	} else {
		http.Error(w, "Cannot edit project context during active model execution phases", http.StatusConflict)
		return
	}

	if err := s.SM.Store.Save(currentCtx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"edits_saved_successfully"}`))
}

// POST /api/session/approve - Trips the human gate to continue execution loops
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type ApprovalPayload struct {
		Approved bool `json:"approved"`
	}

	var payload ApprovalPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Non-blocking channel push to ensure the API server doesn't freeze
	// if the State Machine loop isn't listening at this exact millisecond.
	select {
	case s.ApprovalChan <- payload.Approved:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"signal_transmitted"}`))
	default:
		http.Error(w, "Orchestration loop engine is not currently waiting for an authorization token input", http.StatusConflict)
	}
}
