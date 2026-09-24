# Calibrated UI/UX implementation plan

**Status: proposed; no implementation authorized or started by this plan.**

Translate the approved **Instrument · Calibrated / Tokyo Midnight** mockups into the existing Kotlin/Compose desktop application. The backlog contains **38 implementation features covering all 24 mock surfaces**, followed by an integrated release gate. Most work is presentation and interaction enhancement around existing functionality—not rebuilding the daemon or its workflows.

## 1. References and implementation approach

- [Full mockup atlas](../.mockups/flows/tokyo-midnight/index.html)
- [Canonical Summary](../.mockups/flows/tokyo-midnight/01-summary.html)
- [Shared components](../.mockups/design-system/components.html), [tokens](../.mockups/design-system/tokens.css), [typography](../.mockups/design-system/typography.html)
- [Mockup inventory and limitations](../.mockups/adoption-report.md)
- [Current desktop behavior](../desktop/README.md), [UI verification guidelines](../desktop/UI_DESIGN_GUIDELINES.md), [native keyboard checklist](../desktop/KEYBOARD_SMOKE_CHECKLIST.md)

### What already exists and what changes

| Area | Current implementation | Planned enhancement |
| --- | --- | --- |
| Theme and controls | Semantic dark palette, shared Jewel/Compose controls, rounded 10–18dp shapes in `DesktopTheme.kt` | Calibrated surfaces, flatter shapes, technical typography, consistent spacing and interaction states |
| Shell | Six workspace destinations; 48dp icon rail with hover labels | Approximately 78dp labeled rail at standard density, grouped utility actions, aligned header/footer |
| Summary | Segmented coverage bar; project type in the facts text; existing category metrics and project interpretation | Continuous arcs, inline project type, new composition, file evidence and findings previews |
| Analysis | Persistent file selection, preview/admission, run progress and lifecycle controls | Clearer scope, consent, progress and recovery presentation; same workflow authority |
| Results | Shared browser with filters, selection, evidence and category-specific eligibility | Calibrated rows/cards/details without losing existing search, filters or loaded findings |
| Editing | Existing declaration composer, isolated draft, validation, checks, read-only diff, guarded Apply/Undo | A clearer visual progression and decision area; no relaxation of editing boundaries |
| Tools | Existing command palette, provider dialog, local diagram renderer and terminal | Consistent presentation and accessible native integration, not new execution engines |

### Native translation decisions

1. **Keep six peer workspaces:** Summary, Analysis, Bugs, Performance, Security and Editor. The atlas's 24 pages document functions and states, not 24 new native routes. Consent and provider details remain transient surfaces; editing stays within Editor; Terminal stays a dock.
2. **Use the approved appearance, not browser mechanics.** Do not ship the mock banner, atlas links, fixture selectors, simulation buttons, query-string navigation or synthetic data. The mock's larger-text switch is a review aid, not a requirement for a new settings screen.
3. **Translate CSS dimensions to native layout roles.** Use the 78px rail and 14px Summary row gap as standard-density references, then verify native dp/sp geometry, intrinsic heights, display density and text scaling. Do not copy fixed HTML heights or hide actions to match a screenshot.
4. **Keep truthful category counts.** `ProjectSummaryIssues.kt` already separates category progress from overall tool/AI totals. Do not copy the fixture's category-level provenance breakdown unless actual category evidence supports it. Preserve bug-priority detail and unknown counts.
5. **Preserve project content.** The mock's “Change lifecycle” is a Mini-Orca workflow explainer. It must not replace the project's own saved Flows, architecture, module descriptions or engineering insight.
6. **Use real data for new Summary previews.** File evidence comes from the current selection/status owner; findings come from identity-matched loaded evidence. Use actual detection/report timestamps when available. Otherwise use a truthful title such as “Selected findings,” not an invented recency claim. Keep a route to all records.
7. **Reuse authority and lifetime owners.** Compose renders immutable state and emits actions through `DesktopWorkflowPresenter` and the existing specialized workflows. No new eligibility implementation, parallel consent store or request dispatcher in visual helpers.
8. **Keep execution boundaries explicit.** Workspace navigation, inspection and disclosures do not call providers or execute project code. Opening Terminal is an explicit shell action under its existing contract, not an ordinary navigation side effect. Restoring layouts never starts shells.
9. **Default to no wire-contract change.** Missing display data must be represented truthfully. If a required interaction genuinely needs unavailable data, identify the smallest coordinated API/models/docs/test change before implementation; do not fabricate a field or silently expand this backlog into a backend rewrite.

### Scope exclusions

No light theme or alternative-style selector; new autonomous behavior; multi-file source editing; full-file source editor; provider/credential settings; new terminal platforms; dependency/toolchain upgrades; arbitrary command runner; new benchmark engine; analytics service; or motion-system project.

## 2. Delivery order and milestones

Feature IDs identify scope, not a mandatory numeric implementation order. Respect the dependencies in each feature. Existing working controls remain wired while their replacement presentation is developed; never merge placeholder callbacks.

| Wave | Features | Reviewable outcome |
| --- | --- | --- |
| 1 — Shared foundation | F01–F05 | One native visual language, usable controls, responsive shell and consistent state treatment |
| 2 — Project and Summary | F06–F11, F35–F36 | Open/restore a real project; review the Calibrated Summary, live coverage, diagrams and utility surfaces |
| 3 — Analysis lifecycle | F12–F16 | Select → preview → consent → run → pause/resume/cancel, with truthful retained evidence |
| 4 — Results | F17–F20 | Complete Bugs, Performance and Security experiences, including explicitly trusted Go scans |
| 5 — Declaration change | F23–F34 | Source → context/request → draft → validation → checks/repair → review → Apply → receipt/Undo |
| 6 — Benchmarks and Terminal | F21–F22, F37–F38 | Identity-bound measurements and the native terminal integrated with the new shell |
| 7 — Integrated qualification | Release gate below | Visual, workflow, accessibility and native acceptance for the complete application |

