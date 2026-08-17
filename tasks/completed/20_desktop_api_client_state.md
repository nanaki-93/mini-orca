# 20 — Extend desktop API client and application state

## Status

Complete

## Goal

Give Compose Desktop typed access to all new daemon capabilities and one coherent state model.

## Depends on

Tasks 03, 11, 14, 16, and 19.

## Implementation

- Add Kotlin serializable models and client methods for index, symbols, analysis, context manifest, generation, diff, checks, apply, undo, activity, and effective model status.
- Replace scattered composable-local state with a testable screen/view model and UI state reducer.
- Handle loading, cancellation, conflict, unavailable daemon, and structured API errors consistently.
- Include project revision and base hashes automatically in mutable requests.

## Acceptance criteria

- The desktop app has no untyped JSON parsing for new endpoints.
- API errors render as user-facing messages without losing the selected file or candidate.

## Verification

- Add Kotlin unit tests with a fake HTTP transport or local test server.
