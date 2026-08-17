# 15 — Implement sequential Analyze all

## Status

Complete

## Goal

Allow optional cache warming without turning project import into a large uncontrolled model job.

## Depends on

Task 14.

## Implementation

- Add a project-scoped job that analyzes eligible stale/missing files one at a time.
- Support start, progress, pause, cancel, resume, and bounded retry.
- Respect ContextPolicy, project revision changes, model timeouts, and a configurable file/count budget.
- Persist job state under `.mini-orca/sessions/` and surface errors per file.

## Acceptance criteria

- Import never implicitly starts an Analyze-all job.
- Cancel stops after the active request and leaves completed cache entries valid.
- A reindex invalidates or safely restarts the job.

## Verification

- Add fake-LLM sequential ordering, cancellation, restart, and revision-change tests.
