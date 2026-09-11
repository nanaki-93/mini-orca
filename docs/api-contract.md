# Mini-Orca desktop API contract

**Version:** 4.4.0
**Base URL:** `http://localhost:9090`  
**Content type:** `application/json`

Mini-Orca is a local Compose Desktop client and loopback daemon for one active
project. Its primary editing flow is deliberately narrow: open one Go file,
select or name one declaration, talk in a conversation pinned to that target,
edit the returned declaration draft, validate it, run scoped checks, and then
explicitly confirm a one-file Apply. Source and composed-diff views are
read-only. The daemon never creates multi-file changes, scans automatically,
writes automatically, commits, or pushes.

`docs/openapi.yaml` is the machine-readable request/response contract. The
route test in `cmd/daemon/main_test.go` compares both this table and the OpenAPI
paths with the daemon registrations.

Each request body is one size-limited JSON object. Unknown fields, trailing
values, invalid enum values, and missing required revision guards are rejected
with the structured error response described below.

## Local request boundary

The daemon trusts the local Compose Desktop process to call its loopback API;
it does not provide browser sessions, CORS access, or request authentication.
With the default loopback bind, requests must use a `Host` of `localhost`,
`127.0.0.1`, or `::1` (with an optional port). Requests carrying an `Origin`
header are rejected, and CORS preflight requests receive `403 Forbidden` with
no CORS response headers. This prevents a web page from treating the daemon as
a browser API while preserving native desktop requests, which send no
`Origin` header.

`POST` and `PATCH` requests with a body must send `Content-Type:
application/json` (a charset parameter is permitted); other media types and a
missing content type receive `415 Unsupported Media Type` before a route
handler runs. The documented pause and cancel actions carry their guards in
the query string and remain bodyless. `GET /health` remains available to
loopback health checks with no request body.

`MINI_ORCA_BIND_ADDRESS` can deliberately bind the daemon beyond loopback. In
that mode the Host restriction is relaxed so a configured native client can
reach it, but the API remains unauthenticated and continues to reject browser
origins. Restrict it with network controls; it is not suitable for public or
untrusted networks.

