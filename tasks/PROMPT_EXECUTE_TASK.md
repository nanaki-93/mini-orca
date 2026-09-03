# Prompt template — execute one cleanup task

Use the full sequential prompt for the normal Tasks 103–117 run. This template is for
resuming exactly one Pending or In Progress cleanup task. Replace `{TASK_ID}` and
`{TASK_FILE}` using [`INDEX.md`](INDEX.md).

```text
You are the sole implementation agent for one Mini-Orca cleanup task.

Task ID: {TASK_ID}
Task file: tasks/{TASK_FILE}

Read completely, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/{TASK_FILE}
6. Every completed dependency task named by the selected task
7. Current production files, tests, build files, docs, and recent commits implicated by
   the selected task

Before editing:
- Run git status --short, git diff --name-only, and git diff --cached --name-only.
- Require no staged changes and verify this is the first ready incomplete task.
- Verify every dependency is Complete, its required commit exists, and its acceptance
  behavior still exists.
- Record all pre-existing modifications. Treat unrelated changes as user-owned and do
  not stage, rewrite, or commit them.
- Task 103 may own the initial user-approved PLAN/task execution artifacts documented
  by PROMPT_EXECUTE_LEGACY_CLEANUP.md. No later task inherits that exception.
- Stop if a required edit overlaps a user-owned hunk and cannot be separated safely.
- Never use reset, checkout, restore, stash, clean, rebase, or destructive history
  operations.

Implementation rules:
- Post commentary beginning `Starting Task {TASK_ID}` before implementation.
- Set only the selected task and index row In Progress; do not commit that marker alone.
- Implement only the selected task and every one of its acceptance criteria.
- Follow AGENTS.md, PLAN.md, Clean Code, KISS, and preview-first safety.
- Replace obsolete code instead of retaining old/new paths.
- Add focused tests for changed behavior.
- Do not create another Codex task, delegate writes, or use another implementation writer.
- Do not edit generated output, credentials, local config, or user projects.
- Post concise progress updates at least every 60 seconds during long work.

Verification:
- Run every command in the task's Verification section.
- Run git diff --check and inspect the complete task-owned diff.
- Verify each acceptance criterion explicitly; do not suppress or weaken a real safety
  finding to get a pass.

Completion and commit:
- Only after all criteria pass, set the task Complete, move it to tasks/completed/
  without renaming it, and update tasks/INDEX.md.
- Stage only implementation, tests, docs, task move, index update, and explicitly owned
  PLAN/build changes for this task. Use hunk staging around separable user changes.
- Inspect git diff --cached --name-status and the complete git diff --cached.
- Run git diff --cached --check and confirm no unrelated/generated/secret content.
- Create exactly one commit using the exact subject in the task's Commit section.
- Do not create partial commits, amend, squash, combine, reword, tag, or push.
- Record the hash and verify no selected-task change remains staged or uncommitted.
- Post separate commentary beginning `Task {TASK_ID} complete` with behavior, files,
  checks, exact hash, limitations, and the next ready task.

If blocked:
- Exhaust safe task-scoped investigation first.
- Do not skip ahead or create a partial commit.
- Leave a started task In Progress and report the exact blocker, working-tree state,
  commands run, and next required user/external action.

Final response:
- State task status and one-line outcome.
- Report behavior/removals, acceptance evidence, commands/results, exact commit hash,
  manual limitations, migration impact, and preserved unrelated changes.
- Confirm the commit was not pushed.
- Link the task, PLAN.md, INDEX.md, and key changed files with absolute paths.
```

Example:

```text
Task ID: 103
Task file: 103_cleanup_contract_baseline.md
```
