# Mini-Orca improvement plan

Prepared 2026-09-06 against `7147ab7`, including the existing uncommitted Task 170
evidence. **Implementation status: planned.** This change analyzes the project and
consolidates documentation; it does not implement the backlog or authorize commits.

This is the single product plan and task ledger. Task instructions are below;
[tasks/README.md](tasks/README.md) defines agent execution and automation. Everyday
documentation stays short; this plan is the deliberate exception.

## Product decision

Build a focused engineering IDE for an experienced developer: understand a real
code path, investigate a bug, assess a performance or security concern, and make
one reviewable change. The useful unit of work is a function and its necessary
context, not a repository-wide chat conversation.

The daily loop should be:

```text
Select file/function → choose intent → optional short instruction
    → inspect evidence or a candidate → validate/check → review → explicit Apply
                                                          ↘ learn the mechanism
```

An explanation or investigation ends with evidence; it does not need to create a
draft. A change ends with one named declaration in one existing file, plus only
its required imports. When a request needs several changes, explain the boundary
and propose the first independently useful step. Do not quietly broaden the edit.

Initial priorities: remove distractions, make the existing workflow obvious,
clear the quality debt, harden the execution boundaries, then add Security and
better engineering evidence. Keep Go as the first language with exact editing.
Supporting analysis of other languages is useful current behavior, not legacy
compatibility that should be deleted without evidence.

## What the repository actually contains

This is a source and documentation audit, plus automated checks, not a fresh
native-window review or a full security penetration test. Both supplied images
and three saved production-component captures were inspected. The initial tracked
inventory has 250 files, including 44 Markdown
files / 4,092 Markdown lines; Go and Kotlin counts include tests and fixtures.

| Area | Observed implementation | Consequence for this plan |
| --- | --- | --- |
| Daemon | Go 1.22 module, one external module (`yaml.v3`); `cmd/daemon` → HTTP handlers → `internal/app` → `internal/project`, `llm`, `storage` | Keep these boundaries. No framework or daemon rewrite. |
| Desktop | Compose 1.11.0, Kotlin 2.3.20, Gradle wrapper 9.1.0, Jewel standalone `0.40.0-262.10315.125`; JBR 25 runtime | Jewel migration is delivered. Improve the current components, do not start another toolkit migration. |
| Mutation safety | Immutable chat target, editable declaration draft, candidate composition, revision/hash checks, validation, temporary checks, guarded Apply/Undo | These are product assets. Remove duplication around them, never the guards. |
| Analysis | Deterministic index, explicit Go scans, file analysis, bounded Analyze-all, finding provenance and freshness | Reuse for navigation and evidence. Navigation must not trigger analysis. |
| Performance | Separate reports/jobs; source hypotheses with workload, trade-off and verification plan | Already useful, but no runtime measurements or demonstrated speedups. |
| Learning | Optional four-field `EngineeringInsight`, 1,000-rune bound, parent freshness; collapsed desktop panel | Improve content and presentation instead of adding a course platform. The panel currently concatenates the fields into one paragraph. |
| UI | Five workspaces, shared theme/chrome, flat headers, docked panes/drawers, production render fixtures | Remaining design work is task clarity, distraction removal, native verification and specific observed defects. |
| Complexity | Presenter 1,080 lines; shell 1,121; Performance job 705; Analyze-all 615 | Inspect responsibilities and state ownership. Line counts alone do not justify splitting files. |
| Legacy | Flat model fallback, browser UI, obsolete routes/report writers and old theme were previously removed | No evidence for a second legacy application to delete. Focus on concrete redundant artifacts and branches. |
| Contract duplication | Routes appear in mux, tests, `cleanup-baseline-routes.json`, API prose and OpenAPI; two identical safe model-metadata structs | Remove redundant inventories/types with their tests in FND-05, retaining behavior coverage. |
| Provider boundary | One non-streaming Chat Completions client; default HTTP redirect behavior; `providerStatusError` parses a body it never uses | Review redirects against consent; remove the useless decode without exposing provider bodies. |
| Execution boundary | `runCheckCommand` uses `CombinedOutput`, truncates after capture; a temporary project copy is used for checks | Output capture is not bounded in memory. A copy is not an OS sandbox and child code inherits host capabilities. |
| Network boundary | Loopback daemon default; no Host/Origin guard visible in mux; Docker examples publish `9090:9090` | Harden browser-to-loopback access and host port defaults. These observations require tests, not invented exploit claims. |
| Delivery | Local wrapper and quality script; no `.github` workflow in this checkout | Establish reproducible gates before unattended implementation. |

Fresh audit checks: `go test ./...` passed; `./desktop/gradlew -p desktop test`
succeeded with all tasks up to date. `make quality` passed staticcheck/deadcode,
then failed at `gocyclo -over 15`:

| Function | Complexity | Owner |
| --- | ---: | --- |
| `(*Service).reviewPerformanceFile` | 19 | FND-02 |
| `validPerformanceFinding` | 18 | FND-02 |
| `validPerformanceJob` | 19 | FND-03 |
| `(*Service).StartPerformanceJob` | 18 | FND-03 |
| `(*Service).StartAnalyzeAll` | 16 | FND-04 |

Clone detection and Desktop static checks did not run through that failed target.
Race, vet, packaging, live providers and native accessibility were not rerun for
this documentation change. Historical passes are not fresh acceptance.

Saved fixture observations refine the UI priorities below. These captures were
already present under ignored `desktop/build/reports/ui-precision/`; they are not
fresh renders or native screenshots:

| Capture | Concrete observation | Owner |
| --- | --- | --- |
| `task-169/editor-1000.png` | Files title appears twice; Preview menus, Generate unit test preview and Terminal preview occupy several regions around a narrow source pane | UI-01 removes inert surfaces; UI-02 reconciles header ownership and pane balance |
| `task-170/analysis-1280-600-150-1x.png` | Pause/Cancel remain reachable at 150% text, but run limits repeat above and inside the closed Advanced options summary | UI-02 removes redundant copy while retaining state and limits |
| `task-169/review-ready-800-1.3.png` | Focused checks and Ready to apply headings/states repeat; Run focused checks is visually stronger than the available Apply action | UI-03 makes the next valid action primary and compresses repeated evidence |

After documentation edits, `go test ./cmd/daemon ./internal/config -count=1`
passed, rechecking the retained API/version/configuration contracts.

## Unfinished work carried forward

Superseded does not mean passed. The new backlog replaces the old execution order
and per-task commit requirement; it preserves the unfinished behavioral checks.

| Earlier work | Actual state | New owner / disposition |
| --- | --- | --- |
| Tasks 01–139 | Implemented; some manual release follow-ups outstanding | Preserve existing behavior; provider/content follow-ups go to LEARN-02 and REL-01. |
| Tasks 140–148 and 150–169 | Implemented, including Jewel and flat pane migrations | Remove completed task instructions; recover detail from Git when needed. |
| Task 149 | Superseded; its attainable native visual/regression checks are closed by UI-04 | REL-01 retains final release checks. No second dark-theme implementation. |
| Task 170 | Superseded; its component matrix plus attainable native popup, OS focus and reader checks are closed by UI-04 | Unsupported combinations and retained evidence are linked from [acceptance](docs/RELEASE_ACCEPTANCE.md). |
| Task 171 | Pending; its historical Task 170 dependency is resolved by UI-04 | Runtime/package verification → REL-01; final cleanup → REL-02; quality debt → FND-02–04. Old commit-ledger policy is retired. |
| Insights/Performance manual follow-ups | Remote queue consent, lifecycle UX and real insight usefulness unverified | LEARN-02 and REL-01. Do not recreate insights/jobs. |
| Earlier container acceptance | Image build/health not recorded as passed | REL-01, only for the distribution support actually claimed. |
| Detekt/JBR workaround | JDK 21 launcher for Detekt; JBR 25 app/toolchain; JVM 22 bytecode | Keep the working combination documented in `desktop/README.md`. Change only after a verified compatible release. |

UI-04 records the material native evidence available on the supported host and lists
unsupported combinations explicitly. Those release limitations remain visible in
the acceptance ledger and do not block an independent backend task with passing
relevant tests. An agent must not claim the whole plan complete while a required
release check is unavailable.

## UI and interaction specification

The [dark reference](design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png)
owns color, proportions, the source-first composition and evidence layout. The
[Aurora reference](design/ui-mocks/Gemini_Generated_Image_54u1al54u1al54u1.jpeg)
contributes tab clarity and grouped context. Its light palette is not a second
theme. [Dark UI direction](docs/dark-ui/README.md) and
[desktop guidelines](desktop/UI_DESIGN_GUIDELINES.md) define the maintained rules.

- Keep the current charcoal semantic palette, blue selection, distinct focus,
  square panes and one 1dp separator per boundary. Preserve larger invisible
  splitter hit areas. Keep Jewel and app-owned source/diff rendering.
- Use the current 88dp labeled rail; 256dp Files and 344dp inspector preferences;
  220dp initial bottom pane. These are defaults, not fixed widths. Maintain the
  360dp source floor at the 1000dp docked boundary and restore preferred widths
  after temporary clamping. Below 1000dp use Files/Context drawers.
- Keep 12–13sp body / 18–20sp line height, 11–12sp secondary chrome, 24–28dp rows,
  28–32dp pane headers and a 4dp spacing grid. Grow for text scaling. Essential
  controls and errors must wrap or remain reachable; do not solve overflow by
  shrinking text. The main application toolbar can remain taller.
- Keep Summary, Analysis, Bugs, Performance and Editor. Security is a real sixth
  workspace only when its backend exists. Preserve numbered shortcuts 1–4 and
  contextual command-palette/composer routing; add Security through the palette
  and workspace cycling without stealing a shortcut.
- Show Files and the right inspector in Editor, not around every project report.
  Summary answers “what is this project?”; Analysis owns runs; Bugs/Security own
  triage; Performance owns hypotheses/evidence; Editor owns the current change.
- Make the selected function and next valid action obvious. Use compact intent
  actions: Explain, Find bugs, Improve performance, Review security, Change.
  Cached evidence opens locally. Fresh model work always has an explicit request
  with destination/context preview. A preset prepares a request, never sends it.
- Remove nonfunctional Terminal, Run/Debug, extra tabs, minimap, branch switching,
  scores, feedback and account/settings previews from the normal product. Their
  illustrative presence in a mock is not a reason to maintain controls and tests.
  Retain actual branch information and useful Help/connection details.
- Source and composed diff stay selectable/read-only. The isolated draft is the
  only editor. Review shows one compact scope/evidence summary, one next action,
  a concrete blocked reason when needed, and expandable technical diagnostics.
  Apply remains labeled with its file/function and requires fresh checks.
- Empty, loading, failed, canceled, partial, unavailable and stale are distinct
  textual states. Unknown counts are not zero; daemon health is not model health.
  No fabricated performance/security score or sample project data in real results.

