# Desktop UX/UI simplification plan

Date: 2026-09-02

Status: Complete — automated validation passed; manual GUI checks are documented and were not run in this non-interactive session.

Task source: [`tasks/INDEX.md`](tasks/INDEX.md)

Sequential execution prompt:
[`tasks/PROMPT_EXECUTE_ALL_TASKS.md`](tasks/PROMPT_EXECUTE_ALL_TASKS.md)

## 1. Outcome

Refine the Compose Desktop client so the interface is quieter, denser, and easier to
scan without changing Mini-Orca's preview-first workflow.

The finished UI should:

- show only the **Open project** action when no project is open;
- remove routine project/file counts and revision/hash identifiers from the visible UI;
- use consistent semantic colors to distinguish purpose, state, and action priority;
- remove persistent explanatory copy that repeats what a nearby control already says;
- reduce button and text-input size moderately while keeping keyboard and pointer use
  comfortable;
- retain visible text for important state, errors, destructive actions, and safety gates.

This is a Desktop presentation refinement. Project revisions, file hashes, draft hashes,
and identity checks remain unchanged in state, API requests, validation, checks, Apply,
Undo, and asynchronous response guards.

## 2. Audited baseline

The current Desktop implementation exposes the requested issues in a few concentrated
places:

- `DesktopShell` renders the top bar, workspace rail, workspace content, and status bar
  even when `appState.project` is null.
- `projectBreadcrumbLabel` and `explorerProjectLabel` show the project revision; the
  explorer label also shows the indexed-file count.
- Project Summary shows file/source/line inventory and the full project revision.
- The workspace rail adds analyzed-file, finding, and draft counts to navigation labels.
- Target, Draft, Verify, Apply, finding, and Analyze-all presentation copy exposes
  revisions or hashes even though the guards work without displaying those values.
- Editor stages, suggestions, scan/analysis controls, and draft actions repeat explanatory
  sentences below or beside already clear controls.
- Buttons mostly distinguish only primary versus neutral actions. Pause, Cancel, Apply,
  navigation, and neutral utility actions therefore compete visually.
- Stock Material text fields occupy more vertical space than the rest of the desktop UI.
- Connection information appears in both the top bar and bottom status bar with more
  endpoint/model/latency detail than is needed during normal use.

## 3. Product decisions

### 3.1 Dedicated no-project state

Derive one shell mode from `appState.project != null` and use it as the single visibility
decision.

When no project is open, render a dedicated landing state containing only:

- the Mini-Orca mark and product name as non-interactive identity;
- one prominent **Open project** button;
- a progress indicator while the selected project is opening;
- one concise inline error only after an open attempt fails.

Do not render the normal top bar, workspace rail, explorer, Editor context, workspace
canvas, status bar, Command/Re-index/Reconnect buttons, or Files/Context drawers in this
state. Project-, file-, symbol-, workspace-, draft-, and palette-related shortcuts must be
ignored while the landing state is active. Add `Cmd/Ctrl+O` as the keyboard equivalent of
the one available Open project action.

Canceling the native directory chooser leaves the landing state unchanged. While an
import request is running, disable duplicate activation and keep the progress state in the
same landing surface. A failure leaves Open project available for retry. A successful
import replaces the landing state with the normal project workspace.

### 3.2 Remove implementation identity from routine presentation

Keep identity values in models and guard logic, but do not show raw revision or hash text
in normal screens.

| Surface | New presentation |
| --- | --- |
| Top bar | Project name only; no revision SHA |
| Workspace rail | `Summary`, `Analysis`, `Bugs`, `Editor`; no numeric counts |
| Explorer header | `PROJECT EXPLORER` and compact filter only; no indexed-file count or revision |
| Project Summary | Project type, build metadata, and detected language names; no project inventory/per-language counts or revision |
| Target | Relative path, language, size, line count, and freshness; no file hash |
| Draft | Target name and concise draft state; no project revision, draft revision, or hashes |
| Verify | `Current`, `Stale`, `Passed`, or `Failed` identity/evidence states; no raw identifiers |
| Apply/Undo receipt | Human-readable result and target; no project revision or resulting hash |
| Bugs | Lifecycle and freshness; no project revision in the finding status label |
| Analysis | Current/stale wording without printing the revision value |

Selected-file line count and size remain useful local context. Project-wide inventory
counts are removed from the default Summary because they do not help the focused
one-file workflow.

### 3.3 Reduce visible explanation

Use a simple copy rule: if a label restates a nearby button, tab, badge, or heading,
remove it from the persistent layout.

Remove or collapse:

- page subtitles that only explain the page name;
- the Explorer freshness legend;
- the sentence below the Editor stage buttons;
- suggestion summaries printed under each suggestion button;
- repeated preview/read-only explanations around the same diff;
- repeated Analyze-all/scan lifecycle explanations already represented by a status badge;
- the sentence below Send message;
- finding-classification descriptions when the section label already identifies the
  source;
