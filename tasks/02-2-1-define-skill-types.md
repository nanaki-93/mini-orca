# Task 2.2.1 — Define Skill Types

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Define skill types and structures in `internal/agent/skills/skills.go`.

## Checklist
- [ ] Create `internal/agent/skills/` directory
- [ ] Define `SkillType` enum: `Knowledge`, `Tool`
- [ ] Define `Skill` struct:
  - `Name string`
  - `Type SkillType`
  - `Description string`
  - `PromptTemplate string`
  - `Parameters map[string]string`
- [ ] Define `SkillCategory` enum: `Design`, `Coding`, `Testing`, `Review`, `Principles`

## Dependencies
- Task 2.1.2

## Deliverables
- `internal/agent/skills/skills.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
