package config

import "fmt"

type AgentRoles struct {
	Creation ModelScope `yaml:"creation"`
	Testing  ModelScope `yaml:"testing"`
	Review   ModelScope `yaml:"review"`
}

type AgentRuntimeConfig struct {
	Roles              AgentRoles `yaml:"roles"`
	MaxModelCalls      int        `yaml:"max_model_calls"`
	MaxToolCalls       int        `yaml:"max_tool_calls"`
	MaxInputTokens     int64      `yaml:"max_input_tokens"`
	MaxOutputTokens    int64      `yaml:"max_output_tokens"`
	MaxElapsedSeconds  int        `yaml:"max_elapsed_seconds"`
	MaxConcurrentCalls int        `yaml:"max_concurrent_calls"`
	MaxRepairs         *int       `yaml:"max_repairs"`
}

// AgentLimits is immutable after admission. Token budgets use reported usage
// when available and explicit conservative estimates otherwise.
type AgentLimits struct {
	ModelCalls      int   `json:"model_calls"`
	ToolCalls       int   `json:"tool_calls"`
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ElapsedSeconds  int   `json:"elapsed_seconds"`
	ConcurrentCalls int   `json:"concurrent_calls"`
	Repairs         int   `json:"repairs"`
}

type AgentRuntime struct {
	Roles  AgentRoles
	Limits AgentLimits
}

func ResolveAgentRuntime(configured AgentRuntimeConfig) (AgentRuntime, error) {
	roles := configured.Roles
	if roles.Creation == "" {
		roles.Creation = BugModelScope
	}
	if roles.Testing == "" {
		roles.Testing = FunctionModelScope
	}
	if roles.Review == "" {
		roles.Review = AnalyzeModelScope
	}
	for _, scope := range []ModelScope{roles.Creation, roles.Testing, roles.Review} {
		if scope != AnalyzeModelScope && scope != BugModelScope && scope != FunctionModelScope {
			return AgentRuntime{}, fmt.Errorf("agent_runtime.roles must reference analyze, bug, or function profiles")
		}
	}
	limits := AgentLimits{ModelCalls: configured.MaxModelCalls, ToolCalls: configured.MaxToolCalls, InputTokens: configured.MaxInputTokens, OutputTokens: configured.MaxOutputTokens, ElapsedSeconds: configured.MaxElapsedSeconds, ConcurrentCalls: configured.MaxConcurrentCalls, Repairs: 2}
	for _, item := range []struct {
		value    *int
		fallback int
	}{
		{&limits.ModelCalls, 1024}, {&limits.ToolCalls, 4096}, {&limits.ElapsedSeconds, 86400}, {&limits.ConcurrentCalls, 4},
	} {
		if *item.value == 0 {
			*item.value = item.fallback
		}
	}
	if limits.InputTokens == 0 {
		limits.InputTokens = 128 << 20
	}
	if limits.OutputTokens == 0 {
		limits.OutputTokens = 64 << 20
	}
	if configured.MaxRepairs != nil {
		limits.Repairs = *configured.MaxRepairs
	}
	if err := limits.Validate(); err != nil {
		return AgentRuntime{}, err
	}
	return AgentRuntime{Roles: roles, Limits: limits}, nil
}

func (limits AgentLimits) Validate() error {
	for _, item := range []struct {
		name                    string
		value, minimum, maximum int64
	}{
		{"max_model_calls", int64(limits.ModelCalls), 1, 4096}, {"max_tool_calls", int64(limits.ToolCalls), 1, 16384},
		{"max_input_tokens", limits.InputTokens, 1, 1 << 40}, {"max_output_tokens", limits.OutputTokens, 1, 1 << 40},
		{"max_elapsed_seconds", int64(limits.ElapsedSeconds), 1, 604800}, {"max_concurrent_calls", int64(limits.ConcurrentCalls), 1, 32},
		{"max_repairs", int64(limits.Repairs), 0, 2},
	} {
		if item.value < item.minimum || item.value > item.maximum {
			return fmt.Errorf("agent_runtime.%s must be between %d and %d", item.name, item.minimum, item.maximum)
		}
	}
	return nil
}
