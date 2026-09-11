# Mini-Orca UX, project analysis and terminal implementation

Implement the approved 2026-09-11 product scope in [PLAN.md](../PLAN.md): readable
model results, project-wide analysis with separate result pages, prominent Go
function creation and a real terminal replacing the three duplicated bottom tools.

## Authorized execution

The user requested task preparation and scheduled execution with **GPT-6 Astra,
Extra High** (`gpt-6-astra`, `xhigh`). These 17 ordered cards are the current
implementation source of truth. [PLAN.md](../PLAN.md) owns product decisions and
the concise status ledger; [the execution guide](../tasks/README.md) defines the
per-wake procedure. The completed cleanup specification is preserved in
[the historical archive](history/cleanup-tasks-2026-09-10.md).

- Work in the current `codex/autopilot` checkout and preserve the existing UI
  changes. Re-read `AGENTS.md` and UI guidelines before relevant work.
- Process at most one card per scheduled wake. Select the first unchecked card;
  never skip an incomplete predecessor. If work spans wakes, resume its recorded
  stage rather than starting a second writer or repeating completed work.
- Use Astra Extra High for implementation and review of the diff. No additional
  agents, standalone jobs or historical dispatcher are required by this queue.
- A passing verification command plus a production-quality diff review is required
  before checking a task. Run the full desktop test suite before accepting any
  desktop-changing card, as required by `AGENTS.md`, using cached results when valid.
- The user authorized one local commit per completed task on 2026-09-11. After
  required checks and diff review pass, include that task's implementation and
  related checklist/status updates in one commit with its task ID in the subject.
  Inspect the staged diff and commit only that scope; preserve unrelated staged
  changes and exclude unrelated pre-existing working-tree edits. Never use a
  blanket add/commit of the repository. Verify/report the commit hash before
  advancing. If commit creation fails, record the pending commit stage and pause;
  on recovery check Git history before retrying to avoid duplicate commits.
- Allow the initial implementation attempt and at most two focused correction
  attempts for a concrete failure. Retain the exact command, exit/result, relevant
  diagnostics and correction history in the card. Do not reset repair accounting
  on the next wake. Exhausted failures go to `docs/errors.log` and pause the scheduler.
- Edit the listed targets and their required behavior tests. Administrative updates
  to this checklist, `PLAN.md` status and `docs/errors.log` are allowed. If a necessary
  coupled file was omitted, record the concrete compile/behavior dependency and
  narrowly correct the target list before editing it; this is task preparation,
  not permission for unrelated scope expansion.
- Preserve one-file preview/Review/Apply, source/revision checks, consent, loopback
  policy and benchmark trust. No pushes, releases, live Mini-Orca model
  evaluation, destructive cleanup or changes to historical scheduler grants are
  authorized. Dependency resolution and fake-provider/temporary-project tests are
  part of implementation; do not edit generated build output by hand.
- Advance only after required checks pass. Missing mandatory native evidence or
  incompatible terminal packaging keeps that card incomplete; report the specific
  blocker, preserve the candidate and pause rather than shipping a placeholder.

## Validation environment

Use the documented JDK 21 launcher and Java/JBR 25 toolchain. Task commands are
from the repository root. The scheduler prompt records verified local toolchain
paths; keep machine-specific locations out of versioned configuration. Direct
wrapper commands need the Java 21 launcher and a discoverable Java 25 toolchain;
passing `-Porg.gradle.java.installations.paths=<verified-java25-home>` is a runtime
location override, not a replacement for the prescribed test selection.

All cards are initially unchecked. Product decisions D1–D5 are confirmed in PLAN.md.
Task completion requires both the specified validation and its authorized local
commit; a checked card with a pending/failed commit must be finished before the
next card starts. Commit hashes can be reported in the task's final response or
an ignored local execution receipt; do not amend solely to embed a commit's own hash.

## Task UX-01 — Establish readable result primitives

- [x] UX-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt` — semantic typography and result accents.
- `desktop/src/main/kotlin/io/miniorca/desktop/ChromeControls.kt` — reusable labeled headers/badges/disclosures.
- `desktop/src/main/kotlin/io/miniorca/desktop/ModelResultContent.kt` — new bounded freeform-text presentation component.
- `desktop/src/test/kotlin/io/miniorca/desktop/ModelResultContentTest.kt` — new formatting/fallback behavior tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopThemeTest.kt` — contrast and role coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — expose the existing test-only Compose fixture for reuse by the new component tests; no existing fixtures or assertions change.
- `desktop/UI_DESIGN_GUIDELINES.md` — accepted hierarchy, disclosure and scrolling rules.

**Inputs / dependencies**
- D3; existing working-tree Context changes and the referenced dark mock.

