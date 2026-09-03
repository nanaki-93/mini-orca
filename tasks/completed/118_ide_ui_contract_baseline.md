# 118 — Establish the IDE UI contract baseline

## Status

Complete

## Goal

Turn the approved IDE-style UI plan into an executable Desktop contract, record the
current baseline, and protect Mini-Orca's safety and accessibility behavior before the
shell changes.

## Depends on

Task 117.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 118` and name the
  baseline states, contract tests, documentation, planning artifacts, and checks.
- After the commit, post a separate update beginning with `Task 118 complete` and
  report the baseline evidence, tests/results, exact commit hash, and that Task 119 is
  next.

## Implementation

- Read `plan.md`, inventory the current landing, Summary, Analysis, Bugs, source,
  drafting, review, Apply, receipt, and Undo states, and record the relevant current
  dimensions and responsive behavior.
- Extend `desktop/KEYBOARD_SMOKE_CHECKLIST.md` with a baseline screenshot and viewport
  matrix for approximately `1440x900`, `1100x760`, exactly `1000dp`, and below
  `1000dp`. Capture screenshots when an interactive Desktop is available; otherwise
  record that limitation without claiming a pass.
- Add or consolidate pure Desktop tests for the non-negotiable UI contract: landing
  isolation, one active project/file/symbol, read-only source/diff, explicit
  draft-discard, current validation/check evidence before Apply, provider confirmation,
  keyboard access, and `<1000dp` drawers.
- Do not add screenshot-test dependencies or build a second preview application only
  to manufacture baseline images.
- Confirm the plan, Tasks 118–132, task index, task workflow, and sequential execution
  prompt agree on scope, order, commit subjects, and final acceptance.
- This task owns the initial uncommitted `plan.md` and Tasks 118–132 planning artifacts.
  If Git still records the historical name as `PLAN.md` on a case-insensitive
  filesystem, include the safe case-only rename to `plan.md` in this task's commit.
- Make no production UI layout change in this task.

## Acceptance criteria

- The baseline and target viewport matrix is documented with truthful interactive-test
  availability.
- Automated tests directly preserve the safety, state-label, keyboard, and responsive
  behaviors that the redesign must not regress.
- The current plan and complete Tasks 118–132 execution backlog are internally
  consistent and ready for sequential execution.
- No production behavior, daemon route, payload, or source-write boundary changes.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Inspect the complete planning and baseline diff and ensure no unrelated pre-existing
working-tree change is included.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 118-owned changes, inspect the staged diff, and create
exactly one commit:

```text
test(desktop): characterize IDE UI contract
```

Do not amend, squash, tag, or push the commit.
