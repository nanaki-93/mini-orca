# Mini-Orca — clearer results, unified analysis and a terminal

Planning proposal prepared on **2026-09-11**. Preserve the current dark IDE style,
make model explanations and actions readable at a glance, run project-wide analysis
with separate **Bugs / Performance / Security** result pages, expose new-function
generation, and replace the duplicated bottom tools with a real terminal. The user
approved the plan and authorized scheduled implementation on **2026-09-11**.
Task preparation and scheduler setup do not mark any implementation complete.

This file owns product decisions and the execution/status ledger. `docs/tasks.md`
owns the current implementation cards. Previous status and acceptance records
remain below; their automation and commit instructions do not extend this scope.

## Findings from the current checkout

| Request | What exists | What needs to change |
| --- | --- | --- |
| Cleaner UI and readable explanations | Jewel, shared dark tokens, pane headers, badges and disclosures already exist. The uncommitted Context changes add Actions / Explain / Details. | Apply a consistent content hierarchy throughout the app. `DeclarationExplanationDetails` joins several facts into muted text with middle dots; Assistant renders each turn as plain `Text`; insight labels use 10sp text and nested scrolling. |
| One analysis workflow | Separate Analysis, Bugs, Performance and Security workspaces; separate Analyze-all and Performance jobs. Their daemon controllers explicitly reject simultaneous runs. | One project-wide Start action and progress owner; results stay in their own Bugs, Performance and Security pages. A Start button cannot simply launch the existing jobs together. |
| Generate a new function | `create_symbol` is already supported by chat, draft composition, validation and Apply. The desktop enters it through `create_declaration` in the command palette. | A visible file-level action, a clear creation form, correct focus and useful eligibility messages. Reuse the current generation pipeline. |
| Remove bottom duplicates | Problems repeats findings, Checks repeats Review evidence, and Output aggregates operation/scan diagnostics. | Move any unique diagnostic information into its owning workflow before deleting these three bottom surfaces. |
| Bottom terminal | No terminal implementation or terminal dependency is present. | Add desktop-owned PTY lifecycle and a terminal renderer; an output text box is insufficient for interactive shell behavior. |

Relevant boundaries: `internal/app/` owns requests, source freshness and guarded
mutation; `internal/project/` owns typed reports and persistence;
`internal/api/handlers/` and `cmd/daemon/main.go` own HTTP contracts; desktop
presenter/controller state owns UI publication and rejects late results.

The working tree already contains changes to Context, shared controls/theme,
layout defaults, desktop documentation and visual/keyboard tests. Use that working
tree as the implementation baseline and preserve unrelated edits. No new theme,
Jewel migration, broad refactor or insight qualification campaign is required.

## Confirmed product decisions

The user answered the planning questions on **2026-09-11**. These answers define
the implementation scope; the later request authorizes its scheduled execution.

| ID | Question | User answer and resulting scope |
| --- | --- | --- |
| D1 | Analyze the selected file or always the whole project? | **Always analyze the whole project.** The primary Analyze action never depends on the selected file. File selection/filtering affects result inspection only. |
| D2 | Real interactive terminal or individual command runner? | **Real interactive terminal.** Use a local shell rooted in the open project; PTY/native packaging and keyboard verification are required. |
| D3 | Stronger hierarchy with expandable detail, or all detail expanded? | **Use the proposed hierarchy.** Short visible summary, headings, labeled severity badges, highlighted code references and expandable technical details; retain the style. |
| D4 | Go-only new functions, or additional languages? | **Make Go generation prominent.** Additional language generation is outside this plan. |
| D5 | How should the unified analysis be presented? | **One Start button; results in their own sections; Analysis only tracks progress.** Keep separate Bugs, Performance and Security destinations. Coordinating the existing analyzers is the implementation recommendation; the user did not request a single model response. |

Keep **Summary / Analysis / Bugs / Performance / Security / Editor** as distinct
destinations. Give the three results pages a clear shared visual grouping in the
sidebar, with counts/state next to their labels. **Analysis** owns Start and run
progress, including coverage and failures; it does not contain findings lists.
Its progress rows link to each result page. Each result page can filter to a file
without starting analysis or changing the project-wide run scope.

Keep source-based analysis separate from explicit execution: tests/vet remain an
optional **Run checks** action inside Bugs, and benchmark measurement remains
explicit in the Performance detail/review flow. The ordinary Analyze action can
run the existing passive Go security rules, but does not silently start tests,
benchmarks or model-suggested commands.

## Intended interaction and presentation

1. Open a project; its name and the project-wide **Start analysis** action are visible.
2. Choose **Start analysis**. A compact preflight identifies the project, covered files,
   excluded/oversized files, relevant providers and any required fresh consent.
   Existing advanced limits remain available in a disclosure.
3. Analysis shows one run with three progress rows. Results appear progressively
   in the separate Bugs / Performance / Security pages. Each section
   distinguishes running, completed-empty, partial, failed, skipped, canceled and
   stale. A failure in one section does not erase valid results in another.
4. Each result starts with a concise title and visible severity/state, followed by
   the supported explanation and source location. Evidence, provenance and longer
   recommendations expand locally. Selecting a source reference opens the exact
   indexed file/line; **Prepare fix** only prepares the existing Assistant flow.
5. In Editor, **New function** is visible with an open eligible Go file, including
   a valid package-only file with no declarations. It opens a form showing the
   file, required function name and behavior request. Generate produces an editable
   isolated draft and a read-only composed diff; validation, checks and explicit
   Apply/Undo remain the existing workflow.
6. The bottom area contains **Terminal** only. It starts collapsed with an explicit
   open control. First activation starts the shell; collapsing preserves the
   session. Close/restart and session-exit states are visible.

Presentation rules:

- Keep the shared dark palette, square panes, separators and blue selection edge.
  Use teal for explanatory content and labeled warning/error/success accents for
  state. Category identity must not look like a severity verdict.
