# QUAL-05 recovery: thinking with structured final output

Prepare a local runtime that permits reasoning before enforcing the final JSON schema, validate it offline, run a bounded development pilot, and conditionally advance through qualification and release validation. This is an implementation specification for the four requested stages; **PLAN.md remains the only backlog/status ledger**. The user authorized execution with “go for it”; REC-04–07 are now registered in PLAN.md. The scheduler remains Paused during manual runtime preparation.

## Baseline and execution contract

- Reviewed baseline: `32b5b89cbefd2b39882b58dc6cdefc55cb13fd0b` on `codex/autopilot`.
- Current state: QUAL-05 Blocked, QUAL-06 Pending, REL-01 Blocked; scheduler Paused. Campaign consumption is **36 development / 0 qualification**. Preserve all five existing grants and historical receipts.
- Thinking-enabled v12 returned schema JSON only in reasoning, with empty final content. Thinking-disabled v12 delivered 6/6 usable and complete answers, but only 1/4 substantive insights qualified; that insight scored 7/8. Both controls passed and no critical false claims were found.
- The proposed next candidate is `qwen38-v12-thinking-schema-1`. Keep the same Qwen model **artifacts**, `file-analysis-v12`, response schema, corpus and quality thresholds. A different server may require a different wire model identifier; verify its mapping to the same artifact before fixing grant identity.
- The user subsequently authorized execution of all four stages. PLAN.md records reversible runtime setup, exactly six further development requests, conditional qualification and release validation. The ledger grant is still unapplied; no deployment or publication is authorized. The existing 24 qualification requests remain conditional on a passing development pilot.
- Use one isolated Terra writer, fresh independent review and coordinator validation. Allow the initial attempt plus two Terra repairs, then Sol with an initial attempt plus two repairs. Preserve attempt history. Agent token totals are accounting; application-provider requests have a separate hard limit.
- Preserve preview/review/explicit Apply/Undo, strict parsing, target authority, loopback defaults and provider consent. Never reinterpret reasoning as a final answer, repair returned JSON, force insights on controls, or relax thresholds.
- Task IDs REC-04 through REC-07 are registered in PLAN.md. Existing QUAL-05, QUAL-06 and REL-01 cards remain authoritative for their stages.

## Runtime evidence and decision

