# 79 — Complete Analysis and Editor UX acceptance

## Status

Pending

## Goal

Prove and document the simplified Analysis workspace and Editor-only file navigation
without weakening Mini-Orca’s preview-first safety or responsive accessibility.

## Depends on

Tasks 75–78.

## Implementation

- Review the finished UX against every requirement and definition-of-done item in
  `PLAN.md`; fix only regressions within the approved scope.
- Ensure automated coverage includes:
  - coverage and last-run summary counts;
  - failures-only rows and no-errors state;
  - absence of Analysis file-result navigation;
  - wide project-level layout without Editor side panes;
  - wide and narrow Editor chrome;
  - global palette and explorer navigation into Editor;
  - file/Editor state retention across workspace changes.
- Update `desktop/README.md`, `docs/RELEASE_ACCEPTANCE.md`, and
  `desktop/KEYBOARD_SMOKE_CHECKLIST.md` where current wording still describes per-file
  Analysis results or always-visible side panes.
- Confirm no OpenAPI, daemon, persisted-data, configuration, or migration change is
  needed.
- Inspect for dead callbacks/imports, duplicated status classification, generated output,
  credentials, and unrelated changes.

## Acceptance criteria

- Every product and verification item in `PLAN.md` section 8 has automated or
  reproducible evidence; the task/commit-history item is satisfied by this final commit.
- The documented workspace responsibilities match the shipped Compose behavior.
- Analyze-all remains explicit, revision-bound, cancellable, and remotely confirmed.
- Source/diff remain read-only and Apply remains one-file, preview-first, and guarded.
- Tasks 75–78 are Complete and their four required commits exclude pre-existing unrelated
  changes; Task 79 completion metadata is ready for its final commit.
- Completing this task will leave Tasks 75–79 under `tasks/completed/` with five required
  commits and truthful index links.
- No configuration or data migration is required.

## Verification

- Run `./desktop/gradlew -p desktop test`.
- Run `make check` when the environment permits.
- Run `git diff --check`.
- Perform or document the manual keyboard/responsive smoke flow at wide, exactly 1000dp,
  and below 1000dp, including `Cmd/Ctrl+P` from a non-Editor workspace.
- Review the existing Task 75–78 commits for scope and message accuracy; the sequential
  prompt performs the final five-commit review after this task is committed.

## Commit

After all criteria pass, move this task to `tasks/completed/`, update its index row, stage
only Task 79 tests/documentation and status metadata, inspect `git diff --cached`, and
create exactly this commit:

```text
test(desktop): verify analysis and editor workspace UX
```
