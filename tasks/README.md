# Executing the Mini-Orca UI polish queue

[PLAN.md](../PLAN.md) owns confirmed product decisions and current status.
[docs/tasks.md](../docs/tasks.md) is the sole ordered implementation checklist.
The active 2026-09-15 scope is **POLISH-01–06** at the top of the task file. The
completed 17-card queue from 2026-09-12 is historical evidence below that queue.

Automation: **Mini-Orca UI polish** (`mini-orca-ux-implementation`), **Active**,
every 20 minutes and attached to the current Codex task. Local execution requires
the computer on and the app running, as described in the
[scheduled-task documentation](https://learn.chatgpt.com/docs/automations?surface=app).

POLISH-01 passed its user-authorized continuation: category layout, real pointer
and keyboard activation, names/selection, all 456 desktop tests and quality checks.
The [card evidence](../docs/tasks.md#task-polish-01--shared-rounded-category-boxes)
retains previous failures and distinguishes component checks from native evidence.
POLISH-02 also passed after Astra High repair 1/1: Summary alignment, whole-box
navigation and live counts, with all 460 desktop tests and quality checks passing.
Its [card evidence](../docs/tasks.md#task-polish-02--summary-alignment-navigation-and-live-data)
records the interrupted handoff and completed repair. POLISH-03 passed Astra High
repair 1/1, all 462 desktop tests and quality checks. POLISH-04 is next; no later
card has started. Scheduled implementation uses SOL High and repairs use Astra High.
The user's follow-up authorizes local commits after each accepted card;
POLISH-01/02 are committed together as `d67ab5a`, and POLISH-03 is committed separately.

## Per-wake procedure

1. Read current `AGENTS.md`, the status ledger and the first unchecked task card.
   Re-read `desktop/UI_DESIGN_GUIDELINES.md` before UI work. Respect an explicit
   user pause. Never start another writer while recorded work is still running;
   inspect the recorded process/session before deciding it is abandoned.
2. Record the active task and stage in PLAN.md. Inspect the current working-tree
   diff before editing. Earlier accepted changes and the user's uncommitted UI
   work form the baseline; never reset, stash, discard or overwrite them.
3. Implement only that card with SOL High (`gpt-5.6-sol`, `high`). The scheduler
   uses this Codex task directly; no detached runner or additional agents are
   required. Necessary narrow target-list corrections follow `docs/tasks.md`.
4. Run the exact task verification commands with the verified local toolchains.
   For desktop changes, also run the full desktop tests required by AGENTS.md.
   Reuse valid cached results; do not repeatedly rerun passing suites without a
   relevant change. Perform and record any native checks required by the card.
5. Review the actual diff for correctness, failure paths, stale results, privacy,
   maintainability, removed obsolete code and preserved behavior. After a concrete
   failed SOL candidate, record the failure and hand off one focused repair to
   Astra High (`gpt-6-astra`, `high`). An interrupted repair resumes that attempt.
6. When the card passes every required check and diff review, update its checkbox,
   actual test/review record and PLAN.md status, stage only reviewed task changes,
   and create a descriptive local commit naming the POLISH task. Verify the commit
   and report its short hash, then end the wake. A later wake handles the next card.
7. On a failed Astra repair, missing required native capability or material product
   ambiguity, preserve work, record the precise blocker in `docs/errors.log` and
   pause this scheduler. When all six cards pass, record completion and pause it.
   Update scheduling only with the app's automation tool, preserving its other
   fields; do not write raw scheduler files or reactivate historical automations.

## Boundaries

The 2026-09-11 local-commit grant belongs only to the completed historical queue.
The current user explicitly authorizes local commits for completed polish tasks.
Do not push, release, publish, run live
Mini-Orca provider/evaluation campaigns or perform destructive cleanup. The user's
ordinary terminal is a feature being implemented, not authorization for model-driven
commands in project terminals. Preserve source/diff read-only views and explicit
Review/Apply for Mini-Orca candidates. Normal build dependency downloads and tests
using fake providers/temporary fixtures are included in the implementation scope.

The existing scheduler and this task serialize execution; no extra worktrees or
parallel coordinators should be launched. Keep SOL High for normal wakes and Astra
High for the single recorded repair. Report an unavailable requested model rather
than silently substituting another one.

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
