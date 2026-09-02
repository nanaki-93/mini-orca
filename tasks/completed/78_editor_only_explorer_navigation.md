# 78 — Scope explorer, context, and file navigation to Editor

## Status

Complete

## Goal

Give project-level workspaces their full canvas while making every file-opening route
land consistently in Editor.

## Depends on

Task 77.

## Implementation

- Add one explicit, testable decision for whether Editor chrome is visible.
- On wide layouts, render explorer, both relevant resize dividers, and Editor context only
  when `workspace == Workspace.Editor`.
- On Summary, Analysis, and Bugs, render the rail plus full-width workspace content without
  empty Editor side panes.
- Below 1000dp, show Files and Context top-bar/drawer actions only in Editor.
- Close an open Editor drawer when navigation leaves Editor so hidden drawer state cannot
  cover a project-level workspace.
- Preserve the existing 1000dp breakpoint and saved explorer/context widths.
- Centralize app-level “open file in Editor” behavior and reuse it for:
  - file explorer selection;
  - command-palette file selection.
- Preserve finding navigation’s symbol/line target and Editor transition.
- Preserve selected file, selected symbol where valid, Editor surface/stage, asynchronous
  request identity, cancellation, and revision/hash guards.
- Keep global `Cmd/Ctrl+P` available in every workspace; choosing a result must activate
  Editor before loading it.

## Acceptance criteria

- Summary, Analysis, and Bugs reserve no explorer/context width on a wide window.
- Files and Context drawer actions are absent outside Editor on narrow windows.
- Editor retains both wide panes and both narrow drawer actions.
- Leaving Editor with a drawer open cannot leave it covering another workspace.
- Explorer and file-palette selections open the exact indexed path in Editor.
- Returning to Editor restores the previous selection and guarded workflow state.
- Workspace shortcuts, keyboard file palette, semantics, and read-only source/diff behavior
  remain intact.

## Verification

- Extend `DesktopShellTest`, `DesktopAccessibilityTest`, and
  `DesktopIntegrationCoverageTest` for visibility, breakpoint, navigation, and state
  retention rules.
- Run `./desktop/gradlew -p desktop test`.
- Run `git diff --check` and inspect the task diff.

## Commit

After all criteria pass, move this task to `tasks/completed/`, update its index row, use
hunk-level staging for pre-modified Desktop files, inspect `git diff --cached`, and create
exactly this commit:

```text
feat(desktop): scope file navigation to editor
```