**Recommended first review milestone:** waves 1 and 2. Validate the real native shell and Summary before extending the composition across every workflow.

Independent work can proceed after shared components stabilize: Summary/diagrams, analysis, and the read-only Editor can be separate streams. Diff review can proceed alongside checks once draft validation is available. Benchmark work requires the validated-draft path. Coordinate edits to shared shell, theme, presenter wiring and `ReviewEvidencePane.kt` rather than assigning overlapping implementations blindly.

Sizes below are relative scope, **not calendar estimates**: **S** = bounded view; **M** = several components plus state integration; **L** = a substantial cross-pane or native interaction. Every feature includes tests and visual review. Accessibility and failure states are not deferred to wave 7.

## 3. Mock-to-feature coverage

`Mxx` below refers to the corresponding mock. All 24 surfaces have an implementation owner.

| Mock/function | Native destination | Features |
| --- | --- | --- |
| [M01 Summary](../.mockups/flows/tokyo-midnight/01-summary.html) | Summary | F08, F09, F10; shared F11 diagrams |
| [M02 Project](../.mockups/flows/tokyo-midnight/02-project.html) | Landing, chooser, project menu | F06, F07 |
| [M03 Analysis files](../.mockups/flows/tokyo-midnight/03-analysis-files.html) | Analysis / Files | F12 |
| [M04 Preview and consent](../.mockups/flows/tokyo-midnight/04-analysis-consent.html) | Analysis admission dialog | F13, F14 |
| [M05 Analysis run](../.mockups/flows/tokyo-midnight/05-analysis-run.html) | Analysis; compact Summary/header status | F15, F16 |
| [M06 Bugs](../.mockups/flows/tokyo-midnight/06-bugs.html) | Bugs | F17 |
| [M07 Performance](../.mockups/flows/tokyo-midnight/07-performance.html) | Performance | F18 |
| [M08 Security](../.mockups/flows/tokyo-midnight/08-security.html) | Security | F19 |
| [M09 Source](../.mockups/flows/tokyo-midnight/09-source.html) | Editor / Files / Source | F23 |
| [M10 Context](../.mockups/flows/tokyo-midnight/10-context.html) | Editor / Context | F24 |
| [M11 Assistant](../.mockups/flows/tokyo-midnight/11-assistant.html) | Editor / Assistant | F26 |
| [M12 New declaration](../.mockups/flows/tokyo-midnight/12-new-declaration.html) | Assistant creation mode | F27 |
| [M13 Draft](../.mockups/flows/tokyo-midnight/13-draft.html) | Isolated draft editor | F28, F29 |
| [M14 Checks](../.mockups/flows/tokyo-midnight/14-checks.html) | Review evidence and repair handoff | F30, F31 |
| [M15 Review / Apply](../.mockups/flows/tokyo-midnight/15-review.html) | Candidate comparison and decision pane | F32, F33 |
| [M16 Receipt / Undo](../.mockups/flows/tokyo-midnight/16-receipt.html) | Post-Apply Review state | F34 |
| [M17 Benchmark](../.mockups/flows/tokyo-midnight/17-benchmark.html) | Performance / Benchmark evidence | F21, F22 |
| [M18 Terminal](../.mockups/flows/tokyo-midnight/18-terminal.html) | Terminal dock | F37, F38 |
| [M19 Search](../.mockups/flows/tokyo-midnight/19-search.html) | Command palette | F35 |
| [M20 Providers](../.mockups/flows/tokyo-midnight/20-providers.html) | Provider details dialog | F36 |
| [M21 States and recovery](../.mockups/flows/tokyo-midnight/21-states.html) | Applicable owner surfaces, not a new workspace | F05 plus each feature's failure cases |
| [M22 Diagrams](../.mockups/flows/tokyo-midnight/22-diagrams.html) | Summary diagrams / local expanded viewer | F11 |
| [M23 Verified scan](../.mockups/flows/tokyo-midnight/23-verified-scan.html) | Bugs / verified Go scan | F20 |
| [M24 Context manifest](../.mockups/flows/tokyo-midnight/24-context-manifest.html) | Read-only context inspector | F25 |

## 4. Detailed feature backlog

**Path convention:** implementation owners below are in `desktop/src/main/kotlin/io/miniorca/desktop/`; named test classes are in `desktop/src/test/kotlin/io/miniorca/desktop/`. They are starting points, not permission to refactor entire files. F01–F05 provide shared prerequisites for all subsequent surfaces.

### F01 — Calibrated native theme and typography

**Size:** M · **Depends on:** none · **Reference:** shared tokens and typography.

**Implement:** map Tokyo Midnight to the existing semantic palette; introduce compact monospaced workspace/evidence roles with sans-serif navigation/narrative roles; flatten panel/control shapes; consolidate spacing and separators. Keep semantic warning/error/focus/diff roles rather than styling by feature name.

**Acceptance:** rendered controls and text match the selected direction at standard density; contrast passes the maintained checks for normal, selected, hovered, focused and disabled states; long paths and source remain legible at larger text. No remote fonts, new theme choices or scattered screen-local color literals.

**Owners/checks:** `DesktopTheme.kt`, `DesktopIcons.kt`; `DesktopThemeTest`, `DesktopContrastTest`, production component captures.

### F02 — Shared controls and evidence primitives

**Size:** L · **Depends on:** F01 · **Reference:** component showcase.

**Implement:** restyle existing buttons, fields, checkboxes, tabs, cards, badges, disclosures, dialogs and status/evidence rows. Establish consistent primary/destructive action hierarchy, disabled explanations, focus styling and copy/select behavior. Reuse shared controls instead of creating a second component library.

**Acceptance:** keyboard and pointer activation agree; labels and states are accessible without color or hover; dialogs contain focus and restore it on dismissal; larger text does not clip controls. Optional help may collapse, but active failures and required consent remain visible. Model prose still uses the bounded `ModelResultContent` renderer.