- Use a readable 12–13sp body with 18–20sp line height and semibold headings. Do not
  use tiny muted labels for the main explanation. Render structured facts as
  separate labeled rows or lists, never one long middle-dot-separated sentence.
- Keep the summary, actionable failures and primary action visible. Collapse
  provider metadata, hashes, repetitive context and optional technical detail.
  Disclosure controls must remain obvious and keyboard accessible.
- Render model emphasis, lists and inline code through a bounded text component
  where the payload is freeform. Structured explanation fields remain structured;
  do not invent a summary or run another model to improve formatting. Code blocks
  remain selectable/read-only, and malformed formatting remains readable text.
- Use one main scroll region per pane; avoid a scrollable explanation nested in
  another scrollable sidebar. Long paths/errors must wrap or offer full detail.
- Counts mean actual current findings under the selected result filters. Missing, unrun,
  unavailable or stale results must not become green zeroes. Preserve AI/tool
  provenance and distinguish performance hypotheses from measured benchmarks.

## Architecture and migration decisions

**Unified analysis:** use one daemon run coordinator for project-wide scope,
calling the existing per-file services. Preserve their validators,
source-policy checks and report stores. Replace the duplicated scheduling owners
as the unified runner takes over; do not layer a third independent scheduler over
Analyze-all and Performance. Existing public routes should be narrow adapters to
the shared owner where their behavior can be preserved. Define and document any
unavoidable incompatibility before removing a public route.

The primary UI has no file/project scope selector. Existing file-analysis APIs can
remain available for compatibility, but Context offers **Analyze project** or
**View this file's results**, clearly distinguishing execution from local filtering.

One run needs a stable ID/generation, project/revision, immutable queue identities,
per-file hashes, policy/provider fingerprints, bounded work/attempt accounting and
per-section progress. Project scope means enumerating all policy-eligible files;
process them in bounded batches when necessary rather than silently analyzing only
the first 100/500 files. Show excluded/unsupported coverage separately. If a time
or request budget is reached, preserve pending work and show Paused/Partial with
explicit continuation; never label incomplete project coverage Completed.
Persist source-free progress using `internal/storage/atomic.go`.
Report publication and progress updates must remain tied to the captured run;
late results cannot overwrite a replacement run or a newly opened project.

**Classification is a real contract gap:** current semantic `risks` have no
category, while Performance and Security have different typed contracts. Do not
label every old risk a bug or categorize prose with keyword matching. Add an
explicit category to newly generated semantic risks, validate its enum and route
it at the report boundary. Existing specialized reports carry their known
category. Retain source-specific evidence types behind a small presentation
adapter, rather than flattening away security anchors or benchmark evidence.
Old unclassified reports remain readable as previous analysis details, with a
refresh label; they do not enter fresh category counts. General explanations and
non-finding suggestions remain in Context/Summary rather than becoming bugs.

Changes to the semantic response schema require a new prompt/cache identity and
updated offline contract/evaluation tests. Preserve historical evaluation verdicts,
budgets and sealed material; this plan does not resume live qualification.

**Consent and cost:** admission binds the specific queue and relevant providers.
Keep Bug and Analyze scope confirmations explicit, and retain fresh Security
review intent within that admission. Do not reuse an unrelated Summary/Performance
checkbox as Security consent. Opening pages, switching sections and restoring
reports make no provider requests. Resume after restart or changed consent context
requires the corresponding explicit action; authorization is not persisted as a
reusable boolean. Avoid multiplying the existing provider retry loop by a second
unbounded job retry loop.

**Creation:** use `ChatEditMode.CreateSymbol`, the current chat routes and
`ComposeGoDeclaration`. Creation appends one supported top-level declaration and
explicit imports to the current existing Go file. Creating files, multi-function
edits and adding receiver-method creation are outside the confirmed Go-creation scope.
Retain the already supported type-creation capability through a clearly labeled
secondary creation mode. Correct the desktop's ASCII-only/keyword validation
mismatch with the Go boundary and ensure invalid names are explained before a
model request. The daemon remains authoritative.

**Terminal:** own the shell in the desktop process, without adding a general
command-execution HTTP endpoint. Start with one session for the current project,
no tabs/session persistence requirement. Use the local project's canonical path;
if the daemon exposes a container-only path, show an actionable unavailable state
instead of silently starting in another directory. A local path mapping can be a
follow-up if container-backed use requires it.

