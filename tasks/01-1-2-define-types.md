# Task 1.1.2 — Define Types in types.go

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Define all core types in `internal/model/types.go`.

## Checklist
- [ ] Define `Provider` struct (name, base URL, config)
- [ ] Define `Model` struct (id, object, owned_by)
- [ ] Define `ModelConfig` struct (provider, model_id, temperature, max_tokens)
- [ ] Define `ChatRequest` struct (messages, model, temperature, stream)
- [ ] Define `ChatResponse` struct (choices, usage, model)
- [ ] Define `ChatMessage` struct (role, content)
- [ ] Define `PhaseConfig` struct (model config + phase-specific overrides)
- [ ] Define `Config` struct (top-level config with phases, providers, agents)
- [ ] Define `Router` struct (map of provider name → Provider, active config)

## Dependencies
- Task 1.1.1

## Deliverables
- `internal/model/types.go` with all type definitions

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
