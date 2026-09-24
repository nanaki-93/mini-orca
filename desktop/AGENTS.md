# Desktop client

Read [../AGENTS.md](../AGENTS.md) first. Before UI changes, read
[UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) and inspect the relevant existing
components and state owners. [README.md](README.md) owns desktop setup and use.

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

- Follow the [UI guidelines](UI_DESIGN_GUIDELINES.md) for interaction,
  accessibility and verification. Copy, palette, shapes, layout and navigation
  may be redesigned; previous mocks and theme tokens are not required targets.
- Coordinate changed components with the current client and state owners. Keep
  valid actions reachable, errors visible and details available. Do not add
  fabricated metrics, dummy callbacks or unsupported controls.
- Preserve labeled states for empty/loading/stale/failed/partial/canceled results.
  Keep unknown values distinct from zero and provider status distinct from daemon
  connectivity. Color supplements labels and keyboard focus.
- Choose responsive behavior for the new design. Preserve saved pane preferences
  where applicable and keep long paths, larger text and actions accessible.
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
them to the chosen design. Follow the UI guidelines' viewport, text-scale
and state checks, including the review with optional help collapsed. Use the
existing `DesktopVisualLayoutTest` fixtures.
Use [KEYBOARD_SMOKE_CHECKLIST.md](KEYBOARD_SMOKE_CHECKLIST.md) for affected native
behavior. Report offscreen rendering and native-window evidence separately;
component tests cannot prove OS focus, popup placement or screen-reader behavior.

For terminal/runtime/packaging changes, run the relevant documented distribution
and native smoke checks. Record unavailable platform checks accurately; do not
claim packaging or native acceptance from unit tests alone.
