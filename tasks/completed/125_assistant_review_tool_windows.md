# 125 — Add Assistant and Review tool windows

## Status

Complete

## Goal

Place conversation, draft editing, evidence, Apply, receipt, and Undo behind stable
Assistant and Review tabs while preserving every guarded workflow rule.

## Depends on

Task 124.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 125` and name the
  Assistant/Review states, workflow badges, identity guards, likely files, and checks.
- After the commit, post a separate update beginning with `Task 125 complete` and
  report workflow placement, guard preservation, tests/results, exact commit hash,
  and that Task 126 is next.

## Implementation

- Move the bound conversation, request composer, generated declaration/import editor,
  and validation diagnostics into the Assistant tab.
- Move scope identity, validation evidence, focused-check evidence, exact Apply
  action, receipt, and Undo into the Review tab.
- Pin the active project-relative file and symbol/new-declaration target at the top of
  both tabs.
- Derive textual tab badges for Draft, Invalid, Checks failed, Ready to apply, and
  Applied. Badges summarize state and never authorize an action.
- Make stage transitions discoverable without automatically changing the active
  tool-window tab or stealing input focus.
- Preserve explicit remote confirmation, Generate/Cancel, manual draft invalidation,
  validation, checks, exact Apply eligibility, receipt, and Undo behavior.
- Preserve the explicit discard decision before changing an active draft target.
- Keep the editor's Review surface focused on the diff; keep evidence/actions in the
  right Review tab without duplicating business rules.
- Remove superseded conditional right-pane replacement code in the same change.

## Acceptance criteria

- Current file/target scope is visible throughout conversation, draft, and review.
- Manual draft changes still invalidate validation and check evidence.
- Apply remains unavailable until current validation, checks, and identity pass, and
  remains an exact user action.
- Opening or changing tabs never sends, validates, checks, applies, or undoes work.
- Late/canceled presenter results cannot switch tabs or overwrite the active target.
- Existing presenter, draft, review, Apply, and Undo tests pass with focused tab-state
  coverage added.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
make check
git diff --check
```

Use `make check` because this task touches the complete source-safety presentation
boundary even though no daemon change is intended.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 125 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(desktop): organize Assistant and Review tools
```

Do not amend, squash, tag, or push the commit.