**Implementation rules**
- Extend the existing design system; no palette migration or repeated elevated cards.
- Support the small useful formatting subset: paragraphs, emphasis, lists, inline code and fenced code. Treat unsupported markup as text; no HTML/webview, remote image loading or executable content.
- Use selectable text, visible focus and semantic labels; retain the complete underlying response when using a collapsed preview.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ModelResultContentTest' --tests 'io.miniorca.desktop.DesktopThemeTest'`

**Execution record**
Completed 2026-09-11 at the user's request. Validation and diff review passed;
the local task commit is identified by UX-01 in Git history and the execution receipt.
Captured the existing worktree and empty index under the
ignored `.mini-orca/autopilot/ux/UX-01/baseline/` directory. The narrowly added
test target exposes the existing offscreen renderer so the new component can
exercise real layout, disclosure and keyboard behavior without duplicating it.
The planning queue, execution guide and cleanup archive prepared in this task
are related documentation for the first implementation commit.

Initial prescribed verification exited 1: 19 tests, one failure in
`DesktopThemeTest.resultRolesKeepHeadingsLabelsCodeAndBadgeTextReadable` at the
badge contrast assertion. Transparent badge fills on selected rows reduced
success/error label contrast below 4.5:1. Correction 1 resolves badge fills
against the panel with an opaque result; the test checks that production color
and retains all supported-surface label assertions. The eight formatter/production
render tests passed, including disclosure activation through the keyboard and
replacement-response reset. Logs and renders are in the ignored UX-01 evidence
directory. A formatter invocation first used a relative init-script path that
Gradle resolved under `desktop/`; rerunning with its absolute path succeeded
before verification and required no source correction.

Correction-1 verification exited 1: 19 tests, one remaining failure in the same
test's raw severity-color assertion on a selected surface. The badge fix passed;
the assertion incorrectly bypassed its opaque background. Correction 2 separates
uncontained prose roles from badge roles and verifies badge contrast using the
actual resolved background on every supported parent surface. Severity labels
are not weakened or removed from the checks. Review also retained spaced emphasis
delimiters literally so multiplication in prose is not reformatted.

Final prescribed verification exited 0: 19 tests, no failures/errors/skips.
`./desktop/gradlew -p desktop test detekt spotlessCheck` exited 0: all 362 desktop
tests passed with no failures/errors/skips, no static-analysis findings, and clean
formatting. Both commands used the documented Java 21 launcher/Java 25 toolchain
runtime override and wrote offscreen renders to ignored local evidence paths.
Reviewed production renders at narrow/wide widths and 150% text, including visible
keyboard focus, expanding/collapsing the full response, wrapped labels and existing
shared chrome/summary views. This is offscreen Compose evidence, not native
screen-reader acceptance. No daemon behavior changed; Go tests and live provider
tests were not run. No dependency, configuration or migration steps are needed.

Diff review checked literal fallbacks, formatting bounds, response reset, single
scroll ownership, existing status semantics and retained full response data.
`git diff --check` passed; out-of-scope baseline files are unchanged. The local
commit includes only UX-01 implementation, fixture visibility and related planning
records; pre-existing desktop edits are excluded. UX-02 remains unchecked.

The exact commit candidate was additionally validated in an isolated source
snapshot with the pre-existing edits excluded: `test detekt spotlessCheck` exited
0, all 360 tests passed with no failures/errors/skips, and quality checks passed.
The two additional working-tree tests belong to the preserved Context changes.
All ten pre-existing desktop diffs were compared against the baseline and retained
exactly outside this commit. This check confirms UX-01 has no dependency on those
uncommitted changes.

## Task UX-02 — Apply the hierarchy to explanations and model responses

- [ ] UX-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — summary, labeled explanation facts and technical disclosures.
- `desktop/src/main/kotlin/io/miniorca/desktop/AssistantToolWindow.kt` — distinguish user request, model response and candidate action.
- `desktop/src/main/kotlin/io/miniorca/desktop/EngineeringInsightPanel.kt` — readable labels and single-owner scrolling.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProjectSummaryPane.kt` — concise project explanation with supporting detail.
- `desktop/src/main/kotlin/io/miniorca/desktop/ReviewEvidencePane.kt` — clearer evidence and blocked-action hierarchy.
- `desktop/src/test/kotlin/io/miniorca/desktop/ContextToolWindowTest.kt` — structured result and state coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/AssistantToolWindowTest.kt` — conversation/draft distinction.
- `desktop/src/test/kotlin/io/miniorca/desktop/EngineeringInsightPanelTest.kt` — optional/stale insight behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — populated/empty/long-text production renders.

**Inputs / dependencies**
- UX-01.

