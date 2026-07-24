package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"mini-orca/internal/orchestrator"
	"mini-orca/internal/state"
)

// ApproveHandler handles approval-related API endpoints.
type ApproveHandler struct {
	orchestrator *orchestrator.Orchestrator
}

// NewApproveHandler creates a new approval handler.
func NewApproveHandler(orch *orchestrator.Orchestrator) *ApproveHandler {
	return &ApproveHandler{
		orchestrator: orch,
	}
}

// Approve handles POST /api/approve
func (h *ApproveHandler) Approve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phase   state.Phase `json:"phase"`
		Message string      `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := h.orchestrator.ApprovePhase(req.Phase, req.Message); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "approved",
		"phase":     req.Phase,
		"message":   req.Message,
		"approved_at": time.Now(),
	})
}

// Reject handles POST /api/reject
func (h *ApproveHandler) Reject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phase   state.Phase `json:"phase"`
		Message string      `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := h.orchestrator.RejectPhase(req.Phase, req.Message); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "rejected",
		"phase":   req.Phase,
		"message": req.Message,
	})
}

// GetPendingApprovals handles GET /api/pending-approvals
func (h *ApproveHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	pending := h.orchestrator.GetPendingApprovals()
	
	response := make([]map[string]interface{}, 0)
	for _, phase := range pending {
		response = append(response, map[string]interface{}{
			"phase":  phase,
			"name":   getPhaseName(phase),
			"reason": getPhaseReason(phase),
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getPhaseName(phase state.Phase) string {
	switch phase {
	case state.PhasePlanningReview:
		return "Planning Review"
	case state.PhaseHumanReview:
		return "Human Review"
	default:
		return "Unknown"
	}
}

func getPhaseReason(phase state.Phase) string {
	switch phase {
	case state.PhasePlanningReview:
		return "Review the generated plan before proceeding"
	case state.PhaseHumanReview:
		return "Review all changes before completing the session"
	default:
		return ""
	}
}
