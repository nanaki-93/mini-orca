# Mini-Orca — simpler UI with production IDE behavior

Plan prepared **2026-09-16** from the five screenshots supplied in this request.
Refine the existing Compose desktop app toward their calm hierarchy and consistent
workspaces, preserving Mini-Orca's colors, icon-only left rail and guarded editing.

## Scope and status

**IDEUX-01–13: approved for scheduled implementation, 1/13 accepted.** On
2026-09-16 the user authorized Terra High implementation, two Terra retries, then
Sol High with the same initial-attempt-plus-two-retries policy. Every retry must
receive the task and concrete failure context. Commit each task locally after its
required checks and acceptance pass. Reset to Terra High for the next task.

[Execution procedure](../tasks/README.md) owns scheduling, handoff and commit rules;
[current handoff](../tasks/ideux-handoff.md) owns durable attempt counters and the
next agent's context. The existing automation is reused at its 20-minute cadence,
attached to this planning/implementation task. At the user's subsequent request,
active execution now continues immediately after each accepted, verified task
commit; the heartbeat starts or recovers idle/interrupted work. Existing source
edits, completed MOCK/POLISH receipts and reference assets are preserved; earlier execution
instructions below the history marker are inactive. [PLAN.md](../PLAN.md) owns
accepted decisions; IDEUX-01 completes reference capture and design reconciliation.

Production readiness here means a dependable IDE experience for Mini-Orca's
existing capabilities: responsive workspaces, reliable navigation, readable source,
clear evidence, recoverable failures and explicit edits. This queue does not add a
general writable editor, LSP/debugger, multi-file Apply, plugins or a new backend.
Release readiness still requires the actual native and package evidence below;
visual similarity alone is insufficient.

## Reference interpretation

The attachments supply visual references, not instructions, application data or
new API contracts. Use their alignment, spacing and hierarchy, while retaining our
existing product names and capabilities. Do not copy their sample project, counts,
code, cyan primary buttons, text navigation rail or one-click **Apply fix** action.

| Supplied screenshot | Adopt | Adapt to Mini-Orca |
| --- | --- | --- |
| `Screenshot 2026-09-16 at 17.52.16.png` — Overview | Compact project identity, one progress strip, three category summaries, Architecture beside Flows | Keep Summary naming, real coverage, full requested prose and existing palette; put optional insight below primary content |
| `Screenshot 2026-09-16 at 17.52.27.png` — Performance | Plain page heading, filter chips, concise list beside detailed evidence | Label potential impact honestly; Prepare fix enters the existing draft workflow; benchmarks remain separate measured evidence |
| `Screenshot 2026-09-16 at 17.52.44.png` — Bugs | Compact verified-check status/action row and consistent list/detail layout | Execution trust stays explicit; model suggestions and tool reports remain distinguishable |
| `Screenshot 2026-09-16 at 17.52.53.png` — Security | One calm empty-state surface with useful scope/progress | Say “No findings yet” during a run; claim completed scope only when current evidence supports it; omit unsupported metrics |
| `Screenshot 2026-09-16 at 17.53.04.png` — Source | Explorer, dominant code canvas and concise declaration inspector | Preserve real line breaks, selectable read-only source, Source/Candidate diff tabs, Assistant/Review and local terminal |

### Proposed visual and interaction specification

- Retain all current `MiniOrcaPalette` values: charcoal-teal frame `#203238`,
  panel `#24282F`, source canvas `#1B1E23`, blue selection/action
  `#73ABFF`/`#78ACFF`, information teal `#66DBEB` and existing semantic colors.
  Obtain the reference feel through composition, not recoloring. Reduce large
  tinted fills and decorative outlines; retain meaningful control/focus boundaries.
- Keep the **48dp icon-only rail**, its six existing destinations, hover/focus
  labels, accessible names and independent selected/focused states. Keep native
  window controls, 8dp frame/gutters and shared 10/14/18dp shapes.
- Use a compact 56dp-minimum toolbar: project/branch, a working search entry,
  then separate analysis and daemon states. Prefer plain labeled status indicators
  over multiple heavy badges. Project context takes priority over repeated branding.
- Use 24dp wide / 16dp compact page insets, 16dp major gaps and shared typography:
  24sp page identity, 16sp section/finding headings, 14sp body, 13sp metadata.
  Source retains readable monospace. These are base sizes, not clipping limits;
  content grows at 125/150% text scale.
- Summary: active-run strip when relevant; project/purpose/metadata; three equal
  neutral category surfaces with small semantic accents; Architecture and Flows
  side by side. Modules and engineering insight stay available below these primary
  sections. Coverage remains visible without another large dashboard card.
- Results: one unboxed heading with count/state and View analysis; filters below;
  a roughly 42/58 list/detail split when both remain readable. Each row shows a
  severity/impact label, title and location. Put the full explanation in detail.
  Selection uses our blue treatment; severity never doubles as selection color.
- Source: Explorer / code / Context remain resizable. Explain and Refactor stay
  explicit; New function has one clear file-scoped entry. Review retains the exact
  target, readiness, evidence, draft action and guarded Apply/Undo. Terminal keeps
  its existing collapsed/docked/overlay behavior.
- Use short action labels and one clear primary action per relevant panel. Keep
  errors, execution trust, consent, evidence origin and stale state visible where
  needed to decide. Optional metadata/logs live in existing disclosures. Do not
  collapse user-requested explanations merely to make a screenshot look sparse.

### Repository findings that drive the work

- `DesktopTheme.kt`, `IdeShell.kt` and the current uncommitted MOCK changes already
  supply the palette, icon rail, rounded frame, pane resizing and large-text work.
  Extend them; rebuilding the shell would duplicate accepted work.
- `MainToolbar` advertises files/symbols/commands, but `DesktopShell` opens
  `PaletteMode.Actions`; `CommandPaletteDialog` has no mode switch. Symbols are
  currently scoped to the active file. IDEUX-03 closes this observable mismatch.
- `ProjectSummaryPane` currently gives coverage a full card and places engineering
  insight before Flows. `AnalysisRunPanel` repeats a circular and linear progress
  treatment. IDEUX-04/05 simplify hierarchy while retaining their state owners.
- `AnalysisResultsPane` already shares list/detail composition, but selection is
  local to its composition and there are no severity/impact filters.
  `ResultSectionHeader` adds another boxed icon header. IDEUX-06/07 refine these
  maintained components rather than introducing three independent page systems.
- Empty result copy is caller-supplied and generic; an empty list can coexist with
  running, failed, stale or loading state. Counts arrive separately from details.
  `AnalysisResultPageState`, section coverage and run identity must drive IDEUX-08.
- Bugs hides its only scan action inside a disclosure. Several detail views hide
  model-versus-tool origin inside technical evidence. IDEUX-09–11 expose only the
  decision-critical distinctions and retain the existing eligibility owners.
- `SourceEditorPane`, `ContextToolWindow`, `ReviewToolWindow`, presenter navigation
  and terminal ownership already implement the deliberate edit workflow. IDEUX-12
  polishes that path; it does not replace it with the reference's direct Apply.

## Implementation and acceptance rules

- Run one ordered card attempt at a time using the current execution procedure.
  Terra High gets the initial attempt and retries 1/2 and 2/2; after exhaustion,
  Sol High gets its initial attempt and retries 1/2 and 2/2. Pause after the final
  Sol failure. Interruption resumes the same attempt; it does not reset counters.
  A failure must be recorded before dispatching another agent. The execution
  handoff, failure log, PLAN status and this ledger may be updated for every card.
- Review and commit each accepted task before starting the next. This current
  user grant supersedes older no-commit and model restrictions. It does not permit
  pushes, publication, live provider campaigns or unrelated changes.
- Immediately dispatch the next incomplete card after the verified commit and
  handoff update. A failed attempt similarly dispatches its next permitted fresh
  retry after recording the full failure packet and confirming the prior worker
  stopped. Keep one writer and continue until completion, exhausted retries,
  a genuine external blocker or an explicit user pause.
- Every card starts from the current working tree and applicable area guide.
  Preserve overlapping edits and historical receipts. No toolchain/dependency
  upgrade, new service, fabricated production data or API extension is planned.
- Components emit intents; `DesktopWorkflowPresenter` and existing analysis,
  benchmark, security and draft owners retain domain rules and side effects.
  Opening a page, selecting/filtering a result, changing a search mode or restoring
  local UI state must not invoke a provider, execute project code or write source.
  Ordinary daemon reads for source/navigation remain permitted.
- Scope identity includes project/revision/run where relevant. Retained evidence
  stays inspectable with its actual stale/error state. Unknown is not zero;
  processed is not successful; connected daemon is not healthy provider.
- Each implementation card includes focused behavioral checks and visual inspection
  of affected production components. Then run
  `./scripts/desktop-gradle.sh test spotlessCheck detekt` for Kotlin changes and
  `git diff --check`. Commands below are planned verification, not recorded passes.
- Extend existing deterministic fixtures and tests. Exercise controls, state changes,
  cancellation/stale results and meaningful boundaries; do not test only constants
  or bless new screenshots. Generated captures stay under ignored build reports.
- IDEUX-13 is the final acceptance gate. Until it passes, call the work implemented
  or partially validated as appropriate, not production-ready or visually accepted.

## Task IDEUX-01 — Preserve the references and reconcile the design specification

**Status:** [x] Accepted — 2026-09-16; Terra High initial attempt; 0 retries.

**Target files**

- `design/ui-mocks/ide-reference-2026-09-16/01-overview.png` — new, unmodified copy of attachment 17.52.16.
- `design/ui-mocks/ide-reference-2026-09-16/02-performance.png` — new, unmodified copy of attachment 17.52.27.
- `design/ui-mocks/ide-reference-2026-09-16/03-bugs.png` — new, unmodified copy of attachment 17.52.44.
- `design/ui-mocks/ide-reference-2026-09-16/04-security.png` — new, unmodified copy of attachment 17.52.53.
- `design/ui-mocks/ide-reference-2026-09-16/05-source.png` — new, unmodified copy of attachment 17.53.04.
- `PLAN.md` — record this scope and the retained versus changed design decisions.
- `desktop/UI_DESIGN_GUIDELINES.md` — update reference links and the hierarchy rules above.
- `tasks/README.md` — retain the current authorized scheduler/retry/commit procedure and distinguish it from historical grants.
- `docs/tasks.md` — record reference availability and later implementation status.

**Inputs / dependencies**

- This user request, its five original attachments and the current uncommitted
  implementation. No dependency on an old scheduler or completed queue.

**Implementation rules**

- Inspect all five original images and preserve them as immutable references.
  Keep older mockups and acceptance receipts intact. If originals are unavailable,
  record that asset dependency; do not substitute generated approximations.
- Reconcile the previous rounded design with the specification above. Keep all
  colors, icon navigation and source-safety boundaries explicit in the owning docs.
- Capture existing production Summary, Analysis, results and Source/Review before
  visual changes, using maintained fixtures. Label these as baseline evidence,
  not acceptance of this queue. Inspect reference and baseline side by side.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --rerun-tasks --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/before"
git diff --check
```

Verify local links and the five asset copies against their originals.

### IDEUX-01 acceptance evidence — 2026-09-16

All five supplied originals were available, inspected and copied byte-for-byte to
the target paths. SHA-256 values: `01-overview` `eaf2148e366600936b5afe4f7d9bc4aa739914b9a64dd847d03a55ba05f95a47`;
`02-performance` `d9142f6ba7f715aeec7d44f345609e90f31b49b7eb290c69237f8ecbe9317751`;
`03-bugs` `5ab47b01160482f97649cdeeebd06feee2547a7db8de8e02f13a02341ca8f33c`;
`04-security` `a53a373c1e7e57c07a1267d76fd13d6dfa40fb7c62e42d8b5204956b05fa54fc`;
and `05-source` `9d3974878e5e9c725f2fe501a1cde36b74533d31dc8fa33d9fb83ecb626a0b1a`.

`DesktopVisualLayoutTest` rendered the current dirty working-tree baseline before
any IDEUX production change to `desktop/build/reports/ide-ux/before/`. Inspected
Summary, Analysis, Performance detail, Source and Review renders confirm the
existing rounded frame, lifecycle fixtures and guarded edit workflow that
later cards must reconcile with the quieter reference hierarchy. This is baseline
evidence only, not clean-HEAD reproduction, new-UI implementation or acceptance.
The retained implementation already has heavier boxed summary/results treatment,
a prominent analysis progress panel and a full Review readiness region; later cards
will simplify visual hierarchy without replacing the real state owners or guarded
source mutation path.

The card's exact `desktop-gradle.sh` command above exited 0: 51 tests, 0 failures,
0 errors and 0 skipped, recorded in
`desktop/build/test-results/test/TEST-io.miniorca.desktop.DesktopVisualLayoutTest.xml`
(timestamp `2026-09-16T10:40:51.881Z`). `git diff --check` passed. The coordinator
verified the five originals against their copies, current local documentation
links, unchanged historical text and unchanged application files against the
pre-attempt snapshot. No native or new-implementation acceptance is claimed.

Inspected baseline images under `desktop/build/reports/ide-ux/before/`:

- `summary-dashboard-1440-900-1.0.png` and `summary-frame-1600-1000-1.0.png`.
- `analysis-progress-1440-1.0.png` and `analysis-frame-1600-1000-1.0.png`.
- `results-performance-detail-1440-1.0.png`.
- `editor-1440.png`.
- `review-ready-1440-900-1.0.png`.

The task commit includes the new plan/scheduler documentation and preserved
historical documentation already present in the working tree as context for this
reconciliation. The earlier application implementation and older untracked mockups
remain untouched and outside this documentation/reference commit. The verified
commit hash and next attempt are recorded in `tasks/ideux-handoff.md` after commit.

## Task IDEUX-02 — Quiet the shell and establish shared page hierarchy

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — shared page typography and restrained structural surface treatment.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopHeader.kt` — project-first toolbar alignment and lighter labeled statuses.
- `desktop/src/main/kotlin/io/miniorca/desktop/IdeShell.kt` — rail/chrome alignment while preserving resize and terminal geometry.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — integrate the changed toolbar without changing workflow ownership.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — production shell and shared page renders.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopContrastTest.kt` — actual backgrounds, controls and focus contrast.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopShellTest.kt` — toolbar/status and shell behavior regressions.
- `desktop/UI_DESIGN_GUIDELINES.md` — final shared measurements.
- `desktop/UI_CONTRAST.md` — measured contrast for changed treatments.

**Inputs / dependencies**

- IDEUX-01; existing Jewel controls, palette, shapes, layout preferences and rail.

**Implementation rules**

- Keep every palette value. Reduce decorative card/header outlines through shared
  structural variants, without reducing focus or control visibility. Do not change
  every `WorkspaceSection` globally when only page-heading surfaces need to flatten.
- Align project/branch and search consecutively; keep status on the right at wide
  widths and wrap deliberately at narrow widths. Remove redundant toolbar wordmark
  text if it competes with project identity; retain the existing product mark.
- Preserve six icon-only rail destinations with hover/focus labels. Keep status
  labels meaningful when idle, running, stale, disconnected or reconnecting.
- Retain the 1000dp shell dock/drawer boundary, saved widths and terminal inset.
  Controls must remain reachable at 800×650 and 1280×600 with 150% text.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopContrastTest' --tests 'io.miniorca.desktop.DesktopLayoutStateTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/shell"
```

## Task IDEUX-03 — Make the toolbar search reach all three existing modes

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/CommandPalette.kt` — visible Files / Symbols / Commands mode controls and bounded results.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — palette mode/query ownership and explicit switching.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — toolbar launch, mode intents and focus return.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopHeader.kt` — accurate shortcut hint on the search entry.
- `desktop/src/test/kotlin/io/miniorca/desktop/CommandPaletteTest.kt` — mode switching, query, empty and activation behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — retained shortcuts and focus ownership.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — real palette interactions and long-result layout.

**Inputs / dependencies**

- IDEUX-02; `PaletteMode`, typed search results and existing file/action dispatch.

**Implementation rules**

- Toolbar opens Files by default; the palette exposes all three modes without
  requiring knowledge of shortcuts. Existing file/symbol/action shortcuts still
  open their corresponding mode directly. Display only a shortcut that is wired.
- Keep Files project-index scoped and Symbols explicitly labeled as active-file
  symbols. Do not imply project-wide symbol search or add a backend index here.
- Switching modes retains the typed query, resets the highlighted result safely
  and leaves focus in the search field. Opening a fresh palette starts a new query.
- Up/Down, Enter and Escape work consistently; keep highlighted results visible
  in a bounded scrollable area at large text sizes. Empty results never activate.
- Inspection and switching are local. Commands retain their existing preparation,
  scope, consent and trust paths; Enter must not silently bypass admission.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.CommandPaletteTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/search"
```

## Task IDEUX-04 — Recompose Summary around project, categories and flows

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — primary content order and responsive columns.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryVisuals.kt` — compact retained coverage presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryIssues.kt` — neutral category surfaces with restrained semantic accents.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisCategoryPanels.kt` — maintain one shared category component for Summary and Analysis.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryPaneTest.kt` — known/missing/stale content and layout behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryIssuesTest.kt` — current counts and navigation.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — full Summary reference comparison.

**Inputs / dependencies**

- IDEUX-02; current summary projections, category count reconciliation and local
  `MermaidDiagram` disclosure behavior.

**Implementation rules**

- Keep project name, purpose and metadata together. Replace the oversized coverage
  card with a compact coverage row below identity, retaining unknown/stale/failed
  distinctions and View analysis. Do not present coverage as run completion.
- Show three equal neutral category surfaces with a small accent edge, readable
  count/state and whole-surface navigation. Keep existing category color meanings.
- Put Architecture and Flows in balanced columns; move modules and engineering
  insight after these, retaining their full content and existing diagram controls.
  Stack naturally when content width or text scale requires it.
- Remove repeated totals only when they add no distinct fact. Preserve the
  difference between overall tool findings and current-run model counts. Missing
  descriptions do not become invented summaries; no project retains Open project
  through the shell's existing entry point.
- Verify selected-file coverage and category counts while updates arrive in
  different orders. Clicking categories or expanding diagrams remains local.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ProjectSummaryPaneTest' --tests 'io.miniorca.desktop.ProjectSummaryIssuesTest' --tests 'io.miniorca.desktop.MermaidRendererTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/summary"
```

## Task IDEUX-05 — Share one compact run-progress strip

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisRunStrip.kt` — new shared presentation of existing progress and control intents.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — replace redundant progress chrome in Analysis.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — active/paused run strip above project identity.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — supply existing analysis state/actions to Summary.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — reuse or refine progress projection without a second lifecycle rule set.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — lifecycle, count and identity behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — progress strip interactions at wide/compact sizes.

**Inputs / dependencies**

- IDEUX-04; `ProjectRunPresentation`, `AnalysisWorkspaceActions` and current
  admission/pause/resume/cancel owners.

**Implementation rules**

- Show status/current path, finished/total files, one progress track and valid run
  controls in one row when space allows. Wrap rather than clip. Indeterminate
  progress is appropriate when the denominator is unavailable; omit invented ETA.
- Summary shows the strip for active, paused or interrupted current runs; Analysis
  remains the owner of scope selection, start/retry, detailed failures and history.
  Failed/stale state remains visible in Summary coverage after a run stops.
- Reuse existing command eligibility and pending-action disabling. Resume retains
  admission rules. No ambiguous × control: cancellation must be a labeled action.
- Finished files may include failed/partial stages; never label them successful.
  Show multiple active paths compactly with the full list available on demand.
  Old project/run responses must not replace current progress.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.DesktopAnalysisWorkflowTest' --tests 'io.miniorca.desktop.DesktopAnalysisAdmissionTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/progress"
```

## Task IDEUX-06 — Simplify the shared finding list and detail layout

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — flat page heading, concise rows and consistent detail hierarchy.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisResultsPane.kt` — page spacing and bounded header/list/detail regions.
- `desktop/src/test/kotlin/io/miniorca/desktop/FindingsPresentationTest.kt` — preserved evidence text and selected-row meaning.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt` — responsive panes, long errors and scroll behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — all three production result pages.

**Inputs / dependencies**

- IDEUX-02; existing result adapters and one-scroll-owner list/detail components.

**Implementation rules**

- Replace the boxed category icon/header with page title, count/state and trailing
  View analysis. Keep errors visible without consuming most of a short window.
- Rows show severity/impact, title and location, plus material stale/lifecycle
  state. Remove repeated two-line explanation previews; full prose stays in detail.
  Use a slim severity accent with a separate blue selected outline/fill.
- Retain roughly 42/58 panes at sufficient width. Base the list/detail fallback
  on usable content width and text scale, independent of the shell's 1000dp rule.
  Compact mode has an explicit Back to results with focus/scroll restoration.
- Keep one vertical scroll owner per pane; a new selection starts detail at the
  top. No dead action footer or fixed minimum height that hides actions at 150%.
  Preserve selectable model prose and literal fallback behavior.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.FindingsPresentationTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.ModelResultContentTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/results"
```

## Task IDEUX-07 — Add local filters and stable result navigation

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/ResultBrowserState.kt` — new small, typed UI state for filters and selection.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisResultsPane.kt` — filter chips, query field and result selection integration.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — row focus, selection and no-match presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — retain browser state across workspace navigation, scoped to project/category/run.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — pass browser state to Bugs.
- `desktop/src/main/kotlin/io/miniorca/desktop/PerformanceWorkspace.kt` — pass browser state with impact labels.
- `desktop/src/main/kotlin/io/miniorca/desktop/SecurityWorkspace.kt` — pass browser state with severity labels.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultBrowserStateTest.kt` — new filtering, identity and selection transitions.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt` — real filter/back/keyboard interactions.

**Inputs / dependencies**

- IDEUX-06; current stable result keys and producer-specific severity/impact values.

**Implementation rules**

- Provide All and applicable severity/impact chips with local counts plus a compact
  title/path filter. Keep Critical and unknown/other values representable; do not
  silently normalize unknowns to Low. Performance calls its dimension Impact.
- Counts refer to loaded rows before the text filter. Keep them distinct from
  daemon-reported totals when detail loading lags; never manufacture missing rows.
- Filter/query/selected key and list position survive leaving and returning to the
  same workspace during the same project/run. Clear them on project/revision/run
  replacement; no disk persistence or cross-project carryover is needed.
- Keep selection by key when rows reorder. On wide first entry, select the first
  visible result locally; compact mode starts on the list. If a filter or update
  removes selection, select the first remaining row on wide views and return to
  the list on compact views. Empty matches show Clear filters, not “no issues.”
- Arrow keys move row focus and keep it visible; Enter/Space inspect. Filtering,
  automatic detail selection and returning to a page never prepare or apply a fix.
  Exercise several hundred deterministic rows using the existing lazy list.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ResultBrowserStateTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/filters"
```

## Task IDEUX-08 — Make empty results and coverage truthful

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — explicit result availability/empty-state projection from existing data.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisResultsPane.kt` — one full-width state surface when no rows exist.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — status, scope and available recovery presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — remove Bugs' generic empty-message override.
- `desktop/src/main/kotlin/io/miniorca/desktop/PerformanceWorkspace.kt` — consume shared state presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/SecurityWorkspace.kt` — remove unconditional clean-scope empty wording.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — complete/partial/stale/loading/error combinations.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt` — retained rows and empty-state layout.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — Security reference and lifecycle renders.

**Inputs / dependencies**

- IDEUX-07; `AnalysisResultPageState`, `AnalysisSectionProgress`,
  `AnalysisRunCoverage`, matching section identity and current loaded row set.

**Implementation rules**

- Distinguish no project, not started, loading, running with no findings yet,
  paused/interrupted, completed empty, partial, failed, canceled, unavailable,
  stale and filter-no-match. Define the precedence explicitly; a refresh error
  with retained rows keeps the rows and visible error, not a blank success panel.
- Completed-empty wording requires a current completed category, known zero count
  and matching loaded details. Pending details or a positive reported count with
  zero loaded rows cannot become a clean result. Unknown counts display —.
- Show covered/total using the existing category coverage unit and labels. Do not
  call stages “files scanned” or invent the reference's “2/5 checks.” Display elapsed
  time only when available, labeled as run time rather than category scan time.
- One short state and useful next action are sufficient. View analysis returns to
  existing scope/retry controls; navigation never starts analysis or Security review.
  Running zero is “No findings yet”; completed zero is scoped evidence, not a
  blanket “project is secure” claim.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.SecurityWorkspaceTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/states"
```

## Task IDEUX-09 — Make Bugs evidence and verified checks immediately usable

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — verified-check status/action row and Bugs detail hierarchy.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — concise visible evidence origin and action priority.
- `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` — reuse existing classification/lifecycle presentation.
- `desktop/src/test/kotlin/io/miniorca/desktop/BugsWorkspaceStateTest.kt` — classification and scan states.
- `desktop/src/test/kotlin/io/miniorca/desktop/FindingsPresentationTest.kt` — evidence identity and guarded action behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt` — visible action/trust and retained diagnostics.

**Inputs / dependencies**

- IDEUX-08; `classifyFinding`, `findingCanPrepareFix`, scan execution trust and
  existing finding lifecycle actions.

**Implementation rules**

- Keep Verified checks, actual state and its run/cancel action on one compact row.
  Command/output details remain expandable. Running/pausing/canceling disables
  conflicting actions according to existing owners.
- Retain explicit execution trust at the action boundary. Do not replace the
  current trust-and-run behavior with an unqualified one-click Run checks merely
  to copy the screenshot. Compact copy must still identify project-code execution.
- Detail shows whether the selected result is a model suggestion or tool report
  before Prepare fix. Technical provenance remains optional. Do not put every
  source/profile identifier back into each list row.
- Prepare fix stays primary and enters the current declaration task; Dismiss and
  other supported triage stay secondary with real lifecycle/error handling.
  No Apply fix action, fabricated code snippet or new dismissal persistence.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.BugsWorkspaceStateTest' --tests 'io.miniorca.desktop.FindingsPresentationTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/bugs"
```

## Task IDEUX-10 — Clarify Performance recommendations and measured evidence

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/PerformanceWorkspace.kt` — concise recommendation detail and benchmark status placement.
- `desktop/src/test/kotlin/io/miniorca/desktop/PerformanceWorkspaceTest.kt` — recommendation/currentness and measurement states.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt` — compact detail and action reachability.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — reference comparison and benchmark details.

**Inputs / dependencies**

- IDEUX-08 and IDEUX-09; `PerformanceResult`, `performanceCanPrepare` and
  `performanceBenchmarkPresentation` remain the respective domain owners.

**Implementation rules**

- Lead with potential impact, target, observed pattern and recommendation, with a
  concise visible “Model suggestion” identity. Preserve workload/trade-off content
  in the existing disclosure and a visible indication when measurement is absent.
- Make Prepare fix prominent without suggesting any measured speedup. Code blocks
  appear only when actual returned content contains code; never synthesize a fix
  from prose for display. No unsupported Dismiss action for specialized reports.
- Keep benchmark status discoverable above results and measured details in their
  own disclosure. Preserve sample identity, failed/inconclusive/stale outcomes,
  missing metrics and explicit local execution; never turn absent evidence into 0%.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.PerformanceWorkspaceTest' --tests 'io.miniorca.desktop.DesktopBenchmarkWorkflowTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/performance"
