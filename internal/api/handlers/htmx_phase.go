package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/orchestrator"
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
	data.TotalPhases = 4 // coding, testing, review, human_review

	// Set phase-specific data based on the requested phase
	switch phase {
	case "coding":
		data.CodingPhase = true
		data.CodingPhaseData = &CodingPhaseData{
			Status:        "running",
			CurrentFile:   "generating...",
			GeneratedCode: "",
		}
		data.CurrentPhaseName = "Coding"
	case "testing":
		data.TestingPhase = true
		data.TestingPhaseData = &TestingPhaseData{
			Status: "running",
		}
		data.CurrentPhaseName = "Testing"
	case "review":
		data.ReviewPhase = true
		data.ReviewPhaseData = &ReviewPhaseData{
			Status: "running",
		}
		data.CurrentPhaseName = "Review"
	case "human_review":
		data.HumanReviewPhase = true
		data.HumanReviewPhaseData = &HumanReviewPhaseData{
			CurrentPhase: "human_review",
			Output:       "Review pending...",
		}
		data.CurrentPhaseName = "Human Review"
	default:
		data.CurrentPhaseName = "Unknown"
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
func getCurrentPhaseInfo(phase string) (string, string) {
	switch phase {
	case string(orchestrator.PhaseCoding):
		return "Coding", "in_progress"
	case string(orchestrator.PhaseTesting):
		return "Testing", "in_progress"
	case string(orchestrator.PhaseReview):
		return "Review", "in_progress"
	case string(orchestrator.PhaseHumanReview):
		return "Human Review", "in_progress"
	default:
		return "Unknown", "pending"
	}
}

// getCurrentPhaseName returns the current phase name as a string.
func getCurrentPhaseName(phase string) string {
	switch phase {
	case string(orchestrator.PhaseCoding):
		return "Coding"
	case string(orchestrator.PhaseTesting):
		return "Testing"
	case string(orchestrator.PhaseReview):
		return "Review"
	case string(orchestrator.PhaseHumanReview):
		return "Human Review"
	default:
		return "Unknown"
	}
}

// getPhaseProgress returns a progress percentage based on session status.
func getPhaseProgress(status string) int {
	switch status {
	case "completed":
		return 100
	case "running":
		return 45
	case "pending":
		return 0
	default:
		return 0
	}
}
