# Release acceptance

**Current state: incomplete.** UI-04 establishes the attainable native/accessibility
checks on the supported macOS host. Provider, lifecycle and distribution acceptance
remain open. [PLAN.md](../PLAN.md) is the task/status source; this document owns
release evidence and open validation obligations.

## Evidence and open work

| Evidence | Result / boundary | Plan owner |
| --- | --- | --- |
| Fresh `go test ./...` on 2026-09-06 | Passed, including daemon route/version contracts; some packages cached | Foundation |
| Final Desktop gate on 2026-09-07 | 291 tests passed with zero failures; Spotless and Detekt passed on the documented Java 21/JBR 25 split | UI-04 complete |
| Post-cleanup daemon/config tests (`-count=1`) | Passed, including maintained documentation/version contracts | Foundation |
| Fresh `make quality` on 2026-09-08 | Static analysis, reachability, clone detection, Go complexity, and Desktop static checks passed | REL-01 |
| Final focused UI matrix | Forced 24 visual-layout, 8 accessibility and 7 keyboard-navigation tests passed; 70 ignored PNGs | UI-04 complete |
| Task 170 production render additions | Historical component/test evidence is tracked; generated images remain ignored build output | UI-04 complete |
| FND-01 production fixture baseline | Current 1440×900 and 999×760 Editor, Analysis, populated Review, and populated Performance component captures; JBR 25.0.4.1 rerun, not native acceptance | UI-04 complete |
| Saved fixture inspection during planning | Editor at 1000dp, Analysis at 150% text and ready Review inspected; concrete UI corrections recorded in PLAN | UI-01–03 |
| Native window/edge popup/OS focus/reader | Material supported-host checks passed at 1000×760, 999×760, 800×650 and 1280×600; VoiceOver 10 services active while names/states were inspected; unsupported combinations are recorded separately | UI-04 complete; remaining release combinations REL-01 |
| Desktop `run` runtime | Fixed Java 22-bytecode startup on a Java 21 Gradle launcher; direct and scripted runs launched the app with JBR 25, confirmed by `jcmd` | UI-04 complete |
| Real local/remote/mixed model scopes | Manual compatibility and end-to-end checks not recorded as passed | REL-01 |
| Insight usefulness and Performance lifecycle UI | Manual content-quality/consent follow-ups outstanding | LEARN-02, REL-01 |
| Final packages / supported Docker image | Earlier macOS startup smoke is not final acceptance; Docker release evidence outstanding | REL-01 |

The exact quality diagnostics are in the plan. An old quality deferral is not a
current release waiver. Tasks 149/170/171 are superseded by their plan owners,
not marked passed. The Security scan/review and optional benchmark comparison
APIs are implemented; SEC-08 and PERF-03 still own their desktop presentation.

## Historical QUAL-05 development pilots — 2026-09-08

The local `bug` profile remained `qwen/qwen3-coder-30b`. After the original
six-request budget, the user authorized six more development requests through
persistent grant `qual05-extension-1`; no counters or prior results were reset.
All 12 attempts completed under 4,096 output tokens and 300 seconds per attempt,
without retries. These historical development runs consumed 4,978 output tokens;
qualification consumption remained zero. The then-current development ceiling was
exhausted. The subsequent Qwen3.8 recovery is recorded below.

| Candidate / source-free receipt | Independent digest-bound score audit | Collection metadata |
| --- | --- | --- |
| Historical `v7-dev1` — ignored `.mini-orca/autopilot/engineering-insight-evaluation/qual05-v7-dev1-receipt.json` | Lock/cancellation 5/8; slice-capacity 7/8; trivial control intentionally omitted; zero critical false claims. | Three completed attempts; 1,320 output tokens. This historical collection was not rerun. |
| Corrected `v8-dev2` at accepted code `4293799289201c6797e735c60c1e3aa567055206` — ignored `.mini-orca/autopilot/engineering-insight-evaluation/qual05-v8-dev2-receipt.json` | Lock/cancellation 6/8; slice-capacity omitted its insight (0); trivial control intentionally omitted; zero critical false claims. | Three completed attempts; 1,228 output tokens; 4,984ms median and 9,311ms nearest-rank p95 successful latency. |
| Recovery `v9-dev3` at `e778302` — ignored `qual05-v9-dev3-receipt.json` | Lock/cancellation 6/8; slice-capacity 4/8; trivial insight omitted; zero critical false claims. | Three usable/complete attempts; 1,355 output tokens; median 7,058ms, nearest-rank p95 9,420ms. |
| Final `v10-dev4` at `c0e8dc2` — ignored `qual05-v10-dev4-receipt.json` | Lock/cancellation 6/8; slice-capacity insight absent (0); trivial insight omitted, but one critical false claim elsewhere in that summary. | Three usable/complete attempts; 1,075 output tokens; median 4,110ms, nearest-rank p95 9,700ms. |

A fresh independent agent scores each pilot against actual private replies and
joins scores to response digests. The v9 recovery produced a low-value allocation
insight; v10 omitted that substantive insight again and falsely attributed a public
API endpoint to the constant-returning helper. Neither recovery candidate
qualified for promotion, so QUAL-05 was Blocked and no candidate was frozen.
Collection validity is not evidence of useful-insight coverage or a release pass.
Historical scores and receipts are preserved; private replies are discarded after
scoring. Further development required a new explicit candidate/budget decision;
the scheduler was stopped until the recovery authorization below.

Validation: independent code reviews, `make check`, `make quality`, focused Go
tests, formatting and vet passed. A subsequent race run hit the existing
`TestPerformanceJobRecoveryResumesInterruptedFile/paused` TempDir cleanup failure;
its full `make test-race` rerun passed. No qualification corpus or rubric changed.

The following collection commands are historical evidence only and must not be
replayed: all development slots are now consumed.

```sh
go run ./cmd/engineering-insight-eval -mode collect -root . -run-id qual05-v7-dev1 \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v7-dev1-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json -config config.yaml \
  -candidate-id v7-dev1 -provider configured-bug -prompt-version file-analysis-v7 \
  -corpus-id engineering-insight-v1 -base-revision 4f682b04f01474cf36404b173e0591097364eabf

go run ./cmd/engineering-insight-eval -mode collect -root . -run-id qual05-v8-dev2 \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v8-dev2-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json -config config.yaml \
  -candidate-id v8-dev2 -provider configured-bug -prompt-version file-analysis-v8 \
  -corpus-id engineering-insight-v1 -base-revision 4293799289201c6797e735c60c1e3aa567055206
```

Recovery collection used the same flags above with these exact identities:

| Run ID / candidate ID | Prompt | Base revision |
| --- | --- | --- |
| `qual05-v9-dev3` / `v9-dev3` | `file-analysis-v9` | `e778302dbe08ffabcff715121562fb7261496b6c` |
| `qual05-v10-dev4` / `v10-dev4` | `file-analysis-v10` | `c0e8dc25e075549228bea8aa16020a9ebd53be81` |

