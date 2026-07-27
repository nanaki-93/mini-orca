# Task 1.3.2 — Define Config Schema

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Define all config structs in `internal/config/config.go`.

## Checklist
- [x] Define `Config` struct with fields: Models, Agents, Skills
- [x] Define `ModelsConfig` with: ActiveProvider, Providers map, Phases map
- [x] Define `ProviderConfig` with: BaseURL, APIKey
- [x] Define `PhaseModelConfig` with: Provider, Model, Temperature, MaxTokens
- [x] Define `AgentsConfig` with: Planner, Coder, Tester, Reviewer AgentConfigs
- [x] Define `AgentConfig` with: Skills, Model (optional override)
- [x] Define `SkillsConfig` with: Knowledge map, Tools map

## Dependencies
- Task 1.1.2

## Deliverables
- `internal/config/config.go` with all config struct definitions

## Status
- [x] Done
