# 91 — Remove the Editor source instruction

## Status

Complete

## Goal

Remove the visible “Click a highlighted declaration…” subtitle while preserving every
source interaction and accessibility behavior it describes.

## Depends on

Task 90.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 91` and name the exact
  copy removal, source interaction regression coverage, and focused checks.
- After the task commit succeeds, post a separate update beginning with
  `Task 91 complete` and state the behavior, checks, commit hash, and that Task 92 is
  next.

## Implementation

- Remove the visible `Text` containing “Click a highlighted declaration start line to
  inspect it. Drag anywhere to select source text.” from `SourceEditorPane.kt`.
- Remove spacing or imports made obsolete only by that subtitle.
- Do not change declaration hit testing, source-line emphasis, pointer hover/tap
  behavior, drag detection, selectable source, focused-line presentation, content
  descriptions, or keyboard symbol navigation.
- Do not add replacement instructional copy elsewhere.

## Acceptance criteria

- The “Click a…” subtitle and its reserved vertical space are absent for files with
  symbols.
- Clicking a declaration start line still selects it and opens the same context.
- Dragging source still selects text without changing the selected declaration.
- Selectable declaration semantics and keyboard navigation remain available.
- No source or diff content becomes editable.

## Verification

- Update focused source/editor tests to assert the subtitle is not part of the
  presentation while existing click-versus-drag and line-selection tests remain green.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Commit

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, update
its `tasks/INDEX.md` row, stage only Task 91 changes, inspect the staged diff, and create
exactly this commit:

```text
refactor(desktop): remove source helper subtitle
```

Do not amend, combine, or push the commit.