Each receipt is `<run-id>-receipt.json` in the same ignored evaluation directory.
The unchanged corpus digest is
`e4e7c71e1721986417e59dfcc581e7f31c7eeac1829fd20f441bc3dfec35a6c7`.
The one-time grant command was `go run ./cmd/engineering-insight-eval -mode
grant-development -root . -authorization-id qual05-extension-1 -requests 6`.

The historical v10 source-free receipt can be validated without a provider call:

```sh
go run ./cmd/engineering-insight-eval \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v10-dev4-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -candidate-id v10-dev4 -provider configured-bug -model qwen/qwen3-coder-30b \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision c0e8dc25e075549228bea8aa16020a9ebd53be81 \
  -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
```

## Qwen3.8 frozen recovery pilot — 2026-09-08 UTC

**QUAL-05 failed; the recovery scheduler is Paused.** Grant
`qual05-qwen38-recovery-1` added exactly six development slots to the existing
campaign without resetting its 12 consumed requests. Both three-case batches
were collected before independent content scoring, at identical frozen settings
and base `2e823825aa3c1ed9c272e179db247b781a15745c`. The run IDs below distinguish
repetitions; the CLI records repetition 1 within each batch. No qualification
request, extra probe, retry, prompt change or model switch occurred.

The candidate was `qwen38-v10-recovery-1`, provider label `configured-bug`, local
model `qwen/qwen3.8-27b`, artifact `lmstudio-community/Qwen3.8-27B-MLX-4bit`,
LM Studio 0.4.23+1 / MLX 1.11.0. The frozen profile used low thinking,
temperature 1, top-p 0.95, top-k 20, min-p 0, presence penalty 0 and repeat penalty
1, with 4,096 total completion tokens including reasoning and 300 seconds per
attempt. Application input budget was 16,384 tokens; **measured effective runtime
context was 119,552**, parallelism 1, with no speculative draft. REC-03 documented
why MLX expanded the initial context target; 16K was not the runtime cap.

The unchanged prompt was `file-analysis-v10`, corpus `engineering-insight-v1`,
SHA-256 `e4e7c71e1721986417e59dfcc581e7f31c7eeac1829fd20f441bc3dfec35a6c7`.
Both pre-batch checks of the private candidate manifest passed; manifest SHA-256
was `9cfc0ae5d60af3798a2c9f1024c356864e8f32649c62dd46591b6c24fb45f3c5`.
The manifest, readback evidence and verifier remain under the ignored evaluation
root's `qwen38-v10-recovery-1/` directory. No production behavior changed during
collection or this evidence update, and no qualification candidate was promoted.

A fresh independent GPT-5.6 Terra agent scored every whole emitted response
against the development source/rubric, including the final parent summary for
false claims. This is agent scoring, not human testing. Scores below list
correctness / local relevance / trade-off clarity / useful verification, each 0–2.
Controls' 8/8 scores represent successful omission, not retained insight examples.

| Run / case | Dimension scores | Complete / insight status | Completion tokens / latency |
| --- | --- | --- | --- |
| `qual05-qwen38-dev1` / lock-cancellation | 2 / 2 / 1 / 2 = **7/8** | Complete; eligible insight | 1,653 / 123,839ms |
| `qual05-qwen38-dev1` / slice-capacity | 2 / 2 / 2 / 2 = **8/8 diagnostic** | Degraded; nested insight rejected; ineligible | 1,246 / 93,972ms |
| `qual05-qwen38-dev1` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 347 / 27,581ms |
| `qual05-qwen38-dev2` / lock-cancellation | 2 / 2 / 1 / 2 = **7/8** | Complete; eligible insight | 1,340 / 97,938ms |
| `qual05-qwen38-dev2` / slice-capacity | 2 / 1 / 2 / 2 = **7/8 diagnostic** | Degraded; nested insight rejected; ineligible | 1,414 / 101,905ms |
| `qual05-qwen38-dev2` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 421 / 30,699ms |

Both locking insights lacked the rubric's full uncertainty caveat about whether
serialization is needed. The second allocation insight lacked the full local
reference set. Neither omission was a critical false claim. The scorer found
**zero critical false claims across all six final summaries** and confirmed both
controls intentionally omitted insight material.

Both allocation responses contained a valid top-level insight and a **string**
in `suggestions[0].engineering_insight`. The same field requires a four-part object
at every location. The parser preserves the valid parent and top-level insight,
but rejects that nested field. `evaluationOptionalState` merges a nested rejection
into the attempt's `optional_insight: rejected` and degraded status. Thus these
allocation insights are diagnostically useful, but neither attempt counts under
`hasUsefulSubstantiveInsight`. They are not retained examples. The failure is
schema adherence, not absent allocation reasoning, truncation or a token cutoff.
The next separately authorized recovery should target this nested schema boundary
and repeat a fresh finite pilot; these failed attempts remain failed evidence.

Official pilot totals are **6/6 usable, 4/6 complete, 2/4 useful substantive,
2/2 intentional control omissions, zero critical claims**. Requirements were all
six complete and all four substantive attempts eligible at >=6/8, so promotion
fails. All attempts ended with `stop`; none timed out or had invalid provider
metadata. Batch 1 consumed 3,246 completion tokens, batch 2 consumed 3,175, for
**6,421** including reasoning. Combined successful latency median was **95,955ms**
and nearest-rank p95 **123,839ms**, with zero timeout/censored attempts. Per-batch
median/p95 were 93,972/123,839ms and 97,938/101,905ms. These are six-case development
observations, not qualification reliability or a controlled model speed comparison.

The persistent campaign now records **18 development / 0 qualification requests**;
all 24 conditional qualification slots remain unused. The dispatcher's removed
token cutoff and Terra-to-Sol coding repair policy do not enlarge this separate
application-provider budget. QUAL-06 remains Pending behind Blocked QUAL-05;
REL-01 remains Blocked. No further recovery grant is implied.

Both scored, source-free `<run-id>-receipt.json` files remain in
`.mini-orca/autopilot/engineering-insight-evaluation/`, alongside durable manifests
and `qual05-qwen38-scoring/audit.json`. Every score joined to its whole-response
SHA-256. Private responses were discarded with the CLI after scoring and offline
validation, and absence was verified; no response prose was copied into Git.
The second batch's coordinator wrapper raised an attribute error while recording
the already-finished child result. Its exit-code value was lost, not assumed zero.
The durable manifest was finished with three completed attempts and its exported
receipt; process inspection confirmed no collector remained. No call was replayed.

The following are **historical collection commands; do not replay**. The grant
was applied once, and the development budget is now exhausted. Both batches used
the same collection command with `RUN_ID` set once to `qual05-qwen38-dev1` and once
to `qual05-qwen38-dev2`:

