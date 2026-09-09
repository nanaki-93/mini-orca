# Recovery after the failed medium-reasoning qualification

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

- [ ] RCV-05 accepted

**Target files**
- `internal/app/file_analysis.go` — bump the final selected-file prompt identity to v13 alongside version-aware runner/test changes.
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

- [ ] RCV-06 accepted

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
