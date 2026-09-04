# UI refinement acceptance record

**Recorded:** 2026-09-04
**Current result:** Task 159 component, responsive, semantic, and interaction
acceptance passed. Native-window and screen-reader acceptance remains a release
follow-up; this record does not treat it as passed. Task 160 completes the
repository-wide and sequence audit.

## Requirement-to-evidence matrix

| Requirement | Automated/component evidence | Outcome |
| --- | --- | --- |
| Responsive production surfaces | `DesktopVisualLayoutTest` renders Summary, Analysis, Editor, Performance, Problems, Assistant, Review, menus, disclosures, drawers, and bottom overlay at `1440x900`, `1920x1080`, exactly `1000x760`, `999x760`, `800x650`, and 130% text scale where the surface is affected. `DesktopLayoutStateTest` verifies the exact docked/drawer boundary, pane clamping, resizing, and preference round-trips. | Passed. |
| Bounds, long content, and reachability | The render fixture asserts visible text layouts are not ellipsized or vertically clipped, checks long provider/menu text wraps, and detects scroll semantics. Captures cover empty, paused, failed, stale, populated, expanded, and disabled states. | Passed for the rendered Compose components. |
| Keyboard interaction and focus | `DesktopVisualLayoutTest.keyboardEventsNavigateAndActivateTheProductionRailAndCommandPalette` sends actual Compose key events for rail arrow/Enter and palette arrow/Enter. The same suite sends Enter/Space to disclosures and Escape to the Preview menu. `DesktopKeyboardNavigationTest` covers tab-group routing, topmost transient-surface priority, focus-restoration target selection, workspace changes, and responsive regions. | Passed in the Compose test scene. |
| Accessible state and contrast | `DesktopAccessibilityTest`, `DesktopKeyboardNavigationTest`, and component semantics tests cover names and textual selected/focused/expanded/disabled/read-only state. `DesktopThemeTest` checks required text, focus, selected, disabled, and diff contrast pairs. | Passed for Compose semantics and token contrast. |
| Preview and disclosure isolation | Visual interaction tests open/close Preview/menu/dialog and findings/disclosure content without workflow callbacks; `PreviewFeatureTest` retains the explicitly local-only contract. | Passed. |
| Source/diff and review safety | `EditorInspectionStateTest`, `DiffViewerTest`, `ReviewEvidencePaneTest`, `DesktopIntegrationCoverageTest`, and review visual states preserve selectable source identity, read-only evidence, current validation/check identity, guarded Apply, receipt, and Undo. | Passed. |
| Native window, keyboard, and screen reader | The 2026-09-04 desktop-surface inventory contained no Mini-Orca application (`apps: []`) and only the Codex in-app browser. No Mini-Orca native window, screen-reader output, or native keyboard path could be exercised. | Unavailable — not a pass. |

## Reviewed production-component renders

The Task 159 verification command writes ignored test output under
`desktop/build/reports/ui-refinement/after/`. Reviewed representative captures
include `summary-dashboard-1440.png`, `summary-dashboard-800-1.3.png`,
`analysis-1440-1.0.png`, `analysis-1920-1.0.png`, `analysis-1000-1.0.png`,
`analysis-999-1.0.png`, `analysis-800-1.0.png`,
`analysis-long-destination-1000-1.3.png`, `performance-populated-1440.png`,
`performance-empty-800-1.3.png`, `bugs-failed-800-1.3.png`,
`editor-candidate-800-1.3.png`, `review-failed-800-1.3.png`,
`assistant-stale-800-1.3.png`, `popup-surface-320-1.3.png`,
`findings-filters-expanded-480-1.3.png`, `tool-window-controls-360-1.3.png`, and
`rail-keyboard-arrow-120-1.3.png`. They are offscreen Compose/Skia renders of
production components with fixture-only data; they are not native screenshots.

The command-palette dialog, Preview dialog, project/Preview dropdown triggers, and
bottom overlay are interaction/semantics-tested in the offscreen scene. Compose
Desktop's `DropdownMenu` and `AlertDialog` window layers are not painted into this
scene, so their visual placement is intentionally left to the native release
follow-up rather than claimed from blank layer captures. The directly rendered
`IdePopupMenuSurface` capture verifies the production menu surface itself.

## Native release follow-ups

On a running Mini-Orca desktop build with an accessible fixture project, a release
operator must:

1. Capture native windows at `1440x900`, `1920x1080`, `1000x760`, `999x760`, and
   `800x650`, then repeat the affected matrix at 130% text scale. Include long
   project/file/model names and Summary, Analysis, Editor/Review, Performance,
   Problems, menus, disclosures, drawers, overlays, and dialogs.
2. Complete [KEYBOARD_SMOKE_CHECKLIST.md](KEYBOARD_SMOKE_CHECKLIST.md) using
   keyboard-only navigation: Tab order, arrows, Enter/Space, Escape’s topmost
   surface, focus restoration, workspace changes, drawer behavior, resizing, and
   retained pane preferences.
3. Use the supported screen reader to verify names, selected/expanded/disabled
   state, textual status, source/diff read-only semantics, and visible focus.

Any observed native defect blocks release acceptance and must be fixed in its
owning component. These follow-ups do not alter Task 149's independent Pending
status.
