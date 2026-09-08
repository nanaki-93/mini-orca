# Implementing the plan with agents

[PLAN.md](../PLAN.md) is the only backlog/status ledger. Each task card includes
entry points, dependencies, acceptance and tests. Do not create one file per task.
The retained [Task 170 note](170_ui_precision_accessibility.md) is working evidence,
not a second execution queue; old execution prompts are superseded.

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
   prior review. Limit repair/review retries to two, then stop with a diagnosis.
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
two failed repairs or exhausted budget. Start with one task per run and configurable
wall-time/token limits. Budget exhaustion is incomplete work, never success.
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
allows at most two repair/review cycles, updates the ledger and commits an accepted
task. It performs at most one task per run and never overlaps writers. A task that
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

## Scheduled insight recovery

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
