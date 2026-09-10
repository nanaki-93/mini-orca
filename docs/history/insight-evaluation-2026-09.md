# Historical insight runtime and evaluation — September 2026

> **Inactive archive.** All runtime, collector, scorer, grant and recovery commands
> below are historical reference only. Do not execute them as a queue or infer
> renewed authorization. [PLAN.md](../../PLAN.md) owns current decisions and
> [release acceptance](../RELEASE_ACCEPTANCE.md) retains the evidence and limits.
> QUAL-06 failed, RCV-07 failed and RCV-08 was not run. Insight qualification and
> the historical dispatcher remain **Paused / standby**. Cumulative consumption
> is **60 development / 24 qualification requests** (48/24 original, 12/0
> successor); the sealed conditional 24-case qualification holdout remains unused.
> The active cleanup scheduler authorizes no live model evaluation.

The first source below preserves the complete former
`docs/insights-performance/README.md` at the CLN-11 baseline (HEAD `0bbbef0`).
The second preserves **fe4cdad:docs/tasks.md**, including accepted RCV-01–06 and
unchecked RCV-07–08. Historical imperatives, ceilings and intermediate statuses
are retained as evidence, not present permissions or passing verdicts. Relative
Markdown links were rebased; source paths and command text remain unchanged.
CLN-08 moved evaluator implementation into `internal/insighteval`; historical
references to its former `internal/app` paths are intentionally retained.