The installed `mlx_vlm/structured.py` contains `ThinkingAwareLogitsProcessor`; `mlx_vlm/server/generation.py` uses it to defer grammar processing until thinking ends. The current LM Studio `mlx_engine/generate.py` BatchedVision schema path instead attaches the grammar directly. Upstream [MLX-VLM PR #1299](https://github.com/Blaizzy/mlx-vlm/pull/1299) implements the delayed-grammar behavior. Availability of that code is not proof that a configured request reaches it.

Preferred candidate: an isolated, version-pinned **standalone MLX-VLM server**, reusing the local Qwen weights and the existing OpenAI-compatible client interface where compatible. Do not patch LM Studio's bundled files in place. Check standalone runtime dependencies and the actual request path before selecting this route. Do not substitute a newer dependency set or model silently.

The execution audit rejected bundled MLX-VLM 0.6.5: its continuous generation sampler drops `top_k`, and it lacks a hard active-sequence limit. The coordinator explicitly selected published **MLX-VLM 0.7.0** for the next compatibility audit; its wheel SHA-256 is `5ea0c2b8182c055068cb0da59cb2503c238c428a07daaa0bebc246e8c666f4d3`. It requires MLX/MLX-Metal 0.32.2. Use a separate pinned environment, never an in-place bundle upgrade. Selection is not runtime acceptance; REC-04/06 must verify its actual route and dependency identity before generation.

Read-only reference locations on this host:

- Runtime root: `/Users/marcoandreose/.lmstudio/extensions/backends/vendor/_amphibian/app-mlx-generate-mac26-arm64@33`.
- Installed package: `lib/python3.11/site-packages/mlx_vlm/` under that root.
- Existing weights: `/Users/marcoandreose/.lmstudio/models/lmstudio-community/Qwen3.8-27B-MLX-4bit`.
- Prior candidate manifest: `.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-off-1/manifest.json`.

The bundled standalone schema exposes `enable_thinking` and `repetition_penalty`; Mini-Orca currently sends `reasoning_effort` and `repeat_penalty`. Published 0.7.0 additionally forwards `reasoning_effort` to the template; verify this before retaining the previous `low` setting. Do not assume matching names, equivalent defaults, token accounting or low-effort behavior. Resolve these differences before any generation.

## REC-04 — Register recovery and establish runtime compatibility (stage 1)

**Target files**

- `PLAN.md` — register REC-04 through REC-07, dependencies, request authorization boundaries and links to this specification; preserve previous verdicts.
- `docs/RELEASE_ACCEPTANCE.md` — record the runtime decision and source-free compatibility evidence.
- `.mini-orca/autopilot/coordinator/QUAL-05-thinking-schema.json` — new private coordinator record, without changing existing campaign counters.
- `.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-schema-1/runtime-compatibility.json` — private inventory, exact versions/hashes, request-field mapping and unresolved limitations.

**Inputs / dependencies**

- The baseline, current PLAN.md contract, tasks/README.md and applicable execution authorization.
- No dependency on new generation; all checks in this task are read-only or use synthetic inputs.

**Implementation rules**

- Trace the exact OpenAI chat-completion route through request decoding, generation arguments, template rendering, grammar wrapping, final/reasoning separation and usage reporting.
- Establish how to preserve temperature 1, top-p 0.95, top-k 20, min-p 0, neutral presence penalty 0 and repetition penalty 1. Unsupported non-neutral settings must fail explicitly. Record any supported neutral omission rather than claim an ignored field was honored.
- Prove how thinking is enabled. Do not equate a server boolean with the old LM Studio low-effort prompt instruction. Record effective template differences and any unavoidable experimental variable for review before freezing.
- Verify the model wire identifier resolves to the existing artifact hashes, one generation lane, the application input limit 16,384, and a bounded runtime context. Target the previous actual context 119,552 if supported; any changed context is an explicit candidate decision, not an undisclosed default.
- Inventory dependencies without downloading/loading weights or generating output. Record whether an isolated runtime can reuse installed dependencies or needs a pinned environment. Do not alter vendor packages or the active LM Studio session.
- Accept only a complete compatibility map with no silent field loss, known channel semantics and verified total-token accounting. If unavailable, mark the runtime prerequisite unresolved and make no provider calls.

**Verification command**

`git diff --check`

The coordinator also verifies all entries in `runtime-compatibility.json` against installed source and package metadata, without generation. This documentation check alone does not establish runtime readiness.

## REC-05 — Prepare a bounded standalone runtime and lifecycle checks (stage 1)

**Target files**

- `scripts/insight_runtime.py` — new small coordinator launcher/preflight utility for the selected local backend.
- `scripts/tests/test_insight_runtime.py` — lifecycle, configuration and failure tests using fake processes/transports.
- `scripts/insight-runtime-requirements.txt` — exact dependency pins selected from REC-04, if an isolated environment is necessary.
- `docs/insights-performance/README.md` — reproducible setup, effective settings, shutdown and rollback instructions.
- `.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-schema-1/config.yaml` — private candidate configuration; never tracked.

**Inputs / dependencies**

- Accepted REC-04 compatibility decision and authorization for the specific reversible host setup.
- The existing model artifact, an available loopback port and sufficient memory for one loaded model.

**Implementation rules**

- Expose explicit `check`, `start` and `stop` operations; `check` must not generate, load a model or start a service. Fail on missing dependencies or an occupied port rather than install silently or terminate another process.
- Bind only `127.0.0.1`; prefer port 1235 if free, leaving the daemon and existing LM Studio endpoint unchanged. Record actual endpoint and process identity. Stop only the process owned by this coordinator, with bounded waits and reliable cleanup.
- Reuse local weights; disable incidental downloads. Avoid two resident copies of the model. If unloading the old lane is necessary, record a reversible handoff and verified reload instructions before acting.
- Construct the server environment from an explicit allowlist of required OS keys and fixed runtime settings. Do not copy the parent environment: inherited KV quantization, APC disk caching, draft and additional model preload settings must not enter the process.
- Establish server-level thinking, lane and sampling settings using the REC-04 mapping. Inspect the effective template, not just command-line flags. Disable speculative decoding for this candidate.
- Preserve the application's 4,096 total completion-token ceiling, including reasoning, and 300-second attempt deadline. Distinguish model load/readiness timeout from request generation time.
- Keep logs private and source-free where possible. Do not introduce a second campaign, an inference proxy, public port, credential store or background scheduler.
- If a server setting cannot be verified without inference, mark it as a first-pilot-attempt check. Do not add a hidden smoke request.

**Verification command**

`python3 -m unittest discover -s scripts/tests -p 'test_insight_runtime.py'`

Then `python3 scripts/insight_runtime.py check --candidate-dir .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-schema-1` must report zero generation requests. These CLI operations are to be implemented by this task, not commands available today.

## REC-06 — Verify the production request and channel boundary offline (stage 2)

**Target files**

- `scripts/tests/test_insight_runtime.py` — installed/pinned backend conformance and token-transition tests.
- `internal/llm/client.go` and `internal/llm/client_test.go` — only any minimal typed compatibility change demonstrated necessary by REC-04.
- `internal/config/model_profiles.go` and `internal/config/config_test.go` — only validation needed for a demonstrated configuration change.
- `.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-schema-1/verify.py` — new private pre-batch verifier.
- `docs/RELEASE_ACCEPTANCE.md` — independent review and offline evidence.

**Inputs / dependencies**

- Accepted REC-04 and REC-05; exact pinned runtime and candidate configuration.
- Existing shared `FileAnalysisResponseSchema()` and strict production parser in `internal/app/file_analysis.go`.

**Implementation rules**

- Test the actual server route with generation stubbed, plus the actual thinking-aware processor with synthetic token sequences. Do not settle for testing a wrapper that the active route never calls.
- Assert that the grammar is inactive during thinking, activates after the closing token, constrains the final JSON, and resets between requests. Cover thinking disabled, already-closed prefixes, missing end markers, budget-forced closure and sequential request isolation.
- Feed the production schema to the pinned compiler. Test null/omitted/full optional insights, escaped strings, extra fields, partial objects and malformed JSON against both grammar expectations and production parsing.
- Verify final content remains separate from reasoning, usage includes both, truncation is not reported as successful completion, and rejected requests cannot trigger retry or additional reservation. Include backend validation statuses actually used by this server.
- Preserve ordinary chat behavior and reject unsupported request options explicitly. Prefer an existing configuration boundary; do not add an unrestricted extra-parameters map or an unnecessary adapter.
- Freeze hashes for model artifacts, server/runtime files, configuration, effective template, schema and corpus. Verify drift, wrong endpoint/model, extra lane, incomplete readiness and stale process ownership all stop before generation.
- Run fresh independent review and full coordinator gates on the final code diff. No live request is permitted in this task.

**Verification command**

```sh
python3 -m unittest discover -s scripts/tests -p 'test_insight_runtime.py'
go test ./internal/llm ./internal/config ./internal/app ./cmd/engineering-insight-eval
make check
```

Use the established private validation sandbox and headless Java environment on this host, with `MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem` and `MINI_ORCA_JBR25_HOME=/Users/marcoandreose/.sdkman/candidates/java/25.0.4+1.58348-jbr`. Verify those SDK paths before use; do not change the validation gate to hide a failure.

## REC-07 — Prepare exactly six additional development slots (stage 3)

**Target files**

- `internal/app/engineering_insight_runner.go` and `internal/app/engineering_insight_runner_test.go` — one append-only runtime-candidate grant and its boundary tests.
- `cmd/engineering-insight-eval/main.go` and `cmd/engineering-insight-eval/main_test.go` — exact CLI grant route and rejection tests.
- `PLAN.md` — record the actual evaluation authorization and accepted frozen candidate identity.
- `docs/insights-performance/README.md` — document the added bound without rewriting history.

**Inputs / dependencies**

- Accepted REC-06 and an explicit new six-request development authorization. Preparation alone is not a grant.
- Exact candidate, wire model identifier, prompt, schema and effective thinking configuration established by REC-04–06.

**Implementation rules**

- Proposed authorization ID: `qual05-qwen38-thinking-schema-1`; candidate ID: `qwen38-v12-thinking-schema-1`. Append one sixth grant only after exactly 36 development requests are consumed, making the ceiling **42**. Qualification remains 24.
- Preserve all previous grants, historical identity windows, receipts and consumed counters. Bind the new grant to the reviewed runtime candidate, verified model identity, unchanged v12 prompt and loopback structured dispatch; verify runtime/template identity in coordinator preflight.
- Reject duplicate/seventh grants, unbound CLI fallback, corrupt ledgers, wrong identity/configuration, non-loopback dispatch and the 43rd request before HTTP dispatch.
- Test using fake HTTP: exactly six additional requests after valid history, zero calls for a seventh attempt or invalid identity, and no replay of uncertain/finished/terminated runs.
- The writer never applies the grant. After code review and coordinator validation, the coordinator applies it once and records the result in the existing campaign. Do not implement a generic unlimited extension mechanism.

**Verification command**

```sh
go test ./internal/app ./cmd/engineering-insight-eval
make check
```

## QUAL-05 continuation — Collect and score the frozen pilot (stage 3)

**Target files**

- `.mini-orca/autopilot/engineering-insight-evaluation/qwen38-v12-thinking-schema-1/manifest.json` — immutable final candidate identity.
- `.mini-orca/autopilot/coordinator/QUAL-05-thinking-schema.json` — lease, run IDs, exact commands, process state, reviews and verdict.
- `.mini-orca/autopilot/engineering-insight-evaluation/qual05-qwen38-thinking-schema-scoring/audit.json` — private independent source-free scoring evidence.
- `PLAN.md` and `docs/RELEASE_ACCEPTANCE.md` — accepted result, scores, latency, tokens, accounting and failure diagnosis.

**Inputs / dependencies**

- Accepted REC-07, one applied grant, accepted REC-06 readiness and a clean frozen pilot HEAD.
- Existing development fixtures/rubric only; qualification outputs remain unavailable for tuning.

**Implementation rules**

- The coordinator owns the existing lease and sole live generation process. Freeze exact commands and run identities before dispatch: `qual05-qwen38-thinking-schema-dev1` and `qual05-qwen38-thinking-schema-dev2`.
- Run the same three development cases twice, one lane, with unchanged prompt/schema/corpus. Reverify runtime and manifest before each batch; first scheduled attempt supplies any unavoidable live compatibility evidence.
- No probes, automatic retries, tuning or extra calls. Count every dispatched or uncertain attempt. Stop on confirmed incompatibility, drift, permanent rejection or uncertain process state; unused slots do not authorize tuning a replacement candidate within this run.
- Collect both batches before content scoring unless a stop condition occurs. A fresh independent scorer reads actual final prose and fixture source, joins whole-response digests, and checks all final summaries for critical false claims.
- Require **6/6 usable and complete; 4/4 substantive insights scoring at least 6/8; 2/2 intentional control omissions; zero critical false claims**. Missing insights score zero. Do not count reasoning as delivered content.
- Store strict scores, validate both receipts offline, discard evaluator response files and verify absence. Keep only source-free evidence and explicitly eligible sanitized examples. Host log retention is a separate recorded fact.
- On failure mark QUAL-05 Blocked and keep the scheduler Paused, without another grant or candidate switch. A stronger model becomes a separate proposed recovery, not an automatic unbudgeted fallback.
- On success, independent review must accept the evidence before local integration. Record qualification HEAD only after verifying the transition from pilot HEAD changes documentation alone and runtime configuration is unchanged.

**Verification command**

```sh
python3 - <<'PYVERIFY'
import json
import pathlib
import subprocess
root = pathlib.Path('.mini-orca/autopilot/engineering-insight-evaluation')
manifest = json.loads((root / 'qwen38-v12-thinking-schema-1/manifest.json').read_text())
state = json.loads(pathlib.Path('.mini-orca/autopilot/coordinator/QUAL-05-thinking-schema.json').read_text())
for run in ('qual05-qwen38-thinking-schema-dev1', 'qual05-qwen38-thinking-schema-dev2'):
    subprocess.run([
        'go', 'run', './cmd/engineering-insight-eval',
        '-receipt', str(root / (run + '-receipt.json')),
        '-cases', 'internal/app/testdata/engineering-insight-eval/cases.json',
        '-candidate-id', manifest['candidate_id'], '-provider', manifest['provider'],
        '-model', manifest['model'], '-prompt-version', manifest['prompt_version'],
        '-corpus-id', manifest['corpus_id'], '-base-revision', state['pilot_base'],
        '-max-requests', '6', '-max-output-tokens', '4096',
        '-attempt-timeout-seconds', '300',
    ], check=True)
PYVERIFY
```

This command is offline and requires the completed scored receipts and frozen records. Record fully expanded collection/scoring commands before any live execution; no wire model identifier or base is guessed here.

## QUAL-06 continuation — Run conditional qualification (stage 4)

**Target files**

- `PLAN.md` and `docs/RELEASE_ACCEPTANCE.md` — qualification readiness, source-free results and accepted verdict.
- `.mini-orca/autopilot/coordinator/QUAL-06-thinking-schema.json` — new durable qualification coordinator record within the existing campaign.
- `.mini-orca/autopilot/engineering-insight-evaluation/qual06-qwen38-thinking-schema-scoring/audit.json` — private independent scoring evidence.

**Inputs / dependencies**

- Accepted successful QUAL-05 continuation and verified frozen qualification HEAD.
- Existing conditional authorization for 24 qualification requests; no new development authorization is implied.

**Implementation rules**

- Update the existing `mini-orca-autopilot` heartbeat through the automation tool, preserving its 15-minute cadence, target task and notification policy. Resume only after the passing pilot is integrated. Do not create a duplicate automation or edit its TOML directly.
- Its prompt must identify the promoted manifest/state, enforce the sole coordinator lease, reject overlapping/uncertain replay and drift, forbid further development calls, and stop after the qualification verdict. Quiet unchanged state remains quiet.
- Collect exactly 12 qualification cases twice: 16 substantive attempts and 8 controls. Use the frozen model/runtime/schema/template/sampling, 4,096 total completion tokens and 300 seconds per attempt. No tuning, probes, automatic retries or model switches.
- Independently score every emitted response and full final summary; validate schedule and digest joins. Gates are **23/24 usable, 22/24 complete, 13/16 useful substantive at least 6/8, 8/8 intentional control omissions, zero critical false claims**.
- Discard evaluator response text after score/receipt validation. Independently review and integrate the source-free verdict. Mark QUAL-06 Complete only on all gates; otherwise Blocked. Pause the heartbeat after either verdict.

**Verification command**

```sh
go test ./internal/app ./cmd/engineering-insight-eval -count=1
git diff --check
```

Additionally validate the scored qualification receipt with the existing CLI using its exact manifest identities, `-max-requests 24`, `-max-output-tokens 4096` and `-attempt-timeout-seconds 300`. Receipt validation is offline and must not recollect any case.

## REL-01 continuation — Validate the release candidate without publishing (stage 4)

**Target files**

- `PLAN.md` — final release-validation status and unresolved prerequisites.
- `docs/RELEASE_ACCEPTANCE.md` — supported-platform, packaging, runtime setup, source-safety and native acceptance evidence.
- `docs/insights-performance/README.md` — reproducible setup for the accepted runtime profile, including rollback and limitations.

**Inputs / dependencies**

- Accepted passing QUAL-06 and authorization to execute the REL-01 validation scope. The previous schedule intentionally stops after qualification; do not silently extend it to release work.
- Existing retained native/package evidence, supported platform scope and disposable release fixtures.

**Implementation rules**

- Run the project quality/validation gates and rebuild affected desktop distributions with the supported JBR. Reuse still-valid prior evidence; repeat checks affected by runtime/configuration changes.
- Verify the qualified profile reaches the actual production selected-file analysis path, not only the evaluator's `bug` profile. Validate configuration resolution offline and make any necessary promotion explicit; do not silently change unrelated scopes. A behavior/configuration change that invalidates qualification returns to the appropriate gate.
- Validate loopback operation, backend unavailable/cancelled states, denied remote consent, stale files, required checks, explicit review/Apply and Undo on disposable fixtures. Do not spend unapproved live inference requests for packaging or UI smoke checks; reuse accepted provider evidence and deterministic transports. If fresh inference is essential, document its exact budget dependency first.
- Check package startup and required JBR modules; claim only tested platforms. Revalidate Docker only within supported scope and applicable authorization; no destructive cleanup targets.
- Record readiness and remaining limitations. Do not push, publish, deploy or create a public release as part of validation.

**Verification command**

```sh
./scripts/validate.sh
make desktop-build
git diff --check
```

Native bundle startup and review/Apply/Undo smoke checks are additional manual acceptance steps; passing shell commands alone cannot satisfy them.