## Live routes

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness and daemon version. |
| GET | `/status` | Daemon status and `single_coder_preview` workflow identifier. |
| POST | `/api/projects/current/chat/sessions` | Open a Go declaration conversation pinned to project/file/revision/hash, mode, target, and optional reviewed task spec. |
| POST | `/api/projects/current/chat/sessions/{sessionID}/messages` | Request one declaration proposal; the body cannot retarget the session and may explicitly request a bounded task repair. |
| GET | `/api/models/current` | Non-secret `analyze`, `bug`, and `function` model catalog. |
| GET | `/api/projects/current/context` | Bounded context manifest for a project-relative `path`; source is never returned. |
| POST | `/api/projects/import` | Import the user-selected project and build deterministic project facts. |
| POST | `/api/projects/restore` | Restore a previously imported local project without contacting the model. |
| GET | `/api/projects/current/overview` | Read source-free metrics, structured analysis, coverage, and finding counts for `project_revision`. |
| POST | `/api/projects/current/analysis/preview` | Whole-project, read-only preflight with strict project/revision guards. |
| POST | `/api/projects/current/analysis/run` | Durably admit the previewed run with fresh provider and Security intent. |
| GET | `/api/projects/current/analysis/run` | Read progress with `project_id` and `project_revision`; 204 when absent. |
| POST | `/api/projects/current/analysis/run/control` | Pause, resume or cancel the exact run generation. |
| GET | `/api/projects/current/analysis/results` | Read Bugs, Performance or Security evidence using full run identity; optional captured-file filter. |
| GET | `/api/projects/current/findings` | List source-free verified findings and AI suggestions with provenance, filters, and freshness. |
| PATCH | `/api/projects/current/findings/{findingID}` | Record an explicit user triage status for one finding. |
| GET | `/api/projects/current/scan` | Read an explicitly started Go scan for `project_revision` from a temporary copied workspace. |
| POST | `/api/projects/current/scan` | Start an explicit Go parser/vet/test scan in a temporary copied workspace. |
| DELETE | `/api/projects/current/scan` | Cancel the active explicit Go scan. |
| GET | `/api/projects/current/execution-trust` | Read current-session trusted local execution status and exact project-code command scope; `task_test_name` selects a generated task-test argv. |
| POST | `/api/projects/current/execution-trust` | Explicitly trust local execution for the current project revision. |
| GET | `/api/projects/current/index` | Read deterministic eligible-file and symbol facts. |
| GET | `/api/projects/current/files/info` | Read safe selected-file information for project-relative `path`. |
| GET | `/api/projects/current/files/symbols` | List atomic targets in one selected eligible file. |
| GET | `/api/projects/current/impact` | Read advisory source-free impact references for one file or symbol. |
| GET | `/api/projects/current/git` | Read target-file Git availability, branch, and status. |
| GET | `/api/projects/current/files/analysis` | Read cached semantic analysis for one selected file and revision. |
| POST | `/api/projects/current/files/analysis` | Explicitly analyze exactly one selected file. |
| POST | `/api/projects/current/files/security-scan` | Scan one selected eligible Go file with deterministic source-only rules; no provider request or subprocess execution occurs. Matches require syntactic review and do not prove exploitability. At most five source-order matches are returned; a `partial` report says when that limit truncates coverage. |
| POST | `/api/projects/current/files/explanation` | Explicitly explain one exact indexed Go declaration using transient Function-scope context. |
| POST | `/api/projects/current/security-review` | Explicitly perform one passive, source-free AI Security review for one current eligible file, optionally one exact declaration. |
| GET | `/api/projects/current/analysis-job` | Read explicit bounded Analyze-all progress. |
| POST | `/api/projects/current/analysis-job` | Start bounded sequential Analyze-all cache warming. |
| POST | `/api/projects/current/analysis-job/pause` | Pause Analyze-all after its active file finishes. |
| POST | `/api/projects/current/analysis-job/resume` | Resume a persisted paused Analyze-all job. |
| POST | `/api/projects/current/analysis-job/cancel` | Cancel active/pending Analyze-all work. |
| GET | `/api/projects/current/performance` | Read cached source-based performance findings and queue-derived coverage; no model work occurs. |
| GET | `/api/projects/current/performance/context` | Preview the bounded policy-filtered Performance queue and Analysis-model manifest. |
| GET | `/api/projects/current/performance-job` | Read persisted source-free Performance-job progress. |
| POST | `/api/projects/current/performance-job` | Explicitly start a bounded sequential Performance review with current queue identity and remote confirmation. |
| POST | `/api/projects/current/performance-job/pause` | Pause after the active file, guarded by current revision and expected job ID. |
| POST | `/api/projects/current/performance-job/resume` | Resume a compatible paused job with fresh remote confirmation. |
| POST | `/api/projects/current/performance-job/cancel` | Cancel active/pending Performance work, retaining completed file reports. |
| POST | `/api/projects/current/reindex` | Refresh deterministic facts without an LLM request. |
| PATCH | `/api/projects/current/drafts/{draftID}` | Replace only the declaration/import list and create the next draft revision. |
| POST | `/api/projects/current/drafts/{draftID}/validate` | Compose and validate one exact draft revision. |
| POST | `/api/projects/current/drafts/{draftID}/checks` | Run scoped checks for one validated draft revision and hash. |
| GET | `/api/projects/current/drafts/{draftID}/benchmarks` | List explicitly selectable existing Go benchmarks in the affected package for one validated draft revision. |
| POST | `/api/projects/current/drafts/{draftID}/benchmarks` | Compare one selected existing Go benchmark in isolated base and candidate copies. |
| POST | `/api/projects/current/apply` | Apply one validated, checked declaration draft only after `confirm: true`. |
| POST | `/api/projects/current/undo` | Restore only the immediately preceding unchanged apply after `confirm: true`. |

Declaration explanation requests bind `project_id`, `project_revision`, `base_file_hash`, `target_path`, and `target_symbol`. The target must resolve to one exact atomic declaration in an eligible indexed Go file. A non-loopback Function provider also requires `confirm_remote_provider: true` on that request. The daemon rechecks the project and file identity after provider work before returning the bounded explanation, source line anchor, optional engineering insight, and `ContextManifest` provenance.

