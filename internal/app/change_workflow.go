package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const changeWorkflowBudget = 30 * time.Minute

func workflowStageScope(stage int) config.ModelScope {
	switch stage {
	case 0:
		return config.BugModelScope
	case 1:
		return config.FunctionModelScope
	default:
		return config.AnalyzeModelScope
	}
}

func (s *Service) workflowStageRuntime(stage int, profile string) modelRuntime {
	runtime, _ := s.selectedRuntime(workflowStageScope(stage), profile)
	return runtime
}

// StartChangeWorkflow captures consent and proposal identity before dispatch.
// The HTTP request only admits the worker; polling/Cancel use its persisted ID.
func (s *Service) StartChangeWorkflow(ctx context.Context, id string, request ChangeWorkflowRequest) (*ChangeSession, error) {
	if err := request.Models.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Message) == "" || len(request.Message) > 8192 {
		return nil, fmt.Errorf("provide a workflow request of at most 8192 bytes")
	}
	if err := s.validateModelSelections(ctx, request.Models.selections()); err != nil {
		return nil, err
	}
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	cleanedMessages := stripWorkflowTestMessages(session.Messages)
	if err := s.admitChangeWorkflow(session, request, len(cleanedMessages)); err != nil {
		return nil, err
	}
	retryMessage := workflowRetryMessage(request.Message, session.Workflow, session.Checks)
	now := time.Now().UTC()
	session.Revision++ // Invalidates operations admitted before this run.
	w := &ChangeWorkflow{ID: fmt.Sprintf("%s-%d", id, session.Revision), Status: "running", Models: request.Models, AutoRepair: request.AutoRepair, StartedAt: now, UpdatedAt: now}
	for i, profile := range request.Models.selections() {
		runtime := s.workflowStageRuntime(i, profile)
		w.Stages = append(w.Stages, ChangeWorkflowStage{Name: []string{"create", "test", "review"}[i], Status: "pending", Model: &runtime.effective})
	}
	w.Stages = append(w.Stages, ChangeWorkflowStage{Name: "human_review", Status: "blocked"})
	session.Workflow, session.Checks, session.ReviewedHash, session.PinnedTests = w, []CheckResult{}, "", nil
	session.Messages = cleanedMessages
	session.UpdatedAt = now
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	delete(s.changeAuthority, session.ProjectID+"/"+id)
	workCtx, cancel := context.WithTimeout(context.Background(), changeWorkflowBudget)
	workCtx = context.WithValue(workCtx, changeWorkflowContextKey{}, w.ID)
	job := &changeWorkflowJob{root: root, sessionID: id, id: w.ID, models: request.Models, autoRepair: request.AutoRepair, cancel: cancel, done: make(chan struct{})}
	s.changeWorkflow.mu.Lock()
	s.changeWorkflow.job = job
	s.changeWorkflow.mu.Unlock()
	go s.runChangeWorkflow(workCtx, job, retryMessage)
	return session, nil
}

func (s *Service) admitChangeWorkflow(session *ChangeSession, request ChangeWorkflowRequest, messageCount int) error {
	s.changeWorkflow.mu.Lock()
	busy := s.changeWorkflow.job != nil && !s.changeWorkflow.job.finished
	s.changeWorkflow.mu.Unlock()
	if busy {
		return fmt.Errorf("%w: a change workflow is already active", project.ErrRevisionConflict)
	}
	if session.Kind == "instructions" || messageCount > maxChangeMessages-4 {
		return fmt.Errorf("start a new feature, fix, performance or security conversation")
	}
	if session.Kind == "security" && !request.ConfirmSecurity {
		return fmt.Errorf("security workflow requires explicit review intent")
	}
	hasTestTarget := false
	for _, target := range session.Targets {
		hasTestTarget = hasTestTarget || isTestFilePath(target.Path)
	}
	if !hasTestTarget {
		return fmt.Errorf("include a test path (e.g. _test.go, .test.ts, test_*.py) in the captured scope for the testing agent")
	}
	for i, profile := range request.Models.selections() {
		runtime := s.workflowStageRuntime(i, profile)
		if err := requireModelRuntimeConfirmation(runtime, slices.Contains(request.ConfirmedProfiles, profile)); err != nil {
			return err
		}
	}
	return s.requireProjectExecutionTrust(session.ProjectRevision)
}

const maxWorkflowAutoRepairs = 2

func (s *Service) runChangeWorkflow(ctx context.Context, job *changeWorkflowJob, message string) {
	defer job.cancel()
	defer close(job.done)
	var runErr error
	currentMessage := message
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			nextMsg, ok := s.prepareWorkflowAutoRepair(ctx, job, message, attempt)
			if !ok {
				break
			}
			currentMessage = nextMsg
		}
		for stage := 0; stage < 3; stage++ {
			if runErr = s.executeChangeWorkflowStage(ctx, job, stage, currentMessage); runErr != nil {
				break
			}
		}
		if !job.autoRepair || attempt >= maxWorkflowAutoRepairs || !s.shouldAutoRepairWorkflow(ctx, job, runErr) {
			break
		}
	}
	s.finishChangeWorkflow(ctx, job, runErr)
}

