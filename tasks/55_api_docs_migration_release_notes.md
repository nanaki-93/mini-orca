# 55 — Synchronize API, migration, and release documentation

## Status

Pending

## Goal

Make the documented product, live routes, configuration, and user workflow match the implementation exactly.

## Depends on

Tasks 37, 41, 52, and 54.

## Implementation

- Update OpenAPI, API reference, README, desktop README, configuration reference, and release notes for four workspaces and file-scoped editable drafts.
- Document structured project analysis, findings provenance, explicit verified scans, Analyze-all, chat sessions, replace/create modes, draft revisions, validation/checks, Apply, and undo.
- State Go-first safe editing and analysis-only limitations for languages without exact validators.
- Document migration from whole-file generation/activity UI to declaration drafts/file conversations without preserving obsolete endpoints or examples.
- Keep route-contract tests aligned with registered routes and remove superseded workflow claims.

## Acceptance criteria

- Documentation never claims multi-file edits, direct source editing, automatic scans, or automatic writes.
- Every live endpoint is documented once and every documented endpoint is registered.
- Privacy, remote-provider confirmation, local configuration, and known limitations are explicit.

## Verification

- Run route-contract/documentation tests, `go test ./cmd/daemon ./internal/api/handlers`, and `git diff --check`.
