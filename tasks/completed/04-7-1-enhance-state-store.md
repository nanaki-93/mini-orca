# Task 4.7.1 — Enhance State Store

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Update the state store with new model support.

## Checklist
- [ ] Update `internal/state/store.go` for new models
- [ ] Implement `SaveSession(session *Session) error`
- [ ] Implement `GetSession(id string) (*Session, error)`
- [ ] Implement `SavePlan(plan *Plan) error`
- [ ] Implement `GetPlan(sessionID string) (*Plan, error)`
- [ ] Implement `UpdateUnitStatus(unitID string, status UnitStatus) error`
- [ ] Implement `GetPendingUnits(planID string) ([]PlanUnit, error)`
- [ ] Implement `SavePhaseHistory(history *PhaseHistory) error`
- [ ] Implement `GetSessionHistory(sessionID string) ([]PhaseHistory, error)`

## Dependencies
- Task 4.2.1, Task 4.2.2, Task 4.2.3, Task 4.2.4

## Deliverables
- Enhanced state store with all CRUD operations

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
