# Task 4.1.2 — Define Orchestrator Struct

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Define the main Orchestrator struct.

## Checklist
- [ ] Create `internal/orchestrator/orchestrator.go`
- [ ] Define `Orchestrator` struct with:
  - `agents map[string]agent.Agent`
  - `router *model.Router`
  - `executor tools.ToolExecutor`
  - `stateStore *state.Store`
  - `config *config.Config`
  - `currentPhase Phase`
  - `currentSession *Session`
  - `ctx context.Context`
  - `cancel context.CancelFunc`
  - `humanGate *HumanGate`

## Dependencies
- Task 4.1.1, Task 4.2.1

## Deliverables
- `internal/orchestrator/orchestrator.go` with Orchestrator struct

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