```sh
go run ./cmd/engineering-insight-eval -mode grant-development -root . \
  -authorization-id qual05-qwen38-recovery-1 -requests 6 \
  -candidate-id qwen38-v10-recovery-1 -model qwen/qwen3.8-27b

python3 .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v10-recovery-1/verify.py

go run ./cmd/engineering-insight-eval -mode development -root . -run-id "$RUN_ID" \
  -receipt ".mini-orca/autopilot/engineering-insight-evaluation/$RUN_ID-receipt.json" \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -config .mini-orca/autopilot/engineering-insight-evaluation/qwen38-v10-recovery-1/config.yaml \
  -candidate-id qwen38-v10-recovery-1 -provider configured-bug \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision 2e823825aa3c1ed9c272e179db247b781a15745c
```

Both receipts passed the following **offline** validation, with `RUN_ID` set to
each run above. Each reported 3/3 usable, 2/3 complete, 1 useful substantive,
1 omitted control and zero critical claims. A collection-valid verdict is not a
pilot pass. The receipt's six-request limit is the collector contract; each batch
consumed three and the separate persistent campaign enforces the overall ceiling.

```sh
go run ./cmd/engineering-insight-eval \
  -receipt ".mini-orca/autopilot/engineering-insight-evaluation/$RUN_ID-receipt.json" \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -candidate-id qwen38-v10-recovery-1 -provider configured-bug -model qwen/qwen3.8-27b \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision 2e823825aa3c1ed9c272e179db247b781a15745c \
  -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
```

Coordinator validation: `go test ./internal/app ./cmd/engineering-insight-eval
-count=1` and `git diff --check` passed. No full Go/race/Desktop/quality rerun was
required for this documentation-only verdict; REC-03's reviewed production code
already passed its full gates. Validation and independent scoring made no
application-provider generation requests.

## QUAL-05 nested insight prompt repair — 2026-09-09

The user requested the bounded schema repair after the failed frozen pilot.
Code `76fdf95b57bead9ed423790c01c03f6a7f77a542` introduces `file-analysis-v11`:
every supported `engineering_insight` location now has the same explicit contract
of exactly four non-empty string fields, no extra keys, or omission/null. The
prompt includes a concise object shape, explicitly excludes strings/arrays/partial
objects, and discourages duplicate per-risk/per-suggestion lessons. This clarifies
the model instruction; it does not add provider-side constrained decoding or prove
that the local model will always comply. The production parser remains strict,
and invalid nested fields still degrade their attempts.

Synthetic regressions cover the observed wrong nested string type in both risks
and suggestions, preserving the parent and top-level insight while recording
rejection/degradation. Correct nested objects, null and omission retain complete
status. The assembled prompt contract is checked, and the cache regression verifies
v10 entries become stale under v11. No fixture rubric, historical score, budget,
provider configuration or live candidate manifest was changed. Refreshing stale
analysis remains an explicit user operation; no migration or automatic generation
is required.

A fresh independent Terra reviewer accepted exact code diff SHA-256
`0d4af9723481b016899992d103ac58a1f55af8340fca6c944e897a745b008dea`.
Focused app tests and formatting passed. The first coordinator `make check` passed
its Go/dispatcher stages but stalled in AWT `nativeGetScreenInsets` while an
offscreen Compose fixture initialized Jewel font metrics. Repeated thread dumps
confirmed the stall; the coordinator stopped only that validation's test executor.
The resulting failed gate and private diagnostics are preserved, not reported as
passing.

Independent review confirmed the suite uses offscreen raster/Compose fixtures and
has no native-window prerequisite or headless skip. The coordinator reran the
**entire `make check`** with the same base-verified Makefile, private HOME/TMPDIR,
filesystem/network sandbox, Java 21 launcher and JBR 25 executor, adding only
`-Djava.awt.headless=true` to `JAVA_TOOL_OPTIONS` with a fresh Gradle daemon. It
passed, including **327 Desktop tests in 38 suites, zero failures/errors/skips**,
Go tests, race checks, vet, formatting and simulated dispatcher tests. The reviewed
code digest was unchanged before and after both validations. This establishes
headless offscreen component coverage; native-window/release acceptance was not
rerun, nor was the separate `make quality` gate. Evidence is retained in ignored
`.mini-orca/autopilot/coordinator/QUAL-05-schema-repair.json` and referenced private
diagnostics.

The code repair is accepted locally, but **QUAL-05 remains Blocked on a fresh
live pilot**. Consumption is still 18 development / 0 qualification, and the
scheduler is Paused. No live effectiveness or promotion is claimed. The old v10
manifest remains historical: a new six-request allowance and new v11 candidate
manifest/run IDs are needed to evaluate all three development cases twice under
unchanged scoring thresholds. All 24 qualification slots remain conditional and
unused. Historical v10 receipts retain their original prompt/base identity.

## Authorized v11 verification pilot — 2026-09-09

The user approved six fresh local requests after the v11 schema repair. The exact
append-only grant `qual05-qwen38-schema-1` was applied once at 18 development /
0 qualification consumption, bound to `qwen38-v11-schema-1`,
`qwen/qwen3.8-27b`, and fixed `file-analysis-v11`. The current development ceiling
is 24; qualification remains separately capped at 24. No prior result or counter
was reset. Grant code `3cf5d9e913f809570fceffa6187a6bebdc5661de` passed independent
review, focused tests, Go quality and full coordinator headless `make check`,
including all 327 Desktop tests with no failures/errors/skips.

The new private manifest SHA-256 is
`e691cfc0504127e7afdedff5b9e76ca6ed6c658017d5a80bea8069d97bb6f142`.
Configuration bytes, model/runtime/template identity and sampling settings match
the failed v10 pilot; the prompt/code identity changes to the reviewed v11 repair
and grant implementation. The prior v10 manifest remains unchanged. Runtime
readback showed the same local model, low thinking and context 119,552 without
generation; full pre-batch checks are required before each dispatch.

The protocol is three development cases twice at one clean base, using
`qual05-qwen38-v11-dev1` and `qual05-qwen38-v11-dev2`, under 4,096 completion tokens
including reasoning and 300 seconds per attempt. Collect both before independent
scoring; no probes, retries or tuning between batches. Require all six summaries
usable/complete, all four substantive attempts eligible at >=6/8, intentional
omission on both controls, and zero critical false claims. Runtime drift or an
uncertain request stops dispatch. Receipt validity alone cannot promote a candidate.
The schedule remains Paused while this manual pilot runs; qualification is a
subsequent task only if the pilot passes and its evidence is accepted.

The applied grant command is historical evidence and must not be replayed:

```sh
go run ./cmd/engineering-insight-eval -mode grant-development -root . \
  -authorization-id qual05-qwen38-schema-1 -requests 6 \
  -candidate-id qwen38-v11-schema-1 -model qwen/qwen3.8-27b \
  -prompt-version file-analysis-v11
```

### v11 verification result — 2026-09-09

