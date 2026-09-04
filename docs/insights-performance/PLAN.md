# Engineering insights and project performance analysis

**Status:** Ready for sequential execution — Tasks 133–139 are prepared; implementation has not started.

**Prepared:** 2026-09-04.

**Dependency:** IDE UI Tasks 118–132 are Complete; reverify Task 132 and its recorded acceptance limitations before implementation.

**Scope:** Go daemon, loopback API, Kotlin/Compose Desktop, tests, and maintained documentation.

This is a follow-on to the [IDE UI plan](../../plan.md), not a replacement for it.
On the current case-insensitive filesystem, root `PLAN.md` and `plan.md` resolve
to the same file. This separate directory preserves the completed IDE plan.
Do not modify its task sequence or close its outstanding acceptance work while
implementing this feature. Re-read the completed IDE implementation before editing.

## 1. Product outcome

Add two focused capabilities:

1. A small **Engineering insight** panel within the page where the user is already
   reading an analysis, inspecting a bug, or reviewing a proposed improvement.
   The panel can be closed and reopened without losing the result.
2. A distinct **Performance** section beside Analysis for an explicit,
   project-wide review of potential performance problems and optimization
   opportunities.

The intended user is an experienced developer, not a beginner. An insight explains
something non-obvious about the current code: a mechanism, hidden assumption,
trade-off, failure mode, or principle that transfers to other engineering work.

### Confirmed exclusions

- No games, quizzes, challenges, exercises, scores, streaks, or gamification.
- No learning profile, mastery tracking, recall queue, curriculum, or study journal.
- No separate Learn workspace, right-side Learn tab, or new tutorial mode.
- No compulsory reflection, reading confirmation, or additional Apply gate.
- No verbose lessons, introductory syntax explanations, or generic best-practice filler.

### Performance scope decision

The first release is a **source-based, AI-assisted performance review**, not a
runtime profiler. It can identify plausible inefficient patterns, explain the
conditions under which they matter, and propose verification steps. It cannot
establish measured bottlenecks, latency, allocation rates, throughput, or speedups.

The UI must say **Source-based review · Not measured**. Benchmark execution,
profiling, application startup, production telemetry, and profile import are
outside this plan. Adding measured performance later requires a separate scope
and execution-safety design; do not quietly implement those features here.

## 2. Product and safety invariants

- Preserve the finished IDE shell, Focus Flow palette, keyboard navigation, and
  docked/drawer boundary at exactly `1000dp`.
- Source and diff remain read-only. One project, one file, and one declaration
  remain the mutation boundary, even when a report covers many files.
- Generation remains preview-first. Validation, checks, draft identity, explicit
  discard, Apply, receipt, and Undo retain their existing authority.
- Viewing a page, selecting a row, or opening/closing an insight never initiates a
  provider request, source write, command, check, or repair.
- Analysis and Performance are independently requested and independently cached.
  Import, restore, reindex, normal Analyze-all, and opening Performance do not
  start performance analysis.
- Preserve loopback defaults, context-policy exclusions, bounded prompts, safe
  indexed paths, and request-specific remote-provider confirmation.
- Treat source text and model output as untrusted. Repository comments cannot
  change the prompt contract or authorize additional context or actions.
- A model interpretation is not a verified diagnostic or measured performance
  result. Passing correctness tests is not evidence of a performance improvement.
- This plan authorizes future feature work only when explicitly executed. Creating
  this document does not implement features or authorize commits.

## 3. Engineering insight content contract

### 3.1 What qualifies

| Result | Useful insight |
| --- | --- |
| Bug or risk | Explain the causal mechanism, why the defect is easy to overlook, or the invariant that would prevent recurrence. |
| Proposed improvement | Explain the design pressure, strongest relevant alternative, and cost or condition that makes the change worthwhile. |
| File/project analysis | Explain non-obvious coupling, ownership, consistency, resource lifetime, or architectural consequences supported by supplied context. |
| Generated declaration | Explain a consequential implementation decision without repeating the change summary. |
| Performance finding | Explain why work grows, resources accumulate, or contention occurs, including workload assumptions and the optimization trade-off. |

An insight is optional. If no sufficiently useful observation exists, omit the
panel instead of inventing sophistication. Do not turn every minor issue into a
lesson. Aim for one concise paragraph of roughly 50–90 words, not an essay.

