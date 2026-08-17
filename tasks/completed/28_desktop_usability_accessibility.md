# 28 — Polish desktop usability and accessibility

## Status

Complete

## Goal

Make the focused workflow fast, legible, and usable without a mouse.

## Depends on

Tasks 21–24.

## Implementation

- Implement command palette and shortcuts: file, symbol, action, generate, cancel, and tab navigation.
- Add syntax highlighting for read-only source/diff views before attempting full editor support.
- Add skeleton/loading rows, progress, empty/error states, and narrow-window drawers.
- Apply visual tokens consistently; use text/icons as well as color for freshness, validation, and severity.
- Audit keyboard traversal, accessible names, focus restoration, contrast, scale, and screen-reader semantics where Compose supports them.

## Acceptance criteria

- Core import-to-preview flow works with keyboard navigation.
- No state is conveyed only by color and no model operation blocks the whole window without cancellation.

## Verification

- Run manual accessibility checklist and desktop regression smoke tests.
