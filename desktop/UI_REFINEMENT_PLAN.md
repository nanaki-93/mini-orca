# UI refinement plan

**Status:** In Progress — Tasks 150–155 established the baseline, removed duplicate Project
navigation, unified shared IDE tokens, corrected workspace widths, and refined popup/disclosure
and tool-window surfaces; Tasks 156–160 remain Pending.

**Requested:** 2026-09-04.

**Scope:** Desktop presentation, navigation simplification, and replacement cleanup.

Follow [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) and the supplied dark
mock. Retain the current Analysis screen's visual direction. This plan supersedes
older instructions to retain a separate Project rail entry, but does not execute
historical task prompts or mark their pending acceptance checks complete.

## Decisions and boundaries

1. Remove **Project from the activity rail**, not project functionality. Editor
   becomes the sole file/source workspace. Keep its indexed file tree, file filter,
   project chooser, re-index action, and narrow-window Files drawer. Label the
   tree **Files** to distinguish it from the top project selector.
2. Reduce routine interface prose throughout the app. Keep concise action labels,
   state, errors, and safety-critical information visible; make supporting detail
   available on demand rather than deleting user data or analysis results.
3. Rebuild Summary using Analysis's metric hierarchy and restrained surfaces.
   Correct Analysis's width without replacing its accepted visual structure.
4. Treat the open/close-menu complaint as covering dropdowns, collapsible sections,
   tool-window headers, drawers, and the controls inside them.
5. Apply one token-based design system across every retained workspace and state.
6. Delete replaced code in the same implementation phase. Cleanup is a completion
   requirement, not a separate optional future project.

No daemon/API changes, source-editing expansion, automatic model requests, or new
Preview capabilities are included. Creating the plan/tasks does not execute them.
The user-requested execution prompt requires one verified local commit per task
when explicitly invoked; no pushes. Preserve remote-provider consent,
candidate validation/check identity, guarded Apply/Undo, and read-only source/diffs.

## Source-backed starting point

| Area | Current evidence | Planned correction |
| --- | --- | --- |
| Navigation | `DesktopLayoutState.kt` maps both `LeftToolWindow.Project` and `Editor` to `Workspace.Editor`; Project is also the saved-layout default | One Editor entry, updated default, no obsolete enum member or alternate route |
| Summary | `ProjectSummaryPane.kt` stacks three full-width panels, verbose status messages, and expanded interpretation lists | Compact facts/metrics first; concise purpose; structured, expandable details |
| Analysis width | `WorkspacePanes.kt` centers content with a hard `1160.dp` maximum, 28dp gutters, and weighted run/control cards | Shared available-width layout with bounded controls and consistent gutters |
| Menus | `DesktopHeader.kt` and `PreviewFeature.kt` use Material dropdown rows independently of quiet chrome | Consistent popup surface, row sizing, icons, focus, and disabled states |
| Disclosure | `EngineeringInsightPanel.kt` uses a button with text chevrons, repeated explanatory labels, and a second close button | One compact toggle header with a vector chevron and integrated state |
| Tokens | `DesktopTheme.kt` still uses the earlier charcoal palette | Reconcile centrally with the guideline palette; update contrast evidence |
| Verification | `DesktopVisualLayoutTest.kt` renders Analysis and Editor with test data | Add Summary and open/closed transient surfaces, including long-content states |

## Implementation order

Execute phases in order after implementation is requested. Each phase includes
focused tests, visual review where relevant, and removal of its superseded code.

The detailed backlog and statuses are in [tasks/INDEX.md](../tasks/INDEX.md).
Use [PROMPT_EXECUTE_UI_REFINEMENT.md](../tasks/PROMPT_EXECUTE_UI_REFINEMENT.md)
for sequential implementation with one commit per task.

| Plan area | Detailed tasks |
| --- | --- |
| Baseline, fixture coverage, and initial planning artifacts | 150 |
| Phase 1 — remove duplicate Project navigation | 151 |
| Phase 2 — shared tokens and workspace widths | 152–153 |
| Phase 3 — popup menus, disclosures, and tool windows | 154–155 |
| Phase 4 — Summary dashboard | 156 |
| Phase 5 — workspace and Editor/workflow copy/polish | 157–158 |
| Phase 6 — responsive/accessibility checks and final acceptance | 159–160 |