Explanation responses are transient. This route does not create chat sessions or drafts, persist chat or analysis history, run checks, or change Apply/Undo state. Cancellation, malformed provider output, and stale request identity return an error without publishing a result.

Security review requests require `project_id`, `project_revision`, `base_file_hash`, `path`, and `confirm_remote_provider`; `symbol` is optional but must identify one exact atomic indexed declaration. The request uses the configured Analyze scope and requires fresh `confirm_remote_provider: true` before a non-loopback provider receives source. It sends at most 64 KiB and accepts at most 64 KiB of one strict JSON response with at most five advisory `model_suspicion` findings. The service rechecks project, revision, file hash, policy, focused declaration, and provider identity before delivery, after response, before caching, and before returning. Reports omit source, redact stored/returned prose, and reject meaningful source-line token sequences even when spacing or punctuation changes. Findings are advisory suspicions, and `completed_empty` means no findings were returned; it does not mean the file is secure. The route never runs suggested exploit code, subprocesses, scans, or project code.

## Trusted local execution

Import, restore, navigation, indexing, parsing, and `gofmt`/`go vet` source analysis never execute imported project code. `go test` can execute package initialization and tests, including generated temporary task tests. Before it can run, the client reads the exact argv scope from `GET /api/projects/current/execution-trust`; pass the validated generated test name as `task_test_name` to receive its exact `-run` argv. The client then explicitly posts `confirm: true` for the active project revision. This in-memory trust is separate from remote-model consent and is cleared when the project is replaced, restored, or reindexed. Commands run only in a temporary copied workspace with a small toolchain environment allowlist; this is trusted local execution, not sandboxing. On supported Unix platforms cancellation owns the command process group and its descendants. Other platforms reject project-code execution when that ownership is unavailable; source-only commands remain available.

## Focused draft lifecycle

Create a session with the imported `project_id`, current `project_revision`,
selected-file `base_file_hash`, project-relative `open_path`, and either
`replace_symbol` or `create_symbol` mode. A replace target must be one exact Go
symbol; a create target must be an absent valid top-level Go identifier. Those
values are immutable for session messages.

A successful session message yields an isolated `Draft`: declaration text,
optional imports, revision, hash, target identity, and state. It never returns
an editable source file. `PATCH` accepts only declaration/import changes and an
expected revision. Any edit clears earlier validation and checks. Validate and
checks each pin the revision (and checks also pin its hash), so stale or changed
drafts cannot be applied.

Project/file analyses, their AI findings and suggestions, and generated drafts
may include an optional `engineering_insight`. It is concise advisory model
prose bound to the parent result's normal freshness identity; it cannot carry a
path, action, or Apply authority. Missing or malformed optional insight text is
omitted without discarding otherwise valid analysis or a draft. Editing a draft
clears its insight for the current candidate revision.

An optional `task_spec` may open only a matching `replace_symbol` session. The
daemon validates its exact indexed target against the current revision and file
hash, then carries it through that session and its drafts. Its optional Go test
candidate is written only to the temporary copied check workspace under a
non-conflicting generated `_test.go` name. It must fail on the captured base
and pass with the composed candidate before its required check succeeds.

When current task-bound checks fail, a client can send the next pinned session
message with `repair: true`. The daemon requires that exact latest failed draft,
pins any reviewed temporary proof carried by that parent for the rest of the
repair lineage, uses bounded sanitized check evidence supplied by the client,
and permits at most three such repair requests. A later repair cannot drop or
replace the pinned proof. Checks, temporary tests, validation, Apply, and Undo
never start a provider request on their own.

## Optional Go benchmark comparison

For a valid displayed Go draft, `GET /api/projects/current/drafts/{draftID}/benchmarks`
lists only valid existing benchmark functions from the draft target's package.
Each choice includes the exact daemon-built argv and an opaque selected-scope
guard bound to a bounded whole-workspace fixture fingerprint before trust is
granted. It requires the same exact draft revision/hash and current project
revision as the comparison request. The list is unavailable when the candidate
is invalid or stale or the affected package has no eligible benchmark. Its
`trusted` flag reports current-session execution consent.

