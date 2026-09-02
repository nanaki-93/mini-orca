# Prompt — execute Tasks 90–94 sequentially with one commit per task

Use this prompt with one primary Codex implementation agent after Task 89 is Complete
and the preceding overlapping Editor work has a stable commit boundary.

```text
You are the sole implementation agent for Mini-Orca Tasks 90–94.

Objective:
Implement the focused Desktop UX/UI refinement in PLAN.md. Execute Tasks 90, 91, 92,
93, and 94 strictly sequentially. Finish, verify, and commit each task before starting
the next. Do not ask the user to say “continue” between tasks, and do not push commits.

Read completely before acting, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/90_editor_active_file_header.md
6. tasks/91_remove_editor_source_instruction.md
7. tasks/92_hide_empty_required_imports.md
8. tasks/93_group_bugs_by_priority.md
9. tasks/94_desktop_ux_refinement_acceptance.md
10. Task 89 and the current production files, tests, and docs implicated by Task 90

Required outcome:
- The active file basename is prominent at the top of Editor source and review views,
  with its project-relative path available to disambiguate duplicate names.
- The visible Editor progress explanation and “Click a…” source subtitle are removed.
- Required imports is absent for an empty draft imports list and unchanged for a
  nonempty list.
- Bugs is grouped by high, medium, low, then fallback priority by default after filters,
  while every card retains explicit provenance.
- Responsive, keyboard, accessibility, read-only source/diff, draft validation, focused
  checks, Apply, Undo, finding actions, and identity guards continue to work.

Non-negotiable boundaries:
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Source and composed diff remain selectable and read-only.
- Only the isolated declaration/import draft is editable before Apply.
- Selection and navigation perform no model call or write. Analyze, Refresh, Send,
  Validate, checks, Apply, Undo, scans, and triage remain explicit.
- Preserve remote-provider confirmation plus project revision, file hash, draft identity,
  and asynchronous-response guards.
- Do not change daemon routes, serialized models, persistence, OpenAPI, provider config,
  or migration behavior.
- Do not edit generated output, credentials, local config, desktop/build,
  desktop/.gradle, or desktop/.kotlin.
- Use one implementation writer for the whole sequence. Do not create another Codex
  task or delegate writes to a subagent.

Starting-worktree and dependency gate:
1. Run git status --short, git diff --name-only, and git diff --cached --name-only.
2. Verify Task 89 is Complete in tasks/INDEX.md and its direct-symbol behavior still
   exists. Do not start Task 90 while Task 89 is In Progress.
3. Require no staged changes. Record all unstaged/untracked changes as user-owned.
4. Stop if a pre-existing change overlaps a Task 90–94 production, test, documentation,
   or metadata hunk and cannot be cleanly excluded from that task's commit. In
   particular, do not commit uncommitted Tasks 85–89 work as part of Task 90.
5. Unrelated, separable user changes may remain unstaged. Never use reset, checkout,
   restore, stash, clean, rebase, or destructive history operations to prepare the tree.

Mandatory task commentary:
- Before Task N, post one update beginning exactly:
  “Starting Task N — <task title>: ...”
- After Task N's commit succeeds, post a separate update beginning exactly:
  “Task N complete — ...”
- At start, name scope, likely files, focused verification, and pre-existing changes.
- At completion, name behavior, commands/results, exact commit hash, and next task.
- Post concise progress updates at least every 60 seconds during long work.

Sequential execution loop for each Task N from 90 through 94:

1. Dependency and clean-boundary check
   - Read Task N completely again.
   - Verify every dependency is Complete and its acceptance behavior still exists.
   - Verify the prior task commit exists when N is greater than 90.
   - Run git status --short and confirm no staged changes or uncommitted leftovers from
     the previous task.

2. Start and scope
   - Inspect implicated code, tests, docs, current diffs, and recent task commits.
   - Post the mandatory Starting Task N commentary.
   - Mark only Task N and its index row In Progress when implementation begins.

3. Implement only Task N
   - Make the smallest cohesive change satisfying every implementation bullet and
     acceptance criterion.
   - Follow Clean Code, KISS, existing package/state boundaries, and AGENTS.md.
   - Remove only implementation made obsolete by this task; do not leave parallel old
     and new presentation paths.
   - Add focused regression tests with the behavior change.
   - Do not absorb a later task merely because its code is nearby.

4. Verify before committing
   - Run every focused command listed by Task N.
   - Run ./desktop/gradlew -p desktop test for every task.
   - Run git diff --check and inspect the complete task-owned diff.
   - Verify every acceptance criterion explicitly. Do not weaken valid tests to pass.
   - For Task 94, also run make check and complete or accurately record the manual GUI
     checks before committing.

5. Complete metadata and create exactly one commit
   - Only after all criteria and checks pass, set Task N to Complete, move it under
     tasks/completed/ without renaming it, and update tasks/INDEX.md.
   - For Task 94, also set PLAN.md to Complete and finish the required documentation.
   - Stage only Task N implementation, tests, docs, task move, index update, and any
     Task-N-owned PLAN update. Use path- or hunk-level staging to exclude user changes.
   - Inspect git diff --cached --name-status and the complete git diff --cached.
   - Run git diff --cached --check.
   - Confirm the staged diff contains every Task N change, no later-task work, no
     generated output, and no unrelated or secret/config content.
   - Commit once using the exact message in Task N's Commit section.
   - Do not amend, squash, combine, reword, tag, or push the commit.
   - Record the resulting commit hash, then run git status --short and confirm no Task N
     change remains uncommitted. Preserve any recorded unrelated user changes.

6. Report and continue
   - Post the mandatory Task N complete commentary including the commit hash.
   - Immediately continue to Task N+1 without waiting for confirmation.

Task ownership:
- Task 90 owns the active-file header, removal of progress presentation, and resulting
  Editor layout/tests. It retains workflow progress state where routing still needs it.
- Task 91 owns only the visible “Click a…” subtitle removal and source-interaction
  regression coverage.
- Task 92 owns conditional Required imports visibility and preservation of nonempty
  editing/invalidation behavior.
- Task 93 owns client-side priority grouping and Bugs presentation/tests, with filters
  first and provenance retained.
- Task 94 owns cross-feature acceptance, documentation, final gap fixes limited to Tasks
  90–93, full validation, and truthful PLAN completion.

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before declaring a blocker.
- Stop for incomplete Task 89, an inseparable pre-existing change, failed dependency,
  missing material product decision, unavailable required dependency, a required check
  that cannot safely be fixed in scope, or inability to produce an isolated commit.
- Leave a started blocked task In Progress; otherwise leave it Pending.
- Never skip a blocked task, mark partial work Complete, create a partial commit,
  fabricate evidence, or continue into a dependent task.
- A commit failure is not task completion. Resolve it without destructive history or
  stop with the staged state and exact error reported.

Final acceptance after the Task 94 commit:
1. Confirm Tasks 90–94 are Complete and linked under tasks/completed/.
2. Confirm exactly five new sequential task commits exist with these subjects:
   - feat(desktop): emphasize active editor file
   - refactor(desktop): remove source helper subtitle
   - fix(desktop): hide empty required imports
   - feat(desktop): group bugs by priority
   - test(desktop): complete UX refinement acceptance
3. Confirm ./desktop/gradlew -p desktop test, make check, and git diff --check passed
   before the final commit, and report any manual check accurately as not run.
4. Confirm no Task 90–94 change remains uncommitted, unrelated user changes remain
   preserved, and nothing is staged.
5. Confirm no commit was amended, squashed, combined, tagged, or pushed.

Final response:
- Lead with whether all five tasks completed and were committed separately.
- List Tasks 90–94 with status, one-line outcome, and commit hash.
- Summarize automated and manual validation, including anything not run.
- Confirm workflow safety, unchanged API/configuration/migration behavior, preserved
  unrelated work, and that no commits were pushed.
- Link PLAN.md, tasks/INDEX.md, and key changed files using absolute paths.
```
