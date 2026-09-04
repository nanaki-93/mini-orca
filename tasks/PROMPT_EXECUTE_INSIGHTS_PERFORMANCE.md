# Prompt — execute Tasks 133–139 sequentially with one commit per task

Use this prompt to implement the complete engineering-insights and Performance
backlog. Creating these task files does not itself start implementation or commits.
When the user asks to execute this prompt, the requested execution includes exactly
one reviewed local commit per task, not automatic commits inside Mini-Orca.

```text
You are the sole implementation agent for Mini-Orca Tasks 133–139.

Objective:
Implement docs/insights-performance/PLAN.md through Tasks 133–139 strictly in numeric
order. Finish, verify, update metadata, and commit each task before beginning the
next. Continue between tasks without asking the user to say “continue”. Stop only
for a genuine blocker or a user instruction. Do not push commits.

Read completely before task actions, in this order:
1. AGENTS.md and any applicable nested instructions.
2. docs/insights-performance/PLAN.md (the active feature plan).
3. plan.md (the completed IDE design and acceptance limitations; do not overwrite it).
4. tasks/README.md and tasks/INDEX.md.
5. tasks/PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md.
6. tasks/completed/132_ide_ui_acceptance.md.
7. 133_insights_performance_contract_baseline.md.
8. 134_engineering_insight_generation.md.
9. 135_inline_engineering_insight_panel.md.
10. 136_performance_file_review.md.
11. 137_project_performance_job.md.
12. 138_performance_workspace.md.
13. 139_insights_performance_acceptance.md.
14. Current production code, tests, build/check scripts, and maintained docs relevant
    to the first incomplete task; read existing BASELINE.md/ACCEPTANCE.md if present.

Resolve each numbered task file under tasks/ if Pending/In Progress, or under
tasks/completed/ if Complete. Require exactly one copy and consistent index state.
Re-read the selected task completely immediately before working on it.

Required product outcome:
- Small concise optional Engineering insight panels embedded in existing result
  pages/tool windows; they can close/reopen without provider work or loss of content.
- No games, quizzes, challenges, ratings, learning profiles, tracking, journals,
  curricula, Learn workspace, or Learn tab.
- A separate Performance section beside Analysis, explicitly labelled
  “Source-based review · Not measured”.
- Explicit bounded project performance review, truthful coverage and partial/error
  states, source-grounded opportunities, conditions/trade-offs, and verification plans.
- Preview-first optimization handoff to one exact eligible Go declaration through
  existing validation/check/Apply/Undo safeguards.
- Exactly seven isolated local task commits with the subjects listed below.

Non-negotiable boundaries:
- Source/diff remain selectable and read-only. Analysis, generation, navigation,
  panel disclosure, findings selection, validation, and checks never write source.
- Apply/Undo remain the only explicit guarded source mutations. Preserve project,
  revision, path, hash, target, task, draft revision/hash, checks, and confirmation.
- Performance does not run the target project, benchmarks, profilers, a terminal,
  telemetry collector, or commands supplied by a model. Do not add measured metrics,
  invented speedups, project performance scores, or a “project is fast” verdict.
- Running Mini-Orca's own development tests is required verification and is distinct
  from adding project-code execution to the Performance product feature.
- A panel opening or workspace selection cannot initiate provider work. Insights
  arrive with existing parent calls; Performance starts/resumes explicitly.
- Insight text is optional, advanced, concise, grounded, advisory, and separate from
  finding identity or Apply authority. A bad optional note must not reject good code.
- Performance uses the existing analyze model explicitly; other operations keep their
  scopes. No new required profile, silent fallback, credentials, or local config edit.
- Remote approval covers the previewed run queue, project, policy, and destination;
  another operation's approval cannot authorize it. Preserve safe context policies,
  loopback binding, bounded requests, source-free metadata, and sanitized diagnostics.
- Performance cache/jobs remain independent of bug analysis and must not reconcile
  under the generic ai finding source or retire bug findings.
- Preserve the finished IDE palette, source-first layout, existing Cmd/Ctrl+1–4
  mappings, keyboard/focus semantics, and the exact 1000dp responsive boundary.
- Keep effects in existing service/presenter boundaries and presentation preferences
  free of workflow authority. Reuse concrete shared policy; do not duplicate job
  controllers, build a general agent framework, or retain obsolete implementations.
- One implementation agent owns writes throughout the sequence. Do not delegate,
  implement tasks in parallel, or create another user-owned Codex task.
- Never modify generated build output, credentials, ignored config.yaml, unrelated
  user projects, or protected user work. Do not run destructive make/Docker targets.

Starting worktree and bootstrap gate:
1. Inspect git status --short, git diff --name-only, git diff --cached --name-only,
   relevant full diffs, and recent history. Record the starting HEAD and every
   pre-existing unstaged, staged, and untracked path before editing.
2. Verify Task 132 is Complete and its behavior/commit exists. Respect any recorded
   historical exceptions or manual limitations; do not rewrite earlier task history.
3. For Task 133 only, the exact initial planning artifacts below may be uncommitted
   and belong in its baseline commit after full inspection:
   - docs/insights-performance/PLAN.md
   - tasks/133_insights_performance_contract_baseline.md
   - tasks/134_engineering_insight_generation.md
   - tasks/135_inline_engineering_insight_panel.md
   - tasks/136_performance_file_review.md
   - tasks/137_project_performance_job.md
   - tasks/138_performance_workspace.md
   - tasks/139_insights_performance_acceptance.md
   - tasks/PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md
   - task-sequence-specific hunks in tasks/README.md and tasks/INDEX.md
4. Require an empty index except for fully reviewed exact Task 133 bootstrap artifacts.
   If unrelated staged changes or ambiguous partially staged hunks exist, stop and
   report them; do not commit, reset, unstage, or stash someone else's work.
5. Treat every other pre-existing modification as user-owned, including AGENTS.md
   and daemon changes even when their filenames are relevant to later tasks. Never
   absorb them into this sequence. Work around separable hunks; stop if the overlap
   cannot be isolated safely.
6. Root PLAN.md and plan.md may resolve to the same file on this filesystem. Keep the
   feature plan under docs/insights-performance/PLAN.md; no case-only root rename or
   replacement belongs to Tasks 133–139.
7. Before broad formatting, inspect whether it would alter user-owned files/hunks.
   Never use formatting to absorb unrelated work; stop if required compliance cannot
   be achieved without an inseparable user change.
8. Do not use reset --hard, checkout/restore, stash, clean, rebase, or destructive
   history operations to manufacture a clean tree. Never create a planning-only commit.
9. Preserve the starting baseline and inspect HEAD before each task commit. If another
   writer commits or changes overlapping work during execution, stop and reconcile
   ownership with the user rather than rewriting or guessing at the new history.

Resume rules:
- Identify the first incomplete task in INDEX.md; validate earlier task completion
  against committed metadata, required subjects, and actual acceptance behavior.
- Do not redo completed tasks or make duplicate commits. “Complete” text without
  its required successful commit is not evidence that the task finished.
- On a continuation of the same implementation, retain its recorded ownership and
  baseline. Uncommitted changes are resumable only when clearly attributable to that
  same task. If attribution is ambiguous, stop instead of claiming them.
- If all seven tasks are already complete and committed, report the existing ledger
  and do not create another commit.

Mandatory user-facing commentary:
- Before work on Task N, post “Starting Task N — <task title>: ...” with the boundary,
  likely files, focused checks, prior commit, and protected pre-existing work.
- Give concise progress updates during work lasting more than 60 seconds.
- After the commit succeeds, post “Task N complete — ...” with behavior, affected
  files, checks/results, exact full commit hash, limitations, and the next task.
- These are conversation updates, not instructions to add commentary to source code.

Sequential loop, for each Task N from 133 through 139:

1. Verify dependency and ownership
   - Read Task N again; it must be the first ready incomplete task.
   - Check all prior required commits exist and no prior task-owned work remains
     staged/uncommitted. Unrelated recorded unstaged user edits may remain untouched.
   - Inspect current code/diffs and understand the surrounding design before editing.

2. Start exactly this task
   - Post its Starting Task N update.
   - Set only its task status and INDEX.md row to In Progress. Do not commit that
     marker separately or mark later tasks In Progress.
   - At Task 133 start, also mark the feature plan In Progress and the index sequence
     active, replacing preparation-only wording. At Task 139 completion, update them
     to Complete with truthful limitations as specified below.

3. Implement its complete bounded behavior
   - Follow AGENTS.md, the active feature plan, this prompt, and the task criteria.
   - Use apply_patch for local edits, normal approved formatting tools where safe,
     Go 1.22 production boundaries, and the checked-in Desktop Gradle wrapper.
   - Add deterministic tests for changed behavior and meaningful error/edge cases.
     Use fake providers/temporary fixtures; never require live model calls in tests.
   - Replace superseded code and remove obsolete helpers/callers/tests in the same
     task. Do not keep legacy alternatives or add speculative abstractions.
   - Do not implement a later task just because nearby code is open. Initial future
     task specifications included by Task 133 are not future feature implementation.
   - Keep required schemas/routes/docs synchronized when their behavior actually
     ships; do not register unsupported endpoints or change local credentials.

4. Verify and review before completion
   - Run every command listed in Task N, plus git diff --check.
   - Inspect the full task-owned diff: naming, error paths, validation, ownership,
     privacy, stale/late results, tests, code removal, and narrowly scoped changes.
   - Explicitly check each acceptance criterion. Do not suppress safety failures,
     weaken tests, introduce test-only production branches, or hide quality findings.
   - Record concise commands/results and manual evidence in Task N before staging.
   - Environment-dependent manual viewport/provider/screen-reader checks may remain
     not run only where the task permits a recorded release follow-up. A real defect
     or a failed/unavailable required automated check blocks the task.

5. Prepare complete metadata in the same commit
   - After checks pass, mark Task N Complete, move its file to tasks/completed/
     without changing its identifier, and update its INDEX.md status/link.
   - Keep one task file, not a Pending copy beside a completed replacement. Fix any
     live documentation links affected by the move.
   - For Task 139, update feature-plan status and the acceptance/workflow docs, with
     every permitted manual limitation adjacent to the completion claim.
   - Metadata is only prepared at this point; do not report completion until the
     isolated task commit actually succeeds.

6. Stage exactly owned content
   - Stage only Task N implementation, tests, documentation, completed task record,
     removed pending path, and index update. Task 133 additionally owns its reviewed
     exact bootstrap artifact list, including still-Pending Tasks 134–139.
   - Use explicit path-level staging for wholly owned files and deliberate hunk-level
     staging for safely separable changes; never use broad git add -A or git add .
   - Inspect git diff --cached --name-status and the complete git diff --cached;
     run git diff --cached --check.
   - Verify no source secrets, local config, generated output, unrelated staged user
     work, omitted task metadata, or future feature implementation is included.

7. Commit exactly once
   - Use the exact subject for Task N from the list below.
   - Do not create checkpoint, planning-only, status-only, fixup, partial, or combined
     task commits. Do not amend, reword, squash, tag, rebase, or push.
   - If commit creation fails, the task is not complete. Resolve safely or report
     the exact staged state/error; do not proceed to a dependent task.
   - Obtain the full commit hash. Confirm the committed task status/move/index are
     correct and no Task N change remains staged or uncommitted.
   - Do not make a second commit to record this commit's own hash inside itself.

8. Report and proceed
   - Post the Task N complete update with exact hash and checks.
   - Immediately continue to Task N+1. After Task 139, perform final ledger checks
     and report; do not start an unrelated feature or create extra commits.

Task ownership and exact commit subjects:
133: Baseline, characterization, contract decisions, and initial planning artifacts.
     test: establish insights and performance baseline
134: Shared insight validation; generation/transport/parent persistence and freshness.
     feat(analysis): add contextual engineering insights
135: Reusable in-page insight panel, existing result surfaces, disclosure/accessibility.
     feat(desktop): add collapsible engineering insights
136: Bounded single-file performance review, policy-safe context, parsing and cache.
     feat(performance): add source-based file review
137: Explicit bounded project job, coverage, restart/concurrency and full loopback API.
     feat(performance): add bounded project review jobs
138: Performance navigation/page, controls/details, provider scope and guarded handoff.
     feat(desktop): add Performance workspace
139: Full feature/regression/privacy acceptance, cleanup, docs and final status.
     chore: complete insights and performance acceptance

Blocking policy:
- Exhaust safe in-task diagnostics and fixes first. Do not stop merely because the
  work is long, but do stop for a missing dependency, inseparable user-owned overlap,
  concurrent-writer conflict, missing material product authority, failed safety
  invariant, required check that cannot pass safely in scope, or isolated commit
  failure that needs user action.
- Do not skip the blocked task, implement dependencies out of order, silently expand
  the approved product scope, fabricate evidence, or commit partial work.
- Keep later tasks Pending. Report the blocker, exact worktree/staging state, work
  already completed, and the minimal user action needed to continue.

Final acceptance after the Task 139 commit:
1. Confirm all seven task files are Complete under tasks/completed/ with valid INDEX.md
   links and no duplicate pending copies.
2. Inspect history from the recorded pre-133 HEAD. Confirm exactly seven task commits
   with the exact ordered subjects, each containing its complete task metadata and
   only owned work. Do not rewrite unrelated historical IDE commits.
3. Confirm make fmt-check, go test ./..., make test-race, make vet, Desktop Spotless/
   Detekt/tests, make check, make quality, and diff checks passed before final commit.
4. Report manual content/viewport/keyboard/provider acceptance truthfully; no fabricated
   model quality, runtime measurements, or claims about unavailable environments.
5. Confirm feature-plan/API/configuration/README docs match the implemented behavior,
   source-safe limits, and actual migration requirements.
6. Confirm no sequence-owned change is uncommitted, protected user edits are preserved,
   and nothing was amended, combined, tagged, or pushed.

Final response:
- Lead with whether all seven tasks and the feature plan completed.
- Provide a compact Task 133–139 table with status, exact full commit hash, and checks.
- Summarize collapsible insights, Performance's source-based limitations, safeguards,
  automated/manual results, remaining release follow-ups, and configuration/cache
  migration requirements (or explicitly state none).
- State that commits were not pushed.
```
