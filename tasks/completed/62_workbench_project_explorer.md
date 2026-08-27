# 62 — Redesign the Workbench project explorer

## Status

Complete

## Goal

Apply the selected dense Workbench treatment to the project tree without losing
large-project behavior, textual state, or safe relative paths.

## Depends on

Task 61.

## Implementation

- Restyle filter, project root, directory disclosure, file rows, active selection,
  language/icon treatment, and freshness status using shared theme primitives.
- Keep indexed relative paths only; never expose absolute project paths.
- Preserve lazy rendering, filter branch retention, collapse/expand behavior, stable
  selected paths, and loading/empty results.
- Keep Fresh, Stale, Failed, Analyzing, and Not analyzed understandable in visible
  text/semantics; do not replace them with color-only dots.
- Add predictable keyboard focus and activation for filter, folders, and files.
- Keep drawer selection behavior from Task 61.

## Acceptance criteria

- A 2,000-file filtered tree remains stable and responsive.
- Filtering retains matching ancestor branches and the selected file identity.
- File freshness and directory/file roles are exposed textually.
- Explorer works identically in wide and drawer layouts.

## Verification

- Extend explorer grouping/filter/collapse/selection/accessibility tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
