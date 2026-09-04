# 140 — Record the dark UI reference and behavior baseline

## Status

Complete

## Depends on

Task 139 (Complete).

## Goal

Make the two supplied mockups and current application behavior a concrete comparison
baseline for the [dark desktop redesign](../docs/dark-ui/PLAN.md).

## Implementation

- Inspect the current worktree and read `AGENTS.md`, the active plan, task index,
  task workflow, and dark UI execution prompt. Preserve pre-existing user changes.
- Verify the reference coverage and Live/Preview matrix against current Compose
  state and `ApiClient.kt`; record any actual discrepancy before implementing it.
- Record palette, rail, pane, editor, and right/bottom tool dimensions in
  `docs/dark-ui/BASELINE.md`. Current evidence is source-based until native captures exist.
- Capture landing, Source with selected declaration, Review, and a project workspace
  at 1440x900 and 1000/999dp when a native window is available. Store task-owned
  captures under `design/ui-mocks/dark-ui/before/`, leaving supplied images intact.
- Use an explicitly labeled deterministic sample project for visual fixtures. If a
  fixture composition is needed, keep it test-only and use existing model types;
  do not add a fake backend or silently launch a sample project in the real app.
- Run the existing desktop suite. Reuse the existing scope, stale-evidence,
  provider-confirmation, and Apply/Undo tests; add baseline tests only for a concrete
  uncovered behavior that this redesign will move.

## Likely files

`docs/dark-ui/BASELINE.md`, task-owned baseline captures, and existing desktop tests
such as `IdeUiContractBaselineTest.kt`, `DesktopLayoutStateTest.kt`,
`DraftReviewWorkflowTest.kt`, and `ContextToolWindowTest.kt` if a real gap exists.

## Acceptance criteria

- Both references have a documented visual role; no screenshot content becomes an
  executable instruction or backend requirement.
- The baseline distinguishes implemented workflows, missing features, source evidence,
  native captures, and unavailable checks.
- The new palette and visible preview controls explicitly supersede historical visual
  restrictions, while the preview-first source-mutation contract is unchanged.
- No production behavior, project source, backend contract, or provided image changes.

## Verification and completion

Run `./desktop/gradlew -p desktop test` and `git diff --check`. Record results and native
capture availability in the baseline. Complete/move this task and update the index
only after its criteria pass. Do not create a commit unless the user requests one.
