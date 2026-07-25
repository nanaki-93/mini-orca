# Task 1.4.2 — Update Agent Client

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Modify the agent client to use the model router instead of direct HTTP calls.

## Checklist
- [ ] Modify `internal/agent/client.go` to accept `*model.Router`
- [ ] Replace direct HTTP calls with `router.Chat(phase, messages)`
- [ ] Remove hardcoded model name
- [ ] Add phase parameter to `Generate()` or equivalent method

## Dependencies
- Task 1.1.4, Task 1.4.1

## Deliverables
- Updated agent client using model router

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
