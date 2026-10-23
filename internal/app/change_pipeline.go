package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

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
	runtime.effective.Timeout = max(s.workflowModelTimeout, duration(s.runtimes.function.effective.Timeout), duration(runtime.effective.Timeout)).String()
	return runtime
}

// StartChangeWorkflow captures consent and proposal identity before dispatch.
// The HTTP request only admits the worker; polling/Cancel use its persisted ID.
func (s *Service) StartChangeWorkflow(ctx context.Context, id string, request ChangeWorkflowRequest) (*ChangeSession, error) {
	request.Models = s.resolveWorkflowModels(request.Models)
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
	if err := s.admitChangeWorkflow(ctx, session, request, len(cleanedMessages)); err != nil {
		return nil, err
	}
	if err := approveWorkflowPlan(session, request.PlanHash); err != nil {
		return nil, err
	}
	startStage, err := s.workflowResumeStage(session, request.ResumeFrom)
	if err != nil {
		return nil, err
	}
	if err := validateResumedModels(session, request.Models, startStage); err != nil {
		return nil, err
	}
	agentCtx, release, err := s.workflowAgentContext(context.Background(), request.Models)
	if err != nil {
		return nil, err
	}
	refreshChangeScopeSuggestions(root, session)
	retryMessage := workflowRetryMessage(request.Message, session.Workflow, session.Checks)
	w := s.prepareWorkflowRun(session, request, startStage, cleanedMessages)
	if err := storeChangeSession(root, session); err != nil {
		release()
		return nil, err
	}
	if startStage != 2 {
		delete(s.changeAuthority, session.ProjectID+"/"+id)
	}
	budget := s.changeWorkflowTimeout(request.Models, startStage, request.AutoRepair)
	workCtx, cancelTimeout := context.WithTimeoutCause(agentCtx, budget, &workflowTimeoutError{limit: budget})
	cancel := func() { cancelTimeout(); release() }
	workCtx = context.WithValue(workCtx, changeWorkflowContextKey{}, w.ID)
	job := &changeWorkflowJob{root: root, sessionID: id, id: w.ID, models: request.Models, specialists: workflowSpecialistIDs(w), autoRepair: request.AutoRepair, startStage: startStage, cancel: cancel, done: make(chan struct{})}
	s.changeWorkflow.mu.Lock()
	s.changeWorkflow.setJob(job)
	s.changeWorkflow.mu.Unlock()
	go s.runChangeWorkflow(workCtx, job, retryMessage)
	return session, nil
}

func (s *Service) admitChangeWorkflow(ctx context.Context, session *ChangeSession, request ChangeWorkflowRequest, messageCount int) error {
	s.changeWorkflow.mu.Lock()
	existing := s.changeWorkflow.getJob(session.ID)
	busy := existing != nil && !existing.finished
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
	return s.requireProjectExecutionTrust(ctx, session.ProjectRevision)
}

func (s *Service) runChangeWorkflow(ctx context.Context, job *changeWorkflowJob, message string) {
	defer job.cancel()
	defer close(job.done)
	currentMessage := message
	if job.startStage == 0 {
		currentMessage = s.probeUncheckedChangesBeforeWorkflow(ctx, job, message)
	}
	limit := 0
	if job.autoRepair {
		limit = s.agentRuntime.Limits.Repairs
	}
	s.changesMu.Lock()
	session, runErr := loadChangeSession(job.root, job.sessionID)
	var input agentruntime.RepairGraphInput
	if runErr == nil {
		input = agentruntime.RepairGraphInput{Snapshot: changeGraphSnapshot(session), MaxRepairs: limit}
	}
	s.changesMu.Unlock()
	if runErr == nil {
		runErr = agentruntime.RunRepairGraph(ctx, input, func(ctx context.Context, attempt int) (bool, error) {
			if attempt > 0 {
				nextMsg, err := s.prepareWorkflowAutoRepair(ctx, job, message, attempt)
				if err != nil {
					return false, err
				}
				currentMessage = nextMsg
			}
			startStage := 0
			if attempt == 0 {
				startStage = job.startStage
			}
			attemptErr := s.executeChangeGraph(ctx, job, startStage, currentMessage)
			if attempt < limit {
				repair, err := s.shouldAutoRepairWorkflow(ctx, job, attemptErr)
				if err != nil || repair {
					return repair, err
				}
			}
			return false, attemptErr
		})
	}
	s.finishChangeWorkflow(ctx, job, runErr)
}