Reference adaptation: keep native window controls and one active source file;
use explicit Review before Apply; no multi-file helper extraction from the mock.
Native titlebar replacement is deferred unless actual usability evidence warrants
the platform cost. Record before/after production captures for changes, not new
mock applications or screenshot hashes as tests.

## Engineering learning and evidence

Target an engineer with seven years of experience. Explain mechanisms, constraints
and failure modes in the selected code: cancellation and ownership, lock scope,
allocation lifetime, query fan-out, backpressure, idempotency, error boundaries,
cache invalidation, authorization and trust transitions. Skip syntax tutorials,
generic SOLID advice, praise and explanations for trivial edits.

Use the existing optional insight fields as four brief pieces: **Mechanism**,
**Why here**, **Trade-off**, **Try/verify elsewhere**. Remain collapsed by default
and under the existing total bound. Source/owner evidence supplies location and
freshness; model prose never acquires mutation authority. Example:

> Mechanism: the lock spans a network wait. Why here: every caller shares the same
> mutex, so one slow request serializes unrelated work. Trade-off: moving the call
> outside the lock needs a second state check before publishing. Verify with a
> delayed fake upstream and a contention profile under concurrent calls.

Performance must distinguish an observed source pattern, a workload-dependent
hypothesis, a proposed experiment and an actual measured result. Security must
distinguish a static rule match, a model suspicion and a verified reproduction.
Neither a passing test nor a clean scan proves general correctness or security.

## Architecture and scope rules for implementation

1. Keep one current API and current metadata schema per feature. Mini-Orca need
   not load retired formats. Recomputable old caches become unavailable with an
   explicit refresh path; never silently reinterpret stale data as current.
2. Apply/Undo receipts and backups are safety state, not disposable caches.
   Unknown/corrupt state disables mutation with a useful error. No task silently
   deletes an imported project's metadata, user configuration, source or backups.
3. Retain intentional boundaries: model-wire vs domain validation, source-context
   traversal vs check-workspace copying, and Go exact editing vs conservative
   other-language analysis. Superficial similarity is not duplication.
4. Reuse the three configured model scopes: `function` for declaration requests
   and explicit explanations; `bug` for existing bug analysis; `analyze` for
   Performance/Security review. Do not introduce another model registry.
5. New reports bind project/revision, file hash, exact scope, policy fingerprint,
   model/provider/reasoning identity and prompt version. Recheck before publishing
   a response. Navigation/filters/disclosures never send a model request.
6. Security v1 is passive analysis of one selected file, optionally focused on one
   function. No network attack scanner, autonomous exploit execution, project-wide
   security job clone or promise of complete taint analysis.
7. Checks execute project-controlled code. A temporary copy protects source from
   ordinary command writes; it does not isolate the host. Bound resources and show
   an explicit trust decision. Do not advertise sandboxing without an OS boundary.
8. Keep generated artifacts ignored and outside maintained docs. Use Git for
   completed history, tests for enforceable contracts, and concise docs for usage.

## Task ledger

Only this table owns status. All tasks start Pending. A dependency is satisfied
only by a reviewed, validated result present in the task's base. “Implemented” is
not “Complete.” Use Pending → Running → Review → Complete, or Blocked with an
exact reason. Blocks affect dependents, not the entire queue.

Effort S means one narrow boundary; M means one cohesive feature slice. These are
scope estimates, not time promises. If a slice proves larger, split it in this
ledger before implementation. The default is one writer, even for ready tasks.

| ID | Deliverable | Dependencies | Effort / risk | Status |
| --- | --- | --- | --- | --- |
| AUTO-00 | Bootstrap unattended plan execution | — | S / low | Complete |
| FND-01 | Reconcile baseline and preserve working evidence | AUTO-00 | S / low | Complete |
| FND-02 | Simplify Performance review/validation | FND-01 | S / medium | Complete |
| FND-03 | Simplify Performance job admission/validation | FND-02 | M / high | Complete |
| FND-04 | Simplify Analyze-all admission | FND-01 | S / high | Complete |
| FND-05 | Remove proven contract and code redundancy | FND-01 | M / medium | Complete |
| FND-06 | Isolate desktop job coordination | FND-03, FND-04 | M / high | Complete |
| AUTO-01 | Reproducible validation entry point and CI | FND-02, FND-03, FND-04, FND-05 | M / medium | Complete |
| UI-01 | Remove unsupported product previews | FND-01 | M / medium | Complete |
| UI-02 | Correct source-first layout against captures | UI-01 | M / medium | Complete |
| UI-03 | Make draft/review progression explicit | UI-02 | M / high | Complete |
| UI-04 | Close inherited native/accessibility checks | UI-03 | M / high | Complete |
| FLOW-01 | Short function-scoped change requests | UI-03 | M / medium | Complete |
| FLOW-02 | Read-only declaration explanation | FLOW-01, SEC-02, FND-06 | M / high | Complete |
| FLOW-03 | Reusable temporary behavioral proof | FLOW-01, SEC-03, SEC-04 | M / high | Complete |
| LEARN-01 | Concise, grounded engineering insights | FLOW-02 | S / medium | Complete |
| LEARN-02 | Evaluate usefulness and model reliability | LEARN-01, FLOW-03 | M / medium | Complete |
| SEC-01 | Harden daemon and container network boundary | FND-01 | M / high | Complete |
| SEC-02 | Bind provider delivery to consent and context | FND-01 | M / high | Complete |
| SEC-03 | Bound subprocess output and workspace resources | FND-01 | M / high | Complete |
| SEC-04 | Explicit project-execution trust and environment | SEC-03 | M / high | Complete |
| SEC-05 | Define strict Security report contracts | FND-05 | S / medium | Complete |
| SEC-06 | Add small deterministic Go security rules | SEC-05 | M / medium | Complete |
| SEC-07 | Add explicit AI Security review | SEC-05, SEC-02, FND-03 | M / high | Complete |
| SEC-08 | Deliver Security workspace and focused handoff | SEC-06, SEC-07, UI-03, FND-06 | M / high | Complete |
| PERF-01 | Measure and improve Mini-Orca hotspots | FND-03, FND-06, SEC-03 | M / medium | Complete |
| PERF-02 | Compare an explicitly selected Go benchmark | SEC-04, FLOW-03, PERF-01 | M / high | Complete |
| PERF-03 | Present measured evidence beside hypotheses | PERF-02, UI-03 | M / medium | Complete |
| AUTO-02 | Bounded agent dispatcher with review gate | AUTO-01 | M / high | Complete |
| QUAL-01 | Realistic development/qualification corpus and rubric | LEARN-02 | S / medium | Complete |
| QUAL-02 | Strict repeated-run receipts and qualification gates | QUAL-01 | M / high | Complete |
| QUAL-03 | Resumable provider-budgeted evaluation runner | QUAL-02 | M / high | Complete |
| QUAL-04 | Grounded insights and intentional omission | QUAL-01, QUAL-03 | S / medium | Complete |
| QUAL-05 | Six-request development pilot and candidate freeze | QUAL-04 | S / high | Pending |
| QUAL-06 | Independent 24-request qualification verdict | QUAL-05 | M / high | Pending |
| REL-01 | End-to-end, native and distribution acceptance | AUTO-01, UI-04, LEARN-02, SEC-01, SEC-08, PERF-03, QUAL-06 | M / high | Blocked |
| REL-02 | Final code/doc retirement and handoff | REL-01, AUTO-02 | S / medium | Pending |

Recommended first delivery: FND-01–06, AUTO-01, UI-01–04, FLOW-01, SEC-01–04.
This yields a smaller, clearer and better bounded existing app before new breadth.
Second delivery: explanation/learning and Security. Third: measured Performance
and dispatcher. Each delivery uses the REL-01 checklist for its included scope; the final ledger
row completes only after its listed dependencies and all required features pass. PERF-02/03 may be
explicitly deferred as a pair if measurements do not justify their cost; never
silently mark deferred work Complete. REL-02 is final-plan acceptance.

## Shared definition of done

Read `AGENTS.md`, the relevant section here, the nearest production code and tests;
read the design guidelines before desktop edits. Entry points below are a starting
map, not permission to rewrite every named file. A task must:

- deliver its behavior and negative cases; remove replaced helpers/call sites;
- keep current project/file/draft identity and remote consent intact;
- add meaningful deterministic tests for changed behavior, using temporary roots
  and fake providers; no live accounts in default tests;
- run focused checks, then the applicable gate below; review the whole diff;
- update only affected usage/contracts and this ledger with actual evidence;
- hand off changed behavior, results, remaining limitations and configuration impact.

| Gate | Commands / evidence |
| --- | --- |
| G — daemon | `go fmt ./...`, `make fmt-check`, `go test ./...`, `make test-race`, `make vet` |
| D — desktop | `./desktop/gradlew -p desktop spotlessCheck detekt test`, with the documented JDK 21 launcher / JBR 25 toolchain |
| C — contract | G plus Desktop `ApiClientContractTest`, route/OpenAPI tests, and both API documents updated for actual API changes |
| V — visual | Production-component renders, checked text/bounds/semantics and native inspection for affected interactions; UI-04 matrix |
| Q — integration | `make check`, `make quality`, `git diff --check`; report any failed stage and stages not run |

FND-02/03/04 may proceed with only the remaining named complexity findings; each
must eliminate its own finding without changing the threshold. AUTO-01 requires
the full quality gate green. No inherited quality exception applies to final release.

## Foundation task instructions

### FND-01 — Reconcile baseline and preserve working evidence

- Inspect current diff/HEAD, [acceptance](docs/RELEASE_ACCEPTANCE.md), the original
  Task 170 test/evidence and the two reference images. Record which changes are
  pre-existing before assigning ownership. No broad staging or reset.
- Capture a minimal current production fixture set: Editor, Analysis, populated
  Review and Performance at 1440×900 and 999×760. Label source, runtime and scale.
- Update the acceptance record with reproducible check results only when needed
  for a changed base; do not rerun every historical migration.
- Accept when the starting behavior, actual failures and UI evidence limitations
  are reproducible, links resolve, and every inherited open item has an owner.
- Verify: relevant existing visual tests, documentation links, `git diff --check`.
  No production behavior or configuration change expected.
- Completed 2026-09-06 from baseline `d0a7cc9`: eight offscreen production-component
  fixtures cover Editor, Analysis, populated Review and populated Performance at
  1440×900 and 999×760. A fresh review found no issues. The coordinator passed
  Spotless, Detekt, the full Desktop tests, a forced 36-test visual/accessibility/
  keyboard run, link validation and `git diff --check`. The then-unavailable native
  window, popup, OS-focus and screen-reader evidence was assigned to UI-04 and is
  closed within its recorded supported-host limits below.

### FND-02 — Simplify Performance review and finding validation

