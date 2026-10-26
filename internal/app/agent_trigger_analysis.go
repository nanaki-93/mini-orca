package app

import (
	"context"
	"crypto/rand"
	"errors"
	"maps"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func triggerWindowAllowance(limits config.AgentTriggers) agentstate.TriggerAllowance {
	return agentstate.TriggerAllowance{Runs: limits.MaxRuns, ModelCalls: limits.MaxModelCalls, InputTokens: limits.MaxInputTokens, OutputTokens: limits.MaxOutputTokens, Seconds: min(limits.WindowSeconds, limits.MaxRuns*limits.RunSeconds)}
}

func triggerRunAllowance(limits config.AgentLimits) agentstate.TriggerAllowance {
	return agentstate.TriggerAllowance{Runs: 1, ModelCalls: limits.ModelCalls, InputTokens: limits.InputTokens, OutputTokens: limits.OutputTokens, Seconds: limits.ElapsedSeconds}
}

func (s *Service) runTriggeredAnalysis(owner *triggerOwner, before, record agentstate.TriggerRecord, path string, now time.Time) error {
	admitted, err := s.prepareTriggeredAttempt(owner, before, &record, path, now)
	if err != nil || !admitted {
		return err
	}
	deadline := record.Grant.ExpiresAt
	if record.Window.EndsAt.Before(deadline) {
		deadline = record.Window.EndsAt
	}
	timed, stop := context.WithDeadline(owner.ctx, deadline)
	defer stop()
	bounded, release, err := agentruntime.WithBudget(timed, record.Grant.RunLimits)
	if err != nil {
		return s.finishTriggeredAttempt(owner, record, nil, agentstate.BudgetUsage{}, err)
	}
	defer release()
	guarded := agentruntime.WithContextAuthority(bounded, s.triggerAuthority(owner, record))
	analysis, err := s.manager.Analysis()
	if err != nil {
		return s.finishTriggeredAttempt(owner, record, nil, agentruntime.BudgetFromContext(bounded).Snapshot(), err)
	}
	runtime := s.semanticCacheRuntime(analysis)
	guarded, err = agentruntime.WithPolicy(guarded, runtime.client)
	var result *project.FileAnalysis
	if err == nil {
		result, err = s.AnalyzeFile(guarded, path, false, true)
	}
	return s.finishTriggeredAttempt(owner, record, result, agentruntime.BudgetFromContext(bounded).Snapshot(), err)
}

func (s *Service) prepareTriggeredAttempt(owner *triggerOwner, before agentstate.TriggerRecord, record *agentstate.TriggerRecord, path string, now time.Time) (bool, error) {
	// Lifecycle operations can cancel and join a watcher while holding this lock.
	// A watcher never waits for it, so shutdown cannot deadlock with admission.
	if !s.jobLifecycleMu.TryLock() {
		record.State, record.Reason = "waiting", "Another project operation owns admission."
		return false, saveTriggerObservation(owner, before, *record)
	}
	defer s.jobLifecycleMu.Unlock()
	if err := owner.ctx.Err(); err != nil {
		return false, err
	}
	if s.manager.Root() != owner.root {
		return false, project.ErrRevisionConflict
	}
	if s.triggerCompetingWork() {
		record.State, record.Reason = "waiting", "An active project job has priority over ambient analysis."
		return false, saveTriggerObservation(owner, before, *record)
	}
	index, err := s.manager.Index()
	if err != nil {
		return false, err
	}
	view, err := s.AgentTriggers(owner.ctx, AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision})
	if err != nil {
		return false, err
	}
	if view.State != "granted" || view.Record.ControlRevision != owner.controlRevision || view.Record.Grant.ID != record.Grant.ID {
		record.State, record.Reason = "stale", "Automatic permission is stale; review the current scope and provider before granting again."
		return false, saveTriggerObservation(owner, before, *record)
	}
	index, err = s.reindexForTrigger(owner, path, record.Observed[path])
	if err != nil {
		return false, err
	}
	if !reserveTriggerAttempt(record, owner.limits, path, index.ProjectRevision, now) {
		return false, saveTriggerObservation(owner, before, *record)
	}
	saved, err := owner.store.SaveTriggers(owner.ctx, *record, before.Version)
	if err != nil {
		return false, err
	}
	*record = saved
	s.notifyEvent("agent_triggers")
	return true, nil
}

