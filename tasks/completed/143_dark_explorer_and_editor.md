# 143 — Restyle the project tree and source editor

## Status

Complete

## Depends on

Task 142.

## Goal

Reproduce the mockups' compact tree, file tab, breadcrumbs, gutter, and syntax hierarchy
using the current single-file, read-only editor.

## Implementation

- Add consistent folder, file-type, expand/collapse, reveal, and filter icon treatment.
  Use restrained selected-row backgrounds and visible textual analysis freshness.
- Give the real active file a tab-shaped header with blue selection edge. Keep Source
  and Review controls, full accessible relative path, selected declaration, and
  read-only state. Additional tab/split/minimap previews are owned by Task 147.
- Consolidate duplicate editor headings and use the planned tab/breadcrumb heights.
- Align line numbers, markers, source baselines, active line, and symbol selection.
  Keep the existing syntax parser and selectable source content.
- Preserve horizontal source scrolling, vertical gutter/source alignment, long paths,
  duplicate basenames, empty files, and existing unsupported-language behavior.

## Likely files

`ExplorerPane.kt`, `EditorWorkspace.kt`, `SourceEditorPane.kt`,
`EditorInspectionState.kt`, and their existing desktop tests.

## Acceptance criteria

- Explorer and editor visibly match the shared tokens, density, icons, and tab style.
- There remains one real active file; selecting another file follows existing draft
  discard handling and clears/reloads the appropriate context.
- Dragging selects text without changing a symbol; declaration navigation and gutter
  markers identify actual source locations.
- Source is never editable, and visual formatting does not alter copied source text.
- Empty/long/unsupported-language files stay readable and do not create fake evidence.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use `ExplorerPaneTest`, `EditorWorkspaceTest`, and `EditorInspectionStateTest` for
changed behavior. Manually inspect a long file, duplicate basenames, text selection,
gutter alignment, and keyboard tree navigation when native access is available.
Complete/move the task and update the index; no unrequested commit.
