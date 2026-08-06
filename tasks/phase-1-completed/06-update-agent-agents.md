# Task 1.6: Update coder.go, tester.go, reviewer.go

## Goal
Update all three agent files to use `*llm.Client` instead of `*model.Router`.

## Files to Modify
- `internal/agent/coder.go`
- `internal/agent/tester.go`
- `internal/agent/reviewer.go`

---

## Changes for coder.go

### Step 1: Update import
**Remove:**
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"
```

**Add:**
```go
import "github.com/nanaki-93/mini-orca/v2/internal/llm"
```

### Step 2: Update NewCoderAgent()
**Replace:**
```go
func NewCoderAgent(router *model.Router, registry *skills.SkillsRegistry) *CoderAgent {
    client := NewClient(router)
    client.name = "coder"
    client.description = "Implements code based on plans and specifications"
    client.phase = model.PhaseCoding
    ...
}
```

**With:**
```go
func NewCoderAgent(llmClient *llm.Client, registry *skills.SkillsRegistry) *CoderAgent {
    client := NewClient(llmClient)
    client.name = "coder"
    client.description = "Implements code based on plans and specifications"
    client.phase = "coding"
    ...
}
```

### Step 3: Update Execute() — router references
**Replace:**
```go
if c.router == nil {
    return nil, fmt.Errorf("coder agent: router not configured")
}
...
resp, err := c.router.Chat(string(c.phase), messages)
```

**With:**
```go
if c.llmClient == nil {
    return nil, fmt.Errorf("coder agent: LLM client not configured")
}
...
resp, err := c.llmClient.Chat(ctx, messages)
```

### Step 4: Update ChatMessage type
**Replace:**
```go
messages := []model.ChatMessage{
    {Role: "user", Content: fullPrompt},
}
```

**With:**
```go
messages := []llm.ChatMessage{
    {Role: "user", Content: fullPrompt},
}
```

---

## Changes for tester.go

### Step 1: Same import change as coder.go
**Remove:** `model` import, **Add:** `llm` import

### Step 2: Update NewTesterAgent()
**Replace:**
```go
func NewTesterAgent(router *model.Router, registry *skills.SkillsRegistry) *TesterAgent {
    client := NewClient(router)
    client.name = "tester"
    client.description = "Tests code and validates functionality against requirements"
    client.phase = model.PhaseTesting
    ...
}
```

**With:**
```go
func NewTesterAgent(llmClient *llm.Client, registry *skills.SkillsRegistry) *TesterAgent {
    client := NewClient(llmClient)
    client.name = "tester"
    client.description = "Tests code and validates functionality against requirements"
    client.phase = "testing"
    ...
}
```

### Step 3: Update Execute()
**Replace:**
```go
if t.router == nil { ... }
messages := []model.ChatMessage{...}
resp, err := t.router.Chat(string(t.phase), messages)
```

**With:**
```go
if t.llmClient == nil { ... }
messages := []llm.ChatMessage{...}
resp, err := t.llmClient.Chat(ctx, messages)
```

---

## Changes for reviewer.go

### Step 1: Same import change as coder.go
**Remove:** `model` import, **Add:** `llm` import

### Step 2: Update NewReviewerAgent()
**Replace:**
```go
func NewReviewerAgent(router *model.Router, registry *skills.SkillsRegistry) *ReviewerAgent {
    client := NewClient(router)
    client.name = "reviewer"
    client.description = "Reviews code for quality, security, and adherence to standards"
    client.phase = model.PhaseReview
    ...
}
```

**With:**
```go
func NewReviewerAgent(llmClient *llm.Client, registry *skills.SkillsRegistry) *ReviewerAgent {
    client := NewClient(llmClient)
    client.name = "reviewer"
    client.description = "Reviews code for quality, security, and adherence to standards"
    client.phase = "review"
    ...
}
```

### Step 3: Update Execute()
**Replace:**
```go
if r.router == nil { ... }
messages := []model.ChatMessage{...}
resp, err := r.router.Chat(string(r.phase), messages)
```

**With:**
```go
if r.llmClient == nil { ... }
messages := []llm.ChatMessage{...}
resp, err := r.llmClient.Chat(ctx, messages)
```

---

## Verification
- `go build ./internal/agent/...` succeeds
- No `model.` references remain in any of the three files
