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
