# 160 — Complete UI refinement acceptance and cleanup

## Status

Pending

## Depends on

Task 159, completed and committed. Validate the complete Task 150–159 commit
sequence before final acceptance.

## Goal

Close the plan with audited UI/behavior evidence, no active legacy replacement
code, synchronized documentation, and one final verified task commit.

## Execution contract

Follow `desktop/UI_REFINEMENT_PLAN.md`, `desktop/UI_DESIGN_GUIDELINES.md`, and
`tasks/PROMPT_EXECUTE_UI_REFINEMENT.md`. Paths in this paragraph are repository-root
relative. This task runs only after explicit execution is requested. Use one
implementation agent, finish and commit this task before starting the next, preserve
user-owned changes, and keep the preview-first workflow intact.

## Likely files

Production/test filenames without a directory are under the corresponding
`desktop/src/main/kotlin/io/miniorca/desktop/` or
`desktop/src/test/kotlin/io/miniorca/desktop/` directory.

`desktop/UI_REFINEMENT_ACCEPTANCE.md`, `UI_REFINEMENT_PLAN.md`, `README.md`,
`UI_DESIGN_GUIDELINES.md` only for verified decisions, visual/keyboard documentation,
`tasks/INDEX.md`, `tasks/README.md`, and in-scope cleanup with tests if needed.

## Implementation

- Audit every requirement against Tasks 151–159 and their evidence: duplicate
  Project removal, shorter UI copy, Summary hierarchy, Analysis width, menus/
  disclosures, global consistency, and replacement cleanup.
- Review the full sequence diff from the recorded baseline, not only the last task.
  Remove dead imports/styles/helpers, superseded branches and documentation,
  duplicate rules, debug output, and unused dependencies attributable to this work.
- Re-run full desktop validation and `make check`; compare `make quality` with the
  exact baseline. New failures or desktop failures block completion. Only an
  independently verified unchanged non-desktop baseline failure may remain as a
  clearly recorded repository-quality limitation; never call that gate passed.
- Reconfirm preference recovery, unique navigation, no incidental provider requests,
  isolated Preview behavior, read-only source/diff, draft evidence, and guarded
  Apply/Undo. Keep real project data and configuration untouched.
- Finish `desktop/UI_REFINEMENT_ACCEPTANCE.md` with requirement/evidence rows,
  command results, before/after conclusions, component-library decision, removed
  legacy pieces, and any explicit native/repository follow-ups.
- Update current README/checklists and plan status to the actually delivered state.
  If automated/component acceptance is complete but native checks are unavailable,
  place that limitation beside the completion claim; do not claim full native or
  release acceptance.
- Task 149 remains a separate outstanding historical acceptance record. Link
  relevant new evidence without silently marking its stricter criteria Complete
  or creating an extra Task 149 commit in this sequence.
- Verify all 11 tasks have one consistent record, index state, and required local
  commit. The final task's own hash is reported after commit, not embedded in itself.

## Required legacy removal

No active legacy shell/theme/menu/Summary implementation or Project navigation
alias may remain. Preserve historical completed records as history, not active
instructions. Cleanup is scoped to this UI sequence; do not refactor unrelated Go
complexity findings or delete user files/build directories to obtain a green gate.

## Acceptance criteria

- Every plan requirement has implementation, tests, reviewed visual evidence, and
  a clearly stated outcome. No known in-scope UI/safety regression remains.
- Required desktop and `make check` gates pass. `make quality` passes or only the
  precisely verified unchanged out-of-scope baseline failure remains documented.
- Native limitations are explicit release follow-ups, not fabricated passes;
  Task 149's independent status is not silently changed.
- All current docs describe one replacement implementation; existing pane
  preferences recover without manual migration and no API/config migration exists.
- Task records/index are synchronized and each completed task has exactly its
  required isolated local commit. No generated output or user-owned work is staged.

## Verification

Run `./desktop/gradlew -p desktop spotlessCheck detekt test`, `make check`,
`make quality`, and `git diff --check`. Verify documentation links, no duplicate
pending/completed records, and the sequence's exact commit subjects. Do not run
Docker cleanup or destructive make targets.

For every task, run `./desktop/gradlew -p desktop spotlessCheck detekt test` and
`git diff --check` before committing; checks already included above need not run
twice. Add behavior-focused tests where coverage is missing. Record actual results
below, including visual evidence and native checks deferred to Task 159/160.
Required automated failures block the task subject only to the explicitly stated
repository-baseline exception in Tasks 150 and 160.

## Commit

After acceptance passes, prepare Complete status, move this file to
`tasks/completed/`, and update its index link/status in the same isolated commit.
Follow the prompt's staging/review protocol. Use exactly this subject:

```text
chore(desktop): complete UI refinement acceptance (task 160)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Not run — task is Pending. Replace this paragraph during execution with actual
checks, outcomes, legacy code removed, evidence paths, and remaining limitations.
