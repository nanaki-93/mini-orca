# Task 2.4.3 — Create Coder Prompt Template

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Create the coder prompt template.

## Checklist
- [ ] Create `internal/agent/prompts/coder.go`
- [ ] Define `BuildCoderPrompt(unit PlanUnit, existingCode string, skills []string) ([]ChatMessage, error)`
- [ ] System message includes role, skill templates, output format, constraints
- [ ] User message includes unit description, existing file content, dependencies

## Dependencies
- Task 2.4.2

## Deliverables
- `internal/agent/prompts/coder.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
