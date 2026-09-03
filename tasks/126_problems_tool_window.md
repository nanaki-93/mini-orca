# 126 — Add the Problems tool window

## Status

Pending

## Goal

Make verified findings and AI suggestions available beside the active source through
a bottom Problems tool window, without turning selection into an automatic fix.

## Depends on

Task 125.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 126` and name the
  Problems model, grouping/filter reuse, navigation boundary, likely files, and
  checks.
- After the commit, post a separate update beginning with `Task 126 complete` and
  report problem navigation, mutation safeguards, tests/results, exact commit hash,
  and that Task 127 is next.

## Implementation

- Connect the bottom tool-window region to a Problems tab using the existing finding
  classification, priority, filtering, lifecycle, and provenance rules.
- Present compact problem rows grouped high, medium, low, then other, with visible
  severity, status, provenance, path, line, and summary.
- Reuse one findings presentation model for the bottom tool and the retained detailed
  Bugs workspace; do not duplicate filtering or lifecycle policy.
- Selecting a problem must open its indexed file, focus its line, and update Context.
- Keep `Prepare fix` as a separate explicit action that continues through the current
  target/task guard. Plain selection must never call the model or create a draft.
- Show a collapsed text summary with total and highest actionable severity.
- Preserve advanced filters, triage actions, empty states, and keyboard focus.
- Keep project-relative path validation at the existing boundary.

## Acceptance criteria

- Problems can be inspected while source remains visible.
- Problem selection performs navigation only and cannot mutate source or start AI.
- `Prepare fix` retains eligibility checks and explicit intent.
- Bottom and detailed Bugs views produce consistent ordering, filters, lifecycle
  actions, and provenance labels.
- Collapsed, empty, loading, filtered, and populated states are understandable without
  color.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Add focused tests for shared presentation, collapsed summaries, problem navigation,
keyboard selection, and the no-automatic-fix boundary.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 126 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): add Problems tool window
```

Do not amend, squash, tag, or push the commit.