Example, illustrating the tone rather than claiming a detected defect:

> A content hash is part of this cache entry's identity, not merely an invalidation
> hint. Checking it again before publishing prevents a slow result from becoming
> authoritative after the file changes. A timestamp comparison is simpler, but
> does not establish that the result belongs to the source currently being viewed.

### 3.2 Structured optional payload

Use one shared value type for an optional `engineering_insight` field:

```text
EngineeringInsight
  mechanism                  required concise string
  why_it_matters_here         required concise string
  tradeoff_or_failure_mode    optional concise string
  transferable_lesson        optional concise string
```

- Require nonblank required fields after trimming and cap the combined text at
  1,000 Unicode code points. Optional fields may be absent; do not force four
  redundant sentences. Keep existing whole-response byte limits as well.
- The model supplies explanatory text only. It cannot supply trusted project IDs,
  file hashes, draft authority, URLs, actions, or arbitrary navigation targets.
- The owning report/finding/proposal supplies scope, provenance, timestamp, and
  freshness. Reuse that identity instead of creating a second insight database.
- An absent or `null` insight is valid. A malformed optional insight is omitted
  with a bounded, sanitized diagnostic; it must not discard otherwise valid code
  or analysis. Invalid required parent content still fails normally.
- Keep unknown-field rejection for the parent output contract. Parse the optional
  insight independently so its failure policy is explicit and testable.
- Present claims as AI interpretation. A valid line reference proves only a valid
  navigation anchor, not the truth of the explanation. Do not fabricate citations.

### 3.3 Attach the insight to its actual owner

- Project report: one optional project-scoped insight, visible with that report.
- File report: an optional file-level insight and optional insights on its risks
  and suggestions. Limit the total to three insights per file-analysis response.
- Bugs/Problems: carry the originating risk's insight through existing finding
  conversion. Do not attach an unrelated file insight to a particular bug.
- Symbol Context: use a matching result when one exists. A file-level insight
  remains explicitly labelled as file-scoped, not as a symbol-specific claim.
- Draft proposal: attach one insight to the assistant proposal and its exact draft
  ID, revision, and hash. Assistant and Review read that same value.
- Performance finding: attach its optional insight to that finding.
- Deterministic parser/vet/test failures receive no fabricated AI explanation.
  Existing generation or explicit repair may return a related insight later;
  checks alone do not trigger an extra model request.

Do not add insight text to finding IDs: explanatory rewording must not create a new
bug or reset triage. Keep insights out of Apply eligibility and audit authority.
Do not automatically copy insight text into generated source comments.

## 4. Insight panel UI/UX

Use one reusable compact `EngineeringInsightPanel` embedded in the relevant page
or existing tool window. Do not reserve another full-height pane or show several
insight cards alongside every unselected result.

```text
Collapsed
  > Engineering insight                         AI interpretation

Expanded
  v Engineering insight                         AI interpretation
  One short paragraph explaining the non-obvious mechanism,
  its relevance here, and the important trade-off.
                                                   Close insight
```

### Placement

| Surface | Placement |
| --- | --- |
| Summary / project analysis | Immediately below the displayed model interpretation. |
| Analysis / file Context | Below the relevant file result or selected suggestion. |
| Bugs / bottom Problems | In the selected finding's details, not every list row. |
| Assistant | Beneath the current proposal explanation, outside the declaration editor. |
| Review | A secondary section below candidate explanation/evidence; never between the Apply button and its safety conditions. |
| Performance | Below the selected performance finding's rationale. |

### Interaction rules

- Default to collapsed with an obvious text-labelled opener. Both the header and
  **Close insight** toggle the same state; closing never deletes the content or
  hides the opener.
- Remember the user's expanded/collapsed preference in presentation settings.
  One preference is sufficient; do not persist an unbounded map of result IDs.
- A new result must not force a closed panel open, change tabs, move keyboard
  focus, or switch the editor from Source to Review.
- Opening/closing is instant local presentation of already returned content.
  Missing content is omitted; there is no misleading Generate button or empty
  learning placeholder.
- Use normal text wrapping and selectable text. Aim for a short block, not a fixed
  height that clips at text scaling. Reuse the existing page/tool-window scroll.
