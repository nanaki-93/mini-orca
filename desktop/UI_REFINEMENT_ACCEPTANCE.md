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

## Final Task 160 audit

Tasks 150–159 each have one reviewed local commit in numeric order. Task 160
audited the range from the Task 150 baseline through Task 159, including active
desktop sources, tests, task records, and current documentation. The active rail
contains only Summary, Analysis, Performance, Bugs & Problems, and Editor;
`LeftToolWindow.Project` and any duplicate Project workspace route are absent.
The Project toolbar menu, project open/re-index flows, Files tree/drawer, and
stored-preference recovery remain the supported project boundary.

The replacement has one token-driven Compose implementation: shared popup and
disclosure surfaces, concise Summary/Analysis/workspace presentation, and the
existing guarded review workflow. No parallel shell, legacy theme/menu/Summary
branch, fixture-only production path, debug output, or generated capture is part
of the delivery. Source and diff remain selectable/read-only; Preview stays local;
remote consent, candidate evidence, validation/check identity, guarded Apply,
receipt, and Undo remain explicit. There is no daemon, API, configuration, or
saved-pane-preference migration.

The component-library decision remains [UI_COMPONENT_DECISION.md](UI_COMPONENT_DECISION.md):
Jewel is not compatible with the pinned Compose 1.7.0 toolchain without a broader
migration, so the focused token-styled Compose components are the supported
replacement.

### Before/after conclusion

The Task 150 baseline retained a duplicate Project rail destination, separate
popup/disclosure treatments, a wide fixed Analysis presentation, a stacked Summary,
and routine repeated narration. The delivered shell has one Editor source route,
shared token-styled chrome, available-width Analysis/Summary layout, compact
expandable project interpretation, concise workspace copy, and an evidence-first
Editor/Review surface. The existing project operations and preview-first mutation
boundary were preserved rather than replaced.

### Delivery ledger through Task 159

| Task | Commit |
| --- | --- |
| 150 | `355e5b390c4d3ccf425c5f96c567d3aac647431a` |
| 151 | `e2f601b62e8f4c2442a6041498f5c8ddb822f5b2` |
| 152 | `b55eb06a4eba5d73bad44257782698740eece32f` |
| 153 | `267d69ca630018eb5f29e7c05bb9910c2a65f683` |
| 154 | `14ff9eda74803748a51ade0a32d38b429bb308b7` |
| 155 | `82bb62557d4df6c7d49500b014072cc31d0a5528` |
| 156 | `a54accd6c9f83a935b81e22f87b50cdccb41110c` |
| 157 | `9b806f935d14a353a73d45e0d85051e7d20bb7d0` |
| 158 | `cede87e58b5134c7c9d838684a8ddc431a9c68d2` |
| 159 | `ba545a9c97046c14fecb57dd1e239e6b4f8e3865` |

### Final verification

| Check | Result |
| --- | --- |
| `./desktop/gradlew -p desktop spotlessCheck detekt test` | Passed. |
| `make check` | Passed: Go formatting, tests, race tests, vet, and desktop tests. |
| `make quality` | Not passed. It stops at the unchanged `gocyclo -over 15` findings: `(*Service).reviewPerformanceFile` (19), `validPerformanceJob` (19), `validPerformanceFinding` (18), `(*Service).StartPerformanceJob` (18), and `(*Service).StartAnalyzeAll` (16). This exactly matches the Task 150 baseline; no Go file in this UI sequence was changed, and the later clone check did not run. |
| `git diff --check` | Passed before the Task 160 commit. |

Automated and rendered-component acceptance is complete. This is not full native
or release acceptance: the native-window, OS keyboard, screen-reader, popup/dialog
placement, and provider-backed end-to-end checks above remain release follow-ups.
Task 149 remains independently Pending.