**Implementation rules**
- Replace joined explanation facts with short labeled sections. Keep the meaningful answer visible before provenance/metadata.
- Do not change model payloads merely for visual formatting, add AI summaries, hide errors in closed details or turn inspection into a provider request.
- Preserve Actions / Explain / Details, read-only content, file/declaration identity and responsive drawers.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ContextToolWindowTest' --tests 'io.miniorca.desktop.AssistantToolWindowTest' --tests 'io.miniorca.desktop.EngineeringInsightPanelTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

**Execution record**
Not started.

## Task CREATE-01 — Expose creation in the normal file workflow

- [ ] CREATE-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/EditorWorkspace.kt` — visible New function action with open-file identity.
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — file-level creation access when no symbol is selected.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — route all creation actions through the existing draft-discard boundary.
- `desktop/src/main/kotlin/io/miniorca/desktop/AssistantToolWindow.kt` — creation heading, name, behavior prompt and Generate label.
- `desktop/src/main/kotlin/io/miniorca/desktop/CommandPalette.kt` — consistent searchable creation names.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkflowToolWindows.kt` — explicit create-mode scope/status.
- `desktop/src/test/kotlin/io/miniorca/desktop/EditorWorkspaceTest.kt` — action discoverability and eligibility.
- `desktop/src/test/kotlin/io/miniorca/desktop/AssistantToolWindowTest.kt` — creation without a selected symbol.
- `desktop/src/test/kotlin/io/miniorca/desktop/CommandPaletteTest.kt` — matching creation entry.

**Inputs / dependencies**
- UX-01; D4.

**Implementation rules**
- Reuse `requestCreateDeclaration`; opening the form does not generate or mutate anything.
- Focus the required name field first, then the behavior prompt. Name input must stay editable while the target is incomplete.
- Make unsupported-language/no-file states informative. Support an existing valid Go file with zero declarations; no existing symbol selection is required.
- A different active draft needs explicit discard. Cancel preserves it. Retain a secondary type-creation path.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.EditorWorkspaceTest' --tests 'io.miniorca.desktop.AssistantToolWindowTest' --tests 'io.miniorca.desktop.CommandPaletteTest'`

**Execution record**
Not started.

## Task CREATE-02 — Close creation validation and lifecycle gaps

- [ ] CREATE-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/FileChatState.kt` — correct Go identifier/keyword eligibility and clear creation failures.
- `desktop/src/test/kotlin/io/miniorca/desktop/FileChatStateTest.kt` — valid/invalid names and target switching.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — create session, generate, cancel and late-response behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DraftReviewWorkflowTest.kt` — creation through validation/checks/review.
- `internal/app/chat_session_test.go` — daemon creation acceptance/rejection and package-only file fixtures.
- `internal/project/go_declaration_edit_test.go` — append/import preservation and no-write preview regressions.
- `internal/app/draft_lifecycle_test.go` — creation Apply/Undo and stale-file rejection.

**Inputs / dependencies**
- CREATE-01.

**Implementation rules**
- Cover blank names, Go keywords, duplicates, supported Unicode identifiers, malformed source, unsupported files and project/file changes mid-request.
- Retain daemon name validation and exact single-declaration/import composition as the source of truth. Do not invent a second generation endpoint.
- Verify no source write before Apply, unchanged unrelated declarations/imports, and safe rejection if the file changes after generation.

**Verification command**
```sh
./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.FileChatStateTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest' --tests 'io.miniorca.desktop.DraftReviewWorkflowTest'
go test ./internal/project ./internal/app -run 'Test(ComposeGoDeclaration|ValidateGoDeclaration|ChatSession|Draft)' -count=1
```

**Execution record**
Not started.

## Task ANA-01 — Define categorized results and unified run contracts

- [ ] ANA-01 completed with required checks and diff review.

**Target files**
- `internal/project/file_analysis.go` — explicit semantic risk category and cache compatibility.
- `internal/project/findings.go` — preserve classification through finding reconciliation without losing triage identity.
- `internal/project/file_analysis_test.go` — old/new report and category cases.
- `internal/project/findings_test.go` — category, provenance and triage preservation.
- `internal/app/analysis_run.go` — new run/queue/section types and request identities.
- `internal/app/analysis_run_test.go` — new contract/state cases.
- `docs/api-contract.md` — scope, result classification, run states and compatibility decisions.
- `docs/openapi.yaml` — documented unified preview/start/status/control/result shapes.

**Inputs / dependencies**
- D1 and D5; existing `AnalyzeAllJob`, `PerformanceJob`, `SecurityFileReport` and findings contracts.

**Implementation rules**
- Define project-wide run scope and explicit Bugs/Performance/Security category enum. File filters are read-only report queries, not execution scope. Keep source-specific detail types and severity/confidence distinct from category.
- Specify a source-free preview with immutable file identities, exclusions, provider requirements and bounded request/attempt expectations.
- Define per-section and overall states, including completed-empty, partial, failure, unavailable, stale, pause/cancel and interrupted recovery. A section with no successful analysis cannot claim zero findings.
- Old category-less risks remain accessible but unclassified until explicit refresh. Do not infer category from text or discard persisted dismissed/fixed states.

