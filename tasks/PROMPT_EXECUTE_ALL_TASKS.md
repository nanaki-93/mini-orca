# Prompt — execute Tasks 85–89 sequentially

Use this prompt with one primary Codex implementation agent. The agent owns the shared
worktree, completes Tasks 85–89 in numeric order, and posts a user-facing commentary
update before and after every task.

```text
You are the primary implementation agent for Mini-Orca Tasks 85–89.

Objective:
Implement the direct-symbol requirements recorded in Tasks 85–89. Those task files are
authoritative for this preceding backlog; the current PLAN.md owns the later Tasks
90–94 refinement. Execute Tasks 85, 86, 87, 88, and 89 strictly sequentially. Finish
and verify each task before starting the next one. Do not ask the user to say “continue”
between tasks.

Read completely before acting, in this order:
1. AGENTS.md
2. PLAN.md
3. tasks/README.md
4. tasks/INDEX.md
5. tasks/85_editor_symbol_selection_contract.md
6. tasks/86_interactive_source_symbol_inspector.md
7. tasks/87_direct_selected_symbol_editing.md
8. tasks/88_contextual_editor_review_flow.md
9. tasks/89_direct_symbol_ux_acceptance.md
10. Current production files, tests, and documentation implicated by Task 85

Required outcome:
- Clicking within an indexed declaration in read-only source selects and highlights that
  declaration and shows its explanation in the right panel.
- Clicking outside declarations clears stale symbol context while keeping the focused
  line.
- The Editor has no File analysis tab and no persistent list of every symbol.
- An eligible exact atomic Go symbol exposes one direct target-naming Edit action;
  Replace is implied and Send remains explicit.
- Create declaration remains available as a secondary Commands route.
- An existing draft is never silently retargeted or discarded by source inspection.
- The Editor uses contextual Inspect → Edit → Review progress rather than surface/stage
  navigation buttons.
- Review contains validation, explicit focused checks, exact guarded Apply, receipt, and
  Undo without a redundant Continue to Apply action.

Non-negotiable product boundaries:
- Preserve the one-project, one-open-file, one-symbol, preview-first workflow.
- Source and composed diff remain selectable and read-only.
- Only the isolated declaration/import draft is editable before Apply.
- Selection and Edit perform no model call or write. Analyze, Refresh, Send, Validate,
  checks, Apply, and Undo remain explicit.
- Remote-provider confirmation and every project revision, file hash, draft identity,
  and asynchronous request guard remain intact.
- Apply names and mutates only the exact selected file/declaration scope.
- Preserve the 1000dp breakpoint, saved Editor pane widths, keyboard navigation,
  project-relative paths, and Editor-only side panes/drawers.
- Do not change daemon routes, serialized models, persistence, provider configuration,
  or migration behavior unless a task explicitly stops on a proven contract blocker.
- Do not edit generated output, local config, credentials, desktop/build,
  desktop/.gradle, or desktop/.kotlin.

Starting-worktree protocol:
1. Run git status --short, git diff --name-only, and git diff --cached --name-only.
2. Record pre-existing changes in the first commentary update and preserve them as
   user-owned, including changes in files this backlog needs.
3. Never use reset, checkout, restore, stash, clean, rebase, or another destructive
   history operation to obtain a clean tree.
4. Do not stage or commit any file. Tasks 85–89 authorize implementation and task
   metadata updates only.
5. Before editing a dirty file, inspect its diff. Stop if the required change overlaps a
   user-owned hunk inseparably.

Single-writer rule:
- You are the only implementation writer for the complete sequence.
- Do not spawn parallel implementation agents or create another Codex task.
- Read-only investigation is allowed, but you remain responsible for every decision,
  edit, test, and task status.

Mandatory task-commentary contract:
- Before Task N, post one update beginning exactly with:
  “Starting Task N — <task title>: ...”
- After Task N passes and metadata is complete, post a separate update beginning exactly
  with:
  “Task N complete — ...”
- Include scope, likely files, focused verification, and pre-existing changes at start;
  include behavior, commands/results, and next task at completion.
- Post concise progress commentary at least every 60 seconds during long work.
- These are user-facing commentary messages, not source-code comments.

Sequential execution loop for each Task N from 85 through 89:

1. Dependency gate
   - Read Task N completely again.
   - Verify every dependency is Complete in tasks/INDEX.md and its behavior still exists.
   - Stop on an incomplete or materially regressed dependency.

2. Start and scope
   - Inspect implicated code, tests, docs, and current diffs.
   - Post the mandatory Starting Task N commentary.
   - Mark Task N and its index row In Progress only when implementation begins.

3. Implement only Task N
   - Make the smallest cohesive change satisfying every implementation bullet and
     acceptance criterion.
   - Follow Clean Code, KISS, existing state/package boundaries, and AGENTS.md.
   - Remove superseded implementations; do not leave old/new interaction models side by
     side.
   - Add focused regression tests with the behavior change.
   - Do not absorb a later task because its code is nearby.

4. Verify
   - Run every focused command listed by Task N.
   - Run ./desktop/gradlew -p desktop test for every task.
   - Run git diff --check and inspect the complete task-owned diff.
   - Verify every acceptance criterion explicitly. Do not weaken valid tests to pass.

5. Complete metadata
   - Only after all criteria and checks pass, mark Task N Complete, move it under
     tasks/completed/, and update tasks/INDEX.md.
   - Leave later tasks Pending. Do not stage or commit.

6. Report and continue
   - Post the mandatory Task N complete commentary.
   - Immediately continue to Task N+1 without waiting for user confirmation.

Task-specific guardrails:
- Task 85 owns pure line-selection, inspector/edit eligibility, current-edit identity,
  and contextual progress contracts. It does not change visible Compose interaction.
- Task 86 owns source clicking, selected-symbol inspector presentation, removal of File
  analysis/all-symbol surfaces, and narrow Context opening. It does not yet redesign the
  edit/review workflow.
- Task 87 owns direct Replace editing, Commands-only Create, and explicit protection
  against discarding/retargeting another draft.
- Task 88 owns removal of stage navigation, contextual progress, automatic presentation
  transitions, and consolidated Review/check/Apply/receipt UI.
- Task 89 owns final gap fixes, documentation, full regression/visual/accessibility
  acceptance, and truthful plan completion status.

Blocking policy:
- Investigate failures and exhaust safe task-scoped fixes before declaring a blocker.
- Stop for an inseparable pre-existing change, failed dependency, missing material
  decision, unavailable required dependency, or validation failure that cannot safely be
  fixed within the current task.
- Leave a started blocked task In Progress; otherwise leave it Pending.
- Never skip a blocked task, mark partial work Complete, fabricate evidence, or continue
  into a dependent task.

Final acceptance after Task 89:
1. Confirm Tasks 85–89 are Complete and linked under tasks/completed/.
2. Leave the current PLAN.md status unchanged; it remains Proposed for Tasks 90–94.
3. Run ./desktop/gradlew -p desktop test, make check when permitted, and
   git diff --check.
4. Review git status --short and the complete diff; confirm no unrelated change was
   overwritten and no file is staged.
5. Report click versus drag source behavior, nested ranges, symbol palette, direct Edit,
   command-only Create, draft-retarget protection, Review/Apply/Undo, and wide/exact
   1000dp/narrow behavior honestly.
6. Confirm source/diff remain read-only, identity guards remain internal and unchanged,
   and no API/configuration/data migration or commit was required.

Final response:
- Lead with whether all five tasks completed.
- List Tasks 85–89 with status and one-line result.
- Summarize commands and outcomes plus any manual check not run.
- Confirm safety/identity boundaries and preserved unrelated work.
- Link PLAN.md, tasks/INDEX.md, and key changed files using absolute paths.
```
