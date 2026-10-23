package app

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type workflowTimeoutError struct {
	stage string
	limit time.Duration
}

func (e *workflowTimeoutError) Error() string {
	if e.stage == "" {
		return fmt.Sprintf("Workflow reached its total %s budget. Inspect the saved evidence and retry the unfinished stage. No source was applied.", e.limit)
	}
	return fmt.Sprintf("%s model request timed out after %s, including retries. Increase timeouts.workflow_model_seconds or choose a faster model, then retry this stage. No source was applied.", e.stage, e.limit)
}

func (e *workflowTimeoutError) Unwrap() error { return context.DeadlineExceeded }

// Preserve a more specific owner deadline when a child returns the context sentinel.
func workflowDeadlineCause(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		return context.Cause(ctx)
	}
	return err
}

// A run must accommodate every remaining model call, including optional repairs.
// Provider retries share each model's deadline; they do not multiply this budget.
func (s *Service) changeWorkflowTimeout(models ChangeWorkflowModels, startStage int, autoRepair bool) time.Duration {
	var full, remaining time.Duration
	for stage, profile := range models.selections() {
		runtime := s.workflowStageRuntime(stage, profile)
		limit := duration(runtime.effective.Timeout)
		full += limit
		if stage >= startStage {
			remaining += limit
		}
	}
	// One check pass has at most five commands: two baselines, vet, gitleaks
	// and candidate tests. Reserve preparation time for copying and identity work.
	checks := 5*s.focusedCheckTimeout + 5*time.Minute
	budget := remaining + checks
	if startStage == 0 {
		budget += checks // An existing proposal may need a pre-generation probe.
	}
	if autoRepair {
		budget += maxWorkflowAutoRepairs * (full + checks)
	}
	return max(30*time.Minute, budget)
}
