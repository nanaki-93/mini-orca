# 60 — Replace Desktop navigation with the top bar and workspace rail

## Status

Complete

## Goal

Implement Direction A's persistent navigation structure using Direction C's visual
language and accessible text labels.

## Depends on

Task 59.

## Implementation

- Replace the current header and horizontal workspace button row with an app top bar
  and labeled Summary, Analysis, Bugs, and Editor workspace rail.
- Show Mini-Orca identity, project/revision breadcrumb, connection/locality state,
  Command, Re-index, and Open project in the top bar.
- Keep textual workspace counts and selected semantics; icons supplement labels.
- Replace emoji/ad-hoc glyphs with a small consistent Compose vector icon set and a
  code-native Mini-Orca mark.
- Preserve pointer navigation, `⌘1`–`⌘4`, `⌘Tab`, command palette access, and busy state.
- Remove the superseded horizontal workspace navigation implementation and tests in
  the same change.

## Acceptance criteria

- All four workspaces are visibly labeled, count-aware, and keyboard reachable.
- Connection and remote/local state remain understandable without color.
- Long project/model labels truncate safely without hiding primary actions.
- No old horizontal workspace navigation remains.

## Verification

- Extend navigation label, semantics, count, and shortcut tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
