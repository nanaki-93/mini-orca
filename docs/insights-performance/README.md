# Engineering insights and Performance

These features are implemented. Insight usefulness and appropriate omission remain
**unqualified and on standby**: QUAL-06 and RCV-07 failed; RCV-08 was not run.
[PLAN.md](../../PLAN.md) owns reopening decisions. [Release acceptance](../RELEASE_ACCEPTANCE.md)
owns the tested scope and limitations; successful schema tests do not validate
factual reliability or content quality.

## Engineering insights

An optional insight explains the mechanism, local significance, trade-off or failure
mode, and transferable lesson for a project, file, finding or draft. The selected-file
contract requires all four fields, requests at most 250 Unicode characters per
field, and retains the existing aggregate parser limit of 1,000 normalized Unicode
runes. Invalid optional insight is omitted without rejecting valid parent output.
Draft edits clear the old insight; stale owners remain labeled.

The prompt asks for visible local evidence, conditional impact and concrete
verification of current behavior. Unshown callee behavior remains unknown; trivial
controls should omit generic lessons. These are requested behaviors, not evidence
that the model consistently follows them. Opening the collapsed panel sends no
model request.

## Performance review and measurements

Performance reports describe source patterns, workload conditions, confidence,
trade-offs and verification plans. The Analysis page admits one whole-project run,
which coordinates semantic, Performance and Security stages. Performance retains
its `analyze` model scope and typed reports, while shared durable run progress lives
in `.mini-orca/analysis/run.json`. Performance owns result browsing and file filters;
it has no separate Start/Pause/Resume controls. Current source, policy, provider and
prompt identities still guard evidence. File input/output is capped at 64 KiB and
five findings. Attempts remain cumulative; legacy Performance jobs retain their
total active-time budget across resumes.
Source review alone establishes no runtime measurement or demonstrated speedup.

An optional comparison runs an explicitly selected existing Go benchmark for a
validated draft with execution consent. Its measured evidence is presented alongside
source hypotheses. Preview, review and explicit one-file Apply remain separate.

## Evidence and historical evaluation

Deterministic fixtures cover schema bounds, source anchors, isolated compilation
and optional-output behavior. The public v1 and v2 development corpora remain at
`internal/app/testdata/engineering-insight-eval/`; evaluator implementation lives
in `internal/insighteval`, behind the `cmd/engineering-insight-eval` entry point.
These tests do not measure explanation usefulness.

The historical dispatcher and insight evaluation remain paused. Preserve cumulative
**60 development / 24 qualification requests** (48/24 original and 12/0 successor),
all grants/receipts, and the sealed unused conditional 24-case holdout. The UX implementation queue does not authorize additional model calls.

- [Historical runtime and evaluation instructions](../history/insight-evaluation-2026-09.md)
- [Recovered v2 specification from fe4cdad:docs/tasks.md](../history/insight-evaluation-2026-09.md#recovery-after-the-failed-medium-reasoning-qualification)
- [Failed qualification verdict](../RELEASE_ACCEPTANCE.md#medium-reasoning-qualification-verdict--2026-09-09)
- [Failed v2 development screen](../RELEASE_ACCEPTANCE.md#v2-development-screen-verdict--2026-09-09)
- [Current release limits](../RELEASE_ACCEPTANCE.md#rel-01-final-acceptance--2026-09-09)