`POST /api/projects/current/drafts/{draftID}/benchmarks` accepts one listed
benchmark name and the displayed selected-scope guard. The daemon recomputes the
scope before execution, so any workspace change since display makes the request
unavailable. The daemon constructs the fixed argv itself: `go test .`, `-run ^$`,
an exact escaped `-bench` filter, five 100 ms samples, `-benchmem`, and bounded
test/process deadlines. It runs the base and candidate in separate temporary
copies with the same captured whole-workspace fingerprint, argv and sanitized
toolchain environment. Request cancellation owns the active process group; no
shell, model-authored command, benchmark generator or project-source write is
involved. A changed baseline or draft invalidates the comparison. Failed,
canceled or incomplete results return no samples and never grant Apply authority.
A completed result returns paired samples as optional evidence without declaring
a winner; reviewers still assess variability and noise.

`POST /api/projects/current/apply` requires the displayed draft id, revision,
hash, project identity, base file hash, and an explicit `confirm: true`. It
re-reads the target and rejects stale state before its atomic one-file write.
Undo similarly requires the post-apply hash and explicit confirmation; it
restores only the immediately preceding unchanged apply. The audit omits source,
draft text, prompts, and secret-like values.

## Project intelligence and limits

Import and reindex build deterministic metadata; neither starts a verified scan
or Analyze-all. Go scans and semantic analysis are always user-started. A scan
prepares a bounded temporary copied workspace before parser, vet, and test phases;
a resource failure is returned as a failed `workspace` phase with sanitized evidence.
Findings keep source (`ai`, parser, vet, or test), confidence, severity, status,
and freshness separate so a model suggestion is never presented as a verified
tool result.

Declaration editing, exact symbol targeting, composition, and mandatory parse /
format checks are Go-first. Other languages may have conservative approximate
symbol extraction and analysis, but do not receive exact declaration editing or
equivalent validators.

## Privacy, configuration, and errors

The daemon binds to loopback by default. It filters ignored, generated,
configuration, and secret-like paths before assembling model context. Each
prompt request checks the effective scope shown by `/api/models/current`: project
import and explicit source-based Performance reviews use `analyze`, selected-file analysis and Analyze-all use `bug`, and
declaration proposals use `function`. A non-loopback scope requires
`confirm_remote_provider: true` for that request only; confirmation for one
scope never authorizes another. Restore, reindex, scans, validation, checks,
Apply, and Undo never require provider confirmation.

Configuration is local-only in `config.yaml` (ignored by Git); begin with
`config.example.yaml`. The API never returns configured credentials. All API
failures use a structured error object with `type`, `message`, and
`user_message`. Project paths are canonical project-relative paths and revision/hash
guards return `409 Conflict` when their captured base is no longer current.

`model_scopes` is loaded only when the daemon starts, and all fixed scopes are
required. A changed model, provider, or reasoning effort makes old AI cache
entries stale. Providers must support OpenAI Chat Completions JSON; native
Anthropic/Gemini endpoints, vendor SDKs, streaming, tool calls, and
credential-vault features are outside this API. An optional scope
`reasoning_effort` is safe metadata and is included in a Chat Completions
request only when configured. Keys belong only in ignored local config and are
neither logged nor returned.

Prompt-bearing provider requests never follow HTTP redirects. Mini-Orca rejects
every 3xx response at the configured provider origin and returns a status-only
error; it does not send the prompt, request body, or authorization header to a
redirect target, and it does not retry a rejected redirect. Update the configured
provider URL explicitly when its endpoint changes.

## Unified analysis contract

These routes are registered through the same loopback/origin policy as the other
local APIs. The shared controller owns unified and compatibility execution.
All bodies reject unknown fields and trailing JSON. Query guards must each occur
once, use only documented names and contain 1–4096 characters. Missing/invalid
request fields return 400, unknown projects 404, identity/lifecycle conflicts 409,
and progress storage failures 500 with sanitized recovery guidance. No saved run
returns 204. Result `path` must be a canonical path captured by this run; an
unknown file yields 409. Filtering leaves the run's project coverage unchanged.

- `POST /api/projects/current/analysis/preview`: accepts `AnalysisPreviewRequest`;
  returns `AnalysisRunPreview` without provider calls, subprocesses or source writes.
