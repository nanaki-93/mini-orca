# Prompt template — execute one task with an Air agent

Copy this prompt and replace `{TASK_FILE}` with one Pending task file from
[`INDEX.md`](INDEX.md).

```text
You are the implementation agent for one Mini-Orca task.

Task file: tasks/{TASK_FILE}

Read completely, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/{TASK_FILE}
6. The production files and tests named or implicated by the task

Before editing:
- Inspect `git status --short` and preserve every unrelated user change.
- Verify every listed dependency is Complete and its relevant acceptance behavior
  still exists. If not, stop and report the exact blocker.
- Summarize the task boundary, files likely to change, and focused checks.

Implementation rules:
- Implement only the selected task. Do not absorb a later task for convenience.
- Reuse existing packages, handlers, storage, API client, Compose state, and safety
  guards. Do not add a framework, database, second service, or duplicate workflow.
- Keep source/diff read-only and AI declaration drafts isolated until explicit Apply.
- Preserve the one-project, one-open-file, one selected/new symbol boundary.
- Never introduce automatic project writes, commits, scans, tests, or multi-file edits.
- Add focused regression tests for every changed behavior.
- Use `apply_patch` for manual file edits and do not edit generated build output.

Air-agent coordination:
- You own all writes for this task until handoff.
- You may delegate bounded read-only research or review to sub-agents.
- Do not allow another implementation agent to edit the same shared worktree files.
- Integrate and verify delegated findings yourself.

Verification:
- Run every command in the task that applies.
- Go changes: format, run focused tests, and vet affected packages.
- Concurrency/filesystem/security changes: run focused race tests.
- Desktop changes: run `./desktop/gradlew -p desktop test`.
- Always run `git diff --check` and inspect the final diff.

Completion:
- Only after all acceptance criteria pass, change the task status to Complete,
  move it to tasks/completed/, and update its link/status in tasks/INDEX.md.
- If blocked, leave it Pending or mark it In Progress truthfully; do not fabricate
  completion or bypass a failed dependency.
- Do not create a commit unless the user explicitly asks.

Finish with: task ID/status, behavior changed, files changed, acceptance evidence,
commands/results, and any blocker or intentionally deferred follow-up.
```

Example task value: `32_restore_green_validation_baseline.md`.
