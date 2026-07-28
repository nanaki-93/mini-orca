# Task 2.7.2 — Create Agent Orchestration Helper

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Create helper functions to run each agent.

## Checklist
- [ ] Create `internal/agent/orchestrator.go`
- [ ] Implement `RunPlanner(goal string) (*AgentResult, error)`
- [ ] Implement `RunCoder(unit PlanUnit) (*AgentResult, error)`
- [ ] Implement `RunTester(code string, testResults string) (*AgentResult, error)`
- [ ] Implement `RunReviewer(code string, plan string) (*AgentResult, error)`

## Dependencies
- Task 2.3.2, Task 2.4.2, Task 2.5.2, Task 2.6.2

## Deliverables
- `internal/agent/orchestrator.go` with helper functions

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