**Owners/checks:** `ChromeControls.kt`, `DesktopTheme.kt`, `DiagnosticText.kt`, `ModelResultContent.kt`; `ChromeControlsTest`, `ModelResultContentTest`, `DesktopVisualLayoutTest`.

### F03 — Application shell, labeled rail, header and footer

**Size:** L · **Depends on:** F02 · **Reference:** chrome on every functional mock.

**Implement:** compact icon-and-label rail with six workspace destinations; grouped Terminal, Commands and Models actions; Calibrated project/branch/search header; separate analysis and daemon status; aligned model-count footer. Utility entries open existing surfaces rather than creating workspaces.

**Acceptance:** the standard rail approximates the approved 78px reference without truncating labels; selected and keyboard-focused states are distinct; existing shortcuts still reach every workspace. Workspace changes preserve file, draft, results and shells. Unknown branch/model values are not fabricated. Analysis and daemon status do not claim provider health.

**Owners/checks:** `IdeShell.kt`, `DesktopHeader.kt`, `DesktopShell.kt`, `DesktopStatusBar.kt`, `DesktopLayoutState.kt`; shell, status-bar and keyboard-navigation tests, native focus smoke.

### F04 — Adaptive layout, larger text and preference compatibility

**Size:** L · **Depends on:** F03 · **Reference:** standard and larger-text mocks.

**Implement:** explicit wide/compact arrangements for shell regions and content grids; adaptable pane widths; reachable scroll regions and action areas; responsive labels and paths. Preserve applicable saved pane dimensions and normalize old preferences. Respect text scaling without adding a speculative settings system.

**Acceptance:** source/diff retain internal horizontal scrolling; the application does not lose content offscreen; consent, diagnostics, Apply and recovery remain reachable at reduced height and 125%/150% text. Resizing preserves focus and selection. Restored layouts never revive execution authority or launch a terminal.

**Owners/checks:** `DesktopLayoutState.kt`, `DesktopShell.kt`, `IdeShell.kt`, `EditorWorkspace.kt`; layout-state tests and the production viewport/text-scale matrix.

### F05 — Consistent states, errors and recovery

**Size:** M · **Depends on:** F02 · **Reference:** M21, applied throughout.

**Implement:** consistent owner-specific presentations for loading, no project, no selection, not analyzed, empty success, filter-no-match, failed, stale, partial, canceled, interrupted and unavailable states. Keep recovery adjacent to the affected operation and preserve useful retained evidence.

**Acceptance:** unknown counts never become zero; failure is not an empty result; old successful evidence cannot mask an active rerun. Retry/reconnect/refresh actions explain their scope and preserve required consent. The specimen gallery does not become a production page or a generic replacement state machine.

**Owners/checks:** `DesktopTheme.kt`, `AnalysisWorkspaceState.kt`, `ResultBrowserState.kt`, `ReviewEvidencePane.kt`, relevant feature owners; parameterized state tests and failure-state renders for each surface.

### F06 — Open project, automatic restore and landing recovery

**Size:** M · **Depends on:** F03, F05 · **Reference:** M02.

**Implement:** Calibrated landing and opening states; clear current/last-project identity; native Open project entry; visible restore failure and retry. Distinguish “opening local project” from analysis activity.

**Acceptance:** chooser cancellation preserves the current state; inaccessible paths show recoverable errors; successful restore loads local metadata without requesting a model or starting a shell. Project-only shortcuts stay unavailable before a project opens. Late responses from an older open attempt cannot replace the latest project.

**Owners/checks:** `DesktopShell.kt`, `DesktopHeader.kt`, `LastProjectStore.kt`, `DesktopWorkflowPresenter.kt`; `LastProjectStoreTest`, shell/presenter tests and native chooser smoke.

### F07 — Project switching and explicit re-indexing

**Size:** M · **Depends on:** F06 · **Reference:** M02.

**Implement:** clear Switch/Open and Re-index actions with project identity and progress; preserve explicit draft-discard handling and shell-close confirmation. Show indexing failures without pretending existing evidence was refreshed.

**Acceptance:** canceling a switch preserves the draft and every terminal tab; confirmed switching rejects old-project responses and closes owned shells through the existing path. Re-indexing changes inventory/freshness, never starts model analysis. Existing file-selection persistence and absent-path exclusions remain intact.

**Owners/checks:** `DesktopHeader.kt`, `DesktopShell.kt`, `DesktopWorkflowPresenter.kt`, `DesktopTerminalWorkspace.kt`; presenter, selection and terminal-switch regression tests.

### F08 — Summary composition, project identity and narrative

**Size:** M · **Depends on:** F03, F04, F05 · **Reference:** M01.

**Implement:** the approved Summary hierarchy: project name and inline type, metadata and explicit analysis action, coverage/results region, architecture and insight, findings/packages, then a correctly separated Change lifecycle section. Preserve access to the project's own Flows and complete model narrative.

**Acceptance:** Go, Java, Kotlin and unknown project types come from real metadata; names/types wrap without overlap. Empty/stale/failed project descriptions retain their meaning independently of file coverage. The lifecycle gap uses a shared spacing role; the application workflow explainer is not presented as the analyzed project's architecture or flows.

**Owners/checks:** `ProjectSummaryPane.kt`, `EngineeringInsightPanel.kt`, `ModelResultContent.kt`; Summary/insight tests and native component comparisons with M01.

### F09 — Continuous-arc analysis coverage and local inspection

**Size:** L · **Depends on:** F08 · **Reference:** M01.

**Implement:** replace the segmented bar with a continuous-arc dial, textual denominator, accessible status legend and local file-scope inspection. Reuse the selected-file coverage and freshness rules already maintained by the client; distinguish saved freshness from active-run progress.

**Acceptance:** percentages use the current selected-file denominator; excluded files never inflate coverage; zero-size states are omitted. Unknown coverage has no invented percentage; empty selection has no health score. All-current does not imply safety. Legend activation is keyboard-accessible and starts no work. Counts and inspected paths agree for the same project/revision; unavailable paths remain explicitly unavailable. Large counts do not produce invisible or misleading negative arcs.

