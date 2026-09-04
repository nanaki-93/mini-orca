# 167 — Refine the Files tree, editor tabs, and breadcrumbs

## Status

Pending

## Depends on

Task 166, including its verified local commit.

## Goal

Make navigation compact and unmistakable while preserving the one-file read-only editor contract.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`ExplorerPane.kt`, `EditorWorkspace.kt`, `SourceEditorPane.kt`, `DiffViewer.kt`, `DesktopIcons.kt`; `ExplorerPaneTest.kt`, `EditorWorkspaceTest.kt`, `DiffViewerTest.kt`, keyboard/visual tests.

## Implementation

1. Use the selected Jewel tree/list foundation for indexed project navigation where its APIs fit, preserving stable file identity, expansion, selection, textual analysis state, virtualization, and existing keyboard behavior.
2. Normalize tree rows to the shared 24–28dp default, consistent indentation, and compact type icons. Keep long relative paths accessible without hiding status or shrinking fonts.
3. Apply the shared active-tab background and 2dp bottom accent line to the one real file/Source/Review chrome. Keep focus separate; do not render illustrative multi-file tabs as live controls.
4. Replace the flattened breadcrumb string UI with actual project-relative segments, subtle separators, and folder/file/symbol icons at 12sp typography. Keep real navigation callbacks only; static segments remain non-actionable.
5. Preserve file/symbol identity when breadcrumbs overflow and provide a full-path accessible/tooltip affordance. Keep source and gutter aligned on the editor canvas.
6. Remove obsolete breadcrumb/selection rendering helpers after their callers move; preserve current source/diff selection, line mapping, evidence labels, and read-only semantics.

## Acceptance criteria

- [ ] Active tabs are identifiable from both background and underline, with distinct focus and non-selected hover states.
- [ ] Breadcrumbs are legible, icon-integrated, and honest about available navigation; root-level, deeply nested, long, and no-symbol cases work.
- [ ] Files navigation still changes only the selected supported file; no editor generalization, filesystem mutation, or fake open files are introduced.
- [ ] Source and diff remain selectable/read-only, with correct gutter alignment and scrolling after typography changes.

## Verification

Run navigation/breadcrumb/read-only tests and keyboard interactions with a deep-tree fixture. Run `./desktop/gradlew -p desktop spotlessCheck detekt test`, visually inspect Source/Review plus long paths, and run `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
feat(desktop): refine editor tabs and breadcrumb navigation
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

