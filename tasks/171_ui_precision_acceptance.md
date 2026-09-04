# 171 — Complete acceptance, documentation, and commit ledger

## Status

Pending

## Depends on

Task 170, including its verified local commit.

## Goal

Hand off the verified UI, exact runtime setup, and auditable task history without hiding unresolved release gates.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`desktop/UI_PRECISION_ACCEPTANCE.md`, `desktop/UI_COMPONENT_DECISION.md`, `desktop/UI_CONTRAST.md`, `desktop/UI_DESIGN_GUIDELINES.md`, `desktop/README.md`, `Plan.md`, `tasks/README.md`, `tasks/INDEX.md`, this task; production code only for narrow final defects with tests.

## Implementation

1. Verify Tasks 161–170 have passed their criteria and each has exactly one isolated local commit. Reconcile the component inventory, visual matrix, dependency/runtime decision, and actual migration removals.
2. Review the complete sequence diff for old card layouts, dead/duplicate controls, unused dependencies, broken links, inaccurate claims, accidental backend changes, and unrelated user edits.
3. Run full desktop/repository validation. Compare any failures with Task 161's exact baseline; report skipped downstream quality stages. Do not weaken thresholds or fix unrelated Go to claim success.
4. Update launch/build/package instructions with the actual pinned Jewel/JBR pairing and any user setup changes. State explicitly whether configuration and stored pane preferences need migration; do not assume none if the implementation changed it.
5. Record native and automated evidence, deliberate adaptations, limitations, and per-task commit hashes. For this task's own hash use the known commit subject/status in the document and report its verified hash after commit; do not amend just to insert a self-referential hash.
6. Update the plan/index only to the status actually achieved. Keep Task 149 and any unrelated acceptance debt truthful; do not delete incomplete acceptance records or reopen finished sequences.

## Acceptance criteria

- [ ] Jewel adoption, layered divider-owned panes, compact header actions, dense typography, active tabs, and icon breadcrumbs are all evidenced in production UI.
- [ ] Required desktop/repository checks pass; any unchanged out-of-scope quality exception requires explicit user direction recorded before full completion. Missing native evidence is not waived automatically.
- [ ] Documentation matches delivered dependencies, behavior, test coverage, and runtime setup. No source/API/config change beyond the approved desktop scope has slipped in.
- [ ] Every task has one real isolated local commit, no push, and an accurate final status. User work remains intact.

## Verification

Run `./desktop/gradlew -p desktop spotlessCheck detekt test createDistributable`, `make check`, `make quality`, local-document link checks, and `git diff --check`. If a required gate fails, leave this task In Progress, report diagnostics and the needed decision, and do not create its completion commit.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
docs(desktop): finalize UI precision acceptance
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