[JediTerm](https://github.com/JetBrains/jediterm) provides an embeddable Swing terminal
and [Pty4J](https://github.com/JetBrains/pty4j) is the associated PTY candidate.
Embedding them in Compose is a proposed implementation choice, not a verified
compatibility claim for this app's JBR 25/macOS arm64 package. Validate dependency
versions, redistribution notices, native libraries, focus and packaging in TERM-01
before relying on them. Do not implement an ANSI emulator from scratch.

The terminal is an ordinary user-controlled local shell: commands can change
files. It must not be described as the daemon's copied-workspace check sandbox.
No model output or report selection writes to terminal stdin. Do not log terminal
input/output to app metadata or include it in prompts automatically. Shell-driven
file changes must invalidate affected review/analysis evidence through the existing
freshness checks. Keep project-execution trust for automated checks independent.

**Retirement:** retain saved Performance/Bugs/Security navigation identities and map
old bottom-tool preferences to a collapsed Terminal. Preserve
pane sizes. Keep existing report files and triage metadata; do not delete old caches
or automatically resume old jobs. Restore any older active job as interrupted/paused
with explicit recovery, and preserve attempt counts during migration.

## Scheduled implementation — authorized 2026-09-11

The user approved this plan and requested scheduled execution with **GPT-6 Astra
Extra High** (`gpt-6-astra`, `xhigh`). The executable cards now live in
[docs/tasks.md](docs/tasks.md); [tasks/README.md](tasks/README.md) owns the run procedure.

Scheduler: **Mini-Orca UX implementation**, every **20 minutes**, attached to this
Codex task; automation ID **mini-orca-ux-implementation**, status **Active**.
The user authorized TERM-02 repair; the ordered scheduler resumes after its verified local commit.
The app accepted the task model override `gpt-6-astra` / `xhigh`; this heartbeat
uses the task's settings rather than a separate scheduler model field. One card per wake,
strictly in order, with required checks and diff review before acceptance.
**Per-task local commits are authorized by the user on 2026-09-11.** Create one
commit for each validated task, including its related checklist/status updates,
with the task ID in the commit subject. Review the staged diff, exclude unrelated
pre-existing edits and preserve any unrelated staged changes. Verify and report
the resulting commit hash before advancing. A failed commit leaves the task at
the commit stage for recovery; never repeat implementation or create a duplicate
commit after an interrupted wake. Pushes and releases remain outside scope.

**Accepted:** 15/17. **Active writer:** none after BOTTOM-01 acceptance. **Next:** BOTTOM-02.

| Order | ID | Outcome | Status |
| --- | --- | --- | --- |
| 1 | [UX-01](docs/tasks.md#task-ux-01--establish-readable-result-primitives) | Establish readable result primitives | Complete; locally committed |
| 2 | [UX-02](docs/tasks.md#task-ux-02--apply-the-hierarchy-to-explanations-and-model-responses) | Apply the hierarchy to explanations and model responses | Complete; locally committed |
| 3 | [CREATE-01](docs/tasks.md#task-create-01--expose-creation-in-the-normal-file-workflow) | Expose creation in the normal file workflow | Complete |
| 4 | [CREATE-02](docs/tasks.md#task-create-02--close-creation-validation-and-lifecycle-gaps) | Close creation validation and lifecycle gaps | Complete; locally committed |
| 5 | [ANA-01](docs/tasks.md#task-ana-01--define-categorized-results-and-unified-run-contracts) | Define categorized results and unified run contracts | Complete; locally committed |
| 6 | [ANA-02](docs/tasks.md#task-ana-02--produce-and-validate-explicit-semantic-categories) | Produce and validate explicit semantic categories | Complete; locally committed |
| 7 | [ANA-03](docs/tasks.md#task-ana-03--compose-the-per-file-analysis-stages) | Compose the per-file analysis stages | Complete; locally committed |
| 8 | [ANA-04](docs/tasks.md#task-ana-04--implement-one-durable-bounded-run-lifecycle) | Implement one durable, bounded run lifecycle | Complete; locally committed |
| 9 | [ANA-05](docs/tasks.md#task-ana-05--migrate-existing-jobs-and-expose-the-unified-api) | Migrate existing jobs and expose the unified API | Complete; locally committed |
| 10 | [ANA-06](docs/tasks.md#task-ana-06--give-the-desktop-one-analysis-owner) | Give the desktop one analysis owner | Complete; locally committed |
| 11 | [ANA-07](docs/tasks.md#task-ana-07--separate-run-progress-from-the-three-result-pages) | Separate run progress from the three result pages | Complete; locally committed |
| 12 | [NAV-01](docs/tasks.md#task-nav-01--distinguish-run-results-and-editing-in-the-sidebar) | Distinguish run, results and editing in the sidebar | Complete; locally committed |
| 13 | [TERM-01](docs/tasks.md#task-term-01--prove-the-terminal-dependency-and-local-session-boundary) | Prove the terminal dependency and local-session boundary | Complete; locally committed |
| 14 | [TERM-02](docs/tasks.md#task-term-02--integrate-the-interactive-terminal-pane) | Integrate the interactive terminal pane | Complete; locally committed |
| 15 | [BOTTOM-01](docs/tasks.md#task-bottom-01--preserve-unique-diagnostics-in-their-owning-workflows) | Preserve unique diagnostics in their owning workflows | Complete; locally committed |
| 16 | [BOTTOM-02](docs/tasks.md#task-bottom-02--replace-the-bottom-tools-with-terminal-only) | Replace the bottom tools with Terminal only | Queued |
| 17 | [VERIFY-01](docs/tasks.md#task-verify-01--validate-the-complete-interaction-and-document-support) | Validate the complete interaction and document support | Queued |

**Accepted BOTTOM-01 — 2026-09-12:** Check/validation details stay in Review,
request failures appear beside their Assistant target, verified-scan commands and
output live in Bugs, and Analysis/status retain operational and selected-file
failures. Shared bounded, selectable diagnostics preserve copy access. Passed
35 prescribed tests, 405 working-tree tests, 403 isolated tests, desktop quality
and package build after two recorded corrections. Native macOS status details
and copy were verified. All ten unrelated UI edits remain untouched. BOTTOM-02
now removes the duplicate bottom tools; no migration or live-provider calls.

**Accepted TERM-02 — 2026-09-12:** The interactive terminal now owns a persistent
project shell with themed output, working resize, explicit close/reopen and
native Editor focus return. Shell edits refresh source and invalidate stale
review evidence. Passed 71 prescribed tests, 398 working-tree tests, 396 isolated
tests, both package builds and desktop quality checks. Native macOS arm64 proof
covers prompt, history, interrupt, Vim, resize, focus return and app-exit child
cleanup. All unrelated UI deltas remain separate. Seven correction groups total,
including the user-authorized continuation, are recorded in docs/tasks.md;
historical failures remain in docs/errors.log. Resume the scheduler for BOTTOM-01
after the verified local commit. No migration, Go change or live provider calls.

**Historical blocked TERM-02 checkpoint — 2026-09-12 (resolved):** Terminal integration candidate was preserved.
Correction-1 passed 71 focused tests. Native verification found PTY resize and
narrow-overlay focus defects; correction-2 addresses those and the routing quality
findings but fails compilation because ScaledTerminalPanel's buffer/style
constructor arguments are reversed. Two corrections exhausted; no third attempted.
Required final checks remain incomplete. See docs/tasks.md and docs/errors.log.
At that checkpoint the scheduler was **Paused**, no TERM-02 commit existed and no later card had started.

**Accepted TERM-01 — 2026-09-12:** Pinned terminal dependencies, packaged notices
and a stable local PTY owner are ready. On macOS arm64, actual packaged-runtime
probes pass cwd, UTF-8, resize, interrupt, child cleanup and bounded close. All 13
terminal tests, 389 working-tree tests, 387 isolated tests, both package builds,
Detekt, Spotless and diff review pass after two recorded corrections. Unrelated
UI changes remain intact. No daemon command endpoint or automatic shell start was
added. Native pane/focus integration is TERM-02, which is next on the active Astra
Extra High scheduler; the TERM-01 commit is recorded in its ignored receipt.

**Accepted NAV-01 — 2026-09-12:** Project, Results and Editing groups distinguish
analysis progress from findings and editing. Full labels, actual run counts/state,
keyboard reveal, project-scoped palette actions and captured provider context are
in place. Stored destinations, shortcuts, pane minima and unrelated UI edits are
preserved. All 50 prescribed tests, 376 working-tree tests, 374 isolated candidate
tests, Detekt, Spotless and diff review passed after two recorded corrections.
Rail component renders cover the viewport/text matrix; native popup/focus and
screen-reader checks remain unverified. The local commit is recorded in the ignored
NAV-01 receipt. Scheduler remains active for TERM-01 next.

**Accepted ANA-07 — 2026-09-11:** Analysis now tracks one whole-project run;
Bugs, Performance and Security present their own results with local filters,
visible severity/state and provenance, and responsive list/detail views. Context
provides Analyze project and local file-results navigation. Exact Go preparation,
triage and explicit benchmark trust remain. The user-authorized repair removed
three references to an uncommitted theme role. The prescribed 55 tests, all 370
working-tree desktop tests, isolated 368-test commit candidate, Detekt, Spotless
and diff checks passed. Component render coverage and native/Go/live-provider
limits are retained in the card. Three corrections are recorded, including the
authorized continuation. The local commit is verified in the ignored receipt;
scheduler resumes with NAV-01 next.

**Accepted ANA-06 — 2026-09-11:** The desktop now admits and follows one daemon-owned
project analysis run. Existing analysis entry points open a shared preview naming
all provider scopes and explicit Security intent. Consent is consumed before each
Start/Resume attempt; reconnect and section reads preserve typed evidence and
localized failures. The duplicate Analyze-all/Performance pollers and standalone
AI Security review owner are removed. Deterministic scans, benchmarks and draft
review retain their explicit workflows. The focused suite, all 378 desktop tests,
quality gates and the isolated commit candidate passed. Two corrections and the
component-render limitations are retained in the card. Result-page restructuring
is ANA-07. The scheduler remains Active; no other card started this wake.

**Accepted ANA-05 — 2026-09-11:** Analyze-all and Performance now use one durable
analysis owner, with strict unified preview/start/read/control/result routes.
The legacy adapters preserve single-purpose scope and bounded queues; Performance
retains its total active-time budget across resumes and conservatively accounts
for interrupted work after restart. Old job metadata remains readable without
dispatch; its incomplete identity/counter contract requires an explicit new start.
The worker-wait cleanup fix and restart-budget regression passed the prescribed
race suite, full Go tests, full race tests, formatting and vet. Four corrections
in total are recorded, including the user-authorized repair continuation. All ten
unrelated desktop files remain unchanged. The local commit was verified and the
scheduler resumed; ANA-06 is next. No push or release is authorized.

Task bodies have moved to `docs/tasks.md` to keep one implementation specification.
The completed cleanup task cards are preserved in
[cleanup history](docs/history/cleanup-tasks-2026-09-10.md). Historical cleanup,
dispatcher and insight-evaluation authorizations remain inactive for this queue.

UX-01 validation on **2026-09-11**: 19 focused tests and all 362 desktop tests
passed; Detekt, Spotless and diff checks passed. Shared headings, labeled badges
and bounded selectable result formatting are ready for adoption in UX-02.
Offscreen production renders covered narrow/wide layouts, larger text, full-response
disclosures and keyboard focus. Two focused corrections resolved badge contrast
and the contrast test's surface assumptions; diagnostics remain in the task card.
No Go changes, dependencies or migration steps. The isolated commit candidate
also passed all 360 tests and quality checks without pre-existing desktop edits.
The authorized local commit's hash is reported in the task response/local receipt.

UX-02 accepted on **2026-09-11**: explanations now separate facts from source
metadata; Assistant distinguishes requests, model responses and drafts; project
and engineering prose use the shared formatter with one parent scroll owner.
Review leads with the next action, and validation messages stay visible. Tightened
formatted list spacing. Passed 43 focused tests, all 366 desktop tests and quality
checks; the isolated commit passed all 364 tests and quality checks. One focused
test correction retained existing insight whitespace normalization. Production
offscreen renders and the exact diff were reviewed. The required explanation
status helper is included; unrelated existing tab/layout work remains uncommitted.
No daemon, API, dependency or migration changes. Commit hash: task response/local receipt.

**Accepted ANA-01 — 2026-09-11:** Added explicit semantic finding categories while
preserving old reports, stable IDs and triage. Defined whole-project preview,
run/control and separate result contracts with bounded requests, transient consent
and honest empty/partial/failure coverage. Future endpoints remain explicitly
planned until ANA-05. Focused/full Go tests, race tests, formatting, vet and diff
review passed after two naming/schema corrections. All ten pre-existing desktop
edits are preserved. No migration or live model calls; desktop checks are not
applicable. Local commit evidence: `.mini-orca/autopilot/ux/ANA-01/receipt.json`.

**Accepted ANA-02 — 2026-09-11:** New semantic findings require an explicit Bugs,
Performance or Security category through the shared schema/parser. Prompt v14
adds category-specific evidence guidance and leaves older caches readable as stale
until explicit analysis. Optional diagnostics and one-file target checks remain.
Focused/full Go tests, race tests, formatting, vet and diff review passed without
corrections. Historical evaluation grants stay fixed; retired v13 dispatch is
rejected without consumption. No live qualification or desktop changes. Local
commit evidence: `.mini-orca/autopilot/ux/ANA-02/receipt.json`.

**Accepted ANA-03 — 2026-09-11:** Composed private per-file semantic, Performance,
Security rules and Security AI stages using existing parsers and report stores.
Matching cache evidence, partial results and producer identities remain distinct;
model failures retain earlier reports. Each request/retry reserves a bounded
attempt and rechecks ownership, source and consent; publication rejects stale or
canceled work. Focused/full Go tests, race tests, formatting, vet and diff review
passed after one correction preserving preparation-error behavior. All ten
unrelated desktop edits are unchanged. No migration, desktop/native checks or
live provider calls; whole-project orchestration remains ANA-04/05 work. Local
commit evidence: `.mini-orca/autopilot/ux/ANA-03/receipt.json`.

**Initial ANA-04 blocker — 2026-09-11 (resolved below):** The durable controller, full inventory/preflight,
bounded stages, controls, recovery and workspace restoration are implemented but
uncommitted. Initial verification exposed missing policy-exclusion accounting;
correction 1 fixed it. Correction 2 fixed stale fault cancellation and serialized
Overview's project/run read. Focused/full Go tests, race tests, formatting and vet
then passed. A final added regression reproduces a remaining defect: after a
report is published but its progress save is lost, resume marks a stage Failed
when its attempt allowance is exhausted, before reusing the matching cached
report. The report remains intact; coverage incorrectly becomes Partial.
The exact prescribed suite now fails that regression. Two corrections are used,
so the scheduler is Paused and ANA-04 remains unchecked. The proposed one-condition
cache-recovery patch is saved but not applied in the ignored ANA-04 evidence
directory. Details are in docs/tasks.md and docs/errors.log. All ten unrelated
desktop files and the empty index are preserved; no later task started.

**Accepted ANA-04 — 2026-09-11:** Added whole-project inventory and request
preflight, bounded sequential execution, durable progress and attempts, explicit
pause/resume/cancel, and recovery without restarting requests or restoring consent.
Source/project changes invalidate publication; Overview restores matching progress.
The user-authorized additional correction lets a matching cached report finish a
stage with zero remaining transport attempts. The reproduced recovery test and
the full prescribed race-enabled suite pass (5.571s); all Go tests, race tests,
formatting, vet and final diff checks pass. Two earlier corrections plus one
explicitly authorized correction are recorded, with failed evidence retained.
The local commit includes only ANA-04 and its records; all ten unrelated desktop
edits are preserved. No configuration/data migration, desktop/native checks or
live provider calls. Unified route and legacy-owner migration remains ANA-05.
Receipt: `.mini-orca/autopilot/ux/ANA-04/receipt.json`.

## Definition of done

- Model explanations have an obvious summary, visible hierarchy and readable detail;
  the current dark style and source-first editor remain recognizable.
- One Start analysis entry point always targets the whole project. Analysis only
  starts/tracks the run; separate Bugs, Performance and Security pages present
  results with honest evidence, provenance and freshness. File filtering never
  silently changes run scope or sends model requests.
- A user can find New function without the command palette and create a function
  in an existing Go file through preview, validation/checks and explicit Apply.
- Problems, Checks and Output no longer occupy the bottom area; their necessary
  information is reachable in the result pages, Analysis progress, Assistant,
  Review or status detail.
- The sole bottom tool is a functioning interactive terminal, with correct focus,
  resize, project binding and teardown; no placeholder terminal is shipped.
- Required automated checks and supported-host native checks have recorded results;
  old report/triage data and user layout preferences are handled deliberately.

## Planning and scheduler setup validation

Repository inspection covered the current working-tree UI, request/state flows,
API routes, report/schema stores, generation/composition/Apply boundaries, job
controllers and existing tests. The original planning step changed only this file.
Scheduler setup moves the 17 task cards into `docs/tasks.md`, archives the completed
cleanup cards and updates `tasks/README.md`; it changes no application code.
Task blocks, dependency order, target paths, Markdown links and `git diff --check`
passed setup verification. At setup all 17 cards were unchecked, with UX-01 first. The
registered heartbeat was read back with the correct task, 20-minute cadence and
Active state. Installed Java 21/25 launcher/toolchain binaries passed version
checks. The ten pre-existing modified desktop files remain unchanged by setup.
Application tests and native terminal compatibility tests are deferred to the
implementation tasks; no runtime behavior or platform compatibility is claimed
from this analysis.

---

## Previous implementation status — retained history

The completed-work ledger below records the earlier accepted scope. The accepted product remains a local-first
assistant for one project, file and symbol, with explicit candidate review and Apply.
REL-01 and REL-02 are **Complete** for the documented limited release scope.
The completed cleanup specification is preserved in
[cleanup history](docs/history/cleanup-tasks-2026-09-10.md), including its execution boundaries.

## Cleanup execution — authorized 2026-09-10

The user authorized background implementation of the cleanup specification in [docs/tasks.md](docs/tasks.md). **Mini-Orca cleanup agents** (mini-orca-cleanup-agents) is now **Paused** after completing all twelve tasks; it ran every 20 minutes and processed at most one task per wake, sequentially, with one writer, fresh independent review, and coordinator-run verification. This is separate from the historical dispatcher and supersedes planning-only wording for CLN tasks only.

Use the current checkout on codex/autopilot and preserve unrelated edits. On 2026-09-10 the user explicitly authorized committing CLN-01 through CLN-03 together, then one local commit per accepted task. The coordinator alone commits after independent review and validation, including only the task changes and its ledger/checklist updates. This supersedes the initial no-commit policy; resets, stashes, pushes, releases, and live Mini-Orca evaluation calls remain outside scope. Historical insight qualification remains on standby and its budgets/evidence are unchanged. Pause the cleanup automation on an unresolved blocker or when the queue is complete.

| ID | Task | Status |
| --- | --- | --- |
| CLN-01 | Make the reachability gate enforce its result | Complete |
| CLN-02 | Remove confirmed unused desktop methods | Complete |
| CLN-03 | Reuse durable storage for evaluation metadata | Complete |
| CLN-04 | Handle Analyze-all persistence failures explicitly | Complete |
| CLN-05 | Handle Performance persistence failures explicitly | Complete |
| CLN-06 | Preserve asynchronous scan failure diagnostics | Complete |
| CLN-07 | Share file-analysis validation and evaluation diagnostics | Complete |
| CLN-08 | Move evaluation tooling out of package app | Complete |
| CLN-09 | Give benchmark operations one desktop owner | Complete |
| CLN-10 | Give security operations one desktop owner | Complete |
| CLN-11 | Separate current documentation from historical execution records | Complete |
| CLN-12 | Validate the cleanup as one release-preserving change | Complete |

**Current:** CLN-01 through CLN-12 Complete. **Active writer:** none. **Next task:** none. Cleanup automation **Paused** after final acceptance.

**Accepted CLN-12 — 2026-09-10:** Aggregate writer review of fe4cdad → 4c1c904 found no actionable code issue, removed test coverage, accidental external API, or source/privacy regression. Ownership moves are distinguished from removal: presenter 1,952 → 1,616 lines with 457 lines in two owners; evaluation/locks moved from app to insighteval; four unused methods and duplicate assessment/replaceable-metadata mechanics removed. Go Test functions 374 → 400 and desktop tests 327 → 351; all baseline evaluation tests remain. No production corrections or configuration/data migration.

Writer and coordinator exact supported validation passed all nine gates; writer diff check passed. Writer Go packages: seven executed/four cached; coordinator: all 11 cached. Race packages were all cached in both runs; daemon contracts were fresh for writer, cached for coordinator. Each Python run discovered 55 tests, executed 54 and skipped opt-in runtime conformance; the sealed-fixture Go skip remains. Desktop static/test tasks were up-to-date, with retained 351 passing tests. `createDistributable` passed with three executed/five up-to-date tasks.

Coordinator native smoke used the current macOS arm64 app bundle/JBR 25 with a synthetic loopback service and in-memory preferences. It observed compact 800 × 600 drawers and wide 1,336 × 768 docked panes, Ctrl keyboard navigation/validation, read-only source/diff labels, one-attempt Security consent with both reports retained, and benchmark GET-only listing followed by explicit trust/run requests and an unavailable synthetic result. Source hash was unchanged; no Apply/Undo, real provider, project execution or benchmark measurement occurred. Both owned processes stopped. Direct Java attachment and intermittent CUA window/capture operations failed; the normal bundle worked. Cmd mappings, exact 1000/999dp/scaling boundaries, editing resistance and spoken VoiceOver remain unverified. This does not establish production-daemon end-to-end behavior or expand release acceptance. Evidence: `.mini-orca/autopilot/cleanup/CLN-12/` (`writer/`, `coordinator-validation.json`, `native/smoke.json`, `native/requests.jsonl`). Fresh independent review approved the frozen candidate with no actionable findings; coordinator acceptance is complete.

The post-smoke validation rerun failed only formatting of the ignored synthetic Go fixture; all other gates passed. The stopped fixture was archived under a fixture extension without changing its source bytes/hash. No production correction or gate change was needed. Failed-run and scratch-repair evidence is retained. Both the repaired rerun and the final frozen-candidate run passed all nine gates. The final run used cached Go/race/daemon results and up-to-date desktop tasks; Python executed 54 of 55 discovered tests with one opt-in skip in 55.174 seconds. Diff and all 19 Markdown link checks passed. Candidate SHA-256: 70652874d9b6981aed0eac2c859f64974b3117f469f32968d5f937c75a38231b. Evidence: .mini-orca/autopilot/cleanup/CLN-12/acceptance.json.

The proposed combined per-file analysis contract remains a separate product decision. Cleanup does not reopen insight qualification, expand release claims, consume grants or expose sealed holdout material. All twelve cleanup tasks are accepted and the cleanup scheduler is Paused; the historical dispatcher and model evaluation remain on standby.

**Accepted 2026-09-10:** CLN-11 separates current plan/feature/agent guidance from inactive improvement and evaluation history, retaining complete prior instructions, recovered fe4cdad:docs/tasks.md provenance, accepted evidence, status rows, failed/unrun verdicts, budgets and holdout limits. Fresh review found no issues and independently verified all 103 anchor mappings. Coordinator passed exact Go documentation checks, 55 Python tests (one opt-in runtime conformance skip), diff checks, all 19 Markdown link checks and mechanical historical preservation. No repairs, configuration/data migration or live evaluation; release edits are navigation-only. Candidate SHA-256: ec33a5d8468e7776685eba02267f42269c686c153c50877abe5e2ca095478c50. Evidence: .mini-orca/autopilot/cleanup/CLN-11/acceptance.json. Full supported validation remains CLN-12.

**Accepted 2026-09-10:** CLN-10 moves security request ownership into DesktopSecurityWorkflow with live controller state, generation checks, per-attempt consent, report/source validation, and lifecycle cancellation. Replacement cancels the prior section while retaining evidence; presenter navigation and benchmark ownership remain intact. Fresh final review found no actionable issues; coordinator executed 68 focused and 351 full desktop tests with no failures/errors/skips, passed desktop quality (up-to-date after writer execution), and verified frozen hashes and unchanged outside-scope files. Two test corrections: structured error fixtures, then deterministic scheduling and endpoint fixture repair for an existing chat-consent test that failed the first coordinator full run. All consent assertions retained; failed-run evidence preserved. No configuration/migration or native/live execution claims; Go checks not run for desktop-only changes. Candidate SHA-256: 2fef2844a5546f776b7a794a105e4c069c8fcd171ec69772477f0e2e9f1b27b8. Evidence: .mini-orca/autopilot/cleanup/CLN-10/acceptance.json.

**Accepted 2026-09-10:** CLN-09 moves benchmark request jobs, generation/identity checks, consent, and lifecycle cancellation into DesktopBenchmarkWorkflow, using the existing authoritative controller state and event sink. Presenter entry points remain delegates. Fresh review found no actionable issues; coordinator executed 70 focused and 337 full desktop tests with no failures/errors/skips, passed desktop quality (up-to-date after writer execution), and verified the frozen candidate and unchanged outside-scope files. No repairs, configuration changes, or migrations. No live benchmark/provider execution or native visual acceptance; Go checks not run for desktop-only changes. Candidate SHA-256: fb4d5e68d11a0cdacb268aa4dbe250d41938d9311f340429839015241a2e585a. Evidence: .mini-orca/autopilot/cleanup/CLN-09/acceptance.json.

**Accepted 2026-09-10:** CLN-08 moves evaluation/campaign ownership and locks into internal/insighteval, retaining the narrow app adapter, CLI/wire identities, fixed grants, private artifacts, and fixture locations. Fresh review found no actionable issues and verified all 63 moved tests remain. Coordinator passed both exact commands, full Go/race tests, formatting, vet, native and Windows/Linux compile checks for both executables, and normalized production/dependency checks. No repairs or runtime platform claims; no live evaluation or qualification. Candidate SHA-256: e69a984329d26204ee629c89c4800949fb695b1e7d4f5706cd9a9612d4d51b3b. Evidence: .mini-orca/autopilot/cleanup/CLN-08/acceptance.json.

**Accepted 2026-09-10:** CLN-07 shares semantic validation and source-free diagnostics through a narrow evaluation adapter, removing duplicate runner decoding and optional validation. Synthetic parity tests preserve prompt/schema/version, provider classification, parent rejection, optional degradation/retention, and private diagnostic classifications. Fresh review found no actionable issues; coordinator passed exact focused tests, all Go tests, full race, formatting, vet, Go quality, and diff checks. One test-fixture correction; no migration or live evaluation calls. Candidate SHA-256: faf6967551c9887d18e27eecb9e2ea8b6c4d44d0fe65b6664b77a964f9625250. Evidence: .mini-orca/autopilot/cleanup/CLN-07/acceptance.json.

**Accepted 2026-09-10:** CLN-06 logs bounded, sanitized secondary failure-report persistence errors while retaining the original scan failure in memory and existing durable progress. Tests verify unchanged disk bytes, secret/path redaction, and replacement project/revision isolation. Fresh review found no actionable issues; coordinator passed focused GoScan race tests, all Go tests, full race, formatting, vet, Go quality, and diff checks. One test-fixture correction; no migration or live evaluation calls. Candidate SHA-256: 91b144d8d0512d240445c7b525d842041fa63d61e1432af408956f75381a6ae6. Evidence: .mini-orca/autopilot/cleanup/CLN-06/acceptance.json.

**Accepted 2026-09-10:** CLN-05 stops Performance work on save failures, exposes durable progress, preserves reports/accounting, and supports explicit recovery with project-scoped detached failures. Following user-authorized continuation, one repair resolved the concurrent Resume/result-write fault by retaining persistence authority through recovery and releasing it for worker join. Fresh review found no findings; coordinator passed focused and full Go tests, race tests, formatting, vet, Go quality, and diff checks. Historical blocked evidence and two earlier preparation corrections remain recorded; no migrations or live evaluation calls. Candidate SHA-256: d7f76368f095939c4dd049b0d92aaf6b1855b9963e2fc182d39b5b9296c290a8. Evidence: .mini-orca/autopilot/cleanup/CLN-05/acceptance.json.

**Accepted 2026-09-10:** CLN-04 stops Analyze-all dispatch on persistence failure, retains sanitized progress errors and completed reports, and requires durable explicit recovery while preserving attempt budgets and replacement identity. Fresh review found no actionable issues; coordinator passed exact Analyze-all race tests, all Go tests, race tests, formatting, vet, Go quality, and diff checks. Two focused preparation corrections; no migrations or live evaluation calls. Candidate SHA-256: 79efd26ba3059477fe28f5ce3e9c275bea26aa2cd6041a618b597af5c9d6ff7a. Evidence: .mini-orca/autopilot/cleanup/CLN-04/acceptance.json.

**Accepted 2026-09-10:** CLN-03 reused shared atomic storage for evaluation metadata, preserving sanitized errors and immutable evidence. Fresh review found no actionable issues; coordinator passed exact focused tests, all Go tests, race tests, formatting, vet, Go quality, and diff checks. No repairs, migrations, or live evaluation calls; accepted before the later batch-commit authorization. Candidate SHA-256: 6267407558d67a09c17753e26526274c205343bdbcd7fed52336e7958b6ae9a8. Evidence: .mini-orca/autopilot/cleanup/CLN-03/acceptance.json.

**Accepted 2026-09-10:** CLN-02 removed four unreferenced desktop methods and redundant nullable test operations. Fresh review found no actionable issues. Writer executed desktop tests (327 passing) and quality; coordinator reran both exact commands successfully (up-to-date) and verified test XML and diff checks. No repairs; accepted before the later batch-commit authorization. Candidate SHA-256: 1713a9ddcb438a3e21037d8d47b8e8b163740c4955156535ef09422c0acd5817. Evidence: .mini-orca/autopilot/cleanup/CLN-02/acceptance.json.

**Accepted 2026-09-10:** CLN-01 makes reachability findings and tool failures fail the gate while preserving later stages. Fresh review found no actionable issues; coordinator reran 4 isolated Python tests, all four Go quality stages, shell syntax, and diff checks successfully. No repairs; accepted before the later batch-commit authorization. Candidate SHA-256: 6c04a707f8c7c85ad4e75d86a3b72327c7ad4ed173474ff0389b8676d102e96d. Exact evidence: .mini-orca/autopilot/cleanup/CLN-01/acceptance.json.

## Current scope decision — 2026-09-09

The user placed engineering-insight improvement and qualification on **standby**.
QUAL-06, RCV-07 and RCV-08 are deferred, not passed; their historical failed or
unrun verdicts remain intact in the historical records. This decision supersedes earlier instructions
that make insight qualification a prerequisite for REL-01 or resume recovery.
Completed insight implementation remains available, with content quality explicitly
unqualified; this documentation change does not disable the feature.

The user subsequently requested closure of **REL-01 → REL-02**. REL-01 accepted
the documented limited release scope, preserving source safety, consent, provider integration,
native UI and distribution requirements. Insight usefulness/omission qualification
is excluded from this release acceptance scope; neither insight quality nor broader
AI factual reliability may be claimed validated by this deferral. REL-01 records
local acceptance separately; publishing remains outside this work.

The historical dispatcher remains **Paused**; the cleanup scheduler above is also **Paused** after completion. No additional model calls or recovery attempts
are authorized. Preserve cumulative consumption of **60 development / 24
qualification requests**, all receipts and grants, and the sealed, unused conditional
24-case holdout. Reopening insight work requires a new user decision and explicit
experiment scope/budget; do not replay the failed runs or weaken their gates.

## Task ledger

The delivered scope and standby tasks retain their accepted statuses below.
These rows are not an active dispatcher queue. Detailed completed task instructions,
original dependencies, dated verdicts and the [old-to-new anchor map](docs/history/improvement-plan-2026-09.md#old-to-new-anchor-map)
are in [improvement history](docs/history/improvement-plan-2026-09.md).
Cleanup statuses are maintained in the table above; checkboxes in docs/tasks.md
mirror acceptance only after independent review and coordinator verification.

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
| AUTO-03 | Repair dispatcher startup diagnostics and explicit recovery | AUTO-02 | M / high | Complete |
| AUTO-04 | Report tokens without blocking and escalate Terra repairs to Sol | AUTO-03 | M / high | Complete |
| AUTO-05 | Isolate validation scratch and preserve failure evidence | AUTO-04 | S / high | Complete |
| REC-01 | Preserve explicit zero-temperature provider requests | QUAL-04, AUTO-05 | S / medium | Complete |
| REC-02 | Account for one model-bound six-request recovery grant | REC-01, QUAL-03 | M / high | Complete |
| REC-03 | Prepare and verify the frozen local reasoning candidate | REC-02 | M / high | Complete |
| REC-04 | Establish standalone runtime compatibility | REC-03, QUAL-03 | S / high | Complete |
| REC-05 | Prepare bounded standalone runtime lifecycle | REC-04 | M / high | Complete |
| REC-06 | Verify reasoning and final-schema boundary offline | REC-05 | M / high | Complete |
| REC-07 | Authorize six runtime-bound development slots | REC-06 | M / high | Complete |
| QUAL-05 | Repeated six-request recovery pilot and candidate freeze | QUAL-04, REC-07 | S / high | Complete |
| QUAL-06 | Independent 24-request qualification verdict | QUAL-05 | M / high | Blocked (standby) |
| RCV-01 | Source-free optional-output rejection diagnostics | QUAL-05 | S / high | Complete |
| RCV-02 | Generation field bounds aligned with parser | RCV-01 | S / high | Complete |
| RCV-03 | General source-grounded explanation contract | RCV-02 | S / high | Complete |
| RCV-04 | Broader development and sealed fresh qualification | RCV-03 | M / high | Complete |
| RCV-05 | Fixed successor accounting and v2 gates | RCV-04 | M / high | Complete |
| RCV-06 | Offline runtime validation and candidate freeze | RCV-05 | M / high | Complete |
| RCV-07 | Twelve distinct development requests | RCV-06 | M / high | Blocked (standby) |
| RCV-08 | Conditional fresh 24-request qualification | RCV-07 | M / high | Blocked (standby) |
| REL-01 | End-to-end, native and distribution acceptance | AUTO-01, UI-04, LEARN-02, SEC-01, SEC-08, PERF-03 | M / high | Complete |
| REL-02 | Final code/doc retirement and handoff | REL-01, AUTO-02 | S / medium | Complete |

## Maintained documentation

- [Usage](README.md), [configuration](CONFIG.md), [API contract](docs/api-contract.md)
  and [desktop runtime](desktop/README.md) describe supported behavior.
- [UI guidelines](desktop/UI_DESIGN_GUIDELINES.md) and
  [insights and Performance](docs/insights-performance/README.md) describe current features.
- [Release acceptance](docs/RELEASE_ACCEPTANCE.md) owns release evidence and limits;
  macOS arm64 Desktop and the Linux arm64 container are the accepted platforms.
  Remote/mixed live compatibility, broader AI factual reliability, unsupported native
  combinations and spoken screen-reader output remain unclaimed.
- [Improvement history](docs/history/improvement-plan-2026-09.md) retains the prior
  plan and paused dispatcher instructions. [Insight history](docs/history/insight-evaluation-2026-09.md)
  retains runtime/campaign commands and **fe4cdad:docs/tasks.md** recovery provenance.

Cleanup requires no configuration or data migration. No publishing, release,
new provider requests or insight recovery is authorized by this documentation.
