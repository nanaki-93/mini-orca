# Executing the Mini-Orca UX implementation queue

[PLAN.md](../PLAN.md) owns confirmed product decisions and current status.
[docs/tasks.md](../docs/tasks.md) is the sole ordered implementation checklist.
The user authorized scheduled execution with **GPT-6 Astra Extra High** on
2026-09-11. All 17 cards completed on **2026-09-12**. The scheduler ran one card per wake,
every 20 minutes, in the current checkout on `codex/autopilot`, and is now Paused.

Automation: **Mini-Orca UX implementation** (`mini-orca-ux-implementation`), **Paused**,
attached to the current Codex task. Astra Extra High is set on that task through
the supported task continuation API; chat-attached automation has no independent
model/effort field. Local execution requires the computer on and the app running,
as described in the [scheduled-task documentation](https://learn.chatgpt.com/docs/automations?surface=app).

## Per-wake procedure

1. Read current `AGENTS.md`, the status ledger and the first unchecked task card.
   Re-read `desktop/UI_DESIGN_GUIDELINES.md` before UI work. Respect an explicit
   user pause. Never start another writer while recorded work is still running;
   inspect the recorded process/session before deciding it is abandoned.
2. Record the active task and stage in PLAN.md. Inspect the current working-tree
   diff before editing. Earlier accepted changes and the user's uncommitted UI
   work form the baseline; never reset, stash, discard or overwrite them.
3. Implement only that card with `gpt-6-astra` and `xhigh` reasoning. The scheduler
   uses this Codex task directly; no detached runner or additional agents are
   required. Necessary narrow target-list corrections follow `docs/tasks.md`.
4. Run the exact task verification commands with the verified local toolchains.
   For desktop changes, also run the full desktop tests required by AGENTS.md.
   Reuse valid cached results; do not repeatedly rerun passing suites without a
   relevant change. Perform and record any native checks required by the card.
5. Review the actual diff for correctness, failure paths, stale results, privacy,
   maintainability, removed obsolete code and preserved behavior. Make at most
   two focused repairs after a concrete failure, retaining all failed attempts.
   An interrupted validation resumes validation, not implementation from scratch.
6. When the card passes every required check and diff review, prepare its checkbox,
   actual test/review record and PLAN.md status update. Stage only that task's
   implementation and related documentation, inspect the staged diff, and create
   one local commit with the task ID in its subject. Preserve unrelated staged
   changes and exclude unrelated pre-existing edits; never use blanket staging.
   Verify/report the commit hash, then end the wake. If the commit fails, record
   the pending commit stage and pause without advancing. After interruption,
   inspect Git history and any execution receipt before retrying; do not duplicate
   a commit or rerun completed implementation. A later wake handles the next card
   only after the previous task's commit is verified.
7. On an exhausted repair, missing required native capability or material product
   ambiguity, preserve work, record the precise blocker in `docs/errors.log` and
   pause this scheduler. When all 17 cards pass, record completion and pause it.
   Update scheduling only with the app's automation tool, preserving its other
   fields; do not write raw scheduler files or reactivate historical automations.

## Boundaries

The user authorized **one local commit per validated task** on 2026-09-11. This
supersedes the earlier no-commit instruction for this queue. Do not push, release, publish, run live
Mini-Orca provider/evaluation campaigns or perform destructive cleanup. The user's
ordinary terminal is a feature being implemented, not authorization for model-driven
commands in project terminals. Preserve source/diff read-only views and explicit
Review/Apply for Mini-Orca candidates. Normal build dependency downloads and tests
using fake providers/temporary fixtures are included in the implementation scope.

The existing scheduler and this task serialize execution; no extra worktrees or
parallel coordinators should be launched. Keep Astra Extra High for this queue
unless the user explicitly changes the requested model. Report an unavailable
requested model rather than silently substituting another one.

## Historical work

The completed CLN-01–12 queue and its past agent/commit authorization are retained
in [cleanup history](../docs/history/cleanup-tasks-2026-09-10.md). Its scheduler
remains paused. The repository dispatcher and insight qualification remain on
standby; their old unchecked boxes and imperative instructions are not active work.
Keep historical receipts, verdicts, budgets and sealed holdout material unchanged.

- [Historical implementation/dispatcher guide](../docs/history/improvement-plan-2026-09.md#implementing-the-plan-with-agents)
- [Historical runtime/evaluation](../docs/history/insight-evaluation-2026-09.md)
- [Release evidence and limits](../docs/RELEASE_ACCEPTANCE.md)

## Final queue acceptance — 2026-09-12

All 17 cards passed and received their authorized local commits. VERIFY-01 closes
full validation, native macOS arm64/JBR 25 terminal proof, visual/keyboard checks
and migration documentation. The scheduler was paused through the app; its prompt,
interval, task target and Astra Extra High setting are retained. No future card,
live provider campaign, push or release is scheduled. See the
[final evidence and limits](../docs/RELEASE_ACCEPTANCE.md#ux-implementation-acceptance--2026-09-12).
