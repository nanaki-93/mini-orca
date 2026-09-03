# 124 — Add the Context tool window

## Status

Complete

## Goal

Give file metadata, declaration details, analysis controls, and read-only context a
stable right-side location that does not steal focus.

## Depends on

Task 123.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 124` and name the
  Context states, selection synchronization, provider confirmation, likely files, and
  checks.
- After the commit, post a separate update beginning with `Task 124 complete` and
  report Context behavior, preserved safety, tests/results, exact commit hash, and
  that Task 125 is next.

## Implementation

- Introduce a right tool-window container with explicit Context, Assistant, and Review
  tabs, then make Context the first fully connected tab.
- Move file fallback and selected-declaration content from `SymbolInspectorPane` into
  a compact Context layout with a persistent text header.
- Keep file path, language, size, line count, analysis freshness, symbol signature,
  exact range, confidence, explanation, and edit eligibility visible in logical
  groups.
- Keep file analysis/refresh/cancel and remote-provider confirmation adjacent and
  explicit; opening Context alone must not contact a provider.
- Synchronize Context with source, Project, command palette, and problem navigation
  without discarding a valid current draft.
- Add read-only impact and Git summaries only when already available in current state;
  do not widen the daemon contract.
- Show changed Context content or state badges without automatically focusing or
  opening the window.
- Remove the superseded standalone context rendering path once connected.

## Acceptance criteria

- Context always identifies whether it describes a file or one exact declaration.
- Selecting a declaration updates Context; clicking outside declarations returns to
  file context while retaining the focused line.
- File analysis remains explicit and remote sends still require confirmation.
- Context selection cannot change source, generate a draft, or weaken active-draft
  discard guards.
- Wide and narrow Context presentation has accessible text labels and no essential
  clipping.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add focused Context state, synchronization, remote-confirmation, and semantics tests.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 124 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): add Context tool window
```

Do not amend, squash, tag, or push the commit.
