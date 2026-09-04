# Mini-Orca IDE-style desktop UI improvement plan

**Status:** Complete — automated implementation and acceptance are complete through Task 132.

**Successor plan:** [Dark desktop redesign](docs/dark-ui/PLAN.md), Tasks 140–149,
uses the new supplied mockups. It supersedes this completed plan's palette-preservation
rule and allows explicitly labeled UI previews for unsupported controls. The workflow
and source-mutation constraints below remain relevant.

**Release-operator limitation:** This environment has no interactive Mini-Orca window or
provider-backed fixture. The remaining live viewport, screen-reader, text-scaling, and
provider-flow checks are recorded in `desktop/KEYBOARD_SMOKE_CHECKLIST.md`; they are not claimed
as completed here.

**Prepared:** 2026-09-04

**Primary scope:** `desktop/` (Kotlin/Compose Desktop)

**Theme direction:** Keep the existing Focus Flow palette and dark visual identity

## 1. Outcome

Evolve Mini-Orca from a set of application workspaces into a compact, source-first desktop environment that feels familiar to GoLand and other JetBrains IDE users.

This is an information-architecture and interaction redesign, not an attempt to reproduce the full feature set of an IDE. The result should make the current Mini-Orca workflow faster to understand and operate:

1. open one project;
2. find and inspect one file;
3. select one declaration;
4. discuss or prepare a scoped change;
5. edit and validate the generated draft;
6. review the diff and evidence;
7. explicitly apply, reject, or undo the change.

The central source view should remain visually stable while supporting information appears in predictable left, right, and bottom tool windows. The existing colors remain the product identity; layout, hierarchy, density, navigation, and state visibility are the main areas of change.

## 2. Product constraints

These requirements are non-negotiable throughout the redesign:

- Source and diff views remain read-only and selectable.
- Generation never writes project files.
- Apply and Undo remain explicit user actions protected by the existing validation, check, revision, hash, and identity guards.
- The active task remains scoped to one project, one file, and one symbol or new declaration.
- The UI must not imply that several files will be edited together.
- Project-relative paths are shown; unsafe arbitrary filesystem traversal is not introduced.
- The daemon remains loopback-only by default.
- Provider destination and remote-context confirmation remain visible in text, not only color or iconography.
- Existing keyboard access and the responsive drawer behavior below `1000dp` are preserved and improved.
- No automatic commits, terminal execution, run/debug system, general file editor, or VCS mutation is added as part of this plan.

## 3. Current UI baseline

The current desktop already provides useful pieces that should be retained:

- a coherent dark palette in `DesktopTheme.kt`;
- project landing and automatic project restoration;
- Summary, Analysis, Bugs, and Editor workspaces;
- an indexed project explorer with path filtering and persisted pane widths;
- a read-only source view with line numbers, lightweight syntax highlighting, and declaration selection;
- a symbol/file inspector;
- a bound AI conversation and editable declaration draft;
- read-only unified and side-by-side diffs;
- validation, focused checks, Apply, receipt, and Undo evidence;
- command dialogs for files, symbols, and contextual actions;
- text-labeled connection, progress, error, and provider states;
- wide docked panes and narrow Editor drawers at the existing `1000dp` breakpoint.

### Current friction to address

| Area | Current behavior | Improvement goal |
| --- | --- | --- |
| Global navigation | A permanent `176dp` workspace rail competes with the Explorer and source | Use a compact tool-window bar and open only the panel needed for the current task |
| Main toolbar | Project, connection, drawer, command, re-index, reconnect, and open actions share one row | Keep frequent context visible; move infrequent project actions into a clear project menu or overflow |
| Center canvas | A file header sits above either source or review with no editor tab model | Add a stable editor tab/header, breadcrumbs, read-only state, and explicit Source/Review surfaces |
| Right pane | File context, conversation, draft, and review replace one another based on workflow state | Use named tool-window tabs so location and purpose stay predictable |
| Problems and checks | Project findings are a separate workspace; focused checks live in Review | Introduce a bottom tool window for Problems, Checks, and activity output |
| Status | The bottom bar exists only while loading or on error | Make status persistent and useful without becoming noisy |
| Commands | Files, symbols, and actions use separate modal modes | Present one consistent search/command surface while retaining focused shortcuts |
| Density | Large cards and repeated headings consume editor space | Use IDE-like compact headers, rows, separators, and toolbars while keeping readable targets |
| Responsiveness | The current narrow behavior is Editor-specific | Apply the same tool-window model consistently while preserving `<1000dp` drawers |
| Discoverability | Color and labels are good, but relationships between selection, draft, checks, and apply are distributed | Keep the task scope and workflow gate visible beside the editor at all times |

