# 40 — Integrate draft validation, checks, Apply, and undo

## Status

Complete

## Goal

Require the latest edited draft to complete Validate → Checks → Apply under existing conflict guards.

## Depends on

Task 39.

## Implementation

- Add revision-guarded operations/endpoints to validate a draft, run focused checks for its exact candidate hash, and retrieve its review state.
- Compose candidates only on the daemon and run checks in the existing isolated workspace.
- Bind check reports to draft ID, draft revision/hash, project revision, base file hash, and target path.
- Update Apply to accept only a current valid checked draft and preserve explicit confirmation, atomic write, audit, and undo behavior.
- Keep comparison/export working only for independently valid drafts sharing the same base, target, mode, and action.

## Acceptance criteria

- Dirty, invalid, unchecked, stale, or hash-mismatched drafts cannot apply.
- Apply changes exactly the bound open file and selected/new symbol plus allowed imports.
- Manual editing after checks immediately disables Apply.
- Undo remains conflict-safe and restores the immediately preceding file content.

## Verification

- Add end-to-end service and handler tests for edit/validate/check/apply, stale evidence, comparison/export, and undo.
- Run `go test ./internal/app ./internal/api/handlers -race`, `make vet`, and `git diff --check`.
