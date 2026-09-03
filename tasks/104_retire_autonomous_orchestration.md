# 104 — Retire autonomous orchestration

## Status

Pending

## Goal

Delete the unreachable autonomous orchestration subsystem and its tests without
changing the focused preview-first runtime.

## Depends on

Task 103.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 104` and identify the
  production-entry-point proof, deletion scope, focused checks, and pre-existing work.
- After the commit, post a separate update beginning with `Task 104 complete` and
  report deletion evidence, commands/results, changed files, exact commit hash, and
  that Task 105 is next.

## Implementation

- Reconfirm from `cmd/mini-orca` and production imports that `internal/orchestrator`
  has no runtime caller, registration, reflection use, build-tag entry point, or
  serialized compatibility responsibility.
- Delete the complete `internal/orchestrator` package, including history, human gate,
  retry, error, orchestration code, tests, and package-only fixtures.
- Remove documentation references that describe this subsystem as current behavior;
  leave the broader documentation consolidation to Task 116.
- Remove any module dependency that becomes unused solely because of this deletion.
- Do not move or preserve the implementation under a new package.
- Do not change `internal/agent` or `internal/tools`; their active slivers are migrated
  in Tasks 105 and 106.

## Acceptance criteria

- `internal/orchestrator` no longer exists.
- `rg` finds no Go import, runtime registration, or current-documentation claim for
  the deleted package.
- No autonomous pipeline, human-gate state machine, retry history, or write workflow
  is copied elsewhere.
- The focused Mini-Orca workflow behaves exactly as characterized by Task 103.

## Verification

Run:

```text
rg -n 'internal/orchestrator|package orchestrator' --glob '!tasks/completed/**' .
make fmt-check
go test ./...
make vet
git diff --check
```

The first command may match the plan and current task history; inspect and confirm it
finds no live import or current product documentation claim.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 104 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor: remove autonomous orchestration
```

Do not amend, squash, tag, or push the commit.
