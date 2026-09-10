# Codebase cleanup and maintainability audit

Audit the existing Go daemon, Compose desktop client, tooling, and operational documentation; reduce avoidable complexity while preserving behavior. The user authorized background implementation of CLN-01 through CLN-12 with agents on 2026-09-10. Only the cleanup scheduler is active; model evaluations and the historical autopilot remain on standby.

## Background execution — authorized 2026-09-10

Automation: **Mini-Orca cleanup agents** (ID: mini-orca-cleanup-agents), attached to this Codex task, active every 20 minutes. Each wake handles at most one ordered cleanup task with one implementation agent, a fresh independent reviewer, and coordinator-run checks. Scheduler setup accepted no implementation. CLN-01 through CLN-05 are accepted; CLN-06 is next. PLAN.md owns current status.

PLAN.md owns status and concise acceptance evidence; the checkboxes below mirror accepted work. The user authorized local commits on 2026-09-10: commit the first three accepted tasks together, then commit every later accepted task separately after independent review and coordinator checks. The coordinator stages only that task and its ledger/checklist updates; workers and reviewers do not commit. Preserve earlier accepted changes and unrelated user edits. Pause the cleanup automation after exhausted repairs or final completion. The historical dispatcher and insight qualification stay paused. This authorization excludes pushes, releases, and live Mini-Orca model evaluation.

## Audit baseline — 2026-09-10

Reviewed clean commit **fe4cdad**. Only this file changed during the audit; scheduler setup subsequently registered the cleanup queue in PLAN.md. The previous recovery specification is retained in Git at **fe4cdad:docs/tasks.md**; PLAN.md and docs/RELEASE_ACCEPTANCE.md retain the current standby decisions and historical verdicts.

**Assessment:** the codebase has useful package boundaries, substantial behavioral tests, shared storage/context/UI primitives, and little detected copy-paste duplication. Its main maintainability problems are concentrated workflow ownership, campaign-specific code mixed into the application package, silent background persistence failures, and a few unused desktop methods. A rewrite or arbitrary line-count reduction is not justified.

Source inventory (physical lines, including comments/blanks; excludes generated output): Go production/fixtures 19,053; Go tests 14,077; desktop production 16,893; desktop tests 11,139; scripts 2,705; script tests 1,857. These are scale indicators, not quality scores.

### Findings and priorities

| ID | Priority / evidence | Finding and intended response |
| --- | --- | --- |
| F01 | P1; scripts/quality.sh:40 | The reachability stage trusts the deadcode process exit code. The pinned tool prints findings and returns normally even when it finds unreachable functions. Make findings fail the gate; current analysis itself reported none. |
| F02 | P2; DesktopLayoutState.kt:95,100,108; DesktopState.kt:629 | closeLeft, closeRight, closeBottom, and DesktopWorkflowController.synchronize have no references in repository source/tests. Remove these four ordinary methods after rechecking references; do not infer that other public members are unused. |
| F03 | P1; internal/app/engineering_insight_runner.go:1946 | writeRunnerJSON duplicates atomic-file mechanics. Its combined write/sync/close condition short-circuits, so write or sync failure can skip Close. Reuse internal/storage.WriteFile for replaceable metadata. Preserve the separate immutable hard-link writer. |
| F04 | P1; internal/app/analyze_all.go:575,583,617 | Worker admission/progress/completion discard persistence errors. In-memory progress can diverge from restart state, and provider work can proceed without its progress being saved. Handle failures before structural refactoring. |
| F05 | P1; internal/app/performance_job.go:486,495,540 | Performance has the same failure class with different lifecycle and budget rules. Keep those rules distinct and make failed persistence observable. |
| F06 | P2; internal/app/go_scan.go:251 | A failed asynchronous scan can also silently fail to save its failure report. Keep the original scan failure and expose the persistence failure through the existing diagnostics boundary. |
| F07 | P2; internal/app/file_analysis.go:398; engineering_insight_runner.go:1665,1736 | Production parsing, optional-section assessment, and evaluation diagnostics repeat decoding/validation of the same response. Return diagnostics from the production parsing boundary and reuse them in evaluation. |
| F08 | P2; internal/app/engineering_insight_runner.go and engineering_insight_evaluation.go | These two files total 2,819 lines and mix fixed campaign identities, budgets, receipt validation, private artifacts, and provider collection into package app. They are reachable from the evaluation CLI, not dead code. Isolate them without removing historical contracts. |
| F09 | P2; DesktopWorkflowPresenter.kt:996–1115,418–528 | The 1,952-line presenter owns connection, project/file loading, analysis, explanation, security, chat, drafts, jobs, and benchmarks. Extract the benchmark and security operation owners in separate passes; retain one authoritative state store. |
| F10 | P2; PLAN.md, docs/RELEASE_ACCEPTANCE.md, docs/insights-performance/README.md | Current decisions and extensive superseded campaign instructions coexist. Separate active navigation from retained historical evidence without deleting verdicts or changing authorization. |