**Owners/checks:** `ProjectSummaryVisuals.kt`, `ProjectSummaryPane.kt`, `AnalysisFileSelection.kt`, `AnalysisFileStatus.kt`; Summary/selection/status tests, arc rendering and legend interaction tests.

### F10 — Summary results, file evidence and findings previews

**Size:** M · **Depends on:** F08, F09 · **Reference:** M01.

**Implement:** restyled Bugs/Performance/Security cards, a compact current-selection evidence ledger and a bounded findings preview with a route to the complete results. Reuse actual identifiers and existing status/provenance presentation.

**Acceptance:** category counts can be unknown independently of coverage; neutral zero results do not assert safety. Preserve supported bug-priority counts and overall tool/AI totals. Show only real file/finding rows, with deterministic selection/order and truthful recency labeling. A selected preview opens its exact finding or explains why it is no longer available; it never opens an unrelated fixture target or prepares a fix.

**Owners/checks:** `ProjectSummaryIssues.kt`, `AnalysisCategoryPanels.kt`, `ProjectSummaryPane.kt`, `FindingsPresentation.kt`; Summary issue tests and result-navigation identity tests.

### F11 — Architecture and project-flow diagrams

**Size:** M · **Depends on:** F08 · **Reference:** M22 and M01.

**Implement:** Calibrated diagram containers, local expand/collapse and zoom, readable source and error fallback. Expose the architecture preview and project flows using the existing renderer; keep larger exploration within a local expanded view rather than adding a workspace. For Summary, render a compact available preview locally, with full controls on expansion.

**Acceptance:** flowcharts, sequence diagrams and supported saved legacy content remain available; invalid/unsupported content shows its source and an error, never an invented diagram. View state resets on owner identity changes, not unrelated recomposition. Large diagrams remain bounded and navigable; opening, zooming and copying do not contact a provider or run project code.

**Owners/checks:** `MermaidDiagram.kt`, `MermaidRenderer.kt`, `MermaidImage.kt`, `ProjectSummaryPane.kt`; `MermaidRendererTest`, expanded/error/large-text renders. No renderer dependency upgrade is included.

### F12 — Analysis file selection and freshness table

**Size:** L · **Depends on:** F03, F04, F05 · **Reference:** M03.

**Implement:** compact searchable file table with state filters, selection, matching/total counts, excluded reasons, details and explicit refresh. Keep Files within Analysis and preserve its disclosure state and visible lock explanation.

**Acceptance:** Select all/Exclude all operate over all eligible files, not just filtered rows. Selection persists per project; save/load failures retain the confirmed selection. Active, paused and interrupted runs lock edits while keeping search/filter/refresh available. Pending/Finished progress does not upgrade stale saved evidence to current. Re-selecting a file restores its real saved state.

**Owners/checks:** `AnalysisFileSelector.kt`, `AnalysisFileSelection.kt`, `AnalysisFileStatus.kt`, `DesktopAnalysisWorkflow.kt`; selection/status/workflow tests, keyboard table operation and large-list layout checks.

### F13 — Analysis preview and request-scope review

**Size:** M · **Depends on:** F12 · **Reference:** M04.

**Implement:** structured preview of selected files, exclusions, applicable stages, reused versus fresh results, expected requests, inclusive retry bounds and model destinations. Show full-run, stale/failed retry and resume scopes distinctly in the admission surface.

**Acceptance:** values come from the returned preview, not client estimates copied from the mock. Loading/failure/no-eligible-file cases have clear outcomes; previews cause no model dispatch. Long destinations and exclusion reasons are readable. Existing daemon dispatch bounds remain informational—no new Run limits controls.

**Owners/checks:** `DesktopAnalysisAdmission.kt`, `DesktopAnalysisWorkflow.kt`, `Models.kt`; admission/workflow tests for fresh, selective, resumed, empty and failed previews.

### F14 — Destination consent, Security intent and run admission

**Size:** M · **Depends on:** F13 · **Reference:** M04.

**Implement:** separate confirmations for every required remote destination and explicit AI Security review intent, beside the final Start/Resume action. Clearly explain which context will be sent and that starting analysis does not execute or modify project code.

**Acceptance:** missing required consent blocks dispatch; changing preview identity/destination invalidates obsolete confirmation through the existing owner. Dismissal does not grant permission. Start is a single explicit admission, not one request per UI card; stale preview/conflict errors return the user to a fresh review. Resume receives a fresh preview without silently changing the admitted refresh policy.

**Owners/checks:** `DesktopAnalysisAdmission.kt`, `DesktopAnalysisWorkflow.kt`, `DesktopWorkflowPresenter.kt`; admission and workflow tests with recorded fake request counts.

### F15 — Analysis progress, evidence and run history

**Size:** M · **Depends on:** F14 · **Reference:** M05.

**Implement:** Calibrated run overview with active paths, per-stage state, finished/total files, retained results, elapsed time and previous-run/failure details. Coordinate the full Analysis view with compact Summary and header status.

**Acceptance:** finished includes partial/failed work and is never labeled all-success; unknown totals remain unknown; no invented ETA. Only matching project/revision/plan/file identities contribute active progress. Results stay navigable during a run. Long-running, failed, interrupted and completed-empty states remain readable with controls visible at reduced height.

**Owners/checks:** `AnalysisRunStrip.kt`, `AnalysisWorkspaceState.kt`, `WorkspacePanes.kt`, `DesktopHeader.kt`; run-presentation tests and Summary/Analysis state consistency renders.

### F16 — Pause, resume, cancel and continuation UX

**Size:** M · **Depends on:** F14, F15 · **Reference:** M05.

**Implement:** explicit Running → Pause requested → Paused transitions; scoped Resume/continuation entry; Cancel feedback and recovery. Keep cumulative attempts, retained results and stop reasons visible.

**Acceptance:** pause waits for the stage boundary; cancel prevents further dispatch without erasing completed evidence. Resume uses F14 admission. Startup never silently resumes work; canceled work requires a new admitted start rather than a fake Resume. Rapid repeated actions and late polling responses cannot restore obsolete authority or mislabel the lifecycle.

