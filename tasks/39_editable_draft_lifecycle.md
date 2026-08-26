# 39 — Add the editable draft lifecycle

## Status

Pending

## Goal

Track manually editable declaration drafts with immutable base identity and revisioned validation state.

## Depends on

Task 38.

## Implementation

- Replace the applicable-candidate-only map with a project-scoped draft store containing base project/file identity, edit mode, target, declaration, imports, lineage, revision, and hash.
- Support generated, dirty, validating, valid, invalid, and stale states without persisting source or invalid candidate files.
- Add a draft-update operation that accepts the expected draft revision and creates a new revision/hash for manual declaration edits.
- Clear validation, checks, comparison, and Apply eligibility on every content/import edit.
- Expire or mark drafts stale when project revision, open path, or base file hash changes.

## Acceptance criteria

- Concurrent or stale draft updates are rejected without losing the current draft.
- Invalid drafts remain editable and recoverable but can never be checked or applied.
- An older validation/check result cannot authorize a newer draft revision.
- Switching projects clears in-memory draft source and preserves only source-free audit metadata.

## Verification

- Add lifecycle, concurrency, invalidation, lineage, and project-switch tests in `internal/app`.
- Run `go test ./internal/app -race`, `make vet`, and `git diff --check`.