Paths abbreviated in F02/F09 are under desktop/src/main/kotlin/io/miniorca/desktop.

### Checks actually run

- make quality — passed: Staticcheck, Go reachability, Go complexity, Go clone detection, Desktop Spotless and configured Detekt checks.
- go test ./... — passed.
- make test-race — passed.
- make fmt-check and make vet — passed.
- ./desktop/gradlew -p desktop test — passed with cached results; then test --rerun-tasks passed with all eight Gradle tasks executed.
- python3 -m unittest discover -s scripts/tests -p 'test_*.py' — 51 tests, passed with one explicitly opt-in runtime-conformance skip.
- Additional Kotlin clone scan at 100 tokens / 10 lines — three matches, all import blocks (0.26% of analyzed lines). Existing Go scan at 70 tokens / 8 lines reported zero clones. These thresholds do not prove absence of smaller or semantic duplication.
- Native UI/accessibility smoke, live providers, pinned insight-runtime conformance, packaging, dependency vulnerability/version audits, and non-host platform execution were not run. The composed make check command was not invoked; its constituent check categories were run as listed.

The desktop build emitted two redundant-nullability warnings in ReviewEvidencePaneTest.kt:214–215, plus Gradle/Skiko/Jewel compatibility warnings. Fix the two local warnings with Task CLN-02; dependency upgrades are a separate compatibility investigation, not an automatic cleanup change.

### Constraints and non-goals

- Preserve loopback defaults, explicit provider consent, source policy, source/revision checks, one-file preview/apply, benchmark execution trust, and read-only source/diff surfaces.
- Preserve API routes, JSON field names, cache namespaces, schema/prompt identities, and persisted evidence unless a task explicitly describes an additive diagnostics field.
- Keep Analyze-all and Performance as separate domain workflows. Similar loops do not justify a generic job engine or generic report cache.
- Retain internal/storage, source-file snapshot validation, context policy, DesktopJobCoordinator, DesktopTheme, and ChromeControls as the existing reuse boundaries.
- Do not change model prompts, merge analysis/security/performance requests, delete tests/fixtures for standby capabilities, erase campaign budgets, or remove evaluation safeguards.
- Do not treat hash/buffer writes or intentionally discarded optional insights as equivalent to ignored filesystem errors.
- Local cleanup commits are explicitly authorized as described above. Provider calls, unrelated scheduler changes, Docker cleanup, and generated-file edits are excluded.
- Tasks are ordered below and explicitly depend on predecessors where needed. PLAN.md remains the status ledger; this file is the cleanup specification. Implementation should register this new scope there without reopening completed or standby work.

## Task CLN-01 — Make the reachability gate enforce its result

- [x] CLN-01 accepted after independent review and coordinator verification.

Accepted 2026-09-10: fresh review reported no actionable findings; coordinator reran the four isolated tests and all Go-only quality stages successfully, plus shell syntax and diff checks. Initially accepted without a commit; included in the later authorized CLN-01–03 batch. Exact candidate/evidence is recorded in PLAN.md and .mini-orca/autopilot/cleanup/CLN-01/acceptance.json.

**Target files**
- scripts/quality.sh — distinguish findings from tool execution failures.
- scripts/tests/test_quality.py — new isolated gate regression tests.

**Inputs / dependencies**
- None. F01; pinned deadcode v0.40.0 and the two current executable roots.

