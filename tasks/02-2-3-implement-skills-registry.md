# Task 2.2.3 — Implement Skills Registry

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Implement the skills registry in `internal/agent/skills/registry.go`.

## Checklist
- [ ] Create `internal/agent/skills/registry.go`
- [ ] Define `SkillsRegistry` struct (map of name → Skill)
- [ ] Implement `Register(skill Skill)`
- [ ] Implement `Get(name string) (*Skill, error)`
- [ ] Implement `GetByCategory(category SkillCategory) []Skill`
- [ ] Implement `GetForAgent(agentName string) []Skill`
- [ ] Pre-register all skills from library

## Dependencies
- Task 2.2.2

## Deliverables
- `internal/agent/skills/registry.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
