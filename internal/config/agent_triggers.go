package config

import "fmt"

// Trigger configuration only sets ceilings. A project must explicitly enable a
// watch and separately grant provider work before any automatic model call.
type AgentTriggers struct {
	PollSeconds     int   `yaml:"poll_seconds" json:"poll_seconds"`
	DebounceSeconds int   `yaml:"debounce_seconds" json:"debounce_seconds"`
	QuietSeconds    int   `yaml:"quiet_seconds" json:"quiet_seconds"`
	WindowSeconds   int   `yaml:"window_seconds" json:"window_seconds"`
	MaxRuns         int   `yaml:"max_runs" json:"max_runs"`
	MaxModelCalls   int   `yaml:"max_model_calls" json:"max_model_calls"`
	MaxInputTokens  int64 `yaml:"max_input_tokens" json:"max_input_tokens"`
	MaxOutputTokens int64 `yaml:"max_output_tokens" json:"max_output_tokens"`
	RunSeconds      int   `yaml:"run_seconds" json:"run_seconds"`
	GrantSeconds    int   `yaml:"grant_seconds" json:"grant_seconds"`
}

func ResolveAgentTriggers(value AgentTriggers) (AgentTriggers, error) {
	for _, field := range []struct {
		target   *int
		fallback int
	}{
		{&value.PollSeconds, 2}, {&value.DebounceSeconds, 3}, {&value.QuietSeconds, 60},
		{&value.WindowSeconds, 3600}, {&value.MaxRuns, 4}, {&value.MaxModelCalls, 16},
		{&value.RunSeconds, 120}, {&value.GrantSeconds, 3600},
	} {
		if *field.target == 0 {
			*field.target = field.fallback
		}
	}
	if value.MaxInputTokens == 0 {
		value.MaxInputTokens = 4 << 20
	}
	if value.MaxOutputTokens == 0 {
		value.MaxOutputTokens = 256 << 10
	}
	if err := value.Validate(); err != nil {
		return AgentTriggers{}, err
	}
	return value, nil
}

func (value AgentTriggers) Validate() error {
	for _, field := range []struct {
		name                    string
		value, minimum, maximum int64
	}{
		{"poll_seconds", int64(value.PollSeconds), 1, 60},
		{"debounce_seconds", int64(value.DebounceSeconds), 1, 300},
		{"quiet_seconds", int64(value.QuietSeconds), 5, 3600},
		{"window_seconds", int64(value.WindowSeconds), 60, 86400},
		{"max_runs", int64(value.MaxRuns), 1, 16},
		{"max_model_calls", int64(value.MaxModelCalls), 1, 64},
		{"max_input_tokens", value.MaxInputTokens, 1024, 64 << 20},
		{"max_output_tokens", value.MaxOutputTokens, 1024, 16 << 20},
		{"run_seconds", int64(value.RunSeconds), 1, 300},
		{"grant_seconds", int64(value.GrantSeconds), 60, 86400},
	} {
		if field.value < field.minimum || field.value > field.maximum {
			return fmt.Errorf("agent_runtime.triggers.%s must be between %d and %d", field.name, field.minimum, field.maximum)
		}
	}
	if value.MaxModelCalls < value.MaxRuns || value.RunSeconds > value.WindowSeconds || value.QuietSeconds > value.WindowSeconds || value.DebounceSeconds > value.WindowSeconds {
		return fmt.Errorf("trigger windows must contain their run/quiet/debounce allowances and at least one model call per run")
	}
	return nil
}

func (value AgentTriggers) RunLimits(ceiling AgentLimits) AgentLimits {
	return AgentLimits{ModelCalls: min(value.MaxModelCalls/value.MaxRuns, ceiling.ModelCalls), ToolCalls: 1,
		InputTokens: min(value.MaxInputTokens/int64(value.MaxRuns), ceiling.InputTokens), OutputTokens: min(value.MaxOutputTokens/int64(value.MaxRuns), ceiling.OutputTokens),
		ElapsedSeconds: min(value.RunSeconds, ceiling.ElapsedSeconds), ConcurrentCalls: 1, Repairs: 0}
}
