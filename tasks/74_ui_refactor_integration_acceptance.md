# 74 — Validate UI refactor integration and acceptance

## Status

Pending

## Goal

Prove the selected hybrid UI is complete, coherent, safe, documented, and free of
the superseded Desktop presentation.

## Depends on

Tasks 57–73.

## Implementation

- Exercise Summary, Analysis, Bugs, and every Editor stage against representative
  project, finding, draft, validation, check, Apply, and Undo states.
- Verify local/remote provider flows, disconnected/loading/error/canceled states,
  dirty/invalid/stale drafts, failed checks, long paths, non-Git projects, and narrow layout.
- Confirm the production UI matches the selected hybrid structure and Focus Flow palette;
  compare against the approved mock sources without sacrificing accessibility/product guards.
- Confirm no retired candidate review UI, duplicate file/symbol facts, raw one-off visual
  system, generated output edits, or new automatic mutation remains.
- Update Desktop usage documentation and task/release evidence to describe the stage flow,
  shortcuts, responsive drawers, and Apply/Undo decision.
- Record any environment-limited visual checks honestly; do not mark complete on partial evidence.

## Acceptance criteria

- Every completion criterion in the UI implementation plan has automated or reproducible evidence.
- All project mutations remain explicit, revision/hash guarded, and limited to one named file.
- The complete UI is keyboard/responsive/read-only where required and visually consistent.
- Every Task 57–74 is Complete and moved under `tasks/completed/`.
- No configuration or data migration is required.

## Verification

- Run focused Desktop integration/semantics tests and the complete UI smoke checklist.
- Run `./desktop/gradlew -p desktop test`, `make check`, and `git diff --check`.
- Inspect final diff and report behavior, files, commands, limitations, and deferred work.