**Verification command**
`go test ./internal/project ./internal/app -run 'Test(FileAnalysis|Finding|AnalysisRun)' -count=1`

**Execution record**
Not started.

## Task ANA-02 — Produce and validate explicit semantic categories

- [ ] ANA-02 completed with required checks and diff review.

**Target files**
- `internal/app/file_analysis.go` — strict category schema/parser/prompt update and new prompt identity.
- `internal/app/file_analysis_test.go` — invalid/missing/new category and category-specific grounding cases.
- `internal/app/file_analysis_evaluation.go` — keep evaluation assessment aligned with the production contract.
- `internal/app/engineering_insight_test.go` — schema identity and optional-insight preservation.
- `internal/insighteval/evaluation_test.go` — offline contract fixtures.
- `internal/insighteval/runner_test.go` — offline runner/schema parity.

**Inputs / dependencies**
- ANA-01.

**Implementation rules**
- Newly generated risks must name their category; only actual correctness, performance or security findings enter the respective section.
- Keep existing source grounding, strict unknown-field rejection, response bounds and optional-field degradation semantics.
- Do not fabricate categories for historical reports. Bump the relevant prompt/cache identity and require explicit refresh to obtain the new classification.
- Update synthetic test fixtures only; never inspect sealed evaluation material or consume provider grants for this task.

**Verification command**
`go test ./internal/app ./internal/insighteval -count=1`

**Execution record**
Not started.

## Task ANA-03 — Compose the per-file analysis stages

- [ ] ANA-03 completed with required checks and diff review.

**Target files**
- `internal/app/analysis_file.go` — new typed stage execution over the existing analyzers.
- `internal/app/analysis_file_test.go` — new partial, cache, cancellation and freshness tests.
- `internal/app/file_analysis.go` — reuse existing semantic execution/publication at the narrow boundary needed by the run.
- `internal/app/performance_review.go` — reuse the existing source review and authorized publication callback.
- `internal/app/security_review.go` — bind advisory execution/publication to admitted run identity and fresh intent.
- `internal/app/security_rules.go` — include passive rules for eligible Go files.
- `internal/app/source_file_snapshot.go` — share only truly identical identity checks.

**Inputs / dependencies**
- ANA-01, ANA-02.

**Implementation rules**
- Execute bounded stages using captured file/revision/hash identities; reuse current parsers and report stores, never call HTTP handlers internally.
- Preserve successful sections when another stage fails. Distinguish reused matching cache evidence from newly requested results; explicit refresh can bypass valid model caches.
- Security keeps deterministic rule matches and AI suggestions separately labeled. Unsupported rules do not suppress an otherwise eligible AI review.
- Check cancellation, source policy, provider consent and run publication authority before dispatch/publication. No tests/vet/benchmarks run here.
- Use stable producer IDs to avoid displaying the same stored finding twice. Do not merge unrelated findings merely because they share a line or title.

**Verification command**
`go test -race ./internal/app -run 'Test(AnalysisFile|AnalyzeFile|ReviewPerformanceFile|ReviewSecurityFile|ScanSecurityFile)' -count=1`

**Execution record**
Not started.

## Task ANA-04 — Implement one durable, bounded run lifecycle

- [ ] ANA-04 completed with required checks and diff review.

**Target files**
- `internal/app/analysis_run.go` — preview/admission, sequential dispatch, pause/resume/cancel and recovery.
- `internal/app/analysis_run_store.go` — new source-free durable run metadata using shared atomic storage.
- `internal/app/analysis_run_test.go` — lifecycle, budget, concurrency and persistence-failure tests.
- `internal/app/analysis_run_store_test.go` — new corruption/interruption/recovery cases.
- `internal/app/service.go` — one coordinator owner and lifecycle wiring.
- `internal/app/project_workspace.go` — restore progress without restarting requests.
- `internal/app/source_file_snapshot.go` — captured-run publication guard where required.

**Inputs / dependencies**
- ANA-03.

**Implementation rules**
- Capture the whole project's deterministic eligible queue with existing context exclusions and per-analyzer source-size limits. File selection must not alter admission.
- Keep explicit batch/time/attempt bounds; reuse existing 100/500 file limits for bounded dispatch batches rather than truncating total project coverage. Calculate expected stage work and retry bounds in preflight rather than multiplying hidden retries. Budget exhaustion retains pending work for explicit continuation and cannot report full completion.
- Save admission before requests and progress before advancing work. Persistence failure stops dispatch and shows recoverable failure without overwriting completed reports or resetting attempts.
- Cancellation stops active requests and further dispatch; pause waits for a defined stage boundary and cannot consume more queued work. Project/source changes mark the run stale.
- Test concurrent start/control, replacement generations, restart, failed writes, provider changes and late completion with deterministic fakes.
- A restored run never restarts itself or carries reusable remote consent. Source-changing Apply invalidates the captured run under existing revision rules.

