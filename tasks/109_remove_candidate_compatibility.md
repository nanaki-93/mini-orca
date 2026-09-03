# 109 — Remove candidate and workflow compatibility

## Status

Pending

## Goal

Delete the old whole-file generation/candidate model, its comparison/export surface,
and the generic workflow package now that the retained path is draft-native.

## Depends on

Task 108.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 109` and name the
  compatibility files/routes/types being removed, retained draft behavior, focused
  tests, and existing changes.
- After the commit, post a separate update beginning with `Task 109 complete` and
  report the deletion, retained flow, commands/results, exact commit hash, and that
  Task 110 is next.

## Implementation

- Trace every caller of whole-file generation, `GenerationPreview`, candidate storage,
  candidate comparison/export, template setters, and compatibility check methods.
- Move the few live hash/ID helpers into a focused draft identity file; do not keep
  `generation.go` solely for utility functions.
- Make the draft store hold the draft-native record introduced by Task 108 rather than
  a legacy generation preview.
- Delete obsolete whole-file response parsing and validation.
- Delete candidate compatibility storage, getters, check bridges, template setters,
  comparison/export services, DTOs, handlers, route registrations, and dedicated tests.
- Move declaration-edit validation types and diff helpers still used by the current
  flow into `internal/project` files named for declaration editing.
- Delete `internal/workflow` after all live types have an owning domain package.
- Delete the source-free draft review/audit cache if the Task 103 inventory confirms
  it has no retained consumer. Preserve Apply/Undo audit persistence.
- Update the API contract and Kotlin client only for candidate-specific routes removed
  in this task; Task 110 owns the rest of the API reduction.
- Do not add deprecation aliases, 410 handlers, adapter types, or compatibility tests.

## Acceptance criteria

- There is one draft representation from model response through review and Apply.
- Whole-file generation, candidate comparison/export, candidate compatibility routes,
  and `internal/workflow` are absent.
- Current declaration composition, diff, validation, checks, Apply, audit, and Undo
  behavior remains covered.
- No current name/comment refers to a completed migration task as justification for a
  compatibility method.
- Dead-code analysis no longer reports the removed candidate/generation surface.

## Verification

Run:

```text
go test ./internal/app ./internal/project ./internal/api/handlers
./desktop/gradlew -p desktop test
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Inspect searches for `GenerationPreview`, legacy `Generate`, candidate compare/export,
and `internal/workflow`; only historical task/plan references may remain.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 109 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(app): remove candidate compatibility
```

Do not amend, squash, tag, or push the commit.
