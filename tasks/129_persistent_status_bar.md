# 129 — Add the persistent Desktop status bar

## Status

Pending

## Goal

Replace the transient loading/error footer with a compact, always-available status bar
that reports only trusted project, editor, provider, and daemon state.

## Depends on

Task 128.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 129` and name the status
  segments, trusted data sources, accessibility behavior, likely files, and checks.
- After the commit, post a separate update beginning with `Task 129 complete` and
  report status behavior, tests/results, exact commit hash, and that Task 130 is next.

## Implementation

- Make the status bar persistent whenever a project workspace is open.
- Derive compact segments for current/last operation message, project/index state,
  selected file language, focused line, file-analysis freshness, scoped provider
  destination/locality, daemon connection, and current error attention.
- Display only values already known by the Desktop state/API contract. Do not invent
  encoding, line-ending, VCS, or provider-health claims.
- Use responsive priority rules so lower-value segments collapse before operation,
  error, remote-provider, or connection state.
- Make actionable status segments keyboard focusable and give every icon/dot a visible
  text equivalent, tooltip, and semantic description.
- Let error/status details open a read-only relevant tool window or popup; do not add a
  second error store.
- Reuse existing connection and model presentation functions and remove the
  error/loading-only footer path.

## Acceptance criteria

- Idle, busy, successful, stale, disconnected, remote, and failed states have accurate
  text presentations.
- The status bar stays compact, does not obscure content, and degrades predictably at
  narrow widths.
- It cannot initiate source mutation or claim unavailable environmental facts.
- Status details derive from authoritative current state and do not retain stale
  project/file values after transitions.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add presentation tests for every status combination, priority/collapse rules, stale
state clearing, semantics, and actionable navigation.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 129 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): add persistent status bar
```

Do not amend, squash, tag, or push the commit.
