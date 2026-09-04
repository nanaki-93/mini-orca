# 148 — Verify responsive and accessible dark UI behavior

## Status

Complete

## Depends on

Task 147.

## Goal

Make the redesigned real and preview controls usable at the supported viewport
boundary, on short windows, with keyboard navigation, and with enlarged text.

## Implementation

- Exercise 1440x900, 1280x800, 1100x760, exactly 1000dp, 999dp, 800x650, and short
  windows. Fix clipping, pane budgets, bottom height, and preview overlay placement.
- Preserve labeled Files/AI Context drawers and bottom overlay below 1000dp, including
  leaving Editor, resizing across the boundary, and reopening collapsed panels.
- Verify tree/rail/tab keyboard navigation, command/composer shortcuts, Escape's
  topmost-surface behavior, and focus restoration after every dialog/drawer/preview.
- Verify readable text at normal and enlarged system scale, including long project
  paths, symbols, diagnostics, provider destinations, file tabs, and status details.
- Audit semantic names for icons, read-only code/diff, Preview controls, selections,
  severity/provenance, connection states, and provider confirmation.
- Recheck actual color/background pairs after all surfaces are integrated. Fix dim
  essential labels and focus indicators at the shared token/style boundary.
- Update `desktop/KEYBOARD_SMOKE_CHECKLIST.md` so current expectations no longer
  describe the historical 176dp rail or transient footer. Retain clearly labeled
  historical evidence and record unavailable live checks without claiming passes.

## Likely files

Affected desktop shell/theme/components, `DesktopAccessibility.kt`,
`DesktopKeyboardNavigation.kt`, the keyboard checklist, and related existing tests.

## Acceptance criteria

- Wide/narrow behavior changes only at the established 1000dp boundary; state and
  persisted pane preferences survive resizing and switching workspaces.
- All essential actions and labels remain reachable without hover or mouse input.
- No overlapping preview/candidate panel obscures required confirmation or Apply evidence.
- Source selection/read-only semantics and all existing command meanings are preserved.
- Measured contrast, tested interactions, and manual-check availability are recorded
  separately; a code inspection is not claimed as a screen-reader or screenshot pass.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use `DesktopAccessibilityTest`, `DesktopKeyboardNavigationTest`, `DesktopLayoutStateTest`,
and affected interaction tests. Run the current keyboard/native viewport checklist
where supported, recording any missing native or assistive-technology evidence.
Complete/move the task and update the index; no unrequested commit.
