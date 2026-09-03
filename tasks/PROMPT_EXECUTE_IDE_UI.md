# Prompt — execute Tasks 118–132 sequentially with one commit per task

Use this prompt with one primary Codex implementation agent for the complete Mini-Orca
IDE-style Desktop UI backlog.

```text
You are the sole implementation agent for Mini-Orca Tasks 118–132.

Objective:
Implement the IDE-style UI redesign in plan.md. Execute Tasks 118 through 132 strictly
in numeric order. Finish, verify, and commit each task before starting the next. Do not
ask the user to say “continue” between tasks. Do not push any commit.

Read completely before acting, in this order:
1. AGENTS.md
2. plan.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/PROMPT_EXECUTE_IDE_UI.md
6. tasks/118_ide_ui_contract_baseline.md
7. tasks/119_desktop_layout_state.md
8. tasks/120_ide_shell_foundation.md
9. tasks/121_project_tool_window.md
10. tasks/122_editor_chrome.md
11. tasks/123_source_gutter_and_viewport.md
12. tasks/124_context_tool_window.md
13. tasks/125_assistant_review_tool_windows.md
14. tasks/126_problems_tool_window.md
15. tasks/127_checks_output_tool_windows.md
16. tasks/128_toolbar_and_command_search.md
17. tasks/129_persistent_status_bar.md
18. tasks/130_responsive_accessible_ide_shell.md
19. tasks/131_workspace_density_polish.md
20. tasks/132_ide_ui_acceptance.md
21. Current Desktop production code, tests, build files, and maintained documentation
    implicated by Task 118

Required outcome:
- Keep the existing Focus Flow palette and dark Mini-Orca identity.
- Establish a source-first IDE shell with Project on the left, source/review in the
  center, Context/Assistant/Review on the right, Problems/Checks/Output at the bottom,
  a compact toolbar, and a persistent status bar.
- Preserve one project, one active file, and one selected/new declaration per task.
- Keep source and diff selectable/read-only and the declaration/import draft isolated
  and editable.
- Keep generation preview-only and preserve explicit, current-evidence Apply and Undo.
- Keep remote-provider destination and confirmation visible before project context is
  sent.
- Keep keyboard navigation, state labels, accessible semantics, and the exact 1000dp
  responsive boundary.
- Produce exactly one isolated, reviewed local commit for every Task 118–132.

Non-negotiable product and architecture boundaries:
- Generation, project/file analysis, problem selection, source navigation, validation,
  and checks never write source.
- Apply and Undo remain explicit user actions and the only source mutations.
- Do not weaken project revision, target path, source hash, task identity, draft
  revision/hash, focused-check, or confirmation guards.
- Do not add source editing, multi-file edits, multi-open file tabs, automatic fixes,
  terminal, run/debug, filesystem mutation, VCS writes, or automatic commits inside
  Mini-Orca.
- Keep the daemon loopback-only by default and do not change its API solely for visual
  parity.
- Keep workflow/network state in DesktopWorkflowPresenter and domain state; layout
  preferences cannot authorize work.
- Do not add a generic docking engine, UI framework, event bus, duplicate global state,
  or a second shell.
- Do not copy JetBrains proprietary icons/assets or exact trade dress.
- Keep Kotlin/Compose versions aligned with the checked-in build and use the Desktop
  Gradle wrapper.
- Do not edit generated output, credentials, ignored local config.yaml, user projects,
  desktop/build, desktop/.gradle, or desktop/.kotlin.
- Use one implementation writer for the entire sequence. Read-only review may be
  delegated only if it cannot edit the shared worktree or mutate state. Do not create
  another user-owned Codex task for this sequence.

Starting-worktree and planning-artifact gate:
1. Run git status --short, git diff --name-only, and git diff --cached --name-only.
2. Verify Task 117 is Complete and its behavior/commit is present.
3. Require no staged changes before Task 118. Record every unstaged and untracked path
   before editing.
4. The user-approved plan.md, tasks/README.md, tasks/INDEX.md,
   tasks/PROMPT_EXECUTE_IDE_UI.md, and pending Task 118–132 files may initially be
   uncommitted planning artifacts. Task 118 owns validating and including those exact
   artifacts in its commit with its baseline work.
5. On a case-insensitive filesystem, Git may still show the historical root filename
   as PLAN.md. Task 118 must record a safe case-only rename to plan.md in its commit so
   links work on case-sensitive clones. Do not lose or duplicate the plan content.
6. Treat every other pre-existing modification as user-owned. Do not stage, rewrite,
   delete, format, or include it in a task commit.
7. Stop if required task work overlaps a user-owned hunk and cannot be separated
   safely.
8. Never use reset --hard, checkout, restore, stash, clean, rebase, or destructive
   history operations to manufacture a clean worktree.

Mandatory task commentary:
- Before Task N, post one update beginning exactly:
  “Starting Task N — <task title>: ...”
- After Task N's commit succeeds, post a separate update beginning exactly:
  “Task N complete — ...”
- At start, name the task boundary, likely files, focused checks, prior commit, and any
  unrelated pre-existing changes that must remain untouched.
- At completion, name behavior changed, commands/results, files changed, exact commit
  hash, and the next task.
- Post concise progress updates at least every 60 seconds during long work.

Sequential loop for every Task N from 118 through 132:

1. Dependency and boundary check
   - Read Task N completely again.
   - Verify every dependency is Complete and its acceptance behavior still exists.
   - Verify Task N is the first ready incomplete task in tasks/INDEX.md.
   - For N > 118, verify the prior required commit exists and no prior-task change is
     staged or uncommitted.
   - Inspect implicated production code, tests, documentation, current diffs, and the
     recent task commit. Understand surrounding design before editing.

2. Start
   - Post the mandatory Starting Task N commentary.
   - Set only Task N and its tasks/INDEX.md row to In Progress when implementation
     starts. Do not commit an In Progress marker separately.

3. Implement only Task N
   - Make the smallest cohesive production-quality change satisfying every
     implementation bullet and acceptance criterion.
   - Follow AGENTS.md, plan.md, KISS, existing package boundaries, and the safety rules
     above.
   - Prefer pure presentation/layout state and small cohesive composables. Reuse
     existing business rules rather than duplicating them for a new visual surface.
   - When a replacement becomes authoritative, delete the obsolete implementation,
     caller, helper, state branch, and redundant tests in the same task.
   - Add deterministic behavior-focused tests for each changed state or interaction.
   - Do not absorb a later task merely because adjacent code is already open.
   - Do not make unrelated cleanup, daemon, API, dependency, or visual-theme changes.

4. Verify before completion
   - Run every command listed by Task N.
   - Run git diff --check and inspect the complete task-owned diff as a senior reviewer.
   - Verify every acceptance criterion explicitly, including error, stale, narrow,
     keyboard, and long-content cases relevant to the task.
   - Do not weaken safety assertions, hide findings with broad suppressions, add test-
     only production branches, or report an unavailable interactive check as passed.
   - If interactive Desktop, screenshot, screen-reader, remote-provider, or OS coverage
     is unavailable, record the exact limitation and continue only where the task
     explicitly permits a release-operator follow-up.
   - Confirm generated output, secrets, local config, and unrelated user changes are
     absent from the task diff.

5. Complete task metadata
   - Only after all automatable acceptance criteria and required checks pass, set Task
     N to Complete, move it under tasks/completed/ without renaming it, and update
     tasks/INDEX.md.
   - For Task 132, also set plan.md to Complete with truthful automated and manual
     status, and update the maintained Desktop documentation.
   - Do not start Task N+1 before Task N's commit succeeds.

6. Stage exactly the task-owned change
   - Stage only Task N implementation, tests, documentation, task move, and index
     update.
   - For Task 118 only, also stage the validated initial plan/task execution artifacts
     named in the starting-worktree gate and the plan filename correction.
   - Use path-level staging for wholly owned files and hunk-level staging for a file
     containing a separable user-owned change.
   - Inspect git diff --cached --name-status and the complete git diff --cached.
   - Run git diff --cached --check.
   - Confirm the staged diff contains all Task N work, none of Task N+1, no unrelated
     user work, no generated output, and no secret or local configuration.

7. Create exactly one commit
   - Use the exact subject from Task N's Commit section and from the ordered list below.
   - Do not create a checkpoint, planning-only, fixup, or partial commit.
   - Do not amend, reword, squash, combine, tag, or push the commit.
   - Record the resulting exact commit hash.
   - Run git status --short and verify no Task N change remains staged or uncommitted.
     Previously recorded unrelated user changes may remain unstaged and must be
     preserved.

8. Report and continue
   - Post the mandatory Task N complete commentary including the exact commit hash.
   - Continue immediately to Task N+1 without waiting for confirmation.

Task ownership:
- Task 118 owns the UI contract/baseline, viewport and screenshot matrix, missing
  characterization tests, and initial plan/task/prompt artifacts.
- Task 119 owns DesktopLayoutState, tool/editor identifiers, layout transitions,
  dimension clamping, and preference migration.
- Task 120 owns the new shell component boundaries, tool-window bar, docked regions,
  dividers, layout wiring, and removal of the old workspace-rail shell.
- Task 121 owns the flat Project tool window, tree keyboard navigation, filter,
  collapse/reveal behavior, and active-file synchronization.
- Task 122 owns the single active-file tab, breadcrumbs, read-only state, explicit
  Source/Review surfaces, and removal of automatic review switching.
- Task 123 owns source/gutter separation, non-mutating markers, source scrolling and
  selection behavior, and measured large-file presentation work.
- Task 124 owns the right tool-window tab frame and fully connected Context content,
  selection synchronization, and analysis controls.
- Task 125 owns Assistant and Review content/tabs, workflow badges, focus stability,
  and preservation of draft/evidence/Apply/Undo rules.
- Task 126 owns the bottom Problems integration, shared findings presentation,
  navigation-only selection, collapsed summary, and explicit Prepare fix boundary.
- Task 127 owns bottom Checks/Output integration, shared evidence/progress
  presentation, long output behavior, attention summaries, and bottom preferences.
- Task 128 owns toolbar prioritization/overflow, project actions, and the unified file,
  symbol, and action command-search shell.
- Task 129 owns persistent trusted status-bar presentation, responsive priorities,
  status navigation, and removal of the transient footer.
- Task 130 owns final wide/narrow policy, bottom overlay/drawer, text scaling,
  keyboard traversal, focus restoration, semantics, and reduced-motion behavior.
- Task 131 owns Summary/Analysis/Bugs density, common visual states, token consistency,
  palette preservation, and obsolete visual-helper removal.
- Task 132 owns the residual UI audit, full automated/manual acceptance, maintained
  documentation, plan completion, final screenshot/keyboard evidence, and commit-
  history verification.

Required commit subjects in order:
1. test(desktop): characterize IDE UI contract
2. refactor(desktop): add IDE layout state
3. feat(desktop): introduce IDE shell
4. feat(desktop): add Project tool window
5. feat(desktop): add active file editor chrome
6. feat(desktop): improve source gutter and viewport
7. feat(desktop): add Context tool window
8. refactor(desktop): organize Assistant and Review tools
9. feat(desktop): add Problems tool window
10. feat(desktop): add Checks and Output tools
11. feat(desktop): unify toolbar command search
12. feat(desktop): add persistent status bar
13. feat(desktop): harden responsive IDE navigation
14. style(desktop): unify IDE workspace presentation
15. chore(desktop): complete IDE UI acceptance

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before reporting a blocker.
- Stop for an incomplete dependency, inseparable user-owned overlap, missing material
  product decision, unavailable required dependency, failed safety invariant, required
  check that cannot safely be fixed in scope, or inability to create an isolated task
  commit.
- Leave a started blocked task In Progress and uncommitted; leave every later task
  Pending.
- Never skip a blocked task, mark partial work Complete, create a partial commit,
  fabricate quality/manual evidence, or continue into a dependent task.
- A commit failure is not task completion. Resolve it without destructive history or
  stop with the exact staged state and error reported.

Final acceptance after the Task 132 commit:
1. Confirm Tasks 118–132 are Complete and linked under tasks/completed/.
2. Confirm exactly 15 new sequential task commits exist with the required subjects.
3. Confirm Desktop Spotless, Detekt, tests, make check, make quality, race tests, and
   git diff --check passed before the final commit.
4. Report interactive viewport, keyboard, provider, and screen-reader results
   truthfully, including every unavailable step.
5. Confirm the shipped shell and maintained docs match plan.md and preserve every
   product constraint.
6. Confirm no old shell, duplicated workflow presentation, transitional flag, or
   obsolete helper remains.
7. Confirm no Task 118–132 change is staged/uncommitted, unrelated user work remains
   preserved, and no task commit was amended, combined, tagged, or pushed.

Final response:
- Lead with whether all 15 tasks completed and whether the IDE-style UI plan is fully
  implemented.
- Provide a compact Task 118–132 table containing task, result, exact commit hash, and
  verification summary.
- Summarize the final shell, workflow/safety invariants, palette outcome, automated
  checks, manual checks, limitations, and any release-operator follow-up.
- State explicitly that commits were not pushed.
```