- Preserve existing color tokens and compact density. Do not add celebratory icons,
  progress badges, ratings, feedback buttons, or preference onboarding.
- Expose expanded/collapsed semantics and keyboard activation with Enter/Space.
  Closing returns focus to the opener. No hover-only essential controls.
- Below `1000dp`, the panel stays inside its page or existing right drawer; no
  separate insight drawer, modal, overlay, or floating window.

### Freshness

- Bind file insights to the owner's project revision and file hash; bind candidate
  insights to the exact draft revision/hash, not only its draft ID.
- On manual draft edits, do not present the previous AI explanation as describing
  the edited candidate. Hide it from current Review or label it historical in the
  proposal history; do not regenerate automatically.
- Stale report insights may remain readable with **Outdated — source changed**.
  Do not navigate using stale line numbers as if they were current evidence.
- Project switches and late/canceled responses must clear or reject incompatible
  insight owners. The expanded preference may persist; old content may not.

## 5. Performance section

### 5.1 Navigation and layout

Add **Performance** adjacent to **Analysis** in the finished IDE navigation and
command search. It is a project-level page, not a renamed bug filter or another
bottom tool-window tab. Keep Context/Assistant/Review and Problems/Checks/Output
unchanged in responsibility.

```text
PERFORMANCE                         Source-based review · Not measured
Project: mini-orca                   Analysis model · Local/Remote destination

[Analyze performance] [Cancel when running] [Run limits]
Reviewed 34 / 100 selected files · 260 eligible · 160 outside this run
Current run: Running / Completed / Partial / Failed / Canceled / Stale

Category      Potential impact   Confidence   Location          Opportunity
I/O           High               Medium       store.go:84       Repeated reads
Memory        Medium             Medium       cache.go:51       Unbounded retention

SELECTED OPPORTUNITY
Observed pattern · When it matters · Proposed improvement
Trade-off · How to verify · Scope/coverage limitations
> Engineering insight
[Open in Editor] [Prepare optimization, when eligible]
```

- Keep the toolbar compact, with limits in a disclosure and related controls
  wrapping at narrow widths. Use existing row, empty-state, and focus components.
- The first visit shows an explicit explanation and Analyze performance action,
  not a spinner for an analysis that the user did not request.
- Row selection displays details in the same page. **Open in Editor** is explicit
  navigation and retains the existing draft-discard decision if the target changes.
- Provide category and potential-impact filters, plus a project-relative path
  filter. Use stable ordering: potential impact, confidence, path, line, ID.
- On narrow screens, render selected details below the list with page scrolling;
  do not compress a permanent three-column report.
- Preserve all existing `Cmd/Ctrl+1`–`4` destinations. Do not derive those shortcuts
  from an enum position after adding Performance. Add command-search access;
  no extra numbered shortcut is required.
- Extend explicit workspace cycling, navigation mapping, provider presentation,
  status clearing, focus handling, and tests for the additional section.

### 5.2 Review categories

| Category | Examples of review targets; not automatic proof of a problem |
| --- | --- |
| CPU / complexity | Repeated work, avoidable scans, nested data-dependent loops, unnecessary parsing or serialization. |
| Memory | Retained references, growing caches/collections, avoidable copying, allocation-heavy paths. |
| I/O | Repeated disk access, request-per-item patterns, chatty database/network operations, missing batching. |
| Concurrency | Long critical sections, blocking under locks, unnecessary serialization, unbounded work creation. |
| Caching | Cache identity, missing bounds, invalidation cost, duplicate computation, unsuitable cache trade-offs. |
| UI / rendering | Repeated expensive derivation or rendering work where relevant source and framework context are available. |

Review eligible source across the project's languages. Exact mutation support
remains Go-first. For unsupported or approximate symbols, retain useful read-only
source review and label its limitations rather than claiming exact analysis.

Do not infer complexity merely from syntax, equate file size with runtime cost, or
assume every loop is a hot path. Cross-file call frequency, production data size,
database plans, scheduler behavior, and actual resource use are usually unknown.

### 5.3 Finding contract

A performance finding contains:

