# Mini-Orca desktop UI refactor plan

## Chosen direction

Implement the recommended hybrid with the Focus Flow color system:

- use Direction A's persistent desktop shell: top bar, labeled workspace rail,
  project explorer, primary work surface, and contextual review panel;
- use Direction C's explicit Editor sequence: **Target → Draft → Verify →
  Apply**;
- use Direction B's calm, plain-language final decision copy;
- use Direction C's deep indigo, violet, cyan, and mint palette throughout.

The target is a strong visual and interaction refactor, not a new product model.
The daemon API and Mini-Orca's one-project, one-file, one-declaration,
preview-first boundaries remain unchanged.

## Execution strategy

The implementation is decomposed into dependency-ordered Tasks 57–74 in
`tasks/INDEX.md`. Each task replaces one cohesive vertical slice, runs its focused
Desktop checks, removes the superseded implementation, and leaves the application
green before the next task starts. `tasks/PROMPT_EXECUTE_UI_REFACTOR.md` is the
copyable master prompt for executing the complete chain sequentially with one
implementation writer.

## Product invariants

The new UI must preserve these behaviors in every phase:

1. Source and composed diffs are selectable and read-only.
2. Only the isolated declaration/import draft is editable.
3. A draft cannot reach Apply unless its project revision, file hash, draft
   revision, validation, and focused checks are all current.
4. Apply names the only writable file and symbol and requires an explicit user
   action.
5. Advisory impact and Git state remain read-only context.
6. Remote-provider confirmation remains visible before prompt-bearing requests.
7. State is expressed with text and semantics, never by color or icons alone.
8. Keyboard navigation and the below-1000dp drawer behavior remain supported.

## Target information architecture

### Persistent desktop shell (wide layout)

Use four stable regions:

1. **Top bar** — Mini-Orca identity, project/revision breadcrumb, local/remote
   connection state, Command, Re-index, and Open project.
2. **Workspace rail** — labeled Summary, Analysis, Bugs, and Editor destinations
   with textual counts and selected semantics.
3. **Project explorer** — filter, relative tree, selection, disclosure controls,
   and compact textual freshness badges.
4. **Work area** — one primary canvas and one contextual panel. In Editor, the
   contextual panel changes with the active stage rather than showing every
   control at once.

Remove the current always-visible metric strip. Its project facts belong in
Summary; its selected-file facts belong in the Editor context header. This
recovers vertical space and removes duplicated information.

Keep the existing resizable explorer/action widths and persistence. At widths
below 1000dp, hide both side panes and expose labeled **Files** and **Context**
drawer buttons. Keep the workspace rail or an equivalent labeled compact
switcher; never squeeze all wide-layout panes into the window.

### Editor flow

The stage bar is visible throughout Editor. Stages are unlocked from existing
state rather than maintained as a second source of business truth.

| Stage | Main canvas | Context panel | Exit gate |
| --- | --- | --- | --- |
| Target | Read-only source with selected line/symbol emphasis | File, replace/create mode, exact symbol, bounded-context summary | `validateChatTarget` returns a valid exact target |
| Draft | File-scoped conversation plus editable declaration/import draft | Draft identity, dirty/validating/invalid/stale state, Inspect context, Validate | Latest draft validation is applicable and still matches the open file |
| Verify | Side-by-side Before/Proposed declaration comparison, with unified view available | Required checks, diagnostics, advisory impact, read-only Git context | `draftReviewEligibility` is eligible for the exact draft revision/hash |
| Apply | Read-only composed diff and concise change summary | Calm “Safe to apply” decision card, exact target, proof rows, explicit Apply | User presses the enabled target-naming Apply button |

Users may revisit an earlier unlocked stage. They may not jump forward past a
gate. Editing a validated draft clears validation/checks and returns the visible
flow to Draft. A stale project/file identity clamps the flow back to Target or a
stale Draft recovery state. After a successful write, Apply becomes an applied
receipt with the new revision/hash and Undo when available.

The Apply stage itself is the deliberate confirmation surface; do not stack a
generic confirmation dialog on top of it. The enabled button must say, for
example, **Apply RegisterRoutes to router.go**, and the surrounding copy must say
that nothing has changed yet.

## Visual system

Replace scattered global colors with a single `MiniOrcaTheme` and semantic
tokens:

| Token | Value | Use |
| --- | --- | --- |
| App background | `#080917` | Window and deepest canvas |
| Surface | `#101225` | Main panels |
| Raised surface | `#171A31` | Cards, controls, active rows |
| Strong surface | `#222640` | Focused controls and selected containers |
| Border | `#292D49` | Dividers and outlines |
| Primary text | `#F2F2FB` | Main copy |
| Secondary text | `#9297B6` | Supporting copy |
| Primary accent | `#9B8CFF` | Active stage and primary focus |
| Secondary accent | `#62D8EF` | Navigation and code context |
| Success | `#55DDB0` | Passed/current state |
| Warning | `#FFC86E` | Stale/attention state |
| Error | `#FF7F9F` | Failed/invalid state |

