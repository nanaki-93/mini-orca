# Task 2.1.2 — Define Agent Interface

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Define the Agent interface and AgentResult struct.

## Checklist
- [ ] Define `Agent` interface with:
  - `Name() string`
  - `Execute(ctx context.Context, input string) (*AgentResult, error)`
  - `GetSkills() []string`
  - `SetSkills(skills []string)`
- [ ] Define `AgentResult` struct:
  - `Output string`
  - `Metadata map[string]string`
  - `Phase string`

## Dependencies
- Task 2.1.1

## Deliverables
- `internal/agent/agent.go` with interface and types

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
