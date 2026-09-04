# Mini-Orca task workflow

The active backlog is [UI precision](../Plan.md): Tasks **161–171**.
Start with the [task index](INDEX.md) and
[sequential execution prompt](PROMPT_EXECUTE_UI_PRECISION.md).

Writing, reading, or reviewing documents does not execute implementation. Explicit
invocation of the new prompt requires one verified local commit per task, never a push.

## Active sequence

The priorities are Jewel adoption with a supported toolchain/runtime, flat panes
with thin dividers, and compact header-owned actions. Task files define bounded
implementation steps, affected files, tests, acceptance, and exact commit subjects.

- Task files stay at stable paths here, including after completion.
- Each has one status: Pending, In Progress, or Complete.
- `INDEX.md` is the authoritative order/status list and must agree with task files.
- Execute 161–171 strictly in order with one writer; no delegation or parallel tasks.
- Post start/completion updates with real verification and commit evidence.
- Complete means acceptance passed and the isolated task commit succeeded.
- Preserve user changes. No broad staging, history rewriting, or unrelated commits.
- Required failures or missing native evidence block their task; never skip ahead.

## Verification and safety

Read `AGENTS.md`, [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md),
[Plan.md](../Plan.md), this file, the index, the execution prompt, and the selected
task. Preserve one project/file/symbol, selectable read-only source/diff, remote
consent, guarded Review/Apply/Undo, and local-only Preview controls.

Use the Gradle wrapper. Every implementation task runs:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Task checks add focused tests, visual inspection, packaging, or native verification.
Tasks 161 and 171 also run `make check` and `make quality`. An unchanged historical
Go quality failure is not a pass or automatic waiver; follow the plan's explicit
baseline/acceptance policy. Never run destructive targets.

## Retained historical acceptance

[Task 149](149_dark_ui_acceptance.md) remains Pending. Its
[dark-UI plan](../docs/dark-ui/PLAN.md),
[acceptance record](../docs/dark-ui/ACCEPTANCE.md), and
[separate prompt](PROMPT_EXECUTE_DARK_UI.md) are retained for unresolved acceptance,
not the active visual direction. Do not execute it during 161–171 or silently
mark it complete.

Completed task/plan/prompt files were removed at the user's request. Git is the full
archive. Design guidelines and baseline/acceptance evidence remain because they
document contracts or limitations, including
[the preceding UI acceptance](../desktop/UI_REFINEMENT_ACCEPTANCE.md).

## Handoff

Report actual task statuses, visible changes, tests/native checks run and not run,
runtime setup, and verified local commit hashes. Do not claim adoption, quality,
or native acceptance from intention alone.
