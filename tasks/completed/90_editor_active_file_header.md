# 90 — Emphasize the active Editor file

## Status

Complete

## Goal

Make the open file immediately visible at the top of the Editor while removing the
visible Editor progress explanation that currently occupies that space.

## Depends on

Task 89.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 90` and name the file
  header, progress-copy removal, responsive layout, focused tests, and any pre-existing
  changes that must remain outside the commit.
- After the task commit succeeds, post a separate update beginning with
  `Task 90 complete` and state the behavior, checks, commit hash, and that Task 91 is
  next.

## Implementation

- Replace the visible `EditorProgressBar` in `EditorWorkspace.kt` with a compact file
  header that is present above both source and review-diff canvases.
- Show `ProjectFileInfo.name` as the primary, high-contrast title and the
  project-relative `ProjectFileInfo.path` as secondary text. Do not expose filesystem
  roots, file hashes, or project revisions.
- Render an explicit neutral no-file state when `ProjectFileInfo` is null; never retain
  a stale prior filename.
- Keep the header outside source/diff scrolling and give the canvas all remaining
  vertical space at wide, exactly-1000dp, and narrow layouts.
- Remove the visible `EDITOR PROGRESS` label, current-state detail, Inspect/Edit/Review
  trail, and progress-specific semantics. Do not replace them with equivalent visible
  explanatory copy.
- Retain `EditorProgressUiState` only where it remains authoritative for source/diff and
  context routing. Remove progress presentation types/helpers/imports with no remaining
  consumer.
- Preserve the application top bar's project identity and all existing source/diff,
  context drawer, validation, checks, Apply, and receipt behavior.

## Acceptance criteria

- Source and review-diff views show the active file basename prominently at their top.
- The relative path distinguishes same-named files without revealing an absolute path.
- No-file state cannot display a stale filename.
- No visible or screen-reader-only Editor progress explanation remains.
- Long source/diff content still scrolls within the remaining canvas, and Editor chrome
  remains correct at the existing responsive breakpoint.
- Workflow state and all mutation guards are unchanged.

## Verification

- Replace `EditorWorkspaceTest` progress-presentation assertions with file-header
  basename/path/no-file/progress-absence coverage.
- Update affected `DesktopShellTest` or integration coverage for source and review modes
  plus wide/narrow layout behavior.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Commit

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, update
its `tasks/INDEX.md` row, stage only Task 90 changes, inspect the staged diff, and create
exactly this commit:

```text
feat(desktop): emphasize active editor file
```

Do not amend, combine, or push the commit.
