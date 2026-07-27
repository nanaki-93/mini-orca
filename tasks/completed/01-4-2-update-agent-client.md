# Task 1.4.2 — Update Agent Client

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Modify the agent client to use the model router instead of direct HTTP calls.

## Checklist
- [x] Modify `internal/agent/client.go` to accept `*model.Router`
- [x] Replace direct HTTP calls with `router.Chat(phase, messages)`
- [x] Remove hardcoded model name
- [x] Add phase parameter to `Generate()` method

## Dependencies
- Task 1.1.4, Task 1.4.1

## Deliverables
- Updated agent client using model router

## Notes
No existing `internal/agent/client.go` was found in the codebase. Created a new agent client package that:
- Accepts `*model.Router` via dependency injection (no direct HTTP calls)
- `Generate(phase, messages)` routes through `router.Chat()` with phase-based config
- `ListModels()` routes through `router.RouteListModels()`
- Validates inputs (nil router, empty phase, empty messages)

## Status
- [x] Done