**Implementation rules**
- Capture structured or unambiguous output and fail on any reported unreachable function as well as tool failure. Preserve actionable diagnostics and aggregate reporting of other stages.
- Empty results pass. Do not hide findings with a broad allowlist or add test roots to make otherwise-unused production functions appear live.
- Keep tools pinned and outside go.mod. Tests use temporary fixtures or executable stubs; test clean output, findings with exit zero, and tool failure without network/model calls.

**Verification command**

    python3 -m unittest discover -s scripts/tests -p 'test_quality.py'
    ./scripts/quality.sh --go-only

## Task CLN-02 — Remove confirmed unused desktop methods

- [x] CLN-02 accepted after independent review and coordinator verification.

Accepted 2026-09-10: removed four methods after reference/callback/reflection checks and retained every test assertion. Fresh review reported no actionable findings. Writer executed 327 passing desktop tests and Spotless/Detekt; coordinator reran the exact commands (up-to-date), checked JUnit results, and passed diff checks. No native smoke was needed for this nonvisual deletion; none is claimed. Evidence: .mini-orca/autopilot/cleanup/CLN-02/acceptance.json.

**Target files**
- desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt — remove closeLeft, closeRight, and closeBottom.
- desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt — remove DesktopWorkflowController.synchronize.
- desktop/src/test/kotlin/io/miniorca/desktop/ReviewEvidencePaneTest.kt — remove redundant nullable access/assertion at the two compiler warning sites.

**Inputs / dependencies**
- CLN-01; F02 and the fresh compiler diagnostics.

**Implementation rules**
- Recheck whole-repository references, callback references, and reflection before deleting; retain a method if a real consumer is discovered and record it.
- Preserve open/toggle/collapse behavior and preference serialization. Do not remove fields, states, or routes merely because no direct call was found.
- Use existing behavioral tests; no tests that assert source text or merely confirm deletion. Do not add a weak identifier-count lint as a Kotlin reachability substitute.

**Verification command**

    ./desktop/gradlew -p desktop test
    ./scripts/quality.sh --desktop-only

## Task CLN-03 — Reuse durable storage for evaluation metadata

- [x] CLN-03 accepted after independent review and coordinator verification.

Accepted 2026-09-10: shared atomic metadata writes now preserve cleanup on failures; compact bytes, private mode, directory preconditions, sanitized errors, and immutable evidence remain intact. Fresh review found no actionable issues. Coordinator passed the exact focused command, full Go/race tests, fmt-check, vet, Go-only quality, and diff checks. No migration or live evaluation. Evidence: .mini-orca/autopilot/cleanup/CLN-03/acceptance.json.

**Target files**
- internal/app/engineering_insight_runner.go — replace writeRunnerJSON file operations with the established storage primitive.
- internal/app/engineering_insight_runner_test.go — metadata write failure and retained-content tests.
- internal/storage/atomic_test.go — extend existing operation-failure coverage only where missing.

**Inputs / dependencies**
- CLN-01; F03; internal/storage/atomic.go.

**Implementation rules**
- Keep compact JSON bytes, file mode 0600, paths, replacement semantics, and user-facing error sanitization. Preserve useful internal error context without exposing private response/source content.
- Reuse storage.WriteFile; ensure caller preconditions on the evaluation directory remain intact.
- Do not replace writePrivateArtifact: its hard-link publication intentionally rejects overwriting prior evidence.
- Verify old metadata survives failure and temporary resources are cleaned up. Retain immutable-artifact collision coverage.

**Verification command**

    go test ./internal/storage ./internal/app -run 'Atomic|WriteFile|EngineeringInsightRunner' -count=1

## Task CLN-04 — Handle Analyze-all persistence failures explicitly

- [x] CLN-04 accepted after independent review and coordinator verification.

**Target files**
- internal/app/analyze_all.go — worker admission, result, invalidation, and completion error paths.
- internal/app/file_analysis_test.go — existing Analyze-all restart/race tests and deterministic write-failure coverage.
- internal/app/service.go — a narrow injectable persistence seam only if needed for deterministic failure tests.

**Inputs / dependencies**
- CLN-03; F04; existing beginPersist serialization and current-job identity checks.

