package app

import "time"

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
