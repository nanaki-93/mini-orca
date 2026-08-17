# Task 1.1 — Fix human-gate concurrency and server timeouts

**Phase:** 1 — Correctness and security  
**Status:** Complete  
**Priority:** Critical  
**Risk:** High

## Goal

Remove the human-review data race and ensure slow LLM operations are not terminated by startup or HTTP timeouts.

## Plan coverage

- Fix the human-gate data race.
- Keep project analysis and generation reliable for normal LLM response times.

## Dependencies

- None.

## Files

- `internal/orchestrator/orchestrator.go`
- `internal/orchestrator/orchestrator_test.go`
- `cmd/daemon/main.go`

## Implementation steps

1. Protect access to the active `HumanGate` pointer with a dedicated `sync.RWMutex`.
2. Add safe getter/setter methods and use them in review execution and completion transitions.
3. Update concurrent tests to access the gate through the synchronized API.
4. Give the startup model-list call a short context deadline so daemon startup cannot hang indefinitely.
5. Set the HTTP write timeout high enough for project analysis and code-generation LLM requests.

## Acceptance criteria

- The race detector reports no concurrent access to the active human gate.
- Human approval still unblocks `RunHumanReview` and permits completion.
- An unavailable model-list endpoint delays startup by at most the configured short timeout.
- Analysis/generation responses may run for several minutes without the server closing the response.

## Verification

```bash
go test -race ./internal/orchestrator
go test ./cmd/daemon
go vet ./...
```