**Implementation rules**
- A failed pre-dispatch write must stop the worker before another provider request. A later failed progress/completion write must stop further work and remain observable to progress callers.
- Keep a controller-local persistence fault and return a sanitized error from the existing progress boundary; preserve current wire statuses. Do not report a durable completion or automatically resume.
- Keep completed reports available, preserve restart accounting, and require a successful explicit recovery action before dispatch continues.
- Distinguish an obsolete worker/revision from a real storage failure. Retain useful diagnostics through existing logging/error patterns without raw source/provider text.
- Cover failure after successful job creation, failure between files, final-write failure, explicit recovery, and project replacement. Use channels/injected failures, not timing sleeps or chmod-dependent tests.

**Verification command**

    go test -race ./internal/app -run 'AnalyzeAll' -count=1

## Task CLN-05 — Handle Performance persistence failures explicitly

- [x] CLN-05 accepted after independent review and coordinator verification.

**Target files**
- internal/app/performance_job.go — next-file admission, results, completion, and detached-project persistence.
- internal/app/performance_job_test.go — deterministic failure and recovery tests.
- internal/app/service.go — extend the narrow test seam only if required.

**Inputs / dependencies**
- CLN-04; F05; existing persistMu, generation, root, and queue identity checks.

**Implementation rules**
- Apply the same failure policy as CLN-04 using Performance's existing lifecycle. Do not merge controllers or their retry/budget rules.
- Persist running admission before provider dispatch. Surface failed progress/completion persistence and stop new work while retaining reports and elapsed/request accounting.
- A stale or detached worker must never change a replacement job, publish into another root, or consume a new job's budget.
- Exercise write failures on admission, result, completion, and project detachment, including retry/restart behavior. Preserve cancellation and partial coverage semantics.

**Verification command**

    go test -race ./internal/app -run 'PerformanceJob|PerformanceReports|ReviewPerformanceFile' -count=1

## Task CLN-06 — Preserve asynchronous scan failure diagnostics

- [ ] CLN-06 accepted after independent review and coordinator verification.

**Target files**
- internal/app/go_scan.go — failed-report serialization/persistence in runStartedGoScan.
- internal/app/go_scan_test.go — original scan failure plus failed report storage.

**Inputs / dependencies**
- CLN-04; F06; failedGoScanReport and existing output sanitization.

**Implementation rules**
- Retain the original scan failure as the primary result and report secondary persistence failure through existing sanitized diagnostics/logging.
- Do not silently convert failed report storage into durable success or drop the in-memory failure report.
- Preserve project/revision checks and cancellation. Do not unify scan execution with model-review jobs or alter execution trust.
- Use a controlled storage failure after scan failure; prove that another project's scan state is unaffected.

**Verification command**

    go test -race ./internal/app -run 'GoScan' -count=1

## Task CLN-07 — Share file-analysis validation and evaluation diagnostics

- [ ] CLN-07 accepted after independent review and coordinator verification.

**Target files**
- internal/app/file_analysis.go — have the existing parsing boundary produce the result and bounded diagnostics together.
- internal/app/file_analysis_evaluation.go — new narrow production adapter for evaluation prompt preparation and response assessment.
- internal/app/engineering_insight_runner.go — consume the adapter; remove redundant optional-state decoding/validation.
- internal/app/file_analysis_test.go — parent-response and optional-section behavior.
- internal/app/engineering_insight_test.go — malformed nested insight behavior.
- internal/app/engineering_insight_runner_test.go — production/evaluation parity.

**Inputs / dependencies**
- CLN-03; F07; current schema, prompt version, insight limits, task-spec validation, and private diagnostic contracts.

**Implementation rules**
- Keep one semantic validator. Diagnostics must distinguish omission, rejection, and accepted output without persisting field text, source, or reasoning.
- Preserve parent-failure handling, optional-section degradation, insight retention limits, and the historical diagnostic classifications even where raw accepted sections are omitted from the displayed result.
- The adapter exposes only prompt/contract preparation and bounded assessment needed by the evaluation command. Its contract includes sanitized effective provider metadata (including remote/loopback classification), so both runner authorization checks can reuse production classification after the package move. Keep transport, retries, caching, campaign accounting, and file writes outside it.
- Keep provider-origin/context binding in production helpers; do not duplicate those rules in evaluation.
- Compare prompt bytes/schema identity and assessment output using synthetic fixtures. No prompt-version bump or model request is needed for a behavior-preserving extraction.

