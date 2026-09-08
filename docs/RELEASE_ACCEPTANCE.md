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

## QUAL-05 development pilot — 2026-09-08

The local `bug` profile remained `qwen/qwen3-coder-30b`. After the original
six-request budget, the user authorized six more development requests through
persistent grant `qual05-extension-1`; no counters or prior results were reset.
All 12 attempts completed under 4,096 output tokens and 300 seconds per attempt,
without retries. Total development consumption is 4,978 output tokens; all 24
qualification requests remain unused. The development ceiling is exhausted.

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
qualifies for promotion, so QUAL-05 remains Blocked and no candidate is frozen.
Collection validity is not evidence of useful-insight coverage or a release pass.
Historical scores and receipts are preserved; private replies are discarded after
scoring. Further development requires a new explicit candidate/budget decision;
the scheduler remains stopped.

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

The latest source-free receipt can be validated without a provider call:

```sh
go run ./cmd/engineering-insight-eval \
  -receipt .mini-orca/autopilot/engineering-insight-evaluation/qual05-v10-dev4-receipt.json \
  -cases internal/app/testdata/engineering-insight-eval/cases.json \
  -candidate-id v10-dev4 -provider configured-bug -model qwen/qwen3-coder-30b \
  -prompt-version file-analysis-v10 -corpus-id engineering-insight-v1 \
  -base-revision c0e8dc25e075549228bea8aa16020a9ebd53be81 \
  -max-requests 6 -max-output-tokens 4096 -attempt-timeout-seconds 300
```

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
