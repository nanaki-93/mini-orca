# 127 — Add Checks and Output tool windows

## Status

Pending

## Goal

Expose validation, focused checks, analysis progress, and sanitized failures in a
predictable bottom area without duplicating workflow state.

## Depends on

Task 126.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 127` and name the
  Checks/Output states, shared presentation logic, persistence, likely files, and
  checks.
- After the commit, post a separate update beginning with `Task 127 complete` and
  report evidence/output behavior, tests/results, exact commit hash, and that Task 128
  is next.

## Implementation

- Add a Checks bottom tab for current validation diagnostics and focused check rows,
  including required/optional, running, passed, skipped, failed, stale, and unavailable
  text states.
- Add an Output bottom tab for current/last project analysis, scan, file analysis,
  generation, cancellation, daemon failure, and sanitized error/status messages
  already held by the presenter.
- Reuse `ReviewEvidencePane` and analysis presentation rules at the narrowest sensible
  boundary. The right Review tab remains the Apply gate; bottom Checks is evidence and
  navigation, not a second authorization path.
- Let failed checks bring the collapsed bottom summary to attention without stealing
  focus or automatically expanding the pane.
- Preserve project-wide Analysis as the detailed home for run limits and controls;
  Output shows concise progress and failures.
- Persist bottom height, collapsed state, and selected bottom tab through
  `DesktopLayoutState` only.
- Bound and scroll long diagnostic/command output and retain selectable text.

## Acceptance criteria

- Checks and Output remain consistent with authoritative presenter/draft state.
- Stale evidence is clearly distinguished and cannot enable Apply.
- Errors are sanitized, selectable, scrollable, and visible without replacing source.
- Opening or selecting bottom tabs has no network or source-write side effect.
- Bottom layout preferences persist without persisting evidence or workflow authority.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add focused tests for evidence mapping, stale/current identity, attention summaries,
output truncation/presentation, and bottom-pane persistence.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 127 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): add Checks and Output tools
```

Do not amend, squash, tag, or push the commit.