**Verification command**

    go test ./internal/app -run 'Semantic|FileAnalysis|EngineeringInsight|Optional' -count=1

## Task CLN-08 — Move evaluation tooling out of package app

- [ ] CLN-08 accepted after independent review and coordinator verification.

**Target files**
- internal/app/engineering_insight_runner.go → internal/insighteval/runner.go — move collection/campaign ownership.
- internal/app/engineering_insight_evaluation.go → internal/insighteval/evaluation.go — move receipt validation/scoring.
- internal/app/engineering_insight_runner_lock_unix.go → internal/insighteval/runner_lock_unix.go — preserve build constraint and locking.
- internal/app/engineering_insight_runner_lock_windows.go → internal/insighteval/runner_lock_windows.go — preserve fail-closed behavior.
- internal/app/engineering_insight_runner_test.go → internal/insighteval/runner_test.go — move tests and use the production adapter.
- internal/app/engineering_insight_evaluation_test.go → internal/insighteval/evaluation_test.go — move receipt tests.
- cmd/engineering-insight-eval/main.go and cmd/engineering-insight-eval/main_test.go — update imports.
- scripts/quality.sh — keep both executable roots under reachability analysis.

**Inputs / dependencies**
- CLN-07; F08; the explicit standby scope in PLAN.md.

**Implementation rules**
- Dependency direction is insighteval → app's narrow evaluation adapter; app must not import insighteval.
- Move existing implementations and remove their old declarations. No compatibility forwarding layer, generic experiment framework, configurable grant system, or changed CLI/wire names.
- Preserve every fixed grant, cap, hash, manifest identity, private-file rule, recovery decision, and synthetic regression fixture. Existing fixture paths remain stable; do not inspect or relocate private sealed sources.
- Use the existing exported prompt/schema accessors and the new adapter instead of exporting the service's internals.
- Verify both entry points and applicable platform constraints. This separates ownership; it does not claim a reduction in reachable total code or a passing insight qualification.

**Verification command**

    go test ./internal/app ./internal/insighteval ./cmd/engineering-insight-eval -count=1
    ./scripts/quality.sh --go-only

## Task CLN-09 — Give benchmark operations one desktop owner

- [ ] CLN-09 accepted after independent review and coordinator verification.

**Target files**
- desktop/src/main/kotlin/io/miniorca/desktop/DesktopBenchmarkWorkflow.kt — new internal owner of benchmark request jobs and generation checks.
- desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt — delegate catalog/select/compare and lifecycle invalidation.
- desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt — retain end-to-end presenter benchmark coverage.
- desktop/src/test/kotlin/io/miniorca/desktop/DesktopBenchmarkWorkflowTest.kt — focused ownership/cancellation coverage.

**Inputs / dependencies**
- CLN-02; F09; existing workflow identities, DesktopState reducer, and API contract tests.

**Implementation rules**
- Move benchmark jobs, generation, catalog/comparison identity checks, and cancellation together. Keep existing event publication and the presenter API stable.
- Keep a single authoritative state store; pass a narrow snapshot accessor/event sink, not a mutable presenter reference or copied state store.
- Preserve explicit benchmark selection/execution consent, draft invalidation, stale-result rejection, project switch, and close behavior.
- Remove the replaced presenter helpers. Do not create a generic workflow base class or alter Performance report polling in DesktopJobCoordinator.

**Verification command**

    ./desktop/gradlew -p desktop test --tests '*DesktopWorkflowPresenterTest' --tests '*DesktopBenchmarkWorkflowTest' --tests '*PerformanceWorkspaceTest'

## Task CLN-10 — Give security operations one desktop owner

- [ ] CLN-10 accepted after independent review and coordinator verification.

**Target files**
- desktop/src/main/kotlin/io/miniorca/desktop/DesktopSecurityWorkflow.kt — new internal owner of scan/review requests and cancellation.
- desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt — delegate security operation lifecycle.
- desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt — retained consent and stale-result integration coverage.
- desktop/src/test/kotlin/io/miniorca/desktop/DesktopSecurityWorkflowTest.kt — focused operation ownership tests.

