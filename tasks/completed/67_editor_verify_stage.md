# 67 — Implement the Editor Verify stage

## Status

Complete

## Goal

Present the composed candidate and all current evidence in one deliberate verification step.

## Depends on

Task 66.

## Implementation

- Use the shared diff viewer as the Verify canvas, defaulting to side-by-side with unified available.
- Present validation identity/diagnostics, required focused checks, timing/state, and collapsed
  command output in the contextual evidence panel.
- Keep advisory impact and Git branch/file/diff state visibly read-only.
- Explain missing, running, failed, stale, skipped, and passed evidence with text.
- Run checks only on explicit user action and bind results to the exact draft revision/hash.
- Enable Continue to Apply only when `draftReviewEligibility` is eligible.
- Replace the verification portion of the old draft review pane and remove it there.

## Acceptance criteria

- Validated but unchecked drafts remain in Verify with an explicit disabled reason.
- Failed/stale checks cannot unlock Apply.
- Editing after checks immediately invalidates the evidence and returns to Draft.
- Impact/Git context cannot trigger project mutations.

## Verification

- Extend eligibility, check identity, diagnostics, impact, and Git presentation tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
