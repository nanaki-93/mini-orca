# 14 — Deliver per-file analysis APIs

## Goal

Expose cached and on-demand semantic analysis to the desktop client.

## Depends on

Tasks 12–13.

## Implementation

- Implement `GET /api/projects/current/files/analysis?path=...` returning status and cached data.
- Implement `POST /api/projects/current/files/analysis` for exactly one file and optional refresh.
- Implement `DELETE /api/projects/current/files/analysis?path=...`.
- Require revision checks and return analysis metadata, never unneeded source text.
- Document asynchronous/running response behavior if analysis is not completed within the request timeout.

## Acceptance criteria

- Missing, stale, fresh, failed, and excluded statuses are distinguishable.
- A client cannot read or clear analysis outside the active project.

## Verification

- Add HTTP success/error/revision tests.
