# Mini-Orca task workflow

This directory contains the implementation backlog for [`../PLAN.md`](../PLAN.md).

## Layout

- Pending tasks live in this directory as `NN_snake_case.md`.
- Completed tasks move to `completed/` without changing their identifier.
- [`INDEX.md`](INDEX.md) is the authoritative dependency order and status list.
- [`PROMPT_EXECUTE_ALL_TASKS.md`](PROMPT_EXECUTE_ALL_TASKS.md) runs the current
  Tasks 75–79 sequentially with one implementation writer and one commit per task.
- [`PROMPT_EXECUTE_UI_REFACTOR.md`](PROMPT_EXECUTE_UI_REFACTOR.md) is retained as
  execution history for the completed Tasks 57–74 UI refactor.
- Tasks 01–74 are completed history. The current Analysis/Editor UX backlog begins
  at Task 75.

## Status lifecycle

Each task has exactly one status: `Pending`, `In Progress`, or `Complete`.

1. The implementation agent selects the first Pending task whose dependencies are Complete.
2. One implementation agent owns that task and its writes until verification finishes.
3. Read-only research or review agents may run in parallel, but implementation agents
   must not edit overlapping files concurrently.
4. The implementation agent runs the task's focused checks and `git diff --check`.
5. Only after every acceptance criterion passes does the agent set the task to
   Complete, move it under `completed/`, and update `INDEX.md`.
6. For Tasks 75–79, the agent stages only task-owned hunks, inspects
   `git diff --cached`, and creates the exact commit named by the task.

Do not skip a blocked dependency, copy its behavior into another package, or mark a
task Complete based only on partial implementation.

## Shared implementation rules

- Read `AGENTS.md`, `PLAN.md`, this file, `INDEX.md`, and the selected task first.
- For the current Tasks 75–79, follow the approved decisions in `PLAN.md`. The old
  UI mocks are historical context, not acceptance requirements for this UX fix.
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Keep source and diff views read-only; only the isolated AI declaration draft is
  editable.
- Never introduce automatic project writes, multi-file candidates, commits, or
  background fixes.
- Reuse the current Go daemon, Compose Desktop client, API client, persistence,
  context policy, revision guards, checks, and audit boundaries.
- Add focused regression tests for every behavior change.
- Preserve unrelated user changes and never edit generated build output.
- Never use `git add -A` or stage a whole pre-modified file blindly. Use hunk-level
  staging and stop if task changes cannot be separated safely from user-owned edits.
- The user has authorized one repository commit for each completed Task 75–79. This
  does not authorize product-generated commits, pushes, rebases, or commits of
  unrelated worktree changes.

## Verification baseline

- Go formatting: `make fmt-check`
- Go tests: `go test ./...`
- Race tests where required: `make test-race`
- Static analysis: `make vet`
- Desktop tests: `./desktop/gradlew -p desktop test`
- Full release gate: `make check`
- Every task: `git diff --check`

The final task report must state the task ID/status, commit hash, files changed,
acceptance criteria verified, commands run, and any blocker or intentionally
deferred work.
