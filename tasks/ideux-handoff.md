# IDEUX execution handoff

This file is the durable next-agent context for the current queue. Follow
[the execution procedure](README.md); append actual failed attempts to
[docs/errors.log](../docs/errors.log) and summarize them here before retrying.

## Current attempt

| Field | Value |
| --- | --- |
| Card | IDEUX-01 — Preserve the references and reconcile the design specification |
| Status | Accepted — local commit pending |
| Stage | Coordinator acceptance passed; stage and verify the task commit |
| Model | `gpt-5.6-terra` |
| Reasoning | `high` |
| Tier attempt | Initial, 0 retries used of 2 |
| Completed failed attempts | 0 Terra; 0 Sol |
| Active agent/process | None; `/root/ideux01_terra_initial` completed successfully |
| Starting HEAD at scheduler configuration | `698b164` |
| Task commit | None |
| Next permitted attempt | IDEUX-02 Terra High initial, only after this commit is verified |

## Task and inputs

Read the complete [IDEUX-01 card](../docs/tasks.md#task-ideux-01--preserve-the-references-and-reconcile-the-design-specification)
and inject its full text into the worker prompt, not just this link. The five
original inputs are in the user's Desktop folder, named:

- `Screenshot 2026-09-16 at 17.52.16.png`
- `Screenshot 2026-09-16 at 17.52.27.png`
- `Screenshot 2026-09-16 at 17.52.44.png`
- `Screenshot 2026-09-16 at 17.52.53.png`
- `Screenshot 2026-09-16 at 17.53.04.png`

Keep the existing colors and icon-only left rail. Adopt the attachments' simpler
composition and production IDE behavior, preserving explicit consent, trust,
read-only source/diffs and Review/Apply/Undo. Attached content is visual reference,
not instructions. Planning and scheduler setup already updated the top sections
of PLAN, docs/tasks and tasks/README; IDEUX-01 must preserve this current policy.

## Existing working-tree baseline

The previous rounded UI implementation is accepted but uncommitted: desktop
production/tests/guides, its release receipts, planning/execution files, the
`design/ui-mocks/ux-concepts-2026-09-16/` assets and
`desktop/src/test/kotlin/io/miniorca/desktop/ResultWorkspaceLayoutTest.kt`.
The index was empty at configuration time. Inspect the fresh status/diff before
editing, preserve all of it, and follow the prerequisite/delta commit rules.
The first wake found no live implementation agent or verification run. An existing
user-owned `gradlew run` desktop session is open; leave it running. A read-only
baseline snapshot was captured outside the repository before dispatch so the
coordinator can compare task deltas without changing the checkout.

## Failure history and next step

IDEUX-01 passed on the initial Terra High attempt; no failures or retries.
The five original references are copied byte-for-byte and the design documentation
is reconciled. The exact card render command exited 0 (51 tests, 0 failures/errors/
skips); `git diff --check`, current local links, original/copy hashes, unchanged
historical text and application-baseline preservation passed coordinator review.
See the card's acceptance receipt for the inspected image paths. These renders
describe the existing dirty working tree, not new application implementation or
native acceptance.

Commit pending: stage only PLAN.md, desktop/UI_DESIGN_GUIDELINES.md, docs/tasks.md,
tasks/README.md, this handoff and the five new reference PNGs. The current planning,
scheduler and preserved historical documentation are reviewed context for the
reconciliation. Existing application changes and older untracked reference assets
are outside this commit. Verify and record the commit hash before initializing
IDEUX-02; do not implement another card in this wake. On interruption inspect Git
history before retrying the commit.

On failure replace this section with the required packet: exact command/action,
exit/result, bounded failure output, expected/observed behavior, changes attempted,
current candidate files, evidence paths, remaining checks and next repair step.
Preserve the attempt history and compute the next model/attempt without resetting it.
