# 167 — Refine the Files tree, editor tabs, and breadcrumbs

## Status

Complete

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

- [x] Active tabs are identifiable from both background and underline, with distinct focus and non-selected hover states.
- [x] Breadcrumbs are legible, icon-integrated, and honest about available navigation; root-level, deeply nested, long, and no-symbol cases work.
- [x] Files navigation still changes only the selected supported file; no editor generalization, filesystem mutation, or fake open files are introduced.
- [x] Source and diff remain selectable/read-only, with correct gutter alignment and scrolling after typography changes.

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

Completed 2026-09-05.

- Kept the existing virtualized explorer because it owns the persisted collapsed-directory set,
  stable indexed-file identity, and established keyboard contract. The available Jewel tree state
  would duplicate that state boundary; existing shared Jewel-adopted chrome primitives remain the
  component foundation. Explorer rows now use a 26dp minimum, consistent indentation, compact
  type icons, and full-path tooltips without hiding textual analysis status.
- Replaced the flattened breadcrumb string with folder/file/symbol segment data, subtle separators,
  compact icons, a full-path tooltip, and a full-path accessibility description. Segments remain
  non-actionable because this workspace exposes no corresponding navigation callback. The single
  real Source file tab and conditional Review tab retain `ChromeTab` active background, underline,
  hover, and focus behavior; no extra files or fake tabs were added.
- Made source and diff rows share the scale-safe minimum line-height metric, preserving selectable,
  read-only source/diff content, gutter alignment, and horizontal/vertical scrolling.
- Ran `spotlessApply`, focused explorer/editor/source/diff/keyboard/visual tests, then
  `JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop spotlessCheck detekt test -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home` successfully. Detekt uses the JDK 21 Gradle launch with the supported JBR 25 Kotlin toolchain.
- Ran `JAVA_HOME=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home ./desktop/gradlew -p desktop packageDistributionForCurrentOS` successfully, and `git diff --check` passed.
- Inspected production fixture evidence: `desktop/build/reports/ui-precision/task-167/editor-1440.png`, `editor-999.png`, `editor-breadcrumbs-deep-480-1.3.png`, `editor-candidate-800-1.3.png`, and `review-failed-800-1.3.png`. These are offscreen Compose renders; native-window, OS keyboard, and screen-reader verification remains for Task 170. No configuration or runtime migration is required.
