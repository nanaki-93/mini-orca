# 57 — Define the Editor flow presentation contract

## Status

Complete

## Goal

Derive the Target → Draft → Verify → Apply UI flow from existing guarded state
without duplicating project, file, draft, validation, or check truth.

## Depends on

Task 56.

## Implementation

- Add pure `EditorStage`, `EditorFlowUiState`, stage-unlock, recommended-stage,
  and active-stage-clamping types/helpers under the Desktop package.
- Derive Target availability from the selected file and `validateChatTarget`.
- Derive Draft/Verify/Apply availability from the bound session, latest editable
  draft, validation identity, checks identity, and `draftReviewEligibility`.
- Represent locked/disabled reasons as display-ready text; do not infer them in
  multiple composables.
- Characterize dirty, validating, invalid, stale, checked, applied, and undone
  transitions with focused tests before changing production layout.
- Keep the new types presentation-only; do not add daemon fields or a second
  workflow state machine.

## Acceptance criteria

- Target is always reachable; later stages unlock only from current guarded state.
- Editing, stale file/project identity, or stale checks immediately relock later stages.
- Active-stage clamping never leaves the UI on a now-locked stage.
- Existing chat/draft/apply behavior remains unchanged.

## Verification

- Run focused Editor-flow state tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
