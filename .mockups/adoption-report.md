# Calibrated — full-app mockup atlas

**Direction:** Approved Instrument · Calibrated (I1.1), using Tokyo Midnight.
**Mode:** Reimagine. All 24 functional surfaces now share the selected style.
**Boundary:** Offline browser mockups grounded in `desktop/README.md` and the Kotlin owners below. This is not authorization for, or delivery of, a production implementation.

## Selected design

- Technical monospaced hierarchy, flat evidence panels and consistent action controls.
- Compact 78px icon-and-label rail; its softer labels remain readable at the selected density.
- Continuous coverage arcs. The denominator is selected files, not all indexed files. Coverage means analysis freshness, never project health, safety or model certainty.
- **Go workspace** beside **mini-orca** on Summary, with wrapping at narrow widths.
- A shared row gap above **Change lifecycle**: 14px at standard text size, scaling with larger text.
- The same header, navigation, component tokens, focus treatment and status language across every functional page.

The earlier desktop-shell, visual-refresh, style variants and Summary alternatives have been removed at the user's request. There are no alternative-style selectors or per-file ring treatments. The six coverage specimens remain because they demonstrate different evidence states, not different designs.

## Inventory

All mock paths below are relative to `.mockups/flows/tokyo-midnight/`. Owner paths are relative to `desktop/src/main/kotlin/io/miniorca/desktop/`.

| Surface | Current behavior owner | Mock |
| --- | --- | --- |
| Summary, coverage, packages, engineering insight | `ProjectSummaryPane.kt`, `ProjectSummaryVisuals.kt` | `01-summary.html` |
| Open / restore / switch project, re-index | `DesktopShell.kt`, `DesktopHeader.kt` | `02-project.html` |
| Analysis file selection, exclusions, freshness | `AnalysisFileSelector.kt` | `03-analysis-files.html` |
| Preview, request bounds, destinations, Security intent | `DesktopAnalysisAdmission.kt` | `04-analysis-consent.html` |
| Run, pause, resume, cancel, partial progress | `WorkspacePanes.kt`, `AnalysisRunStrip.kt` | `05-analysis-run.html` |
| Bug results, evidence, eligible fix preparation | `WorkspacePanes.kt`, `AnalysisResultsPane.kt` | `06-bugs.html` |
| Performance hypotheses, trade-offs | `PerformanceWorkspace.kt` | `07-performance.html` |
| Security source rules / AI hypotheses | `SecurityWorkspace.kt` | `08-security.html` |
| Read-only source, files, symbols | `EditorWorkspace.kt`, `ExplorerPane.kt` | `09-source.html` |
| Declaration / file context, explanation, refactor | `ContextToolWindow.kt` | `10-context.html` |
| Assistant, presets, constraints, destination consent | `AssistantToolWindow.kt` | `11-assistant.html` |
| New Go function / type, name boundary | `AssistantToolWindow.kt`, `CommandPalette.kt` | `12-new-declaration.html` |
| Editable declaration and imports, validate, discard | `AssistantToolWindow.kt` | `13-draft.html` |
| Focused checks, execution trust, repair | `ReviewEvidencePane.kt` | `14-checks.html` |
| Read-only unified / split diff, identity, explicit Apply | `ReviewEvidencePane.kt`, `DiffViewer.kt` | `15-review.html` |
| Apply receipt, guarded Undo | `ReviewEvidencePane.kt` | `16-receipt.html` |
| Compatible benchmark selection, trust, measurements | `PerformanceWorkspace.kt` | `17-benchmark.html` |
| Shell tabs, hide vs close, return freshness | `TerminalToolWindow.kt`, `TerminalTabs.kt` | `18-terminal.html` |
| Files / symbols / contextual commands | `CommandPalette.kt` | `19-search.html` |
| Model scopes and destinations, separate daemon status | `DesktopStatusBar.kt` | `20-providers.html` |
| Empty, failed, stale, partial, canceled, unavailable | Feature state owners and `ReviewEvidencePane.kt` | `21-states.html` |
| Architecture / flow diagrams, source, zoom, fallback | `MermaidDiagram.kt`, `ProjectSummaryPane.kt` | `22-diagrams.html` |
| Explicit trusted Go scan, diagnostics | `WorkspacePanes.kt` | `23-verified-scan.html` |
| Read-only included / excluded context and request limits | `DesktopShell.kt` | `24-context-manifest.html` |

