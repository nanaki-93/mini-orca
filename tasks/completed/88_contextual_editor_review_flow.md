# 88 — Replace Editor navigation buttons with contextual progress and Review

## Status

Complete

## Goal

Remove persistent Editor surface/stage navigation and present one derived
Inspect → Edit → Review flow, with validation, focused checks, guarded Apply, receipt,
and Undo in the context where they are needed.

## Depends on

Task 87.

## Required task commentary

- Before editing, post a concise update beginning with `Starting Task 88` and name the
  progress projection, Verify/Apply consolidation, obsolete plumbing, and focused tests.
- After verification, post a separate update beginning with `Task 88 complete` and state
  the simplified flow delivered, tests run, and that Task 89 is next.

## Implementation

- Replace the four clickable Target/Draft/Verify/Apply buttons with one compact,
  non-interactive progress presentation for Inspect, Edit, and Review. Keep current state
  explicit in text and semantics.
- Derive the visible mode from selection plus current chat/draft/validation/check/receipt
  truth, with only the minimum transient intent needed to open the composer or return to
  a current draft.
- Keep source as the main canvas in Inspect and initial Edit. Use the existing isolated
  editable declaration draft only after a proposal exists.
- On successful draft validation, show the existing selectable read-only diff and Review
  evidence. Manual draft edits must clear old validation/checks and return to Edit.
- Combine `VerifyEvidencePane` and `ApplyDecisionPane` into one cohesive Review context:
  - current scope identity and validation evidence;
  - explicit **Run focused checks** when eligible;
  - collapsed failed/diagnostic command details;
  - read-only impact and Git context;
  - the exact **Apply `<symbol>` to `<path>`** action only when
    `draftReviewEligibility` is eligible;
  - applied receipt and guarded Undo after success.
- Remove **Continue to Apply**, the separate Apply canvas/pane, obsolete stage enum/state
  branches, callbacks, labels, tests, and shortcuts made redundant by the new flow.
- Preserve one concise way to return from Review to the current editable draft without
  weakening or bypassing validation invalidation.
- Ensure global Generate/Validate/Checks/Cancel shortcuts act only when their contextual
  action is available; unavailable hidden actions must not fire.

## Acceptance criteria

- No Source/File analysis or Target/Draft/Verify/Apply navigation button bar remains.
- Inspect, Edit, Review, and receipt state are derived from authoritative workflow state
  and cannot be advanced past guards.
- Validation opens Review without writing; failed/invalid validation stays in Edit with
  actionable diagnostics.
- Review requires explicit focused checks and exposes Apply only for the exact current
  eligible draft.
- There is no redundant Continue to Apply action or second confirmation dialog.
- Apply still names one symbol/file, performs the only source write, and produces the
  existing Undo receipt.
- Read-only source/diff, editable draft boundaries, stale clamping, and remote-provider
  rules remain intact.

## Verification

- Extend/replace `EditorFlowStateTest`, `EditorWorkspaceTest`,
  `ReviewEvidencePaneTest`, `ApplyDecisionPaneTest`, `DraftReviewWorkflowTest`, shell,
  shortcut, and integration tests.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update `tasks/INDEX.md`. Do not stage or commit; this task does not authorize either.
