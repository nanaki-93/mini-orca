# 37 — Expose project overview, scan, and findings APIs

## Status

Complete

## Goal

Provide stable daemon contracts for the Summary, Analysis, and Bugs workspaces.

## Depends on

Tasks 34, 35, and 36.

## Implementation

- Add a project overview response combining deterministic metrics, structured analysis, analysis coverage, and verified/AI finding counts.
- Add revision-guarded endpoints to list/filter findings, update triage, start a verified scan, and read or cancel scan progress.
- Include provenance, confidence, severity, status, path/line/symbol, freshness, and sanitized evidence in finding responses.
- Keep Analyze-all routes compatible and document their `204 No Content` behavior when no job exists.
- Register route-contract tests and update OpenAPI schemas as the live contract changes.

## Acceptance criteria

- Overview remains useful when model analysis or scan data is absent.
- Verified and AI counts are calculated from their explicit source/confidence fields.
- Stale revisions return conflict responses and no endpoint returns source or credentials.
- Every new live route has handler and error-contract coverage.

## Verification

- Run `go test ./cmd/daemon ./internal/api/handlers ./internal/app ./internal/project`.
- Run `make vet` and `git diff --check`.
