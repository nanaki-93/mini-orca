package app

import (
	"fmt"
	"time"
)

// Resuming skips only stages whose output still belongs to this exact proposal.
func (s *Service) workflowResumeStage(session *ChangeSession, from string) (int, error) {
	switch from {
	case "", "create":
		return 0, nil
	case "test":
		if len(session.Changes) > 0 {
			return 1, nil
		}
	case "review":
		if s.hasLivePassingChecks(session) {
			return 2, nil
		}
	default:
		return 0, fmt.Errorf("resume_from must be create, test or review")
	}
	return 0, fmt.Errorf("cannot resume %s without current proposal evidence; run the preceding stage", from)
}

func completeWorkflowStage(stage *ChangeWorkflowStage, now time.Time) {
	stage.Status, stage.Reason = "completed", ""
	if stage.FinishedAt.IsZero() {
		stage.FinishedAt = now
	}
}

func finishWorkflowStageTiming(w *ChangeWorkflow, now time.Time) {
	for i := range w.Stages {
		st := &w.Stages[i]
		if !st.StartedAt.IsZero() && st.FinishedAt.IsZero() {
			st.FinishedAt = now
		}
	}
}

func (s *Service) hasLivePassingChecks(session *ChangeSession) bool {
	authority := s.changeAuthority[session.ProjectID+"/"+session.ID]
	return len(session.Checks) > 0 && requiredChecksPassed(session.Checks) && (authority == "checks:"+session.Hash || authority == "review:"+session.Hash)
}

func (s *Service) prepareWorkflowRun(session *ChangeSession, request ChangeWorkflowRequest, startStage int, cleanedMessages []ChangeMessage) *ChangeWorkflow {
	now := time.Now().UTC()
	session.Revision++ // Invalidates operations admitted before this run.
	attempt := 1
	if session.Workflow != nil {
		attempt += session.Workflow.Attempt + session.Workflow.AutoRepairAttempts
	}
	w := &ChangeWorkflow{Attempt: attempt, ResumeFrom: request.ResumeFrom, ID: fmt.Sprintf("%s-%d", session.ID, session.Revision), Status: "running", Models: request.Models, AutoRepair: request.AutoRepair, StartedAt: now, UpdatedAt: now}
	for i, profile := range request.Models.selections() {
		runtime := s.workflowStageRuntime(i, profile)
		w.Stages = append(w.Stages, ChangeWorkflowStage{Name: []string{"create", "test", "review"}[i], Status: "pending", Model: &runtime.effective})
	}
	w.Stages = append(w.Stages, ChangeWorkflowStage{Name: "human_review", Status: "blocked"})
	for i := 0; i < startStage; i++ {
		if session.Workflow != nil && i < len(session.Workflow.Stages) {
			w.Stages[i] = session.Workflow.Stages[i]
		}
		w.Stages[i].Status, w.Stages[i].Reason = "completed", ""
	}
	retainedChecks := []CheckResult{}
	if len(session.Changes) > 0 && (startStage == 2 || !requiredChecksPassed(session.Checks)) {
		retainedChecks = session.Checks
	}
	session.Workflow, session.Checks, session.ReviewedHash, session.PinnedTests = w, retainedChecks, "", nil
	session.Messages = cleanedMessages
	session.UpdatedAt = now
	return w
}

func validateResumedModels(session *ChangeSession, models ChangeWorkflowModels, startStage int) error {
	if session.Workflow == nil {
		return nil
	}
	profiles := models.selections()
	for i := 0; i < startStage; i++ {
		if session.Workflow.Stages[i].Model.Profile != profiles[i] {
			return fmt.Errorf("retry from %s to change its model, or retain its previously selected model", session.Workflow.Stages[i].Name)
		}
	}
	return nil
}