**Inputs / dependencies**
- CLN-09; F09; existing SecurityWorkspaceState and report/file matching helpers.

**Implementation rules**
- Move security request ownership as one unit; reuse workflow identities and the single event/state boundary established in CLN-09.
- Keep deterministic scan and AI review separate, including per-attempt remote consent, optional symbol scope, retained prior reports, and failure labels.
- Preserve cancellation and stale-result rejection for rapid scan/review changes, file/project changes, and presenter close.
- Do not move navigation or source-editing rules into this owner; do not generalize benchmark and security workflows merely because both use coroutines.
- Keep the UI composition and layout unchanged. If a visual change becomes necessary, follow desktop/UI_DESIGN_GUIDELINES.md and record separate visual verification.

**Verification command**

    ./desktop/gradlew -p desktop test --tests '*DesktopWorkflowPresenterTest' --tests '*DesktopSecurityWorkflowTest' --tests '*SecurityWorkspaceTest'
    ./scripts/quality.sh --desktop-only

## Task CLN-11 — Separate current documentation from historical execution records

- [ ] CLN-11 accepted after independent review and coordinator verification.

**Target files**
- PLAN.md — concise current status and cleanup task ledger.
- docs/history/improvement-plan-2026-09.md — new retained historical task instructions/verdicts.
- docs/insights-performance/README.md — current feature overview and historical links.
- docs/history/insight-evaluation-2026-09.md — retained campaign/runtime instructions and prior recovery specification.
- docs/RELEASE_ACCEPTANCE.md — preserve evidence; update navigation links only.
- tasks/README.md — remove superseded active-looking queue instructions in favor of historical links.
- desktop/UI_DESIGN_GUIDELINES.md — replace completed UI-01 future-tense guidance with current behavior.
- docs/tasks.md — record completed cleanup steps and documentation mappings.

**Inputs / dependencies**
- CLN-08 and CLN-10; F10; previous recovery specification at fe4cdad:docs/tasks.md.

**Implementation rules**
- Preserve historical content, identifiers, outcomes, budgets, and provenance. Clearly label historical commands as non-active; do not reconstruct disposed responses or touch ignored evaluation artifacts.
- Keep PLAN.md the sole execution/status ledger. Move detailed completed instructions into history and provide an old-to-new anchor map; update repository links to moved sections.
- Do not relabel failed or unrun qualification as passed, resume the scheduler, or change acceptance limits.
- Keep active documentation short and accurate; avoid a new duplicate architecture document.

**Verification command**

    go test ./cmd/daemon -run 'Test(ReleaseDocumentationUsesCanonicalVersion|DocumentedRoutesAreHandledByDaemon)$' -count=1
    python3 -m unittest discover -s scripts/tests -p 'test_*.py'
    git diff --check

Also manually verify moved Markdown links and retained historical status references.

## Task CLN-12 — Validate the cleanup as one release-preserving change

- [ ] CLN-12 accepted after independent review and coordinator verification.

**Target files**
- docs/tasks.md — final results, remaining limitations, and completed task status.
- PLAN.md — cleanup completion summary and outstanding dependencies.

**Inputs / dependencies**
- CLN-01 through CLN-11.

**Implementation rules**
- Review the complete diff for changed behavior, accidental public interfaces, duplicate implementations, source/privacy regressions, stale references, and removed test coverage.
- Re-run the supported validation entry point. Distinguish executed, cached, skipped, and unavailable checks; no live evaluation is required or implied.
- Confirm source/diff views, narrow-window behavior, keyboard navigation, and consent remain intact with a targeted native smoke of benchmark/security workflows when the desktop environment is available. Do not claim native verification from unit tests alone.
- Compare responsibilities and removed duplication with the audit baseline; report actual deletions separately from code moved between files. Do not set a line-count quota.
- Keep the proposed combined per-file analysis contract as a separate product decision. Cleanup requires no configuration or data migration; report any deviation before calling it complete.

**Verification command**

    ./scripts/validate.sh
    git diff --check