- a server-derived ID and owner identity;
- category and title;
- **potential impact** (`high`, `medium`, `low`, or `unknown`), not measured severity;
- model confidence (`high`, `medium`, or `low`), not a probability or tool verdict;
- a concrete source-observed pattern and validated indexed location;
- the workload/input conditions under which it could matter;
- a proposed improvement and its trade-off;
- a concise verification plan describing what to measure or compare;
- optional Engineering insight;
- source-based/AI provenance and freshness.

The server binds the path and hash to the reviewed file. Validate returned line
ranges against that file and symbol names against the supplied symbol inventory.
Discard an invalid location/finding with a sanitized diagnostic, never retarget
to another file. A deliberately returned empty findings array is a valid result.
If validation drops findings, expose a partial/unusable-output warning; if every
item in a nonempty response is invalid, fail that file's review rather than
reporting a successful empty assessment.

Do not show numeric project performance scores, estimated milliseconds saved,
invented benchmark charts, speedup percentages, or a green "project is fast"
verdict. Use **No opportunities identified in the reviewed files**, with coverage
and limitations, rather than **No performance issues**.

### 5.4 Safe optimization handoff

**Prepare optimization** is available only for a fresh finding with one eligible
exact Go declaration. It prefills the existing bound request/composer with the
observed pattern, desired behavior, and trade-off; it does not call the model.
Reuse target validation and draft-discard handling instead of creating a parallel
optimization editor or a new Apply path.

Cross-file opportunities and non-Go/approximate targets remain analysis-only.
Never split a report recommendation into automatic multi-file edits. Existing
correctness checks remain required; neither Apply nor passing tests marks an
opportunity as a verified performance win. A new explicit review can reassess
source changes, but measurements remain unavailable in this release.

## 6. Performance analysis pipeline

### 6.1 Explicit bounded project review

1. Read the current project/index snapshot and the shared ContextPolicy.
2. Preview eligible files, excluded/skipped counts, selected queue, limits, provider,
   and the fact that each call carries one file plus bounded source-free facts.
3. Start only after explicit user action and, for remote providers, confirmation
   for this exact run scope. Reject a changed project/revision/policy preview.
4. Process the selected queue sequentially with one bounded performance prompt per
   file. Do not call normal bug analysis to simulate a performance result.
5. Validate and persist each completed file report independently. Recheck source
   identity and the active job ID/generation before publishing; a late response
   from a canceled run cannot overwrite a newer run even at the same revision.
   Keep valid partial results if a later file fails.
6. Build the project report deterministically from the run's current file reports,
   category totals, and coverage. Do not require a second whole-project LLM pass.

This is a project-wide aggregation of bounded file reviews. It does not establish
whole-program hot paths or runtime behavior from filenames and signatures alone.
Only supplied evidence may support cross-component hypotheses, and their missing
runtime/call-graph context must be stated.

### 6.2 Limits, lifecycle, and concurrency

- Reuse the current Analyze-all file-count defaults/caps where practical (currently
  default 100, maximum 500), but keep separate jobs and cache namespaces.
- Apply a per-file source ceiling of 64 KiB and the configured input-context limit,
  whichever is tighter after reserving instructions/output. Skip oversized files
  with an explicit reason rather than silently truncating an apparent full review.
- Cap each model response at 64 KiB and five findings. Preserve stricter configured
  provider output limits. Bound individual finding fields and report/API sizes.
- Add a total execution-time budget: default 15 minutes, maximum 60 minutes.
  Display it before starting. Paused idle time does not consume it, but Resume and
  restart must not reset spent execution time or attempt counters. Budget exhaustion
  yields truthful partial coverage, not a successful full-project assessment.
- Reuse retry/backoff policy with one owner of retry attempts. Do not multiply job
  retries by hidden per-request retry loops. Persist attempts and terminal errors.
- Support Start, Pause, Resume, and Cancel using the existing lifecycle conventions.
  Pause stops before the next file; Cancel cancels the active request. Resume
  requires explicit action and renewed remote confirmation where applicable.
- Store captured project ID/revision, root identity, policy version, options, queue,
  provider/model/prompt identity, file hashes, progress, and start/update times.
- A source/revision/policy change invalidates the captured run; stop rather than
  mix revisions. Completed old results may remain visible as stale history.
