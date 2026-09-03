# Mini-Orca task workflow

This directory contains the implementation backlog for [`../plan.md`](../plan.md).

## Layout

- Pending tasks live in this directory as `NN_snake_case.md`.
- Completed tasks move to `completed/` without changing their identifier.
- [`INDEX.md`](INDEX.md) is the authoritative dependency order and status list.
- [`PROMPT_EXECUTE_IDE_UI.md`](PROMPT_EXECUTE_IDE_UI.md) is the completed execution record for
  Tasks 118–132. It required strict sequencing, one implementation writer, and one verified
  commit per task.
- [`PROMPT_EXECUTE_LEGACY_CLEANUP.md`](PROMPT_EXECUTE_LEGACY_CLEANUP.md) and completed
  Tasks 103–117 remain as the audit record for the previous cleanup sequence.
- [`INDEX.md`](INDEX.md) contains the concise historical ledger for Tasks 01–102. Git
  history is the full archive.

## Completed IDE redesign

Tasks 118–132 completed the IDE-style Compose Desktop redesign while preserving the current
Focus Flow palette and Mini-Orca's one-project, one-file, one-symbol, preview-first workflow.
The completed task records and [`INDEX.md`](INDEX.md) provide the delivery ledger.

Task 118 established the plan, task files, index/workflow updates, execution prompt, and
characterization baseline. Tasks 119–132 then implemented and verified each dependency-ordered
slice without rewriting the preview-first safety contract.

## Status lifecycle

Each task has exactly one status: `Pending`, `In Progress`, or `Complete`.

1. Select the first Pending task whose dependencies are Complete.
2. One implementation agent owns that task and its writes until verification finishes.
3. Read-only research/review may run concurrently only when it cannot edit overlapping
   files or mutate shared state.
4. Run the task's focused checks and `git diff --check`.
5. Only after every acceptance criterion passes, set the task to Complete, move it
   under `completed/`, and update `INDEX.md`.
6. Stage only that task's owned implementation, tests, documentation, task move, and
   index update. Inspect the full staged diff.
7. Create exactly one commit using the task's required subject. Do not begin the next
   task until the commit succeeds and no task-owned change remains uncommitted.
8. Post the required completion commentary and continue to the next ready task without
   waiting for a separate “continue” message.

Do not skip a blocked dependency, duplicate its intended behavior in a later task, or
mark a task Complete based on partial implementation.

## Shared implementation rules

- Read `AGENTS.md`, `plan.md`, this file, `INDEX.md`, the execution prompt, and the
  selected task before acting.
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Keep source and diff selectable/read-only; only the isolated declaration/import
  draft is editable.
- Generation, analysis, selection, navigation, validation, and checks never write
  source. Apply and Undo remain the only explicit guarded source mutations.
- Keep remote-provider destination and confirmation visible in text before sending
  project context.
- Preserve existing palette values, keyboard navigation, state labels, and the exact
  `1000dp` responsive boundary.
- Fit the redesign into the existing presenter and feature-state boundaries. Do not
  add a UI framework, generic docking engine, event bus, or duplicate workflow state.
- Do not add general source editing, multi-file tabs/changes, terminal, run/debug,
  filesystem mutation, automatic fixes, automatic commits, or VCS write features.
- Add deterministic behavior-focused tests with every changed interaction or state.
- Remove obsolete UI branches, helpers, call sites, and tests when their replacement
  becomes authoritative; do not preserve parallel shells.
- Preserve unrelated user changes and never edit generated build output.
- Post the required user-facing commentary before and after every task, with concise
  progress updates during work lasting more than 60 seconds.
- The task commentary requirements are conversation updates, not instructions to add
  source-code comments.
- The user's request authorizes exactly one commit per completed Task 118–132. It does
  not authorize pushes, rebases, tags, amendments, squashes, combined commits, or
  commits containing unrelated work.

## Verification baseline

- Desktop formatting/static/tests: `./desktop/gradlew -p desktop spotlessCheck detekt test`
- Desktop tests: `./desktop/gradlew -p desktop test`
- Go formatting: `make fmt-check`
- Go tests: `go test ./...`
- Race tests where required: `make test-race`
- Static analysis: `make vet`
- Full release gate: `make check`
- Project quality gate: `make quality`
- Every task: `git diff --check`

The final report must state each Task 118–132 status, behavior changed, files changed,
acceptance criteria verified, commands run, any blocker or intentionally unavailable
manual check, and the exact commit hash for every completed task.
