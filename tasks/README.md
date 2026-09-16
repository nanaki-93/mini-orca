# Executing the IDEUX UI/UX queue

The user authorized scheduled **IDEUX-01–13** implementation and a local commit
after each accepted task on **2026-09-16**. [PLAN.md](../PLAN.md) owns the current
scope, [docs/tasks.md](../docs/tasks.md) owns the ordered cards, and
[ideux-handoff.md](ideux-handoff.md) owns the next attempt and failure context.
The user subsequently authorized Astra light (`gpt-6-astra`, `low`) to repair
IDEUX-06 after six failures. The heartbeat remains paused during that repair;
prior counters and the subsequent-card Terra/Sol policy remain unchanged.
Only this top procedure applies. Earlier model, retry, scheduling and commit rules
below the history marker are retained receipts, not current instructions.

IDEUX-01 is a reference and specification card. Its baseline fixture renders are
evidence from the existing dirty working tree, not acceptance of a new visual
implementation or a clean-HEAD reproduction. The five copied reference assets are
immutable; later cards may inspect them but do not replace or regenerate them.

## Schedule and worker models

- Reuse heartbeat `mini-orca-ux-implementation` at minutes 09, 29 and 49 each hour
  in the computer's local timezone, attached to
  **Plan IDE UI/UX improvements** (`01a0a9ab-9f6b-7da1-93e4-94e05548984e`).
  Run in this existing checkout, one ordered card attempt at a time. Continue
  immediately to the next card after acceptance and its verified local commit;
  the user requested this completion-driven continuation on 2026-09-16. The
  fixed clock heartbeat starts or recovers idle/interrupted work, not a delay
  between cards. Use fixed clock times because interval heartbeats count from
  recent thread activity and can postpone the next run during conversation.
  Resume an active attempt/check instead of launching a second writer.
- The heartbeat coordinates; implementation is assigned to one fresh subagent
  with an explicit model and reasoning effort. Use `fork_turns="none"` and provide
  the complete task and handoff in its prompt. This user request authorizes those
  sequential agents; it does not authorize parallel implementation or new sidebar tasks.
- First use **Terra High** (`gpt-5.6-terra`, `high`): initial attempt, retry 1/2,
  retry 2/2. On failure of Terra retry 2/2, escalate the same candidate to **Sol
  High** (`gpt-5.6-sol`, `high`): initial attempt, retry 1/2, retry 2/2. A failed
  Sol retry 2/2 pauses the automation and reports the retained failure. Maximum:
  six completed candidate attempts per card. A new card starts at Terra initial.
- An interrupted turn or still-running check resumes the same attempt. A missing
  toolchain, unavailable requested model, exhausted account capacity or external
  approval blocker is not a failed code attempt: preserve state, report the exact
  blocker and pause if useful work cannot continue. Never substitute a model.
- Use the app automation tool for updates/pause, preserving other fields. No raw
  scheduler edits, detached runners, custom cron or additional automations.

