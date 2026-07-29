# Task 4.6.1 — Implement Retry Configuration

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Add retry configuration to the config system.

## Checklist
- [ ] Add retry config to `Config`:
  ```yaml
  retry:
    max_retries: 3
    backoff_base: 1000  # ms
    backoff_max: 30000  # ms
  ```
- [ ] Define `RetryConfig` struct

## Dependencies
- Task 1.3.2

## Deliverables
- Retry config struct and YAML schema

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
