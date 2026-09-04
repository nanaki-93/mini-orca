# 144 — Reorganize AI Context and restyle project workspaces

## Status

Complete

## Depends on

Task 143.

## Goal

Bring the dark reference's summary, focused analysis, and quick actions into the right
pane, with the second reference's small, clearly separated interpretation sections.

## Implementation

- Keep right tabs AI Context, Assistant, Review; restyle their selected state and badges.
- Reorganize AI Context into Project summary, Focused analysis, and Quick actions.
  Pass the existing project/overview data through the narrow UI-state boundary;
  do not issue new requests just to populate a card.
- Reuse project facts/interpretation and cached file/declaration explanation, risks,
  dependencies, Git context, and impact summaries. Keep provenance and freshness.
- Wire Show more/View full analysis to existing workspaces. Explain reveals cached
  explanation or offers explicit analysis; Find potential bugs uses the current
  analysis route; Refactor opens/prefills the existing eligible-target composer.
- Restyle Summary, Analysis, Bugs, Performance, and Engineering insight with the
  same flat sections, readable controls, and restrained cards.
- Keep structured complexity/readability/dependency-freshness ratings and dedicated
  test generation for Task 147's Preview presentation. Missing real data remains empty.

## Likely files

`ContextToolWindow.kt`, `DesktopApp.kt`, `ProjectSummaryPane.kt`, `WorkspacePanes.kt`,
`PerformanceWorkspace.kt`, `EngineeringInsightPanel.kt`, and existing presentation tests.

## Acceptance criteria

- Right-pane hierarchy is recognizable without duplicating global workspace navigation.
- Displayed project facts and analysis belong to the current project/file/symbol;
  stale or absent data is labeled accurately and never replaced with sample content.
- Opening cards, navigating, or preparing a refactor sends no model request by itself.
- Explicit analysis retains the correct provider destination/confirmation and cancellation.
- Performance remains available and explicitly unmeasured; Engineering insight retains
  close/reopen behavior, ownership, and AI-interpretation labels.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use `ContextToolWindowTest`, `ProjectSummaryPaneTest`, `AnalysisWorkspaceStateTest`,
`BugsWorkspaceStateTest`, and existing performance/integration coverage. Add callback
tests for quick-action routing, missing explanations, and ineligible refactor targets.
Complete/move the task and update the index; no unrequested commit.
