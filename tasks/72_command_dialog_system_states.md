# 72 — Unify command, dialog, and system-state surfaces

## Status

Pending

## Goal

Finish the visual system across command navigation, context inspection, connection,
loading, error, cancellation, and remote-provider decisions.

## Depends on

Task 71.

## Implementation

- Retheme the command palette, context inspector, drawers, status bar, and remaining dialogs.
- Preserve file/symbol/action routing, query behavior, keyboard selection, dismissal,
  focus restoration, and Escape priority.
- Make local/remote provider destination and confirmation explicit before sending context.
- Standardize loading, disconnected, reconnecting, failed, canceled, stale, empty, and
  success copy/actions across all workspaces.
- Keep context included/excluded reasons and truncation/token information visible without
  exposing source content not intended for the UI model.
- Remove superseded one-off dialog/state styling as each surface migrates.

## Acceptance criteria

- Palette and dialogs are fully keyboard-operable with predictable focus restoration.
- Escape closes/cancels only the highest-priority active surface/request.
- Remote/local and connection states remain explicit without color.
- No dialog or status path creates an automatic project write.

## Verification

- Extend palette routing, shortcut priority, context-manifest, and status-label tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.