```

## Task IDEUX-11 — Clarify Security evidence without implying a clean bill of health

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/SecurityWorkspace.kt` — populated detail, visible evidence kind and remediation hierarchy.
- `desktop/src/test/kotlin/io/miniorca/desktop/SecurityWorkspaceTest.kt` — evidence kinds, source anchors and stale eligibility.
- `desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt` — actual detail/action interactions.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — populated and empty Security production renders.

**Inputs / dependencies**

- IDEUX-08 and IDEUX-09; existing report/hash/source-anchor matching,
  `securityFindingCanPrepareFix` and scope-specific Security review intent.

**Implementation rules**

- Use the shared detail hierarchy: severity, target, visible “Model hypothesis”
  or “Source rule” identity, observed condition, remediation and Prepare fix.
  Essential report warnings remain visible; preconditions and safe-verification
  detail remain available without overwhelming the first view.
- Keep currentness and exact declaration eligibility enforced by existing owners.
  Stale anchors may not prepare a fix. No source-rule match is relabeled as a
  confirmed vulnerability, and an unverified model suspicion remains unverified.
- Use IDEUX-08's scoped zero-result surface. Opening Security, switching filters
  or returning from Editor must not start scanning, execute a payload or request
  remote review. Only expose lifecycle actions already supported for that result.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.SecurityWorkspaceTest' --tests 'io.miniorca.desktop.DesktopSecurityWorkflowTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/security"
```

## Task IDEUX-12 — Make Source and Context feel like one IDE workspace

**Status:** [ ] Pending.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/EditorWorkspace.kt` — compact source identity and unambiguous file/draft actions.
- `desktop/src/main/kotlin/io/miniorca/desktop/SourceEditorPane.kt` — remove duplicate identity chrome while preserving code/gutter geometry.
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — concise declaration inspector and one primary action.
- `desktop/src/main/kotlin/io/miniorca/desktop/ExplorerPane.kt` — aligned file rows, readable paths and selected-file treatment.
- `desktop/src/test/kotlin/io/miniorca/desktop/EditorWorkspaceTest.kt` — source selection, read-only behavior and action scope.
- `desktop/src/test/kotlin/io/miniorca/desktop/ContextToolWindowTest.kt` — explicit explanation/refactor and cached/stale content.
- `desktop/src/test/kotlin/io/miniorca/desktop/ExplorerPaneTest.kt` — file selection/filter/reveal behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — full Source/Context and Candidate/Review captures.

**Inputs / dependencies**

- IDEUX-02 and IDEUX-03; existing Explorer, source viewport, inspector, draft,
  Review/Apply/Undo and shell pane persistence. No editor-engine replacement.

**Implementation rules**

- Keep the central code canvas dominant, with aligned gutters, real line breaks,
  selection, horizontal scroll for long lines and explicit read-only identity.
  Do not reproduce the reference's compressed/wrapped source lines.
- Show file/path/symbol identity once in each context that needs it, removing the
  redundant focused-line sentence when breadcrumbs/inspector already convey it.
  Preserve current-line versus declaration-range highlight and accessible meaning.
- Context starts with declaration name/range and actual cached explanation. Explain
  is prominent when explanation is missing; Refactor becomes primary when a current
  explanation is available. Preserve explicit refresh/cancel, remote consent and
  unsupported-target reasons. Selection alone never requests an explanation.
- Keep New function in one file-scoped action position when the same file is shown
  in Editor and Context; retain access in compact drawers. Do not add dummy tabs,
  writable source, auto-generated explanations or duplicate edit controls.
- Preserve Source/Candidate diff, Edit draft, Assistant and Review transitions,
  explicit discard, evidence invalidation and guarded Apply/Undo. Verify terminal
  return retains the existing source-freshness check and draft identity.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.EditorWorkspaceTest' --tests 'io.miniorca.desktop.ContextToolWindowTest' --tests 'io.miniorca.desktop.ExplorerPaneTest' --tests 'io.miniorca.desktop.DraftReviewWorkflowTest' --tests 'io.miniorca.desktop.ReviewToolWindowTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/source"
```

## Task IDEUX-13 — Verify the complete IDE experience and document actual acceptance

**Status:** [ ] Pending.

**Target files**

- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAcceptanceFixture.kt` — deterministic full-workspace states and native interaction coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — integrated production reference and boundary matrix.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAccessibilityTest.kt` — changed controls' names, state and keyboard semantics.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — end-to-end focus and shortcut regressions.
- `desktop/KEYBOARD_SMOKE_CHECKLIST.md` — affected operator journeys.
- `desktop/UI_DESIGN_GUIDELINES.md` — final implemented visual specification.
- `desktop/UI_CONTRAST.md` — final actual foreground/background measurements.
- `desktop/README.md` — concise description of changed navigation and controls.
- `docs/RELEASE_ACCEPTANCE.md` — commands/results, actual renders, native/package evidence and remaining limitations.
- `PLAN.md` — accepted scope/status only after evidence supports it.
- `docs/tasks.md` — each card's real completion/check records.
- `tasks/README.md` — final queue status without importing past grants.

**Inputs / dependencies**

- IDEUX-01–12 complete; the immutable new references, production baseline,
  [component reproduction](RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks),
  [native keyboard checklist](../desktop/KEYBOARD_SMOKE_CHECKLIST.md) and existing
  [terminal procedure](../desktop/TERMINAL.md).

**Implementation rules**

- Render full Summary, Analysis, Bugs, Performance, Security and Source/Review
  with production components and labeled deterministic data. Compare at approximately
  1512×712 logical pixels (the original attachments are 3024×1424 pixels; do not
  assume their exact device scale), plus 1440×900, 1000/999×760, 800×650 and 1280×600.
  Cover 100/125/150% text and representative 1×/2× density. Match hierarchy and
  proportions while deliberately retaining our colors, rail and guarded workflow.
- Check every affected empty/loading/running/paused/interrupted/partial/stale/
  failed/canceled/unavailable state, reported-count/detail lag, long paths/errors
  and dense result sets. Inspect with optional help collapsed; required actions
  and consequences must remain understandable without explanatory paragraphs.
- Native journeys: open/restore project; search each mode; Summary → category →
  detail → prepare → source/draft → Review; explicit checks → Apply → Undo in a
  disposable project with deterministic provider data; resume/cancel analysis;
  filter and navigate back; resize dock/drawer; use Terminal and return to source.
  Validate draft discard, stale evidence and reconnect failures along these paths.
- Verify tab/arrow/Enter/Escape behavior, visible focus, icon names, selection/copy,
  popup placement and focus restoration in the native window. Keep terminal chords
  owned by the terminal. Offscreen tests do not establish OS or screen-reader behavior.
- Run the full desktop gate, repository validation, package build and documented
  packaged-terminal smoke. Inspect before/after production captures against each
  attachment; do not substitute concept mockups or rewrite reference images.
- Fix any caused failure in its owning earlier card before closing acceptance.
  Record untested platforms, screen-reader speech, signing/notarization or blocked
  native flows precisely. Historical passes cannot validate changed inputs.
  Local per-task commits are authorized; publication and live model campaigns are not.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --rerun-tasks --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.DesktopAccessibilityTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.ResultWorkspaceLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ide-ux/final"
./scripts/desktop-gradle.sh test spotlessCheck detekt createDistributable
./scripts/validate.sh
./desktop/scripts/terminal-packaged-smoke.sh
git diff --check
```

Use the documented Java 21 launcher/JBR 25 setup. Report native operator results
separately from these commands, and leave required acceptance pending if blocked.

## Planning validation

The original planning turn changed only `docs/tasks.md`. Its checks on 2026-09-16 passed:
13 ordered cards have the required fields; existing target files, local links and
script paths exist; eight new target paths are explicitly identified; the previous
task file is preserved byte-for-byte below; all other pre-existing changed and
untracked files retain their original hashes; and `git diff --check` is clean.
Implementation commands above have not been run for this new queue; previous
acceptance results below belong only to their original inputs and scopes.

---

## Historical queues and receipts — inactive

Everything below is retained verbatim from the pre-existing task file, including
uncommitted completion records. Its statements about active queues, schedules,
models, permission, commits and next actions describe prior work and do not apply
to IDEUX-01–13 or authorize execution of this new plan.

# Mini-Orca approved rounded mockup implementation — 2026-09-16

