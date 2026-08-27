# 63 — Integrate the Editor workspace and stage navigation

## Status

Complete

## Goal

Make the supported Editor workflow visibly follow Target → Draft → Verify → Apply
while remaining derived from Task 57's presentation contract.

## Depends on

Task 62.

## Implementation

- Add the Editor workspace host and persistent four-stage bar using the Focus Flow design.
- Keep only the currently viewed unlocked stage as transient UI state; clamp it whenever
  file, target, draft, validation, checks, Apply, or Undo state changes.
- Allow navigation to earlier unlocked stages and prevent forward jumps past a gate.
- Present textual complete/current/locked state and disabled reasons for every stage.
- Route each stage through one main canvas and one contextual panel in the Workbench shell.
- Preserve current supported Editor surfaces while subsequent tasks replace each owning
  surface; do not copy them into parallel implementations.

## Acceptance criteria

- The active stage always agrees with current guarded state.
- Editing or staleness immediately moves/clamps the user out of Verify/Apply.
- Stage navigation is pointer- and keyboard-operable and never calls a mutation.
- Other workspaces retain their state when the user leaves and returns to Editor.

## Verification

- Extend stage navigation/clamping/semantics tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
