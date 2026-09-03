# 120 — Build the IDE shell foundation

## Status

Complete

## Goal

Replace the monolithic application framing with a stable toolbar, tool-window bars,
editor area, bottom region, and status region while retaining all current features.

## Depends on

Task 119.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 120` and name the shell
  regions, extraction boundaries, responsive behavior, likely files, and checks.
- After the commit, post a separate update beginning with `Task 120 complete` and
  report the shell structure, preserved behavior, tests/results, exact commit hash,
  and that Task 121 is next.

## Implementation

- Split `DesktopShell.kt` along cohesive visual boundaries into a small root shell and
  reusable main-toolbar, tool-window-bar, docked-pane, editor-area, bottom-region, and
  status-region components.
- Introduce the compact left tool-window bar for Project, Summary, Analysis, Problems,
  and Editor navigation. Glyphs must have tooltips and semantics; the active region
  must expose a visible text title.
- Wire `DesktopLayoutState` to the shell and keep existing feature panes operational
  inside the new regions before later tasks restyle their contents.
- Preserve resizable wide panes, the exact `1000dp` wide/narrow boundary, existing
  modal drawer access below it, and current keyboard shortcuts.
- Give visible dividers a subtle one-pixel treatment while retaining a larger pointer
  target and a testable clamped resize path.
- Keep `MiniOrcaApp` as the composition/lifecycle root and
  `DesktopWorkflowPresenter` as the async workflow owner.
- Remove obsolete shell helpers and old parallel layout branches in the same change.

## Acceptance criteria

- Every current workflow remains reachable in the new frame.
- The center canvas remains available while docked tool windows open or close.
- Narrow windows retain Files and Context access and do not clip essential actions.
- No composable starts daemon work merely because a tool window opens.
- The old full-width workspace rail and superseded shell code do not remain as a
  second implementation.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Perform the shell and workspace-navigation portion of the keyboard smoke checklist
when an interactive Desktop is available.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 120 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): introduce IDE shell
```

Do not amend, squash, tag, or push the commit.
