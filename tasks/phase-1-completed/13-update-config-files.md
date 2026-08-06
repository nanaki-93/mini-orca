# Task 1.13: Update config.yaml and config.example.yaml

## Goal
Simplify config structure from nested models/providers/phases to flat llm section.

## Files to Modify
- `config.yaml`
- `config.example.yaml`

---

## Current Structure
```yaml
models:
  active_provider: "lm-studio"
  providers:
    lm-studio:
      base_url: "http://localhost:1234"
      api_key: ""
  phases:
    coding:
      provider: "lm-studio"
      model: ""
      temperature: 0.7
      max_tokens: 8192
    testing:
      provider: "lm-studio"
      model: ""
      temperature: 0.5
      max_tokens: 4096
    review:
      provider: "lm-studio"
      model: ""
      temperature: 0.3
      max_tokens: 4096
    human_review:
      provider: "lm-studio"
      model: ""
      temperature: 0.0
      max_tokens: 4096
```

## New Structure
```yaml
llm:
  base_url: "http://localhost:1234"
  api_key: ""
  model: ""
  temperature: 0.7
  max_tokens: 8192
```

## Implementation Steps

### Step 1: Open config.yaml
Replace the entire `models:` section with the new `llm:` section above.

### Step 2: Open config.example.yaml
Replace the entire `models:` section with the new `llm:` section above.

### Step 3: Keep everything else unchanged
The following sections remain the same:
- `agents:`
- `skills:`
- `retry:`
- `logging:`

## Verification
- YAML is valid (check with `yq` or similar)
- `go build ./...` succeeds (after config.go is updated)
- Application starts with new config format
