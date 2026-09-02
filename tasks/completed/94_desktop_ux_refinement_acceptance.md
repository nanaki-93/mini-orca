# 94 — Complete Desktop UX refinement acceptance

## Status

Complete

## Goal

Verify and document the active-file header, copy cleanup, conditional imports, and
priority-grouped Bugs behavior as one accessible, responsive Desktop refinement.

## Depends on

Tasks 90–93.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 94` and name the full
  acceptance matrix, documentation, full validation, and any manual GUI limitation.
- After the task commit succeeds, post a separate update beginning with
  `Task 94 complete` and state the shipped outcome, checks, manual limitations, commit
  hash, and that the sequence is finished.

## Implementation

- Review the implementation against every requirement and definition-of-done item in
  `PLAN.md`; fix only regressions within Tasks 90–93.
- Ensure automated coverage proves:
  - active basename, relative path, no-file state, source mode, and review-diff mode;
  - absence of Editor progress explanation and the “Click a…” subtitle;
  - preserved click, drag selection, keyboard selection, source/diff read-only state,
    and wide/narrow Editor behavior;
  - empty imports hidden, nonempty imports editable, and draft invalidation preserved;
  - high/medium/low/fallback priority order after filtering, stable card order, no empty
    groups, and card-level provenance.
- Update `desktop/README.md`, `desktop/KEYBOARD_SMOKE_CHECKLIST.md`, and
  `docs/RELEASE_ACCEPTANCE.md` to match the shipped UI and manual checks.
- Remove only dead presentation helpers, tests, imports, or copy made obsolete by Tasks
  90–93. Do not broaden the change into a theme, navigation, model, or backend refactor.
- Mark `PLAN.md` Complete only after all automated criteria pass and manual checks are
  either performed or recorded accurately as unavailable.
- Confirm no daemon, OpenAPI, persistence, configuration, or migration change is needed.

## Acceptance criteria

- Every `PLAN.md` definition-of-done item has automated or reproducible manual evidence.
- File identity remains clear in source and review modes at wide, exactly 1000dp, and
  below 1000dp layouts.
- Removed copy does not leave inaccessible interactions or unexplained safety gates.
- Conditional imports do not weaken editable-draft validation or identity handling.
- Priority grouping does not erase provenance, filtered-result clarity, or finding
  actions.
- No old and new presentation implementations coexist.
- No API, configuration, persisted-data, or migration step is required.

## Verification

- Run:

  ```text
  ./desktop/gradlew -p desktop test
  make check
  git diff --check
  ```

- Manually inspect the Editor source and review views at wide, exactly 1000dp, and below
  1000dp; cover long and duplicate filenames, click versus drag, keyboard navigation,
  empty/nonempty imports, and the full Bugs priority/filter/provenance presentation.
- If GUI or full validation is unavailable, record the exact limitation and do not claim
  it passed.

## Commit

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, update
its `tasks/INDEX.md` row, include the truthful `PLAN.md` status and documentation, stage
only Task 94 changes, inspect the staged diff, and create exactly this commit:

```text
test(desktop): complete UX refinement acceptance
```

Do not amend, combine, or push the commit. After committing, make no further file edits;
the final report is read-only.