**Verification command**
`go test -race ./internal/app -run 'Test(AnalysisRun|AnalysisRunStore|ProjectOverview)' -count=1`

**Execution record**
Not started.

## Task ANA-05 — Migrate existing jobs and expose the unified API

- [ ] ANA-05 completed with required checks and diff review.

**Target files**
- `internal/app/analyze_all.go` — remove its independent controller/worker; retain only required public-contract adapters.
- `internal/app/performance_job.go` — migrate scheduling ownership and retain required report/queue compatibility.
- `internal/app/service.go` — remove superseded controller fields and wire compatibility adapters to the unified owner.
- `internal/app/file_analysis_test.go` — migrate Analyze-all lifecycle regressions to the shared owner.
- `internal/app/performance_job_test.go` — retain queue/budget/persistence regression coverage.
- `internal/app/cleanup_contract_test.go` — update ownership assertions while preserving cleanup guarantees.
- `internal/api/handlers/analysis_handler.go` — new strict unified analysis handler.
- `internal/api/handlers/analysis_handler_test.go` — new request/identity/consent/error contracts.
- `internal/api/handlers/project_handler.go` — route legacy entry points through the shared lifecycle.
- `internal/api/handlers/project_handler_test.go` — legacy compatibility cases.
- `cmd/daemon/main.go` — register unified routes through existing loopback policy.
- `cmd/daemon/main_test.go` — route and documentation contract coverage.
- `docs/api-contract.md` — final API/migration semantics.
- `docs/openapi.yaml` — match implemented routes and schemas.

**Inputs / dependencies**
- ANA-04.

**Implementation rules**
- Expose preview, start, current run, report and explicit controls. Strictly validate project/revision/file/queue/provider identity at the boundary.
- Existing Analyze-all and Performance starts become single-purpose adapters to the same owner, preserving their documented scope and limits. Remove obsolete locks/workers and migrated duplicate tests, retaining behavioral regressions.
- Read old persisted jobs without dispatch; preserve attempts and progress when presenting interrupted recovery. Never overwrite historical reports to manufacture combined coverage.
- Keep the project Go scan and benchmark execution APIs separate because they execute code. Preserve loopback/origin rules and sanitized errors.
- If a legacy response cannot faithfully project the new lifecycle, record the exact incompatibility and migration before removing it; do not silently change public behavior.

**Verification command**
`go test -race ./internal/app ./internal/api/handlers ./cmd/daemon -count=1`

**Execution record**
Not started.

## Task ANA-06 — Give the desktop one analysis owner

- [ ] ANA-06 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/Models.kt` — unified wire types.
- `desktop/src/main/kotlin/io/miniorca/desktop/ApiClient.kt` — unified API methods.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — one analysis run and typed section evidence.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflow.kt` — new cohesive request/control owner.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt` — delegate analysis commands and remove superseded request logic.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopJobCoordinator.kt` — one analysis poller, retaining the independent verified-scan poller.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopSecurityWorkflow.kt` — remove duplicate run ownership; retain only independently required security operations.
- `desktop/src/test/kotlin/io/miniorca/desktop/ApiClientContractTest.kt` — unified and compatibility payloads.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAnalysisWorkflowTest.kt` — new run ownership/consent/late-result tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — migrated analysis lifecycle cases.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopSecurityWorkflowTest.kt` — preserve fresh intent and retained-evidence assertions.

**Inputs / dependencies**
- ANA-05.

**Implementation rules**
- The daemon owns job scheduling; desktop only admits actions, polls and renders authoritative state. Switching result sections must not trigger work.
- Preserve typed Security evidence and benchmark ownership, file/project identity guards and cancellation cleanup.
- One admission UI names all affected providers/scopes and Security intent. Consume/reset consent at the defined attempt boundary; no silent reuse after restart or changed scope.
- Refresh and reconnect read reports; partial errors remain associated with the correct section/file and do not replace unrelated valid results.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ApiClientContractTest' --tests 'io.miniorca.desktop.DesktopAnalysisWorkflowTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest' --tests 'io.miniorca.desktop.DesktopSecurityWorkflowTest'`

**Execution record**
Not started.

## Task ANA-07 — Separate run progress from the three result pages

