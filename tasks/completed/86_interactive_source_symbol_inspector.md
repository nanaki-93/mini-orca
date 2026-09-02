# 86 — Make source selection drive the symbol inspector

## Status

Complete

## Goal

Make clicking a declaration in the read-only source select and highlight it, then show
only that declaration's analysis in the right panel. Remove the duplicate File analysis
surface and all-symbol lists.

## Depends on

Task 85.

## Required task commentary

- Before editing, post a concise update beginning with `Starting Task 86` and name the
  source interaction, symbol inspector, obsolete surfaces, responsive behavior, and
  focused tests.
- After verification, post a separate update beginning with `Task 86 complete` and state
  the interaction delivered, tests run, and that Task 87 is next.

## Implementation

- Wire `CodePane`/its replacement to the Task 85 selection contract. A pointer click on
  any text source line must update the clicked line and containing symbol.
- Preserve `SelectionContainer`, horizontal/vertical scrolling, source text, line
  numbers, syntax highlighting, and read-only behavior. Confirm dragging still selects
  text instead of triggering repeated symbol changes.
- Keep whole-range selected-symbol emphasis and focused-line emphasis visually and
  textually distinguishable.
- Replace `TargetContextPane`'s symbol list with the Task 85 contextual inspector:
  - compact file fallback when no symbol is selected;
  - selected name/kind, signature, lines, confidence/editability, and only the selected
    symbol explanation;
  - one state-dependent Analyze/Refresh/Cancel action when required;
  - required remote-provider confirmation before Analyze/Refresh.
- Remove the Editor `FileAnalysis` surface, `EditorSurfaceBar`, surface toggle state and
  callbacks, and the `SummaryPane` all-symbol/file-analysis rendering.
- Keep an explicit **Refresh file analysis** entry in Commands when the fresh inspector
  intentionally has no routine Refresh button.
- Preserve useful optional file-level analysis behind at most one collapsed disclosure;
  do not move the symbol inventory into that disclosure.
- On narrow Editor layouts, source and command-palette symbol selection must open the
  existing Context drawer. Closing the drawer retains selection and source highlight.
- Rename/split `SummaryPane.kt` and its tests when needed so obsolete summary state does
  not survive beside the new source/inspector implementation.

## Acceptance criteria

- A single source click selects the containing declaration and updates the right panel.
- A click outside declarations shows file fallback context and no stale symbol
  explanation.
- The inspector never renders a complete symbol list.
- The Editor has no Source/File analysis toggle or duplicate full-canvas analysis view.
- Missing, stale, running, failed, and fresh analysis states show only the planned action.
- Fresh analysis can still be refreshed deliberately from Commands.
- Pointer text selection, read-only source, command-palette symbol selection, wide panes,
  and narrow drawers remain functional.
- No generation, draft mutation, Apply, or file write occurs from selection alone.

## Verification

- Add/extend source presentation, inspector, shell, and responsive tests.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

- Manually inspect click versus drag selection and Context drawer opening at 999dp and
  stable wide panes at exactly 1000dp.

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update `tasks/INDEX.md`. Do not stage or commit; this task does not authorize either.
