# Task 1.1a: Update internal/config/default.go

## Goal
Update default.go to use `LLMConfig` instead of `ModelsConfig`, `ProviderConfig`, `PhaseModelConfig`.

## Implementation Steps

### Step 1: Update DefaultPhaseConfigs() → Remove entirely
This function is no longer needed. Delete it.

### Step 2: Update Default() function
Replace the `Models` section with `LLM`:

**Remove:**
```go
Models: ModelsConfig{
    ActiveProvider: "lm-studio",
    Providers: map[string]ProviderConfig{
        "lm-studio": {
            BaseURL: DefaultProviderURL,
        },
    },
    Phases: DefaultPhaseConfigs(),
},
```

**Add:**
```go
LLM: LLMConfig{
    BaseURL:     DefaultProviderURL,
    Model:       "",
    Temperature: 0.7,
    MaxTokens:   8192,
},
```

## Files to Modify
- `internal/config/default.go`

## Verification
- `go build ./internal/config/...` succeeds
- `go test ./internal/config/...` passes