- Restart never resumes provider work automatically. Restore interrupted progress
  as paused/interrupted with an explicit Resume or Start new review action.
- Permit one Performance job at a time. For v1, reject starting a second background
  project-wide review while Analyze-all or Performance is active; do not silently
  cancel the other job. Keep file navigation and reading cached results available.
- Handle project switching, cancellation, and late provider results through the
  existing service/presenter ownership boundaries. A worker must retain its captured
  project root and cannot publish into a newly opened project's storage.

### 6.3 Honest coverage

Report these separately: total indexed source files, policy-eligible files,
selected files, reviewed files, reused fresh reports, failures, skipped files with
reasons, and files outside the configured run limit. Counts must reconcile.
Retained stale or earlier-run reports are not part of current-run completion.
Each queue entry records the accepted report identity, including explicitly reused
fresh reports; aggregate only entries whose recorded identity still matches.

Run completion and analysis coverage are separate concepts. A selected queue may
finish while the project remains partially reviewed. A run can finish with file
errors; **Completed with errors** is different from **Completed successfully**.
Unavailable and failed analysis must not render as an empty successful report.

## 7. Architecture, API, and storage

### 7.1 Reuse existing boundaries

Relevant starting points (reconfirm after Task 132):

- [File analysis](../../internal/app/file_analysis.go) and
  [project interpretation](../../internal/project/project_analysis_report.go).
- [Draft prompt](../../internal/app/declaration_prompt.go) and
  [chat proposal contract](../../internal/app/chat_session.go).
- [Finding reconciliation](../../internal/project/findings.go), including provenance,
  stable triage, sanitization, and stale state.
- [Analyze-all lifecycle](../../internal/app/analyze_all.go),
  [source traversal](../../internal/project/source_walk.go),
  [context policy](../../internal/project/context_policy.go), and
  [atomic metadata storage](../../internal/storage/atomic.go).
- [Desktop workflow presenter](../../desktop/src/main/kotlin/io/miniorca/desktop/DesktopWorkflowPresenter.kt),
  [state](../../desktop/src/main/kotlin/io/miniorca/desktop/DesktopState.kt), and
  [layout preferences](../../desktop/src/main/kotlin/io/miniorca/desktop/DesktopLayoutState.kt).

Use cohesive feature files for Performance state/service/report/presentation and
the shared insight value/panel. Extract shared queue/cancellation/validation logic
only where both existing and new callers truly share policy; do not copy the
Analyze-all controller wholesale or introduce a generic agent/job framework.

Performance reports remain separate from `UnifiedFinding` in v1. In particular,
do not reconcile Performance under the existing generic `ai` finding source:
that could retire ordinary bug findings. Reuse navigation/presentation primitives,
not bug lifecycle authority or persistence by accident.

### 7.2 Model configuration and privacy

- Insight generation uses its parent operation's existing model call and scope:
  project interpretation uses `analyze`, file/bug analysis uses `bug`, and proposals
  and repairs use `function`. No extra call is made when an insight opens.
- Performance v1 deliberately uses the existing `analyze` profile, labelled
  **Performance · Analysis model**. This is an explicit routing rule, not a fallback
  to another scope when configuration is missing.
- Preserve its real provider/model/context/output/timeout metadata; clamp each
  request deadline to the remaining run budget. No new required configuration
  scope, credentials, provider selector, or runtime dependency is needed.
- Preview the bounded multi-file queue before a remote run, not only the currently
  open file. Confirmation for normal Analysis or another project does not authorize
  this run. Resume must validate the same project, queue, policy, and destination.
- Keep `.mini-orca/`, ignored/generated/binary/secret-like files, symlinks, and unsafe
  paths out of prompt material through the existing policy and path boundary.
- Do not store raw source, whole prompts, unsanitized provider output, credentials,
  or diagnostic secrets in insight/performance metadata or logs.

### 7.3 Proposed API surface

Use the existing size-limited JSON requests, structured errors, and guard patterns.
The following are new routes, not claims that they already exist:

