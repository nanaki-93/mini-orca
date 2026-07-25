# Task 4.4.1 — Implement Human Gate

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Implement the human gate for approval workflows.

## Checklist
- [ ] Create `internal/orchestrator/human_gate.go`
- [ ] Define `HumanGate` struct:
  - `sessionID string`
  - `phase Phase`
  - `output string`
  - `approved bool`
  - `feedback string`
  - `responseChan chan GateResponse`
- [ ] Define `GateResponse` struct:
  - `Action string` (approve, reject, edit)
  - `Feedback string`
  - `Timestamp time.Time`
- [ ] Implement `RequestApproval(output string) (*GateResponse, error)`
- [ ] Implement `Respond(action string, feedback string) error`
- [ ] Implement `Timeout(timeout time.Duration) error`

## Dependencies
- Task 4.2.5

## Deliverables
- `internal/orchestrator/human_gate.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
