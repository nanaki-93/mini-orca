# 87 — Open guarded editing directly from the selected symbol

## Status

Complete

## Goal

Let an eligible declaration selected in source enter the existing Replace workflow
through one direct, target-naming edit action. Move Create declaration out of the common
path without removing it.

## Depends on

Task 86.

## Required task commentary

- Before editing, post a concise update beginning with `Starting Task 87` and name the
  direct Edit route, create command, draft-retarget protection, and focused workflow
  tests.
- After verification, post a separate update beginning with `Task 87 complete` and state
  the editing route delivered, tests run, and that Task 88 is next.

## Implementation

- Add one primary **Edit `<symbol>`** action to an eligible selected-symbol inspector.
- The action must set existing `ReplaceSymbol` intent, keep the selected file/symbol,
  open the edit composer, and focus the message field. It must not send a request,
  generate a draft, or write a file.
- Remove the persistent Replace/Create controls from the Editor context panel.
- Add **Create declaration** to the existing command palette/action route. Only that
  route shows the new-name field and uses `CreateSymbol`.
- Preserve prepared suggestion and finding flows: they select their exact target, prepare
  message text, and still require explicit Send.
- Derive the current edit target from the existing session/draft. If the user requests
  editing a different selected symbol while any current draft exists:
  - do not retarget or clear it automatically;
  - show a concise explicit decision naming both targets;
  - cancel keeps the existing draft;
  - confirm discards only the in-memory chat/draft/check state, then opens the new target.
- Returning selection to the current draft's bound symbol must restore its current edit
  or review route without data loss.
- Keep unsupported-language, approximate, and non-atomic symbols inspectable but make the
  Edit action unavailable with a concise reason.
- Preserve remote-provider confirmation, cancellation, chat session matching, and every
  request/revision/hash guard.

## Acceptance criteria

- Selecting an eligible Go function and pressing Edit opens its message composer with
  Replace implied and no intermediate target picker.
- Edit alone performs no API call or mutation; Send remains explicit.
- Create declaration is still keyboard/pointer reachable through Commands and validates
  the new name as today.
- Existing draft content is never discarded or retargeted by source inspection.
- A target change with a draft requires an explicit destructive decision and keeps the
  draft on cancel.
- Suggestions/findings, chat reuse, draft loading, cancellation, and remote confirmation
  retain their current behavior.

## Verification

- Extend `FileChatStateTest`, `DesktopStateTest`, `DesktopWorkflowControllerTest`,
  `DesktopIntegrationCoverageTest`, and command/accessibility tests as appropriate.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update `tasks/INDEX.md`. Do not stage or commit; this task does not authorize either.