This baseline is based on the current Compose implementation. Before implementation begins, capture reference screenshots at representative sizes so the visual change can be evaluated rather than inferred from code alone.

## 4. Design principles

### 4.1 Source-first

The source or validated diff is the primary surface after a file is selected. Supporting information should not repeatedly replace the center canvas unless the user intentionally opens a project-level view.

### 4.2 Stable spatial model

- Left: project navigation.
- Center: active file source or candidate review.
- Right: file/symbol context and scoped assistant workflow.
- Bottom: problems, checks, and operation output.
- Top: project identity, navigation/search, and essential actions.
- Bottom edge: persistent status.

Each responsibility should have a predictable home.

### 4.3 Progressive disclosure

Default to a clean editor view. Detailed filters, provider information, impact context, check output, and infrequent project actions remain one click or shortcut away. Hidden content must still expose an obvious text label, accessible name, and state summary.

### 4.4 Compact, not cramped

Adopt IDE density through shorter toolbars, list rows, and headings. Keep minimum interactive hit areas, visible focus treatment, text scaling support, and scrolling for long content.

### 4.5 One dominant action per stage

The right tool window should make the next safe action evident:

- no target: choose a file or declaration;
- target selected: describe or start an edit;
- draft generated: edit and validate;
- valid draft: run checks and review;
- current passing evidence: Apply;
- applied receipt: inspect or Undo.

Secondary actions must not visually compete with the current guarded transition.

### 4.6 Familiar patterns without false promises

Use JetBrains-style concepts such as tool windows, an editor header, breadcrumbs, a gutter, contextual actions, search, and a status bar. Do not add controls that suggest unsupported code editing, multi-file refactoring, run/debug, terminal, or VCS write features.

Reference patterns:

