package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/state"
)

// RenderActivityLog handles GET /api/render/activity-log
// Renders the HTML partial for the activity log.
func (h *HTMXRenderHandler) RenderActivityLog(w http.ResponseWriter, r *http.Request) {
	// Get session data
	sessions := h.sessionStore.ListSessions()
	if len(sessions) == 0 {
		api.WriteAppError(w, apperrors.BadRequest("no active session", "No active session found. Please start a new session.", nil))
		return
	}

	session := sessions[0]

	// Build activity entries from session history
	entries := make([]ActivityEntry, 0, len(session.History))
	for _, hist := range session.History {
		entries = append(entries, ActivityEntry{
			Phase:     hist.Phase,
			Type:      "system",
			Status:    string(hist.Status),
			Timestamp: hist.StartedAt.Format("15:04:05"),
			Action:    fmt.Sprintf("Phase %s %s", hist.Phase, hist.Status),
		})
	}

	// Get unique phases
	phases := []string{"coding", "testing", "review", "human_review"}
	for _, hist := range session.History {
		found := false
		for _, p := range phases {
			if hist.Phase == p {
				found = true
				break
			}
		}
		if !found {
			phases = append(phases, hist.Phase)
		}
	}

	data := ActivityLogRenderData{
		Entries:    entries,
		Phases:     phases,
		AutoScroll: true,
	}

	rendered, err := h.templateEngine.RenderComponentPartial("activity-log", data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the activity log component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// RenderPhaseTracker handles GET /api/render/phase-tracker
// Renders the HTML partial for the phase tracker.
func (h *HTMXRenderHandler) RenderPhaseTracker(w http.ResponseWriter, r *http.Request) {
	// Get session data
	sessions := h.sessionStore.ListSessions()
	if len(sessions) == 0 {
		api.WriteAppError(w, apperrors.BadRequest("no active session", "No active session found. Please start a new session.", nil))
		return
	}

	session := sessions[0]

	// Build phase tracker items
	allPhases := []PhaseTrackerItem{
		{Name: "Coding", Status: "pending", MaxRetries: 3},
		{Name: "Testing", Status: "pending", MaxRetries: 3},
		{Name: "Review", Status: "pending", MaxRetries: 3},
		{Name: "Human Review", Status: "pending", MaxRetries: 3},
	}

	// Update statuses based on session state
	for i := range allPhases {
		switch session.CurrentPhase {
		case state.PhaseCoding:
			if i == 0 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseTesting:
			if i < 2 {
				allPhases[i].Status = "completed"
			}
			if i == 1 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseReview:
			if i < 3 {
				allPhases[i].Status = "completed"
			}
			if i == 2 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseHumanReview:
			if i < 4 {
				allPhases[i].Status = "completed"
			}
			if i == 3 {
				allPhases[i].Status = "in_progress"
			}
		}

		if session.Status == state.SessionStatusCompleted {
			for j := range allPhases {
				allPhases[j].Status = "completed"
			}
		}
	}

	// Get current phase info
	currentPhaseName, currentPhaseStatus := getCurrentPhaseInfo(session)

	data := PhaseTrackerRenderData{
		Phases:              allPhases,
		CurrentPhaseName:    currentPhaseName,
		CurrentPhaseStatus:  currentPhaseStatus,
		CurrentPhaseMessage: fmt.Sprintf("Session: %s", session.Goal),
	}

	rendered, err := h.templateEngine.RenderComponentPartial("phase-tracker", data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the phase tracker component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// RenderDashboard handles GET /api/render/dashboard
// Renders both phase tracker and activity log in a single request.
// Uses caching to avoid unnecessary re-renders.
func (h *HTMXRenderHandler) RenderDashboard(w http.ResponseWriter, r *http.Request) {
	// Check If-None-Match header for conditional requests
	ifNoneMatch := r.Header.Get("If-None-Match")
	if h.cache != nil && ifNoneMatch != "" {
		if entry, ok := h.cache.Get("/api/render/dashboard"); ok {
			if entry.Etag == ifNoneMatch[1:len(ifNoneMatch)-1] {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// Get session data
	sessions := h.sessionStore.ListSessions()
	if len(sessions) == 0 {
		api.WriteAppError(w, apperrors.BadRequest("no active session", "No active session found. Please start a new session.", nil))
		return
	}

	session := sessions[0]

	// Build phase tracker items
	allPhases := []PhaseTrackerItem{
		{Name: "Coding", Status: "pending", MaxRetries: 3},
		{Name: "Testing", Status: "pending", MaxRetries: 3},
		{Name: "Review", Status: "pending", MaxRetries: 3},
		{Name: "Human Review", Status: "pending", MaxRetries: 3},
	}

	// Update statuses based on session state
	for i := range allPhases {
		switch session.CurrentPhase {
		case state.PhaseCoding:
			if i == 0 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseTesting:
			if i < 2 {
				allPhases[i].Status = "completed"
			}
			if i == 1 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseReview:
			if i < 3 {
				allPhases[i].Status = "completed"
			}
			if i == 2 {
				allPhases[i].Status = "in_progress"
			}
		case state.PhaseHumanReview:
			if i < 4 {
				allPhases[i].Status = "completed"
			}
			if i == 3 {
				allPhases[i].Status = "in_progress"
			}
		}

		if session.Status == state.SessionStatusCompleted {
			for j := range allPhases {
				allPhases[j].Status = "completed"
			}
		}
	}

	currentPhaseName, currentPhaseStatus := getCurrentPhaseInfo(session)

	phaseData := PhaseTrackerRenderData{
		Phases:              allPhases,
		CurrentPhaseName:    currentPhaseName,
		CurrentPhaseStatus:  currentPhaseStatus,
		CurrentPhaseMessage: fmt.Sprintf("Session: %s", session.Goal),
	}

	// Build activity entries from session history
	entries := make([]ActivityEntry, 0, len(session.History))
	for _, hist := range session.History {
		entries = append(entries, ActivityEntry{
			Phase:     hist.Phase,
			Type:      "system",
			Status:    string(hist.Status),
			Timestamp: hist.StartedAt.Format("15:04:05"),
			Action:    fmt.Sprintf("Phase %s %s", hist.Phase, hist.Status),
		})
	}

	// Get unique phases
	phases := []string{"coding", "testing", "review", "human_review"}
	for _, hist := range session.History {
		found := false
		for _, p := range phases {
			if hist.Phase == p {
				found = true
				break
			}
		}
		if !found {
			phases = append(phases, hist.Phase)
		}
	}

	activityData := ActivityLogRenderData{
		Entries:    entries,
		Phases:     phases,
		AutoScroll: true,
	}

	rendered, err := h.templateEngine.RenderComponentPartial("dashboard", DashboardRenderData{
		PhaseTracker: phaseData,
		ActivityLog:  activityData,
	})
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the dashboard component.", err))
		return
	}

	// Cache the response
	if h.cache != nil {
		h.cache.Set("/api/render/dashboard", []byte(rendered), 10*time.Second)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=10")
	w.Header().Set("ETag", "\"dashboard-"+fmt.Sprintf("%d", time.Now().Unix()/10)+"\"")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}
