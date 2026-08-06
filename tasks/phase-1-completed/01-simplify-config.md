# Task 1.1: Simplify internal/config/config.go

## Goal
Remove `ModelsConfig`, `ProviderConfig`, `PhaseModelConfig` structs. Replace with flat `LLMConfig`.

## Current State
- `Config` has `Models ModelsConfig` with `ActiveProvider`, `Providers map[string]ProviderConfig`, `Phases map[string]PhaseModelConfig`
- `ProviderConfig` has `BaseURL`, `APIKey`
- `PhaseModelConfig` has `Provider`, `Model`, `Temperature`, `MaxTokens`

## Implementation Steps

### Step 1: Replace struct definitions
In `internal/config/config.go`, replace the existing config structs:

**Remove:**
```go
type ModelsConfig struct { ... }
type ProviderConfig struct { ... }
type PhaseModelConfig struct { ... }
```

**Add:**
```go
type LLMConfig struct {
    BaseURL     string  `json:"base_url" yaml:"base_url"`
    APIKey      string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
    Model       string  `json:"model" yaml:"model"`
    Temperature float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
    MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
}

type Config struct {
    LLM     LLMConfig     `json:"llm" yaml:"llm"`
    Agents  AgentsConfig  `json:"agents" yaml:"agents"`
    Skills  SkillsConfig  `json:"skills" yaml:"skills"`
    Retry   RetryConfig   `json:"retry" yaml:"retry"`
    Logging LoggingConfig `json:"logging" yaml:"logging"`
}
```

### Step 2: Update applyDefaults()
Replace the models-related default logic:

**Remove:**
```go
if c.Models.Providers == nil { ... }
if c.Models.Phases == nil { ... }
```

**Add:**
```go
if c.LLM.BaseURL == "" {
    c.LLM.BaseURL = DefaultProviderURL
}
if c.LLM.Model == "" {
    c.LLM.Model = ""
}
if c.LLM.Temperature == 0 {
    c.LLM.Temperature = 0.7
}
if c.LLM.MaxTokens == 0 {
    c.LLM.MaxTokens = 8192
}
```

### Step 3: Update Validate()
Replace model validation with LLM validation:

**Remove:**
```go
if c.Models.ActiveProvider == "" { ... }
if _, ok := c.Models.Providers[c.Models.ActiveProvider]; !ok { ... }
if len(c.Models.Phases) == 0 { ... }
for name, p := range c.Models.Providers { ... }
```

**Add:**
```go
if c.LLM.BaseURL == "" {
    return fmt.Errorf("llm.base_url is required")
}
```

### Step 4: Update LoadFromYAML / LoadFromJSON
These should work automatically since the struct tags match YAML keys.

## Files to Modify
- `internal/config/config.go`

## Files to Update After This Task
- `internal/config/default.go` (task 1.a)
- `internal/api/handlers/config.go` (task 1.i)
- `cmd/daemon/main.go` (task 1.h)
- `config.yaml` (task 1.m)

## Verification
- `go build ./internal/config/...` succeeds
- `go test ./internal/config/...` passes
- No references to `config.ModelsConfig`, `config.ProviderConfig`, `config.PhaseModelConfig` remain in this file