**Owners/checks:** `DesktopAnalysisWorkflow.kt`, `AnalysisWorkspaceState.kt`, `AnalysisRunStrip.kt`; deterministic pause/cancel/resume and stale-response tests.

### F17 — Bugs result browsing and fix preparation

**Size:** M · **Depends on:** F05, F15 · **Reference:** M06.

**Implement:** Calibrated severity/title/location rows, current category navigation, local search/filtering, evidence details and an explicit Prepare fix action. Establish the shared results presentation reused by F18/F19, without flattening their domain differences.

**Acceptance:** all loaded findings remain reachable; no-match is distinct from zero findings and failed loading. Selection only inspects. Prepare fix is enabled only by existing current/exact declaration eligibility and pre-fills Assistant without sending. Stale/unclassified/tool-reported/model evidence stays distinguishable; source links preserve exact paths and lines.

**Owners/checks:** `AnalysisResultsPane.kt`, `FindingsPresentation.kt`, `WorkspacePanes.kt`, `BugsWorkspaceState.kt`, `ResultBrowserState.kt`; findings/browser/Bugs tests and result-layout renders.

### F18 — Performance hypotheses and trade-offs

**Size:** M · **Depends on:** F17 · **Reference:** M07.

**Implement:** performance-specific result presentation for observed pattern, potential impact, workload, trade-offs and verification plan; scoped source/fix actions; a clear entry to benchmark evidence.

**Acceptance:** recommendations are explicitly unmeasured until matching measurements exist; confidence is not a measured speedup. All loaded hypotheses and supporting narrative remain accessible. Stale source or inexact targets block preparation with an explanation. Selecting a recommendation or opening benchmark details does not run a benchmark.

**Owners/checks:** `PerformanceWorkspace.kt`, `AnalysisResultsPane.kt`; `PerformanceWorkspaceTest`, shared result-layout tests and evidence distinction renders.

### F19 — Security findings and advisory review

**Size:** M · **Depends on:** F17 · **Reference:** M08.

**Implement:** security-specific evidence for deterministic source-rule findings versus AI hypotheses, exact source anchors, verification guidance, scope and disabled preparation reasons. Surface the existing explicit review-intent path without introducing a second analysis entry mechanism.

**Acceptance:** empty Security results never claim safety; unavailable review is not zero findings. Selection and disclosure are local. AI review respects the current admission/consent owner; Prepare fix retains exact current Go eligibility. Partial, stale and failed evidence is not restyled as verified success.

**Owners/checks:** `SecurityWorkspace.kt`, `DesktopSecurityWorkflow.kt`, `AnalysisResultsPane.kt`; Security workspace/workflow tests and empty/partial/stale renders.

### F20 — Verified Go scan and diagnostic inspection

**Size:** M · **Depends on:** F17 · **Reference:** M23.

**Implement:** a clear Bugs tool section showing the supported scan scope and exact execution request, explicit local trust, scan progress, results and selectable diagnostics. Maintain a visible distinction from AI findings.

**Acceptance:** opening Bugs or expanding diagnostics starts no scan. Untrusted execution requires the explicit action; unsupported projects explain availability. Cancellation, failure and partial diagnostics stay visible. A delayed result for another project/revision cannot replace current evidence, and a tool report is not a general safety assurance.

**Owners/checks:** `WorkspacePanes.kt`, `BugsWorkspaceState.kt`, `DesktopWorkflowPresenter.kt`; presenter/Bugs tests with fake scan responses and trusted-execution checks.

### F21 — Benchmark discovery, choice and execution admission

**Size:** M · **Depends on:** F18, F29 · **Reference:** M17.

**Implement:** compatible benchmark listing, selected benchmark/command/scope, unavailable reasons and explicit trusted comparison for the exact validated candidate. Distinguish read-only catalog discovery from execution.

**Acceptance:** listing/selecting benchmarks does not run project code; choices come from the matching daemon catalog. No validated candidate, empty catalog and stale choice all explain why comparison is blocked. The explicit action shows the selected fixed command and preserves project-revision trust requirements. Changing draft/file/project/choice invalidates obsolete work.

**Owners/checks:** `PerformanceWorkspace.kt`, `DesktopBenchmarkWorkflow.kt`; benchmark workflow tests for catalog identity, selection, trust, failure and late responses.

### F22 — Benchmark measurements and inconclusive evidence

**Size:** M · **Depends on:** F21 · **Reference:** M17.

**Implement:** baseline/candidate measurements with units, sample context, comparison status, command and identity. Show completed, inconclusive, stale, canceled, failed and unavailable outcomes without replacing them with a generic success card.

**Acceptance:** absent allocations/bytes remain unavailable rather than zero; no speedup is inferred without suitable matching measurements. Retained stale measurements are labeled and cannot establish current readiness. Changing candidate identity removes current-comparison claims. Viewing/copying results starts no run, and benchmarks do not become a new unconditional Apply requirement.

**Owners/checks:** `PerformanceWorkspace.kt`, `DesktopBenchmarkWorkflow.kt`, `Models.kt`; performance/benchmark tests and measured/partial/unavailable visual fixtures.

### F23 — Read-only source, file explorer and symbol selection

**Size:** L · **Depends on:** F03, F04, F05 · **Reference:** M09.

**Implement:** the Calibrated file/source/context composition, unambiguous file/declaration breadcrumbs, source selection highlighting and New function entry. Preserve local file-tree and symbol navigation, source text selection and the existing pane preferences.

**Acceptance:** source is never editable; dragging/selecting text does not retarget a draft. Duplicate basenames use relative paths; read errors are explicit. Opening files does not change analysis scope or request model output. Symbol target changes respect active-draft discard rules. Resizing preserves scroll, selection and keyboard access.

**Owners/checks:** `EditorWorkspace.kt`, `SourceEditorPane.kt`, `ExplorerPane.kt`, `EditorInspectionState.kt`; explorer/editor/inspection tests and source-selection native smoke.