func (s *Service) reindexForTrigger(owner *triggerOwner, path, hash string) (*project.ProjectIndex, error) {
	file, err := s.manager.IndexedFile(path)
	if err != nil {
		return nil, err
	}
	if file.ContentHash != "sha256:"+hash {
		s.cancelMissionWaves()
		s.cancelIntegrations()
		s.cancelChangeWorkflowsBecause(stopReasonFileChanged)
		s.cancelGoScan()
		if _, err := s.manager.Reindex(); err != nil {
			return nil, err
		}
		s.resetExecutionTrust(true)
		s.refreshAnalysisRunAfterReindex()
		s.notifyEvent("project_index")
	}
	file, err = s.manager.IndexedFile(path)
	if err != nil {
		return nil, err
	}
	if file.ContentHash != "sha256:"+hash || !isSemanticAnalysisCandidate(*file) {
		return nil, project.ErrRevisionConflict
	}
	return s.manager.Index()
}

func reserveTriggerAttempt(record *agentstate.TriggerRecord, limits config.AgentTriggers, path, revision string, now time.Time) bool {
	ceiling := triggerWindowAllowance(limits)
	if record.Window.StartedAt.IsZero() || !record.Window.EndsAt.After(now) {
		record.Window = agentstate.TriggerWindow{StartedAt: now, EndsAt: now.Add(time.Duration(limits.WindowSeconds) * time.Second), Limits: ceiling}
	}
	charge := triggerRunAllowance(record.Grant.RunLimits)
	charged := record.Window.Charged.Add(charge)
	if !charged.Within(record.Window.Limits) || !charged.Within(ceiling) {
		record.State, record.Reason, record.NextEligibleAt = "budget_exhausted", "The persisted window allowance is exhausted; earlier charges are retained.", record.Window.EndsAt
		return false
	}
	record.Window.Charged = charged
	record.Handled = maps.Clone(record.Handled)
	record.Handled[path] = record.Observed[path]
	record.LastAttempt = &agentstate.TriggerAttempt{ID: rand.Text(), ProjectRevision: revision, Path: path, SourceHash: record.Observed[path], State: "running", StartedAt: now, Reserved: charge}
	record.State, record.Reason = "running", "A bounded semantic analysis attempt owns its reserved window allowance."
	return true
}

func (s *Service) triggerAuthority(owner *triggerOwner, admitted agentstate.TriggerRecord) func(context.Context) error {
	return func(ctx context.Context) error {
		attempt := admitted.LastAttempt
		view, err := s.AgentTriggers(ctx, AgentRunQuery{ProjectID: admitted.ProjectID, ProjectRevision: attempt.ProjectRevision})
		if err != nil {
			return err
		}
		current := view.Record
		if view.State != "granted" || current.ControlRevision != admitted.ControlRevision || current.Grant.ID != admitted.Grant.ID || current.LastAttempt == nil || current.LastAttempt.ID != attempt.ID || current.LastAttempt.State != "running" {
			return agentruntime.ErrPolicyDenied
		}
		policy, err := project.NewContextPolicy(owner.root)
		if err != nil {
			return err
		}
		_, err = policy.ReadSource(ctx, attempt.Path, "sha256:"+attempt.SourceHash)
		return err
	}
}

func (s *Service) finishTriggeredAttempt(owner *triggerOwner, record agentstate.TriggerRecord, result *project.FileAnalysis, usage agentstate.BudgetUsage, runErr error) error {
	now := time.Now().UTC()
	state, reason := triggerAttemptOutcome(result, runErr)
	record.LastAttempt.State, record.LastAttempt.FinishedAt, record.LastAttempt.Usage = state, now, usage
	record.State, record.Reason = state, reason
	delay := owner.limits.QuietSeconds
	if record.Options.Mode == "interval" {
		delay = max(delay, record.Options.IntervalSeconds)
	}
	record.NextEligibleAt = now.Add(time.Duration(delay) * time.Second)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(owner.ctx), 5*time.Second)
	defer cancel()
	_, err := owner.store.SaveTriggers(ctx, record, record.Version)
	s.notifyEvent("agent_triggers")
	return err
}

func triggerAttemptOutcome(result *project.FileAnalysis, err error) (string, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled", "The attempt stopped; its reserved allowance remains charged."
	case errors.Is(err, agentruntime.ErrBudgetExceeded), errors.Is(err, context.DeadlineExceeded):
		return "failed", "The attempt reached its allowance or deadline; inspect retained analysis before another grant."
	case err != nil, result == nil:
		return "failed", "Analysis failed or its receipt is unavailable; no automatic input replay occurs."
	case result.Status != project.AnalysisStatusFresh:
		return "failed", "Analysis did not produce a fresh advisory result; inspect the file's retained status."
	default:
		return "completed", "Advisory semantic analysis completed. Source changes still require explicit proposal review and Apply."
	}
}
