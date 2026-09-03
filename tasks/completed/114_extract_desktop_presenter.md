# 114 — Extract the Desktop workflow presenter

## Status

Complete

## Goal

Move asynchronous daemon interaction, polling, request identity, and workflow
transitions out of the Compose application root into one testable concrete presenter.

## Depends on

Task 113.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 114` and name the
  effects/state moving from `MiniOrcaApp`, presenter boundary, stale-response tests,
  likely files, and checks.
- After the commit, post a separate update beginning with `Task 114 complete` and
  report state ownership, cancellation behavior, commands/results, exact commit hash,
  and that Task 115 is next.

## Implementation

- Inventory every coroutine job, polling loop, API call, request token, workflow
  transition, and mutable state currently owned by `MiniOrcaApp` and
  `DesktopWorkflowController`.
- Introduce one concrete Desktop workflow presenter/controller that owns:
  - `ApiClient` interaction;
  - coroutine jobs and lifecycle cancellation;
  - analysis and scan polling;
  - selected project/file/symbol identities;
  - chat task and draft identities;
  - stale-response rejection;
  - daemon connectivity and operation failures; and
  - an immutable workflow UI snapshot.
- Reduce `MiniOrcaApp` to dependency construction, presenter lifecycle, top-level state
  observation, and root composition.
- Keep ephemeral visual state such as drawer visibility, focus, palette visibility,
  and local filters next to the relevant composable when it has no workflow meaning.
- Replace partial string/boolean identity comparisons with explicit Kotlin value types
  containing the complete project, file, task, and draft identity.
- Cancel superseded work and also reject late results by request identity. A boolean
  loading flag alone is not sufficient.
- Treat `CancellationException` as control flow. Map protocol, daemon connectivity,
  and unexpected failures deliberately and remove broad catches where a narrower
  boundary exists.
- Replace the monolithic reducer with feature-sized pure transitions or clear named
  presenter methods. Do not introduce Redux/MVI libraries, an event bus, or interfaces
  with one production implementation.
- Keep all current UI layout and copy unchanged in this task except changes required
  to connect the presenter.

## Acceptance criteria

- No composable owns a daemon polling loop or coordinates a multi-step API workflow.
- `MiniOrcaApp` is a small composition root and lifecycle host.
- Rapid project/file/symbol changes and late analysis/chat/draft responses cannot
  overwrite the latest selection.
- Cancellation, disconnect/reconnect, Apply, and Undo transitions are unit-testable
  without rendering Compose.
- Preview-first, remote confirmation, identity guards, and read-only source/diff
  behavior are unchanged.

## Verification

Run:

```text
./desktop/gradlew -p desktop test
make check
git diff --check
```

Add focused presenter tests for rapid selection changes, cancellation, late polling,
disconnect/reconnect, draft replacement, Apply, and Undo. Inspect coroutine scopes for
lifecycle leaks.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 114 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(desktop): extract workflow presenter
```

Do not amend, squash, tag, or push the commit.
