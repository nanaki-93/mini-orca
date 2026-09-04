# Dark UI acceptance record

**Recorded:** 2026-09-04  
**Result:** Desktop implementation checks passed. Repository-wide quality and native
visual acceptance are pending, so Task 149 remains Pending.

## Delivered coverage

Tasks 140–148 replace the desktop presentation with a shared charcoal semantic
theme, custom line-icon system, 88dp labeled activity rail, 52dp header, compact
Explorer/editor chrome, AI Context hierarchy, candidate summary, and restyled
bottom tools and status bar. The docked layout uses new 256dp Explorer, 344dp AI
Context, and 220dp bottom-panel defaults while retaining the existing limits and
stored preferences. At the 1000dp boundary it temporarily clamps the docked panes
to keep a 360dp editor; below the boundary it uses the existing labeled drawers and
bottom overlay.

The remaining workspaces retain their existing workflows. Source and diff views are
read-only, provider confirmation remains explicit, and candidate review remains
the path to guarded Apply, receipt, and Undo.

## Live and preview verification

Live controls retain their existing state and callbacks: project/tree selection,
source selection, file and symbol command palette search, daemon and Git status,
context analysis, scoped assistant composition, candidate review, findings,
checks/output, and guarded Apply/Undo.

New unsupported IDE-like controls are visibly marked **Preview** and are local-only:
New file, branch actions, content search, settings/help, extra tabs, Run/Debug,
minimap, feedback, complexity/readability, Generate unit test, and Terminal. Their
dialogs explain the limitation; they do not call the daemon, mutate project files,
launch a process, change Git state, or persist feedback.

## Automated evidence

The following checks passed after the redesign:

- `./desktop/gradlew -p desktop spotlessCheck detekt test`
- `make check`
- `git diff --check`

`make quality` did not pass because its existing Go complexity gate reports these
unrelated functions: `(*Service).reviewPerformanceFile`, `validPerformanceJob`,
`validPerformanceFinding`, `(*Service).StartPerformanceJob`, and
`(*Service).StartAnalyzeAll`. The dark-UI work does not modify those Go files, so
this presentation task does not change their complexity merely to make the
repository-wide quality target green.

The focused desktop tests cover the palette and contrast boundary, icon identity,
preview local-only behavior, candidate summary visibility, docked width clamping and
preference restoration, exact `999dp`/`1000dp` layout behavior, labels, keyboard
navigation, and accessibility semantics.

## Native visual and assistive-technology evidence

No real Mini-Orca desktop window and project fixture were available in this
environment. Consequently, no before/after captures were created under
`design/ui-mocks/dark-ui/`, and the 1440x900, 1280x800, 1000dp, and 999dp native
viewport comparisons were not performed. Native keyboard smoke testing and
screen-reader inspection were also unavailable.

These checks are deliberately not inferred from tokens, source inspection, or unit
tests. They remain the outstanding work for Task 149; this record does not claim
full visual acceptance.

## Scope and migration

Changes are confined to the Compose Desktop presentation, tests, and dark-UI
documentation. There is no daemon/API/configuration migration and no new
source-mutation capability. Existing persisted pane preferences remain valid.