**QUAL-05 remains Blocked; the scheduler remains Paused.** Both batches completed
at clean pilot base `7bcea884d64fbadfc4fc58cce5a2d8afbe0225ab`, with the manifest
above verified before each batch. Candidate `qwen38-v11-schema-1`, provider
`configured-bug`, prompt `file-analysis-v11`, and corpus `engineering-insight-v1`
remained fixed. Corpus SHA-256 was
`e4e7c71e1721986417e59dfcc581e7f31c7eeac1829fd20f441bc3dfec35a6c7`.
Both batches were collected before content scoring. No retry, probe, prompt or
configuration change, model switch, or qualification call occurred.

A fresh independent GPT-5.6 Terra agent scored all six whole emitted responses
against the development fixtures/rubrics and checked final summaries for false
claims. This was agent review, not human testing. Scores list correctness / local
relevance / trade-off clarity / useful verification, each 0–2. Control scores
represent successful omission, not retained insights.

| Run / case | Dimension scores | Complete / insight status | Completion tokens / latency |
| --- | --- | --- | --- |
| `qual05-qwen38-v11-dev1` / lock-cancellation | 2 / 2 / 2 / 2 = **8/8** | Complete; eligible insight | 1,479 / 110,977ms |
| `qual05-qwen38-v11-dev1` / slice-capacity | 2 / 1 / 2 / 1 = **6/8** | Complete; eligible insight | 2,850 / 207,803ms |
| `qual05-qwen38-v11-dev1` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 442 / 35,938ms |
| `qual05-qwen38-v11-dev2` / lock-cancellation | 0 / 0 / 0 / 0 = **0/8 official** | Malformed parent; unusable | 1,754 / 126,515ms |
| `qual05-qwen38-v11-dev2` / slice-capacity | 2 / 2 / 2 / 2 = **8/8** | Complete; eligible insight | 2,398 / 171,202ms |
| `qual05-qwen38-v11-dev2` / trivial-wrapper | 2 / 2 / 2 / 2 = 8/8 omission | Complete; intentional omission | 479 / 34,870ms |

The targeted nested insight error did not recur in either allocation response.
The first allocation insight lacked one fixture-specific local detail and one
measurement detail, earning 6/8. The second locking response instead contains an
**invalid JSON escape in `suggestions[0].action`**. The strict final-content JSON
decoder rejects the entire parent object before insight extraction. Coordinator
inspection independently confirmed this parse failure. Its diagnostic content
scored 2 / 1 / 1 / 2 = 6/8, but it was not delivered as a usable insight and gets
zero official score/coverage; it is not retained. No parser rule was relaxed and
no response was repaired or replayed. The independent scorer found **zero critical
false claims across all six responses** and confirmed both intentional omissions.

Official totals are **5/6 usable, 5/6 complete, 3/4 useful substantive, 2/2
intentional omissions, zero critical claims**. The required six complete and four
useful substantive attempts were not achieved. No qualification candidate or
qualification HEAD was frozen. This small pilot supports improvement at the
targeted schema boundary; it does not establish reliable overall JSON delivery.

All six calls ended with `stop`, with zero transport timeouts, truncations or
invalid provider metadata. Batch token totals were **4,771** and **4,631**, or
**9,402 completion tokens including reasoning**. For the five usable completed
summaries, latency median / nearest-rank p95 were **110,977 / 207,803ms**; per-batch
successful values were 110,977 / 207,803ms and 103,036 / 171,202ms. The malformed
attempt's measured 126,515ms is excluded from successful latency. The receipt
validator groups this one non-success under its `timeout/censored` label; it was
not a transport timeout. Including all six observed call durations, median / p95
were 118,746 / 207,803ms (batch medians 110,977ms and 126,515ms).

Coordinator checks matched all six score keys and SHA-256 response digests,
stored both score maps, and passed both offline receipt validators. All original
private response files were then discarded with the runner's `discard` mode.
Source-free receipts and the scoring audit remain under the ignored evaluation
root, in the two run receipt files and `qual05-qwen38-v11-scoring/`. The durable
coordinator record is `.mini-orca/autopilot/coordinator/QUAL-05-v11-pilot.json`.
Full headless `make check` and independent code review passed before collection,
including 327 Desktop tests; the verdict changes only documentation, so those
code gates were not rerun.

The following offline validation is repeatable and makes no provider calls:

```sh
for run in qual05-qwen38-v11-dev1 qual05-qwen38-v11-dev2; do
  go run ./cmd/engineering-insight-eval \
    -receipt ".mini-orca/autopilot/engineering-insight-evaluation/${run}-receipt.json" \
    -cases internal/app/testdata/engineering-insight-eval/cases.json \
    -candidate-id qwen38-v11-schema-1 -provider configured-bug \
    -model qwen/qwen3.8-27b -prompt-version file-analysis-v11 \
    -corpus-id engineering-insight-v1 \
    -base-revision 7bcea884d64fbadfc4fc58cce5a2d8afbe0225ab \
    -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
done
```

The exact historical collection commands are retained in the private coordinator
record; do not replay either run or the grant command above. Campaign accounting
is **24 development / 0 qualification**. All 24 conditional qualification slots
remain unused; QUAL-06 stays Pending and REL-01 Blocked. The approved failure path
keeps the schedule Paused with no further grant or prompt repair. A subsequent
recovery must address JSON delivery and use a separately authorized finite pilot.

## Structured-output recovery — 2026-09-09

The user authorized schema-constrained file analysis and one fresh six-request
local pilot after v11's invalid JSON escape. The new candidate is
`qwen38-v12-structured-1`, prompt/cache identity `file-analysis-v12`, model
`qwen/qwen3.8-27b`, and provider label `configured-bug`. The new append-only grant
`authqual05-qwen38-structured-1` extends the development ceiling from 24 to 30 only at
24 consumed; qualification stays separately capped at 24. Prior grants and
failed receipts remain historical evidence and must not be replayed.

