# Mini-Orca UI precision plan

**Status:** In progress — Tasks 161–168 are complete; Tasks 169–171 are Pending.
**Prepared:** 2026-09-04.
**Scope:** Desktop presentation, the necessary Jewel build/runtime migration,
desktop tests, and supporting documentation. No daemon/API or workflow expansion.

## 1. Outcome and priorities

Make the existing application read as a dense, deliberately layered desktop IDE,
not a dark dashboard assembled from rounded cards. Preserve the current navigation
and preview-first workflow while correcting the remaining structural UI problems.

The three highest-priority requirements are release gates, not optional polish:

1. **Adopt Jewel as the desktop component foundation**, backed by one semantic
   design-token system. Include the required compatible toolchain/runtime migration.
2. **Separate panes with continuous 1dp dividers**, not thick gutters, rounded
   containers, or indistinguishable backgrounds.
3. **Put compact actions in their owning panel headers**, especially Analysis
   Start/Pause/Resume/Cancel. Remove the separate Run controls card.

The [task index](tasks/INDEX.md) defines the strict 161 → 171 order. The
[execution prompt](tasks/PROMPT_EXECUTE_UI_PRECISION.md) requires exactly one
verified local commit per task when explicitly invoked. Creating these documents
does not execute the tasks or create commits.

## 2. Inspected starting point

The current implementation was inspected in source; this is not a claim of a new
native-window visual review. The supplied [dark mock](design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png)
remains the proportional reference. The user's latest flat-pane direction takes
precedence where the mock uses cards. Its sample files and controls are not features
to implement.

| Existing area | Concrete remaining problem | Replacement |
| --- | --- | --- |
| `DesktopTheme.kt` | `appBackground` and `surface` share a color; `raisedSurface` is still described as a card/panel role; primary body text is 14sp | Named rail/tool-window/editor/overlay roles and dense typography |
| `IdeShell.kt`, `DesktopShell.kt` | Pane boundaries need a coherent surface/separator audit | Edge-to-edge panes, one divider owner per boundary |
| `WorkspacePanes.kt` | `AnalysisRunCard`, Coverage, and a separate Run controls section | Flat progress/coverage rows and one in-header run toolbar |
| `ProjectSummaryPane.kt`, `PerformanceWorkspace.kt` | Remaining padded panel groupings | Compact metrics, lists, and disclosure sections |
| `ContextToolWindow.kt` | 18dp side padding and sections with 20dp vertical padding | Shared dense inspector rhythm and flat disclosure headers |
| `ChromeControls.kt`, `EditorWorkspace.kt` | Custom tabs, string breadcrumbs, and Material-based menu infrastructure | Unified Jewel-backed controls, explicit active tabs, segmented breadcrumbs |
| `DesktopVisualLayoutTest.kt` | Offscreen popup/dialog pixels do not fully represent detached native windows | Retain deterministic component tests; add honest native evidence |

The checked-in build currently uses Kotlin **2.0.21**, Compose **1.7.0**, Gradle
**8.6**, and a JVM **21** toolchain. These are baseline facts, not migration targets.
The previous [component decision](desktop/UI_COMPONENT_DECISION.md) deferred Jewel
only within the narrower Task 154 scope; that restriction does not govern this plan.

## 3. Jewel adoption and dependency gate

