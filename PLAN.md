# Desktop UX/UI refinement plan

Date: 2026-09-02

Status: Complete — automated validation passed; manual GUI acceptance remains unavailable
in this non-interactive environment and is recorded in `docs/RELEASE_ACCEPTANCE.md`.

## Outcome

Make the active file immediately identifiable in the Editor, remove instructional and
progress copy that competes with source content, avoid displaying an empty imports
control, and make Bugs triage start with the highest-priority findings.

This is a desktop-only presentation change. It must not change daemon APIs, finding
serialization, file/draft identity guards, source/diff read-only behavior, or the
explicit validation/check/apply workflow.

## Audited baseline

- `DesktopHeader.kt` shows the project name in the application top bar but not the
  selected file.
- `EditorWorkspace.kt` places an `EDITOR PROGRESS` block above every Editor canvas.
  Its `EditorProgressUiState` is also used to decide whether the canvas shows source
  or the review diff, so the state itself cannot simply be deleted.
- `SourceEditorPane.kt` contains the exact helper copy: “Click a highlighted
  declaration start line to inspect it. Drag anywhere to select source text.”
- `DraftContextPane.kt` unconditionally renders the `Required imports` field, even
  when `EditableDraftState.imports` is empty.
- `BugsWorkspacePane` currently groups the filtered findings by provenance
  (verified, suggested, unclassified). The established finding contract instead has
  normalized severities of `high`, `medium`, and `low`; card provenance is already
  displayed independently.

## Product decisions

### Active file header

Add a compact, persistent header at the top of the Editor canvas, for both source
and review-diff states:

- Show the selected file's basename as the primary, high-contrast title.
- Show its project-relative path as secondary text so duplicate basenames remain
  distinguishable.
- When no file is open, use a neutral no-file state rather than a stale title.
- Keep the header above the scrollable source/diff content, and size the canvas to
  the remaining height so a long file or diff stays usable.

The global application bar continues to identify the project. This header identifies
the file currently being inspected or reviewed; it is not a second file selector.

### Remove redundant Editor/source copy

Remove the visible Editor progress block in full: its section label, current-state
explanation, step trail, and progress-specific accessibility description. Continue
to derive `EditorProgressUiState` internally for routing source versus diff and the
context panel; do not replace the removed block with another progress subtitle.

Remove the visible source-pane “Click a highlighted declaration…” instruction. Keep
the interaction itself, line-level semantics, pointer affordance, drag-to-select
behavior, keyboard symbol picker, focused-line text, and declaration highlighting
unchanged.

### Empty Required imports

Render the `Required imports` compact field only when the editable draft already has
at least one required import. A nonempty field remains editable and retains its
current parsing, disabled-state, validation, and draft-invalidation behavior. Do not
create placeholder imports or add a new import-management flow as part of this
focused change.

### Bugs grouped by priority

Replace Bugs' top-level provenance sections with severity sections in this deterministic
order:

1. `HIGH PRIORITY`
2. `MEDIUM PRIORITY`
3. `LOW PRIORITY`
4. `OTHER PRIORITY` only when legacy or malformed client data has a blank or unknown
   severity.

Apply the existing search and advanced filters before grouping. Render only nonempty
priority sections, preserve the incoming order within a section, and retain the
provenance/confidence text on every card so tool-reported findings and AI suggestions
remain distinguishable without being separate top-level lists. No grouping preference,
new filter, persistence, API, or server-side sorting is needed: priority grouping is
the default Bugs presentation.

## Implementation steps

1. Update `desktop/src/main/kotlin/io/miniorca/desktop/EditorWorkspace.kt` and its
   call site in `DesktopShell.kt`.
   - Replace the progress-bar composable with a small Editor file-header composable
     that accepts the selected `ProjectFileInfo?`.
   - Remove `EditorProgressStep`, progress labels, and progress semantics that have
     no remaining consumer. Keep the derived progress model where `DesktopShell` and
     the context routing need it.
   - Ensure the source or diff canvas occupies the remaining space below the header
     on wide and narrow layouts.

2. Update `desktop/src/main/kotlin/io/miniorca/desktop/SourceEditorPane.kt`.
   - Delete only the visible “Click a…” helper `Text` and its now-unused layout
     spacing/imports.
   - Do not alter `declarationSymbolAtLine`, pointer handling, source selection, or
     semantics describing a selectable declaration.

