# Task 1.1.2 — Define Types in types.go

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Define all core types in `internal/model/types.go`.

## Checklist
- [x] Define `Provider` struct (name, base URL, config)
- [x] Define `Model` struct (id, object, owned_by)
- [x] Define `ModelConfig` struct (provider, model_id, temperature, max_tokens)
- [x] Define `ChatRequest` struct (messages, model, temperature, stream)
- [x] Define `ChatResponse` struct (choices, usage, model)
- [x] Define `ChatMessage` struct (role, content)
- [x] Define `PhaseConfig` struct (model config + phase-specific overrides)
- [x] Define `Config` struct (top-level config with phases, providers, agents)
- [x] Define `Router` struct (map of provider name → Provider, active config)

## Dependencies
- Task 1.1.1

## Deliverables
- `internal/model/types.go` with all type definitions

## Status
- [x] Done
