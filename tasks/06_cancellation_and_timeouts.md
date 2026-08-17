# 06 — Propagate request cancellation and deadlines

## Goal

Stop LLM and check work when the desktop request is canceled or exceeds its declared deadline.

## Depends on

Task 02.

## Implementation

- Accept `context.Context` in generation, summary, tester, reviewer, and executor methods.
- Pass `r.Context()` through the application service into all HTTP and process calls.
- Replace `context.Background()` in request paths.
- Configure separate import, analysis, generation, and focused-check timeouts.
- Return a recognizable canceled/timeout API error and record no partial candidate.

## Acceptance criteria

- Canceling a client request cancels the fake slow LLM call.
- Timeout messages are distinct from model/provider errors.

## Verification

- Add slow-server cancellation tests and run `go test -race ./...`.
