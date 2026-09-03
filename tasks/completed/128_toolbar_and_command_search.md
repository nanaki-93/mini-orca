# 128 — Simplify the toolbar and command search

## Status

Complete

## Goal

Reduce top-bar competition and make file, symbol, and scoped-action discovery feel
consistent and keyboard-driven.

## Depends on

Task 127.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 128` and name the
  toolbar hierarchy, overflow rules, command search model, likely files, and checks.
- After the commit, post a separate update beginning with `Task 128 complete` and
  report toolbar/search behavior, shortcut preservation, tests/results, exact commit
  hash, and that Task 129 is next.

## Implementation

- Keep Mini-Orca identity, current project context, command search, connection state,
  and active operation visible in the main toolbar.
- Move Open project, Re-index, Reconnect, and other infrequent project operations into
  a labeled project widget or overflow when width requires it. Keep critical recovery
  actions discoverable and text-labeled.
- Refactor Files, Symbols, and Actions palette modes onto one reusable command-search
  shell with result type, accessible description, selection highlight, empty state,
  and shortcut hint.
- Add arrow-key selection, `Enter` activation, and predictable focus restoration on
  dismissal. Keep the existing focused shortcuts and current availability checks.
- Constrain results to indexed files, symbols from the active file, and actions valid
  for current state. Do not add global filesystem search or unsupported commands.
- Define deterministic toolbar overflow behavior around `1000dp` and at supported
  text scaling; state text takes priority over decorative content.
- Remove obsolete top-bar buttons/dialog branches once the replacement is complete.

## Acceptance criteria

- The toolbar does not clip essential state or actions at supported widths/scaling.
- File, symbol, and action search share interaction behavior but retain their scopes.
- Existing shortcuts continue to resolve exactly once and unavailable actions remain
  unavailable.
- Search selection cannot bypass indexed paths, target checks, remote confirmation, or
  draft-discard handling.
- Closing search restores focus to the prior meaningful region.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add focused tests for result ordering, scope, keyboard selection, activation,
availability, toolbar overflow, and focus restoration.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 128 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): unify toolbar command search
```

Do not amend, squash, tag, or push the commit.