Production file analysis and evaluation send the same strict
`response_format: json_schema` contract. Other chat/edit calls keep their normal
format. The schema constrains the parent, optional four-field insights, risks,
suggestions and task-spec shapes. Target identity, aggregate insight length and
semantic correctness still require local parsing/scoring. No returned JSON is
repaired and unsupported structured requests do not fall back to ordinary chat.
The response-format identity binds resumable evaluation runs; older cached file
analysis is stale. Providers used for selected-file analysis must support the
schema request. [LM Studio documents this interface](https://lmstudio.ai/docs/developer/openai-compat/structured-output).

Offline verification used the installed Qwen tokenizer and the actual MLX VLM
schema compiler (`llguidance` 1.7.6 / `mlx-vlm` 0.6.5), with no weights loaded,
network access or generation. The final schema SHA-256 was
`579712bc61574034acd3fbbfe3bacc0bb5421bcd16d693abedc04ee158c1fa88`.
All ten synthetic checks passed without compiler warnings: valid omission,
null/full insights and escaped action strings were accepted; scalar, partial,
empty and extra-field insights, extra parent fields and invalid action escapes
were rejected. This verifies local grammar behavior, not live reasoning or
content quality. The frozen manifest binds the compiler/runtime files as well
as model artifacts, configuration, prompt/schema and corpus.

The pilot uses `qual05-qwen38-v12-dev1` and `qual05-qwen38-v12-dev2` at one clean
base, the same three development cases twice. Preserve low thinking, temperature
1, top-p 0.95, top-k 20, min-p 0, presence penalty 0, repeat penalty 1, application
input budget 16,384, actual runtime context 119,552, parallelism 1, no speculative
draft, 4,096 completion tokens including reasoning, and 300 seconds per attempt.
Verify runtime/manifest before each batch. Collect both before scoring, with no
probes, retries or tuning; stop on incompatible runtime, drift or uncertainty.
Require 6/6 usable and complete, 4/4 substantive >=6/8, both intentional control
omissions and zero critical false claims. Only a passing reviewed pilot can
promote a qualification candidate and resume the paused scheduler for QUAL-06.

Code `45cc31c6fae87f41370c2f0c3f0180d6e67fc294` passed fresh independent review
and the full coordinator headless `make check`: Go tests, race, vet, formatting,
dispatcher checks and all 327 Desktop tests (zero failures/errors/skips). Focused
Go tests and Go quality also passed. The first review required a durable stop on
permanent structured-request rejection; one Terra repair added that stop,
counted/exportable failure evidence and zero-call resume behavior. Both
production and evaluation preserve ordinary malformed-content accounting.

The first full gate had one assertion failure in the unchanged Desktop test
`explicitSendMakesOneChatRequestAndPreservesFunctionRemoteConsent`; it passed a
focused rerun without source changes, then the entire final suite passed. The
original failed gate remains in private diagnostics. Headless/offscreen tests
are not native-window acceptance. No Desktop source or production parser was
changed by this recovery.

The six-request grant has been applied once under the reviewed identifier
`authqual05-qwen38-structured-1`; campaign usage remains 24 development /
0 qualification before collection. A coordinator draft initially omitted the
`auth` prefix; that CLI invocation was rejected before any ledger change or
request consumption. The final candidate manifest SHA-256 is
`9d4261b8ec84a1296ee2247269385127bad5e09d8f3dd7ae77e5cffda3f77f64`.
The scheduler remains Paused until this pilot's reviewed verdict. Exact commands
and progress are retained in
`.mini-orca/autopilot/coordinator/QUAL-05-structured-recovery.json`.


## V12 structured pilot verdict — 2026-09-09

**QUAL-05 failed; the scheduler stays Paused.** Both batches completed at frozen
base `8fffeebc7394d4a0e5b746a2b4bbd5a3781bc259`, after both runtime/manifest
preflights passed. All six were malformed, unusable and incomplete, with normal
`stop` completion, valid usage metadata and no timeout. Official coverage is
**0/6 usable/complete, 0/4 useful substantive insights and 0/2 successful control
omissions**. A missing final control answer cannot count as intentional omission.

| Batch | Case | Completion tokens | Latency | Final content |
| --- | --- | ---: | ---: | --- |
| dev1 | locking | 544 | 43.022 s | empty |
| dev1 | allocation | 422 | 30.941 s | empty |
| dev1 | control | 73 | 5.762 s | empty |
| dev2 | locking | 541 | 41.683 s | empty |
| dev2 | allocation | 514 | 38.118 s | empty |
| dev2 | control | 74 | 5.777 s | empty |

This pilot consumed **six requests and 2,168 completion tokens including
reasoning**. Median latency was 34.530 seconds; maximum 43.022 seconds. Campaign
usage is **30 development / 0 qualification**, with no remaining development
slots. All 24 qualification slots remain untouched and conditional on a passing
pilot. No new request, retry or grant followed the failure.

The coordinator matched every retained response digest to its original LM Studio
completion envelope. All six envelopes have empty `content` and nonempty
`reasoning_content`; retained bytes match the reasoning channel exactly. The
source-free channel audit is
`.mini-orca/autopilot/engineering-insight-evaluation/qual05-qwen38-v12-scoring/channel-audit.json`.
This establishes a delivery failure rather than a timeout or JSON escape failure.
Reasoning-only material is diagnostic and cannot be substituted for final output.

Installed runtime inspection supports the mechanism: the `mlx_engine.generate`
BatchedVision branch immediately appends `build_json_schema_logits_processor`
when a schema is supplied. It does not use the installed thinking-aware wrapper,
while the verified low-thinking template ends in an open `<think>` channel.
This suggests the schema constrains reasoning before a final answer starts.
The offline grammar checks were valid but did not exercise this channel boundary.

The next proposed recovery is a **new explicitly thinking-disabled model profile**
with the same schema and a separately authorized bounded pilot. Read-only
`applyPromptTemplate` checks show the model's explicit `enableThinking=false`
override closes the reasoning channel before generation. The installed API
`none` mapping produces a generic false flag, but skips the model's reasoning
level field with a warning; testing that generic flag alone did not produce the
required template. Do not assume `reasoning_effort: none` is sufficient: verify
the effective model template and all other frozen settings before dispatch.
Private source-free mapping/template proofs are in the coordinator directory.
No saved model/runtime preference or frozen candidate setting was changed.


Fresh independent Terra scoring confirmed all six whole-response digest joins
and schema-valid diagnostic JSON. Official dimension scores are **0/0/0/0** for
every attempt. Diagnostic reasoning-only scores, in correctness / local relevance /
trade-off clarity / useful verification order, were locking **2/2/1/2** and
**2/1/1/2**, allocation **2/2/2/2** and **2/2/1/2**; both diagnostic controls
intentionally omitted insights. No critical false claim was found in the retained
diagnostic material. These observations do not provide final-output coverage or
prove that disabling thinking will preserve this quality.

The strict score maps and source-free `audit.json` are in the same private
scoring directory. The coordinator stored both digest-bound score maps, passed
both offline receipt validators with exact frozen identities, then discarded
all six evaluator response files and verified their absence. No example was
retained. Existing LM Studio server logs were read in place for channel diagnosis;
this evaluator cleanup does not purge the host application's log retention.
Both validated receipt reports remain under `.mini-orca/autopilot/diagnostics/`.

The accepted implementation and full prepilot `make check` remain recorded above.
No code changed during collection or diagnosis. This failed profile is not a
promoted qualification candidate; QUAL-06 remains Pending and REL-01 Blocked.


## Thinking-disabled recovery — 2026-09-09

The user authorized exactly six additional development requests for a fresh
thinking-disabled candidate. The candidate is `qwen38-v12-thinking-off-1`, using
`qwen/qwen3.8-27b`, provider `configured-bug`, unchanged `file-analysis-v12` and
unchanged strict response schema. Grant `qual05-qwen38-thinking-off-1` appends six
slots only at 30 consumed; the development ceiling becomes 36, qualification
remains 24. The fifth ledger entry requires reasoning effort `none` and exact
candidate/model/prompt plus loopback dispatch. All earlier results remain intact.

The coordinator switched the loaded model's **Enable Thinking** off through
LM Studio's server inference settings. Read-only base configuration confirms
`ext.virtualModel.customField.qwen.qwen3.827b.enableThinking=false`; this is the
only base-setting change from the v12 pilot. Load configuration, runtime context
119552, application input budget 16384, single lane, model/runtime artifacts and
sampling settings remain unchanged. The private evaluation bug profile changes
only `reasoning_effort: low` to `none`. This is a server-session override, not an
edited model artifact or saved default; runtime/base drift blocks dispatch.

**Diagnostic correction:** the prior generic-flag probe used an unqualified
configuration key and did not establish what the actual request does. The
correct fully qualified key is `llm.prediction.reasoning.enableThinking`.
The corrected readback merges the effective server base with the mapped request
flag, and verifies that both base and request templates close the thinking
channel before generation. An additional isolated readback removes both thinking
fields from the base, then changes only the full generic boolean: true opens
the channel and false closes it. This addresses the independent reviewer's
concern that the explicit custom field could mask the generic flag's effect. The installed `none` mapping still warns that `off`
is not a reasoning-level enum, but supplies the separate boolean thinking flag.
This warning does not establish that the boolean is ineffective. These are
read-only template checks, not generation or proof of content quality.

Collect the three development cases twice, without tuning, automatic retry or
additional probes. Recheck the frozen manifest and runtime before each batch;
use 4096 maximum completion tokens and 300 seconds per request. A fresh independent
scorer must inspect every emitted response and final answer, join exact digests,
and require 6/6 usable/complete, 4/4 substantive >=6/8, 2/2 intentional control
omissions and zero critical false claims. Reasoning-only output cannot be counted
as final output. Validate receipts offline and discard evaluator response files
when scored, retaining only eligible examples and source-free evidence.

The scheduler stays Paused during collection. A failed pilot leaves QUAL-05
Blocked with no further development grant or live retry. Only a passing reviewed
pilot promotes a frozen candidate and resumes the existing 24-request QUAL-06
schedule; no release is authorized.

Accepted code: `598f516c019065769e826050a5181fb5555b4f73`. Independent
code review and focused tests passed; coordinator headless `make check` passed
all Go, race, vet, formatting, dispatcher and 327 Desktop tests, with zero
failures/errors/skips. No schema, analysis prompt or production output parser
changed. The fifth grant has been applied once; usage remains 30 development /
0 qualification before collection. Candidate manifest SHA-256:
`2c61bdbfdb6b3edded2f1c685d5900b280642f12fa437bf3c1e8116b9e9a47ed`. Exact commands,
preflights, validation and progress are retained in
`.mini-orca/autopilot/coordinator/QUAL-05-thinking-off.json`.

Retained working evidence, including pre-existing user edits:

- [Task 170 execution record](../tasks/170_ui_precision_accessibility.md)
- [UI precision evidence](../desktop/UI_PRECISION_ACCEPTANCE.md)
- [Visual fixture reproduction](../desktop/VISUAL_REVIEW.md)
- [Keyboard/native checklist](../desktop/KEYBOARD_SMOKE_CHECKLIST.md)
- [Current contrast measurements](../desktop/UI_CONTRAST.md)

These files are temporarily retained to preserve ongoing work. Their dated
baseline sections describe history. REL-02 consolidates current reproduction and
remaining checks here before retiring redundant history.

## FND-01 reconciliation

FND-01 started at `d0a7cc9`. The only pre-existing worktree edit was the coordinator's
`PLAN.md` status transition for FND-01; it is not part of the evidence change. The preserved Task
170 source/test record was already tracked in that baseline, while all rendered PNGs are ignored.
The current eight-fixture matrix, command, runtime identity, scale, artifact names, and native
limitation are recorded in [UI precision evidence](../desktop/UI_PRECISION_ACCEPTANCE.md).

UI-04 closed the attainable native window, edge popup, operating-system focus and screen-reader
name/state inspection inherited from Tasks 149/170. REL-01 owns provider, lifecycle, package,
distribution and any release matrix combinations that require a different operator surface;
LEARN-02 owns insight usefulness; FND-02–04 and AUTO-01 own the quality findings.

## REL-01 execution — 2026-09-08

Initial integrated base: `bd727e1980cada41e8a5a6d01945143cbb07163c` (`4.4.0`),
clean before execution. The final repair validation below includes the uncommitted
REL-01 complexity repair. The host was macOS 26.6.2 (25G83), arm64, with Temurin
21.0.11+10 as Gradle launcher and ignored JBR `25.0.4+1-b508.27` as Desktop
toolchain. No user project, credential, or real remote provider was used.

| Evidence | Result |
| --- | --- |
| `make check` | Passed with the documented JDK 21/JBR 25 split: Go tests, race tests, vet, dispatcher tests, and Desktop tests. |
| `make quality` | Passed after the repair: staticcheck, deadcode, complexity, clone detection, Spotless, and Detekt all passed. |
| Desktop package | The repaired checkout's `./scripts/desktop-gradle.sh createDistributable` passed with JBR 25 and produced macOS arm64 `desktop/build/compose/binaries/main/app/Mini-Orca.app`, version 4.4.0, 159,968 KiB (~156 MiB). `Info.plist` identifies `io.miniorca.desktop`; the launcher SHA-256 is `229007d006b64fb1666943c0c77301e3b0e35b392d617ab9944721b9ad7ffde7`. |
| Bundle runtime and startup | The freshly generated embedded release manifest reports `JAVA_VERSION="25.0.4"` and lists `java.net.http` and `jdk.unsupported`. The freshly packaged macOS app launched with isolated Java preferences against the loopback daemon. The native chooser selected `desktop/build/rel-01/fixture.uOcgY4` and imported it. |
| Disposable fixture | A copied Git fixture and deterministic loopback fake provider completed replace (`Run`) and create (`ReleaseNote`) separately: import → draft → validation → explicitly trusted copied-workspace checks → explicit Apply → Undo. Parse, format, lint, and tests passed for both. Only `main.go` changed during Apply and Undo restored its original content. The ignored `desktop/build/rel-01/` directory contains sanitized disposable-fixture artifacts as well as copied fixture source; no separate source-free failed-check receipt is retained. |
| Native Security and Performance accessibility smoke | After import, accessibility inspection exposed named radio-button tool windows. Security with `main.go` selected exposed named Scan and Review controls plus source-only/advisory copy; Scan returned `Completed — no findings returned; this does not prove the file is secure.` With the local provider stopped, Review moved through Running to `Failed — Review the request and try again.` Performance showed `Source-based review · Not measured`, the local provider identity, and Preview limits of 10 selected and 0 excluded/oversized/outside. Its deterministic local run exposed named Pause, Resume, and Cancel controls: Pause showed `Paused · resume explicitly`; Resume showed 9 pending/1 running; Cancel ended at `Canceled · completed reviews remain available` and `Partial · canceled review retains completed files.` The changed Security and Performance surfaces remained readable without clipping in native 800×600 and 1305×768 windows. No external provider was used. |
| Docker | Docker Desktop CLI/server 29.3.1 and Compose 5.1.0 built `mini-orca:4.4.0` as image `sha256:502e36efe017669bbc083cb418fb436882c94a89490ac7d82d839408306e3f58`. A container using `config.example.yaml` reached `healthy`; its in-container `/health` response was `{"status":"ok","version":"4.4.0"}`. No cleanup was run. |

### Continued offline acceptance — 2026-09-08

The focused deterministic checks use `httptest` loopback providers and ephemeral
`t.TempDir` project roots. They make no external request. Separately, the ignored
native-run artifacts under `desktop/build/rel-01/` retain sanitized copied fixture
source; no tracked evidence retains fixture source, provider content, or credentials.

| Evidence | Result |
| --- | --- |
| Final candidate gates | A fresh `make check` and `make quality` passed on this uncommitted REL-01 candidate after the offline acceptance and Docker portability work. The check runs Go tests, race/vet, dispatcher tests, and Desktop tests; quality runs static analysis, reachability, complexity, clone detection, Spotless, and Detekt. |
| Source-safe failed checks, stale evidence, and cancellation | `go test -count=1 ./internal/app -run 'TestApplyDraftRejectsSourceChangedAfterChecksWithoutWriting|TestApplyDraftCancellationAndFailedChecksLeaveSourceUntouched|TestDraftValidateCheckApplyAndUndoRequireCurrentEvidence|TestTaskTestChecksRequireBaseFailureAndCandidatePassWithoutWritingProject|TestTaskTestChecksReportCandidateFailureAndUseNonConflictingFilename|TestTaskTestCheckCancellationAndEvidenceSanitization|TestRunCheckCommandHonorsCancellation'` passed. It verifies failed required checks and cancellation leave imported source unchanged, stale validate/check/apply evidence is rejected, and temporary proposed test files do not enter the source project. |
| Offline provider and local/remote/mixed scope consent | `go test -count=1 ./internal/api/handlers -run 'TestPromptHandlersConfirmOnlyTheirOwnRemoteScope|TestChatSessionEndpointReportsCancellationAndStaleSession|TestDraftEndpointsAreRevisionAndHashGuarded'` and `go test -count=1 ./internal/app -run 'TestScopedModelLocalMixedAndRemoteConfirmationMatrixUsesNoProvider|TestRemoteProviderRequiresConfirmation|TestReviewSecurityFileRejectsConsentAndStaleResultsWithoutPublishing|TestExplainDeclarationHandlesRefusedLoopbackProviderWithoutWorkflowMutation'` passed. The scope matrix uses local network aliases and denies confirmation before a request. The refused loopback test first closes its own listener, then verifies the request fails without source, draft, or session mutation. |
| Analyze-all lifecycle | `go test -count=1 ./internal/app -run 'TestAnalyzeAllProcessesEligibleFilesSequentially|TestStartAnalyzeAllCompletesEmptyEligibleSelection|TestAnalyzeAllRetriesFailedFiles|TestAnalyzeAllStopsRetryingAfterRetryBudget|TestAnalyzeAllCancelRetainsCompletedEntries|TestAnalyzeAllPersistsPausedJobAndResumesAfterServiceRestart|TestAnalyzeAllBecomesStaleWhenReindexChangesRevision|TestStartAnalyzeAllRequiresRemoteProviderConfirmation'` passed. It covers start, empty completion, transient failure retry, exhausted retry budget, cancellation with completed entries retained, restart/resume, stale revision, and denied remote consent. |
| Performance lifecycle and source identity | `go test -count=1 ./internal/app -run 'TestPerformanceJobPausesResumesAndDerivesCachedReport|TestPerformanceJobRejectsChangedPreviewAndConcurrentAnalyzeAll|TestPerformanceJobReportsFailedFileAndEmptyQueue|TestPerformanceReportsMarkStaleCachedCoverageWithoutChangingCompletedJob|TestPerformanceReportsPreserveLifecycleStatusWithStaleCoverage|TestRecoverPersistedPerformanceJobNormalizesInterruptedRequests|TestPerformanceJobRecoveryResumesInterruptedFile|TestReviewPerformanceFileRejectsChangedSourceOrPolicyBeforePublication|TestReviewPerformanceFileRechecksCancellationDuringPublicationAuthorization|TestReviewPerformanceFileCancellationPreventsPublication'` passed. It covers empty and failed queues, pause/resume/cancel with partial coverage, preview mismatch, stale reports, exhausted-budget recovery, restart recovery, and publication cancellation. |
| Exact-function handoff | `go test -count=1 ./internal/api/handlers -run 'TestChatSessionEndpointsKeepMessagesBoundToOneFile|TestChatSessionEndpointPinsTaskSpecsAndRejectsUnpreparedRepairs'` and `go test -count=1 ./internal/app -run 'TestAnalyzeFileValidatesAndCachesOneExactBugTaskWithoutAnotherModelCall|TestExplainDeclarationRequiresConsentAndRejectsIneligibleOrStaleTargetsWithoutProviderCall'` passed. The selected path, symbol, hash, and task specification stay pinned through the prepared handoff; unprepared repair, stale, ineligible, or denied requests do not send a provider request. |
| Fixture and corrupt-cache boundary | `go test -count=1 ./internal/project -run 'TestReleaseFixtureRepresentsPolicyAndParserAcceptanceCases|TestPerformanceCacheRecoversMalformedFile|TestPerformanceCacheIsStaleWhenSourceIsUnavailable'` passed. It confirms the fixture's secret placeholders are excluded, parser cases are isolated, corrupt cache is recovered, and unavailable source becomes stale. |
| Native Docker portability repair | The Dockerfile now fails before `go build` when either BuildKit `TARGETOS` or `TARGETARCH` is empty. Normal `docker build --progress=plain -t mini-orca:rel-01-native-r1 .` and `docker compose build mini-orca` passed, each logging `GOOS=linux GOARCH=arm64`. The explicit negative `docker build --progress=plain --build-arg TARGETOS= --build-arg TARGETARCH= -t mini-orca:rel-01-empty-target .` failed at the target guard with exit code 1. The current tag `mini-orca:rel-01-native-r1` has ID `sha256:2c90266ffab445dd0a95a43eb00977abff82be42a177459178098d0b8b8e0be5` and platform `linux/arm64`. The newly retained container `mini-orca-rel01-native-r1` has ID `aee4b1f41878b9552ad2204f5adce613446b764fc12568c1a0cbc427c8170bbd`, image ID `sha256:2c90266ffab445dd0a95a43eb00977abff82be42a177459178098d0b8b8e0be5`, platform `linux`, and is `running healthy`; its sole loopback endpoint `127.0.0.1:52120` returned `{"status":"ok","version":"4.4.0"}`. No Docker cleanup was run. |
| Live-provider compatibility and insight quality, 2026-09-08 | The normal loopback daemon API used the configured local OpenAI-compatible provider and `qwen/qwen3-coder-30b` model alias. Both derived configurations had mode 0600, no API key, local loopback scopes, 800 output-token caps, and one retry maximum. `analyze` was local, so no external data transfer or remote consent prompt occurred. The two runs persisted two `analyze` imports and five `bug` selected-file analyses. Source-free local provider request metadata recorded exactly seven chat-completion requests matching those operations, establishing the exact 7/8 budget and that no retry request occurred. The first run completed its import and cancellation analysis, while n-plus-one and idempotency stored the source-free parser failure `The model returned an unusable file summary.` The second completed its import plus fresh cancellation and idempotency analyses with `file-analysis-v5`. Those parser failures occur after successful transport and outside `s.retry`. The source-free receipt independently validated with `go run ./cmd/engineering-insight-eval -receipt /tmp/mini-orca-rel01-insight-receipt.json -cases internal/app/testdata/engineering-insight-eval/cases.json -provider configured-local-openai-compatible -model qwen/qwen3-coder-30b -prompt-version file-analysis-v5 -max-requests 8 -max-output-tokens 800`; it contains no provider reply, source, endpoint, or credential. The second-run cancellation/locking result scored 0/8 because it gave a generic mutex narrative rather than the lock-held blocking wait and a contending-lock verification. The idempotency result scored 4/8: it identified delegated charging and duplicate-risk context but omitted an idempotency trade-off and concrete retry verification. Neither result is retained; neither was scored as a critical false claim. |

The release scope is the tested macOS arm64 desktop package and the Linux arm64
container on this Docker Desktop host; no other desktop or container platform is
claimed. REL-01 remains blocked on model-quality qualification.

### Precision repair and larger-budget diagnostic — 2026-09-08

Provider finish metadata shows that the original failures were not truncation:
all seven replies ended with `stop` at 373–731 completion tokens. Both rejected
summaries invented parameter/field/type keys in `symbol_explanations`. The v6
prompt explicitly limits those keys to indexed declarations and asks for conditional,
source-grounded insights. The parser now omits the whole invalid optional explanation
map while preserving the useful parent; malformed JSON, invalid risks and target
validation remain strict. No invalid explanation becomes a navigable declaration.

A second diagnostic used the same local model, eight fixture cases, exactly one
request per case, 4,096 output tokens and a 300-second deadline. It exercised the
production prompt/client/parser with deterministic project facts; it did not repeat
import or desktop acceptance. All eight stopped normally at 345–528 completion
tokens; observed per-case latency was 4.2–15.8 seconds. Before optional-section
isolation, 3/8 responses parsed. Offline replay of the identical responses after the
repair produced 8/8 usable parent summaries: 3 complete and 5 degraded by omitted
explanations. No further live requests were used for replay. This is a repair check,
not a fresh 100% reliability result. Generic insights still appeared on trivial
controls, so larger output budgets alone did not resolve teaching quality.

Qualification now requires a preselected sample of at least 20 attempts, >=95%
usable first attempts, >=90% complete explanation sections, >=80% qualifying
insights on substantive cases, zero critical false claims, and omission on every
trivial control. Retained insights still require >=6/8. Record latency median/p95,
completion reasons, output tokens, degraded results and retries separately. The
current eight-case diagnostic does not meet that qualification requirement.

The opt-in collector makes eight local-only requests at 4,096 tokens, 300 seconds
each, with no retry. It uses the ignored root configuration in memory and writes
observations to ignored `.mini-orca/autopilot/insight-live-evaluation.json`; it never
writes credentials, endpoint URLs, source, or raw replies. Observations record only insight presence; evaluate prose separately in the local
provider/app session and retain only scores in a publication receipt. Normal test runs
skip it. A passing collector means collection completed, not that quality passed:

```sh
MINI_ORCA_LIVE_INSIGHT_EVAL=1 go test ./internal/app \
  -run '^TestEngineeringInsightLiveEvaluation$' -count=1 -timeout=45m -v
```

Validation after the precision repair: `make check`, `make quality`, focused
regression tests, and `git diff --check` passed. The live collector is skipped
by default; no provider request was made by those validation gates.

## Reproduce acceptance

Use a temporary copy of [the release fixture](../internal/project/testdata/release-fixture/README.md).
Keep one non-Git copy and a separate disposable Git fixture. Use `.fixture` parser,
vet and test failure inputs only in copies. Placeholder secrets must remain
excluded; never use a real user project or credentials in screenshots/test records.

1. Run `make check`, `make quality` and `git diff --check` from the integrated
   candidate with the documented [desktop runtime](../desktop/README.md#runtime-and-build).
   Record base/result identity, exact commands, failures and skipped stages.
2. Start from an empty desktop preference profile; open/cancel/retry, restore after
   restart, navigate workspaces/files/symbols and inspect the real relative path.
   Restore/navigation must not cause a model request.
3. Replace fixture `Run`, and separately create absent `ReleaseNote`: send, edit
   only the draft, validate, check, review, explicitly Apply and Undo. Verify one
   source file changes; stale selection/source/draft/check evidence blocks Apply.
4. Exercise failed required checks, explicit repair limits, cancellation, source
   changes during requests, denied remote confirmation and provider offline errors.
   Temporary proposed tests never appear in the imported project.
5. Test all-local, one online-compatible and mixed `analyze`/`bug`/`function` scope
   configurations with explicit request consent. Record provider type/model/version
   and results, not keys/prompts. Use an agreed finite request budget.
6. Exercise Analyze-all and Performance preview/start/pause/resume/cancel/restart,
   honest empty/partial/budget-limited/stale/failed states and exact-function handoff.
   Inspect real insights for local relevance, trade-offs and useful verification;
   simple valid JSON alone does not pass content-quality review.
7. Run native views at 1440×900, 1920×1080, 1000×760, 999×760, 800×650 and 1280×600;
   check 100/125/150% text and 1×/2× density where supported. Include long content,
   drawers, splitters, menus/dialogs at window edges, errors and populated Review.
   Use keyboard-only navigation and a supported reader; record observed focus,
   selected/expanded/disabled state, Escape, text selection and resize behavior.
8. Package with JBR 25 and smoke the actual supported artifacts. Verify runtime
   modules, startup and image contents. For Docker, build and verify `/health` and
   container health without cleanup commands. Do not claim untested platforms.
9. As new features land, add their concrete Security/trust/benchmark scenarios to
   this matrix. New feature evidence cannot be inferred from the earlier UI runs.

A release is accepted only when all required checks for its explicitly named scope
pass with evidence. Record operator/date, platform/runtime, fixture, candidate
identity, artifact locations and remaining limitations. Missing required evidence
keeps the release incomplete; independent implementation work can continue.
