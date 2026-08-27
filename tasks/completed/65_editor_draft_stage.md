# 65 — Implement the Editor Draft stage

## Status

Complete

## Goal

Turn file-scoped conversation and declaration editing into one focused Draft stage
with explicit context inspection and validation.

## Depends on

Task 64.

## Implementation

- Split the current focused action surface into bounded conversation, message composer,
  declaration/import editor, draft identity/status, context inspection, and Validate controls.
- Keep the conversation visibly bound to project revision, file hash, mode, and target symbol.
- Preserve remote-provider confirmation before sending bounded source context.
- Show Generated, Dirty, Validating, Invalid, Valid, and Stale states with text and recovery actions.
- Make `⌘K`/Focus Chat and `⌘⇧D`/Focus Draft use real `FocusRequester`s; keep `⌘Enter`,
  `⌘⇧V`, cancellation, and dialog behavior.
- Validation success unlocks Verify; any manual edit clears prior validation/check evidence.
- Delete the superseded focused-action implementation after all behavior is migrated.

## Acceptance criteria

- Send never writes source and creates only a file-scoped declaration draft.
- Draft text/imports are the only editable project-related content.
- Remote requests cannot send until confirmed.
- Dirty/invalid/stale drafts cannot advance; focus shortcuts land on the intended controls.

## Verification

- Extend file-chat, draft-editor, focus, remote-confirmation, and validation tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