- Start in `internal/app/performance_review.go`, `internal/project/performance.go`
  and their existing tests. Separate eligibility/snapshot preparation, model call,
  output validation and publication guards only where those phases are real.
- Split finding field/enum/anchor validation into clear domain checks. Preserve
  source limits, optional insight omission and successful-empty semantics.
- Accept: changed source/policy, cancellation and invalid anchors cannot publish a
  current report; mixed-valid results retain their warning; complexity ≤15.
- Verify: focused Performance tests, G and the Go quality stages. No API/schema change.
- Completed 2026-09-06. Performance review preparation, model execution,
  validation and guarded publication are distinct; stale source or policy cache
  entries become stale derived coverage without rewriting the completed durable
  job. The desktop accepts a report only when its nonblank project, revision and
  queue identity matches the active job, so a polling mismatch cannot show another
  queue's findings or paths. Fresh review accepted the change. Focused and full Go
  tests, race tests, vet, the desktop test suite and diff check passed; the global
  quality gate still reports complexity owned by FND-03 and FND-04.

### FND-03 — Simplify Performance job admission and persisted validation

- Start in `internal/app/performance_job.go` and `performance_job_test.go`.
  Separate request validation, compatible queue reconstruction and starting work.
  Keep a single owner of controller locks and publication authorization.
- Cover pause/resume/cancel, restart interruption, policy/provider/source changes,
  file/time budgets and attempts that cannot reset on resume. Use an injected clock
  if needed for deterministic elapsed-time tests, not sleeps.
- Accept: stale/canceled workers cannot overwrite the active job or publish to a
  switched project; both named complexity failures are removed. Keep the current
  job API and distinct Performance cache namespace.
- Verify: focused lifecycle tests, G, Go quality stages. No generic job engine.
- Completed 2026-09-06. Queue preparation, identity validation and final admission
  are separate and revalidated under lifecycle locking. Performance workers and
  persistence are bound to the full captured job identity and canonical project
  root; project replacement detaches and stales old active work without exposing it
  to the next project. Restart recovery returns interrupted files to pending while
  retaining attempts and charging elapsed time once. Fresh review accepted the
  lifecycle, delayed-write, cross-root-session and restart regressions. `make check`,
  `make test-race`, `make vet`, `make quality` and diff check passed.

### FND-04 — Simplify Analyze-all admission

- Start in `internal/app/analyze_all.go`, its lifecycle tests and `file_analysis.go`.
  Separate eligible selection, limits/confirmation and controller activation.
- Accept: empty/excluded selection, concurrent start, project switch, exhausted
  retry budget and canceled results preserve current behavior; complexity ≤15.
- Verify: Analyze-all/file-analysis tests, G, Go quality stages. Do not unify it
  with Performance simply because both process queues.
- Completed 2026-09-06. Analyze-all separates start preparation, candidate
  selection and lifecycle admission; activation and reindex/project replacement
  serialize reciprocal Analyze-all and Performance admission. The persistence writer
  snapshots the current controller state after acquiring its turn, so an older
  snapshot cannot resurrect stale work. Workers are bound to their job instance,
  preventing an old worker from changing or clearing a replacement job. Fresh
  review accepted deterministic stale-snapshot and replacement-worker regressions.
  Formatting, full Go/race/vet and diff checks passed; global quality now reports
  only complexity owned by FND-03.

### FND-05 — Remove proven contract and code redundancy

- Trace `EffectiveModel`/`ScopedModel` consumers in `internal/app/service.go` and
  `providerStatusError` in `internal/llm/client.go`; consolidate identical metadata
  ownership and remove the unused body decode while preserving sanitized errors.
- In `cmd/daemon/main_test.go`, replace the historical cleanup-route JSON ledger
  with tests of actual mux behavior against the maintained OpenAPI/API contract.
  Delete `docs/cleanup-baseline-routes.json` only in this tested change.
- Audit persistence/config readers for actual old-schema branches. Remove only
  proven retired compatibility; keep unknown/corrupt current-state failure paths,
  context exclusions, Apply/Undo protection and provider API-prefix support.
- Accept: no unused replacement remains, current APIs serialize identically, no
  live consumer loses a route, and malformed provider bodies never appear in errors.
- Verify: C, staticcheck/deadcode, `go mod tidy -diff`, links. Record a short
  removal ledger in the task result, not a new permanent baseline document.
- Completed 2026-09-06. Removed the duplicate Go `ScopedModel` type and conversion,
  the unused provider-error body decode, the hand-maintained route/consumer lists
  and `docs/cleanup-baseline-routes.json`. The maintained OpenAPI and API contract
  now agree and every documented route is exercised through the daemon mux. A fresh
  review found no issues. The coordinator passed C, full Desktop tests,
  staticcheck, deadcode, `go mod tidy -diff`, link validation and `git diff --check`.
  The existing complexity failures remain owned by FND-02, FND-03 and FND-04.

### FND-06 — Isolate desktop job coordination

- Start in `DesktopWorkflowPresenter.kt`, `AnalysisWorkspaceState.kt`,
  `PerformanceWorkspace.kt` and presenter/controller tests (desktop package).
- Extract only asynchronous job polling/lifetime coordination from the presenter
  into a cohesive desktop collaborator. Keep the authoritative workflow snapshot
  and project/file generation guards in one place; no generic state framework.
- Accept: one poll per active job; inactive/terminal jobs stop; project change and
  close cancel work; late responses cannot replace new state; dispatcher remains
  injectable. Preserve different Analysis/Performance lifecycle rules.
- Verify: deterministic fake-API presenter tests, cancellation/order cases and D.
- Completed 2026-09-07. `DesktopJobCoordinator` owns polling and cancellation for
  Analyze-all, Performance and verified scans while the presenter remains the
  single authority for workflow publication. Immutable project/job action targets
  and generation guards prevent stale requests, polls, workspace refreshes,
  reports and findings from changing newer state. Fresh reviews covered replacement,
  cancellation, failure recovery, project-close and delayed-response ordering; the
  final review accepted the task. Repeated presenter tests, the JDK 21/JBR 25
  Spotless/Detekt/full-test gate and `git diff --check` passed.

## UI task instructions

### UI-01 — Remove unsupported product previews

- Trace `PreviewFeature.kt` through `DesktopHeader.kt`, `EditorWorkspace.kt`,
  `ContextToolWindow.kt`, `BottomEvidenceToolWindows.kt`, `DesktopStatusBar.kt`,
  `IdeShell.kt` and their tests. Remove nonfunctional destinations, state, labels,
  callbacks and preview-only icons/helpers that become unused.
- Keep actual command search, branch display, model destinations, output, checks
  and useful help. Do not remove a real capability because its old mock was inert.
- Accept: no dead Terminal/Run/Debug/tab/minimap/score/account UI in the normal
  workflow; no blank gaps; only live actions in the palette; no new network calls.
- Verify: Preview/command/shell tests adjusted to real behavior, D and V. Document
  removed visible previews; no source, API or stored draft migration.
- Completed 2026-09-07. Removed the toolbar, editor, Context and bottom-pane preview
  controls: New file, branch actions, content search, settings/account/help, feedback,
  extra tabs/split editor, minimap, Run/Debug, generated unit tests, fabricated scores
  and Terminal. The real command palette, branch display, provider destinations,
  output, checks and Performance scope preview remain. Retired Terminal preferences
  fall back to Problems. Visual fixtures cover the retained live toolbar and the
  removed controls; command and layout tests cover the live palette and preference
  fallback. Independent diff review, `git diff --check`, and the pinned JDK 21/JBR 25
  `./scripts/validate.sh` gate passed. No source, API or stored-draft migration.

### UI-02 — Correct source-first layout against captures

- Inspect `DesktopShell.kt`, `IdeShell.kt`, `DesktopHeader.kt`, `ChromeControls.kt`,
  `ExplorerPane.kt`, `SourceEditorPane.kt` and shared theme. Compare FND-01 captures
  with the images; list concrete differences before editing.
- Fix pane balance, duplicated borders, clipped actions and inconsistent spacing
  at the shared owner. Keep header-owned actions and current palette. Use the
  specified widths/typography as logical defaults, not absolute pixel copies.
- Remove duplicate Files headings and repeated collapsed run-limit summaries
  observed in the saved fixtures, retaining one clear owner for each value.
- Accept: source has usable space at 1000dp; 999dp drawers retain target and focus;
  wider layout restores preferences; long paths remain discoverable; short windows
  retain the next action without overlapping source. Record deliberate adaptations.
- Verify: layout, shell, explorer and visual tests; D and V. No new theme/toolkit.
- Adaptation: preserve the 999dp drawer boundary and saved pane widths; reduce only the default
  dock preferences and raise the rendered source minimum to 400dp at the 1000dp docked boundary.
  The Explorer owns its wide Files header, the right tabs own their content identity, and narrow
  pane headers move trailing actions below their title when inline space is insufficient.

### UI-03 — Make draft and review progression explicit

- Start in `AssistantToolWindow.kt`, `ReviewEvidencePane.kt`, `EditorWorkspace.kt`,
  `DraftEditorState.kt`, `DesktopState.kt` and review tests. Render progression from
  existing authoritative states: request → draft → validate → checks → review.
- Show one primary next action and its scope. Put raw hashes, command details and
  long diagnostics behind accessible disclosures. Keep failed-check details easy
  to open; do not collapse the evidence required to make an Apply decision.
- In ready Review, make the guarded named Apply visually primary; keep rerun checks
  secondary. Remove repeated Focused checks/Ready to apply headings and state copy.
- Accept: draft edits immediately invalidate old evidence; stale or failed required
  checks visibly block Apply; discard names both scopes; Apply/Undo retain their
  exact identity guards and no new automatic step occurs.
- Verify: draft/review integration and keyboard tests, D and V. No separate UI
  eligibility formula or shortcut around daemon validation.
- Completed 2026-09-07. The Review and Assistant surfaces now present Request,
  Draft, Validation, Focused checks and Review in order from existing authoritative
  state. A single named action remains primary only when its existing guard allows
  it; technical disclosures expose labeled candidate/check hashes, commands and
  full diagnostics without obscuring failure evidence. Fresh review accepted stale,
  failed and ready states, including the bound-request happy-path fixture. Focused
  visual/accessibility/keyboard tests and the full Desktop suite passed. Native
  window, popup, OS-focus and screen-reader acceptance was completed by UI-04.

### UI-04 — Close inherited native and accessibility checks

- Use the actual JBR desktop and disposable release fixture. Reuse the retained
  Task 170 component evidence without calling it native evidence.
- Exercise 1440×900, 1920×1080, 1000×760, 999×760, 800×650 and 1280×600; check
  100/125/150% text and 1×/2× density where supported. Include empty/loading/error/
  stale/populated, long paths, drawers, splitters, dialogs and edge-positioned menus.
