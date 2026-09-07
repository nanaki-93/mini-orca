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

## Opt-in insight evaluation

The deterministic fixture suite checks the schema, 1,000-rune limit, source anchors
and omission behavior. It does not assess whether an explanation is useful. The
representative cases live beside their owner at
`internal/app/testdata/engineering-insight-eval/cases.json`.

For a real-provider sample, a user first selects the provider, model, prompt version,
request cap and output-token cap, then runs no more than that many fixture cases through
the normal explicit-confirmation flow. Do not include source text, endpoint URLs, keys,
or raw provider replies in the receipt. A human scorer records 0–2 for correctness,
local relevance, trade-off clarity and useful verification, plus any critical false
claim. A retained example needs no critical false claim and at least 6/8.

Save a receipt such as this outside the repository:

```json
{
  "provider": "chosen-provider",
  "model": "chosen-model",
  "prompt_version": "file-analysis-v5",
  "sample_count": 1,
  "max_requests": 8,
  "max_output_tokens": 800,
  "samples": [
    {
      "case_name": "allocation",
      "correctness": 2,
      "local_relevance": 2,
      "tradeoff_clarity": 1,
      "useful_verification": 1,
      "critical_false_claim": false,
      "retain_example": true
    }
  ]
}
```

The receipt needs one uniquely named sample per selected case. Validate it separately;
this command makes no provider request:

```sh
go run ./cmd/engineering-insight-eval \
  -receipt /safe/local/insight-evaluation.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -provider chosen-provider \
  -model chosen-model \
  -prompt-version file-analysis-v5 \
  -max-requests 8 \
  -max-output-tokens 800
```
