# Task 1.4.3 — Update Main/Daemon Entry Point

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Update the daemon to initialize the router and pass it to agents.

## Checklist
- [x] In `cmd/daemon/main.go`, load config at startup
- [x] Create Router from config
- [x] Register LM Studio provider
- [x] Pass Router to agent initialization
- [x] Handle startup errors gracefully

## Dependencies
- Task 1.2.1, Task 1.4.2, Task 1.3.3

## Deliverables
- Updated daemon entry point with router initialization

## Notes
No existing `cmd/daemon/main.go` was found. Created a new daemon entry point that:
- Loads config from `config.yaml` or `$MINI_ORCA_CONFIG` env var, falls back to defaults
- Validates configuration before proceeding
- Initializes `model.Router` and registers all configured providers
- Sets phase-specific model configurations via `router.SetDefaultConfig()`
- Creates `agent.Client` with the router (dependency injection)
- Lists available models at startup (non-fatal if unavailable)
- Handles SIGINT/SIGTERM gracefully with clean shutdown

## Status
- [x] Done