Use the standalone library, not the IntelliJ plugin bridge. JetBrains now maintains
Jewel inside `intellij-community`; the old repository is a relocation entry point.
The official standalone setup uses `org.jetbrains.jewel:jewel-int-ui-standalone`
with a release-specific version including its platform build suffix. It requires
aligned Kotlin/Compose dependencies and documents JetBrains Runtime as supported
runtime infrastructure. [Official Jewel documentation](https://github.com/JetBrains/intellij-community/blob/master/platform/jewel/README.md)

Do not copy an old or floating version into the build. In Task 161, select an
actually published stable release and record its complete coordinates, matching
Kotlin/compiler/Compose/Skiko versions, compatible wrapper version, and JBR version
for the existing supported platforms. Check both the
[official release matrix](https://github.com/JetBrains/intellij-community/blob/master/platform/jewel/RELEASE%20NOTES.md)
and published dependency metadata. Current master documentation is not proof that
an artifact is released. Recheck this evidence at execution time.

Task 161 must prove a representative standalone slice: theme, toolbar action, tab,
tree/disclosure, menu, text input, and the existing rendering test harness. Record
resolved dependencies and native launch/runtime requirements. Task 162 then brings
that verified foundation into the production app. A spike is temporary/test-only;
do not retain a parallel application or alternative shell.

Adopt Jewel for standard controls where its API fits. Keep application-owned
source/diff rendering and domain state. Use small wrappers only for genuine shared
policy (density, state semantics, callbacks), not a second component framework.
Map app surface/typography roles into the same theme that styles Jewel. Temporary
Material primitives must consume those same roles, never their default theme;
remove their use as each owning task migrates. By Task 169, remove the obsolete
Material theme and controls unless a specific low-level interop exception is
documented and visually/behaviorally tested. No dual theme or feature flag.

**Fallback rule:** the user's unified-token alternative is available only for a
demonstrated integration blocker (for example, unsupported runtime distribution or
an unfixable accessibility regression). A newer toolchain requirement alone is not
a blocker. Document attempted versions, failing reproduction, and impact; ask the
user before replacing the Jewel adoption path. Never silently call existing
Material styling “Jewel adoption.” Retain native window decorations; custom
titlebars and experimental native-popup flags are not required by this plan.

## 4. Visual specification

### Surface ownership

These are app targets, not a claim that every JetBrains product has identical hex
values. All colors live in the shared semantic palette; no per-screen literals.

| Semantic role | Target | Used by |
| --- | --- | --- |
| Activity rail / outer chrome | `#18191B` | Far-left rail; restrained top/status chrome |
| Tool-window surface | `#1E1F22` | Files, Context/Assistant/Review, Problems/Checks/Output |
| Editor/content canvas | `#2B2D30` | Read-only source/diff canvas; main workspace canvas |
| Pane separator | `#323438`, 1dp | Docked boundaries and section dividers |
| Selection indicator | `#3574F0` | Tab underline and active navigation edge |
| Selected row fill | `#2E436E` | Selected rows; separate from keyboard focus |
| Secondary section heading | Start at `#8A8D93` | 11–12sp section labels; adjust centrally if measured contrast fails |

Source and gutter belong to the editor canvas, not a nested raised card. Tool
windows own their full rectangular background. Section bodies inherit their pane's
surface. Modal dialogs, popup menus, text inputs, and truly isolated draft-editing
boundaries may remain contained; none justify wrapping whole panes in cards.

Every adjoining docked pane has exactly one continuous divider. Draw a 1dp line,
but retain a larger invisible splitter hit area with resize affordance and keyboard
access. Hover/focus may strengthen that line without making it a permanent gutter.
Avoid double borders where a header, pane, and splitter meet. Dialog separation is
not a substitute for pane separation.

### Density and typography

All numbers are logical Compose units (`dp`/`sp`), not forced physical pixels.
At default text scale:

| Element | Target |
| --- | --- |
| Body/list text | 12–13sp, explicit 18–20sp line height |
| Section title | 11–12sp, semibold or short uppercase label, muted text |
| Breadcrumb | 12sp; 14–16dp file/folder/symbol icons |
| Source/diff | Preserve readable monospaced text; target 13sp/20sp initially |
| Tool-window/header row | 28–32dp, 8dp horizontal inset |
| List/tree row | 24–28dp; consistent baseline and indentation |
| Compact toolbar action | 28–32dp target, 16dp icon, 4dp inter-action gap |
| Section content | 8dp normal inset; 4–8dp internal gaps; at most 12dp deliberate section separation |
| Pane corners/elevation | 0dp / none; contained controls may use 4–6dp corners |

These are minimum/default density targets, not fixed heights that clip scaled text.
Grow or reflow at 125% and 150% text scale. Do not shrink fonts to fit. Keep contrast
at least 4.5:1 for normal meaningful text and 3:1 for focus/essential control-state
indicators; subtle structural separators are not the only way to identify focus.
Measure the actual blended/background pairs, including the editor canvas. Preserve
text labels for status and validation errors.

### Shared headers, actions, and disclosures

A normal pane header is a flat row: optional disclosure → icon/title → short
textual state/count → flexible space → compact actions → overflow/collapse.
An action inside the row must not toggle the disclosure or steal its semantics.
Use `expanded` state and keyboard activation on the disclosure control itself.

Analysis has a single header toolbar. Start/Pause/Resume/Cancel use existing
callbacks and enablement rules; only valid actions are active. Keep a textual run
state/progress summary. Put advanced run options in a flat secondary disclosure,
not a replacement boxed Run controls panel. Preserve accessible names and tooltips
for icon-only actions. Use short text toolbar buttons when an icon is ambiguous.
When narrow, move secondary actions into a labeled overflow menu without losing
Cancel or hiding safety messages. Do not invent Pause for a workflow without it.

Apply, Undo, and remote-provider confirmation keep explicit text and their existing
guards. Density must not hide a destructive action behind an unlabeled icon or
remove the evidence a user must review. No duplicate live action bars.

### Tabs, breadcrumbs, and interaction states

Use a consistent active background plus a 2dp accent underline for active editor
and tool-window tabs; visible keyboard focus is a separate outline. Confirm hover,
pressed, selected, disabled, and focus states on all shared controls. Disabled
controls must not dispatch callbacks and must not resemble active selections.

Keep one actual open file and the existing Source/Review surfaces. The illustrative
`server.go`, `routes.go`, and `user.go` tabs do not authorize multi-file editing or
fake tabs. Breadcrumbs show the actual project-relative segments and selected
symbol, with subtle type icons and separators. Only segments with a real supported
navigation callback are interactive. Long paths retain the file/symbol identity
and expose the full path accessibly; use overflow/scrolling without shrinking text.

## 5. Scope and safety invariants

- Preserve Summary, Analysis, Performance, Bugs/Problems, Editor, Files, Context,
  Assistant, Review, Checks, Output, and all real loading/empty/stale/error states.
- Preserve exactly `>= 1000dp` docked and `< 1000dp` drawer/overlay behavior, stored
  pane dimensions, and the current 360dp minimum editor target when docked.
- One project, one file, one symbol; source and diff remain selectable/read-only.
  Only the isolated declaration/import draft is editable.
- Provider requests require existing explicit actions and remote consent. Preserve
  scope, draft identity, validation, current checks, review, Apply receipt, and Undo.
- Unsupported controls remain labeled Preview, local-only, and incapable of source
  writes, provider requests, process execution, or changing workflow eligibility.
- No daemon changes, API/config migration, general source editor, new VCS features,
  docking engine, metrics backend, or new product capabilities.
- Runtime/build changes are restricted to the desktop integration and documented
  launch/package instructions. Do not install or replace system JDKs silently.
- Remove obsolete helpers, styles, and call sites when replacing them; preserve
  user-owned changes and never manually edit generated build output.

## 6. Delivery sequence

Each task file contains its implementation steps, file map, acceptance checks,
focused tests, and exact commit subject. Dependencies form one linear chain.

| Task | Deliverable | Primary requirement |
| ---: | --- | --- |
| 161 | Baseline, component inventory, verified Jewel compatibility spike | Prove adoption feasibility and capture current risks |
| 162 | Production Jewel/runtime integration and semantic theme | One component/theme foundation |
| 163 | Shared dense headers, toolbars, disclosures, typography, and dividers | Reusable precision primitives |
| 164 | Layered shell and continuous pane boundaries | Eliminate surface flatness and heavy gutters |
| 165 | Flat Analysis with header-owned run actions | Remove Run controls/Coverage card layout |
| 166 | Dense Summary and Performance sections | Consistent workspace density |
| 167 | Files tree, active editor tabs, and icon breadcrumbs | Precise navigation/editor chrome |
| 168 | Flat Context, Assistant, and guarded Review | Dense right tool windows without weakened safety |
| 169 | Bottom panes, global menus/dialogs, and migration cleanup | Finish adoption across all remaining surfaces |
| 170 | Responsive, interaction, accessibility, and native visual verification | Prove the result in real UI conditions |
| 171 | Final repository checks, evidence, cleanup, and handoff | Honest acceptance and per-task commit ledger |

## 7. Verification and stop conditions

Every implementation task runs the desktop formatting/static/test suite and
`git diff --check`, plus its focused tests. Task 161 records fresh `make check` and
`make quality` results; Task 171 reruns both. Use the repository wrapper only.

The previous acceptance recorded `make check` passing and `make quality` failing
on five unchanged Go complexity findings. This is history, not a fresh pass or a
standing waiver. Task 161 records exact diagnostics and which later stages did
not run. A new/regressed check blocks its task. If the unchanged out-of-scope Go
baseline remains, earlier UI tasks may proceed, but Task 171 cannot claim the
quality gate passed or mark full acceptance Complete without user direction on
that exception. Do not change Go or weaken thresholds to make a UI task green.

Maintain deterministic production-component tests with fixture-only data. Exercise
all changed callbacks and important disabled/stale/error states; assert real
behavior, dimensions, text visibility, and semantics rather than source-string or
screenshot-hash snapshots. Capture a small, stable comparison set at 1440×900 and
1920×1080, both sides of the 1000dp boundary, 800×650, and a short 1280×600 window.
Check 1×/2× display density and 100%/125%/150% text scale where supported.

Native acceptance includes actual popup/dialog placement, window-edge clipping,
OS key navigation, focus restoration, source/diff selection, splitter interaction,
and a supported screen-reader smoke test. Detached popup semantics or an inline
menu fixture are not native pixel evidence. Use disposable fixtures without real
provider calls or edits to an external user project.

Task 170 remains In Progress if material native evidence is unavailable; report
the precise missing check and request operator help. Do not substitute an HTML
mock or label offscreen screenshots as native. Earlier tasks may accurately report
component-only validation. Required failing/missing acceptance is never silently
waived, and Task 171 must not run past an incomplete Task 170.

## 8. Documentation cleanup and retained history

This planning update removes **64 tracked completed planning documents**: 57
completed task files (103–148 and 150–160), the completed IDE/refinement/insights
plans, and their four completed execution prompts. The old lowercase `plan.md`
is replaced by this canonical `Plan.md`, not kept as a second plan. The removed
files remain recoverable from Git history; no source, design asset, or evidence
record is deleted. Tasks 01–102 had already been consolidated historically.

Keep `tasks/149_dark_ui_acceptance.md` Pending, plus its dark-UI plan/prompt and
acceptance evidence: that sequence is not fully accepted. Those documents are
historical acceptance context, not the active visual direction. Task 170 should
reuse relevant evidence but must not silently mark Task 149 complete. Keep the
design guidelines, component decision, contrast, baseline, visual/keyboard review,
and acceptance records because they describe contracts or unresolved checks.

Task 161 owns these reviewed planning/cleanup changes if they are still uncommitted
when execution starts. The execution prompt defines the precise bootstrap scope.
Do not make an extra planning commit or include unrelated user changes. Future
task files stay at stable paths with their status updated; Git remains the archive.