- [ ] ANA-07 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/AnalysisWorkspaceState.kt` — project coverage, per-analyzer progress and result-page links.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — Analysis start/progress surface and Bugs results page.
- `desktop/src/main/kotlin/io/miniorca/desktop/FindingsPresentation.kt` — readable result rows and visible severity/provenance.
- `desktop/src/main/kotlin/io/miniorca/desktop/BugsWorkspaceState.kt` — retain triage/filter presentation within Bugs section.
- `desktop/src/main/kotlin/io/miniorca/desktop/PerformanceWorkspace.kt` — section content, hypotheses and explicit measurement handoff.
- `desktop/src/main/kotlin/io/miniorca/desktop/SecurityWorkspace.kt` — section content retaining rule/AI distinctions.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — wire unified actions and exact finding-to-editor handoffs.
- `desktop/src/main/kotlin/io/miniorca/desktop/ContextToolWindow.kt` — Analyze project and local View this file's results actions.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — real counts/coverage/filter/state cases.
- `desktop/src/test/kotlin/io/miniorca/desktop/BugsWorkspaceStateTest.kt` — triage retained.
- `desktop/src/test/kotlin/io/miniorca/desktop/PerformanceWorkspaceTest.kt` — hypothesis/measurement distinction.
- `desktop/src/test/kotlin/io/miniorca/desktop/SecurityWorkspaceTest.kt` — partial/source-aware results.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — all three sections and their important states.

**Inputs / dependencies**
- UX-02, ANA-06.

**Implementation rules**
- Analysis shows Start, overall/per-analyzer progress, current file, coverage, pause/resume/cancel and operational failures only. Its Bugs/Performance/Security progress rows link to the corresponding result page; no findings list belongs on Analysis.
- Keep results in their existing distinct workspaces, supplied by the unified run. Their file filters never start another run. Prefer a list/detail layout that becomes a single-column drill-down on narrow windows.
- Each result shows title, explicit severity/state, concise content and source location before secondary evidence. Filters are local and never re-run analysis.
- Do not put general suggestions in Bugs; preserve them in the file explanation. Historical unclassified risks are readable under previous-analysis detail with no fresh count.
- Preserve existing triage, Prepare fix, exact-symbol eligibility and benchmark trust behavior. Opening a finding must not generate a draft.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.BugsWorkspaceStateTest' --tests 'io.miniorca.desktop.PerformanceWorkspaceTest' --tests 'io.miniorca.desktop.SecurityWorkspaceTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

**Execution record**
Not started.

## Task NAV-01 — Distinguish run, results and editing in the sidebar

- [ ] NAV-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — retain distinct workspaces and local per-page result filters.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt` — preserve saved navigation and pane sizes.
- `desktop/src/main/kotlin/io/miniorca/desktop/IdeShell.kt` — grouped Results destinations with labeled count/state and selected accents.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — connect progress-page links and result destinations.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopKeyboardNavigation.kt` — preserve existing workspace shortcuts and focus behavior.
- `desktop/src/main/kotlin/io/miniorca/desktop/CommandPalette.kt` — consistent Start analysis and result-page actions.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — project-wide Start routing and local result navigation.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopStatusBar.kt` — reflect actual run/provider context.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopLayoutStateTest.kt` — preference preservation without side effects.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopShellTest.kt` — new workspace ownership.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — navigation/focus coverage.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopIntegrationCoverageTest.kt` — section-to-editor navigation.

**Inputs / dependencies**
- ANA-07.

**Implementation rules**
- Preserve separate Analysis, Bugs, Performance and Security rail entries as requested. Make their purposes obvious: Analysis runs/tracks; the three Results destinations display findings.
- Group the result entries with restrained spacing/separators, clear labels and real count/state badges. Do not add a second navigation system or turn category tint into severity.
- Preserve Cmd/Ctrl+4 for Editor and existing Summary/Analysis/Bugs/Performance/Security navigation. Saved selections remain valid; keep pane widths and no-network-on-navigation behavior, with independent focus/selected accents.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopLayoutStateTest' --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopIntegrationCoverageTest'`

**Execution record**
Not started.

## Task TERM-01 — Prove the terminal dependency and local-session boundary

- [ ] TERM-01 completed with required checks and diff review.

**Target files**
- `desktop/build.gradle.kts` — pin verified terminal/PTY dependencies and required package modules.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTerminalSession.kt` — new local shell/PTY owner independent of Compose recomposition.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopTerminalSessionTest.kt` — new lifecycle tests with injected process boundary.
- `desktop/TERMINAL.md` — new short dependency, native packaging and supported-host record.

**Inputs / dependencies**
- D2; existing JBR 25/macOS arm64 distribution constraints.

**Implementation rules**
- Evaluate JediTerm/Pty4J with the pinned desktop toolchain; choose exact available artifacts and record notices/native requirements before integrating the UI.
- Prove process startup, working directory, UTF-8, terminal resize, interrupt and bounded teardown on the supported host. Use an isolated temporary directory for smoke commands.
- Launch the user's supported shell as an executable plus arguments, never interpolate the project path into a shell command string. Set project cwd explicitly.
- Define Idle/Starting/Running/Exited/Failed/Closed state, a bounded scrollback policy and process cleanup. No automatic shell start during project restore.
- Missing shell/native library/unavailable local project path gives a visible error and retry action. Do not claim a full terminal from a fake-only test.

