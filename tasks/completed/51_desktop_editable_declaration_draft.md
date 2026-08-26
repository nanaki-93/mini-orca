# 51 — Add editable declaration drafts to Desktop

## Status

Complete

## Goal

Let the user manually improve only the AI-proposed declaration before validation.

## Depends on

Tasks 40, 43, 44, and 50.

## Implementation

- Display the proposed function/type declaration in an editable draft control while keeping original source and diff read-only.
- Model generated, dirty, validating, valid, invalid, and stale states with draft revision/hash.
- Mark the draft dirty on any declaration/import edit and immediately clear validation, checks, comparison, and Apply eligibility.
- Add explicit Validate using the expected draft revision and replace the editor content with daemon-normalized formatted declaration on success.
- Show bounded syntax, duplicate/missing target, import, and out-of-scope diagnostics next to the draft.

## Acceptance criteria

- The user cannot edit unrelated source through the draft control.
- Old validation/check evidence never authorizes edited content.
- Invalid drafts remain editable and recoverable but cannot run checks or Apply.
- Validation is explicit and never writes project source.

## Verification

- Add state tests for manual edits, optimistic conflicts, validation success/failure, normalized content, and stale base identity.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
