# Engineering insights and Performance

These capabilities already exist. The next work is in
[PLAN.md](../../PLAN.md#engineering-learning-and-evidence), not another implementation plan.

Engineering insights are optional, bounded AI interpretation attached to a project,
file, finding or draft. The four fields explain the mechanism, why it matters here,
a trade-off/failure mode and a transferable lesson. The file-analysis prompt asks for
an insight only when it can cite visible local evidence, state impact conditionally,
name a meaningful trade-off, and give a concrete verification with an expected
observation. Its parser retains file insights only when all four fields are present.
The prompt omits low-value wrappers and treats unshown callee behavior as unknown,
rather than claiming a downstream safeguard is absent. Invalid optional insight is
omitted without rejecting valid parent output. The total limit is 1,000 Unicode runes;
draft edits clear the old insight and stale owners remain labeled.

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

## Bounded insight evaluation

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

The command validates receipts by default and never constructs a provider client in that
mode, even when old opt-in environment variables are inherited. Explicit `collect`,
`development`, and `qualification` modes use the configured `bug` scope, production
selected-file prompt, parser, and one isolated fixture per case. This is component-level
evidence; it does not exercise import or the Desktop client.

For a provider run, select one candidate, provider label, prompt, corpus and base
revision before collecting any result. First collect the three development cases once;
only after reviewing that result may one focused correction and one second three-case
development run use the remaining development budget. Qualification schedules every
qualification case twice in corpus order: 24 single attempts, with eight substantive
cases (16 attempts) and four omission controls (8 attempts). Each attempt is bounded to
4,096 output tokens and 300 seconds, with no automatic retry. A remote configured
destination needs `-confirm-remote-provider`; loopback is the default.

Private campaign state lives under ignored `.mini-orca/autopilot/engineering-insight-evaluation/`.
It reserves the original 6-development/24-qualification budget before every dispatch.
The existing explicit append-only grant adds exactly six development requests with a
unique authorization ID, producing a 12-request development cap. One final six-request
Qwen3.8 recovery grant can raise that cap to 18 only after those 12 slots are consumed;
the qualification cap remains 24. The user-authorized v11 verification adds one
last bound six-request grant at 18 consumed, reaching a 24-development ceiling.
It fixes candidate `qwen38-v11-schema-1`, model `qwen/qwen3.8-27b`, and prompt
`file-analysis-v11` at a loopback destination. Grants never happen automatically,
cannot reset consumed requests, and a duplicate authorization ID is rejected. Advisory locks reject concurrent
writers and release after crashes; an in-flight reserved attempt remains `unknown` and consumed on resume.
Source-free receipts and ordinary logs never contain source, endpoint URLs, keys, or response prose.

The historical failed v10 Qwen3.8 candidate used a private ignored `bug` scope
configuration with `temperature: 1`,
`reasoning_effort: low`, `top_p: 0.95`, `top_k: 20`, `min_p: 0`,
`presence_penalty: 0`, and `repeat_penalty: 1`. These optional fields are sent as
OpenAI-compatible request fields only when explicitly configured, including zero;
the runner fingerprint binds their unset/value state before a run can resume.
The installed LM Studio runtime maps `reasoning_effort: low` into the selected
candidate's thinking template. Its MLX sampler accepts the selected `top_p`,
`top_k`, `min_p`, and `repeat_penalty` values. MLX has no presence-penalty
processor, so the required neutral `presence_penalty: 0` is the verified fixed
effective setting; nonzero presence penalties are not supported for this candidate.
The coordinator keeps the runtime/template read-back and candidate manifest in
private ignored evaluation storage and verifies them before collection. That v10
freeze is historical and its development allowance is exhausted. The newly
user-authorized v11 pilot uses a separate `qwen38-v11-schema-1` manifest; do not
reuse the v10 manifest or replay its grant/collection commands.

The v11 verification pilot also failed promotion: 5/6 usable and complete,
3/4 useful substantive, both controls omitted, and zero critical false claims.
Its second locking response contained an invalid JSON escape in a suggestion
action; both allocation responses passed the repaired nested insight schema.
All **24 development requests are consumed**, qualification remains at zero,
and the scheduler is Paused. The v11 manifest is historical evidence, not a
promoted qualification candidate. See the
[verified verdict](../RELEASE_ACCEPTANCE.md#v11-verification-result--2026-09-09).

The user subsequently authorized a structured-output recovery and one fresh
six-request pilot. After code acceptance, `authqual05-qwen38-structured-1` can append
six slots at 24 consumed, bound to `qwen38-v12-structured-1`,
`qwen/qwen3.8-27b`, `file-analysis-v12`, and loopback structured dispatch. The new
ceiling is 30 development requests; qualification remains separately capped at
24. Prior grant forms and receipts remain valid historical evidence. No grant
resets accounting or authorizes replay of an existing run.

The receipt records each scheduled case/repetition/attempt, its source-free response
digest, outcome, token consumption, finish reason, elapsed time and score. A score is
bound to that digest. An independent reviewer compares private prose with the case
rubric and scores correctness, local relevance, trade-off clarity and useful verification
from 0–2, including every emitted malformed or failed response. A retained example needs
no critical false claim and at least 6/8. Collection receipts may be unscored; a
qualification receipt with emitted but unscored prose is invalid.

The structured-output recovery uses `file-analysis-v12`. Selected-file analysis
and its evaluation runner send the same strict `response_format: json_schema`
contract; unrelated chat and edit requests keep their existing format. The
provider must support this request format. An explicit request rejection fails
without unconstrained fallback or repeated rejected requests. Schema constraints
do not replace local parsing, target checks, or content scoring. The response
format identity participates in the evaluation fingerprint, and earlier
cached analysis becomes stale. Historical receipts keep the prompt version and
base used for their actual run; updating these examples does not requalify a
candidate or authorize new model requests.

Save a receipt such as this outside the repository:

```json
{
  "mode": "collection",
  "run_id": "qualified-candidate-2026-09-08",
  "candidate_id": "candidate-v1",
  "provider": "chosen-provider",
  "model": "chosen-model",
  "prompt_version": "file-analysis-v11",
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
  -prompt-version file-analysis-v11 \
  -corpus-id engineering-insight-v1 \
  -base-revision selected-base-revision \
  -max-requests 24 \
  -max-output-tokens 4096 \
  -attempt-timeout-seconds 300
```

Collection requires an explicit mode and run identity. The example below describes
the interface. The active v11 pilot must use its actual frozen identity/run IDs
from PLAN.md, not these placeholders. No test or quality target invokes this
command mode:

```sh
go run ./cmd/engineering-insight-eval \
  -mode development -root . -run-id candidate-v1-development \
  -receipt /safe/local/development-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -config config.yaml -candidate-id candidate-v1 -provider configured-bug \
  -prompt-version file-analysis-v11 -corpus-id engineering-insight-v1 \
  -base-revision selected-base-revision
```

Historical grant commands below are accounting evidence only; both were already
applied and fully consumed. Do not replay them. The first six-request extension was
a separate local state mutation with no provider call:

```sh
go run ./cmd/engineering-insight-eval \
  -mode grant-development -root . -authorization-id qual05-extension-1 -requests 6
```

After the first 12 development requests, the approved Qwen3.8 recovery grant was
appended once. Collection verified its bound candidate and loopback model before
reserving its six requests, all now consumed:

```sh
go run ./cmd/engineering-insight-eval \
  -mode grant-development -root . \
  -authorization-id qual05-qwen38-recovery-1 -requests 6 \
  -candidate-id qwen38-v10-recovery-1 -model qwen/qwen3.8-27b
```

That exhausted the prior 18-request development ceiling. The user then explicitly
approved six new v11 verification requests. The newly implemented grant below was
applied once at 18 consumed, raising the development ceiling to 24 without resetting
consumption. This command is also historical and must not be replayed:

```sh
go run ./cmd/engineering-insight-eval -mode grant-development -root . \
  -authorization-id qual05-qwen38-schema-1 -requests 6 \
  -candidate-id qwen38-v11-schema-1 -model qwen/qwen3.8-27b \
  -prompt-version file-analysis-v11
```

The new grant rejects a changed prompt/candidate/model or non-loopback dispatch,
and no further extension is implemented or authorized. The qualification campaign
and its conditional 24-request ceiling are unchanged.

Private replies are held in mode-0700 storage for an independent scorer. `-mode handoff`
lists only attempt IDs, `-mode score -scores scores.json -receipt scored.json` writes a
source-free digest-bound receipt, `-mode export -receipt receipt.json` exports the current
receipt without prose, and `-mode discard` deletes private response material. None prints
response prose.