- duplicate control-state details repeated in both a run-summary panel and control panel.

Keep visible copy when it is actionable or safety-critical:

- empty, loading, failure, stale, and validation states;
- the remote-provider destination and its explicit confirmation;
- the reason a required next step is blocked;
- validation diagnostics and failed-check output;
- the exact file/symbol named by Apply;
- the applied/Undo result.

Longer explanations can move to hover tooltips, accessible semantics, or an explicit
details disclosure. They must not remain as always-visible labels below buttons.

### 3.4 Semantic action colors

Extend the existing Focus Flow palette with action roles rather than assigning arbitrary
colors per screen.

| Role | Color | Examples |
| --- | --- | --- |
| Primary | Violet `Accent` | Open project, Analyze, Start, Send, Validate |
| Navigation | Cyan `CyanAccent` | Workspace selection, Open in Editor, Context, view switches |
| Positive | Mint `Success` | Continue to Apply, Apply when eligible, completed/current state |
| Attention | Amber `Warning` | Pause, stale state, confirmation required, Undo |
| Destructive/interruption | Rose `Error` | Cancel, Dismiss, failed/invalid state |
| Neutral | Raised/strong surface | Close, Inspect details, secondary utilities |

Implement this as one small `ActionTone` (or equivalent) input on the shared button
primitive. Primary actions use a filled treatment; secondary actions use a tinted surface
or colored outline; selected navigation uses a distinct filled/outlined state. Disabled,
hovered, pressed, and keyboard-focused treatments must remain legible.

Color reinforces meaning but never carries it alone. Buttons retain text, statuses retain
labels, selected controls retain semantics, and warnings/errors retain concise wording.

### 3.5 Compact control scale

Introduce shared compact sizing instead of tuning every call site independently.

| Element | Target |
| --- | --- |
| Standard button | 34–36dp high, 10dp horizontal / 4dp vertical content padding |
| Top-bar/toolbar button | 30–32dp high, 8dp horizontal padding |
| Button text | 11sp default; keep long Apply labels readable |
| Single-line field | 44–46dp high with compact internal padding |
| Multiline chat/draft field | Content-driven with a smaller minimum height; never force single-line sizing |
| Panel padding | 8–12dp according to hierarchy |
| Action spacing | 4–6dp between related controls |

Do not shrink source code, diagnostics, keyboard focus indicators, or important status
text. Avoid icon-only replacements for currently labeled actions.

### 3.6 Additional improvements included in the refinement

1. Replace the duplicated long connection strings with one compact colored connection
   state in the project-open shell. Show Reconnect only when disconnected or after a
   connection failure.
2. Remove numeric badges from the workspace rail. Use a small textual status badge only
   when a workspace needs attention, such as a running analysis or failed finding scan.
3. Put related actions in one compact row when width permits: Pause/Cancel,
   Analyze/Refresh, Open/Prepare/Triage, and Apply/Undo. Wrap on narrow layouts rather
   than stacking a full-width button for every action.
4. Simplify Bugs filters to one compact search field plus an optional Filters disclosure.
   Show active Source/Severity/Freshness/Lifecycle filters as concise labeled controls
   only when used.
5. Place Analyze-all limits in one compact row. Keep bounds and validation, but remove
   repeated limit descriptions from the run summary.
6. Shorten Editor stage labels to `Target`, `Draft`, `Verify`, and `Apply`. Keep the
   current/locked state in a concise visible marker and accessibility semantics instead of
   appending `Current`, `Ready`, or `Locked` to every button.
7. Keep one dominant action per panel. Supporting and destructive actions use their
   semantic secondary treatment so priority is visible at a glance.

## 4. Target layouts

### 4.1 No project

