# 11 — Deliver project-index and symbol APIs

## Goal

Make deterministic project facts available to the desktop client.

## Depends on

Tasks 08–10.

## Implementation

- Implement `GET /api/projects/current/index`.
- Implement `GET /api/projects/current/files/symbols?path=...`.
- Implement `POST /api/projects/current/reindex` with revision-aware response.
- Return stable error schemas for missing project, excluded file, invalid path, stale revision, and unsupported extraction.
- Update API documentation and route-contract fixtures.

## Acceptance criteria

- The API returns only project-relative safe paths.
- Symbol metadata has name, kind, signature, line range, confidence, and atomic-target flag.

## Verification

- Add `httptest` cases for each route and invalid-path case.
