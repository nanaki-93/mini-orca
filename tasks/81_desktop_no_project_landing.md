# 81 — Add the exclusive no-project landing state

## Status

Pending

## Goal

When no project is open, expose only the Open project workflow and prevent hidden
workspace UI or shortcuts from remaining interactive.

## Depends on

Task 80.

## Required task commentary

- Before editing, post a concise user-facing commentary update beginning with
  `Starting Task 81` and name the landing-state boundary, shortcut guard, likely files,
  and focused tests.
- After verification, post a concise update beginning with `Task 81 complete` and state
  the empty-state behavior, transition coverage, tests run, and that Task 82 is next.
- During long-running work, continue posting brief progress commentary at least every
  60 seconds. These are user-facing task updates, not source-code comments.

## Implementation

- Add one pure, testable shell-mode decision derived from whether
  `appState.project != null`. Use the same decision for rendering and shortcut routing.
- Split `DesktopShell` into mutually exclusive branches:
  - a no-project landing state;
  - the existing responsive project workspace.
- Build the landing state from the existing code-native Mini-Orca mark and typography.
  Do not duplicate the logo drawing or introduce an image asset.
- The landing state may render only:
  - Mini-Orca identity;
  - one Open project button using the Task 80 Primary compact style;
  - an in-place progress indicator/status while a chosen project is opening;
  - one concise inline error after an open attempt fails.
- Do not render the normal top bar, workspace rail, explorer, workspace canvas, Editor
  context, status bar, Command/Re-index/Reconnect buttons, Files/Context drawers,
  Command Palette, or Context Inspector while no project is open.
- Add `DesktopShortcut.OpenProject` and map `Cmd/Ctrl+O` to it without breaking the
  existing `Cmd/Ctrl+Shift+O` symbol shortcut.
- In no-project mode, accept only OpenProject at the application shortcut boundary.
  Ignore file, symbol, command, workspace, Editor, Generate, Validate, Checks, and
  next-tab shortcuts. The native chooser may continue handling its own Escape/cancel
  behavior.
- Gate callbacks at the shell boundary so a hidden project-only action is not merely
  invisible but still invokable from keyboard routing.
- Disable duplicate Open project activation while an import request is running. Keep the
  spinner/progress in the landing state rather than revealing the full shell.
- Preserve the current native directory chooser:
  - cancel leaves the landing state unchanged;
  - import failure keeps Open project available for retry;
  - successful import replaces the landing state with the full project workspace.
- Ensure project-only dialogs/drawers are closed or not composed during the transition.
- Do not add automatic project restoration, automatic import, a recent-project list,
  daemon/API changes, or a new close-project feature.

## Acceptance criteria

- With `project == null`, Open project is the only interactive application action.
- No workspace panel, navigation control, status bar, drawer, palette, or reconnect action
  is rendered behind or beside the landing state.
- `Cmd/Ctrl+O` activates Open project; every other project/workspace shortcut is ignored
  in no-project mode.
- `Cmd/Ctrl+Shift+O` still opens symbols when a project is open and does nothing on the
  landing state.
- Canceling the chooser does not create a loading/error state.
- Opening progress blocks duplicate requests without exposing the workspace.
- A failed import shows a concise retryable error; a successful import shows the normal
  shell with no stale landing content.
- Existing wide/narrow workspace behavior is unchanged after a project opens.

## Verification

- Extend `DesktopShellTest` for shell mode and visible-region rules.
- Extend `DesktopAccessibilityTest` for `Cmd/Ctrl+O`, shortcut gating, and retained
  symbol shortcut behavior.
- Extend `DesktopIntegrationCoverageTest` or the nearest workflow-controller test for
  cancel/failure/success transitions where practical without opening a real Swing chooser.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

- Inspect the complete Task 81 diff for project-only UI composed in the landing branch,
  duplicate logo code, unguarded shortcuts, and unrelated changes.

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update its link/status in `tasks/INDEX.md`. Do not create a commit; this task definition
does not authorize commits.