Implement the approved Summary, Analysis and Editor/Review mockups in the production
desktop UI. **MOCK-01–06** are the sole active queue; their product decisions and
reference images are in [PLAN.md](../PLAN.md#approved-visual-target).

## Current execution scope

- Execute serially in this checkout, one card per scheduled wake. Resume an
  interrupted card and its running verification before taking another card.
- Follow current root/area instructions and the accepted mockups. The latest user
  request supersedes older copy/geometry choices where those choices conflict.
  Preserve all unrelated work and earlier accepted changes.
- The user authorized implementation and scheduling, not commits, pushes, releases,
  live provider campaigns or additional agents. Use this task's current settings;
  do not inherit the historical SOL/Astra switching or repair-count policy.
- Each card includes its necessary production/tests/docs. Before extending a target
  list, record the concrete dependency. Fix failures caused by the change; do not
  lower thresholds or replace assertions with checks that merely bless new output.
- UI examples are illustrative. Render real state from current owners; preserve
  unknown versus zero, partial/stale/failed/canceled, model suggestion versus
  verified evidence, provider consent, execution trust, and guarded Apply/Undo.
- Every desktop card requires its focused verification, visual inspection of its
  affected production components, `./scripts/desktop-gradle.sh test spotlessCheck detekt`
  and `git diff --check`. Use README toolchain setup. Reuse passing evidence only
  while its inputs are unchanged.
- Record status, active stage, actual commands/results, render paths, remaining
  differences and blockers on the card. Do not mark completed from compilation,
  generated concept images or tests alone; inspect production renders against the
  approved reference. Distinguish offscreen evidence from native observations.
- [tasks/README.md](../tasks/README.md) owns the wake/stop procedure. Historical
  sections below the history marker are not an execution queue.

## Task MOCK-01 — Rounded frame, pane geometry and toolbar

**Status:** [x] Accepted — 2026-09-16. Frame, geometry and toolbar complete.

**Preparation baseline:** `./scripts/desktop-gradle.sh test spotlessCheck detekt`
passed on 2026-09-16 before implementation. No MOCK card is accepted by that result.
At the start of the first scheduled wake, existing changes were limited to the
approved planning files and mockup assets; no other source writer was active.

**Implementation stage:** accepted; stop this wake. MOCK-02 is next. The scheduler
remains Active at the existing 20-minute cadence, verified in the app and saved
configuration after implementation.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — shared frame color,
  structural shape, typography/spacing and restrained surface treatments.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — continuous frame,
  outer insets and workspace/terminal placement.
- `desktop/src/main/kotlin/io/miniorca/desktop/IdeShell.kt` — clipped pane perimeters,
  rail and gutters that retain accessible splitters.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt` — correct
  available-width accounting for frame/gutters without altering saved preferences.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopHeader.kt` — mockup toolbar
  hierarchy and visible labeled analysis/daemon states at sufficient widths.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopStatusBar.kt` — continuous
  frame and retained right-aligned model details control.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopThemeTest.kt` — contrast and
  shared presentation behavior on actual updated backgrounds.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopLayoutStateTest.kt` — pane
  bounds, narrow transitions and temporary clamping.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopShellTest.kt` — shell/status
  and splitter regressions.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — frame
  renders with filled headers and selected rows, including narrow/short views.
- `desktop/UI_DESIGN_GUIDELINES.md` — new reference, geometry/color/copy decisions.
- `desktop/AGENTS.md` — reconcile structural shape guidance with the accepted frame.
- `desktop/UI_CONTRAST.md` — actual changed semantic contrast measurements.

**Inputs / dependencies**

- None. Use all three rounded PNGs and the user's frame reference linked in PLAN.md.
  Existing `DockedToolWindow`, `EditorArea`, `TerminalDock` and shared shape tokens
  already own pane clipping; extend them rather than adding a second design system.

**Implementation rules**

- Produce visibly inset rounded structural panes with roughly 18dp corners and
  8dp gutters. Keep structural surfaces flat and use quiet charcoal-teal outer
  chrome. Do not simulate the look with screenshots, decorative outlines or fake
  traffic-light buttons.
- Preserve current native window controls and terminal startup/cleanup behavior.
  Native outer corners remain OS-owned; the visible in-app corners must match
  within the window. Any necessary native-window change requires recording the
  target and native verification before taking it.
- Use shared type/spacing roles to match mockup density. Keep all six rail
  destinations, actual project/branch/search controls, distinct analysis and daemon
  states, and model counts. Long project paths and status messages must fit or
  disclose fully without obscuring necessary actions.
- Account for each gutter only once. Preserve minimum usable editor space,
  ≥1000dp docks, <1000dp labeled drawers, pointer/keyboard resizing and preference
  restoration. Existing preferences must not be reset to achieve a screenshot.
- Clip all child backgrounds to the structural shape; retain independent keyboard
  focus and meaningful contrast. Check populated panes, not empty corner samples.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DesktopThemeTest' --tests 'io.miniorca.desktop.DesktopLayoutStateTest' --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/mock-01"
./scripts/desktop-gradle.sh test spotlessCheck detekt
git diff --check
```

**Acceptance**

Frame, toolbar and three Editor panes visibly match the rounded reference; actual
render comparisons cover wide, 1000/999dp, 800×650, 1280×600 and large text, with
no overlap, clipped controls, lost corners or reset stored sizes.

**Acceptance evidence — 2026-09-16**

- Implemented a shared charcoal-teal frame (`#203238`), clipped 18dp workspace
  corners and 8dp insets/gutters. Files, editor and right tools now end together;
  the separately rounded terminal spans their combined width. The 56dp toolbar
  keeps labeled analysis/daemon chips and the status bar shares the outer frame.
- Pane budgeting includes both frame insets and each gutter once. Default docks
  leave a 400dp editor at 1000dp; saved preferences and the 1000/999 breakpoint
  remain unchanged. Splitter handles show keyboard focus and retain pointer/arrow
  resizing with deferred commits. Removed the obsolete terminal separator option.
- Focused command above passed **92 tests, 0 failures/errors/skips**. The final
  `./scripts/desktop-gradle.sh test spotlessCheck detekt` passed **469 tests,
  0 failures/errors/skips**, Spotless and Detekt (0 smells). Reused unchanged
  Spotless/Detekt results on the final test run. `git diff --check` passed.
- The visual suite checks 30 frame combinations: four docked viewports at
  100/125/150% text with expanded/collapsed terminal, plus two narrow viewports
  at those scales with the terminal collapsed. Existing tests cover the separate
  terminal overlay. Pixel checks use filled production headers and selected rows;
  semantic bounds check corners, gutters, pane alignment and terminal width.
- Inspected actual component renders against `03-review-rounded.png` and the
  user's frame reference: `frame-1600-1000-1.0-collapsed.png`, `editor-1440.png`,
  `frame-1000-760-1.0-collapsed.png`, `frame-1000-760-1.5-collapsed.png`,
  `frame-999-760-1.25-collapsed.png`, `frame-800-650-1.5-collapsed.png`, and
  `frame-1280-600-1.5-expanded.png`, all under
  `desktop/build/reports/mockup-ui/mock-01/`. The frame matches the accepted
  silhouette, spacing and flat surface treatment. These are production components
  with fixture data, not native window screenshots.
- During iteration, `spotlessApply compileTestKotlin` first found a malformed
  fixture statement; it passed after correction. A new splitter test initially
  expected 16dp; corrected it to the existing 12dp contract and added actual
  pointer drag/release coverage. No production resize behavior or quality threshold
  was weakened.
- Remaining scope: Summary, Analysis, comparison/progression and Review are
  MOCK-02–05; native acceptance is MOCK-06. MOCK-04 records the large-text
  breadcrumb/gutter issues seen in the source content. This card accepts the frame
  and toolbar only. Native controls, source mutation, provider consent and terminal
  lifecycle code were not changed. No commits or pushes were made.

## Task MOCK-02 — Summary composition and meaningful coverage

**Status:** [x] Accepted — 2026-09-16. Summary composition and coverage complete.

**Active stage:** accepted; stop this wake. MOCK-03 is next. Prior MOCK-01 changes
are preserved. The scheduler remains Active at its existing 20-minute cadence,
verified in the app and saved configuration; no verification is still running.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — render comparison
  showed the old compact type was denser than the approved Summary; add shared
  workspace body/heading/metadata roles while preserving existing control/code roles.
- `desktop/src/main/kotlin/io/miniorca/desktop/ModelResultContent.kt` — necessary
  caller support for those Summary prose roles, preserving the formatter, selection
  and disclosure behavior through an optional text-style argument.
- `desktop/UI_DESIGN_GUIDELINES.md` — document the resulting workspace type roles.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — project
  introduction, coverage and responsive two-column information layout.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryVisuals.kt` — coverage
  presentation and flat module/diagram sections.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryIssues.kt` — readable
  named result cards and supported evidence breakdowns.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisCategoryPanels.kt` — shared
  persistent category labels with whole-card navigation.
- `desktop/src/main/kotlin/io/miniorca/desktop/MermaidDiagram.kt` — necessary adjacent
  dependency: place the existing diagram disclosure beside its Summary section
  heading without duplicating rendering, expansion, source or zoom state.
- `desktop/src/main/kotlin/io/miniorca/desktop/EngineeringInsightPanel.kt` — retain
  complete insight content within the Summary composition if shared changes are needed.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryPaneTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryIssuesTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisFileStatusTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt`
- `desktop/README.md` — resulting Summary behavior and navigation.

**Inputs / dependencies**

- MOCK-01. Reference `design/ui-mocks/ux-concepts-2026-09-16/01-summary-rounded.png`.
  Existing `projectSummaryPresentation`, `analysisSelectionCoverage`,
  `summaryIssueMetrics` and `AnalysisResultPageState` remain state owners.

**Implementation rules**

- Match the project introduction and quiet metadata, dedicated labeled segmented
  coverage, three named category cards and wider-left/narrower-right content.
  Move facts out of the current long metric-card strip; keep all useful data.
- Calculate segments from current selected-file coverage, including failure/running
  states when present. Never divide by zero, hide nonzero exceptional states,
  coerce unavailable counts to zero or label unanalysed files as up to date.
  "View analysis" only navigates; it must not start a run.
- Category names are visible, with exactly-once pointer/keyboard activation and
  current selection/focus semantics. Use neutral treatment for unknown/unanalysed
  outcomes; do not imply Security safety from zero findings.
- Show verified/model/measured breakdowns only where current matching data supports
  them. Retain existing global totals where per-category attribution is unavailable;
  never infer the mockup's sample breakdown from an unrelated total.
- Preserve full purpose/architecture/insight text and offline diagrams. No repeated
  instructional placeholders. Keep modules flat with exact paths and responsibilities.
  Stack columns at narrow widths and enlarged text; use coherent page scrolling.
- Correct the generated Summary image's erroneous Analysis rail highlight in the
  implementation: Summary must select Summary. Omit all concept/example captions.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ProjectSummaryPaneTest' --tests 'io.miniorca.desktop.ProjectSummaryIssuesTest' --tests 'io.miniorca.desktop.AnalysisFileStatusTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/mock-02"
./scripts/desktop-gradle.sh test spotlessCheck detekt
git diff --check
```

**Acceptance**

The full Summary composition matches the reference and remains correct for empty,
populated, outdated, running, failed and unavailable data, live selection changes,
long prose and first-click diagram expansion.

**Acceptance evidence — 2026-09-16**

- Replaced the metric-card strip with project name, full purpose, quiet indexed
  metadata, a labeled selected-file coverage bar and three permanently named
  result cards. Added shared workspace body/heading/metadata type roles. Category
  titles align at the top regardless of missing status or evidence detail.
- Architecture and flat module rows occupy the wider left column; full engineering
  insight and Flows occupy the right. Narrow/enlarged-text layouts stack and retain
  one page scroll. Show/Hide diagram shares the heading row; existing local
  rendering, first-click expansion, source selection and zoom owners are retained.
- Coverage follows current matching selection, keeps nonzero exceptional and
  unaccounted states, and uses overflow-safe segment totals. Unknown coverage is
  separate from zero selected files; a fresh description or completed run alone
  cannot imply current file coverage. Current run lifecycle/reasons remain visible;
  foreign run failures cannot override current coverage or inject failure details.
- View analysis and category activation only navigate. Pointer, Enter/Space and
  focus tests retain exactly-once actions; live selection tests verify the legend
  updates. Unknown/zero result counts are neutral. Removed the old weighted
  traffic-light score; matching priority evidence remains available. Global
  tool-reported/AI totals stay below the cards because category attribution is
  unavailable in those totals.
- Final focused command above passed **70 tests, 0 failures/errors/skips**;
  `spotlessApply` was also run while iterating. Final
  `./scripts/desktop-gradle.sh test spotlessCheck detekt` passed **473 tests,
  0 failures/errors/skips**, Spotless and Detekt (0 smells). `git diff --check`
  passed. No quality thresholds were changed.
- Rendered the full production Summary/frame at 1600×1000, 1440×900, 1000×760,
  999×760, 800×650 and 1280×600, each at 100/125/150% text. Inspected reference,
  narrow, short and enlarged-text images against `01-summary-rounded.png`, including
  `summary-frame-1600-1000-1.0.png`, `summary-frame-1440-900-1.0.png`,
  `summary-frame-1000-760-1.25.png`, `summary-frame-999-760-1.5.png`,
  `summary-frame-800-650-1.5.png` and `summary-frame-1280-600-1.5.png`, under
  `desktop/build/reports/mockup-ui/mock-02/`. Additional component renders/tests
  cover long prose, all coverage states, current selection with older description,
  full insight, collapsed/expanded diagrams and keyboard navigation.
- During iteration, fixed a nullable current-run branch and a fixture state-owner
  argument found by compilation. Updated assertions for the removed metric strip,
  permanent category names and page scrolling; corrected a fixture expectation
  from incomplete to not analyzed. Replaced redundant category-tooltip assertions
  with visible labels, actual hover fill and keyboard/pointer behavior. Final visual
  review corrected card-title alignment and module responsibility typography.
- Remaining differences are intentional data/copy rules already accepted above:
  no concept caption, duplicate Summary heading/count line or unsupported category
  attribution; Summary selects its own rail destination, and full insight labels
  and the real terminal strip remain. Renders use production components with
  fixture data and establish offscreen Summary evidence. Native window acceptance
  remains MOCK-06; Analysis, comparison and Review remain MOCK-03–05. No provider,
  source-mutation or terminal lifecycle code changed; no commits or pushes made.

## Task MOCK-03 — Analysis progress and file-state table

**Status:** [x] Accepted — 2026-09-16. Analysis progress and file table complete.

**Active stage:** accepted; stop this wake. MOCK-04 is next. MOCK-01/02 changes are
preserved and no verification is still running. The existing scheduler remains
Active every 20 minutes, verified through the app and saved configuration.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` and
  `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — move the
  existing Summary section surface to a shared workspace section for Analysis,
  retaining its shape/type/spacing instead of copying another panel implementation.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryVisuals.kt` — update the
  Summary coverage caller to the shared surface name; preserve its appearance.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — Analysis progress
  block, page hierarchy and controls.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisCategoryPanels.kt` — named
  category layout and truthful progress/count states.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — expose
  existing run facts needed for finished/total labels.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisFileSelector.kt` — expanded
  table, local query/filter controls, idle selection and running lock.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisFileStatus.kt` — presentation
  of admitted run progress against matching file-selection identity.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopContrastTest.kt` — the full
  gate found its old header-label assertion; verify the new visible run heading
  against its actual shared panel background while retaining lifecycle contrast.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisFileSelectionTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisFileStatusTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflowTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt`
- `desktop/README.md` — file table, disclosure default and lifecycle behavior.

**Inputs / dependencies**

- MOCK-02. Reference `design/ui-mocks/ux-concepts-2026-09-16/02-analysis-rounded.png`.
  Preserve `projectRunPresentation`, `AnalysisWorkspaceActions`,
  `filteredAnalysisFiles`, selection persistence and existing admission workflow.

**Implementation rules**

- Group run state, completed/total, progress, active path(s) and Pause/Cancel or
  the valid lifecycle actions in one region. Indeterminate/missing totals remain
  explicit; no invented ETA. Keep Start, selective retry and Resume previews.
- Preserve named category cards and live stage coverage, unknown count em dashes,
  failures and actionable diagnostics. Navigation does not admit provider work.
- Show Files expanded by default for a newly opened project, allowing local
  collapse. Align File / Analysis state / Details columns at wide widths;
  reorganize rows at narrow widths and large text without shrinking labels.
- During a run, display current Running/Pending/finished stage facts only from a
  matching admitted run. Keep saved freshness distinct from current-run progress.
  Never mark old results current or classify a file from a different run/project.
- Keep local search/All/Needs attention/Excluded usable while running. Disable
  selection changes for active/paused runs and show the lock reason nearby.
  Idle checkboxes, Select all/Exclude all, Refresh files, automatic save errors and
  retained selections continue to work. Bulk selection still covers all eligible
  files, not just filtered rows.
- Use real filtered/total counts. Do not invent pagination or copy the mockup's
  seven sample files. Preserve responsive scrolling and all included file paths.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.AnalysisFileSelectionTest' --tests 'io.miniorca.desktop.AnalysisFileStatusTest' --tests 'io.miniorca.desktop.DesktopAnalysisWorkflowTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/mock-03"
./scripts/desktop-gradle.sh test spotlessCheck detekt
git diff --check
```

**Acceptance**

Reference-like running layout with readable active path, category names and table;
all idle, paused, canceled, failed, partial and resumed transitions preserve
truthful coverage and selection/consent semantics.

**Acceptance evidence — 2026-09-16**

- Grouped file progress, current paths and valid lifecycle controls in one rounded
  panel using the shared workspace surface. Progress counts files whose stages
  have finished, including unsuccessful terminal outcomes; the label says
  **finished**, never successful/complete. Missing file totals stay unavailable.
  Stored elapsed/window facts and previous-run metadata remain; removed the
  duplicate current-run stage-count line and obsolete stage-progress ratio.
- Files opens expanded for each project. Wide rows align File / Analysis state /
  Details; narrow/enlarged-text rows stack. The active file has the selected fill.
  Search and All / Needs attention / Up to date / Excluded filters are local;
  the footer counts matches against all files, independent of the visible scroll
  window. The bounded virtualized table retains every row; tests reach its last
  file and the page footer. Expanded stage details retain complete reasons.
- Live rows use only active/paused/interrupted runs with matching selection
  project/revision, plan queue identity and run/plan file hashes. Unplanned,
  foreign and mismatched-hash files retain saved status. Completed run stages
  alone never upgrade saved analysis to Up to date; saved freshness remains
  separate. Plan-ineligible stages do not become operational failures. Unknown,
  failed, unavailable, partial, paused and interrupted states retain their meaning.
- Idle checkboxes and all-file bulk selection retain automatic persistence and
  error handling. Active/paused/interrupted selection is locked, with its reason
  visible even when collapsed; bulk controls hide while locked. Search, filters,
  Details, collapse and Refresh remain available. Start/retry/Resume admission,
  exactly-once controls, consent, stale response handling and provider workflows
  remain with existing owners and are covered by the existing workflow tests.
- `./scripts/desktop-gradle.sh spotlessApply compileTestKotlin` passed during
  construction. The final focused command above (also prefixed with
  `spotlessApply`) passed **89 tests, 0 failures/errors/skips**. After updating
  the added contrast target, the final full gate
  `./scripts/desktop-gradle.sh spotlessApply test spotlessCheck detekt` passed
  **479 tests, 0 failures/errors/skips**, Spotless and Detekt (0 smells).
  `git diff --check` passed; quality thresholds are unchanged.
- Rendered the production Analysis/frame at 1600×1000, 1440×900, 1000×760,
  999×760, 800×650 and 1280×600 at 100/125/150% text. Compared the wide running
  composition with `02-analysis-rounded.png`, then inspected narrow, short and
  enlarged-text views. Evidence under `desktop/build/reports/mockup-ui/mock-03/`
  includes `analysis-frame-1600-1000-1.0.png`, `analysis-frame-1440-900-1.0.png`,
  `analysis-frame-1000-760-1.5.png`, `analysis-files-frame-999-760-1.5.png`,
  `analysis-files-frame-800-650-1.5.png`, `analysis-frame-1280-600-1.5.png`,
  `analysis-last-file.png` and `analysis-running-filtered.png`. Existing focused
  renders/tests also cover idle selection, long paths, load/save errors, failures,
  lifecycle controls and unknown category counts. Full gate covers Summary after
  moving its unchanged section surface into the shared theme.
- Iteration corrected old collapsed-default and header assertions, a missing test
  render between disclosure clicks, and a duplicate Details text selector. The
  new full-frame scroll test initially targeted the rail; it now targets the
  Analysis page explicitly, retaining footer-reachability assertions. The full
  gate found one old contrast assertion for the removed current-run line; the
  corrected test checks the new visible heading on its actual panel background.
  A local edit script stopped on an unmatched substring; inspected its partial
  edits and completed them before the final passing runs.
- Intentional reference differences follow the accepted data/copy rules: no
  concept caption or repeated page heading; real counts/paths, an additional
  retained Up to date filter, saved-result Details, Refresh and idle selection
  controls remain functional. The accepted terminal strip is retained. These
  images use production components with test data; native window acceptance is
  still MOCK-06. Candidate comparison and Review remain MOCK-04/05. No provider,
  source-mutation, terminal lifecycle, daemon contract or dependency changes;
  no commits or pushes were made.

## Task MOCK-04 — Candidate comparison and compact progression

**Status:** [x] Accepted — 2026-09-16. Candidate comparison and progression complete.

**Active stage:** accepted; stop this wake. MOCK-05 is next. Prior MOCK-01–03
changes are preserved; no verification is still running. The scheduler remains
Active at the existing 20-minute cadence, checked in the app and saved configuration.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/DiffViewer.kt` — full-height
  Current/Candidate comparison, gutters, markers and readable code.
- `desktop/src/main/kotlin/io/miniorca/desktop/EditorWorkspace.kt` — Source/Candidate
  tabs, breadcrumbs and compact progression without redundant candidate cards.
- `desktop/src/main/kotlin/io/miniorca/desktop/ReviewEvidencePane.kt` — canvas
  composition and shared progression presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — pass immutable
  existing review evidence into the editor surface.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — reuse the existing
  review snapshot construction if necessary for the shared presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/SourceEditorPane.kt` — reuse existing
  syntax rendering only where required by the diff.
- `desktop/README.md` — document the changed Editor tabs, comparison modes and
  local Edit draft/progression behavior beside the existing read-only contract.
- `desktop/src/test/kotlin/io/miniorca/desktop/DiffViewerTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/EditorWorkspaceTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/ReviewEvidencePaneTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt`

**Inputs / dependencies**

- MOCK-03. Reference `design/ui-mocks/ux-concepts-2026-09-16/03-review-rounded.png`.
  Current `UnifiedDiff`, `sideBySideDiffRows`, `editorChromeUiState`,
  `reviewProgressionRows` and review evidence/eligibility remain authoritative.

**Render finding to resolve:** MOCK-01's 1000dp / 150% source render shows the
read-only breadcrumb label wrapping mid-word and source gutter rows losing their
alignment. Check `EditorWorkspace.kt` and `SourceEditorPane.kt` while implementing
this card; compare against `frame-1000-760-1.5-collapsed.png` in the MOCK-01 render
directory. These content areas are not accepted by the frame card. Extend the
existing SourceEditorPane target to include this concrete large-text correction.

**Implementation rules**

- Match the dominant comparison with Current and Candidate column headers, actual
  line numbers, syntax colors and restrained red/green line backgrounds. Retain
  explicit non-color change markers and text selection.
- Preserve every server-supplied line and its hunk/line identity. Multiple removals,
  additions, empty sides, context and long lines must align correctly; padding
  cells must not pretend to be unchanged source. Do not fetch model content or
  reconstruct missing source from assumptions to fill the screenshot.
- Keep side-by-side as the wide default and retain a working unified view for
  compact inspection. Use sensible independent horizontal code scrolling and
  aligned vertical rows, avoiding weighted children inside unbounded width.
- Source and diff stay read-only. Only the existing isolated draft editor edits.
  Tabs and progression navigation must not validate, run checks, generate or apply.
- Derive progression states from the existing evidence functions, including
  failed/stale/missing states. If the editor needs the review snapshot, pass it
  through existing state ownership; do not implement duplicate eligibility rules.
- Keep New function and the valid path to Edit draft. Replace redundant Candidate
  and repeated Candidate diff headers with the approved tab/breadcrumb hierarchy.
  Preserve invalidation on edit and project/file/draft replacement.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DiffViewerTest' --tests 'io.miniorca.desktop.EditorWorkspaceTest' --tests 'io.miniorca.desktop.ReviewEvidencePaneTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/mock-04"
./scripts/desktop-gradle.sh test spotlessCheck detekt
git diff --check
```

**Acceptance**

The central Editor comparison visibly matches the reference with real validated
diffs, aligned long/multiline changes and truthful progression. Source/diff
selection, compact unified access and stale-draft guards remain intact.

**Acceptance evidence — 2026-09-16**

- Replaced the lower Candidate card and repeated canvas heading with Source /
  Candidate diff tabs, a full-path breadcrumb disclosure, a permanent Read-only
  label, and compact Request → Draft → Validate → Checks → Review evidence.
  Status markers and exceptional-state labels use the existing evidence and
  guarded Apply decision functions through the shared immutable Review snapshot.
  The former unconditional “focused checks pending” copy is removed.
- Kept New function and routed Edit draft to the existing Assistant draft focus
  action. Both remain reachable at large text; they share the responsive action
  row so the progress strip does not consume an extra row in short windows.
  Tab/mode/draft navigation remains local, with no generation, validation,
  check execution or source mutation introduced.
- Current/Candidate columns fill the available canvas with clipped 14dp corners,
  shared syntax colors, actual line numbers and explicit +/− markers. Each side
  scrolls horizontally within bounded width; both share vertical scroll state.
  Replacement blocks pair removals/additions in order, preserving unequal blocks,
  empty text, hunk metadata and supplied identities. Missing counterparts remain
  visibly blank and carry an accessible description. Multiline cell heights stay
  aligned. Compact views default to Unified; users can switch either way.
- Fixed breadcrumb width ownership at the outer tooltip so Read-only cannot wrap
  mid-word. Source gutter width now measures the largest number and scales with
  text; numbers and source share the same scaled line height. Existing source
  focus, declaration selection, markers and read-only ownership remain intact.
- The focused command above, prefixed with `spotlessApply`, passed **77 tests,
  0 failures/errors/skips**. The final
  `./scripts/desktop-gradle.sh test spotlessCheck detekt` passed **484 tests,
  0 failures/errors/skips**, Spotless and Detekt (0 smells). `git diff --check`
  passed. No thresholds, baselines, dependencies or daemon contracts changed.
- Interaction tests exercise pointer text selection and keyboard copying using a
  test-owned clipboard, independent horizontal and synchronized vertical scrolling,
  keyboard mode switching, local Candidate/Edit draft navigation, unequal and
  multiline changes, five-digit diff gutters, and stale/failed/running/missing
  evidence plus edit/file-identity invalidation. The full gate retains existing
  source-selection, keyboard, workflow and mutation-guard coverage.
- Rendered production components at 1600×1000, 1440×900, 1000×760, 999×760,
  800×650 and 1280×600 with 100/125/150% text, alongside the earlier frame and
  content regression suite. Compared against `03-review-rounded.png`; inspected
  `comparison-frame-1600-1000-1.0.png`, `comparison-frame-1440-900-1.0.png`,
  `comparison-frame-1000-760-1.5.png`, `comparison-frame-999-760-1.25.png`,
  `comparison-frame-800-650-1.5.png`, `comparison-frame-1280-600-1.5.png`,
  `source-alignment-1000-760-1.5.png`, `diff-long-multiline-1000-1.5.png`, and
  `editor-progression-stale-800-1.5.png` under
  `desktop/build/reports/mockup-ui/mock-04/`. These show the dominant comparison,
  rounded bounds, real change alignment, readable actions and scaled gutters.
- Iteration corrected a legacy fixture still passing a draft instead of the new
  snapshot, and an incorrect shape token name. Initial new assertions mistakenly
  included the Files search field in a read-only check, expected nonexistent text
  selection semantics, and used the wrong evidence detail; these now inspect the
  actual subtree, drag/copy interaction and existing evidence copy. A real
  1280×600/150% canvas-height failure was fixed by moving Edit draft into the
  action row; the minimum usable-height assertion remains unchanged.
- Intentional reference differences retain working New function / mode controls,
  actual line numbers and evidence status rather than sample labels. No branch
  or provider attribution is invented for either diff side. The right Review
  panel in these images is still the previous production panel: its compact
  layout is MOCK-05. These are offscreen production-component renders with fixture
  data; native window acceptance remains MOCK-06. No commits or pushes were made.

## Task MOCK-05 — Compact Review with a reachable guarded action

**Status:** [x] Accepted — 2026-09-16. Compact Review and guarded action complete.

**Active stage:** accepted; stop this wake. MOCK-06 is next. Earlier accepted
changes are preserved, and no verification is running. The existing scheduler
remains Active at 20-minute intervals, verified in the app and saved configuration.

**Target files**

- `desktop/src/main/kotlin/io/miniorca/desktop/ReviewEvidencePane.kt` — target,
  readiness, compact evidence, disclosures and bottom action layout.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkflowToolWindows.kt` — concise
  target/header identity and retained Context/Assistant/Review tabs.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — accessible solid
  positive primary action treatment using the shared control system.
- `desktop/src/main/kotlin/io/miniorca/desktop/ChromeControls.kt` — shared action
  support only if necessary for the approved Apply treatment.
- `desktop/src/test/kotlin/io/miniorca/desktop/ReviewToolWindowTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/ReviewEvidencePaneTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DraftReviewWorkflowTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopThemeTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt`
- `desktop/README.md` — resulting Review/receipt behavior.
- `desktop/UI_CONTRAST.md` — necessary adjacent documentation of the new opaque
  Apply fill and measured label/focus contrast.

**Inputs / dependencies**

- MOCK-04 and the rounded Review reference. Reuse `reviewEvidenceUiState`,
  `applyDecisionUiState`, `reviewNextActionUiState` and existing
  `DraftApplicationActions`; this is a presentation change.

**Implementation rules**

- Match target identity plus Edit draft, readiness state, three compact
  Validation / Focused checks / Source unchanged rows, required-check summary and
  collapsed check/project-context details. Preserve actual evidence and diagnostics.
- Use "Ready to apply" only when the existing decision is eligible. Never claim
  source unchanged solely because a draft exists. Keep stale, skipped, failed,
  missing and running evidence distinct and surface the relevant recovery reason.
- Keep the exact target and one-declaration/one-file consequence next to a clear
  enabled Apply change action only when eligible. Its accessible name retains
  exact scope. Keep Edit draft, trust-scoped checks, rerun and failure repair paths
  usable without duplicate primary actions.
- Place the action region below scrollable evidence, so it remains reachable in
  normal-height windows. At short/narrow/150% text, adapt without overlap or
  cutting off essential consent/error text; allow bounded scrolling if necessary.
- Use an opaque positive action fill with verified text/focus contrast. Do not
  change every positive badge into a primary action.
- After Apply show the actual receipt and guarded Undo from returned state.
  No pre-Apply Undo, automatic mutation, new approval flow or stale evidence bypass.
  Context, Assistant, draft discard and terminal behavior remain intact.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ReviewToolWindowTest' --tests 'io.miniorca.desktop.ReviewEvidencePaneTest' --tests 'io.miniorca.desktop.DraftReviewWorkflowTest' --tests 'io.miniorca.desktop.DesktopThemeTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/mock-05"
./scripts/desktop-gradle.sh test spotlessCheck detekt
git diff --check
```

**Acceptance**

Reference-like ready Review at wide size; no hidden action or lost reason in
invalid, stale, failed, untrusted, running, applied or undo-unavailable states.
Behavior tests prove exactly-once guarded mutation only from explicit actions.

**Acceptance evidence — 2026-09-16**

- Review now shows the declaration/path and local Edit draft action, a readiness
  panel, three compact Validation / Focused checks / Source unchanged rows, and
  reported required-check counts. Check details and Project context start collapsed.
  Removed the repeated REVIEW/Next action/Progress headers and five-row progression
  from this pane; the editor still owns its compact progression strip. Existing
  sanitized failed-output previews, validation diagnostics, complete check output,
  identity disclosures and candidate engineering insight remain available.
- The opaque green Apply change action is below scrolling evidence in normal-height
  panes, beside the one-declaration/one-file consequence and exact target. Its
  accessible name retains the symbol/path. Short or enlarged-text panes scroll as
  a whole; unusually long target scopes have a separately bounded action region.
  Expanding and scrolling details leaves the normal action region anchored.
- Apply/Undo still call the existing guarded application actions. A returned
  receipt alone offers Undo, and returned unavailability disables it. Receipt paths
  take precedence over later file selection. Tests activate Apply by keyboard and
  Undo explicitly, asserting exactly one callback each and no mutation from
  disclosures, scrolling, target navigation or disabled controls.
- Readiness, evidence and next actions reuse their existing owners. Running
  validation/checks take presentation precedence over a previous passing report;
  neither Apply nor a second Rerun is offered while current evidence is running.
  Missing draft identity is Missing, not Stale. Actual stale/failed/skipped states
  remain distinct. Required counts exclude optional checks and do not turn missing
  or stale evidence into zero/passed. Existing eligibility accepts applicable
  skipped checks; that policy remains unchanged, with Skipped and the actual
  passed/required count visible rather than an invented all-checks-passed claim.
- Source-only checks keep their existing path. A generated test's action explicitly
  says Trust local execution & run checks and shows the exact command/project
  revision scope before activation. Rerun stays available inside Check details
  with the same execution disclosure. Failure repair and its existing manual-edit
  fallback remain functional; provider consent and backend trust/hash guards are
  unchanged.
- Added an opaque positive primary tone for Apply without changing positive badges
  or secondary actions. Label contrast is **11.13:1** default, **9.30:1** hover/press
  and **7.91:1** selected. The existing dark/light focus keylines remain visible;
  the full contrast suite covers the new tone on all supported host surfaces.
  Measurements and resulting behavior are documented in the desktop guides.
- The focused command above, prefixed with `spotlessApply`, passed **85 tests,
  0 failures/errors/skips**. The final gate
  `./scripts/desktop-gradle.sh spotlessApply test spotlessCheck detekt` passed
  **492 tests, 0 failures/errors/skips**, Spotless and Detekt (0 smells).
  `git diff --check` passed. No check configuration, thresholds, dependencies,
  toolchains or API contracts changed.
- Inspected production-component renders against `03-review-rounded.png`, including
  full-frame 1600×1000 and 1000×760/150% views, 360×850 ready/detail/focus/receipt
  views, 300×400/150% ready, stale, failed, running and execution-trust recovery,
  and a long target at 300×850/150%. Existing full-frame tests also render
  1440×900, 999×760, 800×650 and 1280×600 at 100/125/150% text. Evidence under
  `desktop/build/reports/mockup-ui/mock-05/` includes
  `comparison-frame-1600-1000-1.0.png`, `comparison-frame-1000-760-1.5.png`,
  `review-compact-ready-360.png`, `review-details-anchored-360.png`,
  `review-apply-focus-360.png`, `review-ready-short-action.png`,
  `review-stale-short-recovery.png`, `review-trust-short-action.png`,
  `review-reported-running-short-recovery.png`,
  `review-long-target-scope-300-850-1.5.png`, and `review-undone-360.png`.
- Iteration corrected a fixture missing its required diff argument, an implicit
  Compose height receiver, and two test tags sharing one layout node. The bounded
  action viewport and content now have separate nodes; the long-path test proves
  actual scrolling. Updated obsolete Progress/Next action/check-detail assertions
  to the new hierarchy while retaining diagnostics, identity and mutation checks.
  Final review added a report-level Running case to prevent a redundant Rerun.
  Final passes include these corrections; no failed check was waived.
- These are offscreen production components with fixture data. The compact Review
  composition, real-state differences and action reachability are accepted here;
  integrated native-window, focus and final regression acceptance remain MOCK-06.
  No model/provider campaign, Mini-Orca Apply/Undo against a real project, commit
  or push was performed during this card.

## Task MOCK-06 — Integrated visual and regression acceptance

**Status:** [x] Accepted.

**Active stage:** completed; all six cards accepted. Final native/test/package
verification finished with no running build or native fixture. The existing
heartbeat is **Paused**, verified in the app and saved configuration. No further
queue is authorized.

**Concrete integration repair:** native terminal expansion exposed the Swing host
painting square bottom corners outside the Compose clip. `IdeShell.kt` (already
a MOCK-01 target) must inset TerminalDock content by 8dp at its side/bottom
edges. The native overlay already supplies dialog padding. Keep the shell, reader,
focus callback and PTY resize ownership unchanged; verify the actual Swing host
after packaging, plus a focused content-bounds test.

**Target files**

- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — full
  production-shell comparison fixtures for all three approved views.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAcceptanceFixture.kt` —
  native fixture coverage using production components and local sample data.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAccessibilityTest.kt`
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt`
- `desktop/KEYBOARD_SMOKE_CHECKLIST.md` — current affected navigation/action checks.
- `desktop/UI_DESIGN_GUIDELINES.md` — final measured geometry/reference alignment.
- `desktop/UI_CONTRAST.md` — final changed colors and actual contrast.
- `desktop/README.md` — concise current behavior.
- `docs/RELEASE_ACCEPTANCE.md` — real render/native/test evidence and limitations.
- `PLAN.md`, `docs/tasks.md`, `tasks/README.md` — accepted status and scheduler closure.
- Production files already listed in MOCK-01–05 only when needed to repair a
  concrete integration or visual failure; record the cause before editing.

**Inputs / dependencies**

- MOCK-01, MOCK-02, MOCK-03, MOCK-04 and MOCK-05 accepted.
  All approved rounded reference images, repository UI guidelines, keyboard
  checklist and component/native reproduction procedure.

**Implementation rules**

- Render FULL Summary, Analysis and Editor/Review with matching deterministic
  fixture state at the reference aspect ratio (approximately 1600×1000), plus
  1440×900, 1000/999dp, 800×650 and 1280×600; include 125/150% text and long content.
  Existing reference PNGs are immutable visual targets; never overwrite them with
  production renders to make a comparison pass.
- Inspect reference and actual render side by side. Compare pane bounds/corners,
  gutters, typography, header/status arrangement, Summary columns, Analysis table,
  diff readability and Review action placement. Fix material mismatches; record
  unavoidable native-font/control differences explicitly. Test success alone
  does not establish visual fidelity.
- Verify local-only navigation/disclosures, keyboard focus and selected states,
  source/diff selection, filter/selection behavior, missing/stale/failed/loading/
  canceled/partial states, guarded Apply/Undo and terminal expand/collapse/return.
- Build and launch the app or maintained native fixture with isolated sample data;
  inspect native corners, resize, toolbar, dock/drawer and focus behavior using
  available computer-use tools. Use disposable project copies/fake providers for
  mutation checks. Do not use real user source or live model requests.
- Run the full local validation and package build. Record native and package
  evidence separately from offscreen rendering. If required native evidence is
  unavailable, complete independent checks and report the precise limitation;
  leave that acceptance pending rather than claiming an exact match.
- Remove superseded presentation helpers and dead call sites introduced by this
  work. Keep historical records intact. Pause the scheduler only after complete
  acceptance, or a documented external blocker requiring user action.

**Verification command**

```sh
./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.DesktopAccessibilityTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' -PvisualOutput="$PWD/desktop/build/reports/mockup-ui/final"
./scripts/desktop-gradle.sh test spotlessCheck detekt createDistributable
./scripts/validate.sh
git diff --check
```

**Acceptance**

All six cards pass their checks and actual visual review. Deliver links to
production-rendered after images, concise behavior/test results, native limitations
if any, and the app launch command. Pause the implementation heartbeat and record
6/6 only when every required acceptance item is satisfied.

**Acceptance record — 2026-09-16**

- Extended the maintained native fixture with full Summary, Analysis and Review
  frames plus the actual interactive `DesktopShell`. The fixture records provider
  and source intents without dispatch and uses only a disposable terminal project.
- Native inspection found the Swing terminal painting square bottom corners over
  the Compose clip. Repaired only the already-listed `IdeShell.kt` owner with an
  8dp side/bottom inset. The new content-bounds test covers 140/220/360dp docks;
  rebuilt native captures confirm the rounded perimeter with a real shell.
- The initial fixture compilation rejected a `UnifiedFinding` passed as a path;
  using its existing `location.path` fixed the callback. The first 71 focused /
  492 desktop tests and full validation passed before the native corner discovery.
  After that repair, the exact focused command above (with `spotlessApply` during
  iteration) passed **72 tests**; the final `test spotlessCheck detekt
  createDistributable` command passed **493 tests**, zero failures/errors/skips,
  Spotless and zero Detekt smells. `./scripts/validate.sh` then passed all nine
  stages; its 55 Python tests retain one existing opt-in conformance skip.
- Additional `MINI_ORCA_JBR25_HOME=/path/to/jbr-25
  ./desktop/scripts/terminal-packaged-smoke.sh` passed against the actual packaged
  JVM: real TTY, cwd, UTF-8, 121×42 resize, Ctrl+C, child cleanup and bounded close.
  Native UI used macOS 27.0 arm64 and JBR 25.0.4.1+1-b583.48, with the Java 21 launcher.
- Reviewed all three full reference-size production renders against the immutable
  approved PNGs, plus narrow/large-text/short states. Render paths, exact commands,
  native screenshots, scope and limitations are in the
  [final acceptance ledger](RELEASE_ACCEPTANCE.md#rounded-mockup-acceptance--2026-09-16).
  Offscreen evidence retains the exact reference/1000/999 sizes; native captures
  use the host's 800×600 and zoomed 1340×768 windows at 100/125/150% Compose text.
- Native checks establish rounded frame/dock/overlay geometry, local filter and
  disclosure behavior, source/diff selection, active-run locks, drawer/palette
  focus recovery, reachable exact Apply scope, stale-target recovery, terminal
  resize/retention and focus return. Both app sessions and their shell PIDs exited;
  both disposable source files remain unchanged. Actual guarded source mutation
  remains deterministic desktop/daemon test evidence, not a native fixture claim.
- `git diff --check`, local references and historical tail preservation pass.
  No quality threshold, dependency, source-safety boundary or accepted earlier
  implementation was removed. Screen-reader speech, other operating systems,
  signing and notarization remain outside this local acceptance. No commit, push,
  publication, live model/evaluation campaign or real-project Apply/Undo ran.

---

## Historical completed queues — not active instructions

The prior task file follows unchanged for receipt preservation. Old unchecked
items, scheduler directions, repair limits, model policies and commit grants do
not belong to MOCK-01–06.

# Mini-Orca UI polish — 2026-09-15

Implement the user's Summary, Analysis and Bugs / Performance / Security cleanup:
rounded shared controls, live clickable category boxes, simpler results and working
offline diagrams. POLISH-01–06 are accepted; the scheduler is paused.

## Current execution scope

- Only **POLISH-01–06** are active. The completed 2026-09-11 queue is retained
  verbatim below as history, including its receipts; its model and commit grants
  do not apply. POLISH-01 synchronizes the current product/execution guides.
- Use **SOL High** (`gpt-5.6-sol`, `high`) for implementation and validation.
  After a concrete code/test/review failure, allow **one Astra High repair**
  (`gpt-6-astra`, `high`) per card, retaining the failure and repair count across
  wakes. A failed repair pauses the scheduler with the candidate preserved.
- Reuse `mini-orca-ux-implementation` every 20 minutes, attached to the current
  task. Work serially in this checkout, one card per wake; resume an interrupted
  stage before taking another card. Pause after POLISH-06 or an actionable blocker.
- Follow root/area `AGENTS.md` and the UI guidelines. Preserve the existing
  uncommitted documentation edits. The user's 2026-09-15 follow-up authorizes a
  local commit after each completed task passes its checks, including the already
  accepted POLISH-01/02 work. Stage only reviewed task changes and report the hash.
  Pushes, releases and historical dispatcher/model-evaluation campaigns remain out of scope.
- Record each card's stage, actual model/effort, repair count, commands/results
  and concise review evidence here. Mark it complete only after its checks and
  the applicable AGENTS gates pass. Add a narrowly necessary target before editing
  it; do not broaden into unrelated cleanup or dependency upgrades.
- Remove the **Run limits UI**, retaining existing daemon bounds and explicit
  continuation/consent. Remove repeated **findings labels**, preserving findings.
  Remove routine provenance/status badges, preserving evidence, uncertainty and
  meaningful failed, partial, stale, unavailable and canceled states.
- Navigation/disclosures remain local. Keep source/diff read-only, existing
  eligibility owners, explicit provider consent, execution trust and guarded
  Review/Apply/Undo. No automatic analysis, benchmark or fix request on selection.
- Use the documented JDK 21/JBR 25 setup from `README.md`. Every desktop card
  also requires `./scripts/desktop-gradle.sh test spotlessCheck detekt` and
  `git diff --check`; focused commands below are iteration checks, not substitutes.

## Task POLISH-01 — Shared rounded category boxes

**Status:** [x] Complete after user-authorized repair; original repair attempts: 1/1, followed by the explicit continuation below.

**SOL candidate — 2026-09-15.** Shared 6dp control and 8dp interactive-card
shape tokens, the reusable whole-box category control, icon/count/status content,
keyboard semantics, documentation alignment and focused regressions are present in
the working tree. `spotlessApply` passed. The focused verification failed during
`:compileKotlin` at `ProjectSummaryVisuals.kt:110`: the refactored
`SummaryIssueIcon` still calls experimental `TooltipArea` but lost its
`ExperimentalFoundationApi` opt-in when the annotation moved to the new icon
helper. Astra High repair 1/1 is limited to restoring the correct opt-in (and any
directly resulting compile/test correction) in the listed targets, then rerunning
the focused command and the required desktop gate. No application test executed
past compilation; no card is accepted yet.

**Astra repair and final review — 2026-09-15.** Restored the experimental API
opt-in to `SummaryIssueIcon`. The first focused run then exposed three Summary
regressions because the extracted icon had replaced the existing accessible
names `Performance Issues` and `Security Issues`; restoring the wrapper's metric
label completed the same icon-extraction repair. No attempt counter was reset.

- Focused verification above, with
  `-PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-01"`: **passed, 44 tests**.
- `./scripts/desktop-gradle.sh test spotlessCheck detekt`: **failed, 455/456 tests
  passed**. `AnalysisFileStatusTest.categoryColorsFollowAnalysisOutcomeRegardlessOfFindingCount`
  fails at line 123: `Bugs must be visible at 1280`. That existing regression still
  requires visible category titles and Completed copy removed by this card; its
  assertions need migration to icon semantics while preserving outcome-color and
  actual visibility checks. It was omitted from the card's target list.
- The combined gate stopped before lint; separate
  `./scripts/desktop-gradle.sh spotlessCheck detekt`: **passed**, zero Detekt smells.
  `git diff --check`: **passed**.
- **Visual review failed independently of the outdated test.** In
  `desktop/build/reports/ui-polish/polish-01/analysis-progress-1280-1.5.png`, only
  the full-width Bugs box is visible. `AnalysisCategoryPanels.kt:34` passes row
  weight to the inner action surface, but `ChromeControls.kt:164` wraps it in an
  unweighted TooltipArea. Performance/Security render outside the available row.
  The focused test invokes semantics directly and misses the offscreen controls.
  A repair must size the immediate row children and assert all three clickable
  bounds and keyboard activation, not merely semantics-tree presence.
- Keyboard-focus category-name disclosure and selected-state semantics also need
  explicit verification before accepting the new shared box. No native-window,
  packaging or screen-reader acceptance is claimed by these offscreen results.

The candidate remains uncommitted. The app confirmed the scheduler **Paused**;
POLISH-02–06 have not started. Failure evidence is retained in `docs/errors.log`.
The user's subsequent request, “repair the task POLISH-01,” authorizes this
continuation beyond the original repair limit. Keep the earlier failures above.
Repair the tooltip surface's layout ownership, verify real pointer/keyboard
interaction and selected/focus disclosure, and migrate the affected outcome test.
`AnalysisFileStatusTest.kt` is added below because its existing category regression
exercises the changed production control. No later card starts during this repair.

**Accepted repair — 2026-09-15.** The manual continuation ran on the current task's
`gpt-6-astra` / `xhigh` setting (verified in its turn metadata); this differs from
the scheduler's Astra High repair setting, which remains unchanged. The original
repair count and failures above remain historical evidence.

- Reproduced the layout defect before repairing it with
  `./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.DesktopVisualLayoutTest.analysisProgressAndResultLinksRemainReadableAcrossSupportedViewports'`:
  Performance had empty visible bounds at 1440×900. The clickable Row now owns
  the caller's sizing modifiers; its delayed hover/focus popup adds no sizing
  wrapper. Category names are available on hover/focus; selection has a checkmark
  and selected semantics independently of the focus outline.
- Migrated the existing outcome regression to category names, actual clickable
  bounds, counts and meaningful states. The first resumed combined selection
  passed 47/48 tests; the new stale-count assertion incorrectly expected the old
  count. Corrected it to require an em dash, preserving the existing state owner.
  Visual review also found missing window dimensions in the offscreen fixture;
  supplying its actual window size now permits checking tooltip placement.
- `./scripts/desktop-gradle.sh spotlessApply test --tests 'io.miniorca.desktop.ChromeControlsTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.AnalysisFileStatusTest' -PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-01-repair"`:
  **passed, 48 tests**, zero failures/errors/skips.
- `./scripts/desktop-gradle.sh test spotlessCheck detekt`: **passed, 456 tests**,
  zero failures/errors/skips and zero Detekt smells. `git diff --check`: **passed**.
- Reviewed production renders against the dark reference: all three equal-size
  boxes remain visible at wide, 1000/999dp, 800×650 and 1280×600 views, 125/150%
  text and the 640/639dp row/column boundary. Verified whole-box pointer clicks,
  Tab order, Enter/Space exactly-once activation, focus/hover names, independent
  selection, empty/failed/stale/unknown states and removal of redundant copy.
  Offscreen evidence is under `desktop/build/reports/ui-polish/polish-01-repair/`;
  native-window, packaging and spoken screen-reader checks were not performed.
- Final diff preserves source/provider/execution guards and the existing work;
  no commits or later-card implementation. Scheduler restoration uses SOL High
  for the next POLISH-02 wake and Astra High for its configured repair.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — shared shape tokens.
- `desktop/src/main/kotlin/io/miniorca/desktop/ChromeControls.kt` — reusable rounded action surfaces.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisCategoryPanels.kt` — common icon/count/progress boxes and whole-box activation.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryVisuals.kt` — reuse category icons without nested focus targets.
- `desktop/src/test/kotlin/io/miniorca/desktop/ChromeControlsTest.kt` — action/focus behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — real component interaction and shape checks.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisFileStatusTest.kt` — preserve outcome colors and visible category/count/state behavior with the new icon presentation.
- `PLAN.md` — concise current product decisions and links to this queue, preserving history.
- `tasks/README.md` — current scheduler procedure, with earlier grants clearly historical.
- `desktop/UI_DESIGN_GUIDELINES.md` — document the requested rounding and copy refinement.
- `desktop/AGENTS.md` — align its geometry sentence with the updated guideline.

**Inputs / dependencies**
- None. Use the existing dark reference and current shared Jewel controls.

**Implementation rules**
- Refine the existing system: approximately 8dp category/result surfaces and 6dp
  contained controls. Keep structural panes flat, spacing dense and semantic colors
  intact; avoid wrapping every section in another card. Update the conflicting
  blanket prohibition on rounded cards only for these requested interactive surfaces.
- Reuse one responsive box component: category icon, count, optional existing Bugs
  priority breakdown and useful progress/state. Icons replace repeated titles;
  expose category names on hover/focus and through accessibility semantics.
- Remove the nested **Open results** button and repeated **finding(s)** caption.
  The entire box activates once by mouse, Enter or Space with visible focus.
  A selected category stays identifiable. Unknown counts remain an em dash.
- Keep important state visible without color alone; omit routine **Complete /
  Completed** copy where redundant. Keep completed-empty distinguishable from
  missing, failed or partial work. Remove superseded rendering helpers as migrated.

**Verification command**
`./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ChromeControlsTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' --tests 'io.miniorca.desktop.AnalysisFileStatusTest'`

## Task POLISH-02 — Summary alignment, navigation and live data

**Status:** [x] Complete after Astra High repair 1/1. POLISH-03 is next.

**SOL candidate — 2026-09-15.** Wired Summary's three shared category boxes to
the real workspace selector and the current run's complete section map, moved the
compact status to the trailing header edge, and preserved live counts plus report
loading/error state. Added current-run recomposition, whole-box navigation,
identity/evidence and polling regressions. This turn ran on the configured
`gpt-5.6-sol` / `high` setting.

`./scripts/desktop-gradle.sh spotlessApply test --tests 'io.miniorca.desktop.ProjectSummaryPaneTest' --tests 'io.miniorca.desktop.ProjectSummaryIssuesTest' --tests 'io.miniorca.desktop.DesktopAnalysisWorkflowTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-02"`
passed **75/76 tests**. The sole failure is
`DesktopVisualLayoutTest.summaryMetricFlowKeepsEveryCoverageBoxVisibleAndRemovesPartialText`:
its old assertion expects **Partial** to be hidden. The shared box now visibly
labels that meaningful state, as the current card and AGENTS rules require.
Astra repair 1/1 is limited to correcting that superseded expectation, rerunning
the focused command and required desktop gate, reviewing the POLISH-02 diff and
renders, and recording acceptance or a concrete blocker. No product threshold or
state distinction may be weakened, and no later card may start in this repair.

**Astra repair and acceptance — 2026-09-15.** Verified actual task configuration
`gpt-6-astra` / `high`. The handoff had switched the model but stopped after a
status reply; no repair process was active when the user asked. Resumed the same
repair, renamed the outdated Partial regression and asserted that its text fits.
Earlier SOL iterations also exposed incomplete test call-site migration and three
old accessible-name expectations; those were corrected before the final 75/76 run.

- Focused verification command above, without `spotlessApply` and with
  `-PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-02-repair"`:
  **76 tests, zero failures/errors/skips**.
- `./scripts/desktop-gradle.sh test spotlessCheck detekt`: **PASS**, all **460
  desktop tests**, zero failures/errors/skips; a subsequent status recovery run
  confirmed unchanged inputs and all tasks up-to-date.
- Reviewed production-component renders for the 1440px completed/live Summary
  and 800px Summary at 150% text: trailing status, fitting category boxes, visible
  partial/paused states and retained metrics. Pointer regression activates every
  category and updates counts within the same mounted Summary.
- Reviewed real workspace navigation, shared run/section ownership, report
  identity checks, stale evidence and removed duplicate icon rendering. Existing
  polling already publishes all categories; no additional timer or backend change.
- `git diff --check`: PASS. Native-window, packaged-app and screen-reader checks
  were not repeated; this acceptance covers desktop tests and offscreen renders.

Only POLISH-02 was repaired. Resume the existing scheduler for POLISH-03 and
restore SOL High after recording acceptance; no commit or later-card edits.

**Target files**
- `PLAN.md` and `tasks/README.md` — narrowly necessary updates to their existing queue status and scheduler handoff records.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — trailing Updated status and shared boxes.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryIssues.kt` — current category metrics and removal of duplicate box rendering.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryVisuals.kt` — status presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — real category navigation and current state wiring.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — shared progress presentation where necessary.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflow.kt` — fix an evidenced progress-publication gap only if needed.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryPaneTest.kt` — summary lifecycle states.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryIssuesTest.kt` — count/evidence identity agreement.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflowTest.kt` — progressive updates and late-result rejection.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — alignment and category activation.

**Inputs / dependencies**
- POLISH-01; existing `AnalysisRun`, section results and presenter-owned polling.

**Implementation rules**
- Align **Updated** to the right edge of its available header; keep meaningful
  alternative states. Remove redundant **Complete** text and routine duplicate
  labels around metrics without discarding purpose, modules or explanation content.
- Summary Bugs, Performance and Security boxes use the shared component and open
  their actual workspace. Navigation performs no provider request or source edit.
- Summary and Analysis derive live counts/progress from the same current run.
  Reuse existing polling, which already loads all three result categories, rather
  than adding a timer or fetching in composition. Preserve cached useful content.
- Test progress changing while Summary remains open, pending result reads,
  mismatched count/evidence snapshots, completion, failure, project/revision changes
  and replacement runs. Late data cannot overwrite the current project/run.

**Verification command**
`./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.ProjectSummaryPaneTest' --tests 'io.miniorca.desktop.ProjectSummaryIssuesTest' --tests 'io.miniorca.desktop.DesktopAnalysisWorkflowTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

## Task POLISH-03 — Compact Analysis and collapsible exclusions

**Status:** [x] Complete after Astra High repair 1/1. POLISH-04 is next.

**Continuation:** The user's status/commit request arrived before the repair ran.
Resume this same attempt with Astra High; commit each accepted card and include
the hash in its completion notification. Add `PLAN.md` and `tasks/README.md` as
necessary status/commit-policy targets before editing their existing records.
The full gate exposed one additional dependent test:
`DesktopContrastTest.analysisHeadersRenderDistinctLabeledLifecycleStates` still
requires the removed Completed label. Add
`desktop/src/test/kotlin/io/miniorca/desktop/DesktopContrastTest.kt` to this card's
targets to check the new header's rendered contrast and meaningful category states
without restoring redundant completion text. Keep all contrast thresholds intact.

**SOL candidate — 2026-09-15.** Replaced the editable Run limits UI with the
existing `AnalysisRunLimits(100, 900, 2)` defaults, condensed active/last-run facts,
removed the Run details disclosure, kept operational failures visible, and made
Files a project-keyed disclosure that starts closed. File rows retain the path,
checkbox, status and concise reason while removing per-file stage details.

The focused command failed at `:compileKotlin` in `WorkspacePanes.kt:112` because
`items(presentation.failures)` resolved to the count overload: the list overload's
`androidx.compose.foundation.lazy.items` import is missing. Astra repair 1/1 is
limited to restoring that import, completing the specified focused tests and
necessary expectation updates for the intentional compact UI, running the full
desktop gate and `git diff --check`, reviewing renders/diff, and recording
acceptance or a concrete blocker. No POLISH-04 work may start in this repair.

**Acceptance — 2026-09-15.** Verified the actual repair model as
`gpt-6-astra` / `high`. Restored the missing list import, moved current/last-run
facts into the first header, kept lifecycle actions and explicit failure reasons,
and moved category coverage into the existing rounded boxes. Files now starts
collapsed, retains selection/filter/bulk actions, shows save failures and run
restrictions while closed, and resets local disclosure state for another project.
Start and retry still use the existing 100-file / 900-second / 2-attempt defaults;
the daemon admission and consent workflow is unchanged.

- Card's focused tests: **58 passed**. With the dependent contrast suite added:
  **63 passed**. Commands used `spotlessApply` and
  `-PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-03-repair"`.
- First full gate: **461/462 passed**; the old contrast test required Completed.
  Migrated it to rendered header contrast and meaningful category states, retaining
  all contrast thresholds. Final `./scripts/desktop-gradle.sh test spotlessCheck
  detekt`: **462 passed**, zero failures/errors/skips; Spotless and Detekt passed.
- Reviewed production renders at wide, 1000/999px, 800×650 and 1280×600 layouts
  including 150% text, collapsed/expanded Files, long paths, errors and lifecycle
  states. Tests prove local disclosure has no requests or saves, exclusions persist,
  project changes reset disclosure, and active-run selection remains locked.
- Reviewed the complete diff and ran `git diff --check`. Native-window, packaged
  app and screen-reader checks were not repeated; no native acceptance is claimed.
- User-authorized catch-up commit for POLISH-01/02: `d67ab5a`. POLISH-03 receives
  its own local commit after acceptance. Resume the scheduler for POLISH-04 with
  the new per-task commit policy, then restore SOL High.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — compact run header, controls and removal of Run details / Run limits.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisCategoryPanels.kt` — shared live coverage presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — necessary run facts and failure presentation.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisFileSelector.kt` — local disclosure and compact selection rows.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — active/last-run and failure behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisFileSelectionTest.kt` — preserve saved exclusions and active-run locking.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — disclosure, default limits and responsive controls.

**Inputs / dependencies**
- POLISH-01 and POLISH-02; existing admission, selection and bounded run contracts.

**Implementation rules**
- Use one first line: **Last run** with available stored time/summary when idle,
  or current run progress while active, with Start/Pause/Resume/Cancel reachable.
  Show only relevant live facts (processed/remaining work and useful elapsed time),
  using the same boxes as Summary. Never invent missing dates or progress.
- Remove **Run details** and migrate useful coverage into the boxes/header;
  keep failure reasons and recovery visible in their owning flow. Remove duplicate
  introductory text and count captions; Analysis remains a progress page.
- Remove **Run limits** and its editing state. Start uses existing
  `AnalysisRunLimits` defaults; preserve the admission's actual scope/destination
  disclosure, daemon limits, paused/partial meaning and explicit continuation.
- Make the exclusions/file-selection section collapsible, initially closed, with
  a useful selected/excluded count. Expand/collapse only changes local UI state.
  Keep selection/filter/bulk actions; remove per-file Details toggles and verbose
  stage breakdowns. Retain path, checkbox and concise actionable disabled/error
  reasons. Saving failures and active-run restrictions remain visible when closed.
- Verify collapse does not save, refresh, start work or lose exclusions; selection
  still persists through its existing explicit action and resets for a new project.

**Verification command**
`./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.AnalysisFileSelectionTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

## Task POLISH-04 — Repair Summary diagrams end to end

**Status:** [x] Complete with SOL High; repair attempts: 0/1. POLISH-05 is next.

**Reproduction.** Existing renderer, grouped-flowchart, branched-sequence and SVG
decode checks pass, so the GraalJS/SVG pipeline is working. The first-frame UI
regression `validDiagramDisclosureAcceptsTheFirstClickWhileRenderingStarts` fails:
**Show diagram** is disabled while the valid source renders asynchronously, so an
immediate user click is discarded. This deliberate failing regression establishes
the reported interaction defect and does not consume the Astra repair allowance.

**Acceptance — 2026-09-16.** Valid Mermaid sources now accept the first disclosure
click while the offline render starts; the expanded disclosure visibly transitions
from Rendering to the decoded image. Loading stays hidden while collapsed. Invalid
inputs still disable disclosure and retain the bounded error/source, and replacing
one with a valid diagram recovers without a model or network request. Fenced input
also accepts case/spacing and CRLF variants. No JS bundle or dependency changed.

- Focused command: **59 passed**, zero failures/errors/skips, covering multiline,
  grouped/branching flowcharts, sequence branches, image pixels, legacy arrows,
  fenced/prose input, first-click behavior, zoom/source and failure recovery.
- `./scripts/desktop-gradle.sh test spotlessCheck detekt`: **464 passed**, zero
  failures/errors/skips; Spotless and Detekt passed. `git diff --check`: PASS.
- `./scripts/desktop-gradle.sh createDistributable`: PASS. The first manual smoke
  invocation omitted the package launcher's Skiko resource path and failed before
  rendering; the corrected JBR 25 invocation used the packaged app jars and
  `Contents/app` native resource path. Updated grouped-flowchart and branched-
  sequence `MermaidRuntimeSmokeKt`: **Packaged Mermaid rendering passed.**
- Reviewed 1440px and 800px/150% production renders plus the initial, unavailable
  and recovery states: labels, nodes and edges are legible; source remains
  selectable. Native pointer/focus and screen-reader checks were not repeated.

Add `PLAN.md` and `tasks/README.md` as narrow status/commit-record targets. Commit
this accepted card locally and resume the scheduler for POLISH-05.

**Target files**
- `PLAN.md` and `tasks/README.md` — accepted-card status and scheduler commit record.
- `desktop/src/main/kotlin/io/miniorca/desktop/MermaidDiagram.kt` — input/disclosure/render state and recovery.
- `desktop/src/main/kotlin/io/miniorca/desktop/MermaidRenderer.kt` — embedded renderer failure if reproduced here.
- `desktop/src/main/kotlin/io/miniorca/desktop/MermaidImage.kt` — SVG/image conversion failure if reproduced here.
- `desktop/mermaid/renderer.js` — authored SVG adapter, only if implicated.
- `desktop/mermaid/runtime.js` — embedded runtime adapter, only if implicated.
- `desktop/src/main/resources/mermaid/renderer.js` — regenerated bundle only, never hand-edit.
- `desktop/build.gradle.kts` — packaged runtime wiring only if required by reproduction.
- `desktop/src/test/kotlin/io/miniorca/desktop/MermaidRendererTest.kt` — regression using the failing input/runtime path.
- `desktop/src/test/kotlin/io/miniorca/desktop/MermaidRuntimeSmoke.kt` — packaged-runtime regression.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProjectSummaryPaneTest.kt` — input/prose compatibility.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — working expand/collapse and visible rendering.

**Inputs / dependencies**
- POLISH-02. Root cause is unverified: reproduce a failure before choosing which
  renderer targets to change. Existing tests render simple graphs and sequences;
  passing those alone does not resolve the user's report.

**Implementation rules**
- Trace Summary Architecture / Flows through input extraction, GraalJS, SVG
  conversion and **Show diagram**. Use a captured failing local input if available,
  otherwise representative supported inputs and native/runtime reproduction.
- Valid supported diagrams must display legible nodes, edges and labels, with
  usable expand/collapse, zoom and selectable source. Check fenced Mermaid,
  multiline graphs, branching/sequence diagrams and supported legacy arrow chains.
- Preserve complete prose and explicit unsupported/malformed errors. No stuck
  loading state, silent empty rendering, network renderer or new model request.
  Keep resource bounds, cancellation and no active links/HTML/external resources.
- Rebuild an affected JS bundle using `npm ci --prefix desktop/mermaid --ignore-scripts`
  then `npm run build --prefix desktop/mermaid`; preserve pinned dependencies.
  If an application runtime defect is involved, build the distributable and run
  the existing `MermaidRuntimeSmokeKt` with its actual bundled runtime/jars.

**Verification command**
`./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.MermaidRendererTest' --tests 'io.miniorca.desktop.ProjectSummaryPaneTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

## Task POLISH-05 — Cleaner result pages and one Prepare fix action

**Status:** [x] Complete after Astra High repair 1/1.

**SOL candidate — 2026-09-16.** Result pages now use the shared category boxes,
rounded selectable rows, compact evidence copy and one **Prepare fix** action.
The focused command failed during `:compileKotlin` before tests ran:
`FindingsPresentation.kt:189` resolves the semantics receiver's `selected` name
to the surrounding selected row value, producing a val reassignment/type mismatch.
Astra High repair 1/1 is limited to disambiguating that selected-state semantics,
then completing focused/full checks, visual review and the authorized task commit.

**Repair continuation — 2026-09-16.** Qualifying `this.selected` restores compilation.
The focused suite reaches 63 tests: 61 pass; two legacy visual expectations still
require the removed Open source action and Completed header. Migrate these checks
within this repair, retaining action isolation and all meaningful lifecycle states.
Review also requires retaining evidence distinctions in disclosures and removing
the now-unused source action wiring. Add the exact dependent targets below before
editing; the repair count remains 1/1.

**Acceptance — 2026-09-16.** Verified model `gpt-6-astra`, effort `high`.
All three result pages use rounded selectable rows, shared selected category boxes
and local cross-category navigation. Routine provenance/lifecycle labels and the
Open source action are removed; Prepare fix retains existing eligibility owners.
Evidence distinctions remain in disclosures, meaningful states remain visible,
and narrow layouts retain Back to results. Removed the obsolete action callbacks
and presentation helpers, including their dependent app/test inputs.

- Focused command with `-PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-05"`:
  **63 passed**, zero failures/errors/skips. Actual pointer clicks exercise all
  category boxes and rows; preparation callbacks fire only on explicit actions.
- Initial full gate: **464/465 passed**. An unchanged diagram regression observed
  failed-state semantics before the disabled button recomposed. Its fixture now
  renders that transition before asserting disabled, and no longer waits for a
  transient Loading state on recovery. No diagram implementation changed.
- Final `./scripts/desktop-gradle.sh test spotlessCheck detekt`: **465 passed**,
  zero failures/errors/skips; Spotless and Detekt passed, zero smells.
  `git diff --check`: PASS.
- Reviewed production renders at 800px and 1280px/150% text against the dark
  reference: category selection, readable rows, detail/back and Prepare fix are
  visible. Matrix checks cover wide/narrow, loading, partial, stale, failed and
  empty results. Native-window and screen-reader checks were not repeated.

Scheduler resumed for POLISH-06. Save this accepted card in its required local
commit and return the task configuration to SOL High without starting another card.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — remove unused result-source callbacks.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAcceptanceFixture.kt` — migrate result action inputs.
- `desktop/src/test/kotlin/io/miniorca/desktop/BugsWorkspaceStateTest.kt` and `desktop/src/test/kotlin/io/miniorca/desktop/DesktopIntegrationCoverageTest.kt` — migrate removed presentation helpers without weakening classification checks.
- `PLAN.md` and `tasks/README.md` — current repair, commit and scheduler status.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — common rows/detail layout, metadata and actions.
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisResultsPane.kt` — shared first-line boxes and removal of repeated banners.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — Bugs detail cleanup and category navigation inputs.
- `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` — separate retained evidence from routine visible labels.
- `desktop/src/main/kotlin/io/miniorca/desktop/PerformanceWorkspace.kt` — typed performance rows/detail and Prepare fix wording.
- `desktop/src/main/kotlin/io/miniorca/desktop/SecurityWorkspace.kt` — typed security rows/detail and guarded fix preparation.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — shared category/header navigation wiring.
- `desktop/src/test/kotlin/io/miniorca/desktop/FindingsPresentationTest.kt` — selection versus explicit fix preparation.
- `desktop/src/test/kotlin/io/miniorca/desktop/PerformanceWorkspaceTest.kt` — evidence and eligibility behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/SecurityWorkspaceTest.kt` — evidence and eligibility behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — all three result pages, narrow back navigation and actions.

**Inputs / dependencies**
- POLISH-01, POLISH-02 and POLISH-03; existing semantic, performance and security
  evidence types and their action/eligibility owners.

**Implementation rules**
- Reuse the same three category boxes in the first line, with current category
  selection, real counts/status and local cross-category navigation. Keep the
  Analysis entry action accessible without repeating titles and coverage paragraphs.
- Remove routine labels from lists and details: **AI suggestions**, **Source AI**,
  **Confidence suggested**, **Performance review**, **AI suspicion**, **Open**,
  **Fresh**, **Not measured**, **Unverified** and their composite badges.
  Preserve severity, useful explanations, source location and actual evidence.
- Convey material evidence differences through the existing evidence/verification
  content and relevant action state, without reinstating the removed badge wall.
  Never turn a suggestion into a verified bug, an unmeasured hypothesis into a
  benchmark result or Security's empty result into assurance. Show stale/failed/
  partial conditions and blocked-action reasons where needed to decide or recover.
- Improve list spacing/alignment with shared rounded selectable rows, readable
  titles, restrained severity and path hierarchy, hover and distinct focus/selection.
  Preserve stable selection keys, long-content access and responsive detail panes.
- Remove **Clear selection** in wide details. Keep **Back to results** in narrow
  layouts so removing the action cannot trap users in the detail pane.
- Remove the redundant **Open source** button; retain exact source location.
  Use **Prepare fix** consistently, including the existing Performance optimization
  preparation callback. It opens/prepares the existing Assistant workflow only;
  do not auto-generate/apply, navigate on ordinary row selection or weaken eligibility.
  Remove obsolete callbacks only after checking all supported callers.

**Verification command**
`./scripts/desktop-gradle.sh test --tests 'io.miniorca.desktop.FindingsPresentationTest' --tests 'io.miniorca.desktop.PerformanceWorkspaceTest' --tests 'io.miniorca.desktop.SecurityWorkspaceTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

## Task POLISH-06 — Validate the complete flow and close the queue

**Status:** [x] Complete with SOL High; repair attempts: 0/1. Queue closed.

**Execution — 2026-09-16.** Verified `gpt-5.6-sol`, effort `high`; clean baseline
at POLISH-05 commit `296657f`. Final acceptance adds direct accessibility and
keyboard coverage for selected category boxes, local row inspection and explicit
Prepare fix, then runs the complete rendered matrix and repository gates below.

**Acceptance — 2026-09-16.** Added direct accessible-name/selected-state checks
for every category box and keyboard separation of navigation, row inspection and
explicit Prepare fix. Corrected the running visual fixture so its category state
matches the running header; no production code changed in this card.

- `./scripts/desktop-gradle.sh test spotlessCheck detekt
  -PvisualOutput="$PWD/desktop/build/reports/ui-polish/polish-06"`: **467 passed**,
  zero failures/errors/skips; Spotless and Detekt passed with zero smells.
- `./scripts/validate.sh`: PASS, all nine stages. Go formatting/tests/race/vet,
  contracts/quality, 55 dispatcher tests (one existing opt-in conformance skip),
  desktop static analysis and desktop tests passed. No live provider call ran.
- `git diff --check`: PASS. Reviewed 390 production-component captures across the
  required sizes/scales/states, long failures, live updates, exclusions, diagrams,
  result detail/back and Prepare fix. Local navigation/disclosures kept workflow
  callbacks at zero until the explicit fix action.
- Native-window focus, screen-reader speech and system display scaling were not
  repeated in this scheduled environment. POLISH-04's packaged Mermaid evidence
  remains valid because its runtime/package inputs are unchanged.

All six cards are accepted. Save this closure in its required local commit and
pause `mini-orca-ux-implementation`; no later task is queued.

**Target files**
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — complete production-component acceptance matrix.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAccessibilityTest.kt` — meaningful names, selected/expanded states and focus.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — relevant navigation regressions.
- `desktop/README.md` — resulting user-visible interactions and any runtime procedure changes.
- `desktop/KEYBOARD_SMOKE_CHECKLIST.md` — changed native checks.
- `docs/RELEASE_ACCEPTANCE.md` — actual visual/native/check results and limitations.
- `PLAN.md` — concise accepted scope/status with evidence link.
- `tasks/README.md` — final scheduler state.
- `docs/tasks.md` — final card evidence and completion state.

**Inputs / dependencies**
- POLISH-01, POLISH-02, POLISH-03, POLISH-04 and POLISH-05.

**Implementation rules**
- Review all changed production panes with optional details collapsed against the
  existing reference/guidelines. Check wide, 1000/999dp, 800×650, 1280×600,
  100/125/150% text, long paths/errors and empty/running/partial/stale/failed states.
- Exercise mouse/keyboard box navigation, live updates without leaving the page,
  exclusions, diagram show/zoom, result detail/back and Prepare fix. Verify absence
  of provider/execution/source-write side effects in local inspection controls.
- Capture offscreen evidence using the maintained `-PvisualOutput` procedure.
  Perform the affected native keyboard/window checks separately; fixture success
  does not prove OS focus or packaged diagram behavior. Report unavailable checks.
- Run relevant final gates; use `./scripts/validate.sh` if implementation required
  any cross-stack changes. Fix change-caused failures within this queue's repair
  policy. No baseline blessing, unrelated refactor or invented passing evidence.
- Record final review and pause this scheduler through the app when complete.

**Verification command**
`./scripts/desktop-gradle.sh test spotlessCheck detekt`

Also run `git diff --check`; inspect rendered artifacts and record native results.

---

## Historical queue — completed 2026-09-12

Everything below is retained historical evidence. Its unchecked intermediate
notes, imperative instructions, model choices and commit grants are inactive.
Only POLISH-01–06 above belong to the 2026-09-15 request.

# Mini-Orca UX, project analysis and terminal implementation

Implement the approved 2026-09-11 product scope in [PLAN.md](../PLAN.md): readable
model results, project-wide analysis with separate result pages, prominent Go
function creation and a real terminal replacing the three duplicated bottom tools.

## Authorized execution

The user requested task preparation and scheduled execution with **GPT-6 Astra,
Extra High** (`gpt-6-astra`, `xhigh`). These 17 ordered cards are the current
implementation source of truth. [PLAN.md](../PLAN.md) owns product decisions and
the concise status ledger; [the execution guide](../tasks/README.md) defines the
per-wake procedure. The completed cleanup specification is preserved in
[the historical archive](history/cleanup-tasks-2026-09-10.md).

- Work in the current `codex/autopilot` checkout and preserve the existing UI
  changes. Re-read `AGENTS.md` and UI guidelines before relevant work.
- Process at most one card per scheduled wake. Select the first unchecked card;
  never skip an incomplete predecessor. If work spans wakes, resume its recorded
  stage rather than starting a second writer or repeating completed work.
- Use Astra Extra High for implementation and review of the diff. No additional
  agents, standalone jobs or historical dispatcher are required by this queue.
- A passing verification command plus a production-quality diff review is required
  before checking a task. Run the full desktop test suite before accepting any
  desktop-changing card, as required by `AGENTS.md`, using cached results when valid.
- The user authorized one local commit per completed task on 2026-09-11. After
  required checks and diff review pass, include that task's implementation and
  related checklist/status updates in one commit with its task ID in the subject.
  Inspect the staged diff and commit only that scope; preserve unrelated staged
  changes and exclude unrelated pre-existing working-tree edits. Never use a
  blanket add/commit of the repository. Verify/report the commit hash before
  advancing. If commit creation fails, record the pending commit stage and pause;
  on recovery check Git history before retrying to avoid duplicate commits.
- Allow the initial implementation attempt and at most two focused correction
  attempts for a concrete failure. Retain the exact command, exit/result, relevant
  diagnostics and correction history in the card. Do not reset repair accounting
  on the next wake. Exhausted failures go to `docs/errors.log` and pause the scheduler.
- Edit the listed targets and their required behavior tests. Administrative updates
  to this checklist, `PLAN.md` status and `docs/errors.log` are allowed. If a necessary
  coupled file was omitted, record the concrete compile/behavior dependency and
  narrowly correct the target list before editing it; this is task preparation,
  not permission for unrelated scope expansion.
- Preserve one-file preview/Review/Apply, source/revision checks, consent, loopback
  policy and benchmark trust. No pushes, releases, live Mini-Orca model
  evaluation, destructive cleanup or changes to historical scheduler grants are
  authorized. Dependency resolution and fake-provider/temporary-project tests are
  part of implementation; do not edit generated build output by hand.
- Advance only after required checks pass. Missing mandatory native evidence or
  incompatible terminal packaging keeps that card incomplete; report the specific
  blocker, preserve the candidate and pause rather than shipping a placeholder.

## Validation environment

Use the documented JDK 21 launcher and Java/JBR 25 toolchain. Task commands are
from the repository root. The scheduler prompt records verified local toolchain
paths; keep machine-specific locations out of versioned configuration. Direct
wrapper commands need the Java 21 launcher and a discoverable Java 25 toolchain;
passing `-Porg.gradle.java.installations.paths=<verified-java25-home>` is a runtime
location override, not a replacement for the prescribed test selection.

All cards are initially unchecked. Product decisions D1–D5 are confirmed in PLAN.md.
Task completion requires both the specified validation and its authorized local
commit; a checked card with a pending/failed commit must be finished before the
next card starts. Commit hashes can be reported in the task's final response or
an ignored local execution receipt; do not amend solely to embed a commit's own hash.

## Task UX-01 — Establish readable result primitives

- [x] UX-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — semantic typography and result accents.
- `desktop/src/main/kotlin/io/miniorca/desktop/ChromeControls.kt` — reusable labeled headers/badges/disclosures.
- `desktop/src/main/kotlin/io/miniorca/desktop/ModelResultContent.kt` — new bounded freeform-text presentation component.
- `desktop/src/test/kotlin/io/miniorca/desktop/ModelResultContentTest.kt` — new formatting/fallback behavior tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopThemeTest.kt` — contrast and role coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — expose the existing test-only Compose fixture for reuse by the new component tests; no existing fixtures or assertions change.
- `desktop/UI_DESIGN_GUIDELINES.md` — accepted hierarchy, disclosure and scrolling rules.

**Inputs / dependencies**
- D3; existing working-tree Context changes and the referenced dark mock.

**Implementation rules**
- Extend the existing design system; no palette migration or repeated elevated cards.
- Support the small useful formatting subset: paragraphs, emphasis, lists, inline code and fenced code. Treat unsupported markup as text; no HTML/webview, remote image loading or executable content.
- Use selectable text, visible focus and semantic labels; retain the complete underlying response when using a collapsed preview.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ModelResultContentTest' --tests 'io.miniorca.desktop.DesktopThemeTest'`

**Execution record**
Completed 2026-09-11 at the user's request. Validation and diff review passed;
the local task commit is identified by UX-01 in Git history and the execution receipt.
Captured the existing worktree and empty index under the
ignored `.mini-orca/autopilot/ux/UX-01/baseline/` directory. The narrowly added
test target exposes the existing offscreen renderer so the new component can
exercise real layout, disclosure and keyboard behavior without duplicating it.
The planning queue, execution guide and cleanup archive prepared in this task
are related documentation for the first implementation commit.

Initial prescribed verification exited 1: 19 tests, one failure in
`DesktopThemeTest.resultRolesKeepHeadingsLabelsCodeAndBadgeTextReadable` at the
badge contrast assertion. Transparent badge fills on selected rows reduced
success/error label contrast below 4.5:1. Correction 1 resolves badge fills
against the panel with an opaque result; the test checks that production color
and retains all supported-surface label assertions. The eight formatter/production
render tests passed, including disclosure activation through the keyboard and
replacement-response reset. Logs and renders are in the ignored UX-01 evidence
directory. A formatter invocation first used a relative init-script path that
Gradle resolved under `desktop/`; rerunning with its absolute path succeeded
before verification and required no source correction.

Correction-1 verification exited 1: 19 tests, one remaining failure in the same
test's raw severity-color assertion on a selected surface. The badge fix passed;
the assertion incorrectly bypassed its opaque background. Correction 2 separates
uncontained prose roles from badge roles and verifies badge contrast using the
actual resolved background on every supported parent surface. Severity labels
are not weakened or removed from the checks. Review also retained spaced emphasis
delimiters literally so multiplication in prose is not reformatted.

Final prescribed verification exited 0: 19 tests, no failures/errors/skips.
`./desktop/gradlew -p desktop test detekt spotlessCheck` exited 0: all 362 desktop
tests passed with no failures/errors/skips, no static-analysis findings, and clean
formatting. Both commands used the documented Java 21 launcher/Java 25 toolchain
runtime override and wrote offscreen renders to ignored local evidence paths.
Reviewed production renders at narrow/wide widths and 150% text, including visible
keyboard focus, expanding/collapsing the full response, wrapped labels and existing
shared chrome/summary views. This is offscreen Compose evidence, not native
screen-reader acceptance. No daemon behavior changed; Go tests and live provider
tests were not run. No dependency, configuration or migration steps are needed.

Diff review checked literal fallbacks, formatting bounds, response reset, single
scroll ownership, existing status semantics and retained full response data.
`git diff --check` passed; out-of-scope baseline files are unchanged. The local
commit includes only UX-01 implementation, fixture visibility and related planning
records; pre-existing desktop edits are excluded. UX-02 remains unchecked.

The exact commit candidate was additionally validated in an isolated source
snapshot with the pre-existing edits excluded: `test detekt spotlessCheck` exited
0, all 360 tests passed with no failures/errors/skips, and quality checks passed.
The two additional working-tree tests belong to the preserved Context changes.
All ten pre-existing desktop diffs were compared against the baseline and retained
exactly outside this commit. This check confirms UX-01 has no dependency on those
uncommitted changes.

## Task UX-02 — Apply the hierarchy to explanations and model responses

- [x] UX-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — summary, labeled explanation facts and technical disclosures.
- `desktop/src/main/kotlin/io/miniorca/desktop/AssistantToolWindow.kt` — distinguish user request, model response and candidate action.
- `desktop/src/main/kotlin/io/miniorca/desktop/EngineeringInsightPanel.kt` — readable labels and single-owner scrolling.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — concise project explanation with supporting detail.
- `desktop/src/main/kotlin/io/miniorca/desktop/ReviewEvidencePane.kt` — clearer evidence and blocked-action hierarchy.
- `desktop/src/main/kotlin/io/miniorca/desktop/ModelResultContent.kt` — remove extra paragraph boundaries exposed by real response lists; preserve literal line breaks and colored markers.
- `desktop/src/test/kotlin/io/miniorca/desktop/ModelResultContentTest.kt` — update the obsolete hanging-indent assertion; production line-spacing coverage is in DesktopVisualLayoutTest.
- `desktop/src/test/kotlin/io/miniorca/desktop/ContextToolWindowTest.kt` — structured result and state coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/AssistantToolWindowTest.kt` — conversation/draft distinction.
- `desktop/src/test/kotlin/io/miniorca/desktop/EngineeringInsightPanelTest.kt` — optional/stale insight behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — populated/empty/long-text production renders.

**Inputs / dependencies**
- UX-01.

**Implementation rules**
- Replace joined explanation facts with short labeled sections. Keep the meaningful answer visible before provenance/metadata.
- Do not change model payloads merely for visual formatting, add AI summaries, hide errors in closed details or turn inspection into a provider request.
- Preserve Actions / Explain / Details, read-only content, file/declaration identity and responsive drawers.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ContextToolWindowTest' --tests 'io.miniorca.desktop.AssistantToolWindowTest' --tests 'io.miniorca.desktop.EngineeringInsightPanelTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

**Execution record**
Completed 2026-09-11 by the authorized scheduler. Validation and diff review passed;
the local UX-02 commit is recorded in Git history and the execution receipt.
One focused correction was used. UX-01 commit and receipt verified. Both documented
Java toolchains remain installed. Captured the baseline and empty index in the
ignored `.mini-orca/autopilot/ux/UX-02/baseline/` directory; the ten pre-existing
desktop diffs are retained, with unrelated changes outside this task's commit scope.

Initial prescribed verification exited 1: 42 tests, one failure at
`EngineeringInsightPanelTest.kt:40`. The new full-response assertion compared
untrimmed fixture text with the existing trimmed insight presentation. Correction
1 preserves that normalization and compares its complete displayed content.
All other formatting, lifecycle, local-navigation and interaction tests passed.
Render inspection also exposed extra blank lines introduced by per-item paragraph
styles; the narrowly added shared-component targets remove those boundaries and
test actual line counts. The explanation-content component is package-internal
for direct production-render testing, keeping its new test independent of the
uncommitted Context-tab changes while the existing integration tests still cover
the current working-tree tabs and consent boundaries.

Commit preparation identified an overlapping dependency: the explanation status
labels previously existed only in the uncommitted Context work. The new lifecycle
badges require that helper, so its state mapping and current failure presentation
are included as related UX-02 code, using the committed result-accent token.
The existing Context tabs, layout defaults, theme/tab accents, documentation and
their unrelated tests remain uncommitted. The complete working-tree behavior is
preserved; only this coupled status presentation joins the task commit.

Final prescribed verification exited 0: 43 tests, no failures/errors/skips.
`./desktop/gradlew -p desktop test detekt spotlessCheck` exited 0: all 366 desktop
tests passed, no failures/errors/skips, no Detekt findings and clean formatting.
The isolated commit candidate also passed `test detekt spotlessCheck`: all 364
tests passed, with no failures/errors/skips or quality findings. The working-tree
suite contains two additional pre-existing Context tests. Commands used the
verified Java 21 launcher and Java 25 toolchain runtime override. Logs and renders
are retained in `.mini-orca/autopilot/ux/UX-02/`; the final focused/full runs followed
the commit-dependency adjustment and removal of misleading always-validate copy.

Reviewed production offscreen renders for narrow/wide panes and 150% text: labeled
explanation facts, literal user requests versus formatted model replies, long-text
disclosures, visible validation errors, prominent next actions and tight list
spacing. Tests cover source-disclosure reset, unchanged local navigation/consent
callbacks, stale/failed/empty states, read-only evidence and one parent scroll owner.
Diff review and `git diff --check` passed. Nine pre-existing desktop diffs are
retained exactly outside the commit; the Context shell/tabs and remaining context
content are also preserved, with the coupled status block accounted for above.
No API/model payload, daemon, dependency, configuration or migration changes.
Go tests, live providers and native OS/screen-reader acceptance were not run;
none is required for this presentation card. CREATE-01 remains unchecked.

## Task CREATE-01 — Expose creation in the normal file workflow

- [x] CREATE-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/EditorWorkspace.kt` — visible New function action with open-file identity.
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — file-level creation access when no symbol is selected.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — route all creation actions through the existing draft-discard boundary.
- `desktop/src/main/kotlin/io/miniorca/desktop/AssistantToolWindow.kt` — creation heading, name, behavior prompt and Generate label.
- `desktop/src/main/kotlin/io/miniorca/desktop/CommandPalette.kt` — consistent searchable creation names.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkflowToolWindows.kt` — explicit create-mode scope/status.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — forward the creation callback from DesktopApp to the production EditorWorkspace.
- `desktop/src/test/kotlin/io/miniorca/desktop/EditorWorkspaceTest.kt` — action discoverability and eligibility.
- `desktop/src/test/kotlin/io/miniorca/desktop/AssistantToolWindowTest.kt` — creation without a selected symbol.
- `desktop/src/test/kotlin/io/miniorca/desktop/CommandPaletteTest.kt` — matching creation entry.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — supply the new editor callback in production fixtures and verify keyboard/layout behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/WorkflowToolWindowsTest.kt` — cover the changed creation scope labels.

**Inputs / dependencies**
- UX-01; D4.

**Implementation rules**
- Reuse `requestCreateDeclaration`; opening the form does not generate or mutate anything.
- Focus the required name field first, then the behavior prompt. Name input must stay editable while the target is incomplete.
- Make unsupported-language/no-file states informative. Support an existing valid Go file with zero declarations; no existing symbol selection is required.
- A different active draft needs explicit discard. Cancel preserves it. Retain a secondary type-creation path.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.EditorWorkspaceTest' --tests 'io.miniorca.desktop.AssistantToolWindowTest' --tests 'io.miniorca.desktop.CommandPaletteTest'`

**Execution record**
Completed 2026-09-11 by the authorized scheduler. Required checks and diff review
passed after two focused corrections; the local CREATE-01 commit is recorded in
Git history and the ignored execution receipt. Verified UX-02 commit/receipt and both Java
toolchains. Captured baseline and empty index under ignored
`.mini-orca/autopilot/ux/CREATE-01/baseline/`. The narrow target additions cover
the required DesktopApp → DesktopShell → EditorWorkspace callback dependency,
existing production fixture call sites and changed creation-scope assertions.
Unrelated pre-existing Context/tab/layout work will remain outside this commit.

Initial prescribed verification exited 0. The full desktop suite passed all 373
tests, but `test detekt spotlessCheck` exited 1 at Detekt: `MiniOrcaApp` cyclomatic
complexity 72 exceeded the existing 65 limit. Correction 1 extracts the shared
composer submission from the screen router and reuses its busy-state expression.
Submission reads the current presenter snapshot and checks the original behavior
before adding the creation-kind prefix, preserving blank-intent rejection for
keyboard submission. No quality threshold or suppression was changed. Logs and
offscreen renders are in `.mini-orca/autopilot/ux/CREATE-01/`.

Correction-1 prescribed verification and all 373 desktop tests passed, but the
full command again exited 1 at Detekt: the screen complexity fell to 66, still
above 65. Correction 2 moves the cohesive creation-admission/draft-discard routing
into a small helper, leaving state updates with the screen. The quality gate is
unchanged. Commit review also identified the pre-existing Right tool-window
keyboard-handler relocation as a required dependency: it confines tab navigation
to the tabs so form arrows/Enter reach the fields. Include that narrow relocation
while retaining the unrelated Context tabs outside the commit. The creation test
now renders the real surrounding tool-window container and verifies field keys
do not select another tool.

Final prescribed verification exited 0: 21 tests, no failures/errors/skips.
`./desktop/gradlew -p desktop test detekt spotlessCheck` exited 0: all 373 working-tree
desktop tests passed, no Detekt findings, clean formatting. The exact isolated
commit candidate also passed `test detekt spotlessCheck`, with 371 tests and no
failures/errors/skips or quality findings. The two additional working-tree tests
belong to the preserved Context tabs. Commands used the verified Java 21 launcher
and Java 25 toolchain runtime override. The specified test selections were retained.

Reviewed production offscreen renders at 360dp/150% text and 800dp: visible New
function action, open-file identity, focused name then Behavior, explicit Generate
function/type and readable unsupported-file guidance. Keyboard tests verify the
editor action and creation fields do not activate surrounding tabs. Existing
source/review behavior, provider consent, constraints, and draft-discard routing
remain intact; opening creation does not submit a request. Package-only Go files
are eligible without a selected symbol. All entry points use the same guarded
creation flow and the existing chat/preview pipeline.

Diff review and `git diff --check` passed. Eight unrelated baseline files are
byte-for-byte unchanged; only the required creation additions were made over the
Context and visual-test baselines. The narrow pre-existing tool-window keyboard
fix is included as a coupled dependency; the remaining Context tabs, layout/theme
changes and unrelated tests remain uncommitted. No daemon/API, dependency,
configuration or migration change. Go tests, live providers and native OS/accessibility
acceptance were not run; this card uses offscreen component evidence. CREATE-02
remains queued for deeper Go identifier and generation/Apply lifecycle coverage.

## Task CREATE-02 — Close creation validation and lifecycle gaps

- [x] CREATE-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/FileChatState.kt` — correct Go identifier/keyword eligibility and clear creation failures.
- `desktop/src/test/kotlin/io/miniorca/desktop/FileChatStateTest.kt` — valid/invalid names and target switching.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — create session, generate, cancel and late-response behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DraftReviewWorkflowTest.kt` — creation through validation/checks/review.
- `internal/app/chat_session_test.go` — daemon creation acceptance/rejection and package-only file fixtures.
- `internal/project/go_declaration_edit_test.go` — append/import preservation and no-write preview regressions.
- `internal/app/draft_lifecycle_test.go` — creation Apply/Undo and stale-file rejection.

**Inputs / dependencies**
- CREATE-01.

**Implementation rules**
- Cover blank names, Go keywords, duplicates, supported Unicode identifiers, malformed source, unsupported files and project/file changes mid-request.
- Retain daemon name validation and exact single-declaration/import composition as the source of truth. Do not invent a second generation endpoint.
- Verify no source write before Apply, unchanged unrelated declarations/imports, and safe rejection if the file changes after generation.

**Verification command**
```sh
./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.FileChatStateTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest' --tests 'io.miniorca.desktop.DraftReviewWorkflowTest'
go test ./internal/project ./internal/app -run 'Test(ComposeGoDeclaration|ValidateGoDeclaration|ChatSession|Draft)' -count=1
```

**Execution record**
Completed 2026-09-11 by the authorized scheduler. Required checks and diff review
passed with no failed attempts or corrections. The local CREATE-02 commit is
recorded in Git history and the ignored execution receipt. Verified the CREATE-01 commit/receipt and both Java
toolchains. Captured the ten pre-existing desktop diffs and empty index under
`.mini-orca/autopilot/ux/CREATE-02/baseline/`; none overlaps the initial task targets.
Initial prescribed desktop verification passed: 66 tests, no failures/errors/skips.
Initial prescribed Go verification also exited 0 for both packages. Final review
made binary-file rejection explicit instead of presenting it as a language error;
the focused desktop checks passed again before full validation. No failed
verification or correction attempt has occurred.

Desktop preflight now accepts Go Unicode letters and decimal digits, rejects
keywords and malformed names, and identifies missing names and binary files.
The daemon remains authoritative for source syntax, declaration identity and
composition. Matched desktop/daemon examples cover Unicode, supplementary letters,
keywords, invalid punctuation, duplicates and unsupported files. Creation works
without a selected symbol in a package-only Go file.

Deterministic fake-transport tests cover explicit generation, rejected requests,
daemon error presentation, cancellation and late replies after file/project/revision
changes. Review coverage now exercises both replacement and creation, retaining
fresh validation/check requirements. Temporary-project tests exercise real daemon
generation with a local fake provider, malformed/retargeted/multiple declarations,
no source writes before Apply, explicit confirmation, preservation of existing
declarations/imports and unrelated files, exact Undo restoration and rejection
after an external source change. Existing daemon behavior passed these cases;
no daemon production code or API was changed.

Final prescribed desktop verification exited 0: 66 tests, no failures/errors/skips.
`./desktop/gradlew -p desktop test detekt spotlessCheck` exited 0: 378 desktop tests
passed with no failures/errors/skips, no Detekt findings and clean formatting.
The prescribed Go command exited 0 for both packages; `go test ./...` passed every
package (some unaffected packages reused cached results), and `make fmt-check vet`
exited 0. Java checks used the verified Java 21 launcher and Java 25 toolchain runtime
override. Logs and test counts are retained in `.mini-orca/autopilot/ux/CREATE-02/`.

Diff review and `git diff --check` passed. All ten pre-existing desktop files remain
byte-for-byte unchanged and outside this commit; this card's targets do not overlap
them. Review checked Unicode character categories, invalid-request short-circuiting,
retained consent/preview boundaries, deterministic failure cases and test cleanup.
No dependencies, configuration or migration steps. Full race/native/packaging
checks were not run for this validation/test card; no live model evaluation was
used. ANA-01 is next and remains unchecked.

## Task ANA-01 — Define categorized results and unified run contracts

- [x] ANA-01 completed with required checks and diff review.

**Target files**
- `internal/project/file_analysis.go` — explicit semantic risk category and cache compatibility.
- `internal/project/findings.go` — preserve classification through finding reconciliation without losing triage identity.
- `internal/project/file_analysis_test.go` — old/new report and category cases.
- `internal/project/findings_test.go` — category, provenance and triage preservation.
- `internal/app/analysis_run.go` — new run/queue/section types and request identities.
- `internal/app/analysis_run_test.go` — new contract/state cases.
- `docs/api-contract.md` — scope, result classification, run states and compatibility decisions.
- `docs/openapi.yaml` — documented unified preview/start/status/control/result shapes.

**Inputs / dependencies**
- D1 and D5; existing `AnalyzeAllJob`, `PerformanceJob`, `SecurityFileReport` and findings contracts.

**Implementation rules**
- Define project-wide run scope and explicit Bugs/Performance/Security category enum. File filters are read-only report queries, not execution scope. Keep source-specific detail types and severity/confidence distinct from category.
- Specify a source-free preview with immutable file identities, exclusions, provider requirements and bounded request/attempt expectations.
- Define per-section and overall states, including completed-empty, partial, failure, unavailable, stale, pause/cancel and interrupted recovery. A section with no successful analysis cannot claim zero findings.
- Old category-less risks remain accessible but unclassified until explicit refresh. Do not infer category from text or discard persisted dismissed/fixed states.

**Verification command**
`go test ./internal/project ./internal/app -run 'Test(FileAnalysis|Finding|AnalysisRun)' -count=1`

**Execution record**
Completed 2026-09-11 by the authorized scheduler after two focused corrections.
Required checks and diff review passed; the local commit is recorded in
`.mini-orca/autopilot/ux/ANA-01/receipt.json`. Verified CREATE-02 commit/receipt and the configured
Java toolchains. The ten pre-existing desktop files and empty index are preserved
under `.mini-orca/autopilot/ux/ANA-01/baseline/`. This card defines contracts;
execution, HTTP handlers and desktop adoption remain in their ordered successor cards.
The initial candidate defines additive category persistence, source-free planned
run contracts, complete request guards and truthful section coverage. Go formatting
is complete. Initial prescribed verification passed `internal/project` but failed
to compile `internal/app`: `AnalysisCoverage` already names the workspace cache
summary in `project_workspace.go` (fresh/stale/missing). Correction 1 renames the
new file-stage coverage contract to `AnalysisRunCoverage`, preserving the existing
workspace contract and changing only this card's new type, tests and schema.
Initial failure is retained in `.mini-orca/autopilot/ux/ANA-01/focused-initial.log`.
Correction 1 compiled, but the same prescribed command failed its OpenAPI test:
the schema rename also changed the existing workspace schema, leaving duplicate
`AnalysisRunCoverage` keys at lines 780/1672. Correction 2 restores the original
workspace `AnalysisCoverage` schema name/reference and keeps only the new run
schema named `AnalysisRunCoverage`. The first repair's failure is retained in
`focused-correction-1.log`; both available corrections are now used.

Final validation passed:
- `go test ./internal/project ./internal/app -run 'Test(FileAnalysis|Finding|AnalysisRun)' -count=1`
  passed both packages after correction 2 (`focused-correction-2.log`).
- `go test ./...` passed all Go packages (`go-full.log`); unaffected packages used
  valid cached results. This includes the live route/documentation parity tests.
- `make test-race` passed all Go packages (`go-race.log`), with cached results for
  unaffected packages.
- `make fmt-check vet` passed (`go-quality.log`); `git diff --check` passed.

Review confirmed the category is additive, unknown writes cannot replace stored
evidence, old reports remain readable, and category assignment preserves finding
IDs, provenance and dismissed/fixed triage. Contract tests cover whole-project
inventory exceeding a dispatch window, all required identity guards, transient
resume intent and truthful empty/partial/failure coverage. Planned OpenAPI refs
resolve without changing live route registration or the existing workspace
coverage schema. The run metadata contains no source or persisted confirmations.
Stage execution, durable coordination, report filtering and HTTP admission remain
successor-card work; this card does not claim those runtime behaviors are active.

All ten pre-existing desktop files were compared byte-for-byte with the baseline;
the initially empty index and predecessor HEAD were verified before staging.
Only this card's eight target files plus PLAN.md and this checklist enter the
commit. Desktop/native checks were not run because this card changes no desktop
code or native behavior. No configuration/data migration or live provider calls.
ANA-02 remains unchecked and is next.

## Task ANA-02 — Produce and validate explicit semantic categories

- [x] ANA-02 completed with required checks and diff review.

**Target files**
- `internal/app/file_analysis.go` — strict category schema/parser/prompt update and new prompt identity.
- `internal/app/file_analysis_test.go` — invalid/missing/new category and category-specific grounding cases.
- `internal/app/file_analysis_evaluation.go` — keep evaluation assessment aligned with the production contract.
- `internal/app/engineering_insight_test.go` — schema identity and optional-insight preservation.
- `internal/insighteval/evaluation_test.go` — offline contract fixtures.
- `internal/insighteval/runner_test.go` — offline runner/schema parity.
- `internal/app/service_test.go` — existing AnalyzeFile-to-draft integration reply must include the now-required category.
- `internal/api/handlers/project_handler_test.go` — existing finding/action HTTP integration reply must include the now-required category.

**Inputs / dependencies**
- ANA-01.

**Implementation rules**
- Newly generated risks must name their category; only actual correctness, performance or security findings enter the respective section.
- Keep existing source grounding, strict unknown-field rejection, response bounds and optional-field degradation semantics.
- Do not fabricate categories for historical reports. Bump the relevant prompt/cache identity and require explicit refresh to obtain the new classification.
- Update synthetic test fixtures only; never inspect sealed evaluation material or consume provider grants for this task.

**Verification command**
`go test ./internal/app ./internal/insighteval -count=1`

**Execution record**
Completed 2026-09-11. All required checks and diff review passed on the initial
candidate; no failed verification or correction attempts. The local commit is
recorded in `.mini-orca/autopilot/ux/ANA-02/receipt.json`.
Verified the ANA-01 commit/receipt and both configured Java toolchains. Preserved
the ten unrelated desktop files and empty index in
`.mini-orca/autopilot/ux/ANA-02/baseline/`. This task updates production category
generation/validation and synthetic contract tests, without live evaluation.
Preparation found two directly coupled integration fixtures outside the original
list; they are now listed above before editing. Historical v13 campaign identities
remain fixed: synthetic tests assert their dispatch is rejected by the new
production prompt identity, while retaining direct reservation/accounting checks.
Initial candidate requires exact category enums in the schema and parser, adds
category-specific grounding guidance, and bumps the prompt to v14. Synthetic v14
prompt digests were regenerated with the production adapter, without provider
calls. Existing optional-degradation fixtures now carry explicit categories;
new cases cover rejection, source-target grounding, passive legacy reads and
explicit refresh.

Validation passed:

- `go test ./internal/app ./internal/insighteval -count=1`: both packages passed
  (`focused-initial.log`).
- `go test ./...`: all Go packages passed, including daemon and handler integration
  (`go-full.log`); unaffected packages used valid cached results.
- `make test-race`: all Go packages passed (`go-race.log`), with cached results for
  unaffected packages.
- `make fmt-check vet` and `git diff --check`: passed (`go-quality.log`).

Review confirmed exact enum enforcement without prose inference, unchanged
severity and optional-field behavior, preserved target/path validation and response
bounds, and shared production/evaluation rejection semantics. The v14 prompt
separates correctness, resource-cost hypotheses and visible trust-boundary concerns;
general advice stays in suggestions. Synthetic tests verify all three categories,
invalid/missing enums, retained optional diagnostics and no model calls during
historical cache reads. Refresh stores a categorized report under v14.

Historical campaign constants, grants, receipts and corpus files are unchanged.
The retired v13 dispatch tests now assert rejection without consumption; direct
tests retain twelve-request limits, non-replay of charged reservations, manifest
protocol checks, locking, source/schedule identity and predecessor/head boundaries.
Current-prompt fake-provider integration continues to cover execution. No live
qualification was run and no model accuracy claim is made from these contract tests.

All ten pre-existing desktop files and the initially empty index were verified
against the baseline before staging. This commit contains the eight listed target
files plus PLAN.md and this checklist. Desktop/native checks were not run because
no desktop/native code changed. No configuration or data migration is required;
older semantic caches remain readable as stale and need explicit analysis to gain
categories. ANA-03 is next and remains unchecked.

## Task ANA-03 — Compose the per-file analysis stages

- [x] ANA-03 completed with required checks and diff review.

**Target files**
- `internal/app/analysis_file.go` — new typed stage execution over the existing analyzers.
- `internal/app/analysis_file_test.go` — new partial, cache, cancellation and freshness tests.
- `internal/app/file_analysis.go` — reuse existing semantic execution/publication at the narrow boundary needed by the run.
- `internal/app/performance_review.go` — reuse the existing source review and authorized publication callback.
- `internal/app/security_review.go` — bind advisory execution/publication to admitted run identity and fresh intent.
- `internal/app/security_rules.go` — include passive rules for eligible Go files.
- `internal/app/source_file_snapshot.go` — share only truly identical identity checks.
- `internal/app/service.go` — narrow retry-loop entry point for an explicit before-attempt guard; existing callers keep the same retry behavior.
- `internal/app/security_review_test.go` — update the existing direct execution call for the explicit optional dispatch argument.

**Inputs / dependencies**
- ANA-01, ANA-02.

**Implementation rules**
- Execute bounded stages using captured file/revision/hash identities; reuse current parsers and report stores, never call HTTP handlers internally.
- Preserve successful sections when another stage fails. Distinguish reused matching cache evidence from newly requested results; explicit refresh can bypass valid model caches.
- Security keeps deterministic rule matches and AI suggestions separately labeled. Unsupported rules do not suppress an otherwise eligible AI review.
- Check cancellation, source policy, provider consent and run publication authority before dispatch/publication. No tests/vet/benchmarks run here.
- Use stable producer IDs to avoid displaying the same stored finding twice. Do not merge unrelated findings merely because they share a line or title.

**Verification command**
`go test -race ./internal/app -run 'Test(AnalysisFile|AnalyzeFile|ReviewPerformanceFile|ReviewSecurityFile|ScanSecurityFile)' -count=1`

**Execution record**
Completed 2026-09-11. Required validation and diff review passed with one focused
correction. The authorized local commit is identified by ANA-03 in Git history
and `.mini-orca/autopilot/ux/ANA-03/receipt.json`.
Verified ANA-02 commit/receipt and both configured Java toolchains. The ten
unrelated desktop files and empty index are preserved under
`.mini-orca/autopilot/ux/ANA-03/baseline/`. Preparation added the directly coupled
retry entry point in service.go so every attempt, including retries, can validate
ownership/source/consent and reserve its budget before dispatch. The per-stage
executor is private; admission and scheduling remain ANA-04/05 work.
The existing direct Security execution test is also a required signature-dependent
target; it keeps its runtime-transition assertion with nil standalone dispatch.
Initial implementation is ready: a private stage executor reuses producer-owned
parsers/stores, validates captured snapshots and explicit publication authority,
and reserves bounded model attempts before every retry. Tests cover partial and
failed stages, cache/refresh, producer identity, consent, cancellation, stale work
and reservation/publication failures. The initial candidate had no repairs.
The initial prescribed race-enabled command passed (`focused-initial.log`).
Review then found a compatibility regression: extracted semantic preparation
errors could be stored as model failures, and Security snapshot read failures
could be reported as ordinary model failures. Correction 1 distinguishes typed
model request/response errors from preparation/identity/storage failures and adds
a standalone oversized-input regression test. One correction is used.

Correction-1 prescribed verification exited 0:
`go test -race ./internal/app -run 'Test(AnalysisFile|AnalyzeFile|ReviewPerformanceFile|ReviewSecurityFile|ScanSecurityFile)' -count=1`
passed in 3.528 seconds. `go test ./...`, `make test-race` and
`make fmt-check vet` each exited 0; unchanged packages may use valid cached results.
Logs are retained as `focused-initial.log`, `focused-correction-1.log`,
`go-tests.log`, `race-tests.log` and `format-vet.log` in the ignored ANA-03 evidence
directory. No verification command failed; the correction addressed a concrete
review finding. No additional correction or passing-suite rerun was needed.

Final diff review checked the synchronous publication boundary for lock reentry,
durable reservation failures, bounded transport retries without runtime mutation,
cache/provider identity, cancellation during active requests and cache reads,
source/policy/generation changes, partial evidence and source-free failure text.
Existing standalone behavior tests remain intact, with the one required optional
dispatch argument added to the direct Security test call. Existing snapshot
validation was reused without editing source_file_snapshot.go. The new executor
is private and does not yet admit or schedule whole-project runs. It executes no
project tests, vet, benchmarks or terminal commands.

`git diff --check` passed. The eight implementation/test files were frozen after
validation; the commit includes those files and the two related plan/checklist
updates. All ten unrelated desktop files were byte-compared with the saved
baseline and excluded from staging. Desktop/native validation and live provider
evaluation were not run for this daemon-only card. No configuration or data
migration is needed. ANA-04 remains unchecked for the next scheduled wake.

## Task ANA-04 — Implement one durable, bounded run lifecycle

- [x] ANA-04 completed with required checks and diff review.

**Target files**
- `internal/app/analysis_run.go` — preview/admission, sequential dispatch, pause/resume/cancel and recovery.
- `internal/app/analysis_run_store.go` — new source-free durable run metadata using shared atomic storage.
- `internal/app/analysis_run_preview.go` — isolate inventory, cache estimates and admission fingerprints from the existing run contracts and controller.
- `internal/app/analysis_run_progress.go` — isolate source-free coverage and stage-result accounting used by execution and restore validation.
- `internal/app/analysis_run_test.go` — lifecycle, budget, concurrency and persistence-failure tests.
- `internal/app/analysis_run_store_test.go` — new corruption/interruption/recovery cases.
- `internal/app/service.go` — one coordinator owner and lifecycle wiring.
- `internal/app/project_workspace.go` — restore progress without restarting requests.
- `internal/app/source_file_snapshot.go` — captured-run publication guard where required.
- `internal/app/analyze_all.go` — only the existing Reindex/ActivateProject lifecycle entry points must invalidate the new run before changing the project; legacy scheduler migration remains ANA-05.
- `internal/app/analysis_file.go` — share the existing cache-freshness predicates with preflight so its request estimates match stage execution.

**Inputs / dependencies**
- ANA-03.

**Implementation rules**
- Capture the whole project's deterministic eligible queue with existing context exclusions and per-analyzer source-size limits. File selection must not alter admission.
- Keep explicit batch/time/attempt bounds; reuse existing 100/500 file limits for bounded dispatch batches rather than truncating total project coverage. Calculate expected stage work and retry bounds in preflight rather than multiplying hidden retries. Budget exhaustion retains pending work for explicit continuation and cannot report full completion.
- Save admission before requests and progress before advancing work. Persistence failure stops dispatch and shows recoverable failure without overwriting completed reports or resetting attempts.
- Cancellation stops active requests and further dispatch; pause waits for a defined stage boundary and cannot consume more queued work. Project/source changes mark the run stale.
- Test concurrent start/control, replacement generations, restart, failed writes, provider changes and late completion with deterministic fakes.
- A restored run never restarts itself or carries reusable remote consent. Source-changing Apply invalidates the captured run under existing revision rules.

**Verification command**
`go test -race ./internal/app -run 'Test(AnalysisRun|AnalysisRunStore|ProjectOverview)' -count=1`

**Execution record**
Completed 2026-09-11 after explicit user authorization for one additional
correction, validation, commit and scheduler resumption. The two earlier
corrections and blocked regression remain recorded below; one further correction
was applied under that authorization. Local commit: ANA-04 in Git history and
`.mini-orca/autopilot/ux/ANA-04/receipt.json`.
Verified ANA-03's commit and receipt, the empty index and both Java toolchains.
Saved the ten unrelated desktop files under the ignored ANA-04 baseline directory.
Preparation adds only lifecycle invalidation call sites in analyze_all.go and
shared freshness predicates in analysis_file.go; these are direct dependencies
of source-changing Apply/Reindex isolation and accurate admission estimates.
The new owner is not registered with HTTP routes until ANA-05 replaces the old
schedulers, so this card does not activate a third public scheduler.
Initial prescribed verification exited 1: the 501-source-file inventory fixture
reported 502 eligible files and only one exclusion. The index already omits
policy-excluded files, while .mini-orcaignore itself remains eligible text under
the existing policy. Correction 1 adds the existing policy-aware project walk to
preflight exclusion accounting and source-inventory freshness checks, and corrects
the fixture's expected two model stages for that text file. The source inventory
still exceeds 500, all exclusions are asserted, and no provider calls are permitted
during preflight. All other initial lifecycle/store tests passed. One correction
is used; the original output is retained in `ANA-04/focused-initial.log`.
Correction-1 verification passed in 5.082 seconds. Full Go tests, race tests and
`make fmt-check vet` also passed. Final review found two related lifecycle gaps:
a write fault followed by Reindex made Stale progress reject cancel recovery,
and Overview read the run before reacquiring project facts, allowing a concurrent
project replacement between those reads. Correction 2 allows explicit cancel to
persist stale fault recovery without reviving work, and keeps Overview's complete
read under the existing project lifecycle lock. Added stale-fault recovery and
categorized partial-evidence regression coverage. Two corrections are used.
Correction-2 prescribed verification passed in 5.433 seconds; full Go tests,
`make test-race` and `make fmt-check vet` each exited 0. Those logs are retained as
`focused-correction-2.log`, `go-tests-correction-2.log`,
`race-tests-correction-2.log` and `format-vet-correction-2.log`.

Final review added `TestAnalysisRunStoreResumeReusesPublishedEvidenceWithNoAttemptsRemaining`
to reproduce process loss after successful report publication but before its
progress update. The restored stage has one charged attempt and a matching fresh
cache; preflight correctly estimates zero further requests for that stage.
The worker's early exhausted-attempt branch instead marks it Failed without
consulting the cache. The report is retained, but run coverage becomes Partial.
The prescribed command now exits 1 (`review-regression.log`, 4.494 seconds):
`run=partial stage=failed cached=false attempts=1 calls=3; want completed_empty,
cached, one retained attempt and three total calls`.

Required next correction: allow matching cached evidence through stage execution
even when transport attempts are exhausted, keeping the existing no-request
allowance and stale/missing-cache checks. A concrete proposed patch is retained at
`.mini-orca/autopilot/ux/ANA-04/proposed-cache-recovery.patch`; it is not applied or
validated. The two-correction allowance is exhausted, so no further production
repair was made. This card remains unchecked, no commit was created and no later
card started. The automation was paused through the app tool and its saved status
was checked. The final candidate, diagnostics and baseline remain under ANA-04;
all ten unrelated desktop files are unchanged. No desktop/native tests, live
provider evaluation, configuration migration, push or release was performed.

The user approved continuation with the prepared cache-recovery fix. The preserved
candidate hashes, baseline HEAD, empty index, ten unrelated desktop files and
both Java toolchains were verified before applying it. This authorized correction
allows already-cached evidence past the exhausted transport-attempt branch;
the stage still receives zero remaining attempts and independently checks cache
freshness before reuse. Existing regression and request-budget assertions remain.

Authorized-recovery validation: the exact prescribed command exited 0 in 5.571
seconds, including the previously failing restart/cache test. It now records
Completed-empty cached evidence, one retained attempt and three total fixture
requests. `go test ./...`, `make test-race` and `make fmt-check vet` each exited 0;
unchanged packages may use valid cached results. Logs are retained as
`focused-authorized-recovery.log`, `go-tests-authorized-recovery.log`,
`race-tests-authorized-recovery.log` and `format-vet-authorized-recovery.log`.

Final review confirmed the production change from the preserved blocked candidate
is exactly the approved conditional; other code and all regression assertions
are unchanged. Cache reuse still passes captured identity, policy, provider and
source checks, and receives zero transport allowance when exhausted. The complete
card preserves independent evidence, bounded attempts, durable reservations,
serialized publication, failure recovery, source-change invalidation and explicit
restart consent. `git diff --check` passed and validated code hashes were frozen
for staging. The authorized local commit includes the ten implementation/test
files and the three related plan/checklist/error records. All ten unrelated
desktop files were byte-compared with the original baseline and excluded.

The scheduler resumes after the verified commit, preserving its Astra Extra High
task settings, 20-minute cadence and one-card-per-wake procedure. ANA-05 remains
unchecked. No configuration or existing-data migration is required; the new run
metadata is created on explicit admission. Desktop/native checks and live provider
evaluation were not run for this daemon-only card. No push or release was made.

## Task ANA-05 — Migrate existing jobs and expose the unified API

- [x] ANA-05 completed with required checks and diff review.

**Target files**
- `internal/app/analyze_all.go` — remove its independent controller/worker; retain only required public-contract adapters.
- `internal/app/performance_job.go` — migrate scheduling ownership and retain required report/queue compatibility.
- `internal/app/service.go` — remove superseded controller fields and wire compatibility adapters to the unified owner.
- `internal/app/file_analysis_test.go` — migrate Analyze-all lifecycle regressions to the shared owner.
- `internal/app/performance_job_test.go` — retain queue/budget/persistence regression coverage.
- `internal/app/cleanup_contract_test.go` — update ownership assertions while preserving cleanup guarantees.
- `internal/api/handlers/analysis_handler.go` — new strict unified analysis handler.
- `internal/api/handlers/analysis_handler_test.go` — new request/identity/consent/error contracts.
- `internal/api/handlers/project_handler.go` — route legacy entry points through the shared lifecycle.
- `internal/api/handlers/project_handler_test.go` — legacy compatibility cases.
- `cmd/daemon/main.go` — register unified routes through existing loopback policy.
- `cmd/daemon/main_test.go` — route and documentation contract coverage.
- `docs/api-contract.md` — final API/migration semantics.
- `docs/openapi.yaml` — match implemented routes and schemas.

**Coupled target correction (before implementation)**
- `internal/app/analysis_run.go`, `analysis_run_preview.go`, `analysis_run_store.go` — the single owner must retain a captured semantic-only or performance-only compatibility scope, exact legacy total-time budget, and reusable admission/control boundaries. Public unified starts remain whole-project/all-stage.
- `internal/app/analysis_compatibility.go` — narrow shared-owner admission, control and projection helpers for the two legacy contracts.
- `internal/app/analysis_run_results.go` — new identity-guarded, read-only section aggregation using existing producer stores and triage.
- `internal/app/analysis_run_test.go`, `analysis_run_store_test.go` — register the formerly planned route contracts and exercise compatibility scope/recovery through the shared owner.
These are direct dependencies of removing both legacy schedulers and serving the
specified result endpoint; they do not expand the product scope.

**Inputs / dependencies**
- ANA-04.

**Implementation rules**
- Expose preview, start, current run, report and explicit controls. Strictly validate project/revision/file/queue/provider identity at the boundary.
- Existing Analyze-all and Performance starts become single-purpose adapters to the same owner, preserving their documented scope and limits. Remove obsolete locks/workers and migrated duplicate tests, retaining behavioral regressions.
- Read old persisted jobs without dispatch; preserve attempts and progress when presenting interrupted recovery. Never overwrite historical reports to manufacture combined coverage.
- Keep the project Go scan and benchmark execution APIs separate because they execute code. Preserve loopback/origin rules and sanitized errors.
- If a legacy response cannot faithfully project the new lifecycle, record the exact incompatibility and migration before removing it; do not silently change public behavior.

**Verification command**
`go test -race ./internal/app ./internal/api/handlers ./cmd/daemon -count=1`

**Execution record**
Started 2026-09-11. Verified ANA-04 commit/receipt and the empty index;
captured all ten unrelated desktop files under the ignored ANA-05 baseline.
Initial implementation prepared; required race verification next. Zero correction
attempts used. Legacy scheduler internals and their direct-lock tests are removed;
retained public behavior tests plus `TestAnalysisCompatibility*` exercise scope,
exclusive admission, pause/cancel, generation/revision guards, cache reuse, durable
recovery, actual attempts, old metadata preservation and total Performance time.
Shared `TestAnalysisRun*` regressions retain admission/result/completion save faults,
source/policy/provider changes, concurrent controls and interrupted recovery.
The detached-root fault regression additionally preserves the earlier cleanup
guarantee. Old root corruption now fails closed without renaming historical files;
its behavior and non-resumable pre-migration counters are explicitly documented.
Removed test-name inventory is retained in ignored ANA-05 execution evidence.

Initial prescribed verification exited 1: application tests did not compile
because migration removed the still-used `storeCachedPerformanceReport` fixture
and the new detached-fault test omitted its `project` import. HTTP handler and
daemon packages passed. Correction 1 restores that fixture/import. The concurrent
control review also found a legacy Performance ID check outside the owner; pause
and cancel now pass the expected ID into the shared identity check, and active
starts reject mismatched queue guards. Lifecycle conflicts consistently return
409. Result reads reject missing committed evidence and newly excluded files,
rather than exposing a successful count with no readable producer report.

Correction-1 verification exited 1. The shared control helper treated an absent
internal revision guard as an explicit mismatch; two retained Performance control
tests failed and one blocked in provider cleanup. SIGQUIT captured the owned test
process stack and ended that blocked run (no unrelated process was interrupted).
The detached-failure test also used the temporary path spelling rather than the
manager's canonical root, so its injected failure never fired. Correction 2 keeps
omitted internal guards optional while enforcing supplied HTTP revisions/IDs,
binds fault injection to the actual canonical root, and makes provider fixture
cleanup unconditional. The route-origin regression uses a valid loopback Host so
it tests origin policy specifically. No application test assertion was relaxed.

Correction-2 prescribed verification exited 1 in 22.456s for `internal/app`.
Exactly one test failed: `TestStartAnalyzeAllReturnsActiveJobWithoutStartingAnotherWorker`.
Its assertions completed, then temporary-project cleanup failed with
`TempDir RemoveAll cleanup: unlinkat ...: directory not empty`. The test waits for
the provider handler response after Cancel, but does not join the shared worker's
final durable cancellation save. HTTP handler race tests passed in 2.809s and
daemon race tests passed in 1.740s. The earlier guard and canonical-root regressions
passed; there were no other reported test failures or race-detector warnings.

The two-correction limit is exhausted. ANA-05 stays unchecked and uncommitted.
Scheduler `mini-orca-ux-implementation` was paused through the app tool and verified
Paused with its original prompt, cadence and task target retained. No test process
remains active. Candidate hashes and precise failure logs are retained in ignored
ANA-05 evidence. The empty index, unchanged ANA-04 HEAD and all ten unrelated
desktop file bytes were verified. `git diff --check` passed. Broader Go/race/vet
checks and final acceptance review await a passing prescribed command; desktop
and native checks were not run for this daemon-only card.

Concrete proposed additional correction, awaiting user authorization: append
`waitAnalysisWindow(t, service)` immediately after the active Analyze-all response
wait in the failing test. This joins the canceled worker before fixture cleanup,
without changing production behavior or removing an assertion. Then rerun the
prescribed command, complete required checks/review, and commit only if accepted.
Do not advance to ANA-06 or reset the recorded correction count.

User-authorized continuation — 2026-09-11: the user explicitly requested
“yes, repair ANA-05” after reviewing the concrete worker-wait fix. Verified every
preserved candidate hash, the unchanged ANA-04 HEAD, the empty index and all ten
unrelated desktop files. Applied the proposed `waitAnalysisWindow(t, service)`
after the provider-response wait. This is correction 3 in total, one additional
user-authorized repair; prior failures and accounting remain unchanged. Required
verification and final review resume from this preserved candidate. Scheduler
remains Paused until acceptance and the authorized local task commit are verified.

The authorized worker-wait repair passed the exact race command: application
23.464s, HTTP handlers 2.695s, daemon 1.664s. Final review found a further ANA-05
compatibility defect: restored active Performance progress retained attempts but
could regain unobserved active time, unlike the old conservative request-start
ledger. The user's request to repair ANA-05 covers completing this same card;
correction 4 (review repair) now conservatively charges the interval since the last
saved active update, capped at the captured total budget. Saved paused progress
spends no idle time. An integration regression checks running/pausing/canceling and
paused restores, durable budget accounting, preserved attempts and no dispatch.
This remains ANA-05 work; the earlier correction count is not reset or hidden.

Final acceptance — 2026-09-11: prescribed race verification passed after the
review repair (`internal/app` 23.032s, handlers 2.748s, daemon 2.106s).
`go test ./...`, `make test-race`, `make fmt-check vet` and `git diff --check`
all passed. Full-suite evidence is retained in the ignored ANA-05 directory;
unchanged packages legitimately reused Go's test cache. There are no active
validators or unresolved reported failures. The final documentation removes stale
future-tense classification claims and describes the implemented migration.

Review verified the single scheduling owner, scope-limited compatibility admission,
actual attempt reservations, total Performance budget and crash recovery, serialized
controls/report publication, provider consent, category/triage preservation, strict
HTTP identity/query guards, and loopback/origin policy. Obsolete legacy workers,
locks and their direct-internal tests are removed; required public regressions and
shared-owner persistence/lifecycle tests remain. Saved legacy progress and reports
are not rewritten to fabricate unified coverage. The new routes expose source-based
analysis only; verified Go scans, benchmark execution, Review/Apply and source files
remain under their existing explicit workflows.

Migration: clients may use the new unified routes; a pre-migration interrupted job
must be reviewed and explicitly restarted because it lacks the new immutable
provider/file identity and transport-attempt ledger. Its original metadata/reports
are retained. No configuration change is required. Desktop/native tests, live
provider evaluation and the combined `make check` desktop/dispatcher steps were
not run for this daemon-only card. The exact implementation plus related plan/API
records form one authorized local commit; its hash and staged/committed file hashes
are recorded in the local receipt after verification. Unrelated desktop edits are
excluded. Resume the existing Astra Extra High scheduler after the verified commit;
end this wake without starting ANA-06.

## Task ANA-06 — Give the desktop one analysis owner

- [x] ANA-06 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/Models.kt` — unified wire types.
- `desktop/src/main/kotlin/io/miniorca/desktop/ApiClient.kt` — unified API methods.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — one analysis run and typed section evidence.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflow.kt` — new cohesive request/control owner.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt` — delegate analysis commands and remove superseded request logic.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopJobCoordinator.kt` — one analysis poller, retaining the independent verified-scan poller.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopSecurityWorkflow.kt` — remove duplicate run ownership; retain only independently required security operations.
- `desktop/src/test/kotlin/io/miniorca/desktop/ApiClientContractTest.kt` — unified and compatibility payloads.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflowTest.kt` — new run ownership/consent/late-result tests.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopAnalysisAdmission.kt` — new shared preview/consent dialog required to admit the unified request.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — mount that dialog and wire its explicit actions.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAnalysisAdmissionTest.kt` — production admission content, consent controls and layout coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — migrated analysis lifecycle cases.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopSecurityWorkflowTest.kt` — preserve fresh intent and retained-evidence assertions.

**Inputs / dependencies**
- ANA-05.

**Implementation rules**
- The daemon owns job scheduling; desktop only admits actions, polls and renders authoritative state. Switching result sections must not trigger work.
- Preserve typed Security evidence and benchmark ownership, file/project identity guards and cancellation cleanup.
- One admission UI names all affected providers/scopes and Security intent. Consume/reset consent at the defined attempt boundary; no silent reuse after restart or changed scope.
- Refresh and reconnect read reports; partial errors remain associated with the correct section/file and do not replace unrelated valid results.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ApiClientContractTest' --tests 'io.miniorca.desktop.DesktopAnalysisWorkflowTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest' --tests 'io.miniorca.desktop.DesktopSecurityWorkflowTest'`

**Execution record**
Started 2026-09-11. Verified the ANA-05 commit/receipt, empty index,
and all ten unrelated desktop files; captured their exact bytes in the ignored
ANA-06 baseline. Read the current UI guidelines. Java 21 and JBR 25 binaries
passed their version checks. Resumed preparation on the next wake with no active
writer or validator; no implementation was repeated. The admission dialog, app
mount and its tests are narrowly added because the owner cannot satisfy fresh
provider/Security consent without an actual user action surface. Existing result
page restructuring remains ANA-07. Initial implementation; zero corrections.

Initial prescribed verification passed. The full desktop suite passed all 375
tests, but the combined quality command failed at Detekt: DesktopState.reduce
measured 66/65 and MiniOrcaApp 67/65 cyclomatic complexity. Correction 1 extracts
the new state transition and admission visibility routing into cohesive helpers.
Review in this correction also preserves immediate triage in unified evidence,
rejects pre-triage pending reads, and restores polling after a failed reindex.
The admission content renders at narrow/large-text and wide/default-text sizes;
required consent controls pass keyboard and state assertions. No native popup or
OS accessibility claim is made by those component renders.

Correction-1 prescribed verification exited 1: 78 tests, one failure in
`finalProgressReloadsAReportReadThatWasStillInFlightAtCompletion`, expected
"completed" but got an empty string. The fixture had not populated its changing
message marker; the other 77 cases, including triage and failed-reindex recovery,
passed. Correction 2 populates that marker from the authoritative run status so
the assertion distinguishes the stale report from the final reread. The unchanged
production correction passes Detekt and Spotless. No earlier failure is discarded.

Final acceptance — 2026-09-11: correction-2 prescribed verification passed all
78 tests. Full desktop `test detekt spotlessCheck` passed all 378 tests with
zero failures/errors/skips, and both quality gates passed. The exact commit
candidate (HEAD plus this card's 14 Kotlin files, excluding the ten unrelated
UI edits) independently passed the same full checks: 376 tests, zero failures,
errors or skips. `git diff --check` passed. Java 21 launched the wrapper with
the documented JBR 25 override; no build configuration or quality gate changed.
Two correction attempts in total; their failures and diagnostics remain above.

Review confirmed one desktop analysis owner and poller, full project admission
with per-preview provider/Security consent, guarded start/resume/pause/cancel,
read-only reconnect/recovery and typed results indexed by category/file. Late
project/generation/report replies, retained partial failures, final-progress
rereads and immediate triage are covered. Closing/selecting a file does not
cancel a daemon-owned project run; explicit Cancel does. The independently
required deterministic file scan, verified Go scan, benchmark trust and draft
Review/Apply retain their own workflows. Legacy client wire APIs remain for
compatibility, while their duplicate presenter requests/polling and old security
AI owner are removed. Existing page intents delegate to the unified preview;
ANA-07 owns replacement of those page layouts and their legacy view bindings.

The shared admission content was rendered and inspected at 360dp/150% and
640dp/100%, including loading/error and separate keyboard consent controls.
These are production-component checks, not native popup placement, OS focus or
screen-reader certification. No live provider evaluation, Go tests or combined
`make check` was run for this desktop-only card. The client requires the unified
daemon routes from ANA-05; no configuration migration is needed.

All ten unrelated desktop files remain byte-identical to the captured baseline.
This card's implementation and checklist/status records form one authorized local
commit; its verified hash and file hashes are recorded in the ignored ANA-06
receipt. The existing Astra Extra High scheduler remains Active. End this wake;
ANA-07 is next.


## Task ANA-07 — Separate run progress from the three result pages

- [x] ANA-07 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — project coverage, per-analyzer progress and result-page links.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — Analysis start/progress surface and Bugs results page.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — readable result rows and visible severity/provenance.
- `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` — retain triage/filter presentation within Bugs section.
- `desktop/src/main/kotlin/io/miniorca/desktop/PerformanceWorkspace.kt` — section content, hypotheses and explicit measurement handoff.
- `desktop/src/main/kotlin/io/miniorca/desktop/SecurityWorkspace.kt` — section content retaining rule/AI distinctions.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — wire unified actions and exact finding-to-editor handoffs.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — replace the old workspace input/action bindings with the unified run and typed section snapshots; sidebar grouping stays NAV-01.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — hold local result-page path filters for the Context handoff.
- `desktop/src/main/kotlin/io/miniorca/desktop/BottomEvidenceToolWindows.kt` — remove its dependency on the replaced legacy Analysis presentation helper while retaining its existing historical output entries; bottom-tool removal remains BOTTOM-02.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt` — route that handoff without requests and guard result-to-editor preparation against current report identities.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — cover the local handoff and project-wide finding preparation.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopShellTest.kt` — migrate assertions for the changed workspace bindings.
- `desktop/src/test/kotlin/io/miniorca/desktop/ContextToolWindowTest.kt` — migrate Context action expectations without altering the pre-existing tab/style work.
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — Analyze project and local View this file's results actions.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — real counts/coverage/filter/state cases.
- `desktop/src/test/kotlin/io/miniorca/desktop/BugsWorkspaceStateTest.kt` — triage retained.
- `desktop/src/test/kotlin/io/miniorca/desktop/PerformanceWorkspaceTest.kt` — hypothesis/measurement distinction.
- `desktop/src/test/kotlin/io/miniorca/desktop/SecurityWorkspaceTest.kt` — partial/source-aware results.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — all three sections and their important states.

**Inputs / dependencies**
- UX-02, ANA-06.

**Implementation rules**
- Analysis shows Start, overall/per-analyzer progress, current file, coverage, pause/resume/cancel and operational failures only. Its Bugs/Performance/Security progress rows link to the corresponding result page; no findings list belongs on Analysis.
- Keep results in their existing distinct workspaces, supplied by the unified run. Their file filters never start another run. Prefer a list/detail layout that becomes a single-column drill-down on narrow windows.
- Each result shows title, explicit severity/state, concise content and source location before secondary evidence. Filters are local and never re-run analysis.
- Do not put general suggestions in Bugs; preserve them in the file explanation. Historical unclassified risks are readable under previous-analysis detail with no fresh count.
- Preserve existing triage, Prepare fix, exact-symbol eligibility and benchmark trust behavior. Opening a finding must not generate a draft.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.BugsWorkspaceStateTest' --tests 'io.miniorca.desktop.PerformanceWorkspaceTest' --tests 'io.miniorca.desktop.SecurityWorkspaceTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

**Execution record**
Started 2026-09-11. Verified the ANA-06 commit/receipt, empty index and
all ten unrelated desktop files. Captured both working and HEAD versions in
the ignored ANA-07 baseline for scoped changes to overlapping Context/visual
test files. No active writer or validator; both Java toolchains verified. Read
the current UI guidelines. Initial preparation; zero corrections. The shell,
local-filter state, presenter and directly affected tests are narrowly added
because they construct the listed page models and own Context/editor handoffs.
No sidebar regrouping, terminal work or bottom-pane removal is included.

Initial implementation replaced the legacy page controls with canonical run progress
and typed section list/detail views, local filters, and guarded editor handoffs.
Initial prescribed verification reached test compilation: two new fixtures used
flattened DesktopState fields instead of ProjectWorkspaceState. Initial Detekt also
caught the added reducer branch at 65/65. Correction 1 uses the real fixture owner
and the existing AnalysisRunUpdated event for local filter state, avoiding another
reducer branch. No verification bypass; the same prescribed command follows.

Correction 1 compiled and ran the prescribed suite: 55 tests, two visual assertions
failed. Full desktop validation identified the same two failures (370 total tests);
Detekt and Spotless passed. Correction 2 updates the error fixture to actually wrap
and migrates the affected Context assertion to Analyze project, whose preview owns
fresh consent. The correction is applied to both working and isolated candidates;
pre-existing Context/style changes remain separate. No further corrections are
available without explicit user direction if verification still fails.

**Blocked after correction 2 — 2026-09-11.** The prescribed 55-test suite and full
370-test working-tree suite now pass; full Detekt and Spotless also pass. Production
component renders cover 1440, 1000/999, 800×650, 1280×600 and 125/150% text, category
rows and detail drill-down, local path filtering, stale/empty/historical evidence
and long operational errors. No native OS focus/screen-reader or live-provider
verification was performed.

An isolated commit candidate built from accepted HEAD (excluding the ten unrelated
UI files) fails compileKotlin: Information is unresolved at FindingsPresentation.kt:204
and WorkspacePanes.kt:98,120. That color exists only in the user's uncommitted theme
work. Consequently ANA-07 is not accepted, staged or committed despite the passing
working-tree suite. Correction accounting remains 2/2; the scheduler is paused.

Proposed continuation: replace those three references with the existing SelectionText
role in the two already listed targets, then rerun the prescribed/full desktop and
isolated checks, review the scoped diff, and create the authorized ANA-07 commit.
The exact unapplied patch is retained at
`.mini-orca/autopilot/ux/ANA-07/proposed-repair.patch`. Do not start NAV-01 or reset
correction accounting. User direction is required to continue repairing this card.

Preservation audit: seven non-overlapping pre-existing files are byte-identical;
the Context and visual test deltas are unchanged. Context's prior tab/style rewrite
is retained; the same legacy analysis-action block was intentionally replaced in
both the working and isolated variants. The strict line-delta audit differed only
at that replaced block and its formatting. All task source files and both variants
are retained in the ignored checkpoint. HEAD remains ANA-06 cb88fce; index empty.

**Authorized repair continuation — 2026-09-11.** The user said “you can continue
with the repair and task.” Verified every saved blocked-checkpoint hash, accepted
HEAD, empty index, idle validators and both Java toolchains. Correction 3 applies
the preserved three-reference patch using SelectionText in FindingsPresentation.kt
and WorkspacePanes.kt, and synchronizes the isolated candidate. Prior failures and
correction accounting remain recorded. Complete this card's prescribed/full/isolated
validation and scoped local commit before resuming the scheduler; no later card
starts in this continuation.

**Accepted after correction 3 — 2026-09-11.** The exact prescribed suite passes
55 tests. Full working-tree `test detekt spotlessCheck` passes all 370 desktop
tests and both quality gates. The isolated accepted-HEAD candidate passes all 368
tests, Detekt and Spotless without any of the ten pre-existing desktop changes.
All three Gradle invocations used Java 21 JAVA_HOME and the explicit JBR25
installation path recorded above. The scoped diff and `git diff --check` pass.
Logs, counts, baseline variants, component renders and candidate hashes are
retained under `.mini-orca/autopilot/ux/ANA-07/`.

Analysis now owns the whole-project Start/progress/coverage/lifecycle surface;
Bugs, Performance and Security own their typed result lists and details. Local
filters and Context's View this file's results action never request analysis.
Severity/state, source locations and provenance precede expandable evidence;
stale and historical content stay distinct from current counts. Existing triage,
source navigation, exact Go preparation and explicit benchmark trust remain.
The repair uses a committed shared color role and adds no theme dependency.

Reviewed production component renders across the recorded viewport/text matrix,
including the repaired progress color and narrow result drill-down. No daemon
code changed, so Go/race/make check were not run. Native OS focus, screen-reader
and popup behavior, and live model providers were not exercised; offscreen
component checks are not claims of that coverage. No configuration or migration
steps are required; the unified daemon contract comes from accepted ANA-05.

Seven untouched pre-existing files remain byte-identical; both overlapping test
files retain exactly their unrelated deltas. Context retains the user's tab/style
rewrite while the same analysis-action replacement is staged from the isolated
variant. Stage only the 20 task Kotlin files plus this checklist, PLAN.md and
errors.log. Create one authorized local ANA-07 commit, verify its file hashes and
receipt, then resume the existing scheduler and end this continuation. NAV-01 is
next; no additional card is implemented here. Prior correction accounting is 3,
including this user-authorized continuation.

## Task NAV-01 — Distinguish run, results and editing in the sidebar

- [x] NAV-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — retain distinct workspaces and local per-page result filters.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt` — preserve saved navigation and pane sizes.
- `desktop/src/main/kotlin/io/miniorca/desktop/IdeShell.kt` — grouped Results destinations with labeled count/state and selected accents.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — connect progress-page links and result destinations.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopKeyboardNavigation.kt` — preserve existing workspace shortcuts and focus behavior.
- `desktop/src/main/kotlin/io/miniorca/desktop/CommandPalette.kt` — consistent Start analysis and result-page actions.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — project-wide Start routing and local result navigation.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopStatusBar.kt` — reflect actual run/provider context.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopLayoutStateTest.kt` — preference preservation without side effects.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopShellTest.kt` — new workspace ownership.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — navigation/focus coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopIntegrationCoverageTest.kt` — section-to-editor navigation.

- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAccessibilityTest.kt` — migrate the existing Bugs label assertion; keep selection/focus semantics and shortcut coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/CommandPaletteTest.kt` — replace obsolete file-analysis actions and assert the new project/navigation scopes.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopStatusBarTest.kt` — verify captured run/provider context and stale/foreign ownership.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — migrate affected palette/rail fixtures and render the grouped production navigation; preserve the user's pre-existing Context tests.

**Inputs / dependencies**
- ANA-07.

**Implementation rules**
- Preserve separate Analysis, Bugs, Performance and Security rail entries as requested. Make their purposes obvious: Analysis runs/tracks; the three Results destinations display findings.
- Group the result entries with restrained spacing/separators, clear labels and real count/state badges. Do not add a second navigation system or turn category tint into severity.
- Preserve Cmd/Ctrl+4 for Editor and existing Summary/Analysis/Bugs/Performance/Security navigation. Saved selections remain valid; keep pane widths and no-network-on-navigation behavior, with independent focus/selected accents.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopLayoutStateTest' --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopIntegrationCoverageTest'`

**Execution record**
Started 2026-09-11. Verified the ANA-07 commit and all receipt hashes, an empty
index, no active writer/validator, both Java toolchains and all ten unrelated
working files. Captured original HEAD and working variants in the ignored NAV-01
baseline. Read AGENTS.md, the run procedure and UI guidelines. Initial
implementation; zero corrections. Added the directly coupled palette/status
unit tests and existing visual fixture file to the target list before edits.
Saved workspace keys and keyboard shortcuts remain stable; no terminal or bottom
pane replacement is included in this card.

**Correction 1 — 2026-09-12.** Initial prescribed verification ran 50 tests;
two dock-width tests failed. The 128dp rail left only 395dp for source at 1000dp
because the existing inspector minimum is 280dp. Use a 120dp rail, preserving
all pane minima and stored preferences (rendered docks 180/283dp, source 400dp).
Large-text review also found a split Completed badge; show the real numeric
badge with a findings label and a separate wrapping state instead. Palette action
rows now separate the action from its scope so the longer project scope remains
readable. The initial log and renders remain under the ignored NAV-01 directory.
One correction is used; rerun the same prescribed verification.

**Correction 2 — 2026-09-12.** Correction-1 prescribed verification passed all
50 tests. The full suite ran 375 tests and failed only the legacy accessibility
expectation `Bugs & Problems tool window, not selected`; production now correctly
says `Bugs tool window, not selected`. Added that coupled assertion file to the
card before migrating it. Detekt and Spotless passed independently. Large-text
render review found Performance splitting at the final letter; reduce entry
horizontal padding from 8dp to 4dp within the same 120dp rail. Add action-palette
render/keyboard coverage for the new wrapping scope descriptions. Two corrections
are now used; failed logs remain `full-correction-1.log` and
`focused-initial.log`, with all intermediate visuals retained.

**Accepted — 2026-09-12.** The prescribed four-class command passes all 50 tests.
`./desktop/gradlew -p desktop test detekt spotlessCheck` passes all 376 tests and
both quality gates. The isolated candidate, built from accepted HEAD plus only
NAV-01 changes, passes all 374 desktop tests, Detekt and Spotless. All invocations
used the documented Java 21 launcher and explicit JBR25 path. Failed attempts,
counts and successful logs are retained in `.mini-orca/autopilot/ux/NAV-01/`.
Two focused corrections were used; their history above remains unchanged.

The rail groups Project, Results and Editing, retains every stored destination
and shortcut, and labels Analysis as Run & progress. Result counts come from the
owned whole-project run, independent of local filters; unknown/stale counts are
not fabricated as zero. Selection and keyboard focus remain separately visible,
with focused entries scrolled into view. The command palette offers whole-project
Start through existing preview/consent and local progress/results navigation;
Editor actions retain their existing preparation flow. Status shows the captured
run and all its provider scopes, without implying live provider connectivity.
The 120dp rail preserves the existing pane minima and restores saved widths.

Reviewed production rail renders at 1440×900, 1000×760, 999×760, 800×650, and
1280×600 at 125%/150% text, including keyboard reveal of Editor. Scope descriptions
and explicit palette keyboard activation pass component checks. Offscreen palette
images do not establish native popup geometry, including the additional ignored
full-size-host reproduction; native popup placement, OS focus and screen-reader
behavior remain unverified. No live provider, Go/race or `make check` execution
was needed for this desktop-only card. No configuration or migration is required.

Final review found no actionable source issue or removed required behavior.
Seven unrelated files are byte-identical, and all pre-existing changed lines in
the three overlapping files remain intact and excluded from the isolated commit.
Only the 15 listed Kotlin files plus PLAN.md and this checklist belong to the
local NAV-01 commit. Verify its exact hashes and empty index in the ignored
receipt, then end this card; the existing Astra Extra High scheduler remains
active for TERM-01 on the next wake.

## Task TERM-01 — Prove the terminal dependency and local-session boundary

- [x] TERM-01 completed with required checks and diff review.

**Target files**
- `desktop/build.gradle.kts` — pin verified terminal/PTY dependencies and required package modules.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTerminalSession.kt` — new local shell/PTY owner independent of Compose recomposition.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopTerminalSessionTest.kt` — new lifecycle tests with injected process boundary.
- `desktop/TERMINAL.md` — new short dependency, native packaging and supported-host record.

- `desktop/settings.gradle.kts` — add the official JediTerm repository restricted to its group; these artifacts are not on Maven Central.
- `desktop/src/main/resources/terminal-licenses/` — package the selected upstream licenses and notices for the newly bundled dependencies.

- `desktop/scripts/terminal-packaged-smoke.c` and `desktop/scripts/terminal-packaged-smoke.sh` — reproducible JNI probe using the bundled runtime, which has no java launcher; no product bootstrap or generated package files are modified.

**Inputs / dependencies**
- D2; existing JBR 25/macOS arm64 distribution constraints.

**Implementation rules**
- Evaluate JediTerm/Pty4J with the pinned desktop toolchain; choose exact available artifacts and record notices/native requirements before integrating the UI.
- Prove process startup, working directory, UTF-8, terminal resize, interrupt and bounded teardown on the supported host. Use an isolated temporary directory for smoke commands.
- Launch the user's supported shell as an executable plus arguments, never interpolate the project path into a shell command string. Set project cwd explicitly.
- Define Idle/Starting/Running/Exited/Failed/Closed state, a bounded scrollback policy and process cleanup. No automatic shell start during project restore.
- Missing shell/native library/unavailable local project path gives a visible error and retry action. Do not claim a full terminal from a fake-only test.

**Verification command**
```sh
./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopTerminalSessionTest'
./scripts/desktop-gradle.sh createDistributable
```
Also record a real packaged-host PTY smoke before TERM-02; dependency/native failure is an explicit implementation dependency, not permission to substitute an output box.

**Execution record**
Started 2026-09-12. Verified accepted NAV-01 hashes, empty index, no active validator, both toolchains and all ten unrelated UI files. Baseline saved in the ignored TERM-01 directory. Selected JediTerm 3.72 (the newest examined release using Kotlin 2.1.21, compatible with this app's Kotlin 2.3 toolchain) and Pty4J 0.13.12 for proof. Newer JediTerm 3.73–3.76 require Kotlin 2.4. Repository and packaged-notice targets added before edits as direct dependency-resolution/redistribution requirements. Initial preparation; zero corrections.

Initial prescribed tests and createDistributable both passed. The additional
real-PTY run reached cwd, UTF-8, TTY, resize and interrupt, then timed out waiting
for the background-child marker. The initial XML/log are retained. A diagnostic
rerun adds bounded synthetic-probe output on failure; no production correction
has been applied and correction accounting remains zero.

**Correction 1 — 2026-09-12.** Diagnostic output identified interactive Bash
history expansion of the probe's `$!` as the missing-marker cause; disable history
expansion only in the synthetic shell. The fixture helper now returns Unit so the
path-boundary test is discovered by JUnit (it previously returned the asserted
exception). Review also adds an actionable local-path error, cleanup if stream
attachment fails after process creation, and an explicit Unit return type to
remove the Kotlin compiler warning. A regression covers attachment cleanup.
One correction used; earlier logs/XML remain intact.

The first correction reached bounded close and exposed a real Pty4J boundary:
UnixPtyProcess implements pid() but rejects Process.toHandle(). The cleanup error
remained visible, so acceptance correctly failed. Resolve ProcessHandle from the
owned PID instead, then repeat native and packaged validation. One correction
has been used; the original failure is retained in focused-correction-1.log.

**Correction 2 — 2026-09-12.** Use ProcessHandle.of(the owned Pty4J PID) to
inspect descendants. The same failure reproduced under the app's bundled JVM,
confirming the required native/package boundary. Also keep any process that
survives bounded teardown owned and in cleanup-pending state so a retry cannot
start an overlapping shell. Full desktop tests and both quality gates had passed
before this correction; native completion remains the acceptance gate. Two
corrections are used; do not reset accounting on a later wake.

**Accepted — 2026-09-12.** The prescribed terminal class passes all 13 tests with
`-PterminalNativeSmoke=true`, including the real PTY probe. Full desktop
`test detekt spotlessCheck` with the same native flag passes 389 tests. The
isolated candidate passes 387 tests and both quality gates. The working and
isolated `createDistributable` commands pass, and both bundled-runtime JNI probes
pass cwd, real TTY descriptors, UTF-8, 121×42 resize, Ctrl+C, background-child
cleanup and a close deadline under 3 seconds. The launchers and JBR25 path are the
verified versions documented for this queue; no toolchain substitution occurred.

The pinned JediTerm 3.72/Pty4J 0.13.12 dependencies and scoped repository resolve
without upgrading Kotlin. A stable local-only owner supplies explicit lifecycle,
retryable launch/path/native errors, UTF-8 connector, 5,000-line scrollback policy,
late-start disposal and bounded cleanup. A surviving shell remains owned and
blocks replacement. No shell starts during construction or project restore, and
no daemon command endpoint, model-to-stdin route or transcript persistence exists.
The terminal pane and app integration remain TERM-02.

The unmodified dependency jars include the macOS universal native library/helper;
all nine notice resources are verified in the app jar. Pre-stage notice hygiene
normalizes line endings/trailing whitespace without changing license wording.
Packages and their native probes were repeated after that resource-only change;
source/build/test hashes remain those of the passing full suites. The reproducible
JNI probe uses the actual packaged JVM and jars, never a substituted development
runtime, and leaves generated package files untouched. Evidence, hashes, initial
failures and final logs are retained under `.mini-orca/autopilot/ux/TERM-01/`.

Final diff review passes. All ten unrelated files remain byte-identical; no overlap
requires partial staging on this card. Stage only the 16 task files plus PLAN.md
and this checklist, create one authorized local TERM-01 commit, and verify its
exact hashes and empty index in the ignored receipt. Two corrections are recorded
above. No Go/race/full make check or live provider execution was needed; no daemon
code changed. Native UI focus, history/copy-paste/full-screen redraw, other hosts,
and signing/notarization are not claimed. No user configuration or data migration
is needed. Keep the Astra Extra High scheduler active for TERM-02 and end this card.

## Task TERM-02 — Integrate the interactive terminal pane

- [x] TERM-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/TerminalToolWindow.kt` — new themed terminal host and explicit session controls.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTerminalSession.kt` — component attach/detach and resize lifecycle.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — stable project-bound session owner/disposal.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt` — read-only freshness refresh when returning from the terminal.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — invalidate selected-file evidence when its observed hash changes.
- `desktop/src/main/kotlin/io/miniorca/desktop/Main.kt` — deterministic shutdown integration where needed.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopKeyboardNavigation.kt` — terminal focus/toggle and key routing.
- `desktop/src/test/kotlin/io/miniorca/desktop/TerminalToolWindowTest.kt` — new session/control behavior tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopTerminalSessionTest.kt` — hide/show/switch/close process behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — focus-aware shortcut behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — shell-originated file changes reject stale review/analysis evidence.

- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — wire explicit activation and focus return through docked/overlay layouts.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt` — add the terminal destination without migrating the old tools before BOTTOM-02.
- `desktop/src/main/kotlin/io/miniorca/desktop/IdeShell.kt` — exhaustive terminal label/icon routing.
- `desktop/TERMINAL.md` — document terminal controls and verified integration behavior.

- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopLayoutStateTest.kt` — replace the retired-terminal fallback expectation now that Terminal is a supported destination.

**Inputs / dependencies**
- UX-01, TERM-01.

**Implementation rules**
- Match source typography/colors; support input, output, ANSI colors, cursor movement, history, copy/paste, Ctrl+C, resize and scrolling through the terminal library.
- First activation starts one shell; collapsing or switching workspaces preserves it. Closing explicitly ends it; reopening starts a new session.
- On project switch with an active session, let the user cancel the switch or explicitly close that session. Never silently redirect a running shell into the new project.
- Terminal focus owns shell keystrokes, including interrupt; app-wide shortcuts must not steal ordinary terminal input. Provide a deliberate way back to app focus.
- Dispose streams/native resources/processes on restart/project close/app exit, with bounded waiting and visible launch/exit errors. Do not persist terminal transcript or automatically send it to a model.
- On return to source/review, recheck the selected file through the existing read-only file-info boundary; a changed hash makes draft/check/analysis evidence stale. Offer explicit Reindex for changed project inventory. Do not parse terminal output to infer changes or auto-run an analysis after shell commands.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.TerminalToolWindowTest' --tests 'io.miniorca.desktop.DesktopTerminalSessionTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest'`
Native verification additionally covers a prompt, history, interruption, full-screen terminal redraw, focus return, resize and app-exit cleanup.

**Execution record**
Started 2026-09-12. Verified TERM-01 receipt, empty index, all ten unrelated file hashes and both Java toolchains. Captured HEAD/working baselines in the ignored TERM-02 directory. Added the narrowly coupled shell, destination and exhaustive label/icon targets before editing. Terminal documentation records the new controls. Initial implementation; zero corrections. Native UI automation is available.

**Correction 1 — 2026-09-12.** Initial prescribed verification exited 1 at test
compilation: JediTerm's Kotlin `getScreenLines()` is a function, not a synthetic
property. Production compilation passed. Corrected the two buffer assertions.
Diff review also found pending project-switch/app-exit continuations could ignore
a later cancellation; the completion callbacks now check the current user intent.
Re-running the exact focused selection; initial log retained locally.

Correction-1 focused tests passed: 71 tests, zero failures/errors/skips, including
the native PTY test. Its combined quality command failed before tests ran because
Detekt measured MiniOrcaApp 66, DesktopShell 73 and the reducer 68 against the
existing threshold 65. The standalone prescribed test selection then passed.

**Correction 2 — 2026-09-12.** Native testing of a temporary project through a
jpackage verification app using the packaged runtime/jars confirmed prompt,
history and Ctrl+C, but found PTY geometry stayed 80×24: Kotlin delegation did
not forward the Java default `resize` method. Forward it explicitly and assert
the emulator connector reaches the process. Returning to Editor also left the
narrow overlay open; dismiss it before restoring focus. Theme the native
scrollbar, preserve text scaling, refresh source on later Editor/Review entry,
and extract cohesive refresh/focus/shortcut branches from the three oversized
routers. The initial JNI-only UI launcher could not initialize macOS graphics;
the jpackage harness supplies the native launcher and packaged Skiko path.
No user sources or providers were used. Re-run prescribed tests, full desktop
suite, quality, and native checks before acceptance. Two corrections total.

**Blocked checkpoint — 2026-09-12.** Correction-2 prescribed verification exited
1 in production compilation: `ScaledTerminalPanel` passes `(settings, style,
buffer)` to a library constructor expecting `(settings, buffer, style)` at
TerminalToolWindow.kt:388. The two allowed corrections are exhausted. Preserve
the candidate and pending one-line repair; do not mark this card complete.
Correction-1's 71 passing tests are historical evidence, not validation of this
candidate. Full desktop tests, final quality, isolated candidate verification and
required native resize/full-screen/focus/project-switch/app-exit checks remain
unrun or incomplete. All earlier commits and unrelated user deltas are retained;
index empty and HEAD unchanged. Failure recorded in docs/errors.log; scheduler
`mini-orca-ux-implementation` paused through the app tool. No task commit.

**User-authorized repair — 2026-09-12.** The user explicitly requested “repair
TERM-02,” authorizing continuation beyond the recorded stop. All checkpoint
hashes and the empty index match; HEAD remains TERM-01. Correct the terminal
panel constructor order and finish this card's required validation. Earlier
two-correction history remains intact; the scheduler stays paused during repair.

Resumed validation: the focused 71 tests passed. Full desktop validation ran 398
tests with one obsolete expectation: saved Terminal was expected to fall back to
Problems. Add the coupled layout test target and replace that retired-feature
expectation; unknown destinations still fall back safely. Native fixture stores
are now injected in memory so UI verification does not alter saved app preferences.
The first resumed edit command did not match the formatted constructor line; its
unchanged compile failure and the subsequent successful explicit patch are retained
in the local repair logs.

**Accepted after authorized continuation — 2026-09-12.** Corrections 3–7 retain
the previous accounting: correct the constructor order; migrate the obsolete
Terminal layout expectation; finish the App/Shell/index routing extractions;
simplify the remaining Shell overlay assignment to pass the strict complexity
threshold; and clear Compose focus before requesting Editor focus. The last fix
addresses a native-only defect: with a docked Swing terminal, Compose still
considered Editor focused and otherwise left shell keys in Swing. The initial
unmatched constructor edit and intermediate Detekt failures are retained in
`repair-focused.log`, `repair-full-updated.log` and `repair-complete.log`.

Final prescribed selection passes **71 tests**, including the opt-in real PTY
smoke. The working-tree full suite passes **398 tests** and the isolated accepted-
HEAD candidate passes **396 tests**, all with zero failures/errors/skips. Both
candidates pass Detekt, Spotless and `createDistributable`. The isolated package's
native probe passes canonical cwd, UTF-8, real TTY, 121×42 resize, interrupt,
child cleanup and bounded close. Verified JDK 21/JBR 25 paths and all commands
are retained in the ignored evidence directory.

Native UI verification used the actual packaged runtime/application jars with a
synthetic test entry point, in-memory preference stores and temporary source;
no live provider or user project was used. Verified prompt, ANSI color, Unicode
paste, history, Ctrl+C, full-screen Vim, explicit close/reopen, persistent shell
PID across hiding/rehosting, and PTY sizes at wide/narrow layouts. At 1280×650,
Ctrl+Shift+F12 returns native focus, Cmd+P opens the app palette, and a shell edit
appears in read-only source with stale-review status. At 999dp the overlay retains
the session and the same return/open shortcuts work. Recorded 1000dp dock and
999dp overlay screenshots; earlier 800×600 verification and 150% text component
renders are retained. Window-close and Cmd+Q checks both removed the owned shell
and a background sleep process. Project-switch isolation/explicit close are
covered by owner tests; native chooser navigation was canceled, so a complete
native project-switch flow is not claimed. No screen-reader or other-host claim.

The final diff excludes all ten unrelated UI edits. The two overlapping files
retain their pre-existing Context-label deltas only in the working tree; the
isolated candidate keeps the accepted HEAD labels. No Go code changed, so Go,
race and full `make check` were not run. No configuration/data migration is
required. Accept one local TERM-02 commit, verify its exact staged hashes and
receipt, then resume the Astra Extra High scheduler with BOTTOM-01 next. No later
card was started during this repair. Logs, package-input hashes, screenshots and
acceptance receipt are under `.mini-orca/autopilot/ux/TERM-02/`.

## Task BOTTOM-01 — Preserve unique diagnostics in their owning workflows

- [x] BOTTOM-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/BottomEvidenceToolWindows.kt` — identify/move uniquely used diagnostic formatting before deletion.
- `desktop/src/main/kotlin/io/miniorca/desktop/ReviewEvidencePane.kt` — all candidate check output and validation errors under Review.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — analysis run failures in Analysis progress details; verified-scan output in Bugs details.
- `desktop/src/main/kotlin/io/miniorca/desktop/AssistantToolWindow.kt` — generation failures beside the request.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopStatusBar.kt` — retain actionable connection/global operation detail access.
- `desktop/src/main/kotlin/io/miniorca/desktop/DiagnosticText.kt` — new shared sanitizer only if it remains used by multiple owners.
- `desktop/src/test/kotlin/io/miniorca/desktop/ReviewEvidencePaneTest.kt` — failed/stale/skipped checks and output access.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — analysis failure visibility.
- `desktop/src/test/kotlin/io/miniorca/desktop/BugsWorkspaceStateTest.kt` — verified-scan output access.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopStatusBarTest.kt` — global failure access.

- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — pass the request-specific failure into Assistant.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — retain request-scoped generation failure and clear it for a new request/selection.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt` — record only the current generation request failure beside its request.
- `desktop/src/test/kotlin/io/miniorca/desktop/AssistantToolWindowTest.kt` — request failure visibility and target isolation.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — generation failure lifetime and stale response guards.

- `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` — remove the now-unused warning-string projection replaced by scan phase details.

**Inputs / dependencies**
- UX-02, ANA-07.

**Implementation rules**
- Inventory the existing Output entries: generation/validation, selected-file failure, Analyze-all failures, scan phase command/output, daemon errors and latest operation.
- Give each piece one reachable owner; reuse diagnostic sanitization/bounds and preserve Copy/detail access. No new global Output view.
- Keep automated check output distinct from the terminal, where it did not execute. Terminal content does not become validation evidence.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ReviewEvidencePaneTest' --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.BugsWorkspaceStateTest' --tests 'io.miniorca.desktop.DesktopStatusBarTest'`

**Execution record**
Started 2026-09-12. Verified TERM-02 commit/receipt, empty index, all ten unrelated UI edits, toolchains and absence of another writer. Baseline captured under `.mini-orca/autopilot/ux/BOTTOM-01/`. Added the coupled App/state/presenter targets because Assistant has no request-failure input and a generic global error cannot safely identify a generation failure. Initial implementation; zero corrections.

Inventory: Review already owns validation diagnostics and focused checks, but its detailed output must retain the bottom sanitizer and selection access. Unified Analysis already owns per-file/stage failures (including migrated Analyze-all); make failure reasons selectable and bounded. Bugs owns scan status/trust but lacks phase command/output. Assistant needs a retained target-specific request failure. Current status owns daemon errors/latest operation and must preserve selected-file failure text plus selectable, scrollable details. No new global Output surface; obsolete bottom components are removed in BOTTOM-02.

**Correction 1 — 2026-09-12.** The prescribed selection passed 35 tests. The
combined full-suite/quality run failed Detekt: DesktopState.reduce measured 65
against the strict threshold 65. Extract its existing validation-start transition
without changing behavior. Visual inspection also found raw control characters
in the old scan-warning preview; the new phase details replace that duplicate
preview, and scan actions precede long output. The status-dialog render fixture
needs the same full-size parent used by production for correct popup placement.
The initial logs/screenshots are retained. Re-run prescribed/full/quality checks.

**Correction 2 — final review, 2026-09-12.** Correction 1 passed the 35 prescribed
tests, 405 working-tree tests, 403 isolated tests, Detekt and Spotless. Final
review found the replaced scan-warning projection had no production consumers;
remove it and its obsolete string assertion (phase visibility is now covered by
the rendered Bugs test). A failed generation also retained the old cancellation
status from request teardown; record a failed-request status with the scoped
failure and assert it. Native status details are centered/readable at 800×600,
and selecting/copying their text into file search works. The offscreen dialog
image remains unsuitable for placement claims; native evidence is retained.

**Accepted — 2026-09-12.** The final prescribed command passes **35 tests**;
full working-tree tests pass **405**, isolated accepted-HEAD candidate tests
**403**, with zero failures/errors/skips. Both candidates pass Detekt and
Spotless; the final working package builds successfully. Validation used the
verified JDK 21 launcher and JBR 25 toolchain through `scripts/desktop-gradle.sh`.
Exact logs and counts are in `.mini-orca/autopilot/ux/BOTTOM-01/`.

Review retains failed/skipped/stale check commands and bounded, selectable output;
its current-draft/Apply guards remain authoritative. Assistant retains a failure
for the matching request target, clears it for a new attempt or file, and rejects
late failures from replaced requests. Analysis owns run/stage failure details;
Bugs owns all verified-scan phase commands and output. Scan execution controls
precede output, and the obsolete warning-string projection is removed. Current
status retains selected-file failures, daemon errors and latest-operation detail
in a bounded scrolling disclosure. The shared renderer preserves line breaks and
tabs, strips control characters, and visibly marks output beyond 4,096 characters.
Opening disclosures is read-only and never dispatches model requests or checks.

Diff review and visual inspection pass for the affected components at narrow
widths and 150% text. A packaged native fixture using temporary source and
in-memory preferences verified the status dialog at 800×600 and selected/copied
its diagnostic text into file search. The fixture was closed. Native dialog
placement is not inferred from the offscreen images; the unchanged final dialog
and text renderer match the native-tested sources. No new native PTY, other-host,
screen-reader or live-provider execution is claimed. No Go code changed, so Go,
race and full `make check` were not run; no configuration/data migration is needed.

Two focused corrections total are retained above. All ten unrelated UI files
remain byte-identical and excluded from the independently tested candidate. Stage
only this card's implementation, checklist/status and failure history, create one
authorized local BOTTOM-01 commit and verify the receipt before ending this wake.
The scheduler remains Active with BOTTOM-02 next; no later card has started.

## Task BOTTOM-02 — Replace the bottom tools with Terminal only

- [x] BOTTOM-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt` — terminal-only preference model and legacy bottom selection fallback.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — terminal dock, collapse control and bounded narrow overlay.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — remove Problems/Checks/Output construction and connect terminal.
- `desktop/src/main/kotlin/io/miniorca/desktop/BottomEvidenceToolWindows.kt` — delete after useful shared code is moved.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProblemsToolWindow.kt` — delete if exclusively used by the removed bottom surface; move any remaining shared finding UI first.
- `desktop/src/test/kotlin/io/miniorca/desktop/BottomEvidenceToolWindowsTest.kt` — remove obsolete surface tests after moving meaningful assertions.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProblemsToolWindowTest.kt` — move shared finding assertions or remove obsolete tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopLayoutStateTest.kt` — old bottom preferences restore collapsed Terminal.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — terminal wide/narrow/collapsed/failed states.

- `desktop/src/main/kotlin/io/miniorca/desktop/IdeShell.kt` — replace the old bottom tab primitives with terminal-only dock, opener and overlay.
- `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` — remove the now-unreferenced bottom problem summary model.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopShellTest.kt` — migrate removed bottom-tab expectations and verify bounded terminal height.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — remove obsolete bottom-tab semantics expectations; preserve terminal routing checks.
- `desktop/src/test/kotlin/io/miniorca/desktop/FindingsPresentationTest.kt` — retain the shared finding behavior tests previously housed under the removed Problems surface.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — remove the compact bottom-only finding row and its now-unused details callback; retain shared result/action components.
- `desktop/TERMINAL.md` — document the sole bottom control and startup preference migration.

**Inputs / dependencies**
- NAV-01, TERM-02, BOTTOM-01.

**Implementation rules**
- Remove all three obsolete bottom tabs, counts, automatic-opening callbacks and associated dead types; preserve only the terminal control and the separate integrated status bar.
- Keep resizable docked height and the <1000dp bounded overlay. Restore pane dimensions without auto-starting a shell or reviving an old bottom selection.
- Collapsing/opening must not recreate the process. Returning from drawers/palette restores focus predictably.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopLayoutStateTest' --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

**Execution record**
Started 2026-09-12. Verified BOTTOM-01 commit/receipt, empty index, all ten unrelated UI edits and both Java toolchains. Saved HEAD/working baselines under `.mini-orca/autopilot/ux/BOTTOM-02/`. Added the directly coupled terminal chrome, obsolete summary model and required behavior-test targets before editing. Preserve shared finding/filter/navigation assertions in their owning test class; obsolete Checks/Output UI assertions are covered by BOTTOM-01 workflow diagnostic tests. Initial implementation; zero corrections.


**Correction 1 — 2026-09-12.** The prescribed command stopped at compilation: nested Compose receiver scope hid `BoxWithConstraints.maxHeight`. Capture viewport height beside width before entering nested pane scopes. Review also found the removed Problems surface was the last consumer of its compact finding row, optional Details callback and severity-summary extension; remove those directly coupled dead helpers and obsolete helper-only assertion. Retained finding navigation/filter tests now live in FindingsPresentationTest. Failed command/output: `focused.log`.

**Accepted — 2026-09-12.** Correction 1 passes the prescribed three-class command:
**61 tests**, zero failures/errors/skips. The full working tree passes **400 tests**;
the isolated candidate excluding unrelated UI edits passes **398 tests**. Both
pass Detekt and Spotless; the isolated candidate also passes `createDistributable`.
The five-test net reduction removes six obsolete bottom-panel/summary tests and
adds a terminal control/lifecycle presentation test. Three shared finding tests
were moved intact in purpose to FindingsPresentationTest; filter and source-only
navigation interaction tests now exercise the actual Bugs workspace. Workflow
check/output diagnostics remain covered by the accepted BOTTOM-01 tests.

The bottom destination enum, old pane constructors, tab selection/counts, summary
models and compact bottom-only finding helpers are removed. One Terminal control
owns open/collapse; the integrated status bar remains separate. Startup keeps pane
sizes and restores a collapsed terminal, ignoring/removing old bottom selections
and visibility preferences. It never starts a process from saved layout. Dock
height temporarily clamps in short windows without overwriting the saved value.

Native verification on macOS arm64 used the isolated packaged runtime/application
jars and the existing synthetic terminal entry point with temporary project and
in-memory preferences. Verified 800x600 startup/overlay, 1280x650 dock and resize,
exact 1000dp dock/999dp overlay, and 1280x600 short-window layout. One shell PID
survived collapse/open, Hide/Enter reopen, resize and responsive rehosting; observed
PTY dimensions changed with the available region. Ctrl+Shift+F12 restored native
app shortcuts in both presentations. Drawer dismissal, palette navigation and
terminal opener focus worked; Cmd+Q stopped the owned shell. Component renders
cover collapsed/expanded/failed state and 150% text. No new other-host, screen-reader
or full-screen-terminal claim. Native checks, screenshots and package-input hashes
are under the ignored BOTTOM-02 evidence directory.

Final scope review found no unrelated changes in the isolated candidate, no
remaining consumers of deleted helpers and no added model/source-write authority.
All ten prior UI edits remain preserved; three overlapping files were reconstructed
against their saved baseline to exclude the user changes from the tested commit.
No Go code changed, so Go/race/full `make check` were not run. Preference migration
is automatic; no manual configuration step is needed. Create and verify the one
local BOTTOM-02 commit, then end this wake with VERIFY-01 next and scheduler Active.

## Task VERIFY-01 — Validate the complete interaction and document support

- [x] VERIFY-01 completed with required checks and diff review.

**Target files**
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopIntegrationCoverageTest.kt` — project-wide analysis, separate result pages/file filters and creation handoffs.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — final production-component fixture matrix.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAccessibilityTest.kt` — names, states, focus and read-only semantics.
- `desktop/src/test/kotlin/io/miniorca/desktop/IdeUiContractBaselineTest.kt` — update the baseline to actual supported controls.
- `desktop/KEYBOARD_SMOKE_CHECKLIST.md` — new navigation/terminal procedures.
- `desktop/UI_CONTRAST.md` — measured final text/state/focus contrast.
- `desktop/README.md` — actual Analysis, creation and terminal usage/shortcuts.
- `README.md` — supported user workflow and migrations.
- `docs/insights-performance/README.md` — new section ownership and preserved evidence distinctions.
- `docs/RELEASE_ACCEPTANCE.md` — actual automated/native results and limitations.
- `PLAN.md` — task status, accepted decisions and outstanding dependencies.

- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAcceptanceFixture.kt` — isolated native entry point reusing production panes and existing test data, with explicit text-scale and lifecycle selection; no provider access.
- `tasks/README.md` — record the authorized scheduler stop after final acceptance.

- `internal/app/analysis_run_preview.go` — split inventory, stage planning and resume request accounting while retaining identical admission guards.
- `internal/app/analysis_run_store.go` — separate persisted header, file, stage and aggregate evidence validation.
- `internal/app/analysis_run_results.go` — separate read-only semantic, performance and security evidence projection.
- `internal/app/analysis_run.go` — separate section validation, locked controls, stage dispatch and publication guards.
- `internal/app/analysis_compatibility.go` — factor compatibility budget/scope checks and shared queue identity validation.
- `internal/app/analysis_file.go` — separate request validation and rejected-stage presentation.
- `internal/app/file_analysis.go` — extract semantic risk validation without changing diagnostic behavior.
- `internal/app/service.go` — remove unreachable schema retry wrapper.
- `internal/app/performance_review.go` — remove unused standalone performance orchestration replaced by the shared stage pipeline.
- `internal/app/performance_review_test.go` — run existing publication safety regressions through the production stage pipeline.
- `internal/project/security.go` — remove unused store convenience wrapper; retain the guarded cache store.
- `internal/app/file_analysis_test.go` — wait for canceled worker completion before deleting its temporary project; preserve cancellation assertions.

**Inputs / dependencies**
- CREATE-02, BOTTOM-02 and every preceding analysis/UI task.

**Implementation rules**
- Review the aggregate diff for obsolete controllers/surfaces, accidental duplicate rules, hidden actions, unhandled failures and unrelated edits. Update dependent existing tests rather than keeping obsolete UI behavior alive.
- Native visual matrix: wide, 1000/999dp, 800×650, 1280×600, 125/150% text; long content/paths/errors; each result state and each terminal state. Verify important information is distinguishable without depending on color alone.
- End-to-end with a fake provider/temp project: partial three-section run, cancel/resume/restart, source change, new function in a package-only file, draft preservation, Review/Apply/Undo and terminal-triggered evidence invalidation. No live provider is needed.
- Verify terminal packaging on the supported macOS arm64/JBR 25 host, local cwd, keyboard routing, PTY resize and process cleanup. Record any untested host separately; do not broaden the accepted platform claim from compilation alone.
- Document report/prompt identity and visual-preference migrations. No model configuration rename is assumed. Keep historical release/qualification limits intact.

**Verification command**
```sh
./scripts/validate.sh
./scripts/desktop-gradle.sh createDistributable
git diff --check
```
`scripts/validate.sh` covers the formatting, Go, race, vet, desktop and dispatcher
checks represented by `make check`, plus static and daemon-contract gates. Do not
repeat equivalent full suites once they pass unless subsequent changes justify it.
Use the existing production-component reproduction command in
`docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks` and record native smoke
separately. Report executed/cached/skipped/unavailable checks accurately.

**Execution record**
Started 2026-09-12. Verified BOTTOM-02 receipt, accepted HEAD, empty index, all ten unrelated UI hashes and absence of another writer/validator. Baselines saved under `.mini-orca/autopilot/ux/VERIFY-01/`. Review the aggregate implementation and current test coverage before adding the final integration/native fixtures; retain historical release limitations. Initial implementation; zero corrections.

Preparation inventory: existing fake-provider/temp-project Go integration tests already cover partial categorized runs, cancellation, resumed generations, process-loss restoration, package-only creation through real validation/checks/Apply/Undo and source-conflict rejection. Reuse their full-gate results instead of duplicating those pipelines. Add a native visual fixture to close the formerly unverified text-scale/lifecycle matrix, and extend desktop cross-boundary tests for draft preservation and source invalidation across all result pages.

**Correction 1 — 2026-09-12.** The first prescribed full gate passed Go formatting/tests/race/vet, daemon contracts, dispatcher tests and desktop static/tests, but failed Go quality: three unreachable helpers and twelve analysis functions above complexity 15. Preserve the full failure in `.mini-orca/autopilot/ux/VERIFY-01/validate.log`. Add the directly affected backend files and publication tests above before editing. Remove the obsolete wrappers, migrate their safety tests to the actual shared stage pipeline, and separate inventory/planning, stored-state validation, evidence projection and locked lifecycle steps without weakening guards or quality thresholds. No native acceptance claim yet.

**Correction 2 — 2026-09-12.** The correction-1 aggregate run exposed a fixture cleanup race: `TestAnalyzeAllCancelRetainsCompletedEntries` saw the legacy Canceled projection and deleted its temporary directory while the shared worker was finishing its final save (`TempDir RemoveAll cleanup: .../.mini-orca/analysis: directory not empty`). The race suite passed. Add an explicit wait on the existing worker-completion signal after cancellation; do not add sleeps, retries, suppress the cleanup error, or change production cancellation semantics. The initial full-gate log is retained as `validate-correction-1.log`; the remaining stages passed except one residual complexity value of 16 in `runAnalysisWindow`. Extract its final queue-freshness/completion transition into a locked helper, retaining the final save and its error handling. This is the second and final focused repair.

**Accepted — 2026-09-12.** The second correction passes the complete prescribed
`./scripts/validate.sh` gate. All nine stages pass, including Go tests/race/vet,
static/reachability/complexity/clone checks, daemon contracts and dispatcher tests
(55, one opt-in insight-runtime conformance skip). Desktop has 403 isolated tests
and 405 tests with the user's UI changes, zero failures/errors/skips; Spotless and
Detekt pass in both. The explicit component reproduction passes 43 tests with fresh
renders. `createDistributable`, the packaged-runtime PTY probe and diff checks pass.
Cached package/Gradle results are reused where source inputs are unchanged.

Native production-pane fixtures verify all eleven analysis/result states at
800×650/150%, 1000/999dp Editor layouts at 125%, 1280×600 at 150%, long failures,
and every terminal lifecycle label. One real shell PID survives resize/navigation
and text-scale changes; actual PTY dimensions change. Focus return, natural exit 7,
explicit reopen/new PID and explicit close pass; both shell PIDs and the fixture
app are stopped. Earlier TERM-02/BOTTOM-02 full product native evidence remains
applicable to unchanged Desktop production code. Synthetic projection values and
fixture-injected font scale are layout evidence; real service tests separately
prove counts, consent, cancellation/restoration and one-file creation/Apply/Undo.
Exact evidence and remaining platform/provider limits are recorded in
[release acceptance](RELEASE_ACCEPTANCE.md#ux-implementation-acceptance--2026-09-12).

Review confirms one analysis owner; required public compatibility adapters converge
on it. Obsolete bottom surfaces and dead backend wrappers are removed. The backend
cleanup preserves lock boundaries, durable request reservations, source/report
identity checks, sanitized failures and explicit provider consent. Existing
performance publication safety tests now run the actual shared stage pipeline;
the canceled-worker fixture uses synchronization rather than timing or retries.
No quality limit, test or guard was weakened. No configuration rename or manual
migration is needed; source/diff remain read-only until explicit reviewed Apply.

All ten unrelated UI edits are preserved and excluded from the exact tested
candidate; overlapping documentation/test changes round-trip against baselines.
Code hashes remain unchanged through final documentation. The app confirmed the
scheduler is Paused after all 17 cards passed. Create the one authorized local
VERIFY-01 commit and verify its receipt before ending this wake; do not start any
new task. No push, release, live provider call or destructive cleanup was performed.
