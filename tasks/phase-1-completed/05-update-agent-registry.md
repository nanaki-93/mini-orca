# Task 1.5: Update internal/agent/registry.go

## Goal
Update registry to use `*llm.Client` instead of `*model.Router`.

## Files to Modify
- `internal/agent/registry.go`

## Current State
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"

func (r *Registry) FromConfig(cfg model.AgentConfig, router *model.Router) error
func InitAgentRegistry(router *model.Router) *Registry
```

## Implementation Steps

### Step 1: Update import
**Remove:**
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"
```

**Add:**
```go
import "github.com/nanaki-93/mini-orca/v2/internal/llm"
```

### Step 2: Update FromConfig()
**Replace:**
```go
func (r *Registry) FromConfig(cfg model.AgentConfig, router *model.Router) error {
    a := NewClient(router)
    a.name = cfg.Name
    a.description = cfg.Description
    a.phase = cfg.Phase
    return r.Register(cfg.Name, a)
}
```

**With:**
```go
// FromConfig is deprecated - use InitAgentRegistry with *llm.Client directly
func (r *Registry) FromConfig(cfg model.AgentConfig, router *model.Router) error {
    return fmt.Errorf("FromConfig is deprecated, use InitAgentRegistry")
}
```

### Step 3: Update InitAgentRegistry()
**Replace:**
```go
func InitAgentRegistry(router *model.Router) *Registry {
    registry := NewRegistry()
    coder := NewCoderAgent(router, nil)
    if err := registry.Register(coder.Name(), coder); err != nil {
        logging.Warn("Failed to register coder agent", "error", err)
    }
    logging.Info("Agent registry initialized", "count", len(registry.List()))
    return registry
}
```

**With:**
```go
func InitAgentRegistry(llmClient *llm.Client) *Registry {
    registry := NewRegistry()
    coder := NewCoderAgent(llmClient, nil)
    if err := registry.Register(coder.Name(), coder); err != nil {
        logging.Warn("Failed to register coder agent", "error", err)
    }
    logging.Info("Agent registry initialized", "count", len(registry.List()))
    return registry
}
```

## Verification
- `go build ./internal/agent/...` succeeds (after tasks 1.3, 1.4, 1.6)
- No `model.` references remain in this file
