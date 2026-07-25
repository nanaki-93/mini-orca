# Task 1.3.2 — Define Config Schema

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Define all config structs in `internal/config/config.go`.

## Checklist
- [ ] Define `Config` struct with fields: Models, Agents, Skills
- [ ] Define `ModelsConfig` with: ActiveProvider, Providers map, Phases map
- [ ] Define `ProviderConfig` with: BaseURL, APIKey
- [ ] Define `PhaseModelConfig` with: Provider, Model, Temperature, MaxTokens
- [ ] Define `AgentsConfig` with: Planner, Coder, Tester, Reviewer AgentConfigs
- [ ] Define `AgentConfig` with: Skills, Model (optional override)
- [ ] Define `SkillsConfig` with: Knowledge map, Tools map

## Dependencies
- Task 1.1.2

## Deliverables
- `internal/config/config.go` with all config struct definitions

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
