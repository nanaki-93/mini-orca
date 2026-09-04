# 142 — Restyle the shell, toolbar, and navigation

## Status

Complete

## Depends on

Task 141.

## Goal

Make the app's main frame match the dark reference while preserving the existing
workspace and tool-window model.

## Implementation

- Replace letter/glyph navigation with an 88dp rail of line icons and visible labels:
  Project, Summary, Analysis, Performance, Bugs & Problems, and Editor.
- Keep existing workspace identities/shortcuts; adapt labels without renaming domain
  states or losing Performance. Make rail content usable on short windows.
- Restyle the toolbar with project menu, honest branch context, a search-shaped
  command entry, daemon/provider state, and room for the later preview utilities.
  The command surface searches existing files/symbols/actions and preserves its
  current contextual keyboard behavior.
- Compose the Explorer/editor/bottom region beside a full-height right pane, with
  bottom tools spanning Explorer plus editor. Keep one shell and real resize handles.
- Apply the plan's initial sizes only to new preferences. Clamp visible docked widths
  to available space while preserving stored widths and a usable editor at 1000dp.
- Keep native window chrome. Restyle landing/open/restore/error states in the same pass.

## Likely files

`DesktopHeader.kt`, `IdeShell.kt`, `DesktopShell.kt`, `DesktopLayoutState.kt`,
`CommandPalette.kt`, and the existing shell/layout/keyboard tests under `desktop/`.

## Acceptance criteria

- Global identity, search, labeled navigation, source region, right pane, bottom
  tools, and status have the reference's clear hierarchy and charcoal styling.
- Exactly 1000dp remains docked and 999dp uses the existing drawers/overlay model.
- Resize and temporary viewport clamps never overwrite stored user preferences.
- Selecting a workspace or opening Commands never generates, changes source, or
  alters the sole selected file/declaration except through existing navigation.
- Daemon connectivity is not mislabeled as a confirmed connection to every model.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Exercise `DesktopShellTest`, `DesktopLayoutStateTest`, `CommandPaletteTest`, and
`DesktopKeyboardNavigationTest`, adding boundary tests for effective pane sizing
and preference restoration. Compare wide and 999dp native views when available.
Complete/move the task and update the index; no unrequested commit.