func (s *Service) probeUncheckedChangesBeforeWorkflow(ctx context.Context, job *changeWorkflowJob, message string) string {
	if strings.Contains(message, "Previous check failures to resolve:") || strings.Contains(message, "Critic review feedback:") {
		return message
	}
	s.changesMu.Lock()
	session, err := loadChangeSession(job.root, job.sessionID)
	needsProbe := err == nil && session.Workflow != nil && session.Workflow.ID == job.id && len(session.Changes) > 0 && len(session.Checks) == 0
	var id ChangeIdentity
	if needsProbe {
		id = identityForChange(session)
	}
	s.changesMu.Unlock()
	if !needsProbe {
		return message
	}
	checked, err := s.CheckChange(ctx, job.sessionID, ChangeCheckRequest{ChangeIdentity: id, CheckOptions: CheckOptions{RunTests: true}})
	if err != nil || requiredChecksPassed(checked.Checks) {
		return message
	}
	return workflowRetryMessage(message, checked.Workflow, checked.Checks)
}

func (s *Service) shouldAutoRepairWorkflow(ctx context.Context, job *changeWorkflowJob, runErr error) (bool, error) {
	if ctx.Err() != nil {
		return false, context.Cause(ctx)
	}
	if runErr != nil && !errors.Is(runErr, errWorkflowChecksFailed) {
		return false, nil
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := loadChangeSession(job.root, job.sessionID)
	if err != nil {
		return false, err
	}
	if session.Workflow == nil || session.Workflow.ID != job.id {
		return false, project.ErrRevisionConflict
	}
	if len(session.ScopeSuggestions) > 0 {
		return false, nil
	}
	if errors.Is(runErr, errWorkflowChecksFailed) {
		return workflowChecksRepairable(session.Checks), nil
	}
	return session.Workflow.Review != nil && session.Workflow.Review.ProposalHash == session.Hash && session.Workflow.Review.Verdict == "changes_requested" && specialistsComplete(session.Workflow, session.Hash), nil
}

// A missing or interrupted check is not evidence that a model should change code.
func workflowChecksRepairable(checks []CheckResult) bool {
	failed := false
	for _, check := range checks {
		if !check.Required || check.State == CheckPassed {
			continue
		}
		if check.State != CheckFailed {
			return false
		}
		failed = true
	}
	return failed
}

func (s *Service) prepareWorkflowAutoRepair(ctx context.Context, job *changeWorkflowJob, baseMessage string, attempt int) (string, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := loadChangeSession(job.root, job.sessionID)
	if err != nil {
		return "", err
	}
	if session.Workflow == nil || session.Workflow.ID != job.id {
		return "", project.ErrRevisionConflict
	}
	if _, err := s.loadChangeGraphSnapshot(ctx, job, changeGraphSnapshot(session)); err != nil {
		return "", err
	}
	refreshChangeScopeSuggestions(job.root, session)
	if len(session.ScopeSuggestions) > 0 {
		return "", fmt.Errorf("approve suggested scope before repairing")
	}
	probe, lessons := probeWorkflowFriction(session.Workflow, session.Checks)
	session.Workflow.ProbeSummary = probe
	session.Workflow.HarnessLessons = mergeHarnessLessons(session.Workflow.HarnessLessons, lessons)
	retryMsg := workflowRetryMessage(baseMessage, session.Workflow, session.Checks)
	session.Messages = stripWorkflowTestMessages(session.Messages)
	session.Workflow.AutoRepairAttempts = attempt
	session.Workflow.Review = nil
	resetWorkflowSpecialists(session.Workflow)
	for i := 0; i < 3 && i < len(session.Workflow.Stages); i++ {
		session.Workflow.Stages[i].Status = "pending"
		session.Workflow.Stages[i].Reason = ""
		session.Workflow.Stages[i].StartedAt, session.Workflow.Stages[i].FinishedAt = time.Time{}, time.Time{}
	}
	session.ReviewedHash = ""
	session.Workflow.UpdatedAt, session.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	if err := storeChangeSession(job.root, session); err != nil {
		return "", err
	}
	s.notifyEvent("workflow")
	return retryMsg, nil
}

var errWorkflowChecksFailed = errors.New("workflow checks failed")

func identityForChange(session *ChangeSession) ChangeIdentity {
	return ChangeIdentity{ProjectID: session.ProjectID, ProjectRevision: session.ProjectRevision, Revision: session.Revision, Hash: session.Hash}
}

func (s *Service) beginChangeWorkflowStage(ctx context.Context, job *changeWorkflowJob, stage int, identity ChangeIdentity) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	snapshot := agentruntime.ChangeSnapshot{ProjectID: identity.ProjectID, ProjectRevision: identity.ProjectRevision, SessionID: job.sessionID, WorkflowID: job.id, Revision: identity.Revision, Hash: identity.Hash}
	session, err := s.loadChangeGraphSnapshot(ctx, job, snapshot)
	if err != nil {
		return nil, err
	}
	if stage > 0 {
		completeWorkflowStage(&session.Workflow.Stages[stage-1], time.Now().UTC())
	}
	session.Workflow.Stages[stage].StartedAt = time.Now().UTC()
	session.Workflow.Stages[stage].Status = "running"
	session.Workflow.Stages[stage].Reason = ""
	session.Workflow.UpdatedAt, session.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	err = storeChangeSession(job.root, session)
	s.notifyEvent("workflow")
	return session, err
}

