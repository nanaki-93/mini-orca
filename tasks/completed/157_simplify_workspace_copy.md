# 157 — Simplify workspace copy and information density

## Status

Complete

## Depends on

Task 156, completed and committed.

## Goal

Reduce routine narration and finish coherent presentation on project-level
workspaces without deleting useful analysis or operational information.

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

`WorkspacePanes.kt`, `AnalysisWorkspaceState.kt`, `PerformanceWorkspace.kt`,
`BugsWorkspaceState.kt`, `ProblemsToolWindow.kt`, `FindingsPresentation.kt`,
`ProjectSummaryPane.kt`, and their tests.

## Implementation

- Audit normal, empty, running, paused, canceled, partial, stale, and failed states
  in Summary, Analysis, Performance, and Bugs/Problems. Use the baseline inventory
  to avoid polishing only the happy path.
- Remove subtitles and repeated sentences that add nothing beyond the heading,
  status, counters, or next action. In Analysis retain the accepted metrics,
  progress, and controls; shorten its repetitive run and empty-error descriptions.
- Prefer one concise ordinary empty-state line, with recovery actions where useful.
  Show optional helper text contextually, not under every field.
- Standardize section headings, rows, filters, badges, button placement, and
  table/detail density with the shared theme and disclosure components.
- Keep job limits, progress/count meaning, actionable failures, provider destination,
  and required approval visible at the decision point.
- Preserve Performance's source-based/unmeasured distinction, finding provenance,
  severity, stale status, conditions/trade-offs, and verification evidence.
- Put supporting long explanations behind explicit Details when appropriate,
  preserving exact returned data and full paths. Do not remove model results,
  triage options, filtering behavior, or fabricate concise replacements.
- Update only presentation strings and composition. Do not alter job lifecycle,
  filtering/ranking policy, source targeting, provider scope, or mutation guards.

## Required legacy removal

Delete retired copy fields, duplicate labels, old card/list composition, unused
presentation helpers, and obsolete exact-sentence assertions after callers migrate.
Do not leave old verbose mode or duplicate state interpretation rules.

## Acceptance criteria

- Default project workspaces no longer repeat heading/state information in
  explanatory paragraphs; primary data and the eligible action are easy to scan.
- Full failures, findings, analysis details, and paths remain accessible.
- Verification/provenance, unmeasured Performance status, remote approval, and
  stale-state warnings remain explicit.
- Filtering, source navigation, lifecycle controls, and provider callbacks behave
  exactly as before; no new requests occur from display changes.

## Verification

Extend affected workspace/state/findings tests for compact empty/error and active
run states, full detail reachability, filtering/triage behavior, stale provenance,
and consent. Render representative wide/narrow populated and failure views; retain
Analysis layout/progress regression checks.

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
style(desktop): simplify workspace copy (task 157)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Passed `./desktop/gradlew -p desktop spotlessCheck detekt test
-PvisualOutput="$PWD/desktop/build/reports/ui-refinement/task-157"` and `git diff
--check` on 2026-09-04. Removed duplicate Analysis coverage narration and the
retired coverage summary helper; shortened Analyze-all lifecycle, failure, verified
scan, Findings/Problems, and Performance state copy; and standardized the affected
section labels. Performance now presents its destination once in the Review panel:
next to active controls while running, and through its existing approval control
when a review can be started or resumed. The source-based/unmeasured distinction,
limits, counts, scan isolation boundary, warnings, stale states, provider consent,
filtering, lifecycle controls, and full finding/performance evidence remain intact.
Focused state and visual tests prove no display callback occurs before interaction,
Preview remains an explicit local action, and detail evidence remains reachable.
Visual review passed for `performance-populated-1440.png`,
`performance-empty-800-1.3.png`, `bugs-failed-800-1.3.png`,
`problems-empty-800-1.3.png`, and the retained Analysis lifecycle captures under
`desktop/build/reports/ui-refinement/task-157/`. Native window and screen-reader
acceptance remain deferred to Tasks 159/160.
