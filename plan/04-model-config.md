# Mini-Orca v2.0 — Model Configuration System

## 1. Overview

The model configuration system uses **LM Studio as the primary provider** but is built with an interface-first design, making it trivial to add Ollama, OpenAI, Anthropic, or any other OpenAI-compatible provider later.

**Key Design Principle:** Start simple (LM Studio only), but architect for extension (interface-based).

---

## 2. Provider Interface (Extensible)

```go
// internal/model/provider.go

// ChatMessage represents a single message in a conversation
type ChatMessage struct {
    Role    string `json:"role"`    // "system", "user", "assistant"
    Content string `json:"content"`
}

// ChatRequest is the standard request format for all providers
type ChatRequest struct {
    Model       string        `json:"model"`
    Messages    []ChatMessage `json:"messages"`
    Temperature float64       `json:"temperature"`
    MaxTokens   int           `json:"max_tokens"`
    TopP        float64       `json:"top_p,omitempty"`
    Stream      bool          `json:"stream,omitempty"`
}

// ChatResponse is the standard response format for all providers
type ChatResponse struct {
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

type Choice struct {
    Message   ChatMessage `json:"message"`
    Index     int         `json:"index"`
    FinishReason string    `json:"finish_reason"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// Provider is the interface ALL providers must implement.
// This is the key abstraction that makes adding new providers easy.
type Provider interface {
    // Chat sends a chat request and returns the response
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    
    // ListModels returns available models for this provider
    ListModels(ctx context.Context) ([]string, error)
    
    // Name returns the provider name (e.g., "lm-studio", "ollama", "openai")
    Name() string
}
```

---

## 3. Model Configuration

```go
// internal/model/config.go

type ModelConfig struct {
    Provider    string        `json:"provider"`    // "lm-studio" (only one for now)
    Model       string        `json:"model"`       // e.g., "qwen/qwen3-coder-30b"
    Temperature float64       `json:"temperature"` // 0.0 - 1.0
    MaxTokens   int           `json:"max_tokens"`  // Max response tokens
    TopP        float64       `json:"top_p"`       // Nucleus sampling
    Timeout     time.Duration `json:"timeout"`     // Request timeout
}

// PhaseConfigs holds the model configuration for each phase
type PhaseConfigs struct {
    Planning  ModelConfig `json:"planning"`
    Coding    ModelConfig `json:"coding"`
    Testing   ModelConfig `json:"testing"`
    Review    ModelConfig `json:"review"`
}

// DefaultPhaseConfigs returns sensible defaults (all using LM Studio)
func DefaultPhaseConfigs() PhaseConfigs {
    return PhaseConfigs{
        Planning: ModelConfig{
            Provider:    "lm-studio",
            Model:       "qwen/qwen3-coder-30b",
            Temperature: 0.3,
            MaxTokens:   4096,
            TopP:        0.9,
            Timeout:     120 * time.Second,
        },
        Coding: ModelConfig{
            Provider:    "lm-studio",
            Model:       "qwen/qwen3-coder-30b",
            Temperature: 0.1,  // Low for deterministic code
            MaxTokens:   8192,
            TopP:        0.9,
            Timeout:     180 * time.Second,
        },
        Testing: ModelConfig{
            Provider:    "lm-studio",
            Model:       "qwen/qwen3-coder-30b",
            Temperature: 0.2,
            MaxTokens:   2048,
            TopP:        0.9,
            Timeout:     60 * time.Second,
        },
        Review: ModelConfig{
            Provider:    "lm-studio",
            Model:       "qwen/qwen3-coder-30b",
            Temperature: 0.2,
            MaxTokens:   4096,
            TopP:        0.9,
            Timeout:     120 * time.Second,
        },
    }
}
```

---

## 4. LM Studio Provider (Primary Implementation)

```go
// internal/model/lm_studio.go

