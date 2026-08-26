# 42 — Decompose the desktop shell

## Status

Pending

## Goal

Create clear desktop component boundaries before adding four workspaces and editable drafts.

## Depends on

Task 32.

## Implementation

- Extract the shell, header, explorer, command palette, status bar, existing summary, focused action, and review composables from `Main.kt` into cohesive files.
- Keep `Main.kt` limited to application startup, theme, and top-level composition wiring.
- Move daemon calls and coroutine job lifecycle out of leaf rendering composables without introducing a framework or duplicate state store.
- Preserve pane resizing, stored widths, responsive drawers, keyboard shortcuts, generation, checks, Apply, undo, comparison, export, and activity.
- Keep source and diff views read-only and selectable.

## Acceptance criteria

- The refactor causes no intended visual, API, or workflow change.
- No extracted leaf composable performs daemon I/O.
- Existing desktop unit tests and the documented smoke flow remain valid.

## Verification

- Add focused tests only where extraction creates testable helpers or state behavior.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