**Verification command**
```sh
./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopTerminalSessionTest'
./scripts/desktop-gradle.sh createDistributable
```
Also record a real packaged-host PTY smoke before TERM-02; dependency/native failure is an explicit implementation dependency, not permission to substitute an output box.

**Execution record**
Not started.

## Task TERM-02 — Integrate the interactive terminal pane

- [ ] TERM-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/TerminalToolWindow.kt` — new themed terminal host and explicit session controls.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopTerminalSession.kt` — component attach/detach and resize lifecycle.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — stable project-bound session owner/disposal.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt` — read-only freshness refresh when returning from the terminal.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt` — invalidate selected-file evidence when its observed hash changes.
- `desktop/src/main/kotlin/io/miniorca/desktop/Main.kt` — deterministic shutdown integration where needed.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopKeyboardNavigation.kt` — terminal focus/toggle and key routing.
- `desktop/src/test/kotlin/io/miniorca/desktop/TerminalToolWindowTest.kt` — new session/control behavior tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopTerminalSessionTest.kt` — hide/show/switch/close process behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopKeyboardNavigationTest.kt` — focus-aware shortcut behavior.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopWorkflowPresenterTest.kt` — shell-originated file changes reject stale review/analysis evidence.

**Inputs / dependencies**
- UX-01, TERM-01.

**Implementation rules**
- Match source typography/colors; support input, output, ANSI colors, cursor movement, history, copy/paste, Ctrl+C, resize and scrolling through the terminal library.
- First activation starts one shell; collapsing or switching workspaces preserves it. Closing explicitly ends it; reopening starts a new session.
- On project switch with an active session, let the user cancel the switch or explicitly close that session. Never silently redirect a running shell into the new project.
- Terminal focus owns shell keystrokes, including interrupt; app-wide shortcuts must not steal ordinary terminal input. Provide a deliberate way back to app focus.
- Dispose streams/native resources/processes on restart/project close/app exit, with bounded waiting and visible launch/exit errors. Do not persist terminal transcript or automatically send it to a model.
- On return to source/review, recheck the selected file through the existing read-only file-info boundary; a changed hash makes draft/check/analysis evidence stale. Offer explicit Reindex for changed project inventory. Do not parse terminal output to infer changes or auto-run an analysis after shell commands.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.TerminalToolWindowTest' --tests 'io.miniorca.desktop.DesktopTerminalSessionTest' --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' --tests 'io.miniorca.desktop.DesktopWorkflowPresenterTest'`
Native verification additionally covers a prompt, history, interruption, full-screen terminal redraw, focus return, resize and app-exit cleanup.

**Execution record**
Not started.

## Task BOTTOM-01 — Preserve unique diagnostics in their owning workflows

- [ ] BOTTOM-01 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/BottomEvidenceToolWindows.kt` — identify/move uniquely used diagnostic formatting before deletion.
- `desktop/src/main/kotlin/io/miniorca/desktop/ReviewEvidencePane.kt` — all candidate check output and validation errors under Review.
- `desktop/src/main/kotlin/io/miniorca/desktop/WorkspacePanes.kt` — analysis run failures in Analysis progress details; verified-scan output in Bugs details.
- `desktop/src/main/kotlin/io/miniorca/desktop/AssistantToolWindow.kt` — generation failures beside the request.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopStatusBar.kt` — retain actionable connection/global operation detail access.
- `desktop/src/main/kotlin/io/miniorca/desktop/DiagnosticText.kt` — new shared sanitizer only if it remains used by multiple owners.
- `desktop/src/test/kotlin/io/miniorca/desktop/ReviewEvidencePaneTest.kt` — failed/stale/skipped checks and output access.
- `desktop/src/test/kotlin/io/miniorca/desktop/AnalysisWorkspaceStateTest.kt` — analysis failure visibility.
- `desktop/src/test/kotlin/io/miniorca/desktop/BugsWorkspaceStateTest.kt` — verified-scan output access.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopStatusBarTest.kt` — global failure access.

**Inputs / dependencies**
- UX-02, ANA-07.

**Implementation rules**
- Inventory the existing Output entries: generation/validation, selected-file failure, Analyze-all failures, scan phase command/output, daemon errors and latest operation.
- Give each piece one reachable owner; reuse diagnostic sanitization/bounds and preserve Copy/detail access. No new global Output view.
- Keep automated check output distinct from the terminal, where it did not execute. Terminal content does not become validation evidence.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.ReviewEvidencePaneTest' --tests 'io.miniorca.desktop.AnalysisWorkspaceStateTest' --tests 'io.miniorca.desktop.BugsWorkspaceStateTest' --tests 'io.miniorca.desktop.DesktopStatusBarTest'`