Local scheduling needs the computer on and the app running; see the
[official scheduled-task documentation](https://learn.chatgpt.com/docs/automations?surface=app).

## Continuous execution procedure

1. Read root and relevant area instructions, this procedure, the current PLAN,
   the first incomplete IDEUX card and the handoff. Inspect Git status/diffs and
   active agents/processes. Resume existing work. Do not race another writer.
2. Before dispatch, persist the card, model tier, attempt number, stage and worker
   identity when available in the handoff. Capture the starting HEAD and changed
   paths so accepted earlier work can be distinguished from the new task delta.
3. Dispatch one bounded implementation attempt using the model above. Its prompt
   must include the full card text (targets, dependencies, rules, verification),
   the user requirement to preserve colors/icons and keep the UI simple, relevant
   reference paths, and the complete retry packet below. A pointer to a log alone
   is insufficient. Use the same working tree; retain the failed candidate for repair.
   While it works, the coordinator may inspect baseline ownership and prepare
   acceptance review, but must not edit the worker's targets or spawn another writer.
4. The agent implements only that card, runs its focused verification plus required
   area gates and inspects actual production renders. Once a candidate verification
   or substantive acceptance review fails, stop that attempt and return its exact
   failure; do not consume hidden extra repair rounds inside the agent. Ordinary
   planned test-first red cases are not failed candidate verdicts.
5. On failure, append a bounded factual entry to [docs/errors.log](../docs/errors.log),
   update the handoff and leave the card incomplete. Set the next attempt/model
   according to the table above; do not reset counts after a wake, compaction,
   model switch or intermittent partial success. Once the previous worker has
   stopped, immediately dispatch the next permitted fresh agent with the full task
   plus the failure packet. Pause if the attempt limit or a blocker requires it.
6. On success, review the diff and evidence against the complete card. Failed or
   unavailable required evidence cannot be called passing. Record exact commands,
   results and image paths; then create and verify the task's local commit as below.
   Immediately initialize and dispatch the next incomplete card at Terra High,
   repeating this procedure in the same active run without a scheduled delay.
7. Pause the same heartbeat after IDEUX-13 passes and is committed, after Sol's
   final failed retry, or on a true external blocker. Report the current task,
   attempts, last failure or commit and next required action. Keep unaffected
   earlier task acceptance and historical receipts intact.

Keep the coordinator active while workers or checks run, using bounded waits and
persisting progress. An incoming heartbeat resumes that same work. Continue until
the queue is complete, retries are exhausted, a genuine external blocker prevents
progress or the user pauses; do not stop solely because one card finished.

## Failure packet passed to the next agent

Every retry and model escalation receives all of:

- Task ID/title and full current card, including any justified target correction.
- Current tier/attempt and complete prior attempt sequence; next permitted attempt.
- Starting/current HEAD, changed files and relevant candidate diff summary.
- Exact failed command or interaction, exit status, failing tests/assertions,
  relevant bounded error output, and visual evidence paths when applicable.
- Expected versus observed behavior, diagnosis with uncertainty, changes already
  tried and their outcomes, passing checks still valid, remaining checks and the
  next concrete repair step. Do not dump source or secrets into diagnostics.
- Active worker/build/session identity and whether it has exited. Never repeat a
  source mutation or commit just because a previous response was interrupted.

The handoff is mandatory even when the next agent uses the same model. No failure
exists yet at configuration time; do not invent one to fill the template.

## Acceptance and commits

Implementation agents return a candidate and evidence; the coordinator owns final
acceptance, staging and the commit. Review the staged diff and commit with a
descriptive subject containing the card ID, such as `IDEUX-02: simplify IDE chrome`.
Verify the new HEAD and record its hash before starting another card. If interrupted
after committing, inspect Git history first. If only the commit fails, keep the
stage `commit pending` and retry that operation without reimplementing the task.

The checkout begins with accepted but uncommitted MOCK work. Never blindly stage
everything, discard that work or silently attribute it to a new card. Stage the
reviewed task delta separately where possible. If an accepted uncommitted
prerequisite is necessary for a coherent task commit, inspect and validate it,
explicitly identify it in the first relevant commit's body/receipt, and include
only the required prerequisite files/hunks. Leave unrelated work untouched.
Do not alter unrelated staged changes. No amend/rewrite, push, release, publication,
live Mini-Orca provider campaign or destructive cleanup is authorized.

The current user instructions override builder-executor defaults that forbid
commits or permit retries within one agent. If that skill is used, preserve its
target/check discipline while following this explicit scheduling, fresh-agent
retry and per-task commit policy. State/log/PLAN updates needed by this procedure
are authorized administrative targets for every IDEUX card.

---

## Historical execution instructions — inactive

The preceding procedure supersedes every execution/model/commit grant below.
All original text and acceptance receipts are preserved.

# Executing the approved rounded mockup UI

The user authorized implementation and scheduled continuation on **2026-09-16**.
[PLAN.md](../PLAN.md) owns the approved design and current status;
[docs/tasks.md](../docs/tasks.md) owns the six ordered **MOCK-01–06** cards.
All previous queues and execution grants below the history marker are historical.

## Scheduler

Reuse `mini-orca-ux-implementation` as **Mini-Orca rounded mockup implementation**,
attached to **Show UI UX improvements** (`01a0a651-1e1a-79e0-b884-9c57dd054e5b`).
All six cards are accepted. The app and saved configuration confirm **Paused**
after final acceptance. The retained schedule is every 20 minutes, serially in this
checkout, one card per wake with the current task's model and reasoning settings.
This completed scope replaced the finished polish prompt; it does not resume POLISH
or any other queue. The [final evidence](../docs/RELEASE_ACCEPTANCE.md#rounded-mockup-acceptance--2026-09-16)
records production renders, native inspection, all required gates and limitations.

Local execution requires the computer on, app running and checkout available:
[scheduled-task documentation](https://learn.chatgpt.com/docs/automations?surface=app).

## Per-wake procedure

1. Read current root/area instructions, the active PLAN section and first pending
   MOCK card. Inspect the actual rounded reference image before visual changes.
   Respect user steering and pauses. Existing active work wins over a new wake;
   never start overlapping writers, a detached runner, additional tasks or agents.
2. Inspect Git status and relevant staged/unstaged diffs. Preserve prior accepted
   and unrelated changes. Record the active card/stage in docs/tasks.md; resume an
   interrupted build/test or implementation before starting new work.
3. Implement that card end to end with the existing ownership and controls.
   Mockup sample facts/captions are not production data. Necessary adjacent targets
   must be justified on the card before editing; do not broaden scope or upgrade
   dependencies.
4. Run focused checks while iterating, then the card's full required gates and
   inspect production renders against the approved PNG. Fix caused failures;
   retain meaningful assertions and quality thresholds. Use valid cached evidence
   only while the tested inputs are unchanged.
5. Review the final diff, including untracked sources. Record actual commands,
   results, image paths and remaining visual/native limitations. Mark only the
   accepted card complete and update PLAN status. End this wake after one card.
6. If work is interrupted, retain the next step and any process/session identity.
   Continue repairable code/test failures on the same card. If a true external
   blocker prevents useful progress, record its exact action/error, notify the
   user and pause the heartbeat via the app tool.
7. When MOCK-06 passes and all six cards are accepted, pause
   `mini-orca-ux-implementation` using `automation_update`, preserve its remaining
   fields, and report completion with actual production screenshots.

Only meaningful completion, failure or required user action needs a notification;
unchanged/non-actionable state stays quiet.

## Current authorization

Implement source/tests/docs and run ordinary local builds and deterministic
validation in this checkout. Preserve real source/diff read-only views and all
existing provider consent, execution trust, Review/Apply/Undo and terminal ownership.
Do not commit, push, release, publish, run live Mini-Orca provider/evaluation
campaigns, or revive old dispatcher/model-switch/repair-limit policies without
a new user request.

---

## Historical execution procedures — inactive

The earlier guide is preserved below. Its completed scopes, model choices and
local-commit grants do not apply to MOCK-01–06.

# Executing the Mini-Orca UI polish queue

[PLAN.md](../PLAN.md) owns confirmed product decisions and current status.
[docs/tasks.md](../docs/tasks.md) is the sole ordered implementation checklist.
The active 2026-09-15 scope is **POLISH-01–06** at the top of the task file. The
completed 17-card queue from 2026-09-12 is historical evidence below that queue.

Automation: **Mini-Orca UI polish** (`mini-orca-ux-implementation`), **Paused**,
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
repair 1/1, all 462 desktop tests and quality checks. POLISH-04 passed with SOL
High, including first-click recovery and packaged offline Mermaid checks. POLISH-05
passed Astra High repair 1/1, all 465 desktop tests and quality checks. POLISH-06
passed all 467 desktop tests and the nine-stage repository validation. The six-card
queue is complete and the scheduler is Paused.
Scheduled implementation uses SOL High and repairs use Astra High.
The user's follow-up authorizes local commits after each accepted card;
POLISH-01/02 are committed together as `d67ab5a`, POLISH-03 as `fb53e5f`, and
POLISH-04 as `96fcad9`, and POLISH-05 as `296657f`. POLISH-06 is saved in its own
closure commit after acceptance.

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
