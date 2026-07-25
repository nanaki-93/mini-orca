# Task 4.2.1 — Define Session Model

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Define the Session struct in the state package.

## Checklist
- [ ] Create `internal/state/session.go`
- [ ] Define `Session` struct:
  - `ID string` (UUID)
  - `Goal string`
  - `ProjectPath string`
  - `ProjectType string`
  - `CreatedAt time.Time`
  - `UpdatedAt time.Time`
  - `CurrentPhase Phase`
  - `Status SessionStatus`
  - `Plan *Plan`
  - `AtomicUnits []AtomicUnit`
  - `History []PhaseHistory`
  - `Error string`

## Dependencies
- Task 4.1.1

## Deliverables
- `internal/state/session.go` with Session struct

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
