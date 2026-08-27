# 70 — Refactor the Analysis workspace

## Status

Complete

## Goal

Turn Analyze-all configuration, progress, results, and controls into a clear Focus
Flow workspace without changing explicit job semantics.

## Depends on

Task 69.

## Implementation

- Recompose coverage, limits, remote confirmation, job state, progress, warnings,
  per-file outcomes, and Start/Pause/Resume/Cancel controls into themed panels.
- Keep Analyze-all explicitly user-started; import/reindex must not start it.
- Preserve bounds, retries, polling, cancellation, terminal states, and stale-revision rules.
- Use a lazy result list where it preserves file navigation and focus.
- Present running/paused/completed/canceled/failed states with text and next actions.

## Acceptance criteria

- All job controls retain current guards and remote-provider confirmation.
- Progress and partial completed results remain visible during pause/cancel/failure.
- File result navigation opens the correct indexed file.
- Large result sets remain responsive.

## Verification

- Extend Analyze-all state/progress/navigation tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