- Verify keyboard-only traversal, source/diff selection, one-layer Escape,
  focus restoration, resizing and retained preferences. Run VoiceOver or another
  supported reader; record OS/runtime/reader and observed names/states.
- Accept: mock hierarchy and all material native checks evidenced; unsupported
  combinations explicitly listed. Missing native access leaves this task Blocked.
- Verify: D, V and the current keyboard checklist. Update the single acceptance
  ledger; identify which remaining 149/170 requirements this closes.
- Completed 2026-09-07 on macOS 26.6.2 with the exact pinned JBR, a disposable
  project and loopback daemon. Native checks at 1000×760, 999×760, 800×650 and
  1280×600 covered wide/compact panes, menus, drawers, source selection, one-layer
  Escape, focus restoration, splitter resizing and retained preferences. They found
  and fixed rail selection plus palette/bottom-overlay focus restoration. VoiceOver
  10 was enabled and its output service was active while the packaged app's native
  accessibility tree exposed named controls and selected/error state. The
  full `make check` gate and a forced 24 visual, 8 accessibility and 7 keyboard run
  passed; 291 Desktop tests passed in the final full run. The external Q2789 was
  detected at 2560×1440, but the automation surface could not bind the Gradle-run
  window placed there or relocate its packaged fixture across the negative-origin
  display boundary. Exact native 1440×900/1920×1080, 125%/150% text, alternate
  density, loading/stale/generated-diff/provider/guarded-Review states and a speech
  transcript remain explicitly unclaimed. Deterministic production-component tests
  cover those UI states and scales. This closes the attainable UI-04 requirements
  inherited from Tasks 149/170; release/provider/package work remains with REL-01.

## Function workflow and learning task instructions

### FLOW-01 — Short function-scoped change requests

- Start in `EditorContextualActions.kt`, `DirectEditState.kt`, `FileChatState.kt`,
  `AssistantToolWindow.kt` and existing chat/declaration prompt boundaries.
- Add small presets for bug fix, performance improvement and behavior change.
  Selection supplies path/symbol/context; the user supplies only intent such as
  “preserve order when deduplicating” or “return a typed error for a missing user.”
  Keep advanced constraints in a disclosure; don't require a long prompt template.
- Accept: preset click only prepares/focuses the bound composer; explicit Send
  performs one request with current consent; create-declaration still requires an
  absent valid name; unsupported/multi-function scope gives an actionable boundary.
- Verify: preset/no-call tests, target/discard/stale cases, D; G/C only if a daemon
  contract changes. Prefer the current chat API over new intent endpoints.
- Completed 2026-09-07. Exact Go function and method targets now offer compact bug,
  performance and behavior presets. A preset prepares and focuses the existing
  composer without a provider call; optional constraints stay in a collapsed
  disclosure, and explicit Send uses the existing chat request and Function consent.
  Target changes and new ad hoc flows synchronously clear stale prepared text and
  hidden task metadata, while genuine prepared findings retain their bound task.
  Create validation and actionable invalid-target boundaries remain intact. A fresh
  review accepted the final repair; the forced Desktop gate passed 279 tests with
  zero failures and Detekt reported zero findings. No daemon contract changed.

### FLOW-02 — Explain a declaration without producing a draft

- Reuse `function_context.go`, policy/token limits and model routing. Add one
  explicit read-only declaration-explanation operation in app/handlers/API and
  wire its result through the desktop presenter and Context pane.
- Request binds project/revision, file/hash, exact declaration and confirmation;
  response carries source anchors, concise explanation, optional insight and
  provenance. Use `function` scope; fresh requests need their own consent. Keep
  results in current session state initially; no new chat-history database.
- Accept: opening cached content does no work; requesting an explanation creates
  no draft/check/Apply state; changed selection, cancellation, malformed output,
  excluded context and provider errors are handled explicitly.
- Verify: strict contract/fake-provider no-mutation tests, C, D. Proposed route:
  `POST /api/projects/current/files/explanation`; finalize its exact schema in
  OpenAPI in this task, without adding unrelated review modes.
- Completed 2026-09-07. The explicit explanation route binds current project,
  file hash and one exact policy-eligible Go declaration, uses the Function model
  with per-request remote consent, and returns bounded structured explanation,
  source anchors, optional insight and context provenance. Pre-send and post-model
  identity/policy checks prevent stale context delivery or publication; malformed,
  canceled, unsupported and provider-failed requests create no session, draft,
  check, source write or history. Context keeps cached explanations local and adds
  labeled Explain, Refresh and Cancel states with late-result guards. Fresh review
  accepted the final contract repair. Gates G, C and D passed with the documented
  Java 21 launcher/JBR 25 toolchain; wide and responsive production fixtures were
  rendered and inspected. No configuration or persistence migration is required.

### FLOW-03 — Reuse temporary behavioral proof for changes

- Inspect `bug_task_spec.go`, `chat_session.go`, `draft_checks.go` and draft-check
  UI. Reuse the current bounded temporary Go test proposal for a function change,
  without requiring that every useful test originate in a bug finding.
- Present test intent/content before execution. The test lives only in the copied
  workspace. For replacements, distinguish a meaningful base failure/candidate
  pass from compilation failure. New declarations may have an absent-symbol base;
  label that limitation instead of claiming a reproduced behavioral defect.
- Accept: malformed tests, mismatched targets, changed drafts, timeouts and failed
  candidate checks block their applicable path; repairs remain explicit and bounded
  at three. No source test file, sibling change or auto-repair loop is introduced.
- Verify: base/candidate fixtures, target/revision/repair guards, G/C and D. Document
  test execution trust and any exact contract extension.
- Completed 2026-09-07. Strict test validation, copied-workspace execution,
  package-scoped trust, truncation, create-mode limits and generic proof
  inheritance are covered. A repair now adopts a reviewed optional proof from
  its exact failed parent into the session and next draft; full task/proof
  equality then prevents later repairs from dropping or replacing it before a
  provider request. The desktop retains the adopted proof for review and trust
  presentation. Focused repair/UI tests and `make check` passed G, C and D. The
  quality runner passed static analysis, reachability, clone detection and
  Desktop checks, but its existing complexity gate still reports
  `checkWorkspaceCopier.copy` and `LoadPerformanceFileReport`; neither function
  was changed by FLOW-03. No API shape, configuration or persistence migration
  is required.

### LEARN-01 — Improve engineering insight content and presentation

- Start in `engineering_insight.go`, `declaration_prompt.go`, `file_analysis.go`,
  `performance_review.go`, `EngineeringInsightPanel.kt` and their tests.
- Preserve the existing four-field schema and total 1,000-rune bound. Render each
  nonempty field as a compact labeled piece instead of one concatenated paragraph.
  Prompts request a concrete mechanism, local evidence, trade-off and verification
  idea; omit low-value or trivial lessons.
- Accept: optional malformed insight cannot reject valid parent data; absent fields
  create no empty rows; stale ownership is visible; draft edits clear old lessons;
  disclosure/navigation never contacts a provider or changes evidence.
- Verify: Unicode/limits/malformed/omission fixtures, meaningful senior examples,
  G when prompts/parser change, D and narrow/150% V.
- Completed 2026-09-07. All producers now share concise guidance for a concrete
  mechanism, local evidence, a real trade-off and a reusable verification idea;
  file, project and Performance prompt versions invalidate prior cached prose.
  Optional malformed insights are omitted without rejecting valid parent data,
  whitespace is normalized before the aggregate 1,000-Unicode-rune limit, and the
  desktop renders only nonempty labeled pieces with owner and stale state. Draft
  edits still clear prior insights and disclosure remains local presentation state.
  Fresh review accepted the result after two repairs. G, D and focused narrow/150%
  visual checks passed; no API, schema or manual migration is required.

### LEARN-02 — Evaluate usefulness and model reliability

- Extend relevant testdata with a small representative set: cancellation/locking,
  allocation, N+1 I/O, idempotency, authorization, trivial edit, no-finding and
  malformed/overconfident output. Keep cases beside their owning tests.
- Automated assertions check structure, boundedness, source anchors and omission.
  A separately invoked live evaluation scores correctness, local relevance,
  trade-off clarity and useful verification (0–2 each); require no critical false
  claim and at least 6/8 for retained examples before accepting prompt changes.
- Accept: model/prompt version and sample count recorded, bad outputs preserved as
  sanitized fixtures, no claim that format tests prove teaching quality. Include
  counterexamples that should return nothing and independently inspect the labels.
- Verify: deterministic suite in G/D; a manual live sample with an explicitly chosen
  model/provider and budget. Carry unrun live quality evidence into REL-01.
- Completed 2026-09-07. Eight sanitized fixtures cover the required useful,
  no-insight and malformed cases; deterministic Go checks enforce structure,
  source anchors, the 1,000-rune bound and omission, while Desktop tests inspect
  the labels independently. An opt-in receipt validator records the selected
  provider, model, prompt version, sample count and request/token budgets, then
  enforces human scores, critical-false-claim rejection and the 6/8 retention
  threshold without contacting a provider. Two repair/review cycles removed new
  quality findings and added command-boundary coverage. `make check` passed; the
  quality runner has only the existing pending-Security reachability output and
  complexity findings in `checkWorkspaceCopier.copy` and
  `LoadPerformanceFileReport`. No live sample was run; REL-01 retains that evidence
  requirement until a user selects the provider, model and budget.

## Security task instructions

### SEC-01 — Harden daemon and container access

- Start in `cmd/daemon/main.go`, request decoder/handlers, Dockerfile, Compose,
  Makefile, `DOCKER.md` and HTTP tests. Write a brief threat boundary in the API
  guide: local process trust, browser-origin threats and explicit external binding.
- Bind published host ports to loopback by default in maintained deployment paths.
  Add centralized Host/Origin and JSON content-type policy for the local API;
  reject cross-origin browser mutations and unexpected hosts before handler effects.
  Do not use permissive CORS as authentication or break the native client.
- Accept: forged Host, hostile Origin, preflight, non-JSON body and allowed desktop
  calls have deterministic tests; health checks still work; externally reachable
  deployment remains explicit and documented, never described as authenticated.
- Verify: G/C, container config checks and disposable health smoke when available.
  Document exact new request/deployment requirements; no credential committed.
- Completed 2026-09-07. The daemon now applies a single outer local-API policy:
  loopback Host validation by default, browser Origin/preflight rejection, and JSON
  media-type checks for known or streamed mutation bodies. Documented bodyless job
  controls remain compatible. Maintained Docker/Compose publications bind to host
  loopback; explicit external binding is documented as unauthenticated. Fresh review
  accepted hostile-host/origin, bodyless native, non-JSON and chunked-body behavior.
  Full validation, race, vet, formatting and diff checks passed. Docker image/health
  smoke remains unavailable because this host has no usable Compose-capable daemon.

