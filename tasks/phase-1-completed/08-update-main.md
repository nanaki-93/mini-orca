# Task 1.8: Update cmd/daemon/main.go

## Goal
Remove model router initialization, use simple LLM client creation.

## Files to Modify
- `cmd/daemon/main.go`

## Current State (relevant parts)
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"

router, err := model.InitRouter(cfg)
...
agentRegistry := agent.InitAgentRegistry(router)
agentOrchestrator := agent.NewOrchestrator(router, skillsRegistry, executor)
coder := agent.NewCoderAgent(router, skillsRegistry)
tester := agent.NewTesterAgent(router, skillsRegistry)
reviewer := agent.NewReviewerAgent(router, skillsRegistry)
listClient := agent.NewClient(router)
models, err := listClient.ListModels()
...
logging.Info("Startup config", "active_provider", cfg.Models.ActiveProvider, "registered_providers", router.ListProviders(), "configured_phases", len(cfg.Models.Phases))
```

## Implementation Steps

### Step 1: Update imports
**Remove:**
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"
```

**Add:**
```go
import "github.com/nanaki-93/mini-orca/v2/internal/llm"
```

### Step 2: Replace router initialization with LLM client
**Remove:**
```go
router, err := model.InitRouter(cfg)
if err != nil {
    logging.Error("Failed to initialize router", "error", err)
    os.Exit(1)
}
```

**Add:**
```go
llmClient := llm.NewClient(
    cfg.LLM.BaseURL,
    cfg.LLM.APIKey,
    cfg.LLM.Model,
    cfg.LLM.Temperature,
    cfg.LLM.MaxTokens,
)
```

### Step 3: Update agent registry initialization
**Replace:**
```go
agentRegistry := agent.InitAgentRegistry(router)
```

**With:**
```go
agentRegistry := agent.InitAgentRegistry(llmClient)
```

### Step 4: Update orchestrator creation
**Replace:**
```go
agentOrchestrator := agent.NewOrchestrator(router, skillsRegistry, executor)
```

**With:**
```go
agentOrchestrator := agent.NewOrchestrator(llmClient, skillsRegistry, executor)
```

### Step 5: Update agent creation
**Replace:**
```go
coder := agent.NewCoderAgent(router, skillsRegistry)
tester := agent.NewTesterAgent(router, skillsRegistry)
reviewer := agent.NewReviewerAgent(router, skillsRegistry)
```

**With:**
```go
coder := agent.NewCoderAgent(llmClient, skillsRegistry)
tester := agent.NewTesterAgent(llmClient, skillsRegistry)
reviewer := agent.NewReviewerAgent(llmClient, skillsRegistry)
```

### Step 6: Update client creation and model listing
**Replace:**
```go
listClient := agent.NewClient(router)
models, err := listClient.ListModels()
if err != nil {
    logging.Warn("Failed to list models", "error", err)
} else {
    logging.Info("Available models", "count", len(models))
    for _, m := range models {
        logging.Debug("Model detail", "id", m.ID, "owned_by", m.OwnedBy)
    }
}
```

**With:**
```go
models, err := llmClient.ListModels(context.Background())
if err != nil {
    logging.Warn("Failed to list models", "error", err)
} else {
    logging.Info("Available models", "count", len(models))
    for _, m := range models {
        logging.Debug("Model detail", "id", m.ID, "owned_by", m.OwnedBy)
    }
}
```

### Step 7: Update startup logging
**Replace:**
```go
logging.Info("Startup config",
    "active_provider", cfg.Models.ActiveProvider,
    "registered_providers", router.ListProviders(),
    "configured_phases", len(cfg.Models.Phases))
```

**With:**
```go
logging.Info("Startup config",
    "base_url", cfg.LLM.BaseURL,
    "model", cfg.LLM.Model,
    "temperature", cfg.LLM.Temperature)
```

## Verification
- `go build ./cmd/daemon/...` succeeds
- No `model.` references remain in this file
