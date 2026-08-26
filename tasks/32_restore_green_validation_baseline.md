# 32 — Restore the green validation baseline

## Status

Pending

## Goal

Make the existing Go validation suite deterministic and non-hanging before feature work begins.

## Depends on

Task 31.

## Implementation

- Fix Analyze-all HTTP fixtures to decode `llm.ChatRequest` and inspect message content instead of matching unescaped prompt JSON in the raw request body.
- Replace unbounded test-channel waits with bounded helpers that fail from the test goroutine and always release server handlers.
- Make cancellation tests prove client and worker cancellation without depending on fragile `httptest` cleanup timing.
- Determine whether any observed hang is a production cancellation defect; fix production code only when a focused regression demonstrates it.
- Keep retry and cancellation behavior unchanged unless a test proves the behavior is wrong.

## Acceptance criteria

- `internal/app` and `internal/llm` tests finish without timeout or leaked handlers.
- Analyze-all ordering, pause, resume, cancel, and completed-cache retention remain covered.
- The complete Go test suite passes repeatedly.

## Verification

- Run `go test ./internal/llm ./internal/app ./internal/api/handlers -count=3 -timeout=90s`.
- Run `go test ./...`, `make vet`, and `git diff --check`.
