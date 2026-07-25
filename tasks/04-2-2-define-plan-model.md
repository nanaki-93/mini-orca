# Task 4.2.2 — Define Plan Model

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Define the Plan struct.

## Checklist
- [ ] Define `Plan` struct:
  - `ID string`
  - `SessionID string`
  - `GeneratedAt time.Time`
  - `GeneratedBy string` (agent name)
  - `Units []PlanUnit`
  - `Status PlanStatus`
  - `ApprovedAt time.Time`
  - `ApprovedBy string`
  - `RejectionReason string`

## Dependencies
- Task 4.2.1

## Deliverables
- Plan struct definition

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