func (s *Service) shouldAutoRepairWorkflow(ctx context.Context, job *changeWorkflowJob, runErr error) bool {
	if ctx.Err() != nil {
		return false
	}
	if errors.Is(runErr, errWorkflowChecksFailed) {
		return true
	}
	if runErr != nil {
		return false
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := readChangeSession(job.root, job.sessionID)
	if err != nil || session.Workflow == nil || session.Workflow.Review == nil {
		return false
	}
	return session.Workflow.Review.Verdict == "changes_requested"
}

func (s *Service) prepareWorkflowAutoRepair(ctx context.Context, job *changeWorkflowJob, baseMessage string, attempt int) (string, bool) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := readChangeSession(job.root, job.sessionID)
	if err != nil || session.Workflow == nil || session.Workflow.ID != job.id {
		return "", false
	}
	if _, _, err := s.loadChangeForAction(ctx, session.ID, identityForChange(session)); err != nil {
		return "", false
	}
	probe, lessons := probeWorkflowFriction(session.Workflow, session.Checks)
	session.Workflow.ProbeSummary = probe
	session.Workflow.HarnessLessons = mergeHarnessLessons(session.Workflow.HarnessLessons, lessons)
	retryMsg := workflowRetryMessage(baseMessage, session.Workflow, session.Checks)
	session.Messages = stripWorkflowTestMessages(session.Messages)
	session.Workflow.AutoRepairAttempts = attempt
	session.Workflow.Review = nil
	for i := 0; i < 3 && i < len(session.Workflow.Stages); i++ {
		session.Workflow.Stages[i].Status = "pending"
	}
	session.Checks, session.ReviewedHash = []CheckResult{}, ""
	session.Workflow.UpdatedAt, session.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	if err := writeChangeSession(job.root, session); err != nil {
		return "", false
	}
	s.notifyEvent("workflow")
	return retryMsg, true
}

func (s *Service) executeChangeWorkflowStage(ctx context.Context, job *changeWorkflowJob, stage int, message string) error {
	session, err := s.beginChangeWorkflowStage(ctx, job, stage)
	if err != nil {
		return err
	}
	runtime := s.workflowStageRuntime(stage, job.models.selections()[stage])
	switch stage {
	case 0:
		isRepair := session.Workflow != nil && session.Workflow.AutoRepairAttempts > 0
		_, err = s.generateChangeWithRuntime(ctx, job.root, session, ChangeMessageRequest{ChangeIdentity: identityForChange(session), Message: message, Repair: isRepair}, runtime)
	case 1:
		session, err = s.generateWorkflowTests(ctx, job.root, session, runtime)
		if err == nil {
			session, err = s.CheckChange(ctx, job.sessionID, ChangeCheckRequest{ChangeIdentity: identityForChange(session), CheckOptions: CheckOptions{RunTests: true}})
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
	err = writeChangeSession(job.root, session)
	s.notifyEvent("workflow")
	return session, err
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
			w.Status, w.Reason = workflowFailure(runErr, session.Checks)
			stopChangeWorkflowStages(w)
			if !errors.Is(runErr, errWorkflowChecksFailed) {
				delete(s.changeAuthority, session.ProjectID+"/"+session.ID)
			}
		}
		if probe, lessons := probeWorkflowFriction(w, session.Checks); probe != "" || len(lessons) > 0 {
			if w.ProbeSummary == "" {
				w.ProbeSummary = probe
			}
			w.HarnessLessons = mergeHarnessLessons(w.HarnessLessons, lessons)
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
	s.notifyEvent("workflow")
}

func workflowFailure(err error, checks []CheckResult) (string, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled", "Workflow canceled. No source was applied."
	case errors.Is(err, context.DeadlineExceeded):
		return "failed", "Workflow timed out. Reduce scope or check the selected provider."
	case errors.Is(err, project.ErrRevisionConflict), errors.Is(err, project.ErrExcludedFile):
		return "stale", "Project source or instructions changed. Start a fresh conversation."
	case errors.Is(err, errWorkflowChecksFailed):
		base := "Tests or source checks failed. Inspect the check evidence before running again."
		if detail := summarizeFailedChecks(checks); detail != "" {
			return "failed", base + " (" + detail + ")"
		}
		return "failed", base
	default:
		base := "The stage could not complete or returned an invalid response. Check its model and captured scope, then run again."
		if detail := sanitizeWorkflowError(err); detail != "" {
			return "failed", base + " (" + detail + ")"
		}
		return "failed", base
	}
}

func summarizeFailedChecks(checks []CheckResult) string {
	for _, check := range checks {
		if !check.Required || check.State == CheckPassed {
			continue
		}
		line := firstCheckFailureLine(check.Output)
		if line != "" {
			return check.Name + ": " + line
		}
		return check.Name + ": " + check.State
	}
	return ""
}

func firstCheckFailureLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.Contains(trimmed, "--- FAIL:") || strings.HasPrefix(trimmed, "FAIL") || strings.Contains(trimmed, "Error:") || strings.Contains(trimmed, "undefined:") {
			if len(trimmed) > 140 {
				return trimmed[:140] + "…"
			}
			return trimmed
		}
	}
	for _, raw := range lines {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			if len(trimmed) > 140 {
				return trimmed[:140] + "…"
			}
			return trimmed
		}
	}
	return ""
}

func sanitizeWorkflowError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	if len(text) > 160 {
		text = text[:160] + "…"
	}
	return text
}