- `POST /api/projects/current/analysis/run`: accepts `AnalysisRunStartRequest`;
  returns an admitted `AnalysisRun` with status 202 after durable admission.
- `GET /api/projects/current/analysis/run`: reads current progress for the required
  `project_id` and `project_revision`; it never starts or resumes work.
- `POST /api/projects/current/analysis/run/control`: accepts
  `AnalysisRunControlRequest` for pause, resume or cancel.
- `GET /api/projects/current/analysis/results`: accepts the complete run identity
  as query guards plus `category` and optional project-relative `path`; returns
  `AnalysisSectionResults`. A file filter only changes the read view.

There is one execution scope: `project`. No selected file, path filter or list of
user-picked files belongs in a start request. Preview captures the whole eligible
project inventory, sorted by canonical path, with content hashes, language, size,
stage eligibility/cache dispositions and explicit exclusions. The 100-file default
and 500-file maximum bound a dispatch window, not the inventory or total coverage.
Default window time is 900 seconds, maximum 3600. Total attempts per model stage
include the initial request and transport retries: default 2, maximum 4. Counters
never reset on resume/restart. Cached work and passive Security rules make zero
model requests; retries cannot be multiplied by an undisclosed outer retry loop.

Four stage identities are fixed: `semantic`, `performance`, `security_rules`,
`security_ai`. Semantic risks can feed all three result categories. Performance
reviews feed Performance; Security rules and advisory reviews feed Security.
The latter three retain their existing report types and provenance. A stage's
consumers do not classify its prose. Stage work counts once in request budgets,
even when it feeds several result sections. The preview exposes expected model
requests without retries and the inclusive maximum for the remaining work.

`queue_id` binds project/revision, policy, provider fingerprints, ordered file/hash
and stage identities, exclusions, refresh choice and limits. Effective provider
identity includes scope, model, origin, reasoning, context/timeout/retry settings
and prompt/rule versions. Cache availability is excluded from this stable identity:
the run's own cache writes cannot invalidate its remaining queue. `preview_id`
additionally binds current cache dispositions, remaining attempts/work and request
bounds. Start echoes limits/refresh and both identities; the daemon recomputes
them before admission. A changed preflight yields 409 and requires a fresh preview.

Resume first requests a new preview with `resume_run` identifying the existing
run. It preserves that run's captured scope, limits and cumulative attempt counts,
but recalculates remaining request bounds. Control echoes the run identity,
fresh `preview_id` and fresh confirmations. The durable initial `plan` retains the
original source-free admission evidence; it never contains usable confirmation.
Run `id` plus `generation` prevents late results or controls from acting on a
replacement run with the same queue. All identity mismatches return 409. Unknown
request fields/enums, missing guards or invalid limits return 400.

Provider confirmation is a list of the displayed effective provider IDs. Security
AI additionally requires explicit `security_review: true`, including for a local
provider. This intent is limited to the displayed admitted work; it is consumed
when start/resume is attempted, is never saved in a run, and cannot survive restart
or authorize a changed provider/project. Pause/cancel do not carry confirmations.
Scope-specific existing consent rules remain enforced before each provider dispatch.
The run never invokes tests, vet, benchmarks, exploits or suggested shell commands.

### Legacy job migration (ANA-05)

Analyze-all and Performance routes use the same durable analysis controller.
They capture only their original semantic or performance stage and at most their
existing `max_files` limit; they never authorize Security or another model scope.
Their projections keep the original job/file response shapes and bounded queue
IDs. New metadata lives in `.mini-orca/analysis/run.json`; old session job files
and historical producer reports are retained, not rewritten. Unified previews
show `compatibility_stage` when resuming one of these bounded jobs, making its
limited coverage explicit. An ordinary unified start always captures every stage
and the whole project.

Legacy responses project queued/running to running, pausing/paused to paused,
canceling/canceled to canceled, and terminal stage outcomes to completed with
per-file errors. `interrupted` is an additive job state for recoverable shared
progress and saved pre-migration jobs. Provider retries are now charged as actual
transport attempts, bounded by the smaller effective-provider and job allowance;
there is no additional semantic parser retry loop. Performance retains its total
active-time budget across resumes (including sub-second accounting), rather than
receiving a new window allowance. After process loss, an active job conservatively
charges the interval since its last durable progress update, capped at that total
budget; a saved paused job does not spend its idle time. An exhausted legacy budget
requires a new start.

