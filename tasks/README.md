# Mini-Orca task workflow

This directory contains the implementation backlog for [`../PLAN.md`](../PLAN.md).

## Layout

- Pending tasks live in this directory as `NN_snake_case.md`.
- Completed tasks move to `completed/` without changing their identifier.
- [`INDEX.md`](INDEX.md) is the authoritative dependency order and status list.
- [`PROMPT_EXECUTE_ALL_TASKS.md`](PROMPT_EXECUTE_ALL_TASKS.md) runs the current
  Tasks 85–89 sequentially with one implementation writer and a required user-facing
  start/completion commentary update for every task.
- [`PROMPT_EXECUTE_DESKTOP_UX_REFINEMENTS.md`](PROMPT_EXECUTE_DESKTOP_UX_REFINEMENTS.md)
  runs queued Tasks 90–94 sequentially after Task 89, with exactly one reviewed local
  commit per completed task.
- [`PROMPT_EXECUTE_UI_REFACTOR.md`](PROMPT_EXECUTE_UI_REFACTOR.md) is retained as
  execution history for the completed Tasks 57–74 UI refactor.
- Tasks 01–84 are completed history. The direct-symbol Editor backlog is Tasks 85–89;
  the focused Desktop UX refinement is Tasks 90–94.

## Status lifecycle

Each task has exactly one status: `Pending`, `In Progress`, or `Complete`.

1. The implementation agent selects the first Pending task whose dependencies are Complete.
2. One implementation agent owns that task and its writes until verification finishes.
3. Read-only research or review agents may run in parallel, but implementation agents
   must not edit overlapping files concurrently.
4. The implementation agent runs the task's focused checks and `git diff --check`.
5. Only after every acceptance criterion passes does the agent set the task to
   Complete, move it under `completed/`, and update `INDEX.md`.
6. For Tasks 90–94, the agent stages only task-owned changes, reviews the staged diff,
   and creates the task's one required local commit. Earlier tasks retain their own
   documented commit policy.
7. The agent posts the task's required completion commentary and continues to the next
   ready task without waiting for a separate “continue” message.

Do not skip a blocked dependency, copy its behavior into another package, or mark a
task Complete based only on partial implementation.

## Shared implementation rules

- Read `AGENTS.md`, `PLAN.md`, this file, `INDEX.md`, and the selected task first.
- For Tasks 85–89, follow their task-local direct-symbol requirements. For Tasks 90–94,
  follow the current `PLAN.md`. Old UI mocks and Tasks 57–84 are historical context,
  not acceptance requirements except where the current plan explicitly preserves their
  safety behavior.
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Keep source and diff views read-only; only the isolated AI declaration draft is
  editable.
- Never introduce automatic project writes, multi-file candidates, commits, or
  background fixes.
- Reuse the current Go daemon, Compose Desktop client, API client, persistence,
  context policy, revision guards, checks, and audit boundaries.
- Add focused regression tests for every behavior change.
- Preserve unrelated user changes and never edit generated build output.
- Post the required user-facing commentary before and after every task, with concise
  progress updates during work lasting more than 60 seconds.
- The task comments are conversation commentary, not a reason to add source-code comments.
- Tasks 85–89 retain their no-commit rule. Tasks 90–94 require exactly one reviewed
  local commit each, using the exact message in the task file; they do not authorize
  pushes, rebases, tags, amendments, squashes, or combined commits.
- Do not start Task 90 until Task 89 is Complete and pre-existing changes that overlap
  the new task files have an established commit boundary. Never absorb unrelated or
  preceding-backlog changes into a Task 90–94 commit.

## Verification baseline

- Go formatting: `make fmt-check`
- Go tests: `go test ./...`
- Race tests where required: `make test-race`
- Static analysis: `make vet`
- Desktop tests: `./desktop/gradlew -p desktop test`
- Full release gate: `make check`
- Every task: `git diff --check`

The final task report must state each task ID/status, behavior changed, files changed,
acceptance criteria verified, commands run, commit hash, and any blocker or intentionally
deferred work.