Use system UI and monospace fonts; do not add a font download. Use a restrained
4/8px spacing rhythm, 8–14px radii, clear focus rings, and minimal static
gradients. Avoid blur-heavy effects that are expensive or inconsistent on
Compose Desktop.

Replace emoji and ad-hoc glyphs with a small consistent Compose vector icon set.
Every navigation icon keeps a visible text label; every icon-only utility gets a
tooltip and content description. Implement the simple Mini-Orca mark as a
code-native Compose drawing rather than a raster dependency.

## Code structure

Keep API orchestration in `MiniOrcaApp`; make the new UI stateless where
practical. Do not mirror server/domain state in composables.

### Presentation state

Add `EditorFlowState.kt` with small pure types:

- `EditorStage { Target, Draft, Verify, Apply }`;
- `EditorFlowUiState`, containing the active/unlocked stages and display-ready
  target, draft, validation, check, and apply summaries;
- `editorFlowUiState(...)`, derived from `DesktopState`, `ChatEditMode`, and the
  existing target/apply eligibility functions;
- `DiffRow`/`DiffCell` plus `sideBySideDiffRows(UnifiedDiff)` so the C-style
  comparison is deterministic and testable.

Only the user's currently viewed unlocked stage is transient UI state. Clamp it
against `EditorFlowUiState.unlockedStages` whenever source state changes. The
project/file/draft/check data remains owned by `DesktopState`.

### Compose files

Refactor by responsibility, keeping files cohesive rather than creating a
component for every visual atom:

- `DesktopTheme.kt` — `MiniOrcaTheme`, semantic colors, typography, shapes,
  spacing, status colors, and code token colors.
- `DesktopShell.kt` — responsive slot-based scaffold, global shortcuts,
  resizable panes, drawers, modals, and status bar only.
- `DesktopHeader.kt` — replace the current horizontal workspace row with
  `AppTopBar` and `WorkspaceRail`; retain pure label/semantics helpers.
- `ExplorerPane.kt` — apply the Workbench tree treatment while retaining the
  existing relative-path, filter, collapse, selection, and freshness logic.
- `EditorWorkspace.kt` (new) — stage bar, stage navigation/clamping, source/diff
  canvas, and stage-specific context slots.
- `EditorStagePanes.kt` (new) — Target and Draft surfaces extracted from the
  current editor brief and focused-action UI.
- `DiffViewer.kt` (new) — selectable unified and side-by-side read-only views,
  line numbers, scope framing, and added/removed text labels in semantics.
- `ReviewEvidencePane.kt` (new) — Verify evidence and the final Apply decision
  card, including applied/Undo receipt.
- `WorkspacePanes.kt`, `ProjectSummaryPane.kt`, and `SummaryPane.kt` — retain
  behavior but adopt the shared cards, section headers, empty states, filters,
  and typography after Editor is complete.
- `CommandPalette.kt` — retheme and preserve keyboard-first selection.

Do not pass the current large candidate/comparison callback list through
`DesktopShell`. Give each workspace its own content composable and a small,
named actions type where grouping materially reduces the call signature.

## Remove obsolete desktop UI in the same refactor

The current repository documents the whole-file generation UI as retired, and
the supported send action already uses file-scoped chat/drafts. Before building
the new Editor, remove the unreachable parallel preview implementation so the
new flow does not sit beside legacy UI:

- delete `ReviewPane.kt` and its candidate/comparison/activity presentation;
- remove the candidate branch from `ContentPane`;
- remove `generatePreview`, alternate comparison, candidate export/check/apply,
  duplicate `applied`, activity refresh, and their unused local UI fields from
  `DesktopApp.kt`;
- remove candidate-only state/events/controller methods and
  `candidateApplyEligibility` from `DesktopState.kt`;
- remove `PromptTemplates.kt` and its tests if its dead `prepareTemplate` call
  remains unreferenced when implementation starts;
- remove candidate-only desktop API methods/models/tests when no supported
  desktop path consumes them; keep the draft check report, renaming it to
  `DraftCheckReport` only if that can be completed atomically across desktop
  call sites.

This cleanup is desktop-scoped. Do not change daemon routes or external API
contracts as part of the visual refactor.

## Implementation sequence

### 1. Lock behavior and remove the dead UI path

- Add the initial `EditorFlowUiState` gate tests against current state types.
- Prove the supported chat/draft path covers send, edit, validate, checks,
  review, Apply, and Undo.
- Remove the retired candidate UI and its desktop-only wiring/tests.
- Keep the app compiling and all remaining desktop tests green before visual
  work begins.

Acceptance: no supported behavior changes; `DesktopShell` no longer receives
candidate/comparison/activity callbacks; no old/new review panes coexist.

### 2. Install the Focus Flow theme and reusable primitives

- Replace raw colors with semantic theme tokens.
- Add shared surface, section label, status badge, labeled icon button, stage
  indicator, empty/error/loading state, and code-line primitives only where
  they remove real duplication.
- Move syntax highlighting to theme colors without changing source text.
- Add visible keyboard focus and disabled-state treatments.

