package app

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Workflow choices reference catalog models or configured profiles; credentials
// and retry limits remain daemon-owned.
type ChangeWorkflowModels struct {
	Create string `json:"create"`
	Test   string `json:"test"`
	Review string `json:"review"`
}

func (models ChangeWorkflowModels) selections() []string {
	return []string{models.Create, models.Test, models.Review}
}

func (models ChangeWorkflowModels) validate() error {
	for _, profile := range models.selections() {
		if !validModelSelection(profile) {
			return fmt.Errorf("workflow models must reference catalog model IDs or configured analyze, bug or function profiles")
		}
	}
	return nil
}

type ChangeWorkflowRequest struct {
	PlanHash   string `json:"plan_hash,omitempty"`
	ResumeFrom string `json:"resume_from,omitempty"`
	ChangeIdentity
	Message           string               `json:"message"`
	Models            ChangeWorkflowModels `json:"models"`
	ConfirmedProfiles []string             `json:"confirmed_profiles"`
	ConfirmSecurity   bool                 `json:"confirm_security"`
	AutoRepair        bool                 `json:"auto_repair,omitempty"`
}

type ChangeWorkflowControl struct {
	ProjectID       string `json:"project_id"`
	ProjectRevision string `json:"project_revision"`
	WorkflowID      string `json:"workflow_id"`
}

type ChangeWorkflowStage struct {
	StartedAt  time.Time       `json:"started_at,omitempty"`
	FinishedAt time.Time       `json:"finished_at,omitempty"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Reason     string          `json:"reason,omitempty"`
	Model      *EffectiveModel `json:"model,omitempty"`
}

type ChangeWorkflowReview struct {
	ProposalHash string   `json:"proposal_hash"`
	Verdict      string   `json:"verdict"`
	Summary      string   `json:"summary"`
	Findings     []string `json:"findings"`
}

type ChangeSpecialistReview struct {
	ID       string                `json:"id"`
	Status   string                `json:"status"`
	Reason   string                `json:"reason,omitempty"`
	Evidence string                `json:"evidence"`
	Model    *EffectiveModel       `json:"model"`
	Review   *ChangeWorkflowReview `json:"review,omitempty"`
}

// Workflow reports live in private change history alongside captured source.
// A model verdict never establishes human approval or measured performance.
type ChangeWorkflow struct {
	Attempt            int                      `json:"attempt,omitempty"`
	ResumeFrom         string                   `json:"resume_from,omitempty"`
	ID                 string                   `json:"id"`
	Status             string                   `json:"status"`
	Reason             string                   `json:"reason,omitempty"`
	Models             ChangeWorkflowModels     `json:"models"`
	AutoRepair         bool                     `json:"auto_repair,omitempty"`
	AutoRepairAttempts int                      `json:"auto_repair_attempts,omitempty"`
	ProbeSummary       string                   `json:"probe_summary,omitempty"`
	HarnessLessons     []string                 `json:"harness_lessons,omitempty"`
	Stages             []ChangeWorkflowStage    `json:"stages"`
	Review             *ChangeWorkflowReview    `json:"review,omitempty"`
	Specialists        []ChangeSpecialistReview `json:"specialists,omitempty"`
	StartedAt          time.Time                `json:"started_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
}

type changeWorkflowContextKey struct{}

type changeWorkflowJob struct {
	root, sessionID, id string
	models              ChangeWorkflowModels
	startStage          int
	specialists         []string
	autoRepair          bool
	cancel              context.CancelFunc
	done                chan struct{}
	finished            bool
	failure             string
}

// The controller tracks active workflow workers per change session. Session/proposal
// writes use changesMu; this mutex is never held while acquiring changesMu or doing I/O.
type changeWorkflowController struct {
	mu   sync.Mutex
	job  *changeWorkflowJob
	jobs map[string]*changeWorkflowJob
}

func (c *changeWorkflowController) getJob(sessionID string) *changeWorkflowJob {
	if c.jobs != nil {
		if j := c.jobs[sessionID]; j != nil {
			return j
		}
	}
	if c.job != nil && c.job.sessionID == sessionID {
		return c.job
	}
	return nil
}

func (c *changeWorkflowController) setJob(job *changeWorkflowJob) {
	if c.jobs == nil {
		c.jobs = make(map[string]*changeWorkflowJob)
	}
	c.job = job
	c.jobs[job.sessionID] = job
}

func (c *changeWorkflowController) allJobs() []*changeWorkflowJob {
	seen := make(map[*changeWorkflowJob]bool)
	var out []*changeWorkflowJob
	for _, j := range c.jobs {
		if j != nil && !seen[j] {
			seen[j] = true
			out = append(out, j)
		}
	}
	if c.job != nil && !seen[c.job] {
		out = append(out, c.job)
	}
	return out
}

func isChangePipelineActive(workflow *ChangeWorkflow) bool {
	return workflow != nil && (workflow.Status == "running" || workflow.Status == "canceling")
}

func canHumanReviewProposal(session *ChangeSession) bool {
	w := session.Workflow
	return w == nil || (w.Status == "awaiting_human_review" && w.Review != nil && w.Review.ProposalHash == session.Hash && w.Review.Verdict == "approve" && specialistsComplete(w, session.Hash))
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
			if w.Stages[i].Reason == "" {
				w.Stages[i].Reason = w.Reason
			}
		case "pending":
			w.Stages[i].Status = "skipped"
		}
	}
	w.Stages[3].Status = "blocked"
	stopWorkflowSpecialists(w)
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
	if err := validateWorkflowSpecialists(w); err != nil {
		return err
	}
	if w.Review != nil {
		return validateWorkflowReview(w.Review)
	}
	return nil
}
