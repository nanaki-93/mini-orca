# 03 — Align API contract, routes, and documentation

## Goal

Make registered HTTP routes, OpenAPI, README, Docker guidance, and desktop-client expectations truthful and synchronized.

## Depends on

Tasks 01–02.

## Implementation

- Define request/response schemas for project, index, summary, generation, validation, apply/undo, status, and errors.
- Remove obsolete session/gate/browser-action paths from OpenAPI and docs, or implement only routes selected in Task 01.
- Document daemon endpoints as local desktop-client APIs rather than browser IDE routes.
- Add a route inventory test that fails when documented live routes drift from the mux.
- Update README, API.md, docs, Docker documentation, and version/configuration examples.

## Acceptance criteria

- Every documented non-deprecated route is registered and tested.
- No docs promise the removed web IDE or automatic coder/tester/reviewer workflow.

## Verification

- Run contract tests and inspect generated OpenAPI validation.
