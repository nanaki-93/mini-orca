# 45 — Build top-level desktop navigation

## Status

Pending

## Goal

Expose Summary, Analysis, Bugs, and Editor as distinct stable workspaces.

## Depends on

Task 44.

## Implementation

- Add clearly labeled top-level navigation and make Summary the initial workspace after import.
- Keep project identity, daemon/model connection, active operations, and global errors visible across workspaces.
- Preserve file, symbol, and focused-action command palettes and route their results to Editor.
- Show analysis/finding/draft counts with text or icons in addition to color.
- Provide a navigation action that opens a finding's project-relative file and exact symbol/line context in Editor.

## Acceptance criteria

- All four workspaces are reachable by mouse and keyboard.
- Switching workspaces does not silently discard the selected file or a dirty draft.
- Navigation to Editor never selects a file outside the active project/index.

## Verification

- Add navigation and state-preservation tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
