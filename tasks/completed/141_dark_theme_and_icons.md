# 141 — Implement the charcoal theme and icon system

## Status

Complete

## Depends on

Task 140.

## Goal

Replace the purple Focus Flow appearance with the charcoal-and-blue visual system
defined in the [plan](../docs/dark-ui/PLAN.md#visual-system).

## Implementation

- Replace semantic palette values in `DesktopTheme.kt`, including syntax and diff
  backgrounds. Separate primary-button fill from light blue link/selection text.
- Standardize type, minimum control heights, spacing, corners, border, hover,
  selected, pressed, disabled, and cyan keyboard-focus treatment.
- Keep system UI/monospace fonts and the existing Mini-Orca mark. Add only the small
  native vector set needed for navigation, file types, and toolbar actions in a
  cohesive `DesktopIcons.kt` if existing Compose vectors are insufficient.
- Add a shared Preview badge and accessible local popover in a small
  `PreviewFeature.kt`; expose only local presentation callbacks, with no presenter
  or API dependency. Feature-specific content belongs to its owning pane.
- Update the exact old-palette assertions and obsolete Focus Flow comments. Remove
  redundant styling once replaced; do not keep a second palette or theme toggle.
- Verify final color combinations against the plan's contrast targets, including
  blended hover/selection colors and readable added/removed diff text.

## Likely files

`desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt`, `DesktopIcons.kt`,
`PreviewFeature.kt`, `DiffViewer.kt`, and affected theme/presentation tests.

## Acceptance criteria

- Every shared control inherits the new dark system; no screen switches to light colors.
- A compact line icon, visible/accessible label, and Preview state can be reused
  without network assets or another UI framework.
- Focus remains distinct from selection, and states remain legible without color.
- Syntax styling leaves source bytes/text unchanged; diff colors retain clear +/- cues.
- Preview activation can only show local explanatory UI and restores focus on close.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use `DesktopThemeTest` and `DiffViewerTest` for affected behavior; add Preview
interaction tests at the narrow local boundary. Document measured color pairs.
Complete/move the task and update the index after verification; no unrequested commit.
