# 53 — Complete accessible responsive interaction

## Status

Pending

## Goal

Make the complete four-workspace and editable-draft workflow usable by keyboard and below 1000dp.

## Depends on

Tasks 45, 46, 47, 48, 49, 50, 51, and 52.

## Implementation

- Add workspace shortcuts and update workspace cycling from three to four destinations.
- Provide keyboard routes to explorer, symbol picker, Bugs filters, chat composer, draft editor, Validate, checks, and review actions.
- Restore focus predictably after dialogs, file/finding navigation, generation, validation, Apply, and undo.
- Add semantic roles/descriptions and selected/disabled state for new controls.
- Keep severity, confidence, freshness, scan/job state, draft state, validation, and connection understandable without color.
- Preserve responsive drawers while keeping the compact file/symbol brief visible.

## Acceptance criteria

- Open file → select/create target → chat → edit → validate → checks → Apply is keyboard-operable.
- Escape cancels only the active dialog or request.
- Compact layout does not hide the open-file brief or permit source editing.

## Verification

- Extend accessibility/shortcut tests and add semantics checks for critical controls.
- Run `./desktop/gradlew -p desktop test` and the desktop keyboard smoke checklist.
