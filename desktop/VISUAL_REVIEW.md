# Dark UI visual correction — 2026-09-04

The user's native Analysis screenshot exposed problems that the initial palette and state tests
did not catch: every navigation item looked like a primary button, preview utilities crowded the
toolbar, rail labels wrapped awkwardly, and small data sets occupied large undifferentiated cards.

The correction uses separate chrome and workflow-action styles, vector line drawings, compact
menus, a coverage/progress layout for Analysis, flat AI Context sections, a Problems table with
expandable details, and lighter source highlighting. Search fields now use compact text input
with an explicit accessible name; form fields retain a visible label when populated. Native
macOS title bars request dark appearance before AWT starts, using the
[JDK appearance property](https://www.formdev.com/flatlaf/macos/#appearance_of_window_title_bars).
Restart the desktop process to load the rebuilt UI and native appearance setting.

## Reproduce the rendered evidence

From the repository root:

```sh
./desktop/gradlew -p desktop spotlessCheck detekt test \
  -PvisualOutput="$PWD/desktop/build/reports/visual-review"
```

The render harness uses the actual production Compose components and Skia, with a project
explicitly labeled `go-shop · fixture` and a `Visual fixture · no backend` status strip. It does
not connect to a daemon or provider. The test-only scene adapter is tied to the pinned Compose
version and is not part of the application.

Reviewed captures:

- Analysis: `1440x900`, `1000x760`, `999x760`, `800x650`, and `1000x800` at 130% text scale.
- Editor/tree/AI Context/Problems: `1440x900`, `1000x760`, and `999x760`.
- Rendered interaction checks cover Preview menu/dialog isolation, finding-details selection
  before workflow actions, compact field editing/naming, and essential chrome label layout.

The desktop formatting, static-analysis, and full test suite passed. These changes are confined
to `desktop/`; daemon checks were not repeated for this visual correction. No API, configuration,
or pane-preference migration is required.

The captures are offscreen component fixtures, not live OS-window screenshots. They verify
Compose layout and reveal clipping, but do not establish native title-bar, screen-reader,
provider-flow, or complete visual acceptance for Task 149. Those checks remain manual.

## UI refinement component verification — Task 159

Task 159 reran the production-component scene at `1440x900`, `1920x1080`,
`1000x760`, `999x760`, and `800x650`, with affected controls at 130% text scale.
The matrix includes Summary, Analysis, Editor/Review, Performance, Problems,
Assistant, a direct production menu surface, disclosures, tool-window controls,
and the keyboard-focused rail. It includes empty, paused, failed, stale,
populated, expanded, long-value, and disabled states. The fixture now clears its
raster canvas before every frame; this prevents pixels from a collapsed lazy-list
state appearing in a later expanded-state capture.

Run:

```sh
./desktop/gradlew -p desktop spotlessCheck detekt test \
  -PvisualOutput="$PWD/desktop/build/reports/ui-refinement/after"
```

Reviewed outputs include `summary-dashboard-1440.png`,
`summary-dashboard-800-1.3.png`, `analysis-1440-1.0.png`,
`analysis-1920-1.0.png`, `analysis-1000-1.0.png`, `analysis-999-1.0.png`,
`analysis-800-1.0.png`, `analysis-long-destination-1000-1.3.png`,
`performance-populated-1440.png`, `performance-empty-800-1.3.png`,
`bugs-failed-800-1.3.png`, `editor-candidate-800-1.3.png`,
`review-failed-800-1.3.png`, `assistant-stale-800-1.3.png`,
`popup-surface-320-1.3.png`, `findings-filters-expanded-480-1.3.png`,
`tool-window-controls-360-1.3.png`, and `rail-keyboard-arrow-120-1.3.png`.

`DropdownMenu` and `AlertDialog` use a Desktop window layer that the offscreen
scene cannot paint. Their trigger, dismiss, focus, and local-only behavior are
tested through production Compose semantics, but native popup/dialog placement is
not inferred from a blank offscreen layer. It remains an explicit release check in
[KEYBOARD_SMOKE_CHECKLIST.md](KEYBOARD_SMOKE_CHECKLIST.md) and
[UI_REFINEMENT_ACCEPTANCE.md](UI_REFINEMENT_ACCEPTANCE.md).
