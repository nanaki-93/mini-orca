package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const changeWorkflowBudget = 30 * time.Minute

// StartChangeWorkflow captures consent and proposal identity before dispatch.
// The HTTP request only admits the worker; polling/Cancel use its persisted ID.
func (s *Service) StartChangeWorkflow(ctx context.Context, id string, request ChangeWorkflowRequest) (*ChangeSession, error) {
	if err := request.Models.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Message) == "" || len(request.Message) > 8192 {
		return nil, fmt.Errorf("provide a workflow request of at most 8192 bytes")
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	if err := s.admitChangeWorkflow(session, request); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session.Revision++ // Invalidates operations admitted before this run.
	w := &ChangeWorkflow{ID: fmt.Sprintf("%s-%d", id, session.Revision), Status: "running", Models: request.Models, StartedAt: now, UpdatedAt: now}
	for i, profile := range []string{request.Models.Create, request.Models.Test, request.Models.Review} {
		runtime, _ := s.runtimes.forScope(profile)
		w.Stages = append(w.Stages, ChangeWorkflowStage{Name: []string{"create", "test", "review"}[i], Status: "pending", Model: &runtime.effective})
	}
	w.Stages = append(w.Stages, ChangeWorkflowStage{Name: "human_review", Status: "blocked"})
	session.Workflow, session.Checks, session.ReviewedHash = w, []DraftCheck{}, ""
	session.UpdatedAt = now
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	delete(s.changeAuthority, session.ProjectID+"/"+id)
	workCtx, cancel := context.WithTimeout(context.Background(), changeWorkflowBudget)
	workCtx = context.WithValue(workCtx, changeWorkflowContextKey{}, w.ID)
	job := &changeWorkflowJob{root: root, sessionID: id, id: w.ID, models: request.Models, cancel: cancel, done: make(chan struct{})}
	s.changeWorkflow.mu.Lock()
	s.changeWorkflow.job = job
	s.changeWorkflow.mu.Unlock()
	go s.runChangeWorkflow(workCtx, job, request.Message)
	return session, nil
}

func (s *Service) admitChangeWorkflow(session *ChangeSession, request ChangeWorkflowRequest) error {
	s.changeWorkflow.mu.Lock()
	busy := s.changeWorkflow.job != nil && !s.changeWorkflow.job.finished
	s.changeWorkflow.mu.Unlock()
	if busy {
		return fmt.Errorf("%w: a change workflow is already active", project.ErrRevisionConflict)
	}
	if session.Kind == "instructions" || len(session.Messages) > maxChangeMessages-4 {
		return fmt.Errorf("start a new feature, fix, performance or security conversation")
	}
	if session.Kind == "security" && !request.ConfirmSecurity {
		return fmt.Errorf("security workflow requires explicit review intent")
	}
	hasTestTarget := false
	for _, target := range session.Targets {
		hasTestTarget = hasTestTarget || strings.HasSuffix(target.Path, "_test.go")
	}
	if !hasTestTarget {
		return fmt.Errorf("include a Go test path (_test.go) in the captured scope for the testing agent")
	}
	for _, profile := range []string{request.Models.Create, request.Models.Test, request.Models.Review} {
		runtime, _ := s.runtimes.forScope(profile)
		if err := requireModelRuntimeConfirmation(runtime, slices.Contains(request.ConfirmedProfiles, profile)); err != nil {
			return err
		}
	}
	return s.requireProjectExecutionTrust(session.ProjectRevision)
}

func (s *Service) runChangeWorkflow(ctx context.Context, job *changeWorkflowJob, message string) {
	defer job.cancel()
	defer close(job.done)
	var runErr error
	for stage := 0; stage < 3; stage++ {
		if runErr = s.executeChangeWorkflowStage(ctx, job, stage, message); runErr != nil {
			break
		}
	}
	s.finishChangeWorkflow(ctx, job, runErr)
}