### SEC-02 — Bind provider delivery to consent and safe context

- Start in `internal/llm/client.go`, model confirmation, context policy/builders
  and source-output parsers. Make redirect policy explicit: reject prompt-bearing
  redirects by default, especially local→remote or origin changes, without silently
  retrying at another destination or forwarding credentials.
- Test project text/comments containing instructions to change scope or reveal
  secrets. Treat them as data; strict anchors/target validation remain authoritative.
  Check excluded paths, symlinks, oversized context and late publication.
- Accept: declined consent or redirect sends no prompt to the second destination;
  errors/logs expose no provider body/key; model output cannot broaden the mutation.
- Verify: two-server `httptest` redirect fixtures including 307/308, request counters,
  exclusion/provenance tests and G. Document redirect behavior as a deliberate change.
- Completed 2026-09-07. Prompt-bearing requests now perform one provider exchange
  and reject every 3xx response, including malformed or missing redirect locations,
  with a status-only error and no retry, second-destination request or credential/body
  forwarding. Remote consent still gates each request. Context targets must pass the
  project policy; bounded inventory and snippets keep exact inclusion, exclusion and
  truncation provenance while retaining the selected target. Injection fixtures prove
  source instructions cannot retarget a draft or mutate source. Fresh review accepted
  the repaired implementation; formatting, full Go tests, race tests, vet and diff
  checks passed. The documented redirect behavior requires no configuration migration.

### SEC-03 — Bound command output and copied-workspace resources

- Start at `runCheckCommand`, `copyCheckWorkspace`, Go scans and all callers.
  Capture output into a bounded writer while draining streams; avoid collecting
  unlimited bytes before truncation. Keep exit, cancellation and truncation distinct.
- Add file/byte limits and cancellation checks during workspace copying; omit VCS
  internals and known build caches without dropping required project test fixtures.
  Preserve symlink exclusion and cleanup on every failure path.
- Accept: huge output stays memory-bounded, cancellation cannot hang on output,
  excessive copies fail clearly, useful diagnostics are redacted, and imported
  source remains unchanged. Define one owner for limits instead of per-call values.
- Verify: deterministic helper-process output/timeout/exit tests and temp-root copy
  tests, G. Do not claim process isolation or silently turn incomplete copies into
  valid check evidence.
- Completed 2026-09-07. Draft checks and Go scans now share an 8 KiB draining
  output collector plus one copied-workspace budget of 4,096 regular files and
  64 MiB. Copying checks cancellation during traversal and reads, excludes VCS,
  Mini-Orca and known build metadata, preserves testdata fixtures, rejects changed
  or nonregular source identities and removes every incomplete workspace. Async
  copy failures publish a sanitized `workspace` phase; API and Desktop wording now
  describes temporary copied workspaces without implying process isolation.
  Authorization variants are fully redacted. Fresh review accepted the result
  after two repairs; G, C, D and full diff checks passed. The added workspace
  phase needs no configuration or data migration.

### SEC-04 — Explicit project-code execution trust

- Reuse the shared check boundary for scans, lint/tests and temporary task tests.
  Distinguish source-only operations from commands that execute project code.
  Show the exact command scope and require an explicit project/session trust action
  before those commands run; changing the project resets that authorization.
- Build a deliberate child environment that excludes provider credentials and
  unnecessary inherited secrets; retain the documented toolchain essentials.
  Own cancellation of descendants on supported platforms, with platform-specific
  helpers only where required. Fail clearly if a claimed capability is unavailable.
- Accept: no project code runs on import/navigation; denial runs no process;
  trust is separate from remote-model consent; canceled children are accounted for.
  The UI explicitly says trusted local execution, not sandboxed execution.
- Verify: environment sentinel and child-process fixtures, scope/consent API and UI
  tests, G/C/D. An OS sandbox is a separate future capability, not implied here.
- Completed 2026-09-07. Project-code checks now require explicit in-memory trust
  for the active project revision and recheck that revision immediately before
  each `go test` process starts. The Desktop shows the exact scan or generated
  task-test argv, verifies it against the daemon scope, and keeps source-only
  draft checks free of execution consent. Child processes receive an allowlisted
  toolchain environment; supported Unix systems own and cancel the whole process
  group, while unsupported platforms reject project-code execution clearly.
  Project replacement and reindex clear trust, independently of remote-provider
  consent. Fresh review accepted the result after two repairs; G, C and D passed,
  including provider-secret, descendant-cancellation and cross-compile fixtures.
  No configuration or data migration is required, and no OS sandbox is implied.

### SEC-05 — Define the Security evidence contract

- Add a small domain report beside `internal/project/performance.go`/findings.
  Fields: rule/category, title, source anchor, severity, confidence, evidence kind,
  observed condition, preconditions/unknowns, remediation and verification idea;
  optional CWE/reference only when known. Reuse insight and owner provenance.
- Keep stable finding identity independent of model wording/insight. Define report
  states (not run, running, completed-empty, partial, stale, failed, canceled) and
  triage separately from verification. Store no raw secrets or full prompt/source.
- Accept: strict bounded decoding, valid file/symbol anchors, successful-empty,
  partial rejection and corrupt-cache handling are specified and tested. Keep
  Security persistence separate from bug/Performance reconciliation.
- Verify: project schema/parse/persistence fixtures and G. No visible workspace or
  new generic finding framework before there is a producer.
- Completed 2026-09-07. Security now has a separate source-bound report,
  finding identity, lifecycle, provenance, anchor and cache contract for future
  deterministic and AI producers. Strict decoding rejects malformed, oversized,
  mixed-invalid and duplicate results; persistence distinguishes empty, partial,
  stale, failed and canceled states while keeping triage separate from verification.
  Before storage, prose and provenance redact quoted JSON credentials,
  provider-prefixed token/key names, secret-access-key variants, escaped quoted
  values and colon/equals Authorization forms. Raw-cache tests cover those forms,
  and identity fields reject secret-like assignments. G passed, including the
  complete race suite; the descendant-cancellation fixture now waits for observed
  child startup instead of assuming it occurs within 30 ms. SEC-05 adds no new
  complexity finding. The quality runner still stops on the two existing untouched
  findings in `checkWorkspaceCopier.copy` and `LoadPerformanceFileReport`. No API,
  UI, configuration or persistence migration is introduced.

### SEC-06 — Add a small deterministic Go rule set

- Implement source-only AST checks for two concrete patterns: explicit TLS
  verification disabling and dynamic shell-command construction. Inspect alias
  imports, scope/shadowing and literals; do not classify lookalike names as calls.
- Report “rule match / review required” with the exact source condition and unknown
  reachability. A syntactic match is not proof of externally exploitable behavior.
  Limit v1 to one selected eligible Go file; unsupported languages say unavailable.
- Accept: positive, negative, test-fixture, alias, shadowing and unrelated-symbol
  cases; no provider/process call; excluded/symlink paths stay excluded.
- Verify: rule fixtures, G/C if exposing the source-only scan operation. Add only
  the needed endpoint and keep mutation routes unchanged.
- Completed 2026-09-07. The explicit Security scan endpoint applies two bounded,
  source-only Go rules to one current policy-eligible file: literal TLS verification
  disabling and dynamic shell command construction. Findings are source-ordered,
  capped at five, labeled as syntactic review rather than exploit proof, cached in
  the separate source-free Security report, and never contact a provider or run
  project code. Import aliases, dot imports, lexical scopes, local type and generic
  parameter shadowing, shadowed predeclared booleans, static literals, lookalikes,
  test files, stale revisions, exclusions and symlinks have regression coverage.
  G/C passed, including the race suite and full Desktop tests. The quality runner
  reports only the two accepted baseline complexities in `checkWorkspaceCopier.copy`
  and `LoadPerformanceFileReport`; SEC-06 adds no quality finding. No configuration
  or persistence migration is required.

### SEC-07 — Add explicit AI Security review

- Reuse policy/context/confirmation/runtime boundaries from Performance, with
  `analyze` scope and a distinct Security prompt/report. Review one selected file
  (optionally one exact declaration), at most 64 KiB source/output and five findings.
  No new project-wide queue controller in v1.
- Prompt for attacker-controlled input, trust boundary, observed operation,
  assumptions, remediation and a safe verification plan; allow no findings.
  Model suggestions remain advisory even beside deterministic results.
- Accept: canceled/stale project/file/provider/policy results cannot publish;
  malformed/over-limit anchors fail correctly; prompt injection cannot retarget;
  absence of findings is not labeled “secure.” Never execute suggested exploit code.
- Verify: fake-provider request/provenance/limits tests, G/C. Put exact routes and
  schemas in OpenAPI/API guide in this task, not another prose contract file.
- Completed 2026-09-08. The explicit Security review route sends one current,
  policy-eligible file and optionally one exact declaration through the configured
  Analyze scope after fresh remote-provider consent. Project, revision, file hash,
  policy, focus and provider identity are rechecked around model delivery and
  immediately before atomic cache publication; canceled or stale results leave no
  report. Strict bounded parsing allows an empty finding list without calling the
  file secure and rejects malformed, out-of-scope or over-limit responses. Returned
  and persisted reports are redacted and source-free: meaningful source-line token
  sequences are rejected even when the provider changes their spacing or
  punctuation. Fake-provider, provenance, limits, atomic-publication, API/OpenAPI
  and source-echo regression tests passed with G/C, including the race suite and
  Desktop tests. The quality runner reports only the two accepted baseline
  complexities. No configuration or persistence migration is required.

### SEC-08 — Deliver Security workspace and one-function handoff

- Extend desktop models/client, workspace state, palette/rail, presenter and
  finding presentation with a real Security workspace. Reuse row/detail and
  evidence primitives where behavior matches; keep independent report state.
- Show source rule matches separately from AI suggestions, filters and triage,
  coverage limits, stale state and explicit Scan/Review actions. Open source at
  its real anchor. Prepare fix only for an exact supported declaration.
- Accept: entry/filter/selection causes no scan; remote review requires consent;
  Prepare fix opens the existing composer without sending; no multi-file patch,
  score or unsupported capability claim. Numbered shortcuts remain unchanged.
- Verify: client/presenter/interaction tests, D/C and V at wide/999dp/150% text.
- Completed 2026-09-08. The desktop now exposes a real Security workspace with
  explicit local Go Scan and advisory AI Review actions, separate source/AI report
  state, local filters, coverage and lifecycle labels, and validated source anchors.
  Remote Review consumes its own one-request Analyze confirmation. Prepare fix
  accepts only a current exact Go declaration, opens and focuses the existing
  Assistant composer, and never sends or writes automatically. Scan and Review
  responses must match project, revision, file hash and their expected evidence
  source before publication, so they cannot overwrite one another. Client,
  presenter, interaction, layout and wide/999dp/150% visual fixtures passed with D
  and `make check`; `make quality` reports only the two accepted baseline Go
  complexities. No configuration, API or persistence migration is required.

