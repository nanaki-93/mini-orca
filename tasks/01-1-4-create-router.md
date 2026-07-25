# Task 1.1.4 — Create Router

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Implement the Router struct in `internal/model/router.go`.

## Checklist
- [ ] Define `NewRouter()` constructor
- [ ] Implement `RegisterProvider(name string, p Provider)` method
- [ ] Implement `GetProvider(name string) (Provider, error)` method
- [ ] Implement `GetPhaseConfig(phase string) (*PhaseConfig, error)` method
- [ ] Implement `Chat(phase string, messages []ChatMessage) (*ChatResponse, error)` — routing logic
- [ ] Add error handling for missing providers/phases

## Dependencies
- Task 1.1.2, Task 1.1.3

## Deliverables
- `internal/model/router.go` with Router implementation

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
