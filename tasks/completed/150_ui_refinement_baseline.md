# 150 — Establish the UI refinement baseline

## Status

Complete

## Depends on

Task 148 and the current post-refactor desktop implementation. Task 149's outstanding
native/quality evidence is tracked separately, not a prerequisite for beginning this
new refinement sequence. Verify actual code and recorded history; do not assume
that its old visual baseline represents the current UI.

## Goal

Establish reproducible visual and behavior evidence before changing the UI, and
bring the plan/backlog into the first verified implementation commit.

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

`DesktopVisualLayoutTest.kt`, existing shell/layout/workflow tests,
`desktop/UI_REFINEMENT_BASELINE.md`, and the exact planning bootstrap artifacts
listed in the execution prompt.

## Implementation

- Read the plan and guidelines, inspect the supplied dark mock and the current
  implementation, and record the starting HEAD and available verification tools.
  Do not store user diffs, credentials, project source, or machine-specific secrets.
- Create `desktop/UI_REFINEMENT_BASELINE.md` with the navigation map, current token
  roles, Analysis width rules, Summary data sources, menu/disclosure inventory,
  current copy problems, and preserved provider/draft/Apply/Undo boundaries.
- Inventory every retained workspace and transient surface. Assign each to Tasks
  151–159, including landing, empty/error states, filters, status details, and dialogs.
- Extend the existing production-component fixture harness only where needed to
  capture Summary and representative open dropdown/disclosure states. Keep sample
  data explicitly test-only and never install a fixture in normal app startup.
- Capture current Analysis and Editor plus Summary and representative menus before
  replacement. Use reproducible dimensions and distinguish offscreen component
  images from native captures. Keep generated output under ignored build reports;
  record commands and conclusions in versioned Markdown.
- Reuse current behavior tests. Add missing characterization coverage for no-action
  navigation/disclosure, preference recovery, and selected-file/draft preservation.
  Do not encode the duplicate Project entry or verbose text as required behavior.
- Run repository checks to establish current failures as observations, not permission
  to alter Go code. Record exact commands, failing gates/functions, and whether the
  files are outside this sequence's scope; never rely only on the old Task 149 report.
- Inspect and include only the prompt's approved bootstrap artifacts in this task's
  commit. Mark this plan In Progress; leave Tasks 151–160 Pending. No production
  navigation, palette, dependency, or workflow changes belong in this task.

## Required legacy removal

Remove obsolete fixture duplication if replaced. Do not delete historical task
records or invent future production scaffolding merely to prepare later tasks.

## Acceptance criteria

- The plan, 11 task files, index, workflow, and execution prompt agree on scope,
  dependencies, exact commit subjects, and one-commit-per-task ownership.
- The baseline is reproducible and covers the actual current implementation,
  including before-images for the user's affected screens.
- Existing behavior remains unchanged; missing native evidence is explicitly
  marked unavailable and old Task 149 remains truthful.
- Required desktop checks pass; repository-only failures are independently
  identified and recorded for the final comparison.
- User-owned changes are excluded except for the explicitly reviewed bootstrap
  artifacts belonging to this requested plan.

## Verification

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` with
`-PvisualOutput="$PWD/desktop/build/reports/ui-refinement/before"`.
Run `make check` and `make quality` as baseline diagnostics. Desktop or newly
introduced failures block this task; record verified pre-existing non-desktop
failures without expanding scope. Check documentation links and task consistency.

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
test(desktop): establish UI refinement baseline (task 150)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Completed on the recorded baseline `991b916e24dff8048e68eb99a5a37d8a20ceb816`.
`./desktop/gradlew -p desktop spotlessCheck detekt test -PvisualOutput="$PWD/desktop/build/reports/ui-refinement/before"` passed, including
production-component captures for Analysis, Editor, Summary, and expanded Engineering insight,
plus semantic interaction coverage for the actual open Project and Preview menus. `make check`
passed. `make quality` failed at its pre-existing out-of-scope Go `gocyclo -over 15` gate on
`reviewPerformanceFile`, `validPerformanceJob`, `validPerformanceFinding`,
`StartPerformanceJob`, and `StartAnalyzeAll`; no Go changes were made and the later clone check
did not run. `git diff --check` passed. `desktop/UI_REFINEMENT_BASELINE.md` records the capture
provenance, current design/behavior inventory, safety boundaries, and the precise native/popup
raster limitations. The fixture uses explicit no-backend data and does not add a production
renderer; no obsolete fixture duplicate was present to remove. No native Mini-Orca application or
screen reader was available, so those checks remain release follow-ups and Task 149 remains
Pending.
