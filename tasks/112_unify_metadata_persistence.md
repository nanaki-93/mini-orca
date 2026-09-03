# 112 — Unify metadata persistence

## Status

Pending

## Goal

Consolidate equivalent atomic metadata writes and corrupt-file recovery, stop producing
obsolete metadata, and remove dead project persistence helpers.

## Depends on

Task 111.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 112` and name the
  persistence implementations, retained file semantics, obsolete outputs, tests, and
  checks.
- After the commit, post a separate update beginning with `Task 112 complete` and
  report the shared primitive, retired files/helpers, commands/results, exact commit
  hash, and that Task 113 is next.

## Implementation

- Inventory temporary-file/write/sync/chmod/rename sequences used by retained project
  reports, index, findings, file analysis, scan state, and Apply audit.
- Add a small `internal/storage` atomic write primitive that creates the temporary file
  in the destination directory, writes, syncs, applies intended permissions, closes,
  renames, and cleans up on every failure.
- Add one corrupt-file recovery helper only for stores whose current contract already
  recovers corrupt metadata. Preserve recovery naming and diagnostics or document the
  intentionally improved consistent rule.
- Migrate only callers with equivalent metadata semantics. Keep source Apply/Undo in
  the application domain because validation, backup, audit, and recovery differ.
- Keep serialization and domain policy with each store; do not create generic
  repositories, reflection-based stores, or interfaces with one implementation.
- Stop generating `.mini-orca/analysis.md`; retain
  `.mini-orca/project-analysis.json` as the canonical report and remove the obsolete
  projection field.
- Confirm Task 110 removed activity persistence and that no code recreates
  `.mini-orca/sessions/activity.json`.
- Delete dead path-for-write/directory resolvers, obsolete analyzer constructors,
  unused finding reconciliation, redundant report builders, and newly unreachable
  persistence helpers.
- Do not delete pre-existing obsolete files from user projects automatically; document
  safe manual cleanup.

## Acceptance criteria

- Equivalent retained metadata stores use one tested atomic-write implementation.
- Temporary files are cleaned on failure and prior valid data survives a failed write.
- Permissions, deterministic JSON, corrupt-file recovery, and cancellation/error
  behavior remain correct.
- No runtime code writes the obsolete Markdown analysis or activity file.
- Source Apply/Undo remains separate and retains all identity/audit guarantees.

## Verification

Run:

```text
go test ./internal/storage ./internal/project ./internal/app
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Add failure-path tests for write, sync/close where practical, rename, cleanup, corrupt
input, and permission preservation.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 112 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(storage): unify metadata persistence
```

Do not amend, squash, tag, or push the commit.