Task 150 verifies the current post-refactor implementation. Earlier Task 149 remains
a separate outstanding acceptance record, not a hard dependency of this sequence.

### 1. Remove duplicate Project navigation

- Remove the Project rail item, enum case, label/icon mapping, navigation branches,
  and obsolete assertions. Retain project-domain types and the folder icon where
  they are still used by real project/file controls.
- Use Editor as the default left navigation selection. Let existing unknown-enum
  preference recovery resolve a stored `Project` value to Editor; save the current
  value on the normal persistence path. Do not retain a hidden Project route or
  introduce a second layout implementation for compatibility.
- Preserve stored pane sizes, visibility, selected file/symbol, and candidate state.
- Update navigation documentation and test keyboard cycling, command-palette
  routing, project restoration, and Files drawer/reopen behavior.

**Acceptance:** The rail contains Summary, Analysis, Performance, Bugs & Problems,
and Editor, with exactly one selected destination. Existing users with a saved
Project selection reach Editor without losing pane preferences or file access.

### 2. Unify design tokens and correct workspace width

- Apply the guideline's neutral palette centrally: workspace `#1E1F22`, navigation
  `#18191B`, raised panels `#2B2D30`, accent `#3574F0`, muted selection `#2E436E`,
  and border `#323438`. Keep semantic text/status/diff/focus tokens and verify
  contrast, including text on the new action fill.
- Standardize shared typography, 4dp-grid spacing, restrained 1dp borders, and
  6dp contained-control corners. Keep existing quiet chrome and workflow-action
  roles distinct. Do not create a competing theme or per-screen palette.
- Replace Analysis's fixed maximum width with available workspace width. Initial
  targets: 24dp horizontal gutters on wide pages, 16dp on narrow pages; headers,
  metrics, progress, and error rows align to the same content edges.
- Give the run card remaining width and bound its adjacent control column to
  approximately 280–360dp. Stack them when the available content width cannot
  accommodate both; keep active controls reachable above long content.
- Use the same page-width rules for Summary. Bound long prose inside detail
  sections, not the entire dashboard. Preserve the exact 999dp/1000dp shell
  drawer boundary and the user's independently stored Editor pane widths.

**Acceptance:** Analysis retains its metrics/progress/control structure, fills the
available workspace without oversized side gutters, and neither stretches controls
excessively nor clips them on narrow windows. Summary shares its alignment rules.

### 3. Replace inconsistent menus and expandable surfaces

- Evaluate Jewel first for the shared component needs, verifying compatibility
  with the pinned Kotlin/Compose versions. Record the decision before any dependency
  change. If adopting it requires a broader platform migration, keep this phase
  within the existing stack and implement the needed token-styled components.
- Define shared popup/menu-row and disclosure-header treatments at the narrowest
  reusable boundary. Style the actual popup contents, not only their triggers.
- Target compact 32–36dp menu rows at normal text scale, growing as necessary;
  align leading icons, labels, trailing shortcuts/status, and submenu chevrons.
  Use a subtle border/shadow and quiet hover/focus/disabled/destructive states.
- Migrate project and Preview menus, Engineering insight, tool-window open/close
  headers, Files/Context drawers, bottom overlays, filters, and affected dialogs.
- Replace text chevrons and nested close-button/card patterns with a single
  disclosure header. Do not put nested buttons inside another clickable control.
- Keep expanded content on the same grid; group fields and actions without
  repeating the outer container's border or title. Preserve useful disclosure
  preferences and clear expanded/collapsed semantics.
- Support keyboard opening/navigation/activation, Escape, outside-click dismissal
  where appropriate, and focus restoration. Opening/closing a surface is local UI
  state only; it must not trigger analysis or source mutation.

**Acceptance:** Open and closed surfaces belong to the same visual system as the
main shell, remain inside the viewport, and work with keyboard and enlarged text.
Preview items remain explicitly Preview and cannot invoke backend/workflow actions.

### 4. Rebuild Summary around concise project information

- Replace the existing stacked-panel composition. Use a compact heading with
  project identity and build/language metadata, followed by a metric strip for
  indexed files, total lines, verified findings, and AI suggestions from real data.
- Present analysis coverage with compact counts and labeled state. Do not merge
  verified findings with AI suggestions, or render unavailable data as zero.
- Show one concise project-purpose preview with an **AI interpretation** label
  and freshness badge. Keep the full returned text reachable through an explicit
  expansion; do not call a model merely to shorten it.