### Topology

The [atlas](flows/tokyo-midnight/index.html) is a **hub-and-spoke** collection grouped into Understand, Analyze, Change and Tools. Analysis has a select → preview/consent → run branch. Editing has a **hybrid** request → draft → checks → review → receipt path, with source/context, repair and discard cross-links. Numbered filenames organize the inventory; this is not a mandatory 24-step wizard.

### Shared design ownership

- [Entry point](index.html) and [all-functions navigator](flows/tokyo-midnight/index.html).
- [Summary](flows/tokyo-midnight/01-summary.html) is the canonical selected composition.
- [Tokens](design-system/tokens.css) own the locked palette and approved density/type defaults.
- [Shared CSS](design-system/components.css) owns primitives, application chrome and coverage components.
- [Component specimens](design-system/components.html), [palette / contrast](design-system/palette.html) and [typography](design-system/typography.html) document the same direction, not additional choices.
- Production theme, components and workflow state owners remain unchanged. The documentation link in `desktop/UI_DESIGN_GUIDELINES.md` points to the retained Summary instead of the deleted concept. No agent instructions or configuration are changed.

## Functional and evidence boundaries

All numbers, source, paths, requests, findings, measurements and receipts are **illustrative fixtures**. No remote assets, network calls, daemon integration, shell processes, project-code execution, source writes or persistent trust are required. Fixtures generally reset between pages; cross-links may preview an already-prepared stage and never perform its action.

The local demonstrations retain explicit destination consent and Security intent; execution trust for checks/scans/benchmarks; isolated editable declaration/import drafts; selectable read-only source/diffs; explicit Review/Apply; stale Apply blocking; and receipt-gated/conflicting Undo. Name and draft validation are bounded fixture demonstrations, not arbitrary Go parsers or backend guard implementations.

Coverage distinguishes current, outdated, missing, running, failed, incomplete and unavailable states. Unknown coverage uses an em dash, not zero. No selection is a separate neutral state. The default fixture is **5 current, 1 outdated, 1 failed / 7 selected**; one excluded file is outside the denominator. All-current analysis is not an assurance of safety. Model suggestions, source-rule/tool findings and measured benchmark specimens remain distinct.

## Larger-text / offline mirror

Every functional page supports `?persona=reading`: a 20px base, simplified composition, system fonts and no remote resources. The link and atlas checkbox carry this preference through navigation. Summary retains its selected coverage specimen when toggling larger text.

Long paths wrap; code and evidence tables scroll within their own containers. Keyboard operation, visible focus, labeled controls and dialog focus recovery are browser review requirements, not a claim of VoiceOver or native Compose acceptance.

## Verification

- Static checks passed across **29 HTML documents**: local links/fragments, unique IDs/attributes, JavaScript syntax, token resolution, external-resource absence and new-file whitespace. Documentation links and Kotlin owner paths were also verified; `git diff --check` passed.
- Chrome passed **163 viewport/text-size checks** across all functional pages, the navigator, landing page and design-system specimens. No document overflow, unlabeled fields or empty actions were found in those cases. The standard desktop rail is 78px throughout.
- **24 coverage cases** passed across six states, standard/larger text and desktop/narrow widths. Unknown/empty evidence remains distinct. Narrow larger-text legends use one column so incomplete/unavailable labels do not collide. Lifecycle row spacing and inline project type were checked.
- Local interactions passed for consent/retry/resume, selection locking, pause/cancel, declaration name/type boundaries, draft/import evidence invalidation, execution trust, failed checks, stale Apply blocking, receipt/conflict-gated Undo, benchmark outcomes, shell tabs, search, diagram fallback and verified-scan consent. Keyboard legend inspection, dialog Tab/Escape/focus recovery and larger-text navigation also passed.
- All 24 desktop compositions and representative narrow/larger-text, consent, lifecycle and Review renders were visually reviewed. Compact change-flow diagrams wrap rather than hiding later stages offscreen. No external HTTP requests were observed in the main browser run.
- Preservation checks confirmed the palette is unchanged and the other 23 pages retain their functional main content. Summary now uses the approved composition; alternatives, stale links and style selectors are gone.

Go/Gradle and native accessibility checks were **not run**: production code and build inputs are untouched. Browser checks cannot validate daemon guards, model output, operating-system terminal behavior or native accessibility. Translating this design into existing Compose owners requires an explicit production implementation request and the repository's desktop/native checks.
