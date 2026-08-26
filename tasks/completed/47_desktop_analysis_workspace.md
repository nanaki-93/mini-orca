# 47 — Implement the Project Analysis workspace

## Status

Complete

## Goal

Expose semantic-analysis coverage and explicit sequential Analyze-all controls.

## Depends on

Tasks 43, 44, and 45.

## Implementation

- Show fresh, stale, missing, running, and failed counts plus per-file analysis progress.
- Add explicit Start, Pause, Resume, and Cancel actions with file/retry limits and remote-provider confirmation when required.
- Poll only while a job can change and stop on completion, cancellation, stale revision, workspace disposal, or daemon failure.
- Refresh index freshness badges as files complete without losing the open file.
- Open a selected progress row in Editor and expose Analyze/Refresh for one file.

## Acceptance criteria

- Import and reindex never start Analyze-all automatically.
- `204 No Content` displays a clear no-job state.
- Completed summaries remain visible after pause/cancel and stale jobs cannot resume on another revision.

## Verification

- Add controller tests for polling lifecycle, pause/resume/cancel, no-content, retry limits, and stale revision.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
