# 151 — Remove duplicate Project navigation

## Status

Pending

## Depends on

Task 150, completed and committed.

## Goal

Make Editor the sole file/source workspace while preserving all real project and
file-management entry points.

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

`DesktopLayoutState.kt`, `IdeShell.kt`, `DesktopShell.kt`, `ExplorerPane.kt`,
`DesktopKeyboardNavigation.kt`, `CommandPalette.kt`, their callers/tests, and
current navigation documentation.

## Implementation

- Remove `LeftToolWindow.Project` and its rail label/icon mapping and duplicate
  navigation branch. Keep Summary, Analysis, Performance, Problems, and Editor in
  that order, with exactly one active destination.
- Change the default left selection and preference-reader fallback to Editor.
  A saved `ide-left-tool=Project` must recover through the existing unknown-enum
  mechanism; the normal save path writes Editor. Do not add a special legacy route.
- Preserve Explorer/Context widths, visibility, bottom state, current file/symbol,
  and candidate/evidence state. Keep the existing preference keys.
- Label the file-tree pane Files. Preserve the top project selector, Open project,
  Re-index project, indexed tree, filtering, reveal/collapse actions, and narrow
  Files drawer. Do not remove project-domain models or still-used folder icons.
- Check every navigation entry point: rail, shortcuts, workspace cycling, palette,
  last-project restore, status actions, and links from other screens.
- Keep existing Cmd/Ctrl+1–4 assignments and the exact 999dp/1000dp boundary.
  Opening Editor must not silently discard a draft or initiate model work.
- Update affected README/checklist text and fixture expectations in this commit.

## Required legacy removal

Delete the removed enum member, duplicate branch, obsolete label/selection
assertions, and unused helpers. Do not retain a hidden Project entry, compatibility
alias, second shell, or a blanket replacement of legitimate project-domain names.

## Acceptance criteria

- Exactly five workspace entries are visible; only Editor opens the file/source
  workspace. Project selection and Files access remain available.
- Saved Project, missing, and unknown navigation preferences load Editor safely;
  normal save/load round-trips current values and preserves unrelated preferences.
- File selection, keyboard navigation, candidate state, and drawer focus behavior
  are unchanged apart from the duplicate entry's removal.
- No active reference to the removed enum member remains.

## Verification

Update `DesktopLayoutStateTest`, `DesktopShellTest`, `DesktopKeyboardNavigationTest`,
`CommandPaletteTest`, `ExplorerPaneTest`, and affected visual fixtures where relevant.
Test old/missing/corrupt preference values, retained pane values, keyboard cycling,
and no additional callbacks or draft loss on navigation.

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
refactor(desktop): remove duplicate Project navigation (task 151)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Not run — task is Pending. Replace this paragraph during execution with actual
checks, outcomes, legacy code removed, evidence paths, and remaining limitations.