- [GoLand user interface](https://www.jetbrains.com/help/go/ui-reference.html)
- [GoLand new UI](https://www.jetbrains.com/help/go/new-ui.html)
- [GoLand tool windows](https://www.jetbrains.com/help/go/tool-windows.html)
- [GoLand editor basics](https://www.jetbrains.com/help/go/using-code-editor.html)
- [GoLand Project tool window](https://www.jetbrains.com/help/go/project-tool-window.html)
- [GoLand keyboard shortcuts](https://www.jetbrains.com/help/go/mastering-keyboard-shortcuts.html)

These are interaction references, not a requirement to copy JetBrains assets or appearance.

## 5. Target desktop shell

### 5.1 Wide layout (`>= 1000dp`)

```text
+--------------------------------------------------------------------------------------+
| Mini-Orca | project ▾ | path breadcrumbs        | Search/Commands | status | actions |
+----+----------------------+----------------------------------+------------------------+
| P  | PROJECT              | active.go   READ-ONLY             | CONTEXT | ASSISTANT   |
| S  | Filter files…        | src / pkg / active.go / Symbol    |                        |
| A  | ▾ cmd                +----------------------------------+ selected declaration   |
| B  |   ▸ daemon           |  12  func active() {              | explanation            |
|    |   active.go          |  13      ...                       | scope / provider        |
|    |   other.go           |  14  }                             | draft / next action     |
|    |                      |                                    |                        |
|    | resizable tool pane  | source-first editor canvas         | resizable tool pane     |
+----+----------------------+----------------------------------+------------------------+
| Problems  Checks  Output  | current gate summary / selected item                    |
+--------------------------------------------------------------------------------------+
| project indexed | Go | Ln 13 | analysis fresh | local provider | daemon connected     |
+--------------------------------------------------------------------------------------+
```

Tool-window bar entries:

- **Project** — indexed file tree and filter.
- **Summary** — deterministic project facts and advisory model interpretation.
- **Analysis** — project-wide analysis controls and failures.
- **Problems** — verified findings and AI suggestions, with counts expressed in text.

The bar may use compact glyphs, but the active tool window always has a visible text title and every glyph has a tooltip and accessible name.

### 5.2 Narrow layout (`< 1000dp`)

```text
+--------------------------------------------------------------+
| Mini-Orca | project/path… | Search | Project | Context | ⋯   |
+--------------------------------------------------------------+
| active.go · READ-ONLY                                       |
| src / pkg / active.go / Symbol                              |
+--------------------------------------------------------------+
|                                                              |
|                   source or diff canvas                      |
|                                                              |
+--------------------------------------------------------------+
| Problems / Checks summary                         Open panel  |
+--------------------------------------------------------------+
| Ln 13 | Fresh | Connected                                    |
+--------------------------------------------------------------+
```

- Project and right-side tool windows open as modal drawers, preserving the current responsive behavior.
- The bottom tool window becomes a compact summary row and expands as an overlay or drawer.
- Long project paths collapse from the middle and remain available in a tooltip/accessible description.
- Infrequent toolbar actions move into an overflow menu before labels or state text are removed.

### 5.3 Center editor behavior

The center region contains one active file surface in the first release.

- Show a single active-file tab with file name, modified-draft marker, and read-only label.
- Keep the project-relative path in breadcrumbs or a secondary line.
- Do not imply multi-file editing. Multiple open-file tabs are explicitly out of scope.
- Provide two contextual surfaces when a current validated draft exists:
  - **Source** — current project source;
  - **Review** — composed candidate diff.
- Never switch from Source to Review merely because background work finishes. Surface a clear “Review candidate” action and preserve the current selection.
- Keep line numbers in a dedicated gutter.
- Use gutter markers for selected declaration, focused line, and known findings. Markers must have accessible descriptions and must not trigger source mutation.
- Keep source selection separate from a click that selects declaration context.
- Preserve horizontal scrolling for long source lines and ensure the file path/header does not scroll away.

### 5.4 Left Project tool window

- Replace the card-like Project Explorer with a flat, dense tool-window layout.
- Add a compact header with title, collapse-all, and filter/search.
- Keep directories first, file language/freshness indicators, and selected-file synchronization.
- Use a full-row selection background and keyboard traversal for tree nodes.
- Keep folder expansion local and persisted for the current project where practical.
- Add “Select active file” behavior so the tree can return to the open file after filtering or navigation.
- Do not add create, rename, move, or delete actions; the indexed explorer remains navigation-only.

### 5.5 Right Context and Assistant tool window

Use explicit tabs instead of replacing unrelated panels without explanation:

1. **Context**
   - file metadata when no symbol is selected;
   - declaration signature, range, confidence, and explanation when selected;
   - file analysis action and provider confirmation;
   - read-only impact and Git context where relevant.
2. **Assistant**
   - bound target summary pinned at the top;
   - conversation history;
   - request composer and Generate/Cancel action;
   - generated declaration/import editor;
   - validation diagnostics and current draft state.
3. **Review**
   - scope identity;
   - validation and focused-check evidence;
   - exact Apply action;
   - receipt and Undo after application.

When workflow progress changes, update the relevant tab badge and summary. Do not unexpectedly steal focus or open a panel while the user is reading source.

### 5.6 Bottom tool window

Introduce one resizable/collapsible bottom area with:

- **Problems** — the filtered project findings list, grouped by priority and retaining provenance/status text;
- **Checks** — validation diagnostics and focused check status/output for the active draft;
- **Output** — current and last analysis/generation operation status with sanitized error text.

Rules:

- Selecting a problem opens its indexed file and focuses its line in the center editor.
- “Prepare fix” remains a distinct explicit action; selecting a problem alone never starts generation.
- Failed required checks bring the Checks tab to attention but do not automatically apply or regenerate.
- Project-wide Analysis can retain a dedicated central view for detailed controls; its failures also appear in Output.
- The bottom area remembers height and collapsed state.

### 5.7 Persistent status bar

Replace the error/loading-only bar with a persistent, compact status bar. Show only data the application genuinely knows:

- current operation or last concise event message;
- active project/index state;
- selected file language;
- focused source line, when present;
- file-analysis freshness;
- configured provider destination and whether it is local or remote;
- daemon connection state;
- error indicator with text access to details.

Status items are focusable when actionable. Color reinforces labels but never replaces them.

### 5.8 Search and commands

Keep the existing focused shortcuts and present a consistent search experience:

- `Cmd/Ctrl+P` — files;
- `Cmd/Ctrl+Shift+O` — symbols in the active file;
- `Cmd/Ctrl+K` — scoped Mini-Orca actions;
- `Cmd/Ctrl+1` through `4` — preserve current Summary, Analysis, Bugs, and Editor destinations during migration;
- `Escape` — close the topmost transient surface or cancel the active cancellable operation;
- add documented tool-window focus shortcuts only when they do not conflict with existing mappings.

The dialogs should share one reusable shell: search field, keyboard-highlighted result list, result type, shortcut hint, empty state, and `Enter`/arrow-key behavior. Results must be constrained to indexed files, current-file symbols, and actions valid for the current state.

## 6. Visual system: preserve the current theme

Do not introduce a new palette. Retain the existing values and apply them more consistently:

| Semantic role | Current value | IDE-style use |
| --- | --- | --- |
| App background | `#080917` | Editor canvas and outer shell |
| Surface | `#101225` | Tool windows, toolbar, status bar |
| Raised surface | `#171A31` | Popup, selected groups, empty states |
| Strong surface | `#222640` | Active row, active tab, hover/pressed emphasis |
| Border | `#292D49` | One-pixel separators and pane boundaries |
| Primary text | `#F2F2FB` | Active labels and source text |
| Secondary text | `#9297B6` | Metadata, inactive labels, descriptions |
| Faint text | `#5F6485` | Disabled or low-emphasis detail |
| Violet accent | `#9B8CFF` | Primary workflow action and current stage |
| Cyan accent | `#62D8EF` | Navigation, selection, focus, links |
| Mint | `#55DDB0` | Passed/current/safe transition |
| Amber | `#FFC86E` | Attention, remote confirmation, stale state, Undo |
| Rose | `#FF7F9F` | Error, failed check, destructive cancellation |

Keep the existing code token colors. Establish shared tokens for:

- toolbar and status-bar heights;
- tool-window header height;
- compact row height;
- editor tab height;
- divider thickness and pointer hit area;
- focus-ring width;
- corner radius by component type;
- UI, code, caption, and status typography;
- content spacing at compact and standard density.

Avoid a “card inside card” appearance. Cards remain useful for landing, receipts, and important empty/error states; navigation and routine metadata should use flat rows separated by space or subtle borders.

## 7. Interaction and accessibility requirements

- Every action has a visible text label or a discoverable tooltip plus accessible name.
- Every state—selected, running, passed, failed, stale, remote, disabled—has a textual representation.
- Keyboard focus is visible with the cyan accent and at least a two-pixel equivalent outline where the component allows it.
- Tool windows, tree rows, editor surfaces, tab headers, bottom tabs, toolbar, and status widgets participate in a predictable focus order.
- Arrow keys navigate within trees, tabs, and result lists; `Tab` moves between groups.
- `Escape` returns focus to the editor after closing a tool window or popup where appropriate.
- Opening a file or problem announces the project-relative path and focused line through semantics.
- Do not use hover as the only way to reveal an essential action.
- Retain readable contrast using the current palette; verify disabled text, borders, diff rows, and tinted status backgrounds specifically.
- Verify supported text scaling with long paths, symbols, diagnostics, and provider names.
- Animations are short and functional, and reduced-motion behavior avoids large sliding transitions.

## 8. Implementation phases

Each phase should be delivered as a narrow, reviewable change with its own tests. Avoid a single shell rewrite.

### Phase 0 — Baseline and UI contract

**Goal:** Make the redesign measurable and protect existing product behavior.

- [ ] Capture screenshots at approximately `1440x900`, `1100x760`, and a width below `1000dp` for landing, source, drafting, review, Analysis, and Bugs.
- [ ] Record current keyboard paths and focus behavior using `desktop/KEYBOARD_SMOKE_CHECKLIST.md`.
- [ ] Add a short desktop UI contract covering source read-only behavior, preview-before-apply, one-file scope, provider confirmation, and narrow drawers.
- [ ] Identify current component sizes and overflow failures at supported text scaling.
- [ ] Define a fixture state that exposes long paths, duplicate basenames, nested symbols, findings, a draft, failed checks, and a receipt.

**Done when:** Baseline screenshots and an updated smoke checklist can distinguish regressions from intended visual changes.

### Phase 1 — IDE shell foundation

**Goal:** Create the stable frame without changing feature behavior.

- [ ] Extend `DesktopTheme.kt` with semantic layout, typography, divider, focus, and density tokens while preserving all palette values.
- [ ] Split `DesktopShell.kt` into cohesive shell components: main toolbar, tool-window bar, docked pane, editor area, bottom tool window, status bar, and narrow drawers.
- [ ] Introduce a small immutable `DesktopLayoutState` for open tool windows, active tabs, pane sizes, and collapsed state.
- [ ] Replace `PaneWidthStore` with or extend it into a layout store that persists only presentation preferences.
- [ ] Keep workflow/domain state in `DesktopState`; layout state must not own network effects or apply rules.
- [ ] Make dividers visually thin but retain a comfortable invisible drag target and keyboard resize alternative.

**Likely files:**

- `DesktopTheme.kt`
- `DesktopShell.kt`
- new focused files such as `IdeShell.kt`, `ToolWindow.kt`, `DesktopStatusBar.kt`, and `DesktopLayoutState.kt`
- `DesktopShellTest.kt`
- new layout-state tests

**Done when:** Existing screens render inside the new frame with unchanged actions, shortcuts, and daemon calls.

### Phase 2 — Project navigation and editor chrome

**Goal:** Make file discovery and source reading feel like an IDE.

- [ ] Convert `ExplorerPane` from a padded card to a compact Project tool window.
- [ ] Add keyboard tree traversal, collapse-all, active-file synchronization, and a compact filter.
- [ ] Add the single active-file tab, project-relative breadcrumbs, read-only label, and language/freshness summary.
- [ ] Keep selected symbol/focused line stable when toggling surrounding tool windows.
- [ ] Improve source viewport performance for larger files without breaking selectable text.
- [ ] Separate the gutter visually from source content and add non-mutating selection/finding markers.
- [ ] Preserve exact nested-declaration selection and drag-to-select behavior.

**Likely files:**

- `ExplorerPane.kt`
- `EditorWorkspace.kt`
- `SourceEditorPane.kt`
- `EditorInspectionState.kt`
- `DesktopAccessibility.kt`
- their existing tests plus focused tree/editor presentation tests

**Done when:** A user can open a file, recognize its path and state, select a declaration, and navigate entirely by keyboard without losing source context.

### Phase 3 — Context, Assistant, and Review tool windows

**Goal:** Give the file-scoped workflow a stable right-side home.

- [ ] Place `SymbolInspectorPane`, `DraftContextPane`, and `ReviewContextPane` behind explicit Context, Assistant, and Review tabs.
- [ ] Pin the current project-relative file and symbol/new-declaration target at the top of Assistant and Review.
- [ ] Add textual tab badges for relevant state: Draft, Invalid, Checks failed, Ready to apply, Applied.
- [ ] Keep draft editing, validation diagnostics, check state, and exact Apply wording unchanged in meaning.
- [ ] Replace automatic context switching with clear stage actions and non-focus-stealing badges.
- [ ] Keep remote provider confirmation adjacent to the first prompt-bearing action.
- [ ] Ensure changing file or target still requires the existing explicit draft-discard decision.

**Likely files:**

- `SymbolInspectorPane.kt`
- `DraftContextPane.kt`
- `ReviewEvidencePane.kt`
- `EditorContextualActions.kt`
- `DesktopApp.kt`
- workflow and review presentation tests

**Done when:** The current scope and next guarded action remain visible and understandable at every stage, with no new source mutation path.

### Phase 4 — Problems, checks, and operation output

**Goal:** Move supporting evidence into an IDE-style bottom area.

- [ ] Add bottom tabs for Problems, Checks, and Output.
- [ ] Reuse existing findings grouping, filtering, lifecycle, validation, check, and progress presentation logic rather than duplicating it.
- [ ] Make problem selection navigate to source while keeping “Prepare fix” explicit.
- [ ] Show failed analysis files and sanitized operation errors in Output.
- [ ] Provide collapsed summaries such as “3 problems” or “2 checks failed” with visible severity/status text.
- [ ] Keep the detailed project-wide Analysis view for job configuration and controls.
- [ ] Persist bottom-pane height and collapsed state.

**Likely files:**

- `WorkspacePanes.kt`
- `BugsWorkspaceState.kt`
- `AnalysisWorkspaceState.kt`
- `ReviewEvidencePane.kt`
- new `BottomToolWindow.kt`
- related presentation and shell tests

**Done when:** Problems and check failures are available without abandoning the source canvas, and opening them never starts or applies a change.

### Phase 5 — Toolbar, commands, and persistent status

**Goal:** Reduce toolbar noise and improve keyboard-driven navigation.

- [ ] Replace the current wide workspace rail with the compact tool-window bar.
- [ ] Keep project name/path, connection state, search/commands, and current operation visible in the main frame.
- [ ] Move Open project, Re-index, Reconnect, and other infrequent project operations into a project widget or overflow while keeping their text labels.
- [ ] Refactor the three command dialog modes onto one searchable list shell with keyboard selection.
- [ ] Implement the persistent status bar using only existing trusted state.
- [ ] Retain current shortcuts and document any additive JetBrains-inspired tool-window shortcuts.
- [ ] Restore focus to the prior editor/tool-window component after a dialog closes.

**Likely files:**

- `DesktopHeader.kt`
- `CommandPalette.kt`
- `DesktopAccessibility.kt`
- `DesktopShell.kt` or the new shell files
- `CommandPaletteTest.kt`
- `DesktopAccessibilityTest.kt`
- `DesktopShellTest.kt`

**Done when:** Common navigation is faster, the toolbar fits at supported widths, and background/error state is always discoverable without a modal.

### Phase 6 — Project views and visual polish

**Goal:** Make Summary, Analysis, and Bugs consistent with the new shell.

- [ ] Restyle summary content with dense key/value sections and restrained cards.
- [ ] Present Analysis controls in a compact toolbar and progress table/list.
- [ ] Present Bugs/Problems as dense rows with a details area rather than large repeated cards where practical.
- [ ] Standardize empty, loading, stale, failure, and receipt states.
- [ ] Standardize tooltips, hover, pressed, selected, disabled, and focus states.
- [ ] Add short functional transitions for pane open/close and progress changes without delaying work.
- [ ] Remove obsolete shell/workspace UI code in the same changes that replace it.

**Done when:** All retained views share the same hierarchy, density, and interaction language without changing business behavior.

### Phase 7 — Validation and rollout

**Goal:** Verify the redesign across workflows, window sizes, and failure modes.

- [ ] Run desktop formatting/static checks and all desktop tests.
- [ ] Complete the expanded keyboard smoke checklist on macOS and one non-macOS target when available.
- [ ] Repeat the baseline screenshot matrix and compare layout, clipping, state text, and focus.
- [ ] Test with long paths, duplicate file names, large source files, nested declarations, no analysis, stale analysis, remote providers, failed checks, daemon disconnection, and Apply/Undo receipts.
- [ ] Verify no daemon route or payload changed. If an API change becomes necessary, plan and test it separately.
- [ ] Update `desktop/README.md` and the smoke checklist with the final tool-window model and shortcuts.
- [ ] Remove transitional flags and old components after acceptance; do not retain parallel shells.

**Required final checks:**

```bash
./desktop/gradlew -p desktop spotlessCheck detekt test
./desktop/gradlew -p desktop test
```

Run `make check` before release if any shared or daemon code changes despite the intended desktop-only scope.

## 9. State and component architecture

Keep layout preferences separate from workflow truth.

```text
DesktopWorkflowPresenter
  -> DesktopState                       authoritative product/workflow state
  -> DesktopShellState                  presentation assembled for the shell

DesktopLayoutState                     local UI preference only
  -> left tool window + width
  -> right tool window/tab + width
  -> bottom tool window/tab + height
  -> compact/collapsed state
  -> last focused region

IDE shell composables
  -> MainToolbar
  -> ToolWindowBar / ToolWindow
  -> ProjectToolWindow
  -> EditorArea / ActiveFileHeader / EditorBreadcrumbs
  -> ContextAssistantToolWindow
  -> BottomToolWindow
  -> StatusBar
```

Architecture rules:

- Pure functions derive visible labels, badges, action availability, and layout modes.
- Composables render immutable state and emit named intents.
- Network calls and stale-response rejection remain in the presenter/controller boundary.
- Apply eligibility continues to use current daemon-backed evidence; UI badges are not authorization.
- Persist dimensions and visibility, never draft content, provider confirmation, apply eligibility, or workflow progress in the layout store.
- Reuse existing feature state and reducers before introducing new types.
- Split files by cohesive UI responsibility, not one file per small composable.

## 10. Testing strategy

### Unit and presentation tests

Cover pure behavior for:

- wide versus narrow tool-window layout at, below, and above `1000dp`;
- width/height clamping and persistence;
- active tool-window and tab selection;
- toolbar overflow decisions;
- breadcrumb truncation and accessible full-path descriptions;
- active-file/focused-line synchronization;
- command result ordering and keyboard selection;
- status-bar presentations for idle, busy, remote, disconnected, stale, failed, and ready states;
- Problems and Checks collapsed summaries;
- source/review switching without automatic focus changes;
- layout transitions that must not alter draft identity or apply eligibility.

### Compose interaction tests where practical

Cover:

- focus order and visible focus state;
- tree expansion and keyboard navigation;
- opening/closing docked and drawer tool windows;
- search result selection with arrow keys and `Enter`;
- tool-window tab state labels;
- problem-to-source navigation;
- no source mutation from clicks, text selection, or problem navigation.

### Manual acceptance

Extend `desktop/KEYBOARD_SMOKE_CHECKLIST.md` rather than creating a separate checklist. Validate:

- the full project/file/symbol/draft/review/apply/undo flow;
- local and remote provider flows;
- responsive widths and supported text scaling;
- long and pathological content;
- loading, cancellation, stale evidence, daemon failure, and retry;
- mouse-only, keyboard-only, and mixed interaction;
- screen-reader semantics where supported by Compose Desktop.

## 11. Delivery slices

Recommended review order:

| Slice | Content | Risk |
| --- | --- | --- |
| 1 | Tokens, layout state, shell frame, persistent status foundation | Medium |
| 2 | Project tool window and active-file editor chrome | Medium |
| 3 | Context/Assistant/Review right tool window | High; touches guarded workflow presentation |
| 4 | Problems/Checks/Output bottom tool window | Medium |
| 5 | Toolbar and unified command/search interaction | Medium |
| 6 | Summary/Analysis/Bugs density pass, accessibility, docs, cleanup | Low to medium |

Every slice must leave the application usable. Do not merge a shell that depends on later slices to restore essential actions or state visibility.

## 12. Success criteria

The redesign is complete when all of the following are true:

- The active source/diff remains the visual center of the file-scoped workflow.
- Project, Context/Assistant/Review, and Problems/Checks/Output each have a stable location.
- A user can reach an indexed file in two interactions or one shortcut from any project screen.
- The active project-relative file path and selected declaration/task scope are never ambiguous.
- The next guarded workflow action is visible without searching through unrelated panels.
- Source and diff remain read-only, and selecting a finding or symbol never starts generation or applies a change.
- Apply is unavailable until the existing current validation, focused checks, and identity guards pass.
- All running, stale, failed, remote, disabled, and selected states are understandable without relying on color.
- The UI works at `1000dp`, transitions to drawers below `1000dp`, and does not clip essential controls at supported text scaling.
- Existing shortcuts continue to work; new tool-window navigation is keyboard accessible.
- Pane sizes and visibility preferences persist without persisting workflow authority.
- The existing Focus Flow palette remains recognizable and no second theme is introduced.
- Desktop tests, static checks, the keyboard smoke checklist, and the screenshot matrix pass.

## 13. Explicit non-goals

- Building a general-purpose code editor.
- Multiple simultaneously editable files or multi-file generated changes.
- Run/debug configurations, terminal, debugger, or build console execution.
- File creation, rename, move, or deletion in the Project tool window.
- Automatic quick fixes from gutter markers.
- Automatic model calls when a problem, file, or declaration is selected.
- Automatic Apply, Undo, commit, branch, or push operations.
- Replacing the current color theme or adding a theme marketplace.
- Copying JetBrains proprietary icons, assets, or exact trade dress.
- Rewriting the daemon or changing the HTTP contract solely for visual parity.

## 14. First implementation checkpoint

Before production code is changed, approve these four decisions using a lightweight shell mockup built from fixture data:

1. compact tool-window bar versus the current full-width workspace rail;
2. Source/Review surfaces in the center and Context/Assistant/Review tabs on the right;
3. Problems/Checks/Output as a bottom tool window;
4. which project and connection actions remain directly visible in the top toolbar at widths near `1000dp`.

That checkpoint should validate hierarchy and workflow placement using the existing palette. It should not connect to the daemon or become a second UI implementation that must be maintained.
