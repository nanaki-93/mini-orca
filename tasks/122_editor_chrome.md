# 122 — Add active-file editor chrome

## Status

Pending

## Goal

Give the source-first center area a stable active-file tab, breadcrumbs, explicit
Source/Review surfaces, and unambiguous read-only state.

## Depends on

Task 121.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 122` and name the
  active-file header, breadcrumbs, Source/Review state, focus rules, likely files, and
  checks.
- After the commit, post a separate update beginning with `Task 122 complete` and
  report editor behavior, workflow preservation, tests/results, exact commit hash, and
  that Task 123 is next.

## Implementation

- Replace the current file heading with one active-file tab that shows basename,
  project-relative path context, draft-dirty/current-review indicators, and a visible
  `READ-ONLY` label.
- Add project-relative breadcrumbs for the active file and selected declaration.
  Collapse long paths from the middle while retaining the complete accessible label.
- Add explicit `Source` and `Review` editor surfaces when a current validated draft is
  available. Keep one active file; do not introduce multi-open or editable file tabs.
- Do not automatically replace Source or steal focus when validation finishes. Signal
  review readiness and let the user intentionally open Review.
- Preserve the current selected file, selected declaration, focused line, draft, and
  right-tool-window state when switching Source/Review.
- Keep the composed diff read-only and retain side-by-side/unified controls.
- Remove superseded `EditorFileHeader` and automatic review-switching branches once the
  new editor chrome is authoritative.

## Acceptance criteria

- The active project-relative file and read-only status are always clear.
- Review readiness is visible without unexpectedly changing the editor surface.
- Source/Review switching cannot alter draft identity, validation, checks, or Apply
  eligibility.
- No multi-file or source-editing capability is implied or introduced.
- Breadcrumbs, long paths, empty-file state, and duplicate basenames have tested
  presentations and semantics.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Exercise the source-to-review-to-source flow in the smoke checklist when an interactive
Desktop is available.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 122 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): add active file editor chrome
```

Do not amend, squash, tag, or push the commit.
