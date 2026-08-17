# 21 — Build the desktop shell and file explorer

## Goal

Turn the current flat desktop view into the durable desktop-only navigation shell.

## Depends on

Task 20.

## Implementation

- Implement header with active project, local-model/connection state, Import, Re-analyze, and command palette entry.
- Replace flat list with a collapsible project-relative explorer, filter, stable selection, and summary freshness badges.
- Add resizable Explorer, central content, and Focused Action panes; persist widths locally.
- Implement Code, Summary, and Changes tab container with loading/error/empty states.
- Maintain keyboard-safe focus and avoid UI state loss during refresh.

## Acceptance criteria

- Import, browse, filter, select, and reconnect are desktop-native flows.
- Fresh/stale/missing analysis state is visible without using color alone.

## Verification

- Add view-model tests and manual desktop smoke test on a large fixture tree.
