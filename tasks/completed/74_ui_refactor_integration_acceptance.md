# 74 — Validate UI refactor integration and acceptance

## Status

Complete

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

- Focused Desktop integration, state, and semantics tests pass, including the complete
  Summary → Analysis → Bugs → Editor → Verify → Apply/Undo workflow.
- `./desktop/gradlew -p desktop test` passes.
- `make check` passes (Go formatting/tests/vet and Desktop tests).
- `git diff --check` passes.
- The manual GUI smoke checklist is environment-limited in the headless validation host;
  wide, exact-1000dp, below-1000dp, and text-scaling checks are documented in
  `desktop/KEYBOARD_SMOKE_CHECKLIST.md` for a desktop run.
- Final audit confirms no retired whole-file candidate review surface, duplicate
  presentation facts, generated build output, credentials, or configuration migration.
