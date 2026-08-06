# Task 1.9: Update internal/api/handlers/config.go

## Goal
Remove model/provider-specific endpoints and types.

## Files to Modify
- `internal/api/handlers/config.go`

## Implementation Steps

### Step 1: Remove ModelListResponse type
**Delete:**
```go
type ModelListResponse struct {
    ActiveProvider string                      `json:"active_provider"`
    Providers      map[string]config.ProviderConfig `json:"providers"`
    Phases         map[string]config.PhaseModelConfig `json:"phases"`
}
```

### Step 2: Remove PhaseConfigResponse type
**Delete:**
```go
type PhaseConfigResponse struct {
    Phases map[string]config.PhaseModelConfig `json:"phases"`
}
```

### Step 3: Remove ListModels() handler
**Delete:**
```go
func (h *ConfigHandler) ListModels(w http.ResponseWriter, r *http.Request) { ... }
```

### Step 4: Remove GetPhaseConfigs() handler
**Delete:**
```go
func (h *ConfigHandler) GetPhaseConfigs(w http.ResponseWriter, r *http.Request) { ... }
```

### Step 5: Update ConfigUpdateRequest
**Replace:**
```go
type ConfigUpdateRequest struct {
    Models *config.ModelsConfig `json:"models,omitempty"`
    Agents *config.AgentsConfig `json:"agents,omitempty"`
    Skills *config.SkillsConfig `json:"skills,omitempty"`
    Retry  *config.RetryConfig  `json:"retry,omitempty"`
}
```

**With:**
```go
type ConfigUpdateRequest struct {
    LLM    *config.LLMConfig    `json:"llm,omitempty"`
    Agents *config.AgentsConfig `json:"agents,omitempty"`
    Skills *config.SkillsConfig `json:"skills,omitempty"`
    Retry  *config.RetryConfig  `json:"retry,omitempty"`
}
```

### Step 6: Update ConfigResponse
**Replace:**
```go
type ConfigResponse struct {
    Models config.ModelsConfig `json:"models"`
    Agents config.AgentsConfig `json:"agents"`
    Skills config.SkillsConfig `json:"skills"`
    Retry  config.RetryConfig  `json:"retry"`
}
```

**With:**
```go
type ConfigResponse struct {
    LLM    config.LLMConfig    `json:"llm"`
    Agents config.AgentsConfig `json:"agents"`
    Skills config.SkillsConfig `json:"skills"`
    Retry  config.RetryConfig  `json:"retry"`
}
```

### Step 7: Update GetConfig() handler
**Replace:**
```go
api.WriteJSON(w, http.StatusOK, ConfigResponse{
    Models: cfg.Models,
    Agents: cfg.Agents,
    Skills: cfg.Skills,
    Retry:  cfg.Retry,
})
```

**With:**
```go
api.WriteJSON(w, http.StatusOK, ConfigResponse{
    LLM:    cfg.LLM,
    Agents: cfg.Agents,
    Skills: cfg.Skills,
    Retry:  cfg.Retry,
})
```

### Step 8: Remove updateModels() method
**Delete:**
```go
func (s *ConfigStore) updateModels(models *config.ModelsConfig) error { ... }
```

### Step 9: Update applyUpdates()
**Replace:**
```go
if req.Models != nil {
    if err := s.updateModels(req.Models); err != nil {
        return err
    }
}
```

**With:**
```go
if req.LLM != nil {
    if err := s.updateLLM(req.LLM); err != nil {
        return err
    }
}
```

### Step 10: Add updateLLM() method
**Add:**
```go
func (s *ConfigStore) updateLLM(llm *config.LLMConfig) error {
    if llm.BaseURL != "" {
        s.cfg.LLM.BaseURL = llm.BaseURL
    }
    if llm.APIKey != "" {
        s.cfg.LLM.APIKey = llm.APIKey
    }
    if llm.Model != "" {
        s.cfg.LLM.Model = llm.Model
    }
    if llm.Temperature > 0 {
        s.cfg.LLM.Temperature = llm.Temperature
    }
    if llm.MaxTokens > 0 {
        s.cfg.LLM.MaxTokens = llm.MaxTokens
    }
    return nil
}
```

### Step 11: Update copyConfig() — remove model-related deep copies
**Remove from copyConfig():**
```go
cfgCopy.Models.Providers = make(...)
for k, v := range cfg.Models.Providers { ... }
cfgCopy.Models.Phases = make(...)
for k, v := range cfg.Models.Phases { ... }
```

**Add:**
```go
// LLM is a value type, no deep copy needed
```

## Verification
- `go build ./internal/api/handlers/...` succeeds
- No `config.ModelsConfig`, `config.ProviderConfig`, `config.PhaseModelConfig` references remain