Acceptance: the existing app renders entirely from the new palette, all statuses
retain text, and source highlighting still returns identical text.

### 3. Replace the desktop shell

- Build the top bar and labeled workspace rail.
- Move project metrics out of the global strip.
- Restyle Explorer while retaining lazy rows, collapse/filter behavior, stable
  selected path, and pane width persistence.
- Preserve the 1000dp breakpoint and labeled Files/Context drawers.
- Keep connection, status/error, command palette, and cancellation behavior.

Acceptance: all four workspaces remain reachable by pointer and existing
shortcuts; resizing across 999/1000dp never compresses three panes.

### 4. Implement Target and Draft

- Consolidate duplicated file/symbol facts into the Target context panel.
- Keep source selectable/read-only in the main canvas.
- Split `FocusedActionPane` into exact-target selection, bounded conversation,
  editable draft, remote confirmation, context inspection, and Validate actions.
- Use actual `FocusRequester`s so Focus Chat and Focus Draft shortcuts focus the
  intended controls rather than merely opening the action palette.
- Show dirty, validating, invalid, valid, and stale states with text and clear
  recovery actions.

Acceptance: create/replace modes, prepared bug/suggestion requests, remote
confirmation, cancellation, manual edits, and validation work exactly as today.

### 5. Implement Verify and Apply

- Add deterministic unified and side-by-side diff projections.
- Present validation diagnostics and focused checks in the Verify evidence
  panel; keep command output collapsed until requested.
- Show advisory impact and Git context as explicitly read-only.
- Enable Continue to Apply only for the current draft identity.
- Build the final calm decision card and target-naming Apply action.
- Replace the decision card with an applied receipt and Undo action after
  success.

Acceptance: editing or staleness immediately relocks Verify/Apply; no Apply call
is possible from Target, Draft, or Verify; one explicit Apply action mutates only
the named file.

### 6. Bring Summary, Analysis, Bugs, and dialogs into the system

- Recompose Summary into scan-friendly fact, interpretation, and coverage
  cards.
- Turn Analysis job controls/progress and Bugs filters/findings into consistent
  toolbars and cards without changing scan/triage behavior.
- Use lazy lists for potentially large finding/job collections where selection
  behavior permits it.
- Retheme command/context dialogs and all empty, loading, disconnected, failed,
  stale, and canceled states.

Acceptance: deterministic facts remain visually distinct from model
interpretation; verified findings remain distinct from AI suggestions.

### 7. Accessibility, responsive polish, and documentation

- Verify reading/focus order follows rail → explorer → stage canvas → context.
- Preserve `⌘1`–`⌘4`, `⌘Tab`, `⌘P`, `⌘⇧O`, `⌘K`, `⌘Enter`, `⌘⇧F`,
  `⌘⇧D`, `⌘⇧V`, `⌘⇧C`, and `Esc`.
- Add content descriptions and selected/disabled semantics to the new rail,
  stages, badges, diff lines, drawers, and decision controls.
- Check text contrast, focus contrast, text scaling, long paths/symbol names,
  and narrow layouts.
- Update `desktop/README.md` and `desktop/KEYBOARD_SMOKE_CHECKLIST.md` after the
  replacement is complete.

Acceptance: every state remains understandable without color; source/diff stay
read-only; no horizontal clipping at the supported narrow breakpoint.

## Test plan

Add or update focused unit tests for:

- stage unlock/clamping across empty, valid target, generated, dirty, invalid,
  valid, checked, stale, applied, and undone states;
- stale validation/check identity never unlocking Apply;
- side-by-side diff alignment for context, additions, removals, and line numbers;
- workspace/stage semantics including selected state and textual counts;
- target-naming Apply copy and disabled reasons;
- pane width bounds, 999/1000dp behavior, explorer filtering/collapse, source
  text preservation, and keyboard routing;
- removal of retired candidate UI assumptions from desktop tests.

Run while iterating:

```text
./desktop/gradlew -p desktop test --tests io.miniorca.desktop.EditorFlowStateTest
./desktop/gradlew -p desktop test --tests io.miniorca.desktop.DesktopShellTest
./desktop/gradlew -p desktop test --tests io.miniorca.desktop.DraftReviewWorkflowTest
```

Before handoff:

```text
./desktop/gradlew -p desktop test
make check
```

Perform the updated keyboard smoke checklist at wide layout, exactly 1000dp,
and below 1000dp. Exercise local and remote providers, disconnected/loading/error
states, a dirty draft, failed validation, failed checks, stale revision, Apply,
and Undo.

## Completion criteria

The refactor is complete when:

- the production UI visibly matches the selected hybrid and Focus Flow palette;
- the Editor communicates a single, gated Target-to-Apply story;
- no retired candidate UI remains beside the draft workflow;
- duplicate file/symbol/status information has been removed;
- all project mutations remain explicit and single-file;
- desktop tests, full supported validation, and manual accessibility/responsive
  checks pass, with any environment-limited checks reported.

No configuration or data migration is required.
