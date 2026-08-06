# Task 1.4: Update internal/agent/client.go

## Goal
Replace `*model.Router` with `*llm.Client` in the agent Client struct.

## Files to Modify
- `internal/agent/client.go`

## Current State
```go
import "github.com/nanaki-93/mini-orca/v2/internal/model"

type Client struct {
    router      *model.Router
    name        string
    description string
    phase       model.Phase
    skills      []string
}

func NewClient(router *model.Router) *Client
func (c *Client) Execute(ctx context.Context, input string) (*Result, error)
func (c *Client) ListModels() ([]model.Model, error)
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

### Step 2: Update Client struct
**Replace:**
```go
type Client struct {
    router      *model.Router
    name        string
    description string
    phase       model.Phase
    skills      []string
}
```

**With:**
```go
type Client struct {
    llmClient   *llm.Client
    name        string
    description string
    phase       string  // "coding", "testing", "review"
    skills      []string
}
```

### Step 3: Update NewClient()
**Replace:**
```go
func NewClient(router *model.Router) *Client {
    return &Client{
        router: router,
        skills: make([]string, 0),
    }
}
```

**With:**
```go
func NewClient(llmClient *llm.Client) *Client {
    return &Client{
        llmClient: llmClient,
        skills:    make([]string, 0),
    }
}
```

### Step 4: Update Execute()
**Replace:**
```go
resp, err := c.router.Chat(string(c.phase), messages)
```

**With:**
```go
resp, err := c.llmClient.Chat(ctx, messages)
```

### Step 5: Update ListModels()
**Replace:**
```go
return c.router.RouteListModels(context.Background())
```

**With:**
```go
return c.llmClient.ListModels(context.Background())
```

### Step 6: Update router references in Execute()
**Replace:**
```go
if c.router == nil {
    return nil, fmt.Errorf("agent client: router not configured")
}
```

**With:**
```go
if c.llmClient == nil {
    return nil, fmt.Errorf("agent client: LLM client not configured")
}
```

## Verification
- `go build ./internal/agent/...` succeeds (after tasks 1.3, 1.5, 1.6 are done)
- No `model.` references remain in this file
