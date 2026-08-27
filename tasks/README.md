# Mini-Orca task workflow

This directory contains the implementation backlog for [`../PLAN.md`](../PLAN.md).

## Layout

- Pending tasks live in this directory as `NN_snake_case.md`.
- Completed tasks move to `completed/` without changing their identifier.
- [`INDEX.md`](INDEX.md) is the authoritative dependency order and status list.
- [`PROMPT_EXECUTE_UI_REFACTOR.md`](PROMPT_EXECUTE_UI_REFACTOR.md) runs the
  approved Tasks 57–74 sequentially with one implementation writer.
- Tasks 01–31 are completed history. The focused AI IDE roadmap begins at 32,
  and the approved Desktop UI refactor begins at 57.

## Status lifecycle

Each task has exactly one status: `Pending`, `In Progress`, or `Complete`.

1. An Air coordinator selects the first Pending task whose dependencies are Complete.
2. One implementation agent owns that task and its writes until verification finishes.
3. Read-only research or review agents may run in parallel, but implementation agents
   must not edit overlapping files concurrently.
4. The implementation agent runs the task's focused checks and `git diff --check`.
5. Only after every acceptance criterion passes does the agent set the task to
   Complete, move it under `completed/`, and update `INDEX.md`.

Do not skip a blocked dependency, copy its behavior into another package, or mark a
task Complete based only on partial implementation.

## Shared implementation rules

- Read `AGENTS.md`, `PLAN.md`, this file, `INDEX.md`, and the selected task first.
- For Tasks 57–74, also read `design/ui-mocks/IMPLEMENTATION_PLAN.md` and inspect
  the approved mock sources named by the selected task.
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Keep source and diff views read-only; only the isolated AI declaration draft is
  editable.
- Never introduce automatic project writes, multi-file candidates, commits, or
  background fixes.
- Reuse the current Go daemon, Compose Desktop client, API client, persistence,
  context policy, revision guards, checks, and audit boundaries.
- Add focused regression tests for every behavior change.
- Preserve unrelated user changes and never edit generated build output.

## Verification baseline

- Go formatting: `make fmt-check`
- Go tests: `go test ./...`
- Race tests where required: `make test-race`
- Static analysis: `make vet`
- Desktop tests: `./desktop/gradlew -p desktop test`
- Full release gate: `make check`
- Every task: `git diff --check`

The final task report must state the task ID/status, files changed, acceptance
criteria verified, commands run, and any blocker or intentionally deferred work.
