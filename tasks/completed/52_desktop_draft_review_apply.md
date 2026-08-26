# 52 — Integrate draft review, checks, Apply, and undo

## Status

Complete

## Goal

Present one coherent Validate → Checks → Apply review for the latest editable draft.

## Depends on

Tasks 40 and 51.

## Implementation

- Combine draft identity, read-only composed diff, scope diagnostics, focused checks, impact, and Git status in one review flow.
- Require explicit Validate, then required checks, then an Apply confirmation naming the file and symbol.
- Explain every disabled Apply condition using current draft/project/base hashes and validation/check state.
- Refresh source, index revision, findings, analysis freshness, and activity after Apply or undo.
- Preserve discard, request revision, alternate proposal, comparison, source-free export, audit, and revision-guarded undo as secondary actions.

## Acceptance criteria

- Dirty, invalid, unchecked, stale, or mismatched drafts cannot apply.
- Apply changes only the bound open file and source refresh displays the result.
- Manual edits after checks disable Apply immediately.
- Undo remains conflict-safe and does not reuse stale draft evidence.

## Verification

- Add controller/eligibility tests and one fake-transport workflow test from edited draft through Apply and undo.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
