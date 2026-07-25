# Task 4.6.2 — Implement Retry Wrapper

## Milestone
Milestone 4: Orchestrator & State Machine

## Description
Implement retry logic with exponential backoff.

## Checklist
- [ ] Create `internal/orchestrator/retry.go`
- [ ] Implement `WithRetry(fn func() error, maxRetries int) error`
- [ ] Implement exponential backoff
- [ ] Log each retry attempt
- [ ] Return final error after all retries exhausted

## Dependencies
- Task 4.6.1

## Deliverables
- `internal/orchestrator/retry.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
