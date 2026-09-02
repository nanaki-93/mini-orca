# Prompt template — execute one task

Use the all-tasks prompt for the normal Tasks 85–89 sequence. This template is for
resuming exactly one incomplete task. Replace `{TASK_FILE}` and `{TASK_ID}` with the
selected Pending/In Progress task from [`INDEX.md`](INDEX.md).

```text
You are the sole implementation agent for one Mini-Orca task.

Task ID: {TASK_ID}
Task file: tasks/{TASK_FILE}

Read completely, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/{TASK_FILE}
6. Every completed dependency task named by the selected task
7. The current production files, tests, and documentation implicated by the task

Before editing:
- Run `git status --short`, `git diff --name-only`, and
  `git diff --cached --name-only`.
- Preserve all pre-existing changes as user-owned. Relevant Desktop files may already be
  dirty; do not assume the entire file belongs to this task.
- Verify every dependency is Complete and its acceptance behavior still exists.
- State the task boundary, likely files, focused checks, and pre-existing changes that
  must remain outside the commit.
- Stop if the task is not the first ready incomplete task in dependency order.

Implementation rules:
- Implement only the selected task; do not absorb later work.
- Follow AGENTS.md, Clean Code, KISS, and existing package/state boundaries.
- Preserve the one-project, one-file, one-symbol, preview-first workflow.
- Keep source/diff read-only and the declaration/import draft isolated until explicit Apply.
- Preserve Open project, Analyze-all, explicit prompt-bearing actions, remote-provider confirmation,
  revision/hash/request identity, and explicit mutation actions.
- Add focused regression tests for changed behavior.
- Use apply_patch for manual edits and do not edit generated output or local configuration.
- Do not start another implementation writer or create another Codex task.
- Post a user-facing commentary update beginning with `Starting Task {TASK_ID}` before
  implementation, brief progress updates during work longer than 60 seconds, and a
  separate `Task {TASK_ID} complete` commentary update after verification.
- These required task comments are conversation commentary, not source-code comments.

Pre-existing-change rules:
- Never use reset, checkout, restore, stash, clean, rebase, or destructive history commands.
- Do not stage or commit any file. Tasks 85–89 do not authorize commits.
- If required work cannot be separated safely from an overlapping user-owned hunk, stop
  and report the exact overlap.

Verification:
- Run every command listed by the task.
- For Desktop changes, run `./desktop/gradlew -p desktop test`.
- Always run `git diff --check` and inspect the complete task diff.
- Verify every acceptance criterion one by one.

Completion:
- Only after all criteria pass, mark the task Complete, move it to tasks/completed/, and
  update its tasks/INDEX.md link/status.
- Do not commit, push, tag, rebase, or stage the implementation or task metadata.
- Confirm pre-existing unrelated changes remain present and unaltered.

Finish with the task status, behavior changed, files changed, acceptance evidence,
commands/results, and any blocker or deliberately deferred follow-up.
```

Example current task value:

```text
Task ID: 85
Task file: 85_editor_symbol_selection_contract.md
```
