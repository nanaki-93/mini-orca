package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/state"
)

// RenderPhase handles GET /api/render/phase/:phase
// Renders the HTML partial for the specified phase.
func (h *HTMXRenderHandler) RenderPhase(w http.ResponseWriter, r *http.Request) {
	phase := extractPhaseFromPath(r.URL.Path)
	if phase == "" {
		api.WriteAppError(w, apperrors.BadRequest("phase is required", "A phase name must be provided.", nil))
		return
	}

	var data PhaseRenderData
	sessions := h.sessionStore.ListSessions()
	if len(sessions) > 0 {
		session := sessions[0]
		data.FeatureRequest = session.Goal
		data.TotalPhases = 4 // coding, testing, review, human_review

		// Set phase-specific data
		switch session.CurrentPhase {
		case state.PhaseCoding:
			data.CodingPhase = true
			data.CodingPhaseData = &CodingPhaseData{
				Status:        "running",
				CurrentFile:   "generating...",
				GeneratedCode: "",
			}
		case state.PhaseTesting:
			data.TestingPhase = true
			data.TestingPhaseData = &TestingPhaseData{
				Status: "running",
			}
		case state.PhaseReview:
			data.ReviewPhase = true
			data.ReviewPhaseData = &ReviewPhaseData{
				Status: "running",
			}
		case state.PhaseHumanReview:
			data.HumanReviewPhase = true
			data.HumanReviewPhaseData = &HumanReviewPhaseData{
				CurrentPhase: "human_review",
				Output:       "Review pending...",
			}
		}

		// Add session-specific data
		data.CurrentPhaseName = getCurrentPhaseName(session)
	}

	rendered, err := h.templateEngine.RenderPhasePartial(phase, data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the phase component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// extractPhaseFromPath extracts the phase name from the URL path.
// Expected format: /api/render/phase/{phase}
func extractPhaseFromPath(path string) string {
	parts := api.SplitPath(path)
	if len(parts) < 4 {
		return ""
	}
	// parts: ["", "api", "render", "phase", "{phase}"]
	return parts[4]
}

// getPhaseStep returns the current step number for a phase.
func getPhaseStep(phase string) int {
	switch lower(phase) {
	case "coding":
		return 0
	case "testing":
		return 1
	case "review":
		return 2
	case "human_review":
		return 3
	default:
		return 0
	}
}

// getCurrentPhaseInfo returns the current phase name and status.
func getCurrentPhaseInfo(session *state.Session) (string, string) {
	switch session.CurrentPhase {
	case state.PhaseCoding:
		return "Coding", "in_progress"
	case state.PhaseTesting:
		return "Testing", "in_progress"
	case state.PhaseReview:
		return "Review", "in_progress"
	case state.PhaseHumanReview:
		return "Human Review", "in_progress"
	default:
		return "Unknown", "pending"
	}
}

// getCurrentPhaseName returns the current phase name as a string.
func getCurrentPhaseName(session *state.Session) string {
	switch session.CurrentPhase {
	case state.PhaseCoding:
		return "Coding"
	case state.PhaseTesting:
		return "Testing"
	case state.PhaseReview:
		return "Review"
	case state.PhaseHumanReview:
		return "Human Review"
	default:
		return "Unknown"
	}
}

// getPhaseProgress returns a progress percentage based on session status.
func getPhaseProgress(status state.SessionStatus) int {
	switch status {
	case state.SessionStatusCompleted:
		return 100
	case state.SessionStatusRunning:
		return 45
	case state.SessionStatusPending:
		return 0
	default:
		return 0
	}
}