### F24 — Declaration/file context and on-demand explanation

**Size:** M · **Depends on:** F23 · **Reference:** M10.

**Implement:** concise selected-declaration identity, explanation and Refactor actions; file Actions/Details when no declaration is selected; expandable project context and references. Keep request status and source freshness beside the content they qualify.

**Acceptance:** selection/tab changes never generate explanations. Explicit explanation respects its destination/consent boundary; Refactor only prepares the existing composer. Loading/current/stale/canceled/failed explanations are distinct, and delayed answers cannot attach to another declaration. Missing symbols and inexact targets retain useful file details and clear blocked reasons.

**Owners/checks:** `ContextToolWindow.kt`, `EditorInspectionState.kt`, `DesktopWorkflowPresenter.kt`; context and presenter tests, including explanation target changes.

### F25 — Read-only context manifest and request limits

**Size:** M · **Depends on:** F24 · **Reference:** M24.

**Implement:** a structured inspector for included/excluded paths and reasons, sizes, estimated tokens, token/byte limits, truncation and the exact request destination. Make long paths and diagnostic details selectable and readable.

**Acceptance:** inspection does not send context to a provider or grant consent. Loading/failure is not rendered as an empty successful manifest; unavailable limits are not invented. The displayed manifest is bound to the relevant request/target, and stale inspection cannot overwrite a newer one. Closing restores the initiating control's focus.

**Owners/checks:** `DesktopShell.kt`, `DesktopWorkflowPresenter.kt`, `Models.kt`; shell/presenter tests and inspector keyboard/large-text renders.

### F26 — Assistant request composer and scoped conversation

**Size:** L · **Depends on:** F24, F25 · **Reference:** M11.

**Implement:** target and operation header, Fix/Refactor/Document presets, intent, optional advanced constraints, context inspection, destination confirmation and explicit Send/Cancel. Recompose existing conversation content into the approved hierarchy without hiding previous turns or failures.

**Acceptance:** presets change local request intent only; empty intent or ineligible targets cannot send. Remote consent matches the relevant scope/destination. Errors stay beside the failed request; cancellation and late-response guards remain intact. Model prose is readable but supplied HTML, links and images remain inert. An active draft is never silently replaced by navigation.

**Owners/checks:** `AssistantToolWindow.kt`, `FileChatState.kt`, `EditorContextualActions.kt`, `DesktopWorkflowPresenter.kt`; Assistant/chat/presenter tests and consent/cancel renders.

### F27 — New Go function/type preparation

**Size:** M · **Depends on:** F26 · **Reference:** M12.

**Implement:** creation-mode selection, prominent new-name field, behavior request and absent-name feedback. Connect file header, Context and Commands entry points to the same focused composer.

**Acceptance:** package-only Go files work; invalid/reserved/existing names are rejected before a provider request. Non-Go/ineligible files explain the boundary. Preparing or changing creation kind inserts nothing. Canceling a conflicting-draft discard preserves the previous draft. Generation enters the same isolated draft/check/review pipeline as replacement.

**Owners/checks:** `AssistantToolWindow.kt`, `CommandPalette.kt`, `ContextToolWindow.kt`, `FileChatState.kt`; creation/Assistant/command tests and initial-focus checks.

### F28 — Isolated declaration/import draft editor

**Size:** M · **Depends on:** F26, F27 · **Reference:** M13.

**Implement:** clearly separate editable declaration/import fields from the read-only source; show exact target, draft revision, dirty/validated state and explicit discard. Preserve editor text selection, caret and normal text-editing behavior.

**Acceptance:** only the isolated candidate/import list is editable. Any edit invalidates matching validation/check evidence immediately; canceled discard loses nothing. Navigation/resize do not recreate the buffer. New-function, new-type and replacement drafts retain their different scope and import requirements without becoming a full-file editor.

**Owners/checks:** `AssistantToolWindow.kt`, `DraftEditorState.kt`, `DirectEditState.kt`; draft-editor and draft-review tests, caret/selection component checks.

### F29 — Draft validation and diagnostics

**Size:** M · **Depends on:** F28 · **Reference:** M13 and M14.

**Implement:** explicit Validate action, running feedback, current revision identity, readable validation diagnostics and the next eligible action. Preserve validation as a real evidence stage even though the mock's compact breadcrumb does not always name it separately.

**Acceptance:** no auto-validation on tab/disclosure changes. Only a response matching the current candidate can establish validated evidence. Invalid declaration/imports and stale source retain actionable diagnostics and block progression. Editing during/after validation makes older evidence unusable; successful validation does not imply tests passed or source was written.

**Owners/checks:** `DraftEditorState.kt`, `AssistantToolWindow.kt`, `ReviewEvidencePane.kt`, `DesktopWorkflowPresenter.kt`; draft-review/presenter tests and validation failure renders.

### F30 — Trusted focused checks and evidence panel

**Size:** L · **Depends on:** F29 · **Reference:** M14.

**Implement:** compact Validation / Focused checks / Source unchanged rows; exact command/scope, explicit trust-and-run action, running state, required-check summary, rerun and expandable full output. Keep failures visible with optional help collapsed.

**Acceptance:** disclosures never run checks; project-code execution retains its trust requirement. A rerun shows Running even after a previous pass. Missing/skipped/failed/stale evidence cannot imply readiness. Reports must match the current candidate identity; late results are rejected. Long output remains selectable and complete without hiding the next recovery action.

**Owners/checks:** `ReviewEvidencePane.kt`, `DesktopWorkflowPresenter.kt`, `DraftEditorState.kt`; review-evidence/draft-review/presenter tests and reduced-height decision-pane renders.

### F31 — Repair handoff after failed checks

**Size:** M · **Depends on:** F26, F30 · **Reference:** M14, returning to M11/M13.

**Implement:** understandable repair eligibility and bounded retry messaging; explicit handoff of the relevant failure to the existing Assistant repair path; preserve manual Edit draft as a recovery route.