Pre-migration jobs lack captured provider fingerprints and semantic source hashes,
and their attempt counters represented file calls rather than transport requests.
They therefore cannot safely resume under the new authority contract. Reads retain
all original file progress/attempts and present formerly running/paused jobs as
interrupted, without dispatch or writes. Legacy controls on those snapshots return
409 with a migration message: review the retained progress and explicitly start a
new job, or use unified preview/start. A new run has a new identity and accounting;
it does not reinterpret or reset the historical job. Saved reports remain available
through the existing report endpoints and matching caches can be reused.

The single owner rejects overlapping kinds, retains a fault until explicit durable
recovery, and requires the old worker to finish cancellation before replacement.
A project switch may briefly return busy until that worker exits; stale controls
cannot affect a replacement. Legacy clients should move to the unified identity
and preview/control routes to distinguish every lifecycle state and use fresh
provider-specific consent. Legacy remote-confirmation booleans remain limited to
their single provider scope.

### Progress and partial evidence

Run and section states are `queued`, `running`, `pausing`, `paused`, `canceling`,
`canceled`, `interrupted`, `completed`, `completed_empty`, `partial`, `failed`,
`unavailable`, `stale`. File-stage states additionally identify `pending`,
`skipped` and incomplete producer reports as `partial`; paused work stays pending.
Each of Bugs, Performance and Security has its own progress entry. Analysis is
the progress owner; result prose belongs only to the three result reads.

Coverage counts captured file-stage units relevant to the section: total, pending,
running, succeeded, partial, failed, skipped and unavailable. These sum to total;
excluded files are separately visible in the plan. Cached current evidence counts
as succeeded. A partial/truncated report retains findings but cannot claim full
coverage. Pending means unfinished, including abandoned work in canceled/stale
runs; it is not permission to dispatch. A successful stage shared by sections is
counted in each section's coverage, but only once in request/attempt budgets.

`finding_count` is JSON null until there is successful or partial evidence. Zero
means that such evidence returned no findings, not that the project is safe.
`completed_empty` requires nonzero, fully successful coverage and zero findings;
`completed` requires fully successful coverage with findings. A terminal `partial`
section has useful evidence and incomplete coverage, with no work still pending
or running. An all-failed/unavailable section cannot claim zero findings. Stale
counts are historical and must not contribute to fresh navigation badges.

Overall completion requires all three sections to be completely covered. A mixed
terminal outcome with useful evidence is partial; no successful evidence yields
failed or unavailable. Reaching a window file/time/attempt budget with work pending
pauses the run with an explicit reason. Pause stops at a stage boundary; cancel
stops active requests and future dispatch while retaining completed reports.
Resume applies to paused/interrupted runs. Canceled or stale runs require a new
explicit start. Restart restores interrupted progress and never dispatches work.
Window counters reset only on an admitted resume; total elapsed/attempt counters
and completed reports remain. Persistence failure stops dispatch before the next
stage and exposes a recoverable operational failure.

### Category and cache compatibility

`Finding.category` and `UnifiedFinding.category` use `bugs`, `performance` or
`security`. They are independent of severity, confidence, source and the existing
specialized Performance/Security subtype categories. Storage envelope version 1
remains readable: the category field is additive and omitted on historical records.
Absent categories stay unclassified; no keyword inference, default Bugs category
or history rewrite occurs. Such evidence is available as previous analysis and
under `unclassified`, excluded from fresh section counts. General suggestions
remain file explanations and are not findings in any result category.

New semantic model output requires explicit categories and uses the updated prompt
identity introduced by ANA-02. Prompt-sensitive cache lookup returns older reports
as stale while preserving their full explanations and risks. Historical output is
not treated as classified. Unknown nonempty categories are rejected on writes;
absence remains accepted for legacy producers.
Finding IDs intentionally exclude category as well as revision/hash, so adding
or correcting a category preserves existing dismissed/fixed triage for unchanged
evidence. Typed Security triage/verification and Performance hypotheses are retained;
results do not merge unrelated producers merely because line/title text matches.