// LMStudioProvider implements the Provider interface for LM Studio.
// LM Studio exposes an OpenAI-compatible API at localhost:1234
type LMStudioProvider struct {
    BaseURL string
    Client  *http.Client
}

// NewLMStudioProvider creates a new LM Studio provider instance
func NewLMStudioProvider(baseURL string) *LMStudioProvider {
    return &LMStudioProvider{
        BaseURL: baseURL,
        Client: &http.Client{
            Timeout: 300 * time.Second,
        },
    }
}

func (p *LMStudioProvider) Name() string {
    return "lm-studio"
}

// ListModels queries LM Studio for available models
func (p *LMStudioProvider) ListModels(ctx context.Context) ([]string, error) {
    resp, err := p.Client.Get(p.BaseURL + "/v1/models")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result struct {
        Data []struct {
            ID string `json:"id"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    models := make([]string, len(result.Data))
    for i, m := range result.Data {
        models[i] = m.ID
    }
    return models, nil
}

// Chat sends a request to LM Studio and returns the response.
// LM Studio uses the OpenAI-compatible /v1/chat/completions endpoint.
func (p *LMStudioProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Build the request payload (OpenAI-compatible format)
    openaiReq := map[string]interface{}{
        "model":       req.Model,
        "messages":    req.Messages,
        "temperature": req.Temperature,
        "max_tokens":  req.MaxTokens,
        "top_p":       req.TopP,
    }
    
    jsonBody, err := json.Marshal(openaiReq)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }
    
    resp, err := p.Client.Post(
        p.BaseURL+"/v1/chat/completions",
        "application/json",
        bytes.NewBuffer(jsonBody),
    )
    if err != nil {
        return nil, fmt.Errorf("LM Studio is unreachable at %s — is the server running? %w", p.BaseURL, err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("LM Studio returned status: %d", resp.StatusCode)
    }
    
    var chatResp ChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }
    
    return &chatResp, nil
}
```

---

## 5. Future Provider Implementations (Interface Ready)

These are **NOT implemented yet** — just the interface contracts. Adding them is trivial:

### 5.1 Ollama (Future)

```go
// internal/model/ollama.go — FUTURE IMPLEMENTATION

type OllamaProvider struct {
    BaseURL string
    Client  *http.Client
}

func (p *OllamaProvider) Name() string { return "ollama" }

// ListModels queries Ollama's /api/tags endpoint
func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
    // Implementation: GET {baseURL}/api/tags
    // Return list of model names
}

// Chat sends a request to Ollama's /api/chat endpoint
func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Implementation: POST {baseURL}/api/chat
    // Convert Ollama response to standard ChatResponse
}
```

### 5.2 OpenAI (Future)

```go
// internal/model/openai.go — FUTURE IMPLEMENTATION

type OpenAIProvider struct {
    APIKey  string
    BaseURL string
    Client  *http.Client
}

func (p *OpenAIProvider) Name() string { return "openai" }

// ListModels queries OpenAI's /v1/models endpoint
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
    // Implementation: GET https://api.openai.com/v1/models
    // With Bearer token auth
}

// Chat sends a request to OpenAI's /v1/chat/completions endpoint
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Implementation: POST https://api.openai.com/v1/chat/completions
    // With Authorization: Bearer {API_KEY}
}
```

### 5.3 Anthropic (Future)

```go
// internal/model/anthropic.go — FUTURE IMPLEMENTATION

type AnthropicProvider struct {
    APIKey string
    Client *http.Client
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

// ListModels returns available Claude models
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]string, error) {
    return []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}, nil
}

// Chat sends a request to Anthropic's /v1/messages endpoint
func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Implementation: POST https://api.anthropic.com/v1/messages
    // Convert Anthropic format to standard ChatResponse
}
```

---

## 6. Model Router

```go
// internal/model/router.go

type Router struct {
    providers map[string]Provider  // providerName -> Provider instance
    configs   PhaseConfigs
}

// NewRouter creates a router with LM Studio as the only provider
func NewRouter(configs PhaseConfigs) *Router {
    r := &Router{
        providers: make(map[string]Provider),
        configs:   configs,
    }
    
    // Register LM Studio (primary provider)
    lmStudioURL := "http://127.0.0.1:1234"
    if url := os.Getenv("LM_STUDIO_URL"); url != "" {
        lmStudioURL = url
    }
    r.providers["lm-studio"] = NewLMStudioProvider(lmStudioURL)
    
    // Future: Register additional providers when they're implemented
    // if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
    //     r.providers["openai"] = NewOpenAIProvider(apiKey)
    // }
    
    return r
}

// GetProvider returns the provider for a given config
func (r *Router) GetProvider(config ModelConfig) (Provider, error) {
    provider, exists := r.providers[config.Provider]
    if !exists {
        return nil, fmt.Errorf("provider not found: %s", config.Provider)
    }
    return provider, nil
}

// ExecutePhase runs an LLM call with the phase-specific configuration
func (r *Router) ExecutePhase(ctx context.Context, phase Phase, messages []ChatMessage) (*ChatResponse, error) {
    var config ModelConfig
    switch phase {
    case PhasePlanning:
        config = r.configs.Planning
    case PhaseCoding:
        config = r.configs.Coding
    case PhaseTesting:
        config = r.configs.Testing
    case PhaseReview:
        config = r.configs.Review
    default:
        return nil, fmt.Errorf("unknown phase: %s", phase)
    }
    
    provider, err := r.GetProvider(config)
    if err != nil {
        return nil, err
    }
    
    req := ChatRequest{
        Model:       config.Model,
        Messages:    messages,
        Temperature: config.Temperature,
        MaxTokens:   config.MaxTokens,
        TopP:        config.TopP,
    }
    
    return provider.Chat(ctx, req)
}

// ListAvailableModels returns all available models from LM Studio
func (r *Router) ListAvailableModels(ctx context.Context) (map[string][]string, error) {
    result := make(map[string][]string)
    
    for name, provider := range r.providers {
        models, err := provider.ListModels(ctx)
        if err != nil {
            log.Printf("Warning: failed to list models for %s: %v", name, err)
            continue
        }
        result[name] = models
    }
    
    return result, nil
}
```

---

## 7. How to Add a New Provider

Adding a new provider requires just 3 steps:

### Step 1: Create the provider struct

```go
// internal/model/new_provider.go

type NewProvider struct {
    // fields...
}

func (p *NewProvider) Name() string { return "new-provider" }
func (p *NewProvider) ListModels(ctx context.Context) ([]string, error) {
    // Implementation
}
func (p *NewProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Implementation
}
```

### Step 2: Register it in the router

```go
// In NewRouter():
r.providers["new-provider"] = NewNewProvider(...)
```

### Step 3: Add config to YAML

```yaml
providers:
  new-provider:
    base_url: "..."
```

That's it. The rest of the system (agents, orchestrator, frontend) works unchanged.

---

## 8. LM Studio Setup Guide

```bash
# 1. Install LM Studio
# Download from: https://lmstudio.ai

# 2. Download a model
# - Open LM Studio
# - Search for a model (e.g., "Qwen/Qwen2.5-Coder-32B-Instruct")
# - Click download

# 3. Start the local server
# - Go to the server tab (🔌 icon)
# - Select the downloaded model
# - Click "Start Server"
# - Default URL: http://localhost:1234

# 4. Verify it works
curl http://localhost:1234/v1/models

# 5. Configure mini-orca
# Edit config.yaml to match your model name
```

---

## 9. Environment Variables

```bash
# LM Studio URL (optional, defaults to http://127.0.0.1:1234)
export LM_STUDIO_URL=http://localhost:1234

# Future: OpenAI API Key (when implemented)
export OPENAI_API_KEY=sk-...

# Future: Anthropic API Key (when implemented)
export ANTHROPIC_API_KEY=sk-ant-...
```
