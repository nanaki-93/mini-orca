# Prompt — execute Tasks 75–79 sequentially

Use this prompt with one primary Codex implementation agent. The agent owns the shared
worktree and completes Tasks 75–79 in numeric order, with exactly one reviewed commit
for every completed task.

```text
You are the primary implementation agent for Mini-Orca Tasks 75–79.

Objective:
Implement the approved Analysis-summary and Editor-only explorer UX in PLAN.md. Execute
Tasks 75, 76, 77, 78, and 79 strictly sequentially. Complete and commit each task before
starting the next one. Do not ask the user to say “continue” between tasks.

Read completely before acting, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/75_analysis_editor_ux_execution_baseline.md
6. tasks/76_analysis_run_summary_presentation.md
7. tasks/77_analysis_workspace_summary_ui.md
8. tasks/78_editor_only_explorer_navigation.md
9. tasks/79_analysis_editor_ux_acceptance.md
10. The current production files and tests implicated by Task 76

Approved behavior:
- Analysis is an operational summary of project coverage and current/last Analyze-all run.
- Analysis lists only failed/error-bearing files with path, attempts, and sanitized error.
- Analysis has no successful-file cards and no per-file navigation actions.
- Summary, Analysis, and Bugs do not show or reserve space for the file explorer or Editor
  context panel.
- Editor owns the wide explorer/context panes and the narrow Files/Context drawers.
- Explorer or global file-palette selection always activates Editor and opens the exact
  indexed file.
- Per-file semantic analysis remains available inside Editor.
- No daemon/API/schema/configuration migration is part of this backlog.

Non-negotiable product boundaries:
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Source and composed diff remain selectable and read-only.
- Only the isolated declaration/import draft remains editable.
- Analyze-all, one-file analysis, scans, generation, validation, checks, Apply, and Undo
  remain explicit user actions.
- Remote-provider confirmation, project revision, file hash, request identity, and Apply/
  Undo guards remain intact.
- Do not add automatic project writes, multi-file candidates, product-generated commits,
  pushes, scans, tests, fixes, or a second workflow.
- Do not edit generated output, local config, credentials, desktop/build, desktop/.gradle,
  or desktop/.kotlin.

Starting-worktree protocol:
1. Run `git status --short`, `git diff --name-only`, and `git diff --cached --name-only`.
2. Record all pre-existing modifications mentally/in the task update. They are user-owned.
3. The Desktop files implicated by Tasks 77–78 may already be modified. Preserve every
   pre-existing hunk; do not assume a dirty file belongs to this backlog.
4. Never use reset, checkout, restore, stash, clean, rebase, or another destructive history
   operation to obtain a clean tree.
5. Never use `git add -A`, `git add .`, or blindly stage a whole pre-modified file.
6. Use hunk-level/index-only staging for task changes. Before every commit, inspect
   `git diff --cached --stat` and the complete `git diff --cached`.
7. If a task hunk cannot be separated safely from an overlapping pre-existing user hunk,
   stop before staging/committing and report the exact file and overlap.

Single-writer rule:
- You are the only implementation writer for the entire sequence.
- Do not start parallel implementation agents or create another Codex task.
- Read-only investigation is allowed, but you remain responsible for every decision, edit,
  test, staged hunk, and commit.

Sequential execution loop for each Task N from 75 through 79:

1. Dependency gate
   - Read Task N completely again.
   - Verify every dependency is Complete in tasks/INDEX.md and its required behavior exists.
   - If the dependency is incomplete or materially regressed, stop and report the blocker;
     do not skip the task or implement a dependent workaround.

2. Establish scope
   - Inspect the implicated production code, tests, documentation, and current diff.
   - State a concise boundary update: task ID, outcome, files likely to change, focused tests,
     and pre-existing changes that must remain outside the task.
   - Change Task N and its index row to In Progress only when implementation starts.

3. Implement only Task N
   - Make the smallest cohesive change satisfying every task bullet.
   - Follow Clean Code, KISS, existing package/state boundaries, and repository patterns.
   - Replace obsolete callbacks/helpers/branches instead of retaining parallel behavior.
   - Add focused regression tests in the same task as the changed behavior.
   - Do not absorb a later task merely because its code is nearby.

4. Verify
   - Run every focused command listed in Task N.
   - For Tasks 76–79, run `./desktop/gradlew -p desktop test`.
   - Always run `git diff --check` and inspect the complete task-owned diff.
   - Do not weaken, delete, or rewrite a valid test merely to make the suite green.
   - Check every acceptance criterion explicitly against code and test evidence.

5. Complete task metadata
   - Only after all acceptance criteria pass, set Task N to Complete.
   - Move its task file to `tasks/completed/` without changing its number/name.
   - Update tasks/INDEX.md to link to `completed/...` and mark it Complete.
   - Leave every later task Pending.

6. Create exactly one task commit
   - Stage only Task N implementation/tests/docs and its task/index status changes.
   - For Task 75, stage the approved planning/task/prompt files only; do not stage production.
   - Use hunk-level staging for every pre-modified file.
   - Inspect the complete staged diff and confirm it contains no user-owned or later-task hunk.
   - Create the exact commit message from Task N’s Commit section.
   - Do not amend, squash, rebase, push, tag, or create an extra cleanup commit.
   - Record the commit hash and confirm pre-existing unstaged changes remain present.

7. Continue automatically
   - Report a concise task-boundary update with status, commit hash, behavior, tests, and any
     deliberate deferral to the next task.
   - Immediately select Task N+1 and repeat.

Task-specific guardrails:
- Task 75 is documentation/task metadata only. Its commit establishes the executable plan.
- Task 76 owns pure Analyze-all summary classification and tests, not Compose layout.
- Task 77 owns the Analysis workspace replacement and removal of Analysis file navigation;
  it must not remove EditorSurface.FileAnalysis or one-file analysis.
- Task 78 owns Editor-only chrome and consistent file navigation; it must preserve the
  1000dp breakpoint, pane width preferences, keyboard behavior, and selection state.
- Task 79 owns final regression/documentation acceptance and only task-scoped fixes needed
  to make the approved behavior truthful.

Blocking policy:
- Investigate failures and exhaust safe, task-scoped fixes before declaring a blocker.
- Stop for an inseparable pre-existing change, failed dependency, missing material decision,
  unavailable required dependency, or validation failure that cannot safely be fixed in scope.
- Leave the current task In Progress if implementation began, or Pending if it did not.
- Never skip a blocked task, mark partial work Complete, fabricate test evidence, or commit
  unrelated changes.

Final acceptance after Task 79:
1. Confirm Tasks 75–79 are Complete and linked under tasks/completed/.
2. Confirm the five required commits, in order:
   - docs(tasks): define analysis workspace UX backlog
   - feat(desktop): summarize analyze-all results
   - feat(desktop): simplify analysis workspace results
   - feat(desktop): scope file navigation to editor
   - test(desktop): verify analysis and editor workspace UX
3. Run `./desktop/gradlew -p desktop test`, `make check` when permitted, and
   `git diff --check`.
4. Review `git log -5 --oneline` and every commit diff for task scope.
5. Verify pre-existing unrelated changes remain unstaged/uncommitted and unaltered.
6. Report manual wide/exact-1000dp/narrow checks honestly if a GUI is unavailable.
7. Do not push.

Final response:
- Lead with whether all five tasks completed.
- List each task, commit hash, and verification result.
- Summarize the shipped Analysis and Editor behavior.
- State any test or manual check not run and why.
- State that no API/configuration/data migration was required, unless implementation proved
  otherwise and work stopped for approval.
- Link PLAN.md, tasks/INDEX.md, and key changed files using absolute paths.
```
