# 132 — Complete IDE UI acceptance

## Status

Complete

## Goal

Remove transitional UI code, verify the full IDE-style workflow, update maintained
documentation, and close the plan with reproducible evidence.

## Depends on

Tasks 118–131.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 132` and name the
  residual audit, full automated matrix, screenshot/keyboard acceptance, documentation,
  commit-history verification, and checks.
- After the commit, post a separate update beginning with `Task 132 complete` and
  report the final UI outcome, all automated/manual results and limitations, exact
  commit hash, and that the sequence is finished.

## Implementation

- Audit the Desktop for obsolete workspace rail, file header, transient status footer,
  standalone context/review branches, duplicate findings/check presentation, unused
  callbacks, stale theme helpers, debug output, and transitional flags. Remove each
  superseded path; do not retain parallel shells.
- Verify the target shell: Project left, source/review center, Context/Assistant/Review
  right, Problems/Checks/Output bottom, compact toolbar, and persistent status bar.
- Verify one-project/one-file/one-symbol scope, read-only source/diff, explicit draft
  discard, remote confirmation, validation/check identity, exact Apply, receipt, and
  Undo behavior end to end.
- Test landing, restore, all workspaces, long paths, duplicate basenames, large files,
  nested declarations, no/stale/failed analysis, filtered problems, remote provider,
  failed/stale checks, cancellation, late responses, daemon disconnect/reconnect, and
  Apply/Undo receipt refresh.
- Run the final viewport screenshot and keyboard/accessibility matrix. If an
  interactive window or provider is unavailable, record the limitation and remaining
  release-operator check without fabricating a pass.
- Update `desktop/README.md`, `desktop/KEYBOARD_SMOKE_CHECKLIST.md`, `tasks/README.md`,
  and maintained plan/status references to describe the final UI.
- Set `plan.md` to Complete only when the implementation and automated acceptance are
  complete; place any permitted interactive limitation adjacent to the status.
- Verify Tasks 118–132 have exactly one sequential commit each with the required
  subjects and none were combined, amended, tagged, or pushed.

## Acceptance criteria

- Every success criterion in `plan.md` is satisfied or has a truthful, narrowly scoped
  environment-dependent manual limitation.
- No obsolete or duplicate Desktop shell implementation remains.
- All automated checks pass with no unexplained Detekt, formatting, test, or diff
  failure.
- Maintained documentation matches the shipped layout, shortcuts, and safety behavior.
- The final task commit contains no unrelated user work or generated output.

## Verification

Run at minimum:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
make fmt-check
go test ./...
make test-race
make vet
make check
make quality
git diff --check
```

Inspect the full Desktop production/test diff since Task 118, run the maintained
keyboard smoke checklist, validate documentation links, and record all manual results.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, set the plan status truthfully, stage only Task 132 changes, inspect
the staged diff, and create exactly one commit:

```text
chore(desktop): complete IDE UI acceptance
```

Do not amend, squash, tag, or push the commit. After committing, verify no Task 132
change remains staged or uncommitted.
