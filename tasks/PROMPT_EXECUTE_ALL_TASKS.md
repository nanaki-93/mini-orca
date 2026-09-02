# Prompt — execute Tasks 80–84 sequentially

Use this prompt with one primary Codex implementation agent. The agent owns the shared
worktree, completes Tasks 80–84 in numeric order, and posts a user-facing commentary
update for every task before and after its work.

```text
You are the primary implementation agent for Mini-Orca Tasks 80–84.

Objective:
Implement the Desktop UX/UI simplification plan in PLAN.md. Execute Tasks 80, 81, 82,
83, and 84 strictly sequentially. Finish and verify each task before starting the next
one. Do not ask the user to say “continue” between tasks.

Read completely before acting, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/80_desktop_compact_semantic_controls.md
6. tasks/81_desktop_no_project_landing.md
7. tasks/82_desktop_metadata_copy_simplification.md
8. tasks/83_desktop_workspace_density_polish.md
9. tasks/84_desktop_ux_refinement_acceptance.md
10. Current production files, tests, and documentation implicated by Task 80

Required outcome:
- No-project mode shows only Mini-Orca identity, Open project, and contextual
  progress/retry feedback. Open project is the only actionable function.
- Project-only panels, buttons, drawers, palettes, and shortcuts are unavailable until a
  project opens.
- Routine UI omits project inventory/file counts, workspace numeric counters, project
  revision SHAs, file/draft hashes, and similar implementation identity.
- Revision/hash values remain unchanged in models, API requests, stale-response guards,
  draft/check matching, Apply, and Undo.
- Buttons use shared semantic roles for primary, navigation, positive, attention,
  destructive/interruption, and neutral actions.
- Buttons and single-line fields use the approved compact scale without clipping or
  accessibility loss.
- Persistent explanatory labels that restate controls are removed. Actionable errors,
  blocked reasons, remote confirmation, validation/check evidence, exact Apply target,
  and receipt remain visible.
- Connection/status appears once, advanced Bugs filters are on demand, Analyze-all limits
  are compact, and related actions group responsively.

Non-negotiable product boundaries:
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Source and composed diff remain selectable and read-only.
- Only the isolated declaration/import draft remains editable.
- Project import, Analyze-all, one-file analysis, scans, generation, validation, checks,
  Apply, and Undo remain explicit user actions.
- Remote-provider confirmation and every revision/hash/request-identity guard remain
  intact.
- Preserve the 1000dp breakpoint, saved Editor pane widths, keyboard navigation,
  project-relative paths, and Editor-only side panes/drawers.
- Do not add automatic project restoration, imports, writes, analysis, scans, fixes,
  commits, pushes, or a second workflow.
- Do not change daemon routes, serialized API models, persisted data, provider
  configuration, or migration behavior.
- Do not edit generated output, local config, credentials, desktop/build,
  desktop/.gradle, or desktop/.kotlin.

Starting-worktree protocol:
1. Run git status --short, git diff --name-only, and git diff --cached --name-only.
2. Record all pre-existing modifications in the first commentary update. They are
   user-owned, including changes in files also needed by this backlog.
3. Never use reset, checkout, restore, stash, clean, rebase, or another destructive
   history operation to obtain a clean tree.
4. Do not stage or commit any file. Tasks 80–84 authorize implementation and task
   metadata updates only; they do not authorize commits.
5. Before editing a dirty file, inspect its existing diff and preserve every unrelated
   hunk. If required work overlaps inseparably with user-owned changes, stop and report
   the exact file and overlap.

Single-writer rule:
- You are the only implementation writer for the complete sequence.
- Do not spawn parallel implementation agents or create another Codex task.
- Read-only investigation is allowed, but you remain responsible for every decision,
  edit, test, and task status.

Mandatory task-commentary contract:
- “Comment for every task” means a user-facing message in the commentary channel, not a
  source-code comment.
- Before starting each task, post exactly one clearly labeled start update:
  “Starting Task N — <task title>: <scope, likely files, focused verification, and any
  pre-existing changes that must be preserved>.”
- After that task passes verification and its metadata is completed, post exactly one
  clearly labeled completion update:
  “Task N complete — <behavior delivered, tests/results, and next task>.”
- For work lasting more than 60 seconds between those boundaries, post additional short
  progress commentary. Never leave the user without a progress update during a long
  command or investigation.
- Do not combine the completion comment for Task N with the start comment for Task N+1;
  each task must have its own visible boundary comment.
- Do not add source comments merely to satisfy this requirement.

Sequential execution loop for each Task N from 80 through 84:

1. Dependency gate
   - Read Task N completely again.
   - Verify every dependency is Complete in tasks/INDEX.md and its required behavior
     exists in the current worktree.
   - If a dependency is incomplete or materially regressed, stop and report the blocker.
     Do not skip the task or implement a dependent workaround.

2. Start comment and scope
   - Inspect the implicated production code, tests, docs, and current diff.
   - Post the mandatory “Starting Task N” commentary update.
   - Update Task N and its index row to In Progress only when implementation starts.

3. Implement only Task N
   - Make the smallest cohesive change satisfying every Implementation bullet and
     acceptance criterion in Task N.
   - Follow Clean Code, KISS, existing package/state boundaries, and AGENTS.md.
   - Replace obsolete APIs/helpers/branches instead of preserving old/new presentations.
   - Add focused regression tests in the same task as the behavior change.
   - Do not absorb a later task merely because its code is nearby.
   - Continue posting brief commentary if work exceeds 60 seconds.

4. Verify
   - Run every focused command listed in Task N.
   - Run ./desktop/gradlew -p desktop test for Tasks 80–84.
   - Always run git diff --check and inspect the complete task-owned diff.
   - Do not weaken or delete a valid test merely to make the suite green.
   - Check each acceptance criterion explicitly against code and test evidence.

5. Complete task metadata
   - Only after every criterion and required check passes, set Task N to Complete.
   - Move its file to tasks/completed/ without changing its number or filename.
   - Update tasks/INDEX.md to link to completed/... and mark it Complete.
   - Leave every later task Pending.
   - Do not stage or commit these changes.

6. Completion comment and automatic continuation
   - Post the mandatory “Task N complete” commentary update with behavior, commands,
     results, and the next task.
   - Immediately select Task N+1 and repeat without waiting for user confirmation.

Task-specific guardrails:
- Task 80 owns semantic action roles, compact button density, the compact single-line
  input, all control migrations, and their tests. It does not own landing visibility or
  copy removal.
- Task 81 owns the exclusive no-project landing branch, Cmd/Ctrl+O, shortcut gating,
  and open cancel/failure/success transitions. It does not add recent projects or
  automatic restoration.
- Task 82 owns removal of visible counts/revisions/hashes and redundant explanatory copy.
  It must not remove identity fields or weaken any guard.
- Task 83 owns connection/status consolidation, on-demand Bugs filters, compact
  Analyze-all limits, short stage labels, semantic priority, and responsive action groups.
- Task 84 owns final gap fixes, documentation, full validation, manual/reproducible
  acceptance, and only corrections needed to make the plan truthful.

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before declaring a blocker.
- Stop for an inseparable pre-existing change, failed dependency, missing material
  decision, unavailable required dependency, or validation failure that cannot safely be
  fixed within the current task.
- Leave the current task In Progress if implementation began, or Pending if it did not.
- Never skip a blocked task, mark partial work Complete, fabricate test/manual evidence,
  or continue into a dependent task.
- Post a concise blocker commentary, then give a final response naming the exact task,
  file/command, evidence, and user decision or external change required.

Final acceptance after Task 84:
1. Confirm Tasks 80–84 are Complete and linked under tasks/completed/.
2. Confirm PLAN.md status truthfully reflects completion only if all five tasks passed.
3. Run ./desktop/gradlew -p desktop test, make check when permitted, and
   git diff --check.
4. Review git status --short and the complete diff; confirm no pre-existing unrelated
   change was overwritten.
5. Report manual no-project/project-open, wide/exact-1000dp/narrow, keyboard, compact
   control, color/text-state, and read-only/Apply checks honestly.
6. Confirm there are no commits or staged changes created by this execution.

Final response:
- Lead with whether all five tasks completed.
- List Tasks 80–84 with status and one-line behavior/result.
- Summarize all commands and outcomes.
- State any manual check not run and why.
- Confirm revision/hash guards remain internal and unchanged.
- Confirm no API/configuration/data migration or commit was required.
- Confirm pre-existing unrelated worktree changes were preserved.
- Link PLAN.md, tasks/INDEX.md, and key changed files using absolute paths.
```
