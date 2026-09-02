# 77 — Replace Analysis file results with summary and failures

## Status

Complete

## Goal

Make Analysis scan-friendly by showing aggregate coverage/run information and only the
files that failed, without per-file success navigation.

## Depends on

Task 76.

## Implementation

- Update `AnalysisWorkspacePane` to consume the Task 76 presentation model.
- Preserve the existing Analysis heading, explicit Analyze-all lifecycle controls, limit
  inputs, status text, polling-backed updates, and remote-provider confirmation.
- Present compact project-coverage and current/last-run summaries with textual labels.
- Replace `FILE RESULTS` and its all-file cards with `ANALYSIS ERRORS`:
  - render only failure rows;
  - show relative path, attempt count, and sanitized error;
  - provide an explicit no-errors empty state;
  - keep lazy rendering for a large failure set.
- Remove “View file analysis” buttons and the `onViewAnalysis` parameter.
- Remove the now-obsolete `onOpenFileAnalysis` callback chain from `DesktopShell` and
  `ContentPane`, and remove the dedicated `openFileAnalysis` function from `DesktopApp`.
- Remove imports, helpers, branches, and tests made obsolete by that route.
- Do not remove `EditorSurface.FileAnalysis`, one-file Analyze/Refresh, or any Editor
  per-file semantic content.

## Acceptance criteria

- Analysis never renders a successful, pending, or running file as an individual result.
- Every failed/error-bearing job file is visible with path, attempts, and error text.
- No file-navigation action remains in Analysis.
- Analyze-all Start/Pause/Resume/Cancel semantics and remote confirmation are unchanged.
- Per-file analysis remains available from Editor.
- Empty, running, paused, completed, canceled, stale, and failed UI states remain clear
  without relying on color.

## Verification

- Extend Analysis presentation/workspace regression tests where practical.
- Run `./desktop/gradlew -p desktop test`.
- Run `git diff --check` and inspect the task diff for obsolete callback remnants.

## Commit

After all criteria pass, move this task to `tasks/completed/`, update its index row, use
hunk-level staging because the implicated Desktop files may be pre-modified, inspect
`git diff --cached`, and create exactly this commit:

```text
feat(desktop): simplify analysis workspace results
```