```text
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│                        Mini-Orca                             │
│                                                              │
│                    [ Open project ]                          │
│                                                              │
│              concise progress or retry error only            │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

No hidden workspace remains keyboard-active behind this view.

### 4.2 Project open

Retain the current responsive information architecture:

- compact top bar with project name, one connection state, and global project actions;
- text-labeled workspace rail without counts;
- Summary, Analysis, and Bugs as full-width project workspaces;
- Editor-only explorer and contextual panel on wide layouts;
- Editor-only Files and Context drawers below 1000dp;
- read-only source/diff views and the guarded Target → Draft → Verify → Apply flow.

## 5. Implementation design

### 5.1 Presentation helpers first

Add or update small pure helpers for:

- empty-project versus project-open shell mode;
- shortcut availability in each shell mode;
- compact project/workspace/explorer labels;
- finding, analysis, verification, and receipt text without visible revision/hash values;
- action tone selection and status-to-tone mapping.

Keep these helpers free of Compose state so their behavior is easy to test. Do not remove
identity fields from API models or domain state.

### 5.2 Shared UI primitives

Update `DesktopTheme.kt` rather than creating parallel button and field systems:

- extend `FocusFlowButton` with semantic tone and compact/toolbar density;
- add one shared compact single-line text-field wrapper;
- retain ordinary `OutlinedTextField` behavior for multiline declaration/chat editors;
- standardize focus, hover, selected, disabled, and border treatments;
- reuse existing palette colors and introduce no new color unless contrast testing proves
  one is required.

### 5.3 Shell split

Make `DesktopShell` choose between two mutually exclusive branches:

1. `ProjectLanding` when `appState.project == null`;
2. the existing responsive workspace shell when a project exists.

The landing branch receives only the state and callback needed to open a project. Keep
project-only callbacks out of the landing composable so hidden functionality cannot be
accidentally exposed. Gate global shortcuts at the same boundary.

### 5.4 Screen cleanup

Apply the new presentation rules to `DesktopHeader`, `ExplorerPane`,
`ProjectSummaryPane`, `WorkspacePanes`, `EditorWorkspace`, `TargetContextPane`,
`DraftContextPane`, `ReviewEvidencePane`, `SummaryPane`, and `CommandPalette`.

Remove obsolete label helpers, count plumbing, copy branches, tests, and call parameters
made redundant by this change. Do not keep old and new presentations side by side.

## 6. Delivery sequence

| Task | Outcome | Main verification |
| ---: | --- | --- |
| 80 | Add compact semantic button/input primitives and migrate controls without changing workflows | `DesktopThemeTest` and affected pane tests |
| 81 | Add the exclusive no-project landing state and guard every non-project command path | Shell, shortcut, import success/cancel/failure tests |
| 82 | Remove counts/hashes/revisions and redundant explanations while retaining safety feedback | Summary, Analysis, Bugs, Editor, Verify/Apply tests |
| 83 | Consolidate connection/status, filters, stage labels, and responsive action groups | Shell, accessibility, Editor, Bugs, and Analysis tests |
| 84 | Complete responsive, keyboard, contrast, copy, documentation, and regression acceptance | Desktop suite, `make check`, and manual GUI checklist |

Each task should remain Desktop-scoped and leave the client compiling and testable. No
commit, push, or task execution is authorized by this plan alone.

## 7. Verification

Automated coverage should prove:

- no-project mode exposes only Open project and ignores every other shortcut;
- open/cancel/failure/success transitions do not reveal stale workspace content;
- routine display labels contain no project revision, file hash, draft hash, or project
  inventory count;
- internal stale-response, validation, checks, Apply, and Undo identity guards still use
  the unchanged revision/hash values;
- each action tone maps to a readable text-labeled control and state is not color-only;
- compact controls preserve focusability, enabled/disabled semantics, and long-label
  layout;
- 999dp and 1000dp behavior remains correct;
- source and diff remain selectable and read-only;
- remote-provider confirmation remains mandatory before prompt-bearing requests.

Commands:

```text
./desktop/gradlew -p desktop test
git diff --check
make check
```

Run the Desktop test command during each phase. Run `make check` at final acceptance when
the environment permits. Manually inspect the empty landing state plus Summary, Analysis,
Bugs, and every Editor stage at wide and narrow sizes; Compose unit tests alone do not
prove visual density, wrapping, or contrast.

## 8. Safety and compatibility boundaries

- No daemon route, persisted format, API schema, provider configuration, or migration
  changes.
- No removal or weakening of project revision, file hash, draft hash, or request identity
  guards.
- No automatic project import, analysis, scan, generation, Apply, Undo, or file write.
- Remote-provider confirmation remains explicit and visible when relevant.
- Source and composed diffs remain read-only; only the isolated declaration/import draft
  remains editable.
- The existing 1000dp responsive breakpoint, saved pane widths, keyboard workflow, and
  project-relative path presentation remain supported.
- Existing unrelated worktree changes must be preserved. Generated build output and local
  configuration remain untouched.

## 9. Definition of done

1. With no project open, the app shows only product identity, Open project, and contextual
   progress/error feedback; no other panel, button, drawer, palette, or shortcut is
   available.
2. Opening a project transitions to the full workspace without stale empty-state content.
3. Project file counts and project revision SHAs are absent from routine UI.
4. File/draft hashes and other raw identity values are absent from routine UI while all
   identity guards and conflict behavior remain intact.
5. Workspace navigation is free of numeric counters unless a concise attention state is
   genuinely actionable.
6. Buttons visibly distinguish primary, navigation, positive, attention, destructive,
   and neutral purposes using the shared semantic palette and text labels.
7. Redundant explanatory copy below controls is removed; required errors, disabled
   reasons, remote confirmation, and Apply safety information remain.
8. Buttons and single-line inputs use the compact scale consistently without clipping,
   focus loss, or unreadable text.
9. Connection/status presentation is not duplicated, advanced filters are quieter, and
   related actions form compact responsive groups.
10. Desktop tests, `git diff --check`, final supported validation, and the documented
    wide/narrow manual UI checks pass, with any environment-limited check reported.
