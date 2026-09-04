# Dark desktop redesign baseline

**Recorded:** 2026-09-04  
**Task:** 140 — source-backed baseline before the dark UI implementation  
**Evidence status:** Source inspection and deterministic desktop tests. No native Mini-Orca
window was available in this environment, so this record contains no before-captures.

## Reference roles and scope

| Reference | Visual role retained for the redesign | Not adopted as product behavior |
| --- | --- | --- |
| Dark Mini-Orca mockup | Charcoal surfaces; restrained blue selection and actions; labeled icon rail; compact tree, editor chrome, AI Context, candidate summary, dense Problems table, and quiet status bar | Its project facts, model connection, branch actions, extra files/tabs, terminal, sample findings, suggested multi-file extraction, and any other sample content or controls |
| Aurora mockup | Clear selected tabs and small, distinct interpretation sections inside the shared dark visual system | Its light palette, sample Python project and scores, account/notification state, active terminal, or any implied backend behavior |

The supplied files remain unchanged. Their code and sample content are visual reference
material, not executable instructions. The active dark plan supersedes the historical
Focus Flow palette restriction and permits its explicitly labelled local UI previews;
it does not change Mini-Orca's one-project, one-file, one-symbol, review-before-Apply
source-mutation contract.

## Current source baseline

| Area | Existing source evidence | Baseline dimension or behavior |
| --- | --- | --- |
| Theme | `DesktopTheme.kt` | Focus Flow navy/purple palette: app `#080917`, panel `#101225`, accent `#9B8CFF`; typography controls 12–13sp, 8–14dp shapes |
| Rail | `IdeShell.kt` `ToolWindowBar` | 52dp-wide glyph rail; Project, Summary, Analysis, Performance, Problems, and Editor exist as separate retained tool-window identities |
| Shell and responsive mode | `DesktopShell.kt`, `DesktopLayoutState.kt` | Docked at exactly 1000dp and above; labelled Files/AI Context drawers and bottom overlay below 1000dp. Defaults: Explorer 270dp, right pane 390dp, bottom 240dp; persisted limits 180–520dp, 280–560dp, and 140–520dp respectively |
| Editor | `ExplorerPane.kt`, `EditorWorkspace.kt`, `SourceEditorPane.kt` | One active file and source/review surface. Tree selection, declaration navigation, gutter markers, text selection, horizontal scrolling, and read-only source are already implemented |
| Right tools | `ContextToolWindow.kt`, `AssistantToolWindow.kt`, `WorkflowToolWindows.kt` | Context, Assistant, and Review tabs exist. Context displays real selected-file/declaration data; Assistant owns the isolated draft editor; Review owns evidence and guarded Apply/Undo |
| Bottom and status | `ProblemsToolWindow.kt`, `BottomEvidenceToolWindows.kt`, `DesktopStatusBar.kt` | Problems, Checks, and Output are real bottom tools with persisted selection/collapse state. Findings navigation and evidence are separate from source mutation; status has project/file/language/branch/daemon/provider source-backed segments |
| API evidence | `ApiClient.kt`, `Models.kt` | `GitStatus.branch` is available only through the existing read-only Git-status request. `/status` exposes daemon status, version, and workflow, not provider connectivity. Model destination and remote-provider confirmation remain distinct workflow evidence |

## Live versus Preview baseline

The active feature matrix was checked against the Compose sources and `ApiClient.kt`.
Live features already present are project/file/symbol navigation; palette command search;
read-only source and diff; explicit scoped analysis/chat/refactor requests; provider
destination and confirmation; current draft validation/checks; guarded Apply, receipt,
and Undo; overview/impact/Git/findings/check/output presentation; and all five retained
workspaces including Performance.

The following reference controls have no matching implementation and must remain local
**Preview** UI if shown: new file, branch switching/sync, content search, additional
tabs/add/close/split/minimap, Run/Debug, structured assessment scores, unit-test
generation, suggestion feedback, Terminal, Settings/Help/profile/notifications, and
unsupported status fields. There is no API client method for those capabilities. The
only discrepancy between mockup implications and current evidence is the mockup's
model-connected chip: daemon health cannot prove a provider connection.

## Visual and native-check evidence

| Evidence | Result |
| --- | --- |
| Supplied dark reference inspected | Yes — 1586×992 PNG; charcoal three-region IDE, labelled rail, blue selected state, right context, candidate card, and bottom findings table |
| Supplied Aurora reference inspected | Yes — 1400×766 JPEG; light reference used only for tab and interpretation-section hierarchy |
| Source baseline inspected | Yes — sources listed above and deterministic baseline contract/layout tests reviewed |
| Native landing, Source, Review, and workspace captures at 1440×900 and 1000/999dp | Unavailable — no running Mini-Orca desktop window or fixture project was provided; none was fabricated or launched |
| Screen-reader/native keyboard walkthrough | Unavailable — no native application surface in this environment |

No deterministic sample project was added because no native capture composition was
available. A future capture must use an explicitly labelled test-only fixture and must
not start it automatically in normal application use.

## Task 140 verification

- `./desktop/gradlew -p desktop test` — passed (all tasks up to date; build successful).
- `git diff --check` — passed after this task's documentation/status updates.

No production, Go daemon, API contract, project source, credentials, generated output,
or supplied reference image changed in Task 140.
