package api

import (
	"encoding/json"
	"fmt"
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

// GET /dashboard - Renders the main skeleton framework
func (s *Server) handleRenderDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.SM.Store.Load()
	if err != nil {
		http.Error(w, "Failed to load state metadata", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.ExecuteTemplate(w, "dashboard.html", ctx)
}

// GET /web/status - Dynamically called via HTMX loop to render only the badge component
func (s *Server) handleRenderStatusBadge(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.SM.Store.Load()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	colorClass := "bg-slate-800 text-slate-300"
	if ctx.CurrentState == "WAITING_FOR_DESIGN_APPROVAL" || ctx.CurrentState == "WAITING_FOR_TASK_APPROVAL" {
		colorClass = "bg-amber-500/20 text-amber-400 border border-amber-500/30 animate-pulse"
	} else if ctx.CurrentState == "MICRO_EXECUTION" {
		colorClass = "bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 animate-pulse"
	} else if ctx.CurrentState == "COMPLETED" {
		colorClass = "bg-emerald-600 text-white font-bold"
	}

	htmlFragment := fmt.Sprintf(`<span id="status-badge" hx-get="/web/status" hx-trigger="every 2s" hx-swap="outerHTML" class="px-3 py-1 text-xs font-semibold rounded-full %s">%s</span>`, colorClass, ctx.CurrentState)

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(htmlFragment))
}

// GET /web/workstream - Returns the real-time card configurations depending on engine state
func (s *Server) handleRenderWorkstream(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.SM.Store.Load()
	if err != nil {
		http.Error(w, "State Sync Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.ExecuteTemplate(w, "workstream.html", ctx)
}
