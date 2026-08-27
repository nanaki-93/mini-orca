# 61 — Build the responsive Workbench scaffold

## Status

Complete

## Goal

Replace the monolithic shell layout with the selected rail, explorer, primary
canvas, contextual panel, and status-bar structure.

## Depends on

Task 60.

## Implementation

- Refactor the shell into cohesive slots for explorer, workspace content, and
  contextual actions while keeping global shortcuts, dialogs, and cancellation centralized.
- Preserve resizable explorer/context panes, persisted width bounds, and stable dividers.
- Remove the global metric strip; project metrics stay in Summary and selected-file
  facts move into Editor context.
- Keep status/error/connection feedback in a compact textual status bar.
- At widths below 1000dp, hide wide side panes and expose labeled Files and Context
  drawers; do not squeeze all panes together.
- Keep the current production surfaces working in the new slots until their owning
  later task replaces them; do not duplicate any surface.
- Reduce the shell's callback list by giving workspace content its own small named
  action group only where it improves cohesion.

## Acceptance criteria

- Wide layout has stable rail/explorer/canvas/context regions.
- Width persistence and the exact 999/1000dp breakpoint behavior remain correct.
- Drawers close after a file/context action and restore predictable focus.
- No project facts or selected-file facts are duplicated in a global strip.

## Verification

- Extend pane width, breakpoint, drawer, and shell-routing tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