## Performance task instructions

### PERF-01 — Measure Mini-Orca before optimizing it

- Profile the app's own index/traversal, copied check workspace, file navigation,
  source rendering and active-job polling on deterministic small/large fixtures.
  Record workload, hardware/runtime, warm/cold conditions and repetitions.
- Add focused Go benchmarks or desktop timing instrumentation only at an observed
  hot boundary. Investigate full-workspace copies, source rendering work and
  redundant refreshes; do not assume all are bottlenecks from source size alone.
- Accept: baseline and after results are comparable; fix the largest demonstrated
  issue within one boundary, or report no justified change. Set regression budgets
  from measurements, not invented latency claims or timing-flaky unit tests.
- Verify: behavioral tests plus measured benchmark/capture, G or D as affected.
  Keep timing artifacts ignored and report the measurement uncertainty.
- Completed 2026-09-07. On macOS 26.6.2, Apple M5 Pro/48 GiB, Go 1.26.1
  (`GOMAXPROCS=18`) and JBR 25.0.4.1, warm deterministic probes measured a
  2,000×4 KiB traversal at 1.91 ms, warm index rebuild at 71.47 ms, copied
  check workspace at 207.18 ms/69.93 MB allocated, 20k-line/2k-symbol source-row
  preparation at 151.65 ms, 1,000 in-memory polling updates at 0.462 ms and file
  navigation below microsecond clock resolution. Five three-iteration comparable
  copy runs justified reusing one per-copy 32 KiB buffer: allocation fell to
  4.43 MB/op (93.7%) while median time remained 206.62 ms, within filesystem
  noise. Source pages were warm and destinations new; cold disk, Compose paint
  and HTTP polling were not measured, so no latency improvement is claimed. The
  retained benchmark, focused behavior tests, fresh review and G passed; no timing
  artifact, configuration change or migration is required.

### PERF-02 — Compare one explicitly chosen existing Go benchmark

- Build on the bounded trusted execution and exact draft-check workspace. Offer
  selection of an existing benchmark in the affected Go package; the application
  constructs fixed argv with an escaped exact benchmark filter, `-run '^$'`,
  bounded repetition/time and `-benchmem`. Never execute a model-authored shell.
- Run base and candidate in separate copies using the same toolchain, fixture and
  parameters. Capture at least five samples per side by default, limits permitting;
  cancel both safely. Measurement remains optional and never enables Apply itself.
- Accept: no benchmark/invalid candidate/untrusted project yields unavailable;
  baseline changes invalidate comparison; noisy/failed runs cannot claim a win;
  tested project source and its benchmarks are not written by this operation.
- Verify: helper/fake-runner parsing and budget tests plus one disposable real
  benchmark, G/C. No profiler UI, benchmark generator or general command console.
- Completed 2026-09-08. The benchmark catalog lists existing exact Go benchmark
  functions in the affected package without executing code and binds each fixed,
  escaped argv preview to the current draft and bounded workspace fingerprint.
  After revision-bound local execution consent, comparison runs five base and five
  candidate samples in separate disposable copies with the same captured fixture,
  toolchain environment and limits. Current project, draft, source and benchmark
  identity are rechecked between phases. Canceled, failed, truncated, malformed or
  incomplete runs return no samples and make no performance claim; strict parsing
  rejects invalid CPU suffixes, non-finite or negative metrics, duplicate metrics
  and invalid sample counts. The operation never changes Apply eligibility or the
  project source. Fake-runner, budget, parser, API/OpenAPI and one disposable real
  benchmark test passed with G/C, including the race suite and Desktop tests. The
  quality runner reports only the two accepted baseline complexities. No
  configuration or persistence migration is required.

### PERF-03 — Show measurements without overstating conclusions

- Extend `PerformanceWorkspace.kt` and review evidence with a compact comparison
  for PERF-02: workload, base/candidate identity, samples, elapsed test conditions,
  ns/op, B/op, allocs/op where available, and variability/inconclusive state.
- Keep existing source findings labeled Not measured until there is compatible
  evidence; tie each experiment to the selected benchmark and candidate rather
  than declaring the entire project faster. Explain trade-offs through insights.
- Accept: stale, noisy, missing and contradictory measurements are textual; a CPU
  improvement cannot hide higher memory use; viewing evidence starts no process.
- Verify: parsing/presentation fixtures and stale identity tests, D and V. No score
  dashboard and no automatic benchmark after an edit.
- Completed 2026-09-08. The Performance workspace now loads a read-only benchmark
  catalog for the exact validated draft, requires an explicit benchmark choice and
  execution consent, and presents the daemon-built argv before running. Retained
  evidence is bound to the draft, workload, benchmark and scope, with five samples
  per side, test conditions, ns/op, B/op and allocs/op where available. Missing,
  stale, noisy, contradictory and zero-baseline results remain textual and
  inconclusive; CPU improvement cannot conceal a memory regression, and source
  findings remain Not measured. Cancellation and generation guards prevent late
  catalog or comparison results from restoring invalid state. A fresh independent
  review accepted the result. Focused state, presenter, API-contract and rendering
  tests, the full Desktop suite and `make check` passed; Spotless and Detekt passed.
  `make quality` reports only the two accepted baseline Go complexities. No
  configuration or persistence migration is required.

## Automation and release task instructions

### AUTO-00 — Bootstrap unattended plan execution

- Create a dedicated local `codex/autopilot` branch containing the reviewed plan
  baseline, including the preserved Task 170 work. Keep the user's source branch
  available at its original commit; never push or merge the autopilot branch.
- Create one recurring Codex heartbeat for this task. Each run resumes an existing
  Running/Review task or selects the first ready Pending task, then performs at
  most one task with a GPT-5.6 Terra writer, a fresh reviewer and coordinator-run
  validation. The coordinator alone updates this ledger and creates local commits.
- The user's 2026-09-06 request authorizes local task commits on
  `codex/autopilot` after acceptance. It does not authorize pushes, releases,
  deployment, host cleanup, changes outside the repository or merges to another
  branch. Pause for those actions and for decisions that materially change scope.
- Keep successful scheduled runs quiet. Report a blocker only when no independent
  ready task can proceed, and report when release acceptance is ready for the user.
- Accept when the branch exists, the plan baseline is committed, the recurring
  heartbeat is active and its saved prompt contains the dependency, review,
  validation, retry and integration boundaries above. Verify by viewing the saved
  automation and checking the clean branch identity.
- Installed evidence: the active `mini-orca-autopilot` heartbeat runs every 30
  minutes with failed-run-only notifications. This plan baseline is its first local
  commit; the original `V5-Desktop-improve` branch remains at `7147ab7`.

### AUTO-01 — Reproducible validation entry point and CI

- Add a small maintained verification script and CI workflow using existing
  Make/Gradle gates. Support explicit developer-provided JDK/JBR locations without
  machine paths in Git; pin downloaded CI toolchains and verify published checksums.
- Run Go and Desktop lanes with useful per-stage results. Avoid the current quality
  script hiding downstream results after one failure; final exit still fails if
  any required stage fails. Keep thresholds and runtime dependencies unchanged.
- Accept: clean-checkout setup is documented once; route/docs links and tests are
  checked; build outputs stay ignored; default tests need no real provider/key;
  failures retain diagnostics and cannot appear as a green skipped stage.
- Verify: run the script locally, exercise failure reporting, Q. Creating local
  workflow files is the deliverable; enabling hosted services is not implied.
- Completed 2026-09-07. `scripts/validate.sh` runs the maintained Go, documented
  route/API-contract, quality and Desktop gates independently and reports all failed
  stages. `scripts/desktop-gradle.sh` applies Java 21 only for the launcher and Java
  25 only for JBR/JDK toolchains to every Desktop Make or script gate. It verifies
  both `java` and `javac`, rejects non-SDK and wrong-version locations, and accepts a
  Java 21 `JAVA_HOME`/`PATH` fallback only after verifying it rather than deferring to
  Gradle. The workflow invokes that same entry point with immutable action revisions,
  fixed Go/JDK versions, a checksum-verified JBR 25.0.4 download, and the Gradle
  wrapper checksum. Explicit-toolchain local validation, `make check`, `make quality`,
  `git diff --check`, Java 8/21/25 negative toolchain checks, valid `JAVA_HOME`
  fallback, and forced validation/quality failure paths passed with expected statuses.

### AUTO-02 — Bounded agent dispatcher with an independent review gate

- Implement the small workflow described in [tasks/README.md](tasks/README.md),
  preferably a script around `codex exec`, not a second agent platform inside
  Mini-Orca. Start with one writer and one fresh review run per task.
- The dispatcher reads this ledger, checks dependencies and a clean isolated base,
  obtains a lease, runs one allowed task, validates outside the worker, and records
  base/result identity, tests and reviewer outcome. Persist resumable run state in
  ignored local output; never infer success from the worker's final prose.
- Accept: dry-run selects correctly; failed dependency, crash, dirty base, stale
  lease, duplicate schedule, timeout, budget exhaustion, validation failure and
  review rejection stop dependent dispatch. Limit repairs to two attempts. Require
  explicit commit/integration authorization; no push/release or host cleanup.
- Verify: fake CLI/worker tests without paid calls, interruption/resume and lock
  tests, one explicitly authorized pilot task. Leave integration approval pending
  if it has not been granted; do not ask workers to self-approve.
- Completed 2026-09-08. The dispatcher is one repository script with ignored,
  resumable state, an atomic lease, one detached writer, a fresh read-only reviewer,
  two bounded repair cycles and explicit `--integrate` authorization for its local
  commit and fast-forward. It runs the base-verified `make check` gate in a fixed
  macOS sandbox that limits writes to the candidate, scratch and build caches and
  denies non-loopback network access. Agent output
  is bounded JSONL with no result-file path, all Git commands use a constructed
  environment that drops inherited redirects, and an interrupted reserved paid call
  is never retried automatically. The 14-case fake suite includes dependency, dirty
  base, lease, crash, timeout, budget, review, validation, mutation, resume and local
  integration paths without paid calls. Python compilation, shell/diff checks,
  `make check` and the same `make check` inside the production sandbox passed. One
  sandboxed race run hit the existing analyze-all temporary-session cleanup flake;
  its unchanged rerun passed. `make quality` retains only the two accepted baseline
  complexity findings. No configuration or source-data migration is required.

## Insight qualification tasks

These cards turn the remaining REL-01 work into one sequential agent queue. Keep
this ledger authoritative; do not create `docs/tasks.md` or per-task documents.
The existing v6 prompt and optional-explanation isolation are the starting point,
not tasks to reimplement. Follow the ledger dependencies; completed LEARN tasks stay complete.
Use one Terra worker, a fresh reviewer and coordinator-run gates per task, following
`tasks/README.md`. A worker does not approve its own result or change ledger status.

