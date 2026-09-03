# 121 — Rebuild Project as a navigation tool window

## Status

Pending

## Goal

Turn the indexed Explorer into a compact, keyboard-accessible Project tool window
without adding filesystem mutation capabilities.

## Depends on

Task 120.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 121` and name the tree
  model, keyboard behavior, active-file synchronization, likely files, and checks.
- After the commit, post a separate update beginning with `Task 121 complete` and
  report navigation behavior, read-only boundaries, tests/results, exact commit hash,
  and that Task 122 is next.

## Implementation

- Restyle `ExplorerPane` as a flat, dense Project tool window with a compact header,
  filter, collapse-all action, and clear selected-file row.
- Retain directories-first ordering, project-relative paths, language labels, analysis
  freshness, empty/loading states, and duplicate-basename disambiguation.
- Add keyboard tree navigation for moving between visible rows, expanding/collapsing
  directories, and opening the selected indexed file.
- Add “Select active file” behavior that clears an incompatible filter if necessary,
  expands ancestors, and returns focus/selection to the open file.
- Keep expansion state local to the current project and persist only if it can be done
  without stale cross-project state.
- Ensure single-click/keyboard selection opens only indexed files and never creates,
  renames, moves, deletes, or writes filesystem content.
- Remove the old card-based Explorer presentation once the replacement is connected.

## Acceptance criteria

- A user can reach any visible indexed file without a mouse.
- The active file can be located after filtering, opening a problem, or using the file
  command dialog.
- Long paths and duplicate basenames remain unambiguous and accessible.
- Project navigation cannot mutate source or start a model request.
- Empty, loading, filtered-empty, and stale-analysis states remain text-labeled.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add focused tests for tree flattening, collapse/expand, filtering, active-file reveal,
keyboard transitions, and safe indexed selection.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 121 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): add Project tool window
```

Do not amend, squash, tag, or push the commit.
