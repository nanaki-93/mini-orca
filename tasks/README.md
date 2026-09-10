# Executing the authorized cleanup

[PLAN.md](../PLAN.md) is the sole execution/status ledger.
[docs/tasks.md](../docs/tasks.md) defines CLN-01–12 target files, dependencies,
implementation rules and exact checks. Historical task cards and dispatcher
commands are retained in [improvement history](../docs/history/improvement-plan-2026-09.md#implementing-the-plan-with-agents).

## Current execution boundaries

The user authorized the **Mini-Orca cleanup agents** scheduler on 2026-09-10.
It wakes the coordinator every 20 minutes, processes at most one ordered cleanup
task per wake, and uses one implementation agent followed by a fresh independent
reviewer and coordinator-run validation. PLAN.md records the active task and writer.

1. Read [AGENTS.md](../AGENTS.md), the selected cleanup card and its dependencies.
   Inspect surrounding code and tests; preserve earlier accepted and unrelated edits.
2. The writer edits only the listed targets and runs the exact task checks. Read
   [UI guidelines](../desktop/UI_DESIGN_GUIDELINES.md) before UI work. Preserve
   one-file preview/review/Apply, identity checks, consent and execution trust.
3. Freeze a reviewable candidate. A fresh reviewer checks that exact diff and
   the coordinator independently verifies it. A changed candidate needs fresh
   review. A writer does not accept its own work or change coordinator status.
4. Allow at most two focused repairs within the selected task; record failures and
   stop on exhausted repairs or an unresolved boundary. Pause the cleanup scheduler
   on an unresolved blocker or final completion.
5. Only the coordinator records acceptance and commits accepted work. The user's
   2026-09-10 authorization groups CLN-01–03 in one local commit and each later
   accepted task in its own commit, including only its ledger/checklist updates.
   Workers and reviewers do not commit. Pushes, releases and live Mini-Orca model
   evaluation are outside this scope.

## Paused historical work

The repository dispatcher and insight recovery/qualification remain **Paused**.
Their old queue instructions and commands are inactive historical reference, even
where the retained prose says “active” or contains unchecked acceptance boxes.
QUAL-06 and RCV-07 failed; RCV-08 was not run. Preserve all receipts, budgets and
sealed holdout material. Only PLAN.md and a new explicit user decision can reopen
that work; the cleanup scheduler does not resume it.

- [Historical agent and dispatcher guide](../docs/history/improvement-plan-2026-09.md#implementing-the-plan-with-agents)
- [Historical runtime and evaluation](../docs/history/insight-evaluation-2026-09.md)
- [Release evidence and limits](../docs/RELEASE_ACCEPTANCE.md)