3. Update `desktop/src/main/kotlin/io/miniorca/desktop/DraftContextPane.kt`.
   - Put the `Required imports` field behind an explicit nonempty-imports condition.
   - Leave the declaration field, diagnostics, status text, and Validate action in
     their current order when imports are absent.

4. Update `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` and
   `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt`.
   - Add a small pure severity/priority grouping helper that normalizes whitespace and
     case, defines the display order, and retains unknown severities in the final
     fallback group.
   - Feed it the already filtered findings and render priority section labels and
     cards from its nonempty groups.
   - Retire classification-based list-section plumbing, but preserve
     `classifyFinding`/provenance labeling if it remains the single source of truth
     for card provenance text.

5. Update focused desktop tests and the manual acceptance wording.
   - Replace progress-bar assertions with file-header title/path/no-file and
     no-progress-copy assertions.
   - Cover the hidden source helper without weakening source-line selection and drag
     behavior tests.
   - Cover empty versus nonempty imports presentation and confirm nonempty imports
     still reach the existing draft-edit callback.
   - Cover priority order, filtering-before-grouping, source/confidence retained on
     cards, and unknown-severity fallback handling.
   - Amend `desktop/README.md` and `docs/RELEASE_ACCEPTANCE.md` so manual Bugs
     verification expects priority grouping while provenance remains visible.

## Delivery sequence and commit boundaries

Task 89 from the preceding direct-symbol backlog must be Complete before this backlog
starts. Because several new tasks touch the same Editor files, its implementation must
also be committed or otherwise separated from the new work before Task 90 begins. The
execution agent must stop rather than fold pre-existing overlapping changes into a new
task commit.

| Task | Outcome | Required commit |
| ---: | --- | --- |
| [90](tasks/completed/90_editor_active_file_header.md) | Replace visible Editor progress UI with a prominent active-file header | `feat(desktop): emphasize active editor file` |
| [91](tasks/completed/91_remove_editor_source_instruction.md) | Remove the visible “Click a…” source instruction without changing interaction | `refactor(desktop): remove source helper subtitle` |
| [92](tasks/completed/92_hide_empty_required_imports.md) | Hide `Required imports` when the draft import list is empty | `fix(desktop): hide empty required imports` |
| [93](tasks/completed/93_group_bugs_by_priority.md) | Group filtered Bugs findings by high, medium, low, then fallback priority | `feat(desktop): group bugs by priority` |
| [94](tasks/completed/94_desktop_ux_refinement_acceptance.md) | Complete integration, documentation, responsive, accessibility, and regression acceptance | `test(desktop): complete UX refinement acceptance` |

Tasks execute strictly in numeric order with one implementation writer. Each task must
pass its focused checks and the complete Desktop test suite, update its own task/index
metadata, stage only its task-owned changes, and create exactly one reviewed commit
before the next task starts. Do not amend, squash, combine, or push these commits. Use
[`tasks/PROMPT_EXECUTE_DESKTOP_UX_REFINEMENTS.md`](tasks/PROMPT_EXECUTE_DESKTOP_UX_REFINEMENTS.md)
for the full sequence.

## Verification

Run during implementation:

```text
./desktop/gradlew -p desktop test
git diff --check
```

Manual desktop checks:

- Open a file with a repeated basename elsewhere in the project; confirm the large
  Editor title and the relative path identify the active file in both source and
  review-diff states.
- Confirm neither the Editor progress block nor the “Click a…” source instruction is
  visible, while clicking a declaration, dragging to select source, and keyboard
  symbol selection continue to work.
- Generate or load one draft without imports and one with imports; only the latter
  displays `Required imports`, and its field remains editable.
- On Bugs, load a mix of high, medium, low, verified, and AI-suggested findings;
  confirm high appears first, each card still states its provenance, filters regroup
  the visible results, and no empty priority headings appear.
- Repeat the Editor check at 1000dp and below 1000dp to confirm the existing
  Editor-only Files/Context drawer behavior remains intact.

## Definition of done

- The open file's basename and project-relative path are prominent at the top of the
  Editor source and review views.
- Editor progress explanatory UI and the “Click a…” source subtitle are absent.
- Empty editable drafts do not display `Required imports`; nonempty imports behave as
  before.
- Bugs displays the filtered findings in high-to-low priority sections by default,
  with card-level provenance preserved.
- Desktop tests and `git diff --check` pass, and no daemon/API/configuration or
  workflow-safety behavior changed.
