# Task 1.7: Update internal/agent/orchestrator.go

## Goal
Update orchestrator to use `*llm.Client` instead of `*model.Router`.

## Files to Modify
- `internal/agent/orchestrator.go`

## Current State
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"

type Orchestrator struct {
    router   *model.Router
    registry *skills.SkillsRegistry
    executor tools.ToolExecutor
}

func NewOrchestrator(router *model.Router, registry *skills.SkillsRegistry, executor tools.ToolExecutor) *Orchestrator
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

### Step 2: Update Orchestrator struct
**Replace:**
```go
type Orchestrator struct {
    router   *model.Router
    registry *skills.SkillsRegistry
    executor tools.ToolExecutor
}
```

**With:**
```go
type Orchestrator struct {
    llmClient  *llm.Client
    registry   *skills.SkillsRegistry
    executor   tools.ToolExecutor
}
```

### Step 3: Update NewOrchestrator()
**Replace:**
```go
func NewOrchestrator(router *model.Router, registry *skills.SkillsRegistry, executor tools.ToolExecutor) *Orchestrator {
    return &Orchestrator{
        router:   router,
        registry: registry,
        executor: executor,
    }
}
```

**With:**
```go
func NewOrchestrator(llmClient *llm.Client, registry *skills.SkillsRegistry, executor tools.ToolExecutor) *Orchestrator {
    return &Orchestrator{
        llmClient: llmClient,
        registry:  registry,
        executor:  executor,
    }
}
```

### Step 4: Update RunCoderFromPrompt()
**Replace:**
```go
agent := NewCoderAgent(o.router, o.registry)
```

**With:**
```go
agent := NewCoderAgent(o.llmClient, o.registry)
```

### Step 5: Update RunTester()
**Replace:**
```go
agent := NewTesterAgent(o.router, o.registry)
```

**With:**
```go
agent := NewTesterAgent(o.llmClient, o.registry)
```

### Step 6: Update RunReviewer()
**Replace:**
```go
agent := NewReviewerAgent(o.router, o.registry)
```

**With:**
```go
agent := NewReviewerAgent(o.llmClient, o.registry)
```

## Verification
- `go build ./internal/agent/...` succeeds
- No `model.` references remain in this file
