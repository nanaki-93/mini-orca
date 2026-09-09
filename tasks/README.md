# Implementing the plan with agents

[PLAN.md](../PLAN.md) is the only backlog/status ledger. Each task card includes
entry points, dependencies, acceptance and tests. Do not create one file per task.
The [retained Task 170 provenance](../docs/RELEASE_ACCEPTANCE.md#retained-task-170-execution-provenance)
is consolidated historical evidence, not a second execution queue.

## Recommended workflow

Use **GPT-5.6 Terra** for bounded implementation, initially at high reasoning effort
for stateful changes. Its official positioning balances intelligence and cost.
Effort and review policy here are engineering recommendations, not guarantees.
Confirm the exact model is available on the runner; do not silently substitute it.
[Official model reference](https://developers.openai.com/api/docs/models/gpt-5.6-terra)

1. Select one ready task. Give the worker its card, shared rules, relevant code and
   tests, instead of every historical document.
2. Start in a clean isolated worktree based on reviewed integration state. Resolve
   existing user work explicitly before including it in a base; never stash/reset
   it or absorb it into a task automatically.
3. One implementation agent changes code and runs focused checks, then returns a
   reviewable result at **Review**, not self-approved Complete.
4. A fresh reviewer inspects the exact diff, criteria, callers and tests. Prioritize
   scope, stale state, concurrency, consent, source writes and visible interaction.
5. The coordinator independently runs the required gates. Fixes invalidate the
   prior review. Allow the initial Terra attempt plus two repairs; then escalate
   to GPT-5.6 Sol for an initial attempt plus two repairs. Stop with a diagnosis
   only after that sequence is exhausted or a required boundary cannot be met.
6. Integrate only under explicit standing authorization for local task commits and
   integration. Otherwise leave a patch/worktree for review. This document does
   not grant commit permission. Pushes and releases require separate authorization.
7. Update the ledger after acceptance, then select the next ready task. Missing
   native evidence blocks its release gate, not independent backend work.

The commit boundary comes from [AGENTS.md](../AGENTS.md): “Do not create commits
unless explicitly requested.” Finish the code, tests and review before requesting
any still-missing integration authorization. Reuse permission already granted.

Start sequentially: presenter, shell, models and API files are shared hotspots.
Allow at most two implementation lanes only after explicit parallel-work
permission and verified disjoint ownership; integrate serially. The reviewer can
also use Terra. Escalate difficult architecture, concurrency or security findings
if desired; a fresh context matters even when using the same model.

## Copyable single-task instruction

```text
Implement task <ID> from PLAN.md. Read AGENTS.md and the shared definition of done.
Read desktop/UI_DESIGN_GUIDELINES.md before UI changes. Verify dependencies are
accepted in your base and identify existing user work before editing.

Inspect the task's code/tests. Deliver its smallest complete behavior and negative
cases. Preserve one-file preview/review/Apply/Undo, identity guards, provider consent
and explicit execution trust. Remove replaced code. Do not implement adjacent tasks.

Run focused tests and the task gates. Review the diff. Report behavior, files,
actual results, remaining acceptance and migration needs. Leave status at Review
for independent acceptance. Do not commit, push or integrate unless separately
authorized. Do not spawn agents unless requested.
```

Reviewer instruction: “Review task `<ID>` against its criteria and exact diff.
Inspect callers and tests; focus on correctness, stale state, consent, resource
limits and user interaction. Do not edit. Return actionable findings with file/line
evidence, or no findings with explicit verification limits.”

## Making it automatic

Implement AUTO-01, pilot two small tasks, then implement AUTO-02. Use a small
external dispatcher around `codex exec`; do not build an agent platform inside
Mini-Orca. Automation develops the IDE; the IDE still requires explicit Apply.

`codex exec` supports scripted runs, JSON events and schema-constrained final
results. The installed CLI help was checked during planning. From the isolated
worktree, this starts one worker:

```sh
codex exec --model gpt-5.6-terra --sandbox workspace-write \
  "Implement FND-02 from PLAN.md. Follow tasks/README.md. Do not commit or push."
```

It does not supply the coordinator, validation or review gate.
[Official non-interactive documentation](https://learn.chatgpt.com/docs/non-interactive-mode)

AUTO-02 must persist a task lease, base identity, diff digest, worker/reviewer
results, validator results and integration state. Use an atomic lock and resumable
local record; keep generated logs ignored. One coordinator owns plan status.
Stop on missing prerequisites, changed base, unavailable model, failed checks,
the finite Terra/Sol repair sequence being exhausted. Start with one task per
run, a configurable wall-time limit and bounded output. Tokens are recorded for
observability; they do not gate worker, reviewer or integration progress.
Do not place credentials or full project prompts in run logs.

Before unattended integration, establish one explicit policy: allow local task
commits and fast-forward integration after tests/review, or retain patches for
manual acceptance. No-commit workers cannot promise automatic integration of
dependent tasks across fresh worktrees. Never derive this permission from the plan.

## Active autopilot policy

The user's 2026-09-06 request established standing authorization for the Mini-Orca
autopilot to create local task commits on `codex/autopilot` after independent
review and coordinator-run validation. The authorization covers plan tasks only.
It excludes push, merge to another branch, release, deployment, destructive host
cleanup and repository-external changes. Those actions still require the user.

One Codex heartbeat coordinates the queue. On each run it resumes the sole
Running/Review task or selects the first ready Pending task. It uses one GPT-5.6
Terra writer, waits for completion, uses a fresh reviewer, runs the task's gates,
allows two Terra repairs before escalating to Sol with two repairs, updates the
ledger and commits an accepted task. It performs at most one task per run and never overlaps writers. A task that
needs native/provider evidence may remain Blocked while unrelated ready work
continues. The heartbeat reports only actionable blockers and release readiness.

The desktop scheduled heartbeat wakes the coordinator. Local scheduled work
requires the computer on and app running. Scheduling does not replace dependency
checks, locking, budgets or review.
[Official scheduled-task documentation](https://learn.chatgpt.com/docs/automations?surface=app)

AUTO-00 records the installed schedule. AUTO-02 later replaces this bootstrap with
a repository-tested dispatcher; the single-task prompt remains useful for manual
recovery.

## Repository dispatcher

`scripts/autopilot.py` runs at most one eligible ledger task from the clean
`codex/autopilot` branch. It creates an ignored detached worktree, uses ephemeral
schema-constrained Codex runs for a writer and fresh reviewer, then runs the fixed,
base-verified `make check` gate outside the agents. Ignored worker build output is
discarded from the isolated worktree before review and after validation:

```sh
./scripts/autopilot.py --dry-run
./scripts/autopilot.py --task READY-ID
./scripts/autopilot.py --task READY-ID --integrate
```

Without `--integrate`, an accepted result stops at `awaiting_integration`.
`--integrate` explicitly authorizes its local task commit and fast-forward; the
dispatcher never pushes. Safe phases resume from `.mini-orca/autopilot`; an
interrupted paid invocation stops without another paid call. Recover a stale lease
only with `--recover-stale-lease` after confirming its dispatcher and child are gone.
Secure candidate validation currently fails closed unless macOS `sandbox-exec` is
available; the fake CLI suite remains portable and makes no paid calls.

## Automatic repair and model escalation

The user's 2026-09-08 continuation authorizes removing the dispatcher's token
cutoff and automatically escalating stuck coding tasks from GPT-5.6 Terra to
GPT-5.6 Sol. Attempts 1–3 use Terra (initial attempt and two repairs); attempts
4–6 use Sol (initial attempt and two repairs), all at high reasoning effort. A
fresh reviewer uses the current attempt's model. Every changed candidate must
pass that review and the coordinator's fixed validation before integration.

The dispatcher records invocation model, usage, uncertainty and escalation.
The former `--token-budget` option is removed. The 20,000-token reservation is
an accounting estimate for uncertain calls, not a permission or spending limit.
Timeouts remain 30 minutes per invocation, output remains bounded, and one lease
prevents overlapping workers. Known, fully terminated retryable failures can use
the finite repair sequence after candidate boundaries are checked. An invocation
still pending after interruption must be resolved before another call. Missing
credentials/models, live leases, changed bases and unsafe candidate mutations
cannot be solved by blindly starting another worker.

This policy applies to development agents only. It does not increase or remove
the local-model development/qualification request budgets, change the selected
Qwen candidate, or authorize repeated live evaluation calls. The coordinator may
resolve routine implementation/test problems within the existing task scope;
releases, pushes and destructive host changes remain separately authorized.

## Dispatcher failures and explicit recovery

The dispatcher bounds captured stdout/stderr while the child runs and stops its
process group on timeout or output exhaustion. It does not set a process-global
file-size limit: Codex may need to append to existing local databases larger than
the output budget. Worktree validation and sandbox boundaries still apply.

Failed worker/reviewer calls and validation runs retain bounded private diagnostics
outside worker worktrees, in mode-0700 storage with mode-0600 files. Treat this output as untrusted
and potentially sensitive; inspect it locally and do not copy raw output into
Git, ordinary logs, shared receipts or prompts. Ordinary run state records only
diagnostic identity and exit/signal information. A missing successful completion
does not prove that no model request was charged; its reservation stays consumed.

Validation uses private temporary HOME/TMPDIR storage outside Git repositories,
so temporary fixtures do not inherit the integration checkout. Each validation
retains a distinct evidence record across retries. Repair prompts receive only
bounded stage/test identifiers; raw validator output remains in private diagnostics.
Java user.home also points to scratch. Supply the documented
`MINI_ORCA_JDK21_HOME` and `MINI_ORCA_JAVA25_HOME` (or `MINI_ORCA_JBR25_HOME`)
when running validation, because Gradle cannot discover SDKs through the real user
home from this isolated environment. Desktop store tests inject Preferences and
verify store behavior without writing host settings.

Archived recovery of an interrupted or legacy failed run is a separate explicit
coordinator action after diagnosis and accepted repairs. Normal bounded repairs
of known terminated failures follow the automatic policy above. First verify the previous worktree is
unchanged and the dispatcher/child are gone, review and commit any necessary code
repair, and requeue the task as Pending in the clean integration ledger. Recovery
archives the old state unchanged, retains its worktree and token charges, consumes
an attempt from the existing retry allowance, and prepares a fresh run at that
reviewed base. It does not launch a worker or erase reported/uncertain usage. An uncertain
failure remains recorded even when an offline reproduction explains its cause.

For an explicitly authorized failed task, use a unique authorization ID and a
short reason describing the accepted repair:

```sh
./scripts/autopilot.py --task REC-01 --recover-failed-run \
  --recovery-authorization-id rec01-sigxfsz-recovery-1 \
  --recovery-reason 'User-authorized recovery after AUTO-03; retain uncertain invocation charge.'
./scripts/autopilot.py --task REC-01 --dry-run
```

The recovery operation makes no Codex CLI/model invocation, cannot be combined with
`--integrate` or `--dry-run`, and does not modify PLAN.md. A later ordinary dispatch
resumes the selected state. Do not change the committed base between recovery and
that dispatch; resume identity checks still apply. Do not reuse an authorization
ID or delete state files to obtain another attempt.

## Current scheduling scope — 2026-09-09

The user deferred engineering-insight recovery and qualification. Follow the current
scope decision in `PLAN.md`: QUAL-06, RCV-07 and RCV-08 remain Blocked (standby),
REL-01 and REL-02 are Complete; no active delivery task remains. The scheduler
stays Paused. A future wake must not reopen completed or deferred work. The historical recovery instructions below and unchecked
items in `docs/tasks.md` must not trigger work. Preserve durable failed-run state,
receipts, budgets and the sealed holdout; no new provider calls are authorized.
When resumed, reconcile existing task state before dispatch and retain the normal
review/validation gates. This scope decision does not authorize publishing.

## Historical scheduled insight recovery

The 2026-09-08 recovery queue is REC-01 → REC-02 → REC-03 → QUAL-05 → QUAL-06
in `PLAN.md`. The user's request to prepare and schedule this recovery covers the
selected installed local Qwen3.8-27B candidate and six additional development
requests; the existing 24 qualification requests are conditional on pilot success.
The old automation was no longer present in the app when inspected. Its replacement
uses the same 30-minute cadence and failed-run-only notification policy, scoped to
this recovery chain. It pauses on an unresolved blocker, failed pilot, or the final
qualification verdict; it does not automatically continue into release work.

Invoke the repository dispatcher only with an explicit `--task REC-01` or
`--task REC-02` for this schedule; never use its unrestricted next-task selection.
REC-03 also needs coordinator-owned local
runtime/configuration preparation; use the reviewed worker/coordinator workflow
above and mark it Complete only after that preparation succeeds. Do not let the
generic dispatcher's code-only completion bypass host acceptance. QUAL-05/06 are
coordinator-run collections with fresh independent scorers, not provider work
performed by a code writer in an isolated worktree.

Every live attempt uses the canonical checkout's existing ignored evaluation
campaign. Never copy/reset the campaign to obtain another budget. New code is
reviewed and integrated before collection. The plan specifies the exact finite
grant, repeated pilot, model settings, candidate identity, documentation-only
revision transition and qualification gates. No uncounted compatibility probes,
prompt tuning, replacement models or additional scoring-service budget are allowed.
