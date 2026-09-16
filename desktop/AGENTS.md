# Desktop client

Read [../AGENTS.md](../AGENTS.md) first. Before UI changes, read
[UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) and inspect the relevant existing
components and visual reference. [README.md](README.md) owns desktop setup and use.

## State and workflow ownership

- Keep desktop implementation in `desktop/`. Compose components render state and
  emit user actions; `DesktopWorkflowPresenter`, the workflow owners and controller
  own daemon interactions and transitions. Keep visual-only state local.
- Use `ApiClient` and existing serializable models for daemon requests. Preserve
  field names, optional-value semantics, structured failures and request guards.
  A changed daemon contract requires coordinated backend/docs/tests, not a
  client-only assumption or permissive decoding workaround.
- Keep snapshots immutable and reuse project/file/draft/run identities. Reject
  stale responses after navigation, edits, project switches or a replacement job.
  Cancellation alone does not establish that an old response cannot arrive.
- Own coroutines and polling through existing lifecycles. Keep blocking I/O off
  the UI thread, propagate `CancellationException` and dispose resources on close.
  Do not start network calls from composition or restart them on recomposition.
- Reuse the existing eligibility/state owners. Presentation helpers may explain
  a blocked action but must not independently recreate daemon business rules.
- Preserve explicit draft discard, evidence invalidation after edits and guarded
  Review/Apply/Undo. Navigation, tabs and disclosures remain local interactions.
  Only the isolated declaration/import draft is editable; source/diff stay read-only.

## UI implementation

- Apply the [self-explanatory UI and copy rules](UI_DESIGN_GUIDELINES.md#self-explanatory-ui-and-copy)
  before adding text. Let existing tabs, headers and controls identify the task;
  omit default subtitles and repeated explanations. If a control needs teaching
  copy, improve its label, placement or interaction first.
- Extend Jewel and `DesktopTheme.kt`, `ChromeControls.kt`, `DesktopIcons.kt`.
  Use shared semantic colors, type and spacing. Do not add a parallel theme,
  copied per-pane styles, stock Material/Swing appearance or decorative card stacks.
  Use the shared 10dp control, 14dp section/card and 18dp workspace/overlay shapes,
  with pill badges and progress tracks. Keep structural panes flat, clipped and
  inset within the frame; follow the approved mockup geometry in the UI guidelines.
- Preserve workspace/tool-window ownership in the UI guidelines. Keep the next
  valid action reachable, errors visible and technical details available on demand.
  Do not add decorative metrics, dummy callbacks or unsupported controls.
- Preserve labeled states for empty/loading/stale/failed/partial/canceled results.
  Keep unknown values distinct from zero and provider status distinct from daemon
  connectivity. Color supplements labels and keyboard focus.
- Keep docked panes at widths of at least 1000dp and labeled drawers below 1000dp.
  Temporary size clamping must preserve saved preferences. Support long paths,
  wrapped labels and increased text scale without hiding actions or shrinking text.
- Preserve keyboard access, accessible names/states, selection and focus. Reuse
  `ModelResultContent` for freeform model prose and its bounded literal fallback;
  do not activate model-supplied HTML, links or remote images.
- For terminal work, follow [TERMINAL.md](TERMINAL.md). Preserve shell ownership,
  close/project-switch behavior, terminal keystrokes and source freshness checks
  on return. Do not persist transcripts or send them to a provider.

## Build and verification

Use `./scripts/desktop-gradle.sh` from the repository root; it invokes the checked-in
Gradle wrapper with the documented Java 21 launcher and Java/JBR 25 toolchain.
Do not require global Gradle, commit machine paths or change system JDKs. Keep the
existing Kotlin/JVM targets and pinned versions unless an upgrade is requested.

Run focused tests while iterating, then
`./scripts/desktop-gradle.sh test spotlessCheck detekt` for desktop code/build
changes. Format changed Kotlin using the configured Spotless/ktfmt rules; inspect
formatter output for unrelated churn. Do not change lint configuration or snapshots
merely to hide a regression.

For substantive visual changes, render affected production components and compare
them to the reference. Follow the UI guidelines' width, short-window, text-scale
and state checks, including the review with optional help collapsed. Use the
existing `DesktopVisualLayoutTest` fixtures and
[reproduction procedure](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks).
Use [KEYBOARD_SMOKE_CHECKLIST.md](KEYBOARD_SMOKE_CHECKLIST.md) for affected native
behavior. Report offscreen rendering and native-window evidence separately;
component tests cannot prove OS focus, popup placement or screen-reader behavior.

For terminal/runtime/packaging changes, run the relevant documented distribution
and native smoke checks. Record unavailable platform checks accurately; do not
claim packaging or native acceptance from unit tests alone.
