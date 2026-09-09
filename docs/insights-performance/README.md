# Engineering insights and Performance

These capabilities already exist. The next work is in
[PLAN.md](../../PLAN.md#engineering-learning-and-evidence), not another implementation plan.

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