**Execution record**
Not started.

## Task BOTTOM-02 — Replace the bottom tools with Terminal only

- [ ] BOTTOM-02 completed with required checks and diff review.

**Target files**
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt` — terminal-only preference model and legacy bottom selection fallback.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopShell.kt` — terminal dock, collapse control and bounded narrow overlay.
- `desktop/src/main/kotlin/io/miniorca/desktop/DesktopApp.kt` — remove Problems/Checks/Output construction and connect terminal.
- `desktop/src/main/kotlin/io/miniorca/desktop/BottomEvidenceToolWindows.kt` — delete after useful shared code is moved.
- `desktop/src/main/kotlin/io/miniorca/desktop/ProblemsToolWindow.kt` — delete if exclusively used by the removed bottom surface; move any remaining shared finding UI first.
- `desktop/src/test/kotlin/io/miniorca/desktop/BottomEvidenceToolWindowsTest.kt` — remove obsolete surface tests after moving meaningful assertions.
- `desktop/src/test/kotlin/io/miniorca/desktop/ProblemsToolWindowTest.kt` — move shared finding assertions or remove obsolete tests.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopLayoutStateTest.kt` — old bottom preferences restore collapsed Terminal.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — terminal wide/narrow/collapsed/failed states.

**Inputs / dependencies**
- NAV-01, TERM-02, BOTTOM-01.

**Implementation rules**
- Remove all three obsolete bottom tabs, counts, automatic-opening callbacks and associated dead types; preserve only the terminal control and the separate integrated status bar.
- Keep resizable docked height and the <1000dp bounded overlay. Restore pane dimensions without auto-starting a shell or reviving an old bottom selection.
- Collapsing/opening must not recreate the process. Returning from drawers/palette restores focus predictably.

**Verification command**
`./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopLayoutStateTest' --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopVisualLayoutTest'`

**Execution record**
Not started.

## Task VERIFY-01 — Validate the complete interaction and document support

- [ ] VERIFY-01 completed with required checks and diff review.

**Target files**
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopIntegrationCoverageTest.kt` — project-wide analysis, separate result pages/file filters and creation handoffs.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopVisualLayoutTest.kt` — final production-component fixture matrix.
- `desktop/src/test/kotlin/io/miniorca/desktop/DesktopAccessibilityTest.kt` — names, states, focus and read-only semantics.
- `desktop/src/test/kotlin/io/miniorca/desktop/IdeUiContractBaselineTest.kt` — update the baseline to actual supported controls.
- `desktop/KEYBOARD_SMOKE_CHECKLIST.md` — new navigation/terminal procedures.
- `desktop/UI_CONTRAST.md` — measured final text/state/focus contrast.
- `desktop/README.md` — actual Analysis, creation and terminal usage/shortcuts.
- `README.md` — supported user workflow and migrations.
- `docs/insights-performance/README.md` — new section ownership and preserved evidence distinctions.
- `docs/RELEASE_ACCEPTANCE.md` — actual automated/native results and limitations.
- `PLAN.md` — task status, accepted decisions and outstanding dependencies.

**Inputs / dependencies**
- CREATE-02, BOTTOM-02 and every preceding analysis/UI task.

**Implementation rules**
- Review the aggregate diff for obsolete controllers/surfaces, accidental duplicate rules, hidden actions, unhandled failures and unrelated edits. Update dependent existing tests rather than keeping obsolete UI behavior alive.
- Native visual matrix: wide, 1000/999dp, 800×650, 1280×600, 125/150% text; long content/paths/errors; each result state and each terminal state. Verify important information is distinguishable without depending on color alone.
- End-to-end with a fake provider/temp project: partial three-section run, cancel/resume/restart, source change, new function in a package-only file, draft preservation, Review/Apply/Undo and terminal-triggered evidence invalidation. No live provider is needed.
- Verify terminal packaging on the supported macOS arm64/JBR 25 host, local cwd, keyboard routing, PTY resize and process cleanup. Record any untested host separately; do not broaden the accepted platform claim from compilation alone.
- Document report/prompt identity and visual-preference migrations. No model configuration rename is assumed. Keep historical release/qualification limits intact.

**Verification command**
```sh
./scripts/validate.sh
./scripts/desktop-gradle.sh createDistributable
git diff --check
```
`scripts/validate.sh` covers the formatting, Go, race, vet, desktop and dispatcher
checks represented by `make check`, plus static and daemon-contract gates. Do not
repeat equivalent full suites once they pass unless subsequent changes justify it.
Use the existing production-component reproduction command in
`docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks` and record native smoke
separately. Report executed/cached/skipped/unavailable checks accurately.

**Execution record**
Not started.