### Qualification contract shared by QUAL-01–06

- Proposed application-provider budget: **30 requests total**, split into at most
  **6 development requests + 24 qualification requests**. Each attempt allows
  4,096 output tokens and 300 seconds, sequentially, without automatic retry.
  The all-timeout request-time ceiling is 150 minutes, excluding scoring/builds.
  Planning these tasks does not spend or grant this new budget. Before QUAL-05,
  resolve the selected model/provider and one authorization covering both phases;
  reuse any explicit authorization already present instead of asking per case.
  The configured local model is the initial candidate. Do not switch models,
  send fixture content remotely, or add paid scoring calls without authorization.
- Qualification uses **12 distinct cases twice**, with a fixed order/seed and
  configuration: 8 substantive cases (16 attempts) and 4 omission controls
  (8 attempts). Development uses a separate three-case set twice at most.
  An implementation agent must not tune prompts against qualification responses.
  This is a small release sample, not a guarantee about arbitrary user projects.
- Count every scheduled attempt, including timeouts, malformed output, truncation,
  abstention and exhausted budget. Require **23/24 usable first-attempt summaries**,
  **22/24 fully valid summaries without invalid optional sections being dropped**,
  **13/16 substantive attempts with useful insights scoring >=6/8**, omission on
  **all 8 controls**, and **zero critical false claims across all 24 attempts**.
  A legitimate empty explanation map counts as complete; a rejected map does not.
  A rejected optional insight on a control is not successful intentional omission.
- Each retained insight needs correctness, local relevance, trade-off clarity and
  useful verification scores (0–2 each). Missing insight on a substantive case is
  zero useful coverage; errors never disappear from denominators. An independent
  reviewer scores actual prose against source and rubric; format validation,
  insight-presence booleans and model self-scores cannot establish usefulness.
- Store source-free receipts with case/repetition/attempt IDs, configuration and
  corpus/prompt/base identity, failure categories, output tokens, finish reasons,
  elapsed times and scores. Include counts and median/nearest-rank p95 latency,
  distinguishing successful latency from timeout/censored attempts. No endpoint,
  credential, source or free-form model prose in receipts or ordinary logs.
  Use private temporary fixture-only material for scoring; redact before any
  tracked regression fixture and discard temporary response material afterward.
- Failed qualification keeps QUAL-06 and REL-01 Blocked. Preserve all results;
  do not retry only failed cases, lower thresholds, change the corpus mid-run or
  restart a consumed budget. Diagnose offline first. A changed prompt/model starts
  a separately authorized full qualification with fresh run identity; additional
  model evaluation is a conditional recovery path, not mandatory paid work.

### QUAL-01 — Build realistic development and qualification cases

- **Shared input:** read the qualification contract above before work; its budgets,
  sample counts, privacy rules and pass criteria apply to this card.
- **Dependencies:** LEARN-02.
- **Target files:** `internal/app/testdata/engineering-insight-eval/cases.json`,
  `internal/app/engineering_insight_evaluation_test.go`,
  `docs/insights-performance/README.md`.
- **Implement:** evolve the existing corpus into explicit development/qualification
  partitions, with stable IDs, intent (substantive/control), reference mechanisms,
  permitted uncertainty, critical false-claim examples and 0–2 scoring anchors.
  Keep synthetic source beside the owning tests; no additional planning document.
  Supply complete imports/types and deterministic stubs so each case compiles in
  isolation with a temporary go.mod and no network access. Cover locking, cancellation, allocation,
  repeated I/O, idempotency and authorization, including protections in supplied
  callees and cases whose opaque dependencies require qualified statements.
  Keep at least four qualification cases materially different from development
  cases; do not claim secrecy from agents that can read the repository.
  Keep malformed-output parser tests separate from live control cases.
- **Accept:** three development cases and twelve distinct qualification cases
  (eight substantive, four controls) with mechanically checked source anchors;
  isolated compile/tests pass; known valid/invalid outputs still exercise strict
  parent validation, optional omission and the 1,000-rune bound. No fixture-specific
  answer leaks into production prompts.
- **Verify:** `go test ./internal/app -run '^TestEngineeringInsightEvaluation' -count=1`;
  G. Verify each temporary fixture with `go test ./...` using no external modules.
- **Evidence:** accepted after one repair and a fresh independent review. The
  corpus contains 3 development and 12 qualification cases (8 substantive,
  4 controls), each compiled in an offline temporary module. Controls require raw
  insight absence; the synchronized idempotency reference includes consumed-on-error
  semantics. Focused evaluation tests and coordinator-run G passed, including full
  race tests after repair; earlier worker runs encountered unrelated temporary
  directory cleanup flakes. `git diff --check` passed. No provider requests were
  made; the 30-request qualification budget remains unused. The old opt-in collector
  is not usable with this corpus until its scheduled QUAL-03 replacement.

### QUAL-02 — Enforce qualification metrics in the receipt validator

- **Shared input:** read the qualification contract above before work; its budgets,
  sample counts, privacy rules and pass criteria apply to this card.
- **Dependencies:** QUAL-01.
- **Target files:** `internal/app/engineering_insight_evaluation.go`,
  `internal/app/engineering_insight_evaluation_test.go`,
  `cmd/engineering-insight-eval/main.go`,
  `cmd/engineering-insight-eval/main_test.go`,
  `docs/insights-performance/README.md`.
- **Implement:** extend the existing receipt/validator, replacing the old
  one-row-per-case format. Uniquely identify case plus repetition and attempt;
  preserve raw outcome counts and bind scores to the corresponding response digest.
  Validate partition, complete expected schedule, model/prompt/corpus identity,
  authorized caps and actual consumption. Separate valid collection, incomplete
  qualification, failed qualification and passed qualification in output/exit status.
  Compute all shared-contract thresholds and latency statistics; distinguish
  optional-section degradation from failures and deliberate omission. Reject
  unknown fields, trailing JSON, duplicates, missing attempts, mixed candidates,
  invalid/nonfinite measurements and unscored qualification output. Do not print
  arbitrary invalid receipt content in errors. Critical claims block the entire
  run even if the offending example is not retained.
- **Accept:** boundary tests prove 22 usable fails/23 passes, 21 complete fails/22
  passes, 12 useful fails/13 passes, one bad control fails, and any critical claim
  fails. Budget or missing evidence cannot produce a passing verdict. Replace
  obsolete receipt tests/examples; historical recorded results remain historical.
- **Verify:** `go test ./internal/app ./cmd/engineering-insight-eval -count=1`; G.
  All test data is deterministic; the validator makes no provider call.
- **Evidence:** accepted after one repair and a fresh independent review. Strict
  receipt decoding rejects duplicate/unknown/aliased/null fields and trailing JSON;
  qualification binds digest-scored attempts to the external candidate, ordered
  24-attempt schedule and fixed caps. Exact passing/failing threshold boundaries,
  per-case intent stability, outcome/finish consistency and missing evidence are
  covered. Coordinator G and `make quality` passed after repair, plus
  `git diff --check`. No provider requests were made; all 30 authorized requests
  remain available. Collection/development execution is still QUAL-03 work.

### QUAL-03 — Replace the ad hoc collector with a resumable bounded runner

- **Shared input:** read the qualification contract above before work; its budgets,
  sample counts, privacy rules and pass criteria apply to this card.
- **Dependencies:** QUAL-02.
- **Target files:** `internal/app/engineering_insight_live_test.go` (retire),
  `internal/app/engineering_insight_runner.go` (new),
  `internal/app/engineering_insight_runner_test.go` (new),
  `cmd/engineering-insight-eval/main.go`,
  `cmd/engineering-insight-eval/main_test.go`,
  `internal/app/file_analysis.go` (only narrow shared evaluation hooks if needed),
  `docs/insights-performance/README.md`.
- **Implement:** add explicit collect/development/qualification modes to the existing
  command; reuse production selected-file prompt, model client and parser. Keep
  validation mode provider-free. Replace the environment-gated test collector;
  do not maintain two runners or introduce a second app/agent platform. Select
  the configured `bug` scope and production context limits; use synthetic facts
  and one isolated compiled fixture per case. Record that this is component-level
  qualification, not an import/Desktop end-to-end run.
- Persist a private run manifest and attempt reservations atomically under ignored
  `.mini-orca/autopilot/`; reserve budget before network dispatch. Reject concurrent
  writers, changed configuration/corpus/base, reused run IDs and exhausted caps.
  An interrupted in-flight attempt stays unknown/consumed; never automatically
  repeat it. Disable nested retries, count every transport attempt, and apply the
  request deadline. Explicit selection/confirmation binds any remote delivery to
  its configured destination; local-only remains the default. Never modify the
  user's configuration or fixture source. Whitelist persisted provider metadata.
- **Accept:** fake-provider tests prove exact request limits, no hidden import or
  retry, crash/resume accounting, failed persistence before dispatch, lock safety,
  source/secret-free receipts and safe scoring handoff. Default `make check` and
  `make quality` cannot contact a configured live model, even from inherited opt-in
  environment variables. Allow the independent scorer to read selected fixture
  prose privately without requiring a second generation call or source-bearing logs.
- **Verify:** `go test ./internal/app ./cmd/engineering-insight-eval -count=1`; Q.
- **Evidence:** accepted after the user-authorized additional repair and fresh
  independent review. Every scorer/handoff/export entry now uses the shared strict
  run-ID guard; hostile existing dot-ID manifests cannot trigger state deletion.
  API and CLI regressions cover campaign preservation, the actual three-case
  development lifecycle, 6/24 aggregate request caps, consumed crash reservations,
  private response/score digest binding and explicit export. Production runtime
  manifest metadata is shared with the runner. Coordinator `make check` and
  `make quality` passed with `MINI_ORCA_LIVE_INSIGHT_EVAL=1`, plus
  `git diff --check`; the retired environment-gated collector cannot run.
  Windows command cross-build passed; evaluation locking there intentionally fails
  closed at runtime. No real provider requests were made; the 30-call budget is
  unused. The scheduler remains stopped; QUAL-04 is the next ready task.

### QUAL-04 — Improve grounded insights using development cases only

- **Shared input:** read the qualification contract above before work; its budgets,
  sample counts, privacy rules and pass criteria apply to this card.
- **Dependencies:** QUAL-01, QUAL-03.
- **Target files:** `internal/app/file_analysis.go`,
  `internal/app/file_analysis_test.go`, `internal/app/engineering_insight_test.go`,
  `internal/app/testdata/engineering-insight-eval/cases.json` (development examples
  only), `docs/insights-performance/README.md`.
- **Implement:** build on v6 rather than appending another competing instruction
  block. Require a visible mechanism, exact local evidence, a conditional impact,
  meaningful trade-off and concrete verification when an insight is returned.
  Teach omission on trivial wrappers and distinguish unknown downstream behavior
  from demonstrated missing safeguards. Use compact, general examples drawn only
  from development cases; do not demand a lesson for every file or insert a second
  automatic model request. Keep prose limits, advisory status, strict core schema,
  target identity and optional-explanation isolation intact.
