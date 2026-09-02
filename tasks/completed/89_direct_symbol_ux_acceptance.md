# 89 — Complete direct-symbol Editor UX acceptance

## Status

Complete

## Goal

Verify, document, and finish the direct source-selection, contextual symbol analysis, and
guarded edit/review experience across pointer, keyboard, accessibility, and responsive
layouts.

## Depends on

Tasks 85–88.

## Required task commentary

- Before editing, post a concise update beginning with `Starting Task 89` and name the
  acceptance matrix, documentation, full validation, and any manual GUI limitations.
- After verification, post a separate update beginning with `Task 89 complete` and state
  the full UX outcome, commands/results, manual checks, and any limitation.

## Implementation

- Review the implementation against the requirements and acceptance criteria recorded
  in Tasks 85–89; fix only regressions within Tasks 85–88. The current `PLAN.md` owns
  the later Tasks 90–94 refinement and is not Task 89's completion record.
- Ensure automated coverage includes:
  - line-to-symbol hit testing and out-of-symbol clearing;
  - consistent editor/palette/finding/suggestion selection;
  - selected-symbol-only inspector content and every analysis state;
  - direct Replace editing plus command-only Create;
  - draft-retarget protection;
  - contextual progress, validation, checks, Apply, receipt, and Undo guards;
  - absence of old File analysis, all-symbol lists, stage buttons, and Continue to Apply;
  - wide, exactly-1000dp, and narrow Context behavior;
  - pointer, text-selection, keyboard, focus, selected, disabled, and screen-reader state.
- Update `desktop/README.md`, `desktop/KEYBOARD_SMOKE_CHECKLIST.md`, and
  `docs/RELEASE_ACCEPTANCE.md` to match the shipped interaction.
- Do not change the current `PLAN.md` status; it remains Proposed until Tasks 90–94
  complete. Do not rewrite historical completed task files.
- Audit for dead `EditorSurface`, old stage helpers, obsolete `SummaryPane`/symbol-list
  state, duplicate target controls, callback plumbing, generated output, credentials,
  and unrelated changes.
- Confirm no daemon, OpenAPI, persistence, configuration, or migration change was needed.

## Acceptance criteria

- Every direct-symbol requirement and acceptance criterion in Tasks 85–89 has automated
  or reproducible manual evidence.
- A pointer click inside a declaration immediately shows that declaration's explanation;
  dragging still selects source text and never edits it.
- One direct Edit action enters a bound Replace workflow and no source change occurs
  before guarded Apply.
- Create declaration remains available without cluttering the selected-symbol path.
- No active draft can be silently retargeted or discarded.
- Responsive/keyboard/accessibility behavior is usable and documented.
- No old and new Editor interaction models coexist.
- No API, configuration, persisted-data, or migration step is required.

## Verification

- Run:

  ```text
  ./desktop/gradlew -p desktop test
  make check
  git diff --check
  ```

- Manually inspect startup plus the Editor at wide, exactly 1000dp, and below 1000dp.
  Cover click/drag source behavior, long and nested declarations, symbol palette, Context
  drawer opening, every analysis state available to reproduce, direct Edit, Create,
  target-change protection, validation failure/success, checks, Apply, and Undo.
- If GUI or full validation is unavailable, record the exact limitation and do not claim
  it passed.

## Completion

After every criterion passes, set this task to Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Leave the current plan status unchanged. Do not stage or
commit; this task does not authorize either.
