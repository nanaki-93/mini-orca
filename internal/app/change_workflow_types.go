package app

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Workflow choices reuse daemon-owned profiles, including their transport,
// credentials, retry limits and per-request deadlines.
type ChangeWorkflowModels struct {
	Create string `json:"create"`
	Test   string `json:"test"`
	Review string `json:"review"`
}

func (models ChangeWorkflowModels) validate() error {
	for _, profile := range []string{models.Create, models.Test, models.Review} {
		if profile != "analyze" && profile != "bug" && profile != "function" {
			return fmt.Errorf("workflow models must reference configured analyze, bug or function profiles")
		}
	}
	return nil
}

type ChangeWorkflowRequest struct {
	ChangeIdentity
	Message           string               `json:"message"`
	Models            ChangeWorkflowModels `json:"models"`
	ConfirmedProfiles []string             `json:"confirmed_profiles"`
	ConfirmSecurity   bool                 `json:"confirm_security"`
}

type ChangeWorkflowControl struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	WorkflowID      string `json:"workflow_id"`
}

type ChangeWorkflowStage struct {
	Name   string          `json:"name"`
	Status string          `json:"status"`
	Model  *EffectiveModel `json:"model,omitempty"`
}

type ChangeWorkflowReview struct {
	ProposalHash string   `json:"proposal_hash"`
	Verdict      string   `json:"verdict"`
	Summary      string   `json:"summary"`
	Findings     []string `json:"findings"`
}

// Workflow reports live in private change history alongside captured source.
// A model verdict never establishes human approval or measured performance.
type ChangeWorkflow struct {
	ID        string                `json:"id"`
	Status    string                `json:"status"`
	Reason    string                `json:"reason,omitempty"`
	Models    ChangeWorkflowModels  `json:"models"`
	Stages    []ChangeWorkflowStage `json:"stages"`
	Review    *ChangeWorkflowReview `json:"review,omitempty"`
	StartedAt time.Time             `json:"started_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

type changeWorkflowContextKey struct{}

type changeWorkflowJob struct {
	root, sessionID, id string
	models              ChangeWorkflowModels
	cancel              context.CancelFunc
	done                chan struct{}
	finished            bool
	failure             string
}

// The controller owns the single bounded worker. Session/proposal writes use
// changesMu; this mutex is never held while acquiring changesMu or doing I/O.
type changeWorkflowController struct {
	mu  sync.Mutex
	job *changeWorkflowJob
}

func changeWorkflowActive(workflow *ChangeWorkflow) bool {
	return workflow != nil && (workflow.Status == "running" || workflow.Status == "canceling")
}

func changeWorkflowReviewable(session *ChangeSession) bool {
	w := session.Workflow
	return w == nil || (w.Status == "awaiting_human_review" && w.Review != nil && w.Review.ProposalHash == session.Hash && w.Review.Verdict == "approve")
}

func invalidateChangeWorkflow(session *ChangeSession, reason string) {
	if session.Workflow != nil {
		session.Workflow.Status, session.Workflow.Reason = "outdated", reason
		session.Workflow.Stages[3].Status = "blocked"
	}
}

func stopChangeWorkflowStages(w *ChangeWorkflow) {
	for i := range w.Stages[:3] {
		switch w.Stages[i].Status {
		case "running":
			w.Stages[i].Status = w.Status
		case "pending":
			w.Stages[i].Status = "skipped"
		}
	}
	w.Stages[3].Status = "blocked"
}

func validateStoredWorkflow(w *ChangeWorkflow) error {
	if w == nil {
		return nil // Existing schema-1 conversations have no workflow.
	}
	if w.ID == "" || len(w.ID) > 100 || len(w.Stages) != 4 {
		return fmt.Errorf("corrupt change workflow")
	}
	if err := w.Models.validate(); err != nil {
		return err
	}
	switch w.Status {
	case "running", "canceling", "failed", "canceled", "stale", "interrupted", "outdated", "changes_requested", "awaiting_human_review":
	default:
		return fmt.Errorf("corrupt workflow status")
	}
	for i, name := range []string{"create", "test", "review", "human_review"} {
		if w.Stages[i].Name != name {
			return fmt.Errorf("corrupt workflow stages")
		}
	}
	for i, profile := range []string{w.Models.Create, w.Models.Test, w.Models.Review} {
		if w.Stages[i].Model == nil || w.Stages[i].Model.Profile != profile {
			return fmt.Errorf("corrupt workflow model provenance")
		}
	}
	if w.Review != nil {
		return validateWorkflowReview(w.Review)
	}
	return nil
}