- Keep changes local to file analysis unless a demonstrated shared-contract defect
  requires a separately scoped task; avoid gratuitous changes to Security/drafts.
  Increment the prompt version on behavior changes and prove old caches become
  stale. Tests check representative behavior/contracts, not just prompt substrings.
- **Accept:** deterministic positive, uncertainty and omission examples pass;
  no broader quality claim is made until fresh independently scored provider runs.
  The qualification set/rubric has not been modified to fit the candidate.
- **Verify:** `go test ./internal/app ./internal/project -count=1`; G.
- **Evidence:** independent review accepted v7 grounded guidance, advisory
  four-field file-insight validation and shared runner diagnostics. Tests verify
  v6 cache staleness and rejected/degraded incomplete insights at file, risk and
  suggestion locations. Coordinator G, `make quality` and `git diff --check` passed.
  Qualification cases/rubric are unchanged; only development example wording was
  refined. No provider requests were made, and all 30 authorized requests remain
  available. Live insight quality remains unqualified until QUAL-05/06.


### QUAL-05 — Run and independently score the bounded development pilot

- **Shared input:** read the qualification contract above before work; its budgets,
  sample counts, privacy rules and pass criteria apply to this card.
- **Dependencies:** QUAL-04; selected provider/model and the shared 30-request
  authorization resolved before any call. Mark Blocked with the missing decision
  only if existing session authorization does not cover the new run.
- **Target files:** `docs/RELEASE_ACCEPTANCE.md` (sanitized evidence);
  `internal/app/file_analysis.go`, `internal/app/file_analysis_test.go`,
  `internal/app/engineering_insight_test.go` (only a demonstrated bounded repair).
- **Execute:** run the three development cases, inspect real output with a fresh
  scorer, and permit at most one focused correction plus a second three-case pass.
  Six calls is a total cap, not a cap per model or restart; spare calls do not enlarge
  the 24-call qualification budget. All development controls must omit low-value
  lessons and substantive cases must meet >=6/8 before promotion. If the pilot fails,
  stop and record specific mechanism/verification/schema failures. A different
  model is evaluated only within an explicitly authorized candidate/budget decision.
- **Accept:** independent scores and real provider metadata support promoting a
  single candidate; lock prompt/corpus/config/base identity for qualification.
  Do not expose qualification outputs to the tuning worker. This is development
  evidence, never a release reliability percentage.
- **Verify:** run collect and validate with the exact CLI implemented/documented
  by QUAL-03; attach its reproducible command with secret values omitted. Receipt
  validator and `git diff --check`; G if prompt/code changed, fresh review after fixes.

### QUAL-06 — Qualify the frozen candidate and issue the release verdict

- **Shared input:** read the qualification contract above before work; its budgets,
  sample counts, privacy rules and pass criteria apply to this card.
- **Dependencies:** QUAL-05; QUAL-02/03 validation and accounting remain intact.
- **Target files:** `docs/RELEASE_ACCEPTANCE.md`,
  `docs/insights-performance/README.md` (tested scope/limitations only).
- **Execute:** twelve qualification cases twice, exactly 24 reserved single
  attempts at the locked settings. Generate all outputs before tuning or deciding
  to rerun anything. A fresh reviewer scores every attempt against the frozen rubric
  and actual fixture prose, including failed/degraded/control cases; a separate
  coordinator checks evidence joins, counts and the computed verdict. Label agent
  scoring honestly, not as human testing. Any separately metered evaluator requires
  its own explicit budget; do not hide it in the 30 generation requests.
- **Accept:** the receipt validator enforces 23/24 usable, 22/24 fully valid,
  13/16 useful substantive insights, 8/8 controls omitted and zero critical claims.
  Retain failure evidence and explain why this small sample does not guarantee
  production reliability. On failure, stay Blocked with an exact cause and bounded
  next proposal; switching model/prompt or retrying a case cannot salvage this run.
  On success, mark only QUAL-06 Complete and let REL-01 perform final integration.
- **Verify:** exact collection/scoring/validation commands from QUAL-03, independent
  receipt inspection, `go test ./internal/app ./cmd/engineering-insight-eval -count=1`,
  `git diff --check`. No code changes during a qualification run.

### REL-01 — Validate the selected release end to end

- Run Q on the integrated candidate, then desktop packaging with JBR 25. Smoke the
  actual bundle and its required `java.net.http`/`jdk.unsupported` modules. Claim only
  platforms tested; if Docker remains supported, build/health-check it separately.
- Reuse the disposable release fixture for replace/create → validation → checks →
  review → explicit Apply → Undo, failed checks, stale files, cancellation,
  local/offline/remote scopes and denied consent. Include Security/measurements only
  once in this release's implemented scope. Use no secrets in captures or records.
- Repeat native checks for surfaces changed since UI-04, not every historical
  capture. Resume after QUAL-06 passes the frozen qualification contract above;
  reuse its provider/quality evidence instead of spending another live budget.
  Run Q on the final integrated candidate and rebuild/smoke affected distributions
  if implementation changes invalidated their prior evidence. QUAL-06 does not
  replace end-to-end source-safety and native/package acceptance.
- Accept: exact release scope, real check results, runtime setup and unresolved
  limitations recorded in `docs/RELEASE_ACCEPTANCE.md`. Missing required native,
  provider or supported-package evidence blocks this release decision.
- Review evidence 2026-09-08. Q is green after reducing the last two Go complexity
  findings with focused corruption and mid-copy cancellation coverage. The macOS
  arm64 package, embedded JBR 25 modules and native startup passed. A disposable
  fixture passed replace/create through explicit Apply and Undo; native Security
  and local Performance lifecycle states were inspected. Docker Desktop built the
  image and its container reached healthy. The Dockerfile now builds for BuildKit's
  target OS/architecture and fails closed when either target is absent; a newly
  built native arm64 image, Compose build, negative empty-target build, and retained
  healthy loopback container passed. Deterministic ephemeral-root and loopback tests
  passed required-check/source-write protection, stale identities, cancellation,
  refused-provider failure, local/remote/mixed scope consent denial, Analyze-all
  retry-budget handling, Performance empty/failed/partial/stale/restart/budget
  lifecycle states, and exact-function
  handoff. Fresh review accepted this retained repair diff. Native narrow/wide and
  offline failure inspection closed the changed-surface checks for the declared
  macOS arm64 and Linux arm64 release scope. The bounded 2026-09-08 live sample
  used the configured local OpenAI-compatible provider and
  `qwen/qwen3-coder-30b` model alias. All three scopes were capped at 800 output
  tokens with one retry; `analyze` was local, so no external data transfer or
  remote-provider consent prompt was required. It made exactly two imports and
  five selected-file analyses. Source-free local provider request metadata recorded
  exactly seven chat-completion requests matching those persisted operations, which
  establishes the 7/8 budget and that no retry request occurred. The first run
  produced one fresh file analysis and two source-free parser failures,
  `The model returned an unusable file summary.`; the second produced two fresh file
  analyses. Those parser failures occur after successful transport and outside
  `s.retry`. The source-free receipt passed `go run ./cmd/engineering-insight-eval`.
  The fresh cancellation/locking and idempotency insights scored 0/8 and 4/8,
  respectively, and neither is retainable. This selected local model produced only
  3/5 contract-valid file analyses, and the two scored fresh insights failed the
  6/8 threshold. Follow-up diagnosis found unknown parameter/field keys in
  optional symbol explanations; all seven responses ended normally below 800 tokens.
  The repair explicitly constrains prompt keys (file-analysis-v6) and omits the
  entire invalid optional explanation map, preserving strict parent and target
  validation. Eight new local single-attempt cases at 4,096 tokens/300 seconds
  again ended normally (345–528 output tokens). Before omission, 3/8 parsed; replay
  of those same responses with the repair gave 8/8 usable summaries, comprising
  3 complete and 5 with omitted explanations. This is offline replay evidence,
  not an 8/8 fresh-run qualification or proof of insight quality. Trivial controls
  still generated generic insights. REL-01 remains Blocked pending QUAL-01–06,
  which own the repair, bounded runs and useful/appropriately omitted insights. No config migration
  is needed; the prompt version invalidates prior file-analysis cache entries.

### REL-02 — Finish code/document retirement and handoff

- Review integrated diff/dependencies for dead helpers, duplicate rules, leftover
  preview state, unused compatibility readers, misleading docs and debug output.
  Preserve tests for current rejection and safety behavior even when old code is gone.
- Merge remaining Task 170 working notes into the canonical acceptance evidence
  without losing uncommitted user content; retire the old task and redundant
  keyboard/visual history only after their current reproduction instructions and
  open checks have one maintained owner. Git is the archive.
- Accept: one plan/status ledger, one execution guide, one UI guideline, one API
  guide/OpenAPI pair, concise usage/config/runtime docs and one acceptance ledger.
  No new BASELINE/ACCEPTANCE file per task. Plan completion requires every required
  task accepted or a specifically recorded user-approved deferral.
- Verify: Q if code changes; otherwise relevant contract/doc checks and links,
  `git diff --check`, tracked file/Markdown counts and original user-diff preservation.
  Report migrations/reset implications; do not create commits unless authorized.

## Documentation policy and deferred suggestions

This cleanup removes completed task files, duplicate plans/prompts, obsolete UI
baselines and superseded release prose after carrying open obligations above.
It retains the existing uncommitted Task 170 evidence, current contrast results,
and test-consumed API/version artifacts. The detailed old code is in Git history;
runtime compatibility is not maintained by keeping an archive in the repository.

Keep README for use, CONFIG for supported fields, desktop README for runtime and
keys, guidelines for visual rules, API guide/OpenAPI for contracts, this PLAN for
decisions/tasks, task README for execution, and release acceptance for evidence.
Each fact has one authoritative home. Update links on every move; do not create
redirect documents or new task files just to preserve old filenames.

Possible later additions, after using the improved daily loop:

- Dependency vulnerability evidence via a pinned `govulncheck` adapter, separate
  from source findings and explicit about tool/database freshness. It is part of
  Go's supported vulnerability tooling. [Go security documentation](https://go.dev/doc/security/)
- A small local “saved insight” collection only if repeated use demonstrates a
  retrieval need; avoid a learning dashboard, quizzes, streaks or a vector database.
- Parser-backed exact editing for one additional language after choosing a real
  workload. Keep unsupported languages analysis-only in the meantime.

Defer streaming/tool-calling/provider SDK proliferation, autonomous whole-project
repairs, multi-file tabs/editing, terminal/debugger/VCS writes, plugin systems and
custom titlebars. They add maintenance without improving the one-function loop
enough to justify them now.
