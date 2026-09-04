# 153 — Correct Analysis and shared workspace widths

## Status

Complete

## Depends on

Task 152, completed and committed.

## Goal

Preserve the accepted Analysis structure while removing its narrow fixed-width
island and establishing width rules reusable by Summary.

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

`WorkspacePanes.kt`, a small shared page-layout helper if justified,
`DesktopLayoutState.kt` only if needed, and layout/Analysis/visual tests.

## Implementation

- Replace Analysis's centered 1160dp cap with the width actually available inside
  the shell. Account for the rail once, not twice.
- Use 24dp horizontal gutters on wide pages and 16dp on narrow pages. Align heading,
  metric strip, run content, controls, and errors to the same content edges.
- Let the run card take the remaining row width; bound the controls to roughly
  280–360dp at default text scale. Derive stacking from available content and
  minimum usable widths, not total screen width or arbitrary nested weights.
- Keep active controls above the run content when stacked. Long destinations,
  paths, retry/error information, and larger text must wrap/scroll without
  hiding Pause/Cancel/Resume or consent controls.
- Let metric columns reflow based on available width; avoid clipped labels or
  stretched action buttons. Bound prose inside details, not the whole dashboard.
- Extract only the small shared page gutter/width behavior needed by Summary.
  Preserve Editor's separate pane resizing, stored preferences, minimum source
  width, and the exact shell breakpoint.
- Retain existing Analysis job counts, progress formula, lifecycle, options,
  callbacks, and provider guards. Copy changes are owned by Task 157.

## Required legacy removal

Delete the old max-width branch and superseded layout constants/helpers once
replaced. Do not retain old/new layouts behind flags or duplicate width formulas.

## Acceptance criteria

- Analysis keeps its metrics/progress/control hierarchy and uses the available
  width at 1440 and 1920 without the old excessive side gutters.
- Control widths stay bounded, content edges align, and narrow/text-scaled layouts
  expose all controls without overlap or horizontal page overflow.
- Resizing does not overwrite pane preferences, reset form inputs, or trigger work.
- Shared layout code is minimal and ready for Summary without changing it early.

## Verification

Extend Analysis production-component renders and behavior-focused size assertions
at 1440×900, 1920×1080, 1000×760, 999×760, 800×650, and 130% text scale.
Cover long model destinations and active/paused/empty/error states; retain
`AnalysisWorkspaceStateTest` and `DesktopLayoutStateTest` behavior checks.

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
fix(desktop): correct Analysis workspace widths (task 153)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Passed `./desktop/gradlew -p desktop spotlessCheck detekt test -PvisualOutput="$PWD/desktop/build/reports/ui-refinement/task-153"` and `git diff --check` on 2026-09-04. Removed the centered 1160dp Analysis cap and the total-width 820dp layout branch. Shared page gutters now use 24dp on wide pages and 16dp on narrow pages; Analysis derives a 280–360dp control column and its stack threshold from usable content width and font scale, while Summary consumes the same horizontal gutter helper. Visual review passed for `analysis-1920-1.0.png`, `analysis-long-destination-1000-1.3.png`, and `analysis-paused-800-1.3.png`; renders also cover 1440, 1000, 999, 800, empty, and failed states. Native window and screen-reader checks remain unavailable in this session and are deferred to Tasks 159/160.
