# 92 — Hide empty Required imports

## Status

Pending

## Goal

Keep the editable declaration draft focused by omitting the Required imports control
when the current draft has no required imports.

## Depends on

Task 91.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 92` and name the empty
  versus nonempty imports states, draft invalidation guards, and focused checks.
- After the task commit succeeds, post a separate update beginning with
  `Task 92 complete` and state the behavior, checks, commit hash, and that Task 93 is
  next.

## Implementation

- In `DraftContextPane.kt`, render the compact `Required imports` field only when
  `EditableDraftState.imports` is nonempty.
- Preserve the existing comma-separated rendering/parsing, enabled/disabled behavior,
  focus order, `onDraftImports` callback, and validation/check invalidation for nonempty
  imports.
- When imports are empty, place diagnostics and draft status immediately after the
  declaration editor without an empty spacer, label, placeholder, or replacement
  import-management control.
- Do not change declaration-draft models, API payloads, composition rules, or server
  validation.

## Acceptance criteria

- An empty imports list produces no visible or accessibility-node `Required imports`
  field.
- A nonempty imports list produces the current editable field with all values intact.
- Editing a nonempty imports field still reaches the existing draft-edit event and
  invalidates stale validation/check evidence.
- Declaration editing, diagnostics, status, Validate, stale, and validating states are
  unchanged.

## Verification

- Add or update focused presentation/helper tests for empty and nonempty imports.
- Preserve or extend workflow coverage for parsing and draft invalidation after a
  nonempty import edit.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Commit

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, update
its `tasks/INDEX.md` row, stage only Task 92 changes, inspect the staged diff, and create
exactly this commit:

```text
fix(desktop): hide empty required imports
```

Do not amend, combine, or push the commit.