func (s *Service) finishChangeWorkflow(ctx context.Context, job *changeWorkflowJob, runErr error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, err := loadChangeSession(job.root, job.sessionID)
	if err == nil && session.Workflow != nil && session.Workflow.ID == job.id {
		w := session.Workflow
		finishWorkflowStageTiming(w, time.Now().UTC())
		if runErr == nil {
			runErr = s.verifyChangeCurrent(ctx, job.root, session)
		}
		runErr = workflowDeadlineCause(ctx, runErr)
		if runErr == nil {
			w.Stages[2].Status = "completed"
			w.Status = "changes_requested"
			if w.Review != nil && w.Review.Verdict == "approve" {
				w.Status, w.Stages[3].Status = "awaiting_human_review", "waiting"
			}
		} else {
			w.Status, w.Reason = workflowFailure(runErr, session.Checks)
			stopChangeWorkflowStages(w)
			if !errors.Is(runErr, errWorkflowChecksFailed) && !s.hasLivePassingChecks(session) {
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
		err = storeChangeSession(job.root, session)
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
	var timeout *workflowTimeoutError
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled", "Workflow canceled. No source was applied."
	case errors.As(err, &timeout):
		return "failed", timeout.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "failed", "An operation timed out. Inspect the failed stage and check evidence before retrying."
	case errors.Is(err, project.ErrRevisionConflict), errors.Is(err, project.ErrExcludedFile):
		return "stale", "Project source or instructions changed. Start a fresh conversation."
	case errors.Is(err, errWorkflowChecksFailed):
		base := "Tests or source checks failed. Inspect the check evidence before running again."
		if !workflowChecksRepairable(checks) {
			base = "Required checks did not complete. Inspect the check evidence and retry checks before changing code."
		}
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