- Put architecture, components, entry points, flows, risks, and next steps into
  compact expandable details instead of showing every paragraph immediately.
  Preserve their content, risk labels, and stale/error meaning.
- Provide small contextual links to Analysis and Bugs rather than a large action
  card. Keep deterministic facts visible when model interpretation is missing,
  running, stale, or failed.
- Update the presentation model and its shared AI Context consumer deliberately;
  remove redundant presentation fields/helpers instead of adding a parallel model.

**Acceptance:** A populated Summary shows identity, key metrics, coverage, and a
short purpose without requiring a wall of prose. All full interpretation content
remains accessible. Missing data, genuine zero counts, and stale AI results are
visually distinct, including at narrow widths.

### 5. Reduce copy and polish all remaining screens

- Audit Summary, Analysis, Performance, Bugs, Editor, AI Context, Assistant,
  Review, Checks, Output, landing/empty/error states, menus, and status details.
  Apply the same shared styles rather than isolated cosmetic overrides.
- Remove descriptions that repeat a heading, button, status badge, or nearby
  counter. Give ordinary empty states one short line; include recovery actions
  when useful. Show helper text when it is needed, not beneath every control.
- Keep routine actions to short verb-led labels. Use tooltips or explicit Details
  for explanations, with equivalent keyboard access and accessible names.
- Shorten Analysis's repeated subtitle/run/empty-error explanations without
  changing its accepted metric and progress hierarchy.
- Preserve essential warnings, provider destination/consent, stale evidence,
  destructive-action scope, Apply/Undo requirements, and actionable error detail
  at the decision point. Do not hide these solely in a tooltip or disclosure.
- Keep user messages, generated candidate content, logs, and analysis results
  intact; reduce interface narration, not evidence. Full long paths and messages
  must remain reachable if abbreviated in compact rows.

**Acceptance:** Normal screens have no repeated instructional paragraphs or
duplicated state sentences. Primary content and the next eligible action are
easy to scan, while safety-critical information and full detail remain available.

### 6. Verify the complete replacement

- Extend production-component fixtures to Summary, menus, disclosures, drawers,
  and affected workflow screens. Capture representative empty, populated, stale,
  loading, error, long-name, and long-content states.
- Review at 1440×900, 1920×1080, 1000×760, 999×760, and 800×650, plus 130% text
  scale. Check menu anchoring, pane budgets, labels, scroll reachability, and
  open/closed states rather than validating only default screenshots.
- Run desktop formatting, static analysis, tests, and contrast checks. Test saved
  Project preference recovery, unique navigation, preserved file/draft state,
  menu/disclosure focus, absent-versus-zero metrics, and Preview isolation.
- Perform a native keyboard/window review and screen-reader checks where the
  environment supports them. Label unavailable checks explicitly; offscreen
  fixture renders are not native screenshots or complete visual acceptance.
- Update README, keyboard checklist, visual evidence, and current acceptance
  references. Old task records may remain historical, but must not instruct new
  work to restore removed navigation or old component implementations.

**Acceptance:** Every requested area has reviewed before/after evidence, the
desktop checks pass, and the final diff contains no active legacy replacement
branches, stale UI descriptions, unused styles/helpers/imports, or debug code.

## Mandatory cleanup rule for every phase

Identify what each replacement retires before editing. Replace callers, remove
the old implementation and obsolete tests, then add tests for the current behavior.
Do not retain a legacy renderer, fallback theme, feature-flagged old shell, duplicate
navigation route, or compatibility wrapper without a real required boundary need.
Use the existing preference recovery mechanism rather than keeping dead UI alive.
Preserve unrelated code and required public behavior; this is not permission for
an unrelated repository-wide cleanup.

## Validation and handoff

Implementation checks:

```sh
./desktop/gradlew -p desktop spotlessCheck detekt test \
  -PvisualOutput="$PWD/desktop/build/reports/visual-review"
git diff --check
```

Report completed phases, removed legacy pieces, visual evidence, tests, and any
remaining native/manual checks. No user configuration migration is expected;
saved obsolete navigation resolves through the current preference reader. A custom
native titlebar or broader Jewel/toolchain migration is not required for this plan.

This planning change only adds documentation. Runtime code has not been changed
or tested as part of writing the plan/tasks. On execution, replace this planning
status with actual progress and, at Task 160, the verified outcome and limitations.