- [Runtime instructions](#audited-standalone-thinking-runtime)
- [Evaluation protocol and commands](#bounded-insight-evaluation)
- [Recovered v2 specification](#recovery-after-the-failed-medium-reasoning-qualification)
- [Complete old-to-new anchor map](improvement-plan-2026-09.md#old-to-new-anchor-map)
- [Current feature overview](../insights-performance/README.md)

# Engineering insights and Performance

> **Current status — 2026-09-09:** insight improvement and qualification are on
> standby. Historical runtime/evaluation commands below are reference material,
> not an active execution queue or new call authorization. Preserve failed results,
> budgets and the sealed holdout. [PLAN.md](../../PLAN.md) owns reopening decisions;
> [release acceptance](../RELEASE_ACCEPTANCE.md) owns current evidence and limits.


These capabilities already exist. The next work is in
[PLAN.md](improvement-plan-2026-09.md#engineering-learning-and-evidence), not another implementation plan.

## Audited standalone thinking runtime

The medium recovery reuses the REC-05 pinned MLX-VLM 0.7.0 environment for the
`qwen38-v12-medium-1` recovery candidate. It reuses the existing local
Qwen weights through `models/qwen38-v12-medium-1`; the link must resolve
to the audited LM Studio artifact. It never changes LM Studio's vendor runtime.

The private candidate directory is ignored by Git. Its `runtime.json` is the
launcher contract and must be mode 0600. The coordinator prepares it only after
the accepted REC-04 audit and the separately pinned Python 3.11 environment are
available. It contains no provider credentials and does not replace the private
application `config.yaml`.

```json
{
  "schema_version": 1,
  "host": "127.0.0.1",
  "port": 1235,
  "python": "venv/bin/python",
  "python_version": "3.11.16",
  "wire_model": "./models/qwen38-v12-medium-1",
  "artifact_realpath": "/Users/.../.lmstudio/models/lmstudio-community/Qwen3.8-27B-MLX-4bit",
  "artifact_realpath_sha256": "<accepted realpath digest>",
  "artifact_identity_file": "artifact-identity.json",
  "artifact_identity_sha256": "<sha256 of that private identity record>",
  "audit_file": "runtime-compatibility.json",
  "audit_sha256": "2b4bef8b6d05aab0d2acce2e852ccdc5a93bc1ec8994f131cb9160da058751c9",
  "requirements_file": "resolved-requirements.txt",
  "requirements_sha256": "560d232b947d900ac3308c249f2b3bdef4c5136c20fdf680e06b0e528ef4debe",
  "state_dir": "runtime",
  "distributions": { "...all pins from resolved-requirements.txt...": "..." },
  "server_source_sha256": { "...all seven audited server files...": "..." },
  "environment": {
    "MAX_KV_SIZE": "119552",
    "MLX_VLM_MAX_NUM_SEQS": "1",
    "MLX_VLM_ENABLE_THINKING": "1",
    "MLX_VLM_MAX_TOKENS": "4096",
    "MLX_VLM_MODEL_DISCOVERY": "served",
    "MLX_TRUST_REMOTE_CODE": "false",
    "HF_HUB_OFFLINE": "1",
    "TRANSFORMERS_OFFLINE": "1",
    "PYTHONDONTWRITEBYTECODE": "1"
  },
  "request": {
    "model": "./models/qwen38-v12-medium-1",
    "temperature": 1,
    "max_tokens": 4096,
    "reasoning_effort": "medium",
    "top_p": 0.95,
    "top_k": 20
  },
  "max_num_seqs": 1,
  "thinking": true,
  "speculative_decoding": false,
  "apc": false,
  "readiness_timeout_seconds": 300
}
```

`distributions` is the complete 54-package name/version map from the reviewed
lock, and `server_source_sha256` is copied exactly from the accepted audit.
`min_p`, `presence_penalty`, and `repeat_penalty` must be absent. The launcher
rejects any other value for the fixed request and runtime settings. The 300-second
readiness limit applies only to model loading; the application keeps its distinct
300-second request deadline and 4,096 total completion-token ceiling.

Before starting, unload the model in LM Studio and verify that `lms ps --json`
reports an unambiguously empty model list. The launcher does not unload LM Studio
for you. From the repository root, the bounded lifecycle is:

```sh
python3 scripts/insight_runtime.py check --candidate-dir .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-medium-1
python3 scripts/insight_runtime.py start --candidate-dir .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-medium-1
python3 scripts/insight_runtime.py stop --candidate-dir .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-medium-1
```

These commands are for the authorized medium candidate. The preceding
`qwen38-v12-thinking-schema-1` identity is historical; use its candidate path
only when checking its retained low-reasoning evidence.

`check` is read-only: it validates the lock, all installed distribution metadata,
the audited server source hashes, the linked artifact's current file hashes, the
private artifact record, and port 1235. It does not import MLX-VLM, load a model,
start a service, install packages, issue HTTP requests, or generate text. `start`
then launches exactly one process at `127.0.0.1:1235`, with a fresh allowlisted
environment, fixed working directory, and no draft/KV/APC/preload inheritance.
It uses a direct no-proxy, no-redirect loopback request only for `/health`.
Before accepting that response, macOS `lsof` must show the recorded child as the
only listener on `127.0.0.1:1235`; unknown ownership or a listener mismatch stops
startup. Readiness requires the one expected loaded model lane, context 119552,
continuous batching, and APC disabled. It sends no inference request. Every
startup failure, including an interrupt, terminates and reaps the original child
and removes state only after that cleanup succeeds.

State, logs, and the lifecycle lock are private mode 0600 under the candidate's
mode-0700 `runtime/` directory. `start` and `stop` take that nonblocking lock, so
two launch attempts cannot pass preflight together. `stop` compares both PID and
`ps` start/command identity before signalling; an inspection failure, stale PID,
or reused PID is never killed. It removes only confirmed stale coordinator state.
`state_dir` is always the literal `runtime` value: the launcher rejects a runtime
directory symlink and symlinks for its lock, log, state, or state temporary file
before changing modes or writing. The model and venv symlinks remain read-only
runtime inputs and are not affected by this boundary.
To roll back, run `stop`, retain the candidate artifacts for audit, and reload the
previous LM Studio lane through its normal UI. REC-06 must still join the private
application configuration to this candidate and prove the actual request path;
this lifecycle check does not claim that the client has sent the configured wire
request.

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

The historical v1 corpus and results remain unchanged. The proposed
`engineering-insight-v2` successor has a separate public development corpus at
`internal/app/testdata/engineering-insight-eval/v2-development.json`: twelve
distinct one-attempt cases, with eight substantive source mechanisms and four
intentional-omission controls. Its reference examples are source anchored, compile
in isolated offline Go modules, retain the same four 0–2 dimensions, and keep each
insight field within the 250-character generation ceiling. They characterize
current behavior and expected observations; they do not prescribe a replacement
implementation.

The 24-case v2 qualification corpus is evaluator-owned while tuning remains
possible. Its eventual public target is
`internal/app/testdata/engineering-insight-eval/v2-qualification.json`; before it
is published, structural tests record only that the target is sealed and absent.
They do not read private evaluator storage or emit holdout source, anchors, or
digests. Publication checks require sixteen substantive cases and eight controls,
but corpus structure, parser acceptance, and compilation are not evidence that a
model explanation is useful or factually accurate. Only the separately authorized
one-pass development and conditional qualification screens can provide that
evidence.

### Frozen v2 successor period

`engineering-insight-v2-recovery-1` is the sole successor period. It leaves
the historical v1 48/24 counters, seven-grant ledger, and receipts unchanged.
The source-free period record binds the predecessor file hashes, canonical
clean checkout revision, current schema identity, v13 candidate and wire model,
both corpus digests, and the fixed 4,096-token / 300-second / 16,384-input /
119,552-runtime-context / medium / temperature-1 / top-p-.95 / top-k-20 /
one-lane profile. It has independent 12 development and 24 qualification
counters with cumulative
limits of 60 and 48.

After RCV-06 completes the offline freeze and the coordinator advances to
RCV-07 with RCV-01 through RCV-06 accepted and the scheduler still Paused, the
coordinator authorizes it once from the canonical checkout containing the
unchanged predecessor evidence:

```sh
go run ./cmd/engineering-insight-eval -mode authorize-recovery-v2 -root "$(pwd)"
```

That command accepts no candidate, budget, corpus, or schedule override and
does not activate a runtime. Before every reservation the runner rechecks the
same root, predecessor hashes, schema, period identity, and fixed profile. The
reservation journal and its counter are one durable write before provider
dispatch; a resumed run preserves a charged missing result as `unknown`.

After RCV-06 freezes the candidate, collection uses these exact identities:

```sh
go run ./cmd/engineering-insight-eval -mode development -root "$(pwd)" -run-id rcv07-qwen38-v13-dev-1 -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv07-qwen38-v13-dev-1-receipt.json -cases internal/app/testdata/engineering-insight-eval/v2-development.json -config .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v13-recovery-1/config.yaml -candidate-id qwen38-v13-recovery-1 -provider configured-bug -prompt-version file-analysis-v13 -corpus-id engineering-insight-v2 -base-revision "$(git rev-parse HEAD)"
go run ./cmd/engineering-insight-eval -mode qualification -root "$(pwd)" -run-id rcv08-qwen38-v13-qual-1 -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv08-qwen38-v13-qual-1-receipt.json -cases .mini-orca/autopilot/coordinator/rcv-v2-sealed/qualification.json -config .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v13-recovery-1/config.yaml -candidate-id qwen38-v13-recovery-1 -provider configured-bug -prompt-version file-analysis-v13 -corpus-id engineering-insight-v2 -base-revision "$(git rev-parse HEAD)"
```

After each collection finishes, the independent scorer writes one digest-bound
score for every response, including all controls. The reviewed score, export,
and offline validation commands are:

```sh
go run ./cmd/engineering-insight-eval -mode score -root "$(pwd)" -run-id rcv07-qwen38-v13-dev-1 -scores .mini-orca/autopilot/engineering-insight-evaluation/rcv07-qwen38-v13-dev-1-scores.json -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv07-qwen38-v13-dev-1-receipt.json
go run ./cmd/engineering-insight-eval -mode export -root "$(pwd)" -run-id rcv07-qwen38-v13-dev-1 -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv07-qwen38-v13-dev-1-receipt.json
go run ./cmd/engineering-insight-eval -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv07-qwen38-v13-dev-1-receipt.json -cases internal/app/testdata/engineering-insight-eval/v2-development.json -candidate-id qwen38-v13-recovery-1 -provider configured-bug -model ./models/qwen38-v13-recovery-1 -prompt-version file-analysis-v13 -corpus-id engineering-insight-v2 -base-revision "$(git rev-parse HEAD)" -max-requests 12 -max-output-tokens 4096 -attempt-timeout-seconds 300

go run ./cmd/engineering-insight-eval -mode score -root "$(pwd)" -run-id rcv08-qwen38-v13-qual-1 -scores .mini-orca/autopilot/engineering-insight-evaluation/rcv08-qwen38-v13-qual-1-scores.json -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv08-qwen38-v13-qual-1-receipt.json
go run ./cmd/engineering-insight-eval -mode export -root "$(pwd)" -run-id rcv08-qwen38-v13-qual-1 -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv08-qwen38-v13-qual-1-receipt.json
go run ./cmd/engineering-insight-eval -receipt .mini-orca/autopilot/engineering-insight-evaluation/rcv08-qwen38-v13-qual-1-receipt.json -cases .mini-orca/autopilot/coordinator/rcv-v2-sealed/qualification.json -candidate-id qwen38-v13-recovery-1 -provider configured-bug -model ./models/qwen38-v13-recovery-1 -prompt-version file-analysis-v13 -corpus-id engineering-insight-v2 -base-revision "$(git rev-parse HEAD)" -max-requests 24 -max-output-tokens 4096 -attempt-timeout-seconds 300
```

Qualification remains sealed until the candidate is frozen and cannot reserve
until twelve development finals are independently scored: all twelve are usable
and complete, all eight substantive insights score at least 6/8, all four
controls omit the insight, and all twelve whole finals have zero critical false
claims. V1 receipt validation is retained unchanged; v2
receipts carry `protocol_version: "v2"` and use twelve or twenty-four unique,
single-attempt schedule entries.

The command validates receipts by default and never constructs a provider client in that
mode, even when old opt-in environment variables are inherited. Explicit `collect`,
`development`, and `qualification` modes use the configured `bug` scope, production
selected-file prompt, parser, and one isolated fixture per case. This is component-level
evidence; it does not exercise import or the Desktop client.

### Historical v1 operation

The commands and accounting below describe the exhausted v1 campaign only. For
a historical provider run, select one candidate, provider label, prompt, corpus and base
revision before collecting any result. For the authorized medium pilot, collect
the three development cases twice using the same frozen candidate and settings.
Do not tune between batches or retry an attempt. Qualification schedules every
qualification case twice in corpus order: 24 single attempts, with eight substantive
cases (16 attempts) and four omission controls (8 attempts). Each attempt is bounded to
4,096 output tokens and 300 seconds, with no automatic retry. A remote configured
destination needs `-confirm-remote-provider`; loopback is the default.

Private campaign state lives under ignored `.mini-orca/autopilot/engineering-insight-evaluation/`.
Requests are durably reserved before dispatch. Historically, the campaign began
with 6 development and 24 qualification slots. The first append-only grant added
exactly six development requests with a
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


The v12 structured pilot also failed: all six requests returned empty final
content, with generated JSON confined to the provider's reasoning channel.
Usable/complete coverage was 0/6; reasoning-only material cannot count as a
successful insight or control omission. All **30 development requests are now
consumed**, qualification usage remains zero, and the scheduler is Paused.
Installed runtime inspection found immediate grammar enforcement while the
thinking channel was open. The proposed next candidate explicitly disables model
thinking and verifies the effective template before a separately authorized pilot;
API `reasoning_effort: none` alone has not established compatibility. See the
[structured pilot verdict](../RELEASE_ACCEPTANCE.md#v12-structured-pilot-verdict--2026-09-09).


The user then authorized exactly six thinking-disabled development requests.
Grant `qual05-qwen38-thinking-off-1`, candidate `qwen38-v12-thinking-off-1`, model
`qwen/qwen3.8-27b`, unchanged `file-analysis-v12`, reasoning effort `none` and
loopback structured dispatch extend the ceiling to **36**, only after 30 consumed.
Historical grants/receipts remain intact; no sixth grant or 37th request is
permitted. Explicit model thinking is off, and corrected read-only checks verify
both the effective template and the isolated full generic thinking flag. The
prior probe used an incomplete key. Sampling, schema and all quality gates stay
unchanged; no qualification starts before a passing reviewed pilot. See
[thinking-disabled recovery](../RELEASE_ACCEPTANCE.md#thinking-disabled-recovery--2026-09-09).


The thinking-disabled pilot delivered **6/6 usable, complete final answers**, but
only **1/4 substantive attempts supplied a useful insight** (7/8). Both controls
intentionally omitted insights and no critical false claims were found. Both locking repetitions
and one allocation repetition omitted insights, so QUAL-05 remains Blocked.
All **36 development requests are consumed / 0 qualification**, and the scheduler
remains Paused. The final-delivery failure did not recur; insight coverage is now
the remaining pilot blocker. See the
[thinking-disabled verdict](../RELEASE_ACCEPTANCE.md#thinking-disabled-pilot-verdict--2026-09-09).

The user authorized one final bounded recovery: grant `qual05-qwen38-medium-1`,
candidate `qwen38-v12-medium-1`, and wire model
`./models/qwen38-v12-medium-1`. It uses the same audited Qwen3.8 4-bit weights,
MLX-VLM 0.7.0 runtime, `file-analysis-v12` schema, sampler, 119,552-token
context, 4,096-token output ceiling, 300-second request deadline, and one-lane
runtime as the installed low-reasoning candidate. The only changed request field
is `reasoning_effort: medium`. The seventh fixed six-request grant applies only
after 42 development requests are consumed, raises the development ceiling to 48,
and leaves qualification capped at 24. The existing six grants, receipts, and
the low-reasoning runtime remain historical evidence; the runtime launcher and
offline conformance accept only those two fixed profiles.

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
the interface. The medium pilot must use its actual frozen identity/run IDs
from PLAN.md and its private manifest, not these placeholders. No test or quality target invokes this
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

Those historical grants reject a changed prompt/candidate/model or non-loopback
dispatch. The qualification campaign and its conditional 24-request ceiling remain
unchanged.

### Historical thinking-schema development bound

After 36 development requests were consumed, authorization
`qual05-qwen38-thinking-schema-1` appended the sixth six-request slot. It bound
candidate `qwen38-v12-thinking-schema-1` to
`./models/qwen38-v12-thinking-schema-1`, prompt `file-analysis-v12`, structured loopback dispatch, and
`reasoning_effort: low`, raising the development ceiling to 42. Its six requests
are consumed. The seventh grant adds only the fixed medium profile described above;
a duplicate, changed identity, remote destination, or request 49 is rejected before
HTTP dispatch.

Private replies are held in mode-0700 storage for an independent scorer. `-mode handoff`
lists only attempt IDs, `-mode score -scores scores.json -receipt scored.json` writes a
source-free digest-bound receipt, `-mode export -receipt receipt.json` exports the current
receipt without prose, and `-mode discard` deletes private response material. None prints
response prose.

---

# Recovery after the failed medium-reasoning qualification

> **Standby — user decision, 2026-09-09.** This recovery specification is retained
> as history, not the active queue. RCV-07 failed and RCV-08 remains unrun; unchecked
> acceptance boxes do not authorize continuation. Follow the current scope decision
> and REL-01 → REL-02 sequence in [PLAN.md](../../PLAN.md). The scheduler stays Paused;
> preserve all budgets, receipts and the sealed holdout. No new model calls.


Analyze and repair explanation coverage, bounded optional output and factual precision, then evaluate one frozen candidate on broader development cases and fresh qualification cases. The user approved execution with “go for it” on 2026-09-09, including 12 development calls and conditional 24 qualification calls. `PLAN.md` remains the sole execution/status ledger; the scheduler stays Paused during coordinated implementation and no release is authorized.

## Evidence and diagnosis

Baseline: `f4e044e91c69cab47faaf7bb6e92e5fa1cb722e5`, `codex/autopilot`. QUAL-05 passed development; QUAL-06 failed. Scheduler is Paused; the campaign consumed **48 development + 24 qualification requests**. Preserve those counters, seven grants, immutable manifests, scored receipts and adjudication records. Raw qualification responses were disposed after review; do not reconstruct them from reasoning or invent their contents. Historical evidence is in `docs/RELEASE_ACCEPTANCE.md` and the private `qual06-qwen38-medium-scoring` audit.

| Finding | Evidence | Interpretation and recovery |
| --- | --- | --- |
| Delivery mostly works | 24/24 usable, 22/24 complete; normal stops, no timeout/runtime failure; 34,090 raw/reported tokens | Another output-budget increase or runtime replacement is not the first repair supported by this run. |
| Missing insights dominate | Of 16 substantive attempts: ten omitted, two rejected, four retained; only two useful | Improve coverage of meaningful existing behavior and constraints, including correct code. Do not force an insight on controls. |
| Pilot coverage was narrow | Two substantive development patterns versus eight qualification mechanisms; v12 guidance contains long held-lock and zero-capacity-append special cases | Generalization failure is observed. Prompt specialization and restrictive omission language are plausible contributors, not a proven sole cause or proof of model incapability. |
| Valid output shape is insufficient | Backpressure finals falsely equate a successful send with receiver delivery despite unknown buffering | Review factual claims across the entire final response. Grammar constraints cannot establish semantic truth. |
| Verification describes proposed behavior | Repeated-I/O 5/8 twice; idempotency 6/8 twice; both receive zero verification credit | Distinguish a characterization test of current behavior from a test for a proposed change. Preserve opaque-callee and ambiguous-failure uncertainty. |
| Parser and schema have different length constraints | `internal/project/engineering_insight.go` accepts at most 1,000 normalized Unicode runes across the four fields; `fileAnalysisResponseSchemaDocument` has no insight `maxLength` | This is a confirmed contract gap. It is **not a proven explanation** of the two authorization rejections: retained diagnostics collapse rejection reasons and the raw outputs are gone. |
| Repetition added little evidence | All twelve pairs are byte-identical | `ChatRequest` supplies no seed; the installed runtime's `generation.py` sampler selects `DEFAULT_SEED` when absent and constructs position keys. This is a plausible mechanism for repetition, not proof of caching or a universal determinism guarantee. Use distinct cases and continue reporting duplicates. |

**Recommendation:** one bounded repair of the explanation contract on the same Qwen3.8-27B 4-bit / MLX-VLM 0.7.0 / medium-reasoning configuration, followed by a broader development screen. Replace the specialized prompt guidance, rather than append another fixture-specific paragraph. Do not introduce a second generation stage, a second model, automatic retries, keyword-based claim filtering or permissive parsing. If the development screen fails, stop and compare model/task-fit options under a separate decision.

## Approved evaluation contract — activate only after review and freeze

- New corpus/protocol identity: `engineering-insight-v2`. Keep the exposed v1 corpus and receipts immutable as historical/regression material. Do not relabel its qualification results or use renamed versions of its examples as fresh holdouts.
- **12 new development calls**, one attempt per distinct case: eight substantive cases spanning the mechanism families and four controls. A passing screen requires 12/12 usable and complete, all eight substantive insights at least 6/8, all four controls omitted and zero whole-final critical false claims. Inspect all twelve before proposing another change; no mid-run tuning.
- Only after that screen passes: **24 fresh qualification calls**, one attempt on each of 24 distinct cases: sixteen substantive and eight controls. Preserve numeric quality gates: at least 23 usable, 22 complete, 13 useful substantive, all eight controls omitted and zero critical false claims. This is an explicitly versioned change from repeated v1 cases, not a retroactive change to the failed verdict.
- Total proposed new spend: **12 + conditional 24 = at most 36 local-model calls**. After full execution cumulative history would be 60 development / 48 qualification, with the original 48/24 separately intact. No retries, probes or extra evaluator-model requests are included. Ordinary agent review remains separately reported as agent work.
- Keep 4,096 total completion tokens including reasoning, 300 seconds per attempt, input 16,384, context 119,552, temperature 1, top-p .95, top-k 20, one lane, thinking enabled and medium effort. Do not change seed behavior in this recovery. Record duplicate outputs; distinct fixtures still do not establish statistically independent production reliability.
- A separate evaluator authors/seals qualification sources and anchors. The tuning worker receives only the development cases and mechanism-level coverage requirements. Freeze candidate/settings before exposing qualification sources to the collection/scoring process. After disclosure, qualification cannot be reused as a fresh holdout for another tuned candidate.
- This proposal does not reopen the existing exhausted campaign. A reviewed, explicitly authorized successor accounting period must be bound to the unchanged predecessor hashes and exact candidate/corpus/settings. No counter reset, copied campaign root, arbitrary budget override or generic unlimited grant API.

## RCV-01 — Preserve useful source-free rejection diagnostics

- [x] RCV-01 accepted

**Target files**
- `internal/project/engineering_insight.go` — distinguish optional-output rejection categories at the existing parsing boundary.
- `internal/project/engineering_insight_test.go` — typed/shape, empty-field and normalized-length boundaries.
- `internal/app/file_analysis.go` — distinguish the selected-file requirement for all four fields without changing parent preservation.
- `internal/app/engineering_insight_runner.go` — persist bounded private per-attempt optional diagnostics next to the existing run evidence.
- `internal/app/engineering_insight_runner_test.go` — diagnostic identity, privacy and resume behavior.
- `internal/app/engineering_insight_test.go` — top-level and nested rejection isolation.

**Inputs / dependencies**
- Execution approval; existing parser, `evaluationOptionalState`, private scoring handoff and strict receipt contracts. No live request budget is needed for implementation/tests.

**Implementation rules**
- Reuse the production validation path; do not duplicate parsing rules in an evaluator-only classifier. Retain current user-facing generic diagnostics and optional-section isolation.
- Record only a stable reason enum, supported location/index, field presence and normalized rune counts: absent/null, invalid JSON/type/keys, empty required field, over-limit or accepted. Distinguish unrelated symbol-explanation/task-spec degradation. Never persist raw field values, source, reasoning, endpoint secrets or provider prose in this diagnostic record.
- Bind diagnostics to run/attempt identity and response digest; write privately and atomically, never overwrite uncertain attempts. Keep public receipt JSON unchanged unless a separately reviewed version transition is necessary.
- Test accepted, intentionally omitted, overlong and malformed nested/top-level cases. Unknown historical causes remain unknown; synthetic reproduction demonstrates a failure class, not the original authorization cause.

**Verification command**
`go test ./internal/project ./internal/app -run 'EngineeringInsight|OptionalInsights|FileAnalysisInsight|FileAnalysisNested' -count=1`

## RCV-02 — Constrain generated insight lengths to the parser budget

- [x] RCV-02 accepted

**Target files**
- `internal/app/file_analysis.go` — add schema bounds at the shared insight definition and align selected-file instructions.
- `internal/app/file_analysis_test.go` — validate schema/parser boundary examples.
- `internal/project/engineering_insight.go` — expose/reuse the existing aggregate limit only if needed; do not widen it.
- `internal/project/engineering_insight_test.go` — keep aggregate/Unicode behavior explicit.

**Inputs / dependencies**
- RCV-01; existing 1,000-normalized-rune aggregate limit and the same schema reused at all supported insight locations.

**Implementation rules**
- Use a conservative wire ceiling of **250 Unicode characters per field**, so all four together cannot exceed the existing 1,000-rune limit. State that this intentionally narrows generation to concise fields; the parser continues accepting historically valid unequal field lengths within its aggregate limit.
- Derive/check the four-field ceiling against one aggregate constant; avoid silently diverging limits. Verify the installed grammar supports these schema bounds offline in RCV-06.
- Keep insights nullable/omittable. Do not truncate or repair rejected generated text, accept missing fields, promote nested text into another location or enlarge the output budget.
- Test exact boundary, over-boundary, multibyte Unicode, whitespace normalization, nested locations and valid parent preservation. A schema pass is not proof of useful or truthful prose.

**Verification command**
`go test ./internal/project ./internal/app -run 'EngineeringInsight|FileAnalysisResponseSchema|FileAnalysisInsight|FileAnalysisNested' -count=1`

## RCV-03 — Replace pilot-specific instructions with a general explanation contract

- [x] RCV-03 accepted

**Target files**
- `internal/app/file_analysis.go` — replace specialized guidance and simplify duplication; defer the single final prompt-version bump to RCV-05.
- `internal/app/engineering_insight_test.go` — replace obsolete phrase-specific expectations with the new contract checks.
- `internal/app/file_analysis_test.go` — preserve production request/schema/cache-identity behavior.

**Inputs / dependencies**
- RCV-02; current shared `EngineeringInsightPromptInstructions`, which also serves other producers and must not be broadened accidentally.
- Sequencing correction from code inspection: historical fixed-grant tests dispatch v12 and the runner rejects any non-current prompt. RCV-03 validates the prompt rewrite at the existing label while all live requests remain disabled; RCV-05 performs the single v13 identity transition with the successor protocol and historical test updates. No intermediate prompt/schema is eligible for collection.

**Implementation rules**
- Explain an observable relationship or invariant, why it matters locally, a real limitation and a test with expected observations. A proposed change or discovered defect is **not required**: correct locking, existing preallocation, authorization gates and safe delegation can carry useful lessons.
- Replace the long pilot-specific lock/append clauses with general instructions. Do not paste held-out fixture identifiers, expected answers or scoring anchors into the production prompt.
- Bound factual claims to visible behavior; distinguish local guarantees from unknown downstream behavior and conditional concerns. Apply this to purpose, risks, suggestions and explanations as well as the insight.
- Make current-behavior verification the default. Label a proposed-change test explicitly and first state the existing behavior it would change; do not present a failing proposed expectation as the current contract.
- Prefer one concise file-level insight where genuinely useful; retain intentional omission and avoid generic tests/advice on trivial controls. No forced non-null insight and no silent second model call.
- Preserve preview-first behavior, exact-symbol validation and cache identity. Do not claim prompt unit tests establish model coverage or factual accuracy; only the later live screen can measure that.

**Verification command**
`go test ./internal/app -run 'EngineeringInsight|SemanticPrompt|FileAnalysis' -count=1`

## RCV-04 — Author a broader development corpus and seal fresh qualification cases

- [x] RCV-04 accepted

**Target files**
- `internal/app/testdata/engineering-insight-eval/v2-development.json` — new twelve-case development corpus.
- `internal/app/testdata/engineering-insight-eval/v2-qualification.json` — evaluator-owned new 24-case holdout, accessed only after candidate freeze.
- `internal/app/engineering_insight_evaluation_test.go` — fixture identity, partition/count and rubric integrity checks.
- `docs/insights-performance/README.md` — describe v2 data separation and evidence limits.

**Inputs / dependencies**
- RCV-03; approved proposed evaluation design. Use a separate evaluator context for holdout authoring; the tuning worker must not inspect qualification source or anchors.

**Implementation rules**
- Cover synchronization boundaries, cooperative cancellation, existing allocation decisions, opaque repeated operations, idempotency, authorization, delegated verification and channel backpressure. Include both correct and defective implementations; short code alone is not a trivial control.
- Use independently authored control flow and contracts, not identifier-renamed v1 cases. Include meaningful boundary/contrast cases such as buffered versus unbuffered behavior and visible versus opaque safeguards without encoding expected answers in request metadata.
- Use 12 unique development cases (8 substantive/4 controls) and 24 unique qualification cases (16/8), each attempted once. Freeze per-case anchors, acceptable uncertainty and critical factual examples before collection. Retain the same four 0–2 dimensions and >=6/8 useful-insight threshold.
- Seal qualification contents/digests in evaluator-owned private storage while tuning is possible; the named repository target is the eventual publication location after the candidate is frozen or evaluation is complete. Never expose sealed content through writer prompts, review diffs or ordinary logs during tuning. Structural checks before unsealing use counts/digests, not fixture prose.
- Keep `internal/app/testdata/engineering-insight-eval/cases.json` unchanged. Preserve the failed result; new data does not repair old scores.

**Verification command**
`go test ./internal/app -run 'EngineeringInsightEvaluation' -count=1`

## RCV-05 — Support one explicitly bounded successor evaluation period

- [x] RCV-05 accepted

**Target files**
- `internal/app/file_analysis.go` — bump the final selected-file prompt identity to v13 alongside version-aware runner/test changes.
- `internal/app/file_analysis_test.go` — advance the current prompt/cache-version assertion with the v13 identity.
- `internal/app/engineering_insight_runner.go` — fixed successor accounting and exact authorized dispatch identity.
- `internal/app/engineering_insight_runner_test.go` — reservation, exhaustion, migration and resume cases.
- `internal/app/engineering_insight_evaluation.go` — explicitly versioned v2 schedule/expectation validation.
- `internal/app/engineering_insight_evaluation_test.go` — unchanged v1 verdicts and v2 unique-case gates.
- `cmd/engineering-insight-eval/main.go` — select the sealed v2 development/qualification schedules and explicit authorization operation.
- `cmd/engineering-insight-eval/main_test.go` — reject arbitrary root/budget/candidate/schedule substitutions.
- `docs/insights-performance/README.md` — exact new commands and durable accounting boundaries.

**Inputs / dependencies**
- RCV-04; **explicit user approval of 12 development + conditional 24 qualification calls** before activating any successor authorization. Implementation/test fixtures do not grant live permission.

**Implementation rules**
- Current code caps development at 48, qualification at 24, development schedule at three cases and collection validation at six calls; changing a run ID alone cannot execute this proposal. Implement the approved v2 boundary before collection.
- Retain the original canonical campaign as immutable 48/24 history. Add exactly one named successor period under the same canonical evaluation root with predecessor/grant/receipt hashes and its own durable 12/24 reservation counters; expose cumulative totals. Do not repurpose or reset old counters.
- Bind the successor to the exact final candidate, prompt/schema, corpus digests, settings and clean base. No arbitrary increasing-cap interface, automatic eighth development grant, unlimited period creation or worker-local campaign copy.
- Retain v1 receipt validation semantics for historical evidence. Explicitly select v2's twelve distinct development cases and twenty-four distinct qualification cases; do not infer protocol from counts or silently accept malformed repeated schedules.
- Require accepted development evidence before reserving qualification. Reserve before dispatch; preserve unknown/charged attempts, resume the same run and forbid replacement runs, retries and identity drift. Test partial/interrupted runs and concurrent reservation attempts.
- Freeze cumulative accounting and all limits in source-free receipts/period evidence with an explicit version wherever the strict schema changes. A new period must not make historical failures disappear.

**Verification command**
`go test ./internal/app ./cmd/engineering-insight-eval -run 'EngineeringInsight|Evaluation|Development|Qualification' -count=1`

## RCV-06 — Review, validate and freeze the replacement candidate offline

- [x] RCV-06 accepted

**Target files**
- `scripts/insight_runtime.py` — bind only the approved replacement identity if the existing fixed profile table requires it.
- `scripts/tests/test_insight_runtime.py` — reject wrong candidate/runtime/limits.
- `scripts/tests/insight_runtime_conformance.py` — prove new length-bounded schema and unchanged reasoning/sampling route offline.
- `PLAN.md` — register the approved recovery IDs/dependencies/status and budget decision through the coordinator.
- `docs/RELEASE_ACCEPTANCE.md` — source-free code/runtime acceptance and final freeze identity.

**Inputs / dependencies**
- RCV-01 through RCV-05, independent code review and explicit execution/budget approval. A fresh reviewer must inspect the exact final diff after repairs.

**Implementation rules**
- Keep the same installed model artifacts, quantization, pinned runtime and medium settings. Use a new candidate identity and manifest; do not mutate the pilot or failed qualification manifests. Reuse verified assets without downloading/upgrading them.
- Prove actual installed grammar length handling, thinking-before-schema behavior, top-k and total-token cap, one-lane ownership, omission/null paths and new prompt/schema identity with mocked/in-process conformance tests. No generation probes.
- Preserve host readback/restore and new-process ownership rules. Freeze code/config/source/artifact/dependency hashes, corpus digests, accepted rubric, exact commands and accounting authorization. Do not treat a stopped runtime's historical manifest as current readiness.
- Record fixed default seed behavior and use distinct cases; do not expand this repair into seed API plumbing. If an unchanged route cannot be proved, stop before paid requests.
- Run the standard coordinator gates and finite Terra initial+2 repairs, then Sol initial+2 repairs policy for implementation if needed. Live application requests never become those repair retries.
- Scheduler remains Paused until an explicitly authorized coordinator starts RCV-07. It must pause again on failed screen, failed qualification or the final verdict; no automatic REL-01 dispatch.

**Verification command**
`make check && make quality && python3 -m unittest discover -s scripts/tests -p 'test_insight_runtime.py' && git diff --check`

Also record the exact installed-runtime conformance command with the newly frozen private Python/candidate paths; successful mock tests alone do not prove host readiness.

## RCV-07 — Run the twelve-case development screen once

- [ ] RCV-07 accepted

**Target files**
- `docs/RELEASE_ACCEPTANCE.md` — development scores, accounting, causal limits and promotion decision.
- `PLAN.md` — coordinator status and stop/advance decision.

**Inputs / dependencies**
- RCV-06; activated explicit 12-call successor authorization; one live coordinator lease; freshly verified owned runtime at zero successor requests. The tuning worker has not accessed the sealed qualification corpus.

**Implementation rules**
- Run the twelve distinct development cases once with the frozen production request/parser and no retries/tuning. Exact CLI comes from the reviewed RCV-05 implementation; do not invent flags or bypass its accounting from a private wrapper.
- Independently score all whole finals and optional insight locations; retain source-free rejection enums/counts, dimensions, digests, token/latency/completion metadata and duplicates. Never salvage reasoning or reclassify omitted/rejected substantive insight as useful.
- Require every development acceptance gate stated above. On any failure, pause and record the mechanism; no automatic alternate model, prompt iteration or further grant. A failed screen consumes at most the authorized twelve calls and leaves the conditional qualification allocation untouched.
- On pass freeze the candidate for qualification, hand sealed holdout material to the evaluator and record any documentation-only base transition. Restore the original host session after collection and discard raw responses only after independent score/evidence review.

**Verification command**
`go test ./internal/app ./cmd/engineering-insight-eval -count=1 && git diff --check`

Run and record the RCV-05 collector, scorer and offline receipt-validator commands exactly; unit tests alone do not complete this task.

## RCV-08 — Run fresh qualification and stop at its verdict

- [ ] RCV-08 accepted

**Target files**
- `docs/RELEASE_ACCEPTANCE.md` — final gate table, limitations, cleanup and accounting evidence.
- `PLAN.md` — qualification result and remaining release blockers.

**Inputs / dependencies**
- RCV-07 passed; same frozen candidate; explicit conditional 24-call qualification authorization; evaluator-owned holdout unsealed only after freeze; verified durable reservation state and coordinator lease.

**Implementation rules**
- Collect all 24 distinct qualification cases once before scoring or considering any changes. No prompt/code/settings change, retry, replacement run or additional model is allowed.
- A fresh scorer and separate reviewer inspect whole terminal finals against pre-frozen anchors and production acceptance metadata. Preserve independent judgments and explicit adjudication; conditional advice alone is not automatically a critical factual claim.
- Apply the unchanged numeric quality thresholds stated above under the explicit v2 schedule. Bind scores to digests, validate receipt integrity and verdict independently, and report total/unique output counts without a production-reliability guarantee.
- Restore the original model session and verify it, stop only the owned runtime, preserve source-free history and dispose raw responses after final review. Pause the scheduler on success or failure. Release work remains a separate authorized step.
- On failure, retain the failed result and exhausted successor accounting. Recommend a new model/task-design decision only; do not automatically launch it or reuse exposed qualification as fresh evidence.

**Verification command**
`go test ./internal/app ./cmd/engineering-insight-eval -count=1 && git diff --check`

Run and retain the exact RCV-05 collection/scoring/offline-validation commands and independent verdict. A failed quality verdict is not a reason to weaken the validator or rerun the sample.
