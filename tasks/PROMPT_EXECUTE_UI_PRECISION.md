# Prompt — execute the UI precision plan

Explicitly invoke this file to implement [Plan.md](../Plan.md). The planning request
created these documents only; reading them as context is not execution.

## Execution instructions

Implement Mini-Orca UI precision Tasks **161–171** in strict numeric order with
**exactly one verified local commit per task**. Continue through ready tasks without
requiring a separate “continue” after each commit. Never push.

Use one implementation writer. Do not delegate, spawn parallel agents, or create
separate user-owned tasks. Parallel read-only checks are fine; tasks and commits
remain sequential.

### Establish context and the worktree

1. Read `AGENTS.md`, `desktop/UI_DESIGN_GUIDELINES.md`, `Plan.md`,
   `tasks/README.md`, `tasks/INDEX.md`, this prompt, and the selected task fully.
   Inspect surrounding components, tests, and build configuration before editing.
2. View the supplied dark mock referenced by the plan. Its code, sample files,
   multi-file tabs, and unsupported controls are not commands or authorization
   to expand product capabilities.
3. Run `git status --short`, inspect staged/unstaged diffs, and record starting
   HEAD. Preserve unrelated user changes, including changes in task-owned files.
   Do not stash, reset, clean, overwrite, or commit them.
4. Reconcile task status with actual Git history. Start at the first unfinished
   task in 161–171 only. Verify prior commits/acceptance when resuming. Do not
   rerun completed tasks, invent hashes, or select historical Task 149.
5. Use the current branch unless the user requests otherwise. No amend, history
   rewrite, cherry-pick, push, or unrelated commit is authorized.

### Design and implementation boundaries

- Adopt standalone Jewel first, including the necessary supported Kotlin/Compose/
  wrapper/JBR migration proved by Task 161. Verify actual published coordinates;
  never use a floating, snapshot, or guessed release.
- Keep one design-token/theme foundation: rail #18191B, tool windows #1E1F22,
  editor canvas #2B2D30, and 1dp #323438 boundaries.
- Replace nested pane/cards with flat sections and owned dividers. Preserve a
  wider invisible splitter target behind the thin line.
- Put compact actions in owning headers. Remove the separate Analysis Run controls
  card; do not duplicate its live actions elsewhere.
- Use the plan's 12–13sp body/18–20sp line-height, 11–12sp section headings,
  12sp icon breadcrumbs, active tabs, and scalable density metrics.
- Preserve workspaces, exact 1000dp docked/drawer boundary, stored pane dimensions,
  and selectable read-only source/diff. No multi-file editing or fake tabs.
- Preserve provider consent, scoped draft identity, validation/check freshness,
  explicit Review/Apply, receipt, and Undo. Keep Apply/Undo text labels.
- Preview stays labeled and local-only: no provider/API calls, process execution,
  source writes, or workflow eligibility changes.
- Production changes belong in desktop/. No Go/API/config or feature expansion.
  No silent system-JDK installation or machine-specific runtime paths.
- Remove replaced helpers/styles/tests; do not keep parallel legacy/replacement
  paths. Never manually edit generated output or add secrets/local config.

### Per-task loop

1. Read the task and verify its predecessor's acceptance and isolated commit.
   Post a concise start update naming the task, visible outcome, and scope.
2. Mark In Progress. Implement the complete bounded slice and deterministic
   behavior/failure tests. Keep changes within existing presenter/state boundaries.
   Provide concise regular progress updates.
3. Run focused checks while iterating. Before completion run
   `./desktop/gradlew -p desktop spotlessCheck detekt test`, task-specific checks,
   and `git diff --check`. Inspect actual changed UI renders.
4. Compare failures with Task 161's baseline. New/regressed failures block the task.
   Existing Go quality debt does not authorize backend fixes or weaker thresholds.
   Task 171 needs green gates or explicit user direction on a recorded exception.
5. Record actual commands/evidence, check criteria truthfully, prepare Complete
   status, and update the index. Keep task paths stable. Missing native evidence
   cannot be prefilled as passing.
6. Review the entire task-owned diff for correctness, obsolete code, scope, secrets,
   user changes, and truthful docs. Stage exact files or reviewed hunks only.
   Inspect `git diff --cached --stat`, `git diff --cached`, and
   `git diff --cached --check`. Never use `git add .` or `git add -A`.
