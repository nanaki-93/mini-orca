# Engineering insights and Performance

These capabilities already exist. The next work is in
[PLAN.md](../../PLAN.md#engineering-learning-and-evidence), not another implementation plan.

Engineering insights are optional, bounded AI interpretation attached to a project,
file, finding or draft. The four fields explain the mechanism, why it matters here,
a trade-off/failure mode and a transferable lesson. Invalid optional insight is
omitted without rejecting valid parent output. The total limit is 1,000 Unicode
runes; draft edits clear the old insight and stale owners remain labeled.

For an experienced engineer, prefer a concrete explanation of cancellation,
contention, allocation, query fan-out, idempotency, backpressure or authorization.
Skip syntax tutorials and generic advice. Opening the collapsed panel starts no
model request. LEARN-01/02 improve presentation and evaluate actual usefulness.

Performance is **source-based review, not measurement**. Reports describe observed
patterns, workload conditions, confidence, trade-offs and verification plans. They
use the `analyze` model scope, separate report/job state and current source/policy/
provider identity. File input/output is capped at 64 KiB and five findings; jobs
have bounded file/time budgets that do not reset on resume. PERF-01 measures
Mini-Orca itself; PERF-02/03 propose an explicit benchmark comparison feature.

The completed implementation passed automated checks historically. Real-provider
content quality, queue confirmation/lifecycle UI and native checks remain in
[release acceptance](../RELEASE_ACCEPTANCE.md). UI-04 closed the attainable native
matrix; LEARN-02 and REL-01 own content quality, provider lifecycle and the listed
release limitations. No content-quality or measured-speed claim follows from schema tests.