| Method | Proposed route | Purpose |
| --- | --- | --- |
| GET | `/api/projects/current/performance` | Read cached current-run report, findings, provenance, and coverage without AI work. |
| GET | `/api/projects/current/performance/context` | Preview the policy-filtered queue and provider manifest for the requested limits. |
| GET | `/api/projects/current/performance-job` | Read current/restored job progress. |
| POST | `/api/projects/current/performance-job` | Start an explicitly guarded bounded review. |
| POST | `/api/projects/current/performance-job/pause` | Pause after the current file. |
| POST | `/api/projects/current/performance-job/resume` | Explicitly resume compatible captured work. |
| POST | `/api/projects/current/performance-job/cancel` | Cancel active/pending work and retain valid completed reports. |

Start includes expected project/revision/policy identity, file/retry/time limits,
and request-specific remote confirmation. Control requests identify the expected
job and project so an old UI action cannot control a newer run. Mismatched
identity returns conflict; no active project and unsupported requests follow
existing error conventions. Bounded listing/paging must prevent large report
responses from overwhelming the Desktop.

Add optional insight fields to the existing relevant responses, not an independent
Generate insight route. Update the canonical API guide, OpenAPI, Go wire models,
Kotlin serialization models, response fixtures, and route contract tests together.
Keep existing routes and mutation contracts intact.

### 7.4 Persistence and compatibility

- Persist insights with their authoritative parent report/finding. Proposal insights
  have the same in-memory lifetime as current chat/drafts; do not introduce durable
  conversation storage solely for this feature.
- Proposed Performance storage: `.mini-orca/performance/files/<safe-path-hash>.json`
  for file reports and `.mini-orca/sessions/performance-job.json` for captured job
  progress. Use existing atomic writes and corrupt-metadata recovery conventions.
- Derive aggregate report data from authoritative file reports and the job; avoid a
  second persisted aggregate with independent freshness rules.
- Cache validity includes project/revision, file hash, context-policy version,
  model/provider/reasoning configuration, prompt/schema version, and relevant run
  options. Do not reuse normal bug-analysis cache entries for Performance.
- Older metadata without insight fields remains readable. Bump prompt/cache
  versions where generation meaning changes, mark old results appropriately, and
  refresh only through an explicit action. Never regenerate merely to populate a
  panel after upgrade.
- Version any changed model output contract deliberately. Replace prompt/parser/
  fixtures together; do not retain a parallel legacy generation implementation.
  Preserve required public API behavior and document any unavoidable migration.
- Persist only the insight disclosure preference and Performance layout/filter
  preferences in Desktop presentation storage, never report authority or provider
  confirmation. No user configuration migration is expected for this design.

## 8. Implementation sequence after IDE acceptance

Tasks 133–139 are prepared in the [task index](../../tasks/INDEX.md). Use the
[sequential execution prompt](../../tasks/PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md)
to implement them strictly in order with one reviewed local commit per task.
Creating the backlog does not itself start implementation or commits. Each slice
must leave the app usable; the task files supply scope, tests, and commit subjects.

| Task | Deliverable | Acceptance checkpoint |
| --- | --- | --- |
| 133 | Baseline finished IDE behavior; define insight and Performance contracts, scope routing, fixtures, freshness, limits, and API additions. | Tests describe the new behavior without changing existing mutation gates or starting AI on navigation. |
| 134 | Add optional insights to project/file/risk/suggestion/proposal generation and their existing transport/storage conversions. | Same parent model calls; malformed optional notes cannot invalidate a valid parent; insight wording cannot change finding identity. |
| 135 | Build reusable inline insight panel and wire Summary, Analysis/Context, Bugs/Problems, Assistant, and Review. | Close/reopen, keyboard access, persisted disclosure, stale/manual-edit cases, and narrow layouts work without a Learn section. |
| 136 | Implement bounded performance file review, validated report model, policy-safe context preview, and separate cache. | No runtime execution; grounded locations, limits, source changes, empty results, and provider confirmation are tested. |
| 137 | Implement explicit performance job lifecycle, aggregate coverage, API routes, concurrency guards, and restart recovery. | Honest partial/error states; no cross-project publication; bounded retries/time; existing Analyze-all remains independent. |
| 138 | Add Performance navigation/page, provider/coverage status, filtering, insight details, and guarded optimization handoff. | Existing shortcuts remain mapped; selecting findings is non-mutating; only eligible exact targets can prepare an edit. |
| 139 | Complete integration, accessibility, contract/privacy tests, docs, and post-IDE regression acceptance. | Full supported validation passes; manual limitations are recorded; no duplicate legacy implementations remain. |