**Acceptance:** no silent provider retry or repair loop; current destination consent and repair limits remain enforced. Repair failure/cancellation preserves the useful diagnostics and recoverable draft. A changed candidate requires fresh validation/checks before Review/Apply. Stale source and exhausted repair limits cannot be bypassed by the new action presentation.

**Owners/checks:** `ReviewEvidencePane.kt`, `AssistantToolWindow.kt`, `FileChatState.kt`, `DesktopWorkflowPresenter.kt`; repair-limit and request/evidence-identity regression tests.

### F32 — Read-only candidate diff and Review composition

**Size:** L · **Depends on:** F23, F29 · **Reference:** M15.

**Implement:** full-height Current/Candidate comparison beside a concise decision pane; matching breadcrumbs, change scope and import context; Side-by-side default and explicit Unified alternative. Keep Edit draft accessible without making the composed comparison editable.

**Acceptance:** source and diff text are selectable/read-only; vertical rows align and horizontal scrolling stays independent. Narrow layouts retain both sides and controls. Mode/tab changes perform no validation, checks or Apply. The decision area identifies the exact candidate and exposes the existing validation/check/source states, including blocked states before F33's action can be enabled.

**Owners/checks:** `DiffViewer.kt`, `EditorWorkspace.kt`, `ReviewEvidencePane.kt`, `WorkflowToolWindows.kt`; diff/editor/review tests and the comparison viewport matrix.

### F33 — Explicit guarded Apply decision

**Size:** M · **Depends on:** F30, F32 · **Reference:** M15.

**Implement:** a stable Apply action region containing exact declaration/file scope, readiness and any blocking reason. Keep success, conflict and failure feedback local to the operation and connect to the returned receipt.

**Acceptance:** existing eligibility is authoritative; source/draft/project identity changes immediately block stale evidence. One explicit action submits one guarded operation; no optimistic “applied” receipt or hidden auto-Apply. Failed/conflicting writes are not successes. The complete scope and action remain reachable at reduced height/150% text with optional help collapsed.

**Owners/checks:** `ReviewEvidencePane.kt`, `DraftEditorState.kt`, `DesktopWorkflowPresenter.kt`; Apply eligibility/conflict/double-activation tests and native action-reachability checks.

### F34 — Apply receipt and guarded Undo

**Size:** M · **Depends on:** F33 · **Reference:** M16.

**Implement:** post-Apply receipt with actual operation, file/declaration, returned identity/audit details and refreshed source; contextual Undo availability and recovery. Keep receipt and Undo within Review rather than creating an unrelated history system.

**Acceptance:** a real returned receipt—not entering a screen—establishes post-Apply state. Undo affects only the immediately preceding eligible unchanged Apply; external edits block it. Successful Undo refreshes source and cannot enable an older chain. Failed/expired/unavailable Undo retains a clear reason and never claims restoration. Show hashes/audit details only when supplied by the existing result.

**Owners/checks:** `ReviewEvidencePane.kt`, `DesktopWorkflowPresenter.kt`, `Models.kt`; receipt/Undo conflict and source-refresh regression tests.

### F35 — Files, symbols and contextual commands

**Size:** M · **Depends on:** F03, F04 · **Reference:** M19.

**Implement:** Calibrated palette modes and result rows, contextual command availability, full path disambiguation, empty/no-match states and visible shortcut hints. Reuse existing search and action dispatch instead of introducing a new search service.

**Acceptance:** arrow keys navigate results; Enter activates from the search field, while Enter/Space activates focused result controls without stealing spaces from text entry. Escape closes only the top transient surface and restores focus. Query/filtering is local and no-result state is not a loading failure. Commands retain their existing eligibility/confirmation rules. Source navigation and composer preparation do not accidentally generate or execute anything; terminal keystrokes are not intercepted.

**Owners/checks:** `CommandPalette.kt`, `DesktopKeyboardNavigation.kt`, `DesktopAccessibility.kt`, `DesktopShell.kt`; palette/keyboard tests and native popup/focus smoke.

### F36 — Provider/model details and honest connectivity

**Size:** M · **Depends on:** F03, F05 · **Reference:** M20.

**Implement:** read-only scope/destination/model cards or rows for Analyze, Bugs and Function edits; configured local/cloud totals; captured-run configuration where relevant; explicit distinction from daemon connectivity. Connect footer and rail Models entry to the same dialog.

**Acceptance:** shared configured models count once; missing scope data produces unavailable totals. Captured run providers are not mislabeled as current configuration or live health. No invented provider health checks, editable credentials or new settings. Merely opening details grants no consent and contacts no provider; long destinations remain selectable.

**Owners/checks:** `DesktopStatusBar.kt`, `DesktopHeader.kt`, `DesktopShell.kt`; status-bar/model-scope tests, unknown/incomplete catalog and captured-run renders.

### F37 — Terminal dock, tabs and visual integration

**Size:** M · **Depends on:** F03, F04 · **Reference:** M18.

**Implement:** Calibrated bottom dock and shared Terminal/tab bar, active-tab treatment, New shell / Close tab / collapse actions, resize affordance and readable session labels. Apply the palette and supported text scale to the existing native terminal component.

**Acceptance:** the Swing/PTY canvas stays visible within its bounds; tabs and close actions remain reachable when space is limited. Hiding or switching tabs preserves the process and in-memory scrollback; closing a tab is visibly different from hiding the dock. Use real native sessions, never the mock's simulated shell. Qualify this together with F38 before release.

**Owners/checks:** `IdeShell.kt`, `TerminalToolWindow.kt`, `TerminalTabs.kt`; terminal-tab/tool-window tests, native canvas bounds and resize smoke.

### F38 — Terminal focus, session lifecycle and source return

**Size:** L · **Depends on:** F07, F23, F34, F37 · **Reference:** M18.

**Implement:** finish the new shell's integration with terminal key ownership, explicit shell startup, session lifecycle/error feedback, app-focus return, project-switch confirmation and source-freshness recheck. Reuse the current session implementation; this is not a PTY rewrite.

