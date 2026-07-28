# Task 2.6.3 — Create Reviewer Prompt Template

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Create the reviewer prompt template.

## Checklist
- [ ] Create `internal/agent/prompts/reviewer.go`
- [ ] Define `BuildReviewerPrompt(code string, plan string, skills []string) ([]ChatMessage, error)`
- [ ] System message includes role, skill templates, review checklist
- [ ] User message includes code, original plan/spec, previous feedback

## Dependencies
- Task 2.6.2

## Deliverables
- `internal/agent/prompts/reviewer.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
