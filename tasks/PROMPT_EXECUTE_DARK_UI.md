# Prompt — complete outstanding dark-UI acceptance

This historical prompt is retained only because [Task 149](149_dark_ui_acceptance.md)
is Pending. Tasks 140–148 are complete and their task files were retired to Git
history. Do not reimplement them.

The active new backlog is [UI precision](../Plan.md), executed separately through
[PROMPT_EXECUTE_UI_PRECISION.md](PROMPT_EXECUTE_UI_PRECISION.md). Reading either
prompt as planning context does not execute it.

## Instructions when Task 149 is explicitly requested

1. Read `AGENTS.md`, `desktop/UI_DESIGN_GUIDELINES.md`,
   `docs/dark-ui/PLAN.md`, `docs/dark-ui/ACCEPTANCE.md`, and
   `tasks/149_dark_ui_acceptance.md`. Inspect current Git status and relevant
   retained visual/keyboard evidence before acting.
2. Work only on Task 149 acceptance with one writer. Do not delegate, create
   separate tasks, rerun 140–148, or begin 161–171. The historical dark plan's
   visual specification is not authority to undo the newer UI precision work.
3. Follow Task 149's native screenshot, workflow, responsive, keyboard,
   assistive-technology, repository-check, and documentation requirements.
   Preserve the user's files and local fixture safety.
4. Record actual evidence and unavailable checks. Missing material native evidence
   or a required failed gate keeps Task 149 Pending/In Progress, not Complete.
5. Preserve the one-project/file/symbol preview-first flow, read-only source/diff,
   provider consent, and guarded Review/Apply/Undo. Preview is local-only.
   No backend expansion, automatic source writes, or unrelated restyling.
6. Run `./desktop/gradlew -p desktop spotlessCheck detekt test`, `make check`,
   `make quality`, and `git diff --check`; report exact failures and skipped stages.
   Never run destructive targets or Docker cleanup.
7. Only after all required acceptance passes, update the task and index to Complete.
   Keep the task at its stable path under the current task workflow. Report native
   evidence, checks run/not run, remaining limits, and configuration impact.
8. This historical prompt authorizes no staging, commits, or pushes. A commit
   request for the separate 161–171 sequence does not apply to Task 149.
