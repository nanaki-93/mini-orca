# 119 — Add Desktop IDE layout state

## Status

Complete

## Goal

Introduce small, testable layout state and persistence for IDE-style tool windows
without moving workflow authority out of `DesktopWorkflowPresenter`.

## Depends on

Task 118.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 119` and name the layout
  values, persistence migration, pure state tests, likely files, and checks.
- After the commit, post a separate update beginning with `Task 119 complete` and
  report the state boundary, persistence behavior, tests/results, exact commit hash,
  and that Task 120 is next.

## Implementation

- Add a compact immutable `DesktopLayoutState` covering the active left, right, and
  bottom tool windows, their visible/collapsed state, pane sizes, and last focused UI
  region needed for focus restoration.
- Add narrow enums/value types for known tool windows and editor surfaces. Do not add a
  generic docking framework or persist arbitrary composable state.
- Extend or replace `PaneWidthStore` with a layout preference store that retains the
  existing Explorer and action widths, adds bottom height/collapse preferences, clamps
  invalid values, and handles missing or older preference keys.
- Keep provider confirmation, draft contents, request progress, Apply eligibility,
  selected project/file/symbol, and all other workflow truth out of layout
  preferences.
- Add pure transition helpers for opening, closing, selecting, and resizing tool
  windows so shell behavior can be tested without rendering Compose.
- Preserve current visible UI and callbacks in this task; new state may be wired with
  equivalent defaults but must not redesign the shell yet.

## Acceptance criteria

- Layout state has one clear owner and cannot authorize product actions.
- Existing saved pane widths remain usable and all new dimensions are safely clamped.
- Opening one tool window does not silently discard another region's workflow state.
- Pure tests cover defaults, old preferences, corrupt/out-of-range values, transitions,
  and the `1000dp` layout boundary.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 119 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(desktop): add IDE layout state
```

Do not amend, squash, tag, or push the commit.