func (s *Service) executeChangeWorkflowStage(ctx context.Context, job *changeWorkflowJob, stage int, message string) error {
	session, err := s.beginChangeWorkflowStage(ctx, job, stage)
	if err != nil {
		return err
	}
	runtime, _ := s.runtimes.forScope([]string{job.models.Create, job.models.Test, job.models.Review}[stage])
	switch stage {
	case 0:
		_, err = s.generateChangeWithRuntime(ctx, job.root, session, ChangeMessageRequest{ChangeIdentity: identityForChange(session), Message: message}, runtime)
	case 1:
		session, err = s.generateWorkflowTests(ctx, job.root, session, runtime)
		if err == nil {
			session, err = s.CheckChange(ctx, job.sessionID, ChangeCheckRequest{ChangeIdentity: identityForChange(session), DraftCheckOptions: DraftCheckOptions{RunTests: true}})
		}
		if err == nil && !requiredChecksPassed(session.Checks) {
			err = errWorkflowChecksFailed
		}
	case 2:
		err = s.reviewWorkflowProposal(ctx, job.root, session, runtime)
	}
	return err
}

var errWorkflowChecksFailed = errors.New("workflow checks failed")

func identityForChange(session *ChangeSession) ChangeIdentity {
	return ChangeIdentity{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Revision: session.Revision, Hash: session.Hash}
}

func (s *Service) beginChangeWorkflowStage(ctx context.Context, job *changeWorkflowJob, stage int) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := readChangeSession(job.root, job.sessionID)
	if err != nil {
		return nil, err
	}
	if session.Workflow == nil || session.Workflow.ID != job.id || session.Workflow.Models != job.models {
		return nil, project.ErrRevisionConflict
	}
	if _, _, err := s.loadChangeForAction(ctx, session.ID, identityForChange(session)); err != nil {
		return nil, err
	}
	if err := s.requireProjectExecutionTrust(session.ProjectRevision); err != nil {
		return nil, err
	}
	if stage > 0 {
		session.Workflow.Stages[stage-1].Status = "completed"
	}
	session.Workflow.Stages[stage].Status = "running"
	session.Workflow.UpdatedAt, session.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	return session, writeChangeSession(job.root, session)
}

func (s *Service) finishChangeWorkflow(ctx context.Context, job *changeWorkflowJob, runErr error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := readChangeSession(job.root, job.sessionID)
	if err == nil && session.Workflow != nil && session.Workflow.ID == job.id {
		w := session.Workflow
		if runErr == nil {
			runErr = s.verifyChangeCurrent(ctx, job.root, session)
		}
		if runErr == nil {
			w.Stages[2].Status = "completed"
			w.Status = "changes_requested"
			if w.Review != nil && w.Review.Verdict == "approve" {
				w.Status, w.Stages[3].Status = "awaiting_human_review", "waiting"
			}
		} else {
			w.Status, w.Reason = workflowFailure(runErr)
			stopChangeWorkflowStages(w)
			delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
		}
		w.UpdatedAt, session.UpdatedAt = time.Now().UTC(), time.Now().UTC()
		err = writeChangeSession(job.root, session)
	}
	s.changeWorkflow.mu.Lock()
	defer s.changeWorkflow.mu.Unlock()
	job.finished = true
	if err != nil {
		job.failure = "Workflow progress could not be saved. Restore the conversation and run again."
	}
}

func workflowFailure(err error) (string, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled", "Workflow canceled. No source was applied."
	case errors.Is(err, context.DeadlineExceeded):
		return "failed", "Workflow timed out. Reduce scope or check the selected provider."
	case errors.Is(err, project.ErrRevisionConflict), errors.Is(err, project.ErrExcludedFile):
		return "stale", "Project source or instructions changed. Start a fresh conversation."
	case errors.Is(err, errWorkflowChecksFailed):
		return "failed", "Tests or source checks failed. Inspect the check evidence before running again."
	default:
		return "failed", "The stage could not complete or returned an invalid response. Check its model and captured scope, then run again."
	}
}