For daemon slices, run focused tests during development and the relevant full Go
checks before handoff. For Desktop slices, use the checked-in Gradle wrapper and
the finished IDE's formatter/static-analysis/test suite. User-invoked execution of
the prepared prompt includes exactly one verified local commit per Task 133–139,
without a separate planning/status commit, amendment, or push.

## 9. Verification and acceptance

### Automated behavior tests

- Insight missing/null/valid/blank/oversized/unknown fields; malformed optional
  payload isolation; text sanitization; no invented source navigation.
- Insight propagation through project/file findings and proposal serialization;
  stable finding IDs and preserved triage after insight wording changes.
- Exact owner freshness across file switches, project switches, source changes,
  manual draft revisions, reanalysis, cancellation, and late responses.
- Expand/close/reopen and persistence; no provider/source/check/Apply side effects
  from any display action; no content leakage from the prior target.
- Performance category/impact/confidence validation; invalid paths, line ranges,
  symbols and oversized output; valid empty reports and unknown impact.
- Policy exclusions, symlink/path traversal guards, byte/token/file/time limits,
  queue preview mismatch, and remote confirmation for Start and Resume.
- Job idempotence/conflicts, retry caps, pause/resume/cancel, partial results,
  source/policy changes, restart recovery, and project-root isolation.
- Coverage arithmetic for selected/unselected/cached/skipped/failed files, stale
  caches and prior runs; no false full-project or measured-performance verdict.
- Normal Analyze-all and existing bug findings remain unchanged by performance
  analysis. Performance cannot retire bug findings or overwrite their cache.
- Old metadata without insight fields and new versioned performance metadata;
  missing/corrupt files and atomic writes without source/prompt leakage.
- New route registrations match OpenAPI and the canonical API guide. Test Go/Kotlin
  serialization, structured errors, revision conflicts, and bounded report reads.
- Existing target/discard/validation/check/Apply/Undo flow, provider scopes, source
  selection, command search, shortcut mappings, and layout preferences still pass.

### Manual UX/content acceptance

- Inspect wide, exactly `1000dp`, and narrow layouts, with long paths, long insight
  text, empty results, filtered results, stale findings, and text scaling.
- Keyboard-only navigation reaches Performance and all insight toggles. Focus
  remains predictable; collapsed controls are discoverable without hover/color.
- Read a bug, analysis result, and candidate while opening/closing their insight.
  Verify the panel is small, source remains central, and no quiz or learning UI exists.
- Start/cancel/resume a performance review; inspect partial coverage and provider
  scope; navigate a finding and prepare one eligible optimization without generating
  or applying anything until the corresponding explicit action.
- Use a small reviewed fixture set with meaningful bugs, trade-offs, performance
  patterns, and trivial changes. A human checks specificity, correctness, brevity,
  uncertainty, and whether trivial changes appropriately omit insights. Model prose
  quality must not be claimed proven by deterministic parser tests alone.
- Record unavailable interactive/provider checks honestly. Do not mark them passed
  because unit tests pass.

### Final automated checks

```sh
make fmt-check
go test ./...
make test-race
make vet
./desktop/gradlew -p desktop spotlessCheck detekt test
make check
git diff --check
```

Use the final IDE sequence's supported quality targets if additional maintained
checks were added. Do not run destructive targets or Docker cleanup.

### Definition of done

- Useful concise insight is available in a small, closeable/reopenable in-page
  panel for applicable analysis, bugs, improvements, and proposals.
- Performance is independently reachable beside Analysis and reviews the eligible
  project scope only after explicit user action.
- Every performance result has honest source-based provenance, workload conditions,
  verification guidance, and coverage; no unmeasured claim is presented as a fact.
- Safety, privacy, responsiveness, keyboard access, and existing workflow guards
  remain intact. No source writes or code execution were added by these features.
- Maintained README/Desktop usage, API/OpenAPI, configuration scope descriptions,
  keyboard checklist, and release notes describe the shipped behavior and limits.
- Tests and manual acceptance are reported; obsolete code and redundant helpers
  are removed; no credentials, generated build output, or unrelated user changes
  are included.
