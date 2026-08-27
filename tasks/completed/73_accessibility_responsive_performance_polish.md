# 73 — Complete accessibility, responsive, and performance polish

## Status

Complete

## Goal

Prove the redesigned Desktop is usable without a mouse, without color, at supported
window widths, with long content, and with realistic project sizes.

## Depends on

Task 72.

## Implementation

- Verify focus order: top bar/rail → explorer → stage canvas → contextual panel.
- Preserve `⌘1`–`⌘4`, `⌘Tab`, `⌘P`, `⌘⇧O`, `⌘K`, `⌘Enter`, `⌘⇧F`,
  `⌘⇧D`, `⌘⇧V`, `⌘⇧C`, and Escape behavior.
- Add/verify roles, content descriptions, selected/disabled state, textual badges,
  tooltips, focus rings, and stage/diff semantics.
- Exercise wide layout, exactly 1000dp, and below 1000dp; fix clipping, focus loss,
  drawer behavior, long paths/symbols, and text scaling.
- Audit large explorer, finding, and analysis lists for lazy rendering/recomposition costs.
- Keep source/diff selectable and read-only after any performance optimization.
- Update the keyboard smoke checklist to the four-stage Editor flow.

## Acceptance criteria

- The full Target → Draft → Verify → Apply → Undo path is keyboard-operable.
- Every critical state remains understandable without color or icon recognition.
- No horizontal clipping or inaccessible action occurs at supported breakpoints.
- Large project/finding/job views remain responsive and retain correct selection.

## Verification

- Run Desktop accessibility/semantics, shell, explorer, and integration tests.
- Run the updated manual keyboard/responsive smoke checklist.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