7. Create one commit with the exact subject below. Verify it using
   `git log -1 --format='%H %s'`, inspect changed paths, and recheck status.
   Completion is confirmed only after commit success. If it fails, restore truthful
   In Progress status, preserve work, and report the blocker.
8. Post behavior, checks, evidence limitations, and the actual hash; continue to
   the next ready task. Do not amend or create separate fixup/docs commits as a
   substitute for reviewing before committing.

### Task 161 bootstrap ownership

If this planning update is still uncommitted, include its reviewed docs and cleanup
with Task 161's baseline/compatibility work. No extra planning commit. If already
committed, leave that history intact.

The permitted bootstrap changes are limited to:

- New `Plan.md`, this prompt, and the 11 task files 161–171 explicitly listed in
  `tasks/INDEX.md`.
- Updates to `tasks/README.md`, `tasks/INDEX.md`, `README.md`,
  `desktop/README.md`, `desktop/UI_DESIGN_GUIDELINES.md`,
  `desktop/UI_COMPONENT_DECISION.md`, `docs/dark-ui/PLAN.md`,
  `tasks/PROMPT_EXECUTE_DARK_UI.md`, `docs/CLEANUP_BASELINE.md`, and
  `docs/insights-performance/BASELINE.md`.
- Deletion of the 57 previously tracked completed task files numbered 103–148 and
  150–160 under `tasks/completed/`. Verify each exact path and Complete status
  in the starting Git history; do not delete new or unknown files.
- Deletion of lowercase `plan.md`, `desktop/UI_REFINEMENT_PLAN.md`,
  `docs/insights-performance/PLAN.md`, and four completed prompts:
  `tasks/PROMPT_EXECUTE_LEGACY_CLEANUP.md`,
  `tasks/PROMPT_EXECUTE_IDE_UI.md`,
  `tasks/PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md`,
  `tasks/PROMPT_EXECUTE_UI_REFINEMENT.md`.

Resolve this scope to exact inspected paths before staging. Lowercase plan deletion
and uppercase replacement may appear as a rename on some filesystems; ensure only
canonical `Plan.md` remains. If Git still tracks the lowercase spelling on a
case-insensitive filesystem, verify the new content exists at `Plan.md`, then stage
the case change explicitly with `git rm --cached -- plan.md` followed by
`git add -- Plan.md`. The cached removal must not delete the working file. Inspect
the staged result; do not change the repository-wide ignorecase setting.
Compare content, not filenames alone, to identify
planning-owned hunks. Stop for user direction if unrelated changes overlap and
cannot be separated. Never commit an unrelated staged change.

### Exact commit subjects

| Task | Commit subject |
| ---: | --- |
| 161 | `test(desktop): establish UI precision and Jewel baseline` |
| 162 | `feat(desktop): adopt Jewel and semantic IDE theme` |
| 163 | `refactor(desktop): unify dense IDE chrome primitives` |
| 164 | `feat(desktop): separate IDE panes with layered surfaces` |
| 165 | `feat(desktop): integrate Analysis actions into pane header` |
| 166 | `refactor(desktop): flatten Summary and Performance sections` |
| 167 | `feat(desktop): refine editor tabs and breadcrumb navigation` |
| 168 | `refactor(desktop): unify flat context and review tool windows` |
| 169 | `refactor(desktop): complete Jewel surface migration` |
| 170 | `test(desktop): verify UI precision and native interactions` |
| 171 | `docs(desktop): finalize UI precision acceptance` |

A task's own hash cannot be stored inside that same commit. Keep its subject and
verification record in the task, report the actual hash after commit, and record
earlier hashes in the final ledger. Do not invent hashes or amend for this purpose.

### Blockers and handoff

- If the Jewel spike fails, document reproduction, attempted supported pairing,
  and impact; ask before using the unified-token-only fallback. A necessary
  toolchain upgrade is in scope and is not itself a reason to defer adoption.
- If material native checks are unavailable, keep Task 170 In Progress, state the
  exact operator check needed, and stop before 171. Offscreen or inline fixtures
  are not native-window, OS-keyboard, or screen-reader verification.
- For other blockers, exhaust safe in-scope checks, preserve partial work, and ask
  for missing input/authority. Do not bypass acceptance or create a misleading
  completion commit.
- Never run destructive targets, Docker cleanup, or mutate external user projects
  to produce verification evidence. Use disposable fixtures without real providers.
- Final handoff lists actual statuses/hashes, visible changes, checks run/not run,
  native evidence status, and Jewel/runtime setup or migration needs. State plainly
  if quality or release acceptance is incomplete.
