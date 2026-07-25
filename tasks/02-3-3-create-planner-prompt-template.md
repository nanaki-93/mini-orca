# Task 2.3.3 — Create Planner Prompt Template

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Create the planner prompt template in the prompts package.

## Checklist
- [ ] Create `internal/agent/prompts/` directory
- [ ] Create `internal/agent/prompts/planner.go`
- [ ] Define `BuildPlannerPrompt(goal string, skills []string, context string) ([]ChatMessage, error)`
- [ ] System message includes role, skill templates, output format, constraints
- [ ] User message includes goal, project context, constraints

## Dependencies
- Task 2.3.2

## Deliverables
- `internal/agent/prompts/planner.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
