# Task 2.5.3 — Create Tester Prompt Template

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Create the tester prompt template.

## Checklist
- [ ] Create `internal/agent/prompts/tester.go`
- [ ] Define `BuildTesterPrompt(code string, testResults string, skills []string) ([]ChatMessage, error)`
- [ ] System message includes role, skill templates, analysis framework
- [ ] User message includes code, test output, coverage report

## Dependencies
- Task 2.5.2

## Deliverables
- `internal/agent/prompts/tester.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
