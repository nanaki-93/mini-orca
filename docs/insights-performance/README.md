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

Performance reports are **source-based review, not measurement**. They describe
observed patterns, workload conditions, confidence, trade-offs and verification
plans. Reports use the `analyze` model scope, separate report/job state and current
source/policy/provider identity. File input/output is capped at 64 KiB and five
findings; jobs have bounded file/time budgets that do not reset on resume. PERF-01
measured Mini-Orca itself. PERF-02 adds an optional, explicitly selected Go
benchmark comparison for a validated draft; PERF-03 owns its desktop presentation.

The completed implementation passed automated checks historically. Real-provider
content quality, queue confirmation/lifecycle UI and native checks remain in
[release acceptance](../RELEASE_ACCEPTANCE.md). UI-04 closed the attainable native
matrix; LEARN-02 and REL-01 own content quality, provider lifecycle and the listed
release limitations. No content-quality or measured-speed claim follows from schema tests.

## Opt-in insight evaluation

The deterministic fixture suite checks the schema, 1,000-rune limit, source anchors,
offline isolated Go compilation and omission behavior. It does not assess whether an
explanation is useful. The representative cases live beside their owner at
`internal/app/testdata/engineering-insight-eval/cases.json`.

That corpus has three development cases and twelve qualification cases. The
qualification partition has eight substantive cases and four omission controls;
development and qualification cases are deliberately separate. Every case records
its stable name, intent, source anchor, reference mechanism, qualified uncertainty,
examples of critical false claims, and 0–2 anchors for correctness, local relevance,
trade-off clarity and useful verification. The controls are normal low-value code,
not malformed model responses. Parser-malformation coverage is a separate
deterministic test so an invalid optional section cannot be counted as a successful
omission.

For a real-provider run, select one candidate, provider, model, prompt, corpus and base
revision before collecting any result. Qualification schedules every qualification case
twice in corpus order: 24 single attempts, with eight substantive cases (16 attempts)
and four omission controls (8 attempts). Each attempt is bounded to 4,096 output tokens
and 300 seconds, with no automatic retry. Do not include source text, endpoint URLs,
keys, or raw provider replies in a receipt.

The receipt records each scheduled case/repetition/attempt, its source-free response
digest, outcome, token consumption, finish reason, elapsed time and score. A score is
bound to that digest. An independent reviewer compares private prose with the case
rubric and scores correctness, local relevance, trade-off clarity and useful verification
from 0–2, including every emitted malformed or failed response. A retained example needs
no critical false claim and at least 6/8. Collection receipts may be unscored; a
qualification receipt with emitted but unscored prose is invalid.

Save a receipt such as this outside the repository:

```json
{
  "mode": "collection",
  "run_id": "qualified-candidate-2026-09-08",
  "candidate_id": "candidate-v1",
  "provider": "chosen-provider",
  "model": "chosen-model",
  "prompt_version": "file-analysis-v6",
  "corpus_id": "engineering-insight-v1",
  "corpus_digest": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "base_revision": "selected-base-revision",
  "max_requests": 24,
  "max_output_tokens": 4096,
  "attempt_timeout_seconds": 300,
  "consumption": {"requests": 1, "output_tokens": 500},
  "attempts": [
    {
      "case_name": "qualification-lock-snapshot",
      "partition": "qualification",
      "intent": "substantive",
      "repetition": 1,
      "attempt": 1,
      "candidate_id": "candidate-v1",
      "outcome": "completed",
      "usable_summary": true,
      "complete_summary": true,
      "optional_insight": "present",
      "optional_section_degraded": false,
      "emitted_response": true,
      "response_digest": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "output_tokens": 500,
      "finish_reason": "stop",
      "elapsed_milliseconds": 1234,
      "score": {
      "response_digest": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "correctness": 2,
      "local_relevance": 2,
      "tradeoff_clarity": 1,
      "useful_verification": 1,
      "critical_false_claim": false,
      "retain_example": true
      }
    }
  ]
}
```

This JSON shows one valid collection attempt. The full qualification receipt needs all
24 frozen attempts. The validator reports
`collection`, `incomplete`, `failed`, or `passed`; incomplete exits 2 and failed exits
1. Passing requires at least 23 usable summaries, 22 complete summaries, 13 useful
substantive insights, intentional omission on all eight controls, and no critical false
claim. It also reports median successful latency and nearest-rank p95 separately from
timeout/censored attempts. Validate separately; this command makes no provider request:

```sh
go run ./cmd/engineering-insight-eval \
  -receipt /safe/local/insight-evaluation.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -candidate-id candidate-v1 \
  -provider chosen-provider \
  -model chosen-model \
  -prompt-version file-analysis-v6 \
  -corpus-id engineering-insight-v1 \
  -base-revision selected-base-revision \
  -max-requests 24 \
  -max-output-tokens 4096 \
  -attempt-timeout-seconds 300
```
