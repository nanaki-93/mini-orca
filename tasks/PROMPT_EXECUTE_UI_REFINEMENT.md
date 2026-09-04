# Prompt — execute UI refinement Tasks 150–160 sequentially

This is a future execution prompt. Creating or reviewing it does not start
implementation or create commits. When the user explicitly asks to execute it,
execution includes exactly one reviewed local commit per completed task, as
requested. This does not add automatic Git operations to Mini-Orca itself.

```text
Implement desktop/UI_REFINEMENT_PLAN.md through Tasks 150–160, strictly in numeric
order, using one implementation agent. Do not delegate or run tasks in parallel.
Do not create separate user-owned Codex tasks. Finish, validate, update task
metadata, and create one isolated local commit before beginning the next task.
Continue without asking for "continue" between successful tasks. Do not push.

Read before starting:
1. AGENTS.md and any applicable nested instructions.
2. desktop/UI_DESIGN_GUIDELINES.md and desktop/UI_REFINEMENT_PLAN.md completely.
3. tasks/README.md and tasks/INDEX.md.
4. This execution prompt and all Tasks 150–160 to understand their boundaries.
5. desktop/VISUAL_REVIEW.md, the existing keyboard checklist, and Task 149's
   outstanding acceptance record as evidence, not as a competing execution prompt.
6. Current production code/tests for the selected task; re-read that task completely
   immediately before its implementation. Read baseline/acceptance/component
   decision records from this sequence if already present.

Resolve numbered task files under tasks/ when Pending/In Progress and under
tasks/completed/ when Complete. Require exactly one copy and matching index status.
Inspect the supplied dark mock as a visual reference. Its sample project/code,
unsupported controls, and model-connected label do not define new functionality.

Required outcome:
- Remove the duplicate Project rail destination; keep Editor, the Files tree/drawer,
  real project opening/re-indexing, and the other retained workspaces.
- Reduce routine UI narration without deleting returned results or hiding essential
  consent, safety warnings, freshness, errors, or evidence.
- Redesign Summary using Analysis's accepted metric/surface hierarchy.
- Correct Analysis/page widths while preserving its structure and job behavior.
- Unify popup menus, disclosures, drawers, tool-window controls, and remaining UI.
- Apply the shared IDE design guidelines and remove replaced legacy code in every
  task, not in an optional follow-up.
- Produce exactly eleven task commits on a fresh complete run, with the subjects
  below. On a resumed run, create only the still-missing authorized task commits.

Scope and safety:
- Keep production implementation in desktop/, with task and necessary design/docs
  updates. No daemon/API changes, new backend or fake live data, extra model requests,
  external project execution, new Preview features, or source-editing expansion.
- Preserve one project/file/symbol, read-only/selectable source/diff, isolated draft
  editing, remote-provider destination/consent, evidence identity, guarded explicit
  Apply, receipt, and Undo. Display changes cannot authorize source mutation.
- Preview and disclosure controls remain local-only. Daemon health is not proof of
  model connectivity. Missing/stale/failed values remain explicit; missing is not 0.
- Preserve keyboard paths, accessible names/state, font scaling, the exact 1000dp
  docked/drawer boundary, and saved pane preferences.
- Evaluate Jewel in Task 154 using current primary sources. Do not silently broaden
  this sequence into a Kotlin/Compose/Gradle migration or custom native titlebar.
- Use existing architecture and the checked-in Gradle wrapper. No duplicate theme,
  generic docking engine, parallel renderer, dead Project alias, or copied policies.
- Use apply_patch for local edits. Preserve supplied images, unrelated user work,
  credentials, ignored config.yaml, and external projects. Do not edit generated
  build output by hand or stage it. Test-generated reports may remain ignored.
- Do not run destructive Make/Docker targets or use reset --hard, checkout/restore,
  clean, stash, rebase, amend, squash, tags, or pushes to manage this sequence.

Relationship to the earlier backlog:
- Verify Tasks 140–148's completed implementation and the current visual correction.
  Task 149 is an outstanding native/repository acceptance task, not a hard dependency
  of Task 150 and not a request to execute the old dark-UI prompt first.
- Run this new sequence starting at 150, not the first Pending task anywhere in the
  repository. Keep Task 149 Pending unless separately authorized work verifies its
  own criteria; this sequence must not silently close it or add a twelfth commit.
- Preserve its historical limitations. New evidence can be cross-referenced without
  relabeling old failures or unavailable native checks as passed.

Starting worktree and ownership:
1. Inspect git status --short, git diff --name-only, git diff --cached --name-only,
   relevant full diffs, and recent history. Record starting HEAD and pre-existing
   staged/unstaged/untracked paths. Do not copy sensitive user content into reports.
2. Require an empty index except for fully inspected exact bootstrap artifacts
   belonging to this request. Stop for unrelated staged changes or ambiguous
   partially staged overlap; do not unstage/reset/stash someone else's changes.
3. The following initial planning artifacts may be uncommitted and are owned by
   Task 150 only after their contents and relevant hunks are reviewed:
   - desktop/UI_REFINEMENT_PLAN.md
   - desktop/README.md (only the refinement-plan/backlog hunks)
   - tasks/150_ui_refinement_baseline.md
   - tasks/151_remove_project_navigation.md
   - tasks/152_unify_ide_design_tokens.md
   - tasks/153_correct_workspace_widths.md
   - tasks/154_refine_popup_menus.md
   - tasks/155_refine_disclosures_and_drawers.md
   - tasks/156_redesign_summary_dashboard.md
   - tasks/157_simplify_workspace_copy.md
   - tasks/158_polish_editor_workflow_surfaces.md
   - tasks/159_verify_refined_ui_accessibility.md
   - tasks/160_ui_refinement_acceptance.md
   - tasks/PROMPT_EXECUTE_UI_REFINEMENT.md
   - tasks/INDEX.md (only this sequence's index and prior-sequence clarification)
   - tasks/README.md (only this sequence's workflow updates)
4. Task 150 additionally owns its baseline record and characterization/fixture work.
   Commit the bootstrap artifacts with that tested baseline, not in a separate
   planning-only commit. Already committed bootstrap files need no duplicate commit.
5. All other pre-existing changes are user-owned, even in files relevant to later
   tasks. Do not absorb them. Work around separable hunks; stop if safe ownership
   cannot be established. Inspect formatting scope before broad formatting.
6. Before each commit, confirm HEAD has not changed unexpectedly and review staged
   ownership again. If another writer changes overlapping code/history, reconcile
   with the user rather than guessing or rewriting commits.

Resume and exactly-once rules:
- Find the first incomplete task in the 150–160 sequence. Verify prior completed
  task records, exact subjects, committed metadata, and acceptance evidence.
- A Complete marker without its required successful commit is not completed work.
  If an earlier attempt prepared Complete/moved metadata but commit failed, treat
  that same clearly attributable task as unfinished and finish its existing commit.
- Reuse clearly attributable uncommitted work from the same execution. Do not
  re-implement or re-commit already completed tasks. Ambiguous ownership is a blocker.
- If all eleven commits already exist and the task records are consistent, report
  the existing ledger; do not create extra commits.
- If later testing exposes a regression in a previously committed task, fix it
  within the current task only when it directly blocks that task's acceptance.
  Record the repair there; do not amend history or create a separate fixup commit.
  Stop for authorization if the repair materially expands the approved scope.

For each Task N:
1. Verify dependencies and re-read the task.
   Inspect surrounding code, current diff, and the task's tests and removal list.
   Do not begin later production work while this task is unfinished.

2. Announce and start.
   Post "Starting Task N — <title>" with its scope, likely files, checks, previous
   task commit, and any protected work. Give concise progress updates during work
   lasting more than 60 seconds.
   Set only Task N and its index row In Progress. At Task 150 start, also mark the
   refinement plan In Progress. Do not commit progress/status separately.

3. Implement the bounded replacement.
   Use small cohesive shared components and deterministic behavior tests.
   Remove superseded branches/helpers/callers/tests/styles in the same task.
   Preserve required behavior; no brittle tests asserting source strings instead
   of interaction or state. Do not modify fixtures to conceal an implementation bug.

4. Verify and review.
   Run all task-specific checks and:
     ./desktop/gradlew -p desktop spotlessCheck detekt test
     git diff --check
   Review actual production-component renders for affected visual states.
   Use test-only labeled fixtures, no live paid/provider requests. Record dimensions,
   state, capture provenance, and inspected results. Native capture tools are only
   used where available; do not substitute a mock or HTML for actual Compose evidence.
   Check each acceptance criterion, code removal, naming, errors, and safety.
   Required automated failures or observed in-scope visual/accessibility defects
   block completion; do not suppress checks or weaken assertions to pass.

5. Record complete metadata after acceptance.
   Replace the task's Pending evidence placeholder with actual commands/results,
   affected behavior, removed legacy pieces, visual evidence, and limitations.
   Prepare its Complete status, move the one task file to tasks/completed/, and
   update the INDEX.md status/link in the same task-owned change.
   Fix active links affected by the move; keep identifiers stable.
   At Task 160, update plan/workflow/acceptance status truthfully.
   Do not report Task N complete until its commit actually succeeds.

6. Stage precisely and inspect.
   Stage only Task N implementation/tests/docs, its moved completed record and
   removed pending path, and index/status changes. Task 150 also owns the reviewed
   bootstrap list. Use explicit paths, never git add . or git add -A.
   Use deliberate hunk-level staging only where ownership is unambiguous.
   Inspect git diff --cached --name-status and the complete staged diff.
   Run git diff --cached --check. Exclude user changes, secrets, generated output,
   future feature implementations, and omitted metadata. Tests must correspond
   to the staged task contents, not rely on uncommitted future-task fixes.

7. Commit exactly once with the listed subject.
   No partial, checkpoint, planning-only, status-only, combined-task, or fixup commit.
   Do not amend or bypass hooks. If signing/hooks/identity or another commit step
   fails, keep the task unfinished, report exact state, and do not proceed.
   Obtain the full commit hash and verify its files and completed metadata.
   Confirm no task-owned change remains uncommitted; protected unrelated changes
   may remain untouched. Do not create another commit to record its own hash.

8. Report and continue.
   Post "Task N complete" with the hash, checks, legacy removal, important evidence/
   limitations, and next task. Continue to N+1 without requiring "continue".
   After Task 160, verify the full ledger and hand off; do not start unrelated work.

Exact commit subjects:
150: test(desktop): establish UI refinement baseline (task 150)
151: refactor(desktop): remove duplicate Project navigation (task 151)
152: style(desktop): unify IDE design tokens (task 152)
153: fix(desktop): correct Analysis workspace widths (task 153)
154: style(desktop): refine popup menus (task 154)
155: style(desktop): refine disclosures and tool windows (task 155)
156: feat(desktop): redesign Summary dashboard (task 156)
157: style(desktop): simplify workspace copy (task 157)
158: style(desktop): polish editor and review surfaces (task 158)
159: test(desktop): verify refined UI accessibility (task 159)
160: chore(desktop): complete UI refinement acceptance (task 160)

Verification policy:
- Task 150 runs make check and make quality as fresh baseline diagnostics.
  Required desktop checks must pass. Record exact pre-existing non-desktop failures;
  the old Task 149 report alone is not sufficient evidence of today's baseline.
- Task 160 requires full desktop checks and make check to pass, and reruns make
  quality. New failures or desktop failures block completion. Only a precisely
  verified unchanged non-desktop make quality baseline failure may remain as an
  explicit out-of-scope repository limitation. Do not refactor Go just to hide it.
- Component rendering and interaction evidence are required for changed visuals.
  Native keyboard/window and screen-reader checks must be performed when supported.
  If unavailable, record them as release follow-ups, not passes. Observed defects
  must be fixed; lack of a native tool must not be disguised as full visual acceptance.
- Task 160 may record completed implementation/component acceptance with clearly
  adjacent native/repository limitations; it cannot claim full release acceptance
  or automatically complete Task 149.
- Never run destructive targets. No push, user-project mutation, or live provider
  use is authorized by the test/commit requirement.

Blocking policy:
Exhaust safe in-task diagnostics first. Stop for a missing dependency, inseparable
user-owned overlap, concurrent-writer conflict, necessary out-of-scope migration,
failed required check, in-scope safety/UI defect that cannot be fixed safely, or
commit failure needing user input. Do not skip a task, commit partial work, or mark
an unmet criterion passed. Keep later tasks Pending and report completed hashes,
current worktree/index state, exact blocker, and minimum action needed to resume.

Final handoff:
Report Tasks 150–160 status and one full hash per committed task; major behavior/
visual changes and removed legacy code; checks and inspected evidence; explicit
manual/repository limitations; and migration needs (expected: no API/config change,
old Project navigation recovered by existing preference parsing). No hashes are
reported for commits that were not actually created.
```
