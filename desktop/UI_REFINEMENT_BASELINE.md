# UI refinement baseline

**Recorded:** 2026-09-04
**Starting HEAD:** `991b916e24dff8048e68eb99a5a37d8a20ceb816`
**Scope:** Tasks 150–160 desktop refinement; Task 149 remains independently pending.

## Reproducible component captures

Run from the repository root:

```sh
./desktop/gradlew -p desktop spotlessCheck detekt test \
  -PvisualOutput="$PWD/desktop/build/reports/ui-refinement/before"
```

The production Compose fixture harness writes ignored PNGs to the configured directory. It uses
the explicitly labeled `go-shop · fixture` project and `Visual fixture · no backend` state; it
does not open a project, contact a daemon/provider, or mutate workflow state. The baseline
captures Analysis at `1440x900`, `1000x760`, `999x760`, `800x650`, and `1000x800` at 130% text
scale; Editor at `1440x900`, `1000x760`, and `999x760`; Summary at `1440x900`; and an open
Engineering insight disclosure. The actual Project and Preview `DropdownMenu` instances are
opened and inspected through their Compose semantic owners in the same fixture. Compose 1.7's
offscreen raster surface does not paint those detached popup layers, so its blank popup PNGs are
not claimed as visual menu evidence; Task 154 owns completing the menu-rendering adapter. These
are offscreen production-component renders, not native-window screenshots.

The supplied dark mock was inspected as a visual reference. Its sample project, multi-tab editor,
and unsupported controls remain non-functional reference material, not product requirements.

## Starting UI inventory

| Surface | Current implementation | Refinement owner |
| --- | --- | --- |
| Activity rail and workspace routing | `Project`, Summary, Analysis, Performance, Bugs & Problems, and Editor; both Project and Editor route to Editor. | 151 |
| Project opening and file navigation | Toolbar Project dropdown, indexed Explorer tree, filtering, re-indexing, and narrow Files drawer. | 151, 154, 155 |
| Shared tokens and chrome | Earlier charcoal tokens, 4dp spacing primitives, shared button/panel controls, line icons, and Compose Material popup/dialog infrastructure. | 152, 154, 155 |
| Analysis | Centered `1160dp` max-width page with 28dp padding; coverage metrics, run card, bounded job controls, progress, and failure rows. | 153, 157 |
| Summary | Three vertical cards for facts, advisory interpretation, and coverage; complete returned lists expand the initial page. | 156 |
| Menus and Preview dialogs | Project and Preview menus each use local Material dropdown rows; Preview dialogs are local-only and restore trigger focus. | 154 |
| Disclosures and tool windows | Engineering insight uses text chevrons and a separate Close action; docked/drawer/bottom surfaces have independent headers. | 155 |
| Project-level workspaces | Summary, Analysis, Performance, Bugs/Problems contain duplicated explanatory status/copy in ordinary states. | 157 |
| Editor and workflow surfaces | Read-only selectable source/diff, isolated draft, explicit validation/check evidence, guarded Apply/Undo, Assistant, Review, bottom evidence, and status detail surfaces. | 158 |
| Responsive/accessibility verification | `1000dp` stays docked; `<1000dp` uses Files/Context drawers and a bottom overlay. Existing fixtures do not yet cover all refined transient states. | 159 |

## Preserved safety and state boundaries

- One project, indexed file, and selected symbol remain the active scope. Source and diff are
  selectable/read-only; only the isolated declaration/import draft is editable.
- Navigation, menu/disclosure opening, and fixture rendering are presentation-only. They do not
  request models, analyze files, execute external projects, write source, or alter eligibility.
- Remote-provider destination and consent stay textual at the request decision point. Daemon
  connectivity is not provider connectivity.
- Validation and focused checks bind review evidence to the draft. Apply and Undo remain the only
  guarded source mutations, with receipt and invalidation behavior retained.
- Docked Explorer/Context/bottom preferences persist independently. The existing unknown-enum
  preference recovery is the compatibility boundary for retired navigation values.

## Baseline findings for later replacement

- The duplicate `LeftToolWindow.Project` enum entry, icon/label mapping, default selection, and
  Editor routing are active code, not merely historical documentation.
- `DesktopTheme.kt` still carries the earlier charcoal role values; later token work must migrate
  roles centrally instead of applying colors per workspace.
- Analysis constrains its whole page to `1160dp`; a shared gutter rule can replace that cap
  without touching stored Editor pane widths or the `999dp`/`1000dp` breakpoint.
- Summary presentation converts missing overview counts to zero and mixes a long advisory
  interpretation directly into the initial surface; Tasks 156–157 must preserve source and
  freshness meaning while separating absent data from genuine zero values.
- Project and Preview popup contents use separate `DropdownMenuItem` styling. Engineering insight
  duplicates its toggle/prose labels and includes text-glyph chevrons plus an inner Close button.

## Verification availability and known limits

- The desktop fixture uses actual Compose components and Skia. Its output is suitable for layout,
  clipping, semantics, and interaction assertions, not native title-bar or assistive-technology
  acceptance.
- No running Mini-Orca native window or supported screen reader was available in this environment.
  Task 159 records these as release-operator follow-ups; this does not alter Task 149.
- `make check` passed. `make quality` reached its existing `gocyclo -over 15` gate and failed on
  five out-of-scope Go functions: `(*Service).reviewPerformanceFile` (19),
  `validPerformanceJob` (19), `validPerformanceFinding` (18), `(*Service).StartPerformanceJob`
  (18), and `(*Service).StartAnalyzeAll` (16). It therefore did not reach the later clone check;
  no Go change is authorized by this UI sequence.
