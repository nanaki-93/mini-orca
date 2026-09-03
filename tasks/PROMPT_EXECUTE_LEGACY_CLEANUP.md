# Prompt — execute Tasks 103–117 sequentially with one commit per task

Use this prompt with one primary Codex implementation agent for the complete legacy
cleanup and pragmatic refactor backlog.

```text
You are the sole implementation agent for Mini-Orca Tasks 103–117.

Objective:
Implement the cleanup and refactor in PLAN.md. Execute Tasks 103 through 117 strictly
in numeric order. Finish, verify, and commit each task before starting the next. Do not
ask the user to say “continue” between tasks. Do not push any commit.

Read completely before acting, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/103_cleanup_contract_baseline.md
6. tasks/104_retire_autonomous_orchestration.md
7. tasks/105_simplify_scoped_model_execution.md
8. tasks/106_consolidate_project_detection.md
9. tasks/107_strict_scoped_configuration.md
10. tasks/108_draft_identities_apply.md
11. tasks/109_remove_candidate_compatibility.md
12. tasks/110_reduce_loopback_api.md
13. tasks/111_unify_project_traversal.md
14. tasks/112_unify_metadata_persistence.md
15. tasks/113_isolate_go_workflow_state.md
16. tasks/114_extract_desktop_presenter.md
17. tasks/115_simplify_desktop_ui_contracts.md
18. tasks/116_clean_docs_and_build.md
19. tasks/117_cleanup_acceptance.md
20. Current production code, tests, build files, and maintained documentation implicated
    by Task 103

Required outcome:
- Preserve the one-project, one-file, one-symbol, preview-first product workflow.
- Remove the retired orchestrator, generic agent, general tools, generic workflow, thin
  error wrapper, whole-file generation, candidate compatibility, activity history,
  deprecated routes, and obsolete persistence paths.
- Keep one strict analyze/bug/function model configuration and one provider-neutral LLM
  request boundary.
- Keep one draft representation and one explicit, stale-safe Apply/Undo path.
- Share only equivalent source traversal, atomic metadata persistence, identity, and
  transport rules; do not add generic frameworks.
- Give Go mutable state and Desktop asynchronous workflow state clear owners.
- Reduce Compose parameter explosion and use typed API requests.
- Consolidate obsolete documentation/build residue and enforce reproducible quality gates.
- Produce exactly one isolated, reviewed commit for every Task 103–117.

Non-negotiable product and architecture boundaries:
- Generation, analysis, chat, validation, and checks never write source.
- Apply and Undo remain explicit user actions and are the only source mutations.
- Revalidate project revision, target path, source hash, task identity, draft revision,
  and draft hash immediately before Apply.
- Keep the daemon loopback-only by default.
- Keep source and diff selectable/read-only, draft text isolated/editable, keyboard
  navigation, visible state labels, and responsive drawer behavior below 1000dp.
- Keep Go 1.22 compatibility and use the Desktop Gradle wrapper.
- Do not add a DI framework, generic repository, workflow DSL, event bus, autonomous
  agent, generalized command runner, native provider SDK, credential store, speculative
  cache, automatic source/test write, or automatic commit.
- Do not edit generated output, credentials, ignored local config.yaml, user projects,
  desktop/build, desktop/.gradle, or desktop/.kotlin.
- Use one implementation writer for the entire sequence. Do not create another Codex
  task and do not delegate implementation or shared-worktree edits to a subagent.

Starting-worktree and planning-artifact gate:
1. Run git status --short, git diff --name-only, and git diff --cached --name-only.
2. Require Task 102 to be Complete and HEAD to contain its behavior.
3. Require no staged changes. Record all unstaged and untracked paths before editing.
4. The user-approved PLAN.md cleanup rewrite, tasks/README.md, tasks/INDEX.md, this
   prompt, and pending Task 103–117 files may initially be uncommitted planning
   artifacts. Task 103 owns validating and including that exact backlog metadata in
   its commit together with its characterization work.
5. Treat every other pre-existing modification as user-owned. Do not stage it, rewrite
   it, or include it in a task commit.
6. Stop if required work overlaps a user-owned hunk and cannot be separated safely.
7. Never use reset, checkout, restore, stash, clean, rebase, or destructive history
   operations to manufacture a clean worktree.

Mandatory task commentary:
- Before Task N, post one update beginning exactly:
  “Starting Task N — <task title>: ...”
- After Task N's commit succeeds, post a separate update beginning exactly:
  “Task N complete — ...”
- At start, name the task boundary, likely files, focused checks, prior commit, and any
  unrelated pre-existing changes that must remain untouched.
- At completion, name behavior/removals, commands/results, changed files, exact commit
  hash, and the next task.
- Post concise progress updates at least every 60 seconds during long work.

Sequential loop for every Task N from 103 through 117:

1. Dependency and boundary check
   - Read Task N completely again.
   - Verify every dependency is Complete and its acceptance behavior still exists.
   - Verify Task N is the first ready incomplete task in tasks/INDEX.md.
   - For N > 103, verify the prior required commit exists and no prior-task change is
     staged or uncommitted.
   - Inspect implicated code, tests, docs, current diffs, and recent task commits.

2. Start
   - Post the mandatory Starting Task N commentary.
   - Set only Task N and its tasks/INDEX.md row to In Progress when implementation starts.
   - Do not commit the In Progress marker separately.

3. Implement only Task N
   - Make the smallest cohesive change satisfying every implementation bullet and
     acceptance criterion.
   - Follow AGENTS.md, PLAN.md, Clean Code, KISS, and existing safety boundaries.
   - When introducing a replacement, delete the obsolete implementation, caller, test,
     fixture, route, DTO, or documentation in the same task when that deletion belongs
     to Task N.
   - Do not keep compatibility adapters, legacy aliases, or parallel implementations
     unless Task N explicitly retains a public contract until a later named task.
   - Add focused regression/characterization tests with each behavior change.
   - Do not absorb a later task merely because nearby code is already open.

4. Verify before completion
   - Run every command listed by Task N.
   - Run git diff --check and inspect the complete task-owned diff.
   - Verify each acceptance criterion explicitly.
   - Do not weaken safety assertions, hide findings with blanket suppressions, create
     dummy callers for dead code, or report an unavailable manual check as passed.
   - Confirm generated output, credentials, local config, and unrelated changes are absent.

5. Complete task metadata
   - Only after all criteria pass, set Task N to Complete, move it under
     tasks/completed/ without renaming it, and update tasks/INDEX.md.
   - For Task 117, also set PLAN.md to Complete with truthful validation/manual status.
   - Do not start Task N+1 before Task N's commit succeeds.

6. Stage exactly the task-owned change
   - Stage only Task N implementation, tests, docs, task move, index update, and any
     explicitly Task-N-owned plan/build update.
   - For Task 103 only, also stage the validated initial PLAN/tasks execution artifacts
     listed in the starting-worktree gate.
   - Use path-level staging for wholly owned files and hunk-level staging for any file
     containing a separable user-owned change.
   - Inspect git diff --cached --name-status and the complete git diff --cached.
   - Run git diff --cached --check.
   - Confirm the staged diff includes all Task N work, none of Task N+1, no unrelated
     user work, no generated output, and no secret or local configuration.

7. Create exactly one commit
   - Use the exact subject from Task N's Commit section.
   - Do not create a partial/checkpoint commit.
   - Do not amend, reword, squash, fix up, combine, tag, or push the commit.
   - Record the resulting commit hash.
   - Run git status --short and verify no Task N change remains uncommitted. Unrelated
     recorded user changes may remain unstaged and must be preserved.

8. Report and continue
   - Post the mandatory Task N complete commentary including the exact commit hash.
   - Continue immediately to Task N+1 without waiting for confirmation.

Task ownership:
- Task 103 owns route/consumer inventory, critical characterization, baseline evidence,
  and the initial PLAN/task execution artifacts.
- Task 104 owns deletion of internal/orchestrator only.
- Task 105 owns direct scoped model execution and deletion of internal/agent.
- Task 106 owns project detection migration and deletion of internal/tools.
- Task 107 owns strict scoped configuration, one LLM boundary, config migration, and
  module tidiness.
- Task 108 owns explicit draft identities and decomposition of checks/Apply safety.
- Task 109 owns deletion of whole-file generation, candidate compatibility/comparison,
  and internal/workflow.
- Task 110 owns route/DTO/client reduction, activity removal, transport helpers, and
  internal/errors deletion.
- Task 111 owns deterministic eligible-source traversal.
- Task 112 owns atomic metadata persistence, obsolete analysis/activity outputs, and
  dead project persistence helpers.
- Task 113 owns Go workflow state owners, lock boundaries, app complexity, and residual
  Go dead helpers.
- Task 114 owns the Desktop presenter, lifecycle, polling, identity, cancellation, and
  stale-response behavior.
- Task 115 owns Compose feature contracts, typed request DTOs, UI unused code, and
  retained accessibility/responsive behavior.
- Task 116 owns pre-cleanup documentation/task consolidation, Docker residue, Gradle
  compatibility warning, and canonical docs.
- Task 117 owns reproducible quality gates, residual fixes, final metrics, full/manual
  acceptance, PLAN completion, and commit-history verification.

Required commit subjects in order:
1. test: characterize cleanup contract
2. refactor: remove autonomous orchestration
3. refactor(app): remove generic agent layer
4. refactor(project): remove general tool executors
5. refactor(config): enforce scoped model profiles
6. refactor(app): make draft apply identities explicit
7. refactor(app): remove candidate compatibility
8. refactor(api): remove legacy loopback routes
9. refactor(project): unify source traversal
10. refactor(storage): unify metadata persistence
11. refactor(app): isolate workflow state
12. refactor(desktop): extract workflow presenter
13. refactor(desktop): simplify UI and API contracts
14. chore: remove legacy docs and build residue
15. chore: complete legacy cleanup acceptance

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before reporting a blocker.
- Stop for an incomplete dependency, inseparable user-owned overlap, missing material
  product decision, unavailable required local dependency, failed safety invariant, a
  required check that cannot safely be fixed in scope, or inability to create an
  isolated task commit.
- Leave a started blocked task In Progress and uncommitted; leave an unstarted task
  Pending.
- Never skip a blocked task, mark partial work Complete, create a partial commit,
  fabricate quality/manual evidence, or continue into a dependent task.
- A commit failure is not task completion. Resolve it without destructive history or
  stop with the exact staged state and error reported.

Final acceptance after the Task 117 commit:
1. Confirm Tasks 103–117 are Complete and linked under tasks/completed/.
2. Confirm exactly 15 new sequential task commits exist with the required subjects.
3. Confirm make check, make test-race, make quality, go mod tidy -diff, the Desktop
   quality tasks, and git diff --check passed before the final commit.
4. Report manual smoke results truthfully, including every unavailable step.
5. Confirm the package graph, route inventory, configuration, persistence, and Desktop
   architecture match PLAN.md.
6. Confirm preview-first and one-file Apply/Undo safety remains covered.
7. Confirm no Task 103–117 change is staged/uncommitted, unrelated user work remains
   preserved, and no task commit was amended, combined, tagged, or pushed.

Final response:
- Lead with whether all 15 tasks completed and were committed separately.
- List Tasks 103–117 with status, one-line outcome, and commit hash.
- Summarize removed code and the final Go/Desktop architecture.
- Summarize automated and manual validation, metrics, limitations, migration steps,
  workflow safety, and preserved unrelated changes.
- Confirm no commits were pushed.
- Link PLAN.md, tasks/INDEX.md, the release acceptance record, and key changed files
  using absolute paths.
```
