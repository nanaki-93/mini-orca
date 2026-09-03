# 123 — Improve the source gutter and viewport

## Status

Complete

## Goal

Make the read-only source canvas behave like a focused IDE viewer, with a dedicated
gutter, useful markers, and acceptable large-file performance.

## Depends on

Task 122.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 123` and name the source
  row model, gutter markers, selection constraints, performance evidence, likely
  files, and checks.
- After the commit, post a separate update beginning with `Task 123 complete` and
  report source behavior, performance choice, tests/results, exact commit hash, and
  that Task 124 is next.

## Implementation

- Separate gutter presentation from source text while keeping line numbers aligned
  during vertical and horizontal scrolling.
- Add non-mutating gutter markers for the focused line, selected declaration, and
  known findings in the active file. Use current findings data; do not add an API.
- Give every marker a tooltip/semantic description containing its meaning and line.
- Preserve the exact nested-declaration selection rule, click-outside-declaration
  clearing behavior, and drag-to-select source behavior.
- Measure the existing source rendering with a representative large indexed file.
  Improve recomposition/scroll performance with the simplest compatible approach,
  such as cached line presentation or viewport virtualization, without breaking
  multi-line text selection. Document the chosen trade-off in code/tests only where
  it is not self-evident.
- Retain lightweight syntax colors, monospace source, horizontal scrolling, and
  bring-focused-line-into-view behavior.
- Do not add breakpoints, quick-fix execution, code folding, editing, or a parser in
  the Desktop client.

## Acceptance criteria

- Gutter and source rows remain aligned for long, empty, and indented lines.
- Finding markers navigate/select only; they never prepare a fix, call a provider, or
  write source.
- Text dragging remains selection, not symbol navigation.
- A representative large file scrolls and changes focused line without avoidable
  whole-file presentation work; the evidence and trade-off are documented truthfully.
- Existing source selection and accessibility tests continue to pass.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add tests for marker derivation, accessible descriptions, line alignment inputs,
nested declarations, selection versus drag, and finding navigation.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 123 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): improve source gutter and viewport
```

Do not amend, squash, tag, or push the commit.