**Acceptance:** ordinary shell input/Ctrl+C stays with Terminal; Ctrl+Shift+F12 returns to Editor without closing it. Resize reaches real PTY dimensions. Restore starts no shell; canceled switching preserves all sessions; confirmed switching/exit uses owned-process cleanup and shows cleanup-pending failures. Shell edits invalidate stale source/draft/check evidence on return. No transcripts enter persistence or provider context.

**Owners/checks:** `DesktopTerminalWorkspace.kt`, `DesktopTerminalSession.kt`, `TerminalToolWindow.kt`, `DesktopShell.kt`; terminal/session/presenter tests and the documented native/packaged terminal checks on the supported host.

## 5. Definition of done for every feature

1. **Complete interaction:** a real data/state owner and action path, not a static replica. Cover loaded, empty, running, failed, stale and unavailable states where applicable.
2. **Preserved boundaries:** no provider/project-code/source-write side effects from passive UI; consent, trust, discard and identity checks remain in their existing owners. Preserve cancellation and reject outdated asynchronous responses.
3. **Visual evidence:** render the changed production components with deterministic fixtures and compare against the referenced Calibrated mock. Check normal/focus/disabled/error states, not just the happy screenshot.
4. **Accessibility:** labeled focusable controls, meaningful order, text-equivalent states, selectable read-only content, long paths and full diagnostics. Verify with optional help collapsed as well as expanded.
5. **Responsive evidence:** use the matrix below and verify both content and action reachability. Retained pane preferences must not trap a view offscreen.
6. **Behavioral tests:** extend the named owners' tests with successful and boundary/failure cases. Use fake providers and temporary projects, not live credentials. Call-count tests must prove that navigation/disclosures/selection do not dispatch privileged work.
7. **Maintained gates:** run `./scripts/desktop-gradle.sh test spotlessCheck detekt` for desktop changes. If a feature actually changes Go/API behavior, add the corresponding root Go/cross-stack gates and contract tests; do not treat a frontend workaround as a contract change.
8. **Documentation and cleanup:** update the current desktop guide only when behavior is delivered. Remove superseded presentation branches, controls and obsolete tests without weakening safety/quality assertions. Record intentional geometry changes against the selected design, not an unexplained new baseline.

## 6. Integrated release gate

### Viewport, density and state matrix

- Production components at representative logical sizes: **1600×1000, 1440×900, 1024×768, 800×650 and 1280×600**, plus both sides of any changed breakpoint.
- **100%, 125%, 150% text**; representative **1× and 2× display density** checked separately from font scale. The 390px browser mock is a stress reference, not a claim of a supported mobile application.
- All-current/mixed/unavailable/empty selected-file coverage; running/pausing/paused/interrupted/canceled/partial/failed analysis; completed-empty results versus unknown results.
- Missing/stale/failed/running validation and checks; blocked/current Apply; actual receipt, Undo conflict and Undo failure; measured/inconclusive/stale/unavailable benchmark evidence.
- Long project names, paths, destinations, error output and many files/findings. Large lists must remain scrollable and responsive without silently truncating the loaded dataset.
- Keyboard-only operation, topmost-dialog Escape behavior, focus restoration, text selection/copy, visible focus and accessible names/states. Native screen-reader behavior requires separate observation.

### End-to-end acceptance journeys

1. Restore/open → Summary → inspect coverage/files, with no provider request or shell startup.
2. Change file selection → preview → consent → run → pause → fresh resume admission → cancel; retained evidence remains truthful.
3. Browse each result category → inspect evidence → open exact source → prepare eligible fix; selection alone sends nothing.
4. Request replacement/new function/new type → edit isolated draft → validate → trusted checks → repair if needed → review → Apply → receipt → guarded Undo.
5. Change a file externally at Review/after Apply → verify stale Apply/Undo blocking and clear recovery; repeat with a failed freshness read.
6. Select a compatible benchmark → review command/trust → compare → inspect measurements; change the draft while work is pending and reject the obsolete result.
7. Open two real terminal sessions → switch/hide/resize → edit a temporary file → return to Review → confirm invalidation → cancel/confirm project switching and cleanup.
8. Trigger project/provider/read failures and canceled operations → verify no fabricated success, lost draft, provider-health claim or automatic retry.

### Commands and evidence

Use the documented toolchain and root commands; do not hardcode a local JDK path.

```sh
./scripts/desktop-gradle.sh test spotlessCheck detekt
./scripts/validate.sh
```

For F37/F38, also follow [Terminal validation](../desktop/TERMINAL.md#validate-the-terminal), including the explicit native probe, distributable and packaged smoke checks. Native terminal support is currently the documented macOS arm64 target; this plan does not expand it.

Report unit/contract results, production offscreen captures and native-window observations separately. A successful browser mock run is **not** evidence that Compose, screen readers, provider consent, backend guards or real terminals pass.

## 7. Planning assumptions and approval boundaries

- The selected Calibrated direction is the implementation reference for this effort; native accessibility or data constraints may require a documented local adaptation, not a new round of unrelated visual alternatives.
- Existing native behavior takes precedence over illustrative fixture shortcuts. Any deliberate product-policy change needs separate approval, especially authorization, persistence, execution or supported-platform changes.
- API expansion is conditional on demonstrated need. New Summary previews should first use existing selection, overview and loaded finding/report models.
- Layout preference compatibility is required; no planned migration of project metadata, draft formats, provider configuration or credentials.
- Implementation estimates should be refined after the first native foundation/Summary milestone. Do not promise delivery dates from HTML mock complexity alone.
- This document is a proposed backlog, not a completion report or an instruction to launch autonomous implementation.

**Planning verification:** current desktop documentation, key presentation/state owners, test inventory and the 24-surface mock inventory were inspected. All 38 feature entries, acyclic dependencies, delivery-wave ordering, 24-surface coverage, referenced links/owners/tests/commands and whitespace were checked. No Go/Gradle, production rendering or native smoke checks were run for this planning task.
