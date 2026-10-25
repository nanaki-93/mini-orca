# Mini-Orca local API contract

**Version:** 4.4.0
**Base URL:** `http://localhost:9090`  
**Content type:** `application/json`

Mini-Orca uses a pure Go TUI client and loopback daemon for one active project.
The shared change workflow captures up to eight explicit Go/Markdown paths,
prepares versioned proposals, checks them, records diff review and explicitly
applies them with guarded recovery/Undo.
Source and composed diffs stay read-only. The daemon never writes source,
runs project code, commits or pushes merely because a project or history is opened.

`docs/openapi.yaml` is the machine-readable request/response contract. The
route test in `cmd/daemon/main_test.go` compares both this table and the OpenAPI
paths with the daemon registrations.

Each request body is one size-limited JSON object. Unknown fields, trailing
values, invalid enum values, and missing required revision guards are rejected
with the structured error response described below.

## Local request boundary

The daemon trusts the local TUI client to call its loopback API;
it does not provide browser sessions, CORS access, or request authentication.
With the default loopback bind, requests must use a `Host` of `localhost`,
`127.0.0.1`, or `::1` (with an optional port). Requests carrying an `Origin`
header are rejected, and CORS preflight requests receive `403 Forbidden` with
no CORS response headers. This prevents a web page from treating the daemon as
a browser API while preserving local TUI requests, which send no
`Origin` header.

`POST` and `PATCH` requests with a body must send `Content-Type:
application/json` (a charset parameter is permitted); other media types and a
missing content type receive `415 Unsupported Media Type` before a route
handler runs. Analysis controls carry the full run identity in their JSON body. `GET /health` remains available to
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
| GET | `/status` | Daemon status and `shared_proposals` workflow identifier. |
| GET | `/api/models/current` | Non-secret `analyze`, `bug`, and `function` model catalog. |
| GET | `/api/models/available` | Model choices from configured transports, Pi and Agy, with defaults and separate discovery statuses. |
| GET | `/api/projects/current/context` | Bounded context manifest for a project-relative `path`; source is never returned. |
| POST | `/api/projects/import` | Import the user-selected project and build deterministic project facts. |
| POST | `/api/projects/restore` | Restore a previously imported local project without contacting the model. |
| GET | `/api/projects/current` | Read the selected project summary without changing the active project. |
| GET | `/api/projects/current/overview` | Read source-free metrics, structured analysis, optional engineering specification (`docs`), coverage, and finding counts for `project_revision`. |
| GET | `/api/projects/current/analysis/selection` | Read the file checklist and saved exclusions with project/revision guards; no dispatch. |
| POST | `/api/projects/current/analysis/selection` | Save excluded paths with project/revision and selection identity guards; no source changes. |
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
| GET | `/api/projects/current/events` | Stream Server-Sent Events (SSE) when analysis runs, scans, or change workflows transition state. |
| GET | `/api/projects/current/agent-runs` | Read bounded root invocation history with project/revision guards and pagination. |
| GET | `/api/projects/current/agent-runs/events` | Replay source-free durable progress or subscribe using SSE and a sequence cursor. |
| GET | `/api/projects/current/agent-runs/{runID}` | Read invocation hierarchy and retained events; private model text requires `include_text=true`. |
| POST | `/api/projects/current/agent-runs/{runID}/control` | Cancel an active root invocation owned by this daemon. |
| POST | `/api/projects/current/agent-runs/{runID}/input` | Respond to a native checkpoint through the guarded proposal review owner. |
| POST | `/api/projects/current/agent-runs/{runID}/export` | Export one explicitly confirmed source-free trace snapshot to the configured collector. |
| GET | `/api/projects/current/missions` | Read bounded private mission summaries with current project guards. |
| POST | `/api/projects/current/missions` | Save a scoped mission draft without dispatch or source mutation. |
| POST | `/api/projects/current/missions/plan` | Explicitly decompose a bounded mission with native ADK and save an unapproved draft. |
| GET | `/api/projects/current/missions/{missionID}` | Inspect a retained plan, review state and binding freshness. |
| POST | `/api/projects/current/missions/{missionID}` | Revise, review a child or approve an exact saved mission plan. |
| GET | `/api/projects/current/missions/{missionID}/execution` | Inspect the frozen execution receipt and current binding freshness. |
| POST | `/api/projects/current/missions/{missionID}/execution` | Prepare, start, stop, integrate or reconcile a bounded mission execution. |
| GET | `/api/projects/current/memory` | Read bounded memory metadata without private text or provider work. |
| GET | `/api/projects/current/artifacts` | Read project-owned artifact versions with snapshot-bound pagination and no payloads. |
| GET | `/api/projects/current/attachments` | Inspect imported media versions and Function-profile availability without dispatch. |
| POST | `/api/projects/current/attachments` | Explicitly import bounded supplied media bytes into a private artifact. |
| POST | `/api/projects/current/attachments/advice` | Send exact imported versions for one consented native multimodal advisory response. |
| GET | `/api/projects/current/live-sessions` | Inspect live availability and retained metadata without connecting. |
| POST | `/api/projects/current/live-sessions` | Explicitly open one bounded, owned live advisory session. |
| GET | `/api/projects/current/live-sessions/{liveID}` | Inspect session state and private output artifact references. |
| POST | `/api/projects/current/live-sessions/{liveID}/control` | Explicitly send, interrupt, reconnect or close with current control revision. |
| GET | `/api/projects/current/artifacts/{artifactID}` | Explicitly disclose a bounded text preview or binary/research-report metadata. |
| POST | `/api/projects/current/artifacts/{artifactID}/control` | Export a new private metadata file or delete an exact unprotected artifact version. |
| GET | `/api/projects/current/integrations` | Read passive configured MCP connections and current schema grants. |
| POST | `/api/projects/current/integrations/{integrationID}/control` | Explicitly discover or revoke an MCP integration. |
| GET | `/api/projects/current/apis` | Read passive curated API configurations and expiring inventories. |
| POST | `/api/projects/current/apis/{integrationID}/control` | Explicitly discover a local API specification or revoke its grant. |
| GET | `/api/projects/current/api-actions` | Read private exact-action candidates and durable external outcome receipts. |
| POST | `/api/projects/current/api-actions/{actionID}/control` | Approve one exact external request or record a user-reported reconciliation. |
| GET | `/api/projects/current/research` | Read Google Search availability and private report metadata without provider work. |
| POST | `/api/projects/current/research` | Explicitly research a bounded question with admitted sources and search consent. |
| GET | `/api/projects/current/research/{reportID}/report` | Read a private HTML report preserving citations and Google Search suggestions. |
| GET | `/api/projects/current/remote-agents` | Read passive A2A configuration and current card grants. |
| GET | `/api/projects/current/agent-access` | List configured inbound principals and temporary access grants for this project. |
| GET | `/api/projects/current/agent-settings` | Read configured or overridden role, budget and skill settings without dispatch. |
| POST | `/api/projects/current/agent-settings/control` | Confirm a versioned replacement or reset for future admissions. |
| POST | `/api/projects/current/agent-settings/skills/validate` | Explicitly validate a configured skill's captured local text without executing resources. |
| POST | `/api/projects/current/agent-access/{principalID}/control` | Explicitly grant or revoke bounded inbound analysis/proposal access. |
| POST | `/api/projects/current/remote-agents/{integrationID}/control` | Explicitly discover a configured remote card or revoke its grant. |
| GET | `/api/projects/current/delegations` | List durable delegation metadata without private advisory text. |
| POST | `/api/projects/current/delegations` | Send one explicitly admitted advisory analysis or review task. |
| GET | `/api/projects/current/delegations/{delegationID}` | Read a private advisory receipt and current source freshness. |
| POST | `/api/projects/current/delegations/{delegationID}/control` | Explicitly refresh, reconnect, provide input, cancel or reconcile a retained delegation. |
| POST | `/api/projects/current/memory` | Create an explicitly retained, source-bound candidate memory. |
| GET | `/api/projects/current/memory/{memoryID}` | Read private memory text and current source status. |
| POST | `/api/projects/current/memory/{memoryID}` | Edit, accept or forget a memory using its current version. |
| GET | `/api/projects/current/execution-trust` | Read remembered project trust bound to the current revision and exact project-code command scope; `task_test_name` selects a generated task-test argv. |
| POST | `/api/projects/current/execution-trust` | Save or revoke local execution trust for the current project path. |
| GET | `/api/projects/current/index` | Read deterministic eligible-file and symbol facts. |
| GET | `/api/projects/current/files/info` | Read safe selected-file information for project-relative `path`. |
| GET | `/api/projects/current/files/symbols` | List atomic targets in one selected eligible file. |
| GET | `/api/projects/current/impact` | Read advisory source-free impact references for one file or symbol. |
| GET | `/api/projects/current/git/workspace` | Read local branches, worktrees and committed fix history. |
| POST | `/api/projects/current/git/worktrees` | Create a new branch and worktree from the displayed HEAD. |
| POST | `/api/projects/current/git/switch` | Switch a clean project to an existing local branch. |
| POST | `/api/projects/current/git/merge-preview` | Preview the selected worktree's incoming commits and diff. |
| POST | `/api/projects/current/git/merge` | Merge the captured worktree HEAD into the current clean branch, then remove the merged worktree and branch. |
| POST | `/api/projects/current/git/cleanup-merged` | Reconcile an external merge and safely remove the integrated worktree and branch. |
| POST | `/api/projects/current/git/discard-worktree` | Remove a confirmed clean worktree without merging; keep its branch. |
| POST | `/api/projects/current/git/abort-merge` | Abort the identified merge and discard conflict resolutions. |
| GET | `/api/projects/current/git/repository` | Read guarded repository identity, branch and working-tree state. |
| POST | `/api/projects/current/changes/{sessionID}/commit` | Retry the latest failed proposal commit without reapplying source. |
| GET | `/api/projects/current/git` | Read target-file Git availability, branch, and status. |
| GET | `/api/projects/current/files/analysis` | Read cached semantic analysis for one selected file and revision. |
| POST | `/api/projects/current/files/analysis` | Explicitly analyze exactly one selected file. |
| POST | `/api/projects/current/files/security-scan` | Scan one selected eligible Go file with deterministic source-only rules; no provider request or subprocess execution occurs. Matches require syntactic review and do not prove exploitability. At most five source-order matches are returned; a `partial` report says when that limit truncates coverage. |
| POST | `/api/projects/current/files/explanation` | Explicitly explain one exact indexed Go declaration using transient Function-scope context. |
| POST | `/api/projects/current/security-review` | Explicitly perform one passive, source-free AI Security review for one current eligible file, optionally one exact declaration. |
| POST | `/api/projects/current/reindex` | Refresh deterministic facts without an LLM request. |
| POST | `/api/projects/current/plans/investigate` | Investigate captured read/write scope and return validated citations and model suggestions for explicit acceptance. |
| GET | `/api/projects/current/changes` | List local change conversations without generation. |
| POST | `/api/projects/current/changes` | Capture a task and one to eight explicit Go/Markdown paths. |
| GET | `/api/projects/current/changes/recovery` | Read the latest grouped mutation or interrupted recovery journal. |
| GET | `/api/projects/current/changes/{sessionID}` | Read a proposal and recheck source freshness. |
| POST | `/api/projects/current/changes/{sessionID}/resume` | Restore history and clear check/review authority; no provider request. |
| POST | `/api/projects/current/changes/{sessionID}/messages` | Generate a scoped proposal or explicitly request bounded check-driven repair. |
| POST | `/api/projects/current/changes/{sessionID}/checks` | Check the current proposal; tests/vet require independent execution trust. |
| POST | `/api/projects/current/changes/{sessionID}/workflow` | Start creation, test writing/execution and model review for the captured scope. |
| POST | `/api/projects/current/changes/{sessionID}/workflow/cancel` | Cancel the identified workflow without applying source. |
| POST | `/api/projects/current/changes/{sessionID}/workflow/input` | Respond to the exact durable review request with current proposal identity. |
| POST | `/api/projects/current/changes/{sessionID}/plan` | Save revised task intent and scope before generation. |
| POST | `/api/projects/current/changes/{sessionID}/review-file` | Record inspection of a changed file for the current hash. |
| POST | `/api/projects/current/changes/{sessionID}/review` | Record diff review for the current checked hash. |
| POST | `/api/projects/current/changes/{sessionID}/apply` | Explicitly apply the current reviewed proposal with grouped recovery. |
| POST | `/api/projects/current/changes/{sessionID}/undo` | Restore the latest unchanged grouped Apply or recover an interrupted mutation. |
| POST | `/api/projects/current/changes/{sessionID}/verify` | Explicitly verify the latest applied change with fresh identity and trust. |
| GET | `/api/projects/current/instructions` | Preview applicable AGENTS.md content, origins and instruction presets. |
| POST | `/api/projects/current/instructions/proposal` | Prepare an AGENTS.md proposal without a provider or source write. |
| GET | `/api/projects/current/features` | Read saved goals and advisory feature suggestions. |
| POST | `/api/projects/current/features/goals` | Save goals; ideas generated for other goals become stale. |
| POST | `/api/projects/current/features/generate` | Explicitly add new bounded feature ideas, keeping existing ones, with optional profile consent. |
| PATCH | `/api/projects/current/features/{featureID}` | Save, dismiss or reopen an advisory feature suggestion. |

Project analysis keeps the existing string fields: new `architecture` values contain
Mermaid flowcharts and `flows` items contain flowcharts or sequence diagrams. These are
advisory model interpretations. Older prose values remain supported. New `components`
items use `path (Module name): responsibility` for display; clients must retain readable
fallbacks for existing freeform strings.

Project import sends a strict `json_schema` response format for the description,
diagrams, string lists, risks and nullable engineering insights. Prompt version
`project-analysis-v5` specifies JSON escaping and the existing report limits;
earlier successful reports remain readable but are stale. Provider rejection of the structured
request does not trigger a fallback to unconstrained chat. Truncated or refused
replies cannot become fresh reports, even when their content parses as JSON.
Failed descriptions retain the factual inventory and persist a source-free failure
reason; restore reads that outcome without contacting the provider.

Declaration explanation requests bind `project_id`, `project_revision`, `base_file_hash`, `target_path`, and `target_symbol`. The target must resolve to one exact atomic declaration in an eligible indexed Go file. A non-loopback HTTP or CLI Function provider also requires `confirm_remote_provider: true` on that request. The daemon rechecks the project and file identity after provider work before returning the bounded explanation, source line anchor, optional engineering insight, and `ContextManifest` provenance.

Explanation responses are transient. This route does not create chat sessions or drafts, persist chat or analysis history, run checks, or change Apply/Undo state. Cancellation, malformed provider output, and stale request identity return an error without publishing a result.

Security review requests require `project_id`, `project_revision`, `base_file_hash`, `path`, and `confirm_remote_provider`; `symbol` is optional but must identify one exact atomic indexed declaration. The request uses the configured Analyze scope and requires fresh `confirm_remote_provider: true` before a non-loopback HTTP or CLI provider receives source. It sends at most 64 KiB of target source plus up to 32 KiB of applicable project instructions, and accepts at most 64 KiB of one strict JSON response with at most five advisory `model_suspicion` findings. The provider receives a strict `json_schema` response format with lowercase rule/category identifiers and file-specific path, line bounds and symbol choices. Provider rejection of structured output fails the stage without falling back to unconstrained output. Prompt version `security-file-v4` also requires source-free explanations and an empty findings array when no concrete security concern is supported, including documentation and ignore files. Earlier prompt versions remain readable but are stale for current reviews. The service rechecks project, revision, file hash, policy, focused declaration, and provider identity before delivery, after response, before caching, and before returning. Reports omit source, redact stored/returned prose, and reject meaningful source-line token sequences even when spacing or punctuation changes. Findings are advisory suspicions, and `completed_empty` means no findings were returned; it does not mean the file is secure. The route never runs suggested exploit code, scans, or project code; a configured CLI runs only as the model transport.

## Trusted local execution

Import, restore, navigation, indexing, parsing, and `gofmt`/`go vet` source analysis never execute imported project code. `go test` can execute package initialization and tests, including generated temporary task tests. Before it can run, the client reads the exact argv scope from `GET /api/projects/current/execution-trust`; pass the validated generated test name as `task_test_name` to receive its exact `-run` argv. The client then explicitly posts `confirm: true` for the active project revision. This choice is saved in private user settings for the canonical project path and remains separate from remote-model consent. Reopening or reindexing a trusted project validates its current sources and captures a new workspace fingerprint; old revisions and workspace hashes never authorize new jobs. Other project paths and worktrees require their own consent, and a changed command scope requires confirmation again. Posting `confirm: false` revokes saved trust. Invalid or unreadable settings fail closed, and persistence failures are returned as errors. Navigation and trust restoration do not execute project code or dispatch a provider. Commands run only in a temporary copied workspace with a small toolchain environment allowlist; this is trusted local execution, not sandboxing. On supported Unix platforms cancellation owns the command process group and its descendants. Other platforms reject project-code execution when that ownership is unavailable; source-only commands remain available.

## Project intelligence and limits

Import and reindex build deterministic metadata; neither starts a verified scan
or unified analysis. Indexing streams hashes and line counts for large files, while
symbol extraction is limited to text files within the 1 MiB source-view limit.
Larger text files retain full size, hash and line-count facts with a diagnostic
that symbol extraction is unavailable. Index schema version 3 rebuilds older
cached entries under this bounded extraction policy.
Go scans and semantic analysis are always user-started. A scan
prepares a bounded temporary copied workspace before parser, vet, and test phases;
a resource failure is returned as a failed `workspace` phase with sanitized evidence.
Findings keep source (`ai`, parser, vet, or test), confidence, severity, status,
and freshness separate so a model suggestion is never presented as a verified
tool result.

Shared proposals support captured Go and Markdown files. Go changes require
parse and format checks; other languages retain conservative symbol extraction
and analysis without source editing.

## Privacy, configuration, and errors

The daemon binds to loopback by default. It filters ignored, generated,
configuration, and secret-like paths before assembling model context. Each
prompt request checks the effective scope shown by `/api/models/current`: project
import and explicit source-based Performance reviews use `analyze`, selected-file analysis uses `bug`, and
declaration proposals use `function`. A non-loopback HTTP or CLI scope requires
`confirm_remote_provider: true` for that request only; confirmation for one
scope never authorizes another. Restore, reindex, scans, validation, checks,
Apply, and Undo never require provider confirmation.

Context policy reads `.gitignore` and `.mini-orcaignore` in each traversed
directory. Patterns use directory-relative Git ignore semantics, including
anchored paths, directory exclusions, `**`, and ordered negation; an excluded
parent must be reopened before a child can be included. Explicit context-policy
include/exclude overrides retain their precedence, but cannot include secret,
generated, metadata, or symlink paths. Nested ignore changes participate in the
policy fingerprint. Policy version 2 makes evidence captured under the previous
ignore interpretation stale; refreshing it requires the usual explicit analysis.

Configuration is local-only in `config.yaml` (ignored by Git); begin with
`config.example.yaml`. The API never returns configured credentials. All API
failures use a structured error object with `type`, `message`, and
`user_message`. Project paths are canonical project-relative paths and revision/hash
guards return `409 Conflict` when their captured base is no longer current.

`model_scopes` is loaded only when the daemon starts, and all fixed scopes are
required. A changed model, provider, or reasoning effort makes old AI cache
entries stale. Providers can use OpenAI-compatible Chat Completions or native
Google `gemini`/`vertex` adapters. The ADK boundary supports typed function calls
and bounded text streams; existing HTTP proposal operations still return only
completed domain results. Native Anthropic Messages, OpenAI Responses, HTTP model
enumeration and credential-vault features are not exposed. Optional scope
`reasoning_effort` is safe metadata and maps to the selected provider's supported
controls. Keys and Vertex access tokens belong only in ignored local config and
are neither logged nor returned.

Configured model metadata includes `capabilities`: boolean `completion`,
`json_schema`, `tools`, `streaming`, `attachments`, `grounding`, and `live_sessions`.
These describe implemented adapter protocols, not an account's model entitlements.
CLI adapters support completion and locally validated JSON Schema only. HTTP
adapters also support tools and text streaming. Conformed native Google models
advertise attachments and grounding; the dedicated Gemini API live model
advertises live sessions through its separate protocol. Unsupported combinations
remain false. Historical metadata may omit the object; clients
must treat missing capabilities as unavailable. `/api/models/current` remains
passive and sends no provider request. Native model names are configured explicitly;
available-model discovery remains the existing explicit Agy/Pi catalog operation.
See [native Google configuration](../CONFIG.md#native-google-providers).

Scopes can instead select `provider: agy` or `provider: pi` in local configuration.
These CLI transports feed the same analysis and proposal workflows; they add no
command-execution API. Catalog and provenance identify them as `cli://agy` or
`cli://pi`, always with `remote_provider: true` and the same scope-specific
confirmation requirement. CLI `temperature` and `max_tokens` metadata are
`null` (unavailable), while HTTP values remain numeric. Executable paths and
CLI credentials are not exposed. CLI responses must complete successfully;
structured responses are also validated locally before entering domain parsers.
See [CLI configuration and boundaries](../CONFIG.md#cli-providers).

Prompt-bearing HTTP provider requests never follow redirects. Mini-Orca rejects
every 3xx response at the configured provider origin and returns a status-only
error; it does not send the prompt, request body, or authorization header to a
redirect target, and it does not retry a rejected redirect. Update the configured
provider URL explicitly when its endpoint changes.

## Unified analysis contract

These routes are registered through the same loopback/origin policy as the other
local APIs. The shared controller owns unified analysis execution.
All bodies reject unknown fields and trailing JSON. Query guards must each occur
once, use only documented names and contain 1–4096 characters. Missing/invalid
request fields return 400, unknown projects 404, identity/lifecycle conflicts 409,
and progress storage failures 500 with sanitized recovery guidance. No saved run
returns 204. Result `path` must be a canonical path captured by this run; an
unknown file yields 409. Filtering leaves the run's project coverage unchanged.

- `POST /api/projects/current/analysis/preview`: accepts `AnalysisPreviewRequest`;
  returns `AnalysisRunPreview` without provider calls, subprocesses or source writes.
- `POST /api/projects/current/analysis/run`: accepts `AnalysisRunStartRequest`;
  returns an admitted `AnalysisRun` with status 202 after durable admission. When
  `clear_results: true` is paired with `refresh: true`, admission clears saved
  findings, stage reports and feature suggestions (preserving saved project goals)
  and replaces any paused or interrupted run so the new run starts from 0.
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

Four per-file stage identities are fixed: `semantic`, `performance`, `security_rules`,
`security_ai`. Semantic risks can feed all three result categories. Performance
reviews feed Performance; Security rules and advisory reviews feed Security.
The latter three retain their existing report types and provenance. A stage's
consumers do not classify its prose. Stage work counts once in request budgets,
even when it feeds several result sections. The preview exposes expected model
requests without retries and the inclusive maximum for the remaining work.

`GET /api/models/available` returns `models[]` (`id`, `name`, `provider`,
`source`, `location`, `model`), `defaults` mapping the three legacy scopes to
configured IDs, and `pi`/`agy: {status, message?}`. Configured choices are deduplicated
by model/transport/sampling configuration, independently of operation. Pi entries
include every authenticated/custom model reported by `get_available_models`,
including local models from Pi's configuration. Pi discovery disables
tools/extensions/MCP/context loading, sessions and startup network refreshes.
Agy entries use the exact slug and display name returned by `agy --log-file
/dev/null models` (tab-separated stdout in Agy 1.3.4), including the CLI's model
variants. Agy may contact its catalog service; no static model list is used.
Each CLI uses a private working directory and a 20-second deadline. Neither
receives a prompt or project context. No query parameters are accepted. Either
CLI's failure retains configured choices and the other CLI's models with an
explicit `unavailable` status for the failed discovery; a successful empty list
is `ready`.
Only safe metadata is returned, never authentication headers, full endpoint URLs
or executable paths. `location` is `local` for loopback catalog destinations,
`remote` for other nonempty destinations, and `unknown` when Pi omits the URL.
Agy reports `unknown` because its catalog omits endpoints. This describes catalog
metadata; all CLI prompt requests still require consent.

Preview/start accept optional `models: {code, performance, security, features}`. Each value is
an opaque catalog ID (`configured:<hash>`, `pi:<provider>/<model>` or `agy:<slug>`), or a legacy
configured profile name (`analyze`, `bug`, `function`); all four operations need a choice when
the object is present. Older clients and saved runs may supply `review` as the
fallback for either omitted `performance` or `security` choice. Explicit operation
choices take precedence; legacy captured identities remain unchanged. Omission retains Bug for semantic Code analysis and Analyze
for Performance, Security AI and feature discovery. Catalog assignments use the
operation's context and dispatch limits; legacy profile selections retain their
original timeout/retry behavior. Configured choices retain their transport and
sampling settings; discovered CLI choices use that CLI's defaults and exact model
ID, without inheriting a configured scope's reasoning override. Logical scope remains
Bug for Code and Analyze for reviews/features. Clients cannot supply endpoints,
executable paths or credentials. Choices affect dispatch, cache freshness,
provenance and provider accounting without changing daemon configuration.
Preview and start must use identical choices. Invalid syntax returns 400; a
missing choice or changed admission returns 409 before inference. CLI membership
is rechecked before preview, start, resume and standalone feature generation;
discovery failure stops admission. Captured choices survive restart/resume.
Schema-1 runs without `models` remain readable with their original defaults.

Preview/start accept `include_features` (default false for existing clients),
which must match at admission. The web client includes it on every new analysis.
It adds one project-wide `feature_suggestions` step using saved goals, bounded
policy-filtered context and root AGENTS.md; explicit file exclusions also apply
to this context and suggested paths. `preview.features` captures goals/workspace
hashes, the current feature-store hash, exclusions and the selected feature provider/request
allowance. `run.features` exposes its status, cumulative attempts and nullable
`suggestion_count`, separately from the three finding sections. The step uses the
same additive feature generator as the Features routes: `suggestion_count` (0–5)
counts ideas newly added by this run's generation, not the cumulative library,
and `completed_empty` means no new ideas were added. A full feature history
records the step as `failed` with `Feature history is full; existing ideas were
kept.`; it leaves the saved ideas unchanged and is not a run persistence fault.
The step runs once
alongside file stages with its own deadline of at least ten minutes (or the
selected feature model's timeout when longer). File batch/time limits do not consume
its deadline. It retains the run's cumulative attempt allowance and fresh
provider consent, and is not repeated after completion on resume. A batch that
finishes or reaches its file allowance can still publish file results while
feature discovery runs. The window settles after both workers finish; user
pause/cancel stops both workers. Failed generation keeps
previous ideas and permits other stages to continue; overall progress reflects
partial results. Cancellation and source/goal changes reject late publication.
Saved schema-1 runs without feature fields retain their original behavior.
The feature prompt identity (`feature-suggestions-v4`) is part of the provider
fingerprint, so finished runs that included features under an earlier prompt
read as `stale`, not corrupt.

`queue_id` binds project/revision, policy, provider fingerprints, ordered file/hash
and stage identities, exclusions, refresh choice and limits. Effective provider
identity includes scope, model, origin, reasoning, context/timeout/retry settings
and prompt/rule versions. Cache availability is excluded from this stable identity:
the run's own cache writes cannot invalidate its remaining queue. `preview_id`
additionally binds current cache dispositions, remaining attempts/work and request
bounds. Start echoes limits/refresh, `retry_stale_failed`, `stale_only`, `stale_path`, `recover_incomplete`, `include_features`, `models` and both identities; the daemon recomputes
them before admission. A changed preflight yields 409 and requires a fresh preview.

Preview and start accept optional `retry_stale_failed` (default false). When true,
only eligible files with stale or failed producer evidence or durable failed stages
are selected. A failed refresh overrides older cached evidence and is retried even when an
older report remains available. A later saved review supersedes that failure.
Fresh stages reuse their caches; files with no prior analysis are
excluded unless another stage on that file qualifies it. Missing, partial,
canceled and ineligible stages alone do not qualify a file. This option cannot be
combined with `refresh`. The selection is part of both admission fingerprints;
resume echoes the option and retains the admitted file set even after reports
become fresh. Excluded files remain accounted for in the inventory. An empty
selection produces no model requests and is reported as unavailable, not clean.

Preview and start accept `stale_only` (default false) to select just files with
stale evidence. Unrelated failed or missing files are excluded; current stages
stay cached. This option cannot be combined with `refresh`, `retry_stale_failed`,
`recover_incomplete` or `include_features`. It is bound to both admission
fingerprints. Resume uses the captured option and file set; requesting it for a
run without that option returns 409.

With `stale_only`, optional `stale_path` narrows analysis to one exact
project-relative path in the eligible saved selection. Invalid paths or using a
target without `stale_only` return 400; unknown, excluded or ineligible targets
return 409. A target without stale evidence yields an empty preview. The target
is captured in the preview and both fingerprints; start must echo it and cannot
drop or change it. Resume preserves the captured target when omitted and rejects
a different target with 409. This does not change the saved file selection.
Results outside the target remain available with their own freshness, while
user and policy exclusions remain excluded.

Preview and start also accept optional `recover_incomplete` (default false), which
completes unfinished analysis without changing source code. Under the requested
`models`, it selects files with at least one recoverable eligible stage: stale,
failed, partial, missing, canceled, interrupted or unavailable evidence whose model
is configured (passive Security rules need none). Other inventory files are
excluded with `No incomplete analysis for this file.`; user, policy and
unsupported-file exclusions keep their own reasons. Recoverable stages bypass
caches within the attempt limits, current successful stages remain cached with
no requests, and stages whose model is not configured keep that reason with no
requests. Pending, running and paused work belongs to its run, so paused and
interrupted runs still require continuation. The option returns 400 when combined
with `refresh`, `retry_stale_failed` or `include_features`, so a recovery plan
never contains a feature step. It is part of the stored plan and both admission
fingerprints. On resume the captured plan decides it: clients may omit it, but
sending `true` for a run admitted without it returns 409. Resume retains the
admitted file set. An empty recovery selection produces no model requests.
`retry_stale_failed` keeps the narrower semantics above for existing clients.

Unfiltered result reads retain saved evidence from eligible files omitted only by
the stale-only, stale/failed or incomplete-work selection. `retained_files` supplies those
current indexed file identities; user and source-policy exclusions remain
excluded. The dispatch plan, run coverage and file-filter guards still describe
the admitted scope.

File reports retain their original project revision as provenance. Their freshness
depends on that file's current content, applicable instructions, policy and analyzer
configuration. Editing another file does not invalidate them. A saved run may be
stale as an execution snapshot while its unchanged files remain current; result
reads still require that exact saved run identity and the active project.

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

### Retired workflows

The Kotlin desktop, declaration chat/drafts, declaration Apply/Undo and benchmark
comparison, Analyze-all jobs and separate Performance jobs have been removed.
Their former routes return 404. Use `changes` for checked proposals and grouped
Apply/Undo, and `analysis/preview`, `analysis/run` and `analysis/run/control` for
analysis. Read-only declaration explanations remain supported.

Existing shared proposals and unified runs retain their schemas and guards.
Single-stage journals marked `compatibility_stage` as semantic or performance
are ignored: opening a project neither resumes them nor grants consent. Their
bytes remain untouched until an explicitly started unified run replaces
`analysis/run.json`. Old files under `sessions/` and declaration backups are no
longer read or used for Undo. Matching per-file analysis remains reusable.

### Progress and partial evidence

Run and section states are `queued`, `running`, `pausing`, `paused`, `canceling`,
`canceled`, `interrupted`, `completed`, `completed_empty`, `partial`, `failed`,
`unavailable`, `stale`. File-stage states additionally identify `pending`,
`skipped` and incomplete producer reports as `partial`; paused work stays pending.
Each of Bugs, Performance and Security has its own progress entry. Analysis is
the progress owner; result prose belongs only to the three result reads.

Coverage counts captured file-stage units relevant to the section: total, pending,
running, succeeded, partial, failed, skipped and unavailable. These sum to total;
excluded files are separately visible in the plan. Ineligible stages are marked
`skipped` with a reason and omitted from section totals. Cached current evidence counts
as succeeded. A partial/truncated report retains findings but cannot claim full
coverage. Pending means unfinished, including abandoned work in canceled/stale
runs; it is not permission to dispatch. A successful stage shared by sections is
counted in each section's coverage, but only once in request/attempt budgets.

`finding_count` is JSON null until there is successful or partial evidence. Zero
means that such evidence returned no findings, not that the project is safe.
`completed_empty` requires nonzero, fully successful coverage and zero findings;
`completed` requires fully successful coverage with findings. Performance and
Security accept an empty findings array (including a bare `[]`) or blank final
content as no findings. Blank provider content requires an explicit normal
completion; refusals, truncation, missing completions and malformed nonempty
findings remain failures. An empty review is not proof of safety or performance.
Performance sends a strict `json_schema` response format with required finding
fields and file-bounded line numbers. The model supplies line ranges only;
the app derives an unambiguous innermost enclosing declaration from the index.
Ranges spanning declarations remain valid file/line findings without a symbol
label. Legacy responses containing a symbol still require that declaration to
contain the complete range; invalid labels are never silently discarded. Provider
rejection fails the stage without falling back to unconstrained output.
The Performance prompt is `performance-file-v7`; older saved reports remain
readable but require refresh for current coverage. Failed Performance stages
retain source-free explanations for malformed JSON, invalid fields or values,
invalid anchors, and oversized responses. A terminal `partial`
section has useful evidence and incomplete coverage, with no work still pending
or running. An all-failed/unavailable section cannot claim zero findings. Stale
counts are historical and must not contribute to fresh navigation badges.

Performance and Security review freshness uses the requested `configured_model`,
while `model` preserves the provider's returned name, which may be a resolved
alias or version. A different returned name alone does not make a completed
review stale. Older Performance reports omit `configured_model` and reuse `model`
for this check; if it differs from the current selection, explicit recovery must
refresh that review. Source, revision, policy and provider checks still apply.

Result reads also return nullable `saved_finding_count`, the total of the returned
classified semantic, Performance and Security findings. It includes retained stale
reports and is independent of `progress.finding_count`, which still measures the
current run. Unfinished, failed or canceled stages do not hide their last saved
reports. A successful producer review replaces that file's previous report,
including a successful empty review; repeated reads and retries do not add old
totals to new ones. Stale reports retain their original revision/hash and stale
labels. Missing saved evidence remains null, and unclassified risks are excluded
from this total. File-filtered reads count only their returned findings. These
totals are read from producer stores after restart; no run-schema migration is
needed.

Overall completion requires all eligible file-stage units to be completely covered. A mixed
terminal outcome with useful evidence is partial; no successful evidence yields
failed or unavailable. Reaching a window file/time/attempt budget with work pending
pauses the run with an explicit reason. Pause stops at a stage boundary; cancel
stops active requests and future dispatch while retaining completed reports.
Resume applies to paused/interrupted runs. Canceled or stale runs require a new
explicit start. Restart restores interrupted progress and never dispatches work.
Restoring or switching projects interrupts active work without making saved
results stale. Reindexing unchanged files preserves both results and active work;
freshness follows captured source contents and inventory, policy, and provider
identity, not timestamps or app sessions. Reading finished runs checks the live
source revision without rewriting the index; additions or removals of files
excluded by context policy do not invalidate their saved progress or file results.
Start/resume retain the complete admission inventory and execution workspace guards.
Finished results incorrectly marked stale by older restore/reindex behavior
recover in memory when their source, policy and provider identities still match,
including runs with ineligible skipped stages. Changed inputs and unfinished stale
work remain stale.
Window counters reset only on an admitted resume; total elapsed/attempt counters
and completed reports remain. Persistence failure stops dispatch before the next
stage and exposes a recoverable operational failure.

### Category and cache compatibility

`Finding.title` carries a short content-specific name. New semantic output
(`file-analysis-v16`) requests a title for every risk; Performance
(`performance-file-v7`) and AI Security (`security-file-v4`) request the same
2–6 word names, at most 60 characters. File risks retain these names in
`UnifiedFinding.title`; older risks without a title use their existing summary.
The additive field leaves historical reports readable and finding IDs/triage
unchanged. Reading saved results never generates titles with a provider call.

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

### Analysis file selection

`GET /api/projects/current/analysis/selection?project_id=…&project_revision=…`
returns `project_id`, `project_revision`, `selection_id`, `excluded_paths`,
`editable`, and `files` (`path`, `reason`, `stages`). An empty reason means the
file is eligible for selection. The checklist uses the project inventory, omitting build,
dependency and metadata directories; source-policy exclusions and unsupported
files remain visible with a reason and cannot be enabled by the checklist.

`POST` on the same route accepts `project_id`, `project_revision`, `selection_id`,
and a required `excluded_paths` array (an empty array selects all eligible files).
Paths must be unique canonical project-relative paths, up to 20,000 entries.
New exclusions must occur in the inventory; previously saved exclusions for
absent files may be retained. Stale project/selection guards and unknown new paths
return 409. Malformed input returns 400. An active, paused or interrupted run
must finish or be canceled before editing its selection (409 otherwise).

Exclusions are saved atomically in `.mini-orca/analysis/selection.json`, schema
version `1`, and restored across project/app/daemon restarts. Corrupt or unreadable
selection metadata fails explicitly; it never silently selects everything.
New files default to selected. Selection changes invalidate pending admission
through the existing preview/queue identity. The checklist scopes new unified
project analysis runs, including stale/failed retries and incomplete-work recovery;
it does not erase existing results or change explicit single-file actions or the
shared context policy.

Each selectable file's `stages` contains `stage`, `status`, and an explanation in
`reason`. Status reads inspect current source hashes, cache freshness and retained
run progress without model calls, using that run's captured model choices when
its revision still matches. Fresh cache evidence is required for `fresh`;
historic completion alone is insufficient. Changed unindexed files are `stale`.
Missing, failed, partial, unavailable, skipped, pending, paused, canceled and
interrupted analysis remain distinct. Unreadable file/cache evidence is reported
as unavailable with a reason. New unindexed files remain visible with a prompt
to refresh project facts. Ineligible files have an empty stage list and the
file-level exclusion reason. Selection for the next run does not erase saved
analysis status. The client classifies paths in `excluded_paths` as excluded,
just like files with a policy exclusion reason, and omits both from freshness
counts. Retained stage evidence becomes visible again when a file is re-selected.

Both selection responses also include `recovery: {state, file_count, stage_count,
reason?}`, the daemon's read-only answer to whether `recover_incomplete` can finish
saved-selection work now. Clients present it instead of re-deriving eligibility.
`state` is the first match of: `running` (an analysis run is active), `continuation`
(the last run is paused, interrupted or has a retained persistence fault; continue
or cancel it), `not_started` (no saved run for this project), `blocked` (a selected
eligible file changed since indexing or cannot be read; refresh project files),
`available` (at least one recoverable stage), `blocked` (the only unfinished work
needs an unconfigured model profile, which the reason names) and `complete`.
Recoverable stages follow the `recover_incomplete` rule above, observed with the
same model choices as the stage statuses. The counts cover selected eligible files
and recoverable stages under those choices, whatever the state; saved exclusions,
policy exclusions and unsupported files never count. A POST response reflects the
newly saved exclusions. `reason` is present for `running`, `continuation` and
`blocked`.

Overview `analysis_coverage` counts the same selected, eligible files as the file
checklist across applicable stages. Config and saved selection exclusions do not
contribute to coverage. `partial` retains incomplete, paused, interrupted and
canceled files; `unavailable` retains unreadable or unavailable evidence. Captured
run coverage and the project description retain their own historical status.

## Shared AI proposals and project instructions

All new GET routes require `project_id` and `project_revision` query guards.
The instructions preview also requires a canonical relative `path` ending in
`AGENTS.md`. It reads root-to-directory guides, respecting context exclusions
and rejecting symlinks. More local guidance applies within its own directory.
Applicable guides are included by default in project and file analysis, performance
and security AI reviews, declaration explanations, feature suggestions,
and proposal creation, testing and review. They grant no provider, execution or
Apply authority. AI file reports carry optional `instructions_fingerprint`
provenance; changed applicable guides make cached reports stale and invalidate
in-flight results. Older reports without this identity can only remain current
when no applicable guide exists and their other cache identities still match.
The preview's `presets` contain independent guidelines organized by `category`
into common AGENTS.md sections: project overview/architecture, build/development,
code style, testing, security/data, UI/accessibility, documentation and handoff.
These are organizational conventions, not mandatory AGENTS.md fields.
Choices match indexed languages, build manifests, UI files and supported imports
in the selected directory and descendants. This includes Go, Node/TypeScript,
Python, JVM, Rust, shell, Markdown, Go HTTP/database/CLI libraries, React imports
in TypeScript, desktop host manifests, Dockerfiles and GitHub Actions workflows.
Common project rules are always available. Matched entries carry `reason` and `evidence`
(up to three sorted indexed paths); general entries omit these fields. Current
context exclusions and binary-file filtering apply to the indexed evidence.
Reindex to refresh file facts. Loading these choices makes no provider request,
executes no project code and does not write AGENTS.md.

A change creation body carries `project_id`, `project_revision`, `kind`
(`fix`, `performance`, `security`, `feature`, `instructions`), `title`, `paths` and optional
`acceptance_criteria` and `plan`. Source paths must satisfy the context and write
policies; creation is supported, while deletion is unavailable. A manual instruction proposal adds
`content` to this body and requires exactly one AGENTS.md path. It makes no model call.

The TUI supplies a `plan` with `problem`, `evidence`, `approach`, `non_goals`
and optional `read_paths`, plus at least one acceptance criterion. Creation captures
1–8 write paths and up to 8 distinct existing read-only paths. Read context cannot
become a write target. `plan.hash` binds the intent, criteria, source, instructions
and workspace identity. Saving a plan makes no provider request and runs no checks.
`POST /api/projects/current/changes/{sessionID}/plan` accepts `ChangeIdentity`,
`plan`, `paths` and `acceptance_criteria` while the draft has no proposal. It
recaptures the scope, advances the revision and clears plan approval. Once a proposal
exists, changing its intent requires a new plan. Missing scope is never filled from
an arbitrary index entry; oversized scope must be split explicitly by the user.
Optional plan `citations` (at most eight) contain `path`, captured `hash`, one-based
`start_line` / `end_line`, exact `quote` (at most 1024 UTF-8 bytes), and model
`observation` (at most 512 bytes). Citations must match captured read/write source.
Optional `assumptions` contain at most eight nonempty strings of at most 512 bytes.
Both fields participate in approval identity; adding or changing them clears
approval. Empty fields preserve existing plan hashes.

`POST /api/projects/current/plans/investigate` accepts `project_id`,
`project_revision`, `plan`, one to eight explicit write `paths`, optional initial
`acceptance_criteria`, `sources`, and `confirm_remote_provider`. Each selected
read/write path needs exactly one source record: `{path, hash, exists}`. Existing
files require the current indexed hash; new write paths require `exists: false`
and an empty hash. Read context must exist and remain distinct from write scope.
The optional `worktree_path` query selects an existing worktree without creating
one. Investigation creates private run evidence but no change session, execution
trust, source mutation or worktree.

The Function profile performs bounded read-only investigation with admitted
repository tools when supported; completion-only profiles receive captured
source instead. Remote profiles require explicit confirmation. Source, absence,
revision, policy and applicable instructions are checked before dispatch and
again before publication. Changed source returns a conflict. Invalid input or
unusable model evidence returns an error; neither replaces the editable draft.
Cancel the HTTP request to stop investigation.

The result contains `version: "v1"`, project identity, `input_hash`,
`context_manifest`, suggested `approach`, `evidence`, `assumptions`,
`suggested_scope`, and nonempty `acceptance_criteria`. Evidence quotes and hashes
are validated against captured source; observations and assumptions remain model
suggestions. Response limits are 32 KiB, eight citations, eight assumptions, eight
suggested paths, sixteen criteria (512 bytes each), and a 4096-byte approach.
Captured source is capped at 256 KiB; the Function profile's context limit also
applies. Suggested paths never extend admitted scope. Accepting suggestions in
the TUI updates the draft only; explicit plan approval is still required before
generation.

Legacy API conversations and schema-1 history without `plan` retain their direct
message flow. Planned sessions reject generation without current approval.

`POST /api/projects/current/changes/{sessionID}/review-file` accepts `ChangeIdentity`
and a changed `path`. It records `reviewed_files` and `file_review_hash` without
model requests, checks or source writes. Planned sessions require all changed files
reviewed at the current proposal hash before Review/Apply. A complete file review
captures `last_reviewed_hash` and `last_reviewed_changes`; `review_diff` compares
that snapshot with the current proposal, including files returned to original
source. Revising a proposal clears current review progress. Per-file `summary` and
one-based `criteria` references are advisory explanations, not verified coverage.

Finding-driven changes also capture optional `finding_targets` (up to sixteen
distinct entries). Each records the original `project_id`, producer `kind`
(`semantic`, `performance`, or `security`), finding `id` and project-relative `path`
within the captured paths. Security targets also require `source` (`ai` or
`deterministic`); other producers omit it. These immutable references remain in
the session and history when the task is prepared in another worktree. Existing
schema-1 sessions without references remain readable; titles and paths alone are
never used to infer which findings a historical commit addressed.

Messages, checks, review, Apply and Undo carry `project_id`, `project_revision`,
`revision` and `hash`; the session ID is in the route. Messages add `message`,
`confirm_remote_provider`, optional `profile` (`configured:<hash>`,
`pi:<provider>/<model>`, `agy:<slug>`, or legacy `analyze`, `bug`, `function`,
default `function`) and optional `repair`. Consent applies to the selected profile. Repair requires current failed
checks and consumes one of three durable attempts. Checks accept `run_tests` and
`run_lint`; once requested those checks remain required. Go is formatted before
proposal publication where parseable. Unchanged targets and Go rewrites that only
differ in formatting are omitted from the proposal. File diffs align unchanged
lines between separate edits; whitespace inside literals and Markdown remains
visible when it changes. Source parsing/formatting checks are always
required; code tests/vet run only with current execution trust in a copied workspace
using fixed argv. New test files are tested against the base and candidate, and a
passing candidate tests stay pinned through repair. For new Go tests, the
optional baseline observation reports `outcome: failed_before_change`,
`already_passes`, or `could_not_execute`. A build failure is not evidence of bug
reproduction. Existing baseline and candidate tests remain required; the new-test
observation does not replace them. Executed checks include `duration_ms`. Passing parser checks alone
does not establish runtime correctness. Apply and Undo require `confirm: true`.

Apply optionally accepts `commit_message` (non-empty, at most 4096 UTF-8 bytes),
`head` and `branch` to apply and commit the reviewed paths. Git-backed sessions
capture branch/HEAD in addition to source identity; a branch switch or external
commit invalidates the proposal. Git mutations require opening the repository or
worktree root on a local branch with an initial commit and no merge/rebase in progress.
Existing schema-1 history without Git identity remains readable; current request
Git identity and existing source guards still apply when committing it.

The commit uses a private index, raw reviewed blobs, `git commit-tree` and a
compare-and-swap branch update. Unrelated staged/untracked content is excluded;
pre-existing edits or staging on proposal paths block Apply & Commit before any
source writes. This operation skips hooks, attribute filters and signing. Git
identity comes from normal Git configuration. No push or remote request occurs.
The optional `commit` object on mutation receipts, sessions and history records
`state` (`pending`, `failed`, `committed`), message, original branch/HEAD, candidate
`oid` and any error. A failed commit leaves source applied. Retry via
`POST /api/projects/current/changes/{sessionID}/commit` carries current project
identity, proposal hash, `confirm: true` and an editable `commit_message` and
rechecks source and Git identity. A recorded candidate is reconciled after an
interruption before retry is offered; ambiguous outcomes stay pending.
Undo restores source as working-tree changes and does not rewrite commit history.

Every proposal revision clears earlier checks/review. Apply uses current-session
authority and rechecks source, policy, instruction and whole check-workspace hashes.
Local `.mini-orca/changes/` history intentionally contains source, proposed contents
and conversations in private files; it is excluded from prompt context and Git
source indexing. Limits are eight paths, 256 KiB prompt/response content, forty
messages, 2 MiB per conversation/recovery journal and two hundred conversations.
History lists return summaries (`id`, project identity, kind, title, revision,
hash, state, freshness and updated time); selected reads/resume return full contents.
History reads are passive. Resume clears check/review authority; stale history is readable but cannot
be applied.

`POST /api/projects/current/changes/{sessionID}/workflow` starts a bounded agent
workflow and returns HTTP 202 with the captured session and `workflow.id`.
The request carries the current `ChangeIdentity`, `message`, and
optional `models: {create, test, review}`. Omitted or empty role fields use
`agent_runtime.roles` (Creation `bug`, Testing `function`, Review `analyze` by
default). The returned workflow always contains all three resolved choices.
Each explicit model value is an opaque catalog ID
(`configured:<hash>`, `pi:<provider>/<model>`, or `agy:<slug>`) or a legacy
configured profile name (`analyze`, `bug`, or `function`); catalog membership is
rechecked before admission. Resolved defaults require the same consent as explicit
choices. `confirmed_profiles` lists the selected remote
profiles the user approved; local profiles need no remote confirmation. Security
tasks additionally require `confirm_security: true`. Current project execution
trust and at least one captured test path are required before dispatch.
The workflow supports features, fixes, performance and security work; instruction
editing keeps its existing manual proposal flow.

The daemon executes a native ADK workflow graph with stable `create`,
`generate_tests`, `verify`, `review`, and `human_review` nodes. Generation and
test writing use the selected models; verification retains the existing baseline
and candidate checks with controlled commands. The current four public stage
summaries remain compatible: Testing spans test generation and verification.
Private graph events contain node state and captured identities; model content
remains in private completion sessions. Each node start is persisted before its
host operation, with no SDK-owned node retry. Cancellation, invalid predecessor
identity or journal failure stops successor nodes. Restored graph events cannot
authorize or replay source mutations.

Review fans out to correctness and test-quality specialists, plus architecture
for Go or multiple source files. Security tasks add a security specialist only
after explicit security intent; performance tasks add performance advice labeled
`unmeasured`. Routing is frozen at admission. Every specialist uses the selected
Review profile and the same checked proposal, within the shared concurrency and
call limits. `workflow.specialists` retains each role's state, model provenance,
evidence label and completed report. Legacy records may omit this field.
Reports join in canonical role order, deduplicating findings. Each report has at
most 16 findings; the aggregate has at most 80. A missing, failed or canceled
reviewer prevents aggregate approval. Parallel cancellation caused by a failed
reviewer remains a failed run rather than appearing as a user cancellation.

The testing agent may change only captured test
paths; its edits merge with the implementation. Checks use the isolated workspace
and existing regression baseline rules. Tests and review never apply source.
There is one active change workflow per conversation. Each model call uses the
largest of `timeouts.workflow_model_seconds` (900 seconds by default), the
generation timeout and its selected scope's timeout; retries share that deadline.
The review stage allows enough time for reviewers queued by the concurrency limit.
The total deadline is at least 30 minutes and includes remaining model budgets,
check/preparation allowances and enabled automatic repairs. Shared
`agent_runtime` call, token, concurrency and elapsed limits also apply across
stages and retries; exhausted budgets stop dispatch without applying source.
Failed checks
stop before review. A reviewer returning `changes_requested` blocks human approval
until a new run produces a passing proposal and approving review.

For planned sessions, workflow admission requires `plan_hash` matching the
captured plan. Approval does not bypass provider consent, execution trust or stale
source checks. `auto_repair` defaults to false; opting in permits at most two
automatic repairs through a conditional ADK graph. Each repair reserves the
shared repair budget and persists its transition before another generation.
Only actionable failed checks or completed rejecting reviews qualify; authority,
timeout, cancellation and persistence failures never cause automatic regeneration.
Failed checks can expose `scope_suggestions`, but checked-change
workflows never silently add targets and automatic repair stops when expansion
would be needed. Review the suggested scope in a new plan before generation.
Interrupted or unavailable required checks stop automatic repair: they do not
establish a code failure. Their evidence remains available for an explicit retry.
Timeout reasons distinguish a model-stage deadline, a focused check deadline and
the total workflow deadline, with the relevant configuration setting where applicable.

`resume_from` accepts `create` (default), `test`, or `review`. Test-stage retry
reuses the current implementation; critic retry requires live, passing check
authority for the exact current proposal and reuses those checks without running
generation or tests. Keep the model selections for reused stages; changing their
models requires restarting from that stage. Restored historical checks cannot
authorize that shortcut.

`stream_output` defaults to false. Explicitly enabling it captures the choice in
workflow history and requests incremental text from every selected model role.
All selected providers must support streaming at admission; completion-only CLI
profiles are rejected before dispatch. Partial output remains advisory, bounded
private detail and cannot become a proposal until the final response validates.
Attempts, stage `started_at`/`finished_at`, check duration, failure reason and
repair mode are exposed to the TUI. Timing is diagnostic, not a benchmark.

Poll the existing guarded session GET for `workflow.status`, ordered `stages`,
captured model metadata, and the review's `proposal_hash`, `verdict`, `summary`
and `findings`. Status distinguishes `running`, `canceling`, `failed`, `canceled`,
`stale`, `interrupted`, `outdated`, `changes_requested` and
`awaiting_human_review`. The final human stage waits for explicit diff review;
model approval never sets `reviewed_hash`. Existing Review/Apply/Undo routes retain
their identity and hash guards. Changing a proposal invalidates workflow review.

Approving critic results create a native ADK human-input checkpoint, exposed as
`workflow.input`. It includes a unique `request_id`, private run ID, workflow/node
identity, the `proposal-review-v1` schema, fixed `review_proposal` action, exact
`ChangeIdentity`, captured path/source/proposal hashes in `scope`, and `expires_at`
24 hours after creation. Its `status` is `waiting`, `approved`, `rejected`,
`canceled`, or `expired`; completed decisions include `received_at`. Waiting
releases the worker and preserves the checkpoint across restart. Legacy workflows
may omit `input`.

`POST /api/projects/current/changes/{sessionID}/workflow/input` requires
`ChangeIdentity`, `workflow_id`, `request_id`, and `decision` (`approve`, `reject`,
or `cancel`). The daemon revalidates captured source, proposal, scope and current
project/configuration/policy binding. Approval additionally requires live passing
checks and the existing plan/file review guards. Existing `/review` records the
same explicit approval receipt. The receipt is durable before native graph resume;
responses invoke no providers, checks, source writes or Git operations. Apply
remains a separate explicit action.

Repeating the same decision retains its original receipt; a conflicting decision
is rejected. A first response after expiry records `expired`, blocks review, and
requires a new workflow. Reject and cancel also block later approval of that
request; rerunning checks cannot override them. Storage failure leaves authority
ungranted and permits an idempotent retry. SDK session values and forged tool
responses cannot establish domain approval.

`POST /api/projects/current/changes/{sessionID}/workflow/cancel` takes current
`project_id`, `project_revision` and `workflow_id`, so stale controls cannot stop
a replacement run. It also records cancellation of a waiting input request.
Project replacement, reindex and daemon shutdown cancel the
worker. Background work can continue while the client navigates or disconnects.

### Agent invocation history and progress

Explicit project import, feature generation, declaration explanation, standalone
semantic analysis and security review have native knowledge runs. Provider
attempts appear as children and share the host budget; host retries, source
checks and publication authority are unchanged. Import binds its run to the
newly scanned checkout. A provider timeout/refusal retains a failed report and
rechecked factual inventory; journal or policy failures remain errors. Cached
reads, restore and handbook composition make no provider request.

Derived project handbooks use composition schema `2`. Component paths must exist
in the inventory; dependencies come from indexed imports. Model explanations,
diagrams and risks are advisory, and absent scenario steps, guards or decisions
are not invented. Older schema `1` handbooks are rebuilt in memory from retained
reports and indexes on read, without rewriting the saved history. Historical
project-analysis report schemas remain readable.

All agent-history reads require the current `project_id` and `project_revision`;
the existing optional `worktree_path` selects a registered checkout. Unknown or
duplicate query fields are rejected. Reads are passive and return `no-store`.
Curated memory uses `/api/projects/current/memory`. GET list/detail require
`project_id` and `project_revision`; duplicate or unknown query fields are rejected.
List returns at most 512 records, newest first, with empty `text` and
`source_status: unchecked`. Detail returns private text and rechecks source hashes
and context policy (`source_status: current|changed`). `stale` includes revision
differences and, on detail, changed sources; `expired` remains distinct. These reads
are passive, use `Cache-Control: no-store`, and never accept or refresh a record.

POST collection requires `action: create`, current project identity, `version: 0`
and a `draft`. POST detail supports `edit`, `accept`, and `forget`, with the current
positive `version`. Create/edit require a draft; accept/forget reject a draft.
A draft contains `kind` (`decision`, `lesson`, `verified_outcome`), `title` (120 UTF-8
bytes), `text` (2048 UTF-8 bytes), `sources` (1–64 captured path/hash scopes), and
optional `retention_days` (1–365; default 30). Each scope uses a relative `path`,
`sha256:` content `hash`, and `start_line`/`end_line` (both zero for the whole file,
otherwise an inclusive valid range). Excluded paths and changed identities fail.
Create/edit return a candidate, even when the prior version was accepted. Explicit
acceptance rechecks current source and provenance and increments the version;
forget removes the catalog record and returns JSON `null`. Conflicting versions
return 409, missing/foreign records 404, and unavailable storage 500.

Manual decisions/lessons have `provenance.origin: user`. A retained harness lesson
instead supplies `session_id`, exact proposal `identity`, and zero-based
`lesson_index`, with empty text. A verified outcome supplies `session_id` and
`identity` with empty text and no lesson index; it requires current passing proposal
check authority. The host derives its text from those check receipts. These records
retain session/workflow/proposal provenance and must cite captured proposal source.
Curation never runs checks, sends prompts, changes source, or approves a proposal.

Private atomic storage is limited to 512 records and 8 MiB per checkout. Expired
records become ineligible immediately and are pruned on a subsequent explicit
curation write for that project. Forget deletes the catalog entry immediately;
already produced invocation history and previously sent provider requests are
separate records. Automatic conversation ingestion is rejected.

ADK memory retrieval returns at most eight relevant accepted records, within the
workflow's captured file hashes and declaration spans. Partial/inventory-only
context never grants whole-file advice. Changed source/policy, expired, candidate,
or out-of-scope records are excluded. Revision-only historical advice is labelled
stale in model context. Memory is untrusted advisory data, never an instruction,
permission, or current verification claim. Retrieval consumes a shared tool call;
its context consumes the model input budget. Selected versions and source authority
are rechecked immediately before each provider dispatch, including later tool turns.
Editing or forgetting a selected record blocks further dispatch with the old text.

The artifact library uses `/api/projects/current/artifacts` with the same current
project/revision and optional registered `worktree_path`. GET lists only artifacts
with host-recorded project ownership, including retained change evidence and
research. Arbitrary SDK metadata cannot claim project ownership. Responses include
`project_id`, current `project_revision`, a catalog `snapshot`, `artifacts`, and
optional `next_cursor`. `limit` is 1–100 (default 50); cursors are tied to the
project/revision and exact visible metadata snapshot. A changed snapshot returns
409 and requires restarting pagination. List/detail reads are passive and no-store.

Each version has an opaque `id`, immutable positive `version`, SHA-256 `hash`
(64 lowercase hexadecimal characters), name, MIME, byte size, creation time,
session owner, host provenance, URI and protection flags. `revision_status` is
`current`, `stale`, or `unknown`; `source_status: unchecked` never claims current
source validation. `protected` recovery evidence and `in_use` unfinished-run
references set `can_delete: false`. Arbitrary custom metadata and payloads are
omitted from the inventory. All versions appear independently.

GET `/{artifactID}` discloses `preview_kind: text|binary|research_report`. Valid
UTF-8 text and JSON previews contain at most 64 KiB, trimmed at a character boundary
and labelled `truncated` when needed. Binary payloads remain metadata only.
Research returns its `report_id` without an answer preview; the explicit research
HTML route preserves the citations and Search suggestions together.

POST `/{artifactID}/control` requires current `project_id`, `project_revision`,
`action: export|delete`, exact `version` and `hash`. Unknown fields, including any
caller-supplied destination, are rejected. Export writes a newly named private
0600 file under `.mini-orca/exports/artifacts/` in the selected checkout and returns
`state: exported` with its actual `path`, artifact `id`, version and hash. It never
targets source or overwrites an earlier export. Delete removes only that catalog
version and returns `state: deleted` without a path; explicit exported copies
remain separate. Both serialize against catalog mutations and recheck scope at
persistence. Deletion also checks recovery protection and unfinished runs under
the journal lock. Missing/foreign versions return 404, changed selections and
protected deletion 409, invalid requests 400 and storage failures 500.

The TUI exposes these local APIs through Code / Artifacts and Settings / Memory.
`:library-scope main|task` selects checkout ownership, and `:library-refresh`
reloads metadata. Enter explicitly inspects the selected record. Artifact actions
are `:artifact-export`, `:artifact-delete`, `:artifact-next`, `:artifact-first` and
`:artifact-report`. Memory actions are `:memory-create` or `:memory-edit` followed
by `decision|lesson <days> <title> | <text>`, and explicit `:memory-accept`,
`:memory-reject`, or `:memory-forget`. Reject maps only a candidate to guarded
forget; forget is exposed separately for accepted advice. `:memory-retain`
accepts `outcome|lesson:<one-based number> <days> <title>` for the selected exact
proposal; the daemon derives its text. `:memory-source <one-based number>` opens
at most 64 KiB of current matching source, with scope/hash checks and an explicit
truncation label. `:library-hide` closes private previews and `:library-cancel`
stops owned transport without asserting that an already completed write was undone.

`GET /agent-runs` (under `/api/projects/current`) lists roots newest first, with
`limit` from 1 to 100 (default 50), an opaque `cursor`, and `next_cursor` when more
remain. A cursor is bound to its project/revision. Older-revision records are
marked `stale`; records with no live daemon owner are `interrupted`, never
advertised as running or cancelable. Native `parent_id`/`root_id` group attempts,
parallel reviewers, model invocations and human input. Budgets are shared totals,
so child usage must not be added together.
`usage.input_tokens` and `output_tokens` are enforced budget charges, including
conservative estimates for unavailable consumption. New usage snapshots include
optional `accounting: {version: 1, providers: [...]}`. Each bounded provider group
contains a `model_key` hash of the configured provider/model, call/active/failed
counts, `reported_calls`, `partial_usage_calls`, `unknown_usage_calls`,
`cache_known_calls`, charged input/output, reported input/output, and
`cached_input_tokens`. Cached tokens are included in reported input. Partial
measurements are lower bounds; estimates equal charged minus reported tokens.
Complete usage from a failed native response is retained. Missing usage does not
imply failure, and missing cache details do not imply zero cached tokens. The
projection is absent for historical records whose measurements cannot be
reconstructed. Counts use exact integers; no endpoint, credential or provider
diagnostic is part of the projection. Remote-reported delegation consumption is
separate from this shared local provider budget.
Checked workflows also expose `change_session_id` and `workflow_id` for exact
association with the proposal; legacy or unrelated invocations may omit them.

`GET /agent-runs/{runID}` returns the selected `run`, up to 256 related
`invocations` (`has_more` identifies truncation), and the last 256 retained
source-free `events` (`events_truncated` identifies potentially missing history).
The passive detail also includes `trace` and `cost`. `trace` version 1 correlates
the same shared budget across retries and children using an opaque `trace_id`,
a content `snapshot` hash and up to 256 `spans`. Span `id`/`parent_id` are 16-digit
hex identities; `run_key` and `operation_key` are SHA-256 hashes. Kinds are
`invocation`, `node`, `tool`, `provider`, `retry`, `summary`, `delegation` and
`check`. Status describes execution (`running`, `completed`, `failed`, `canceled`,
`timed_out`, projected `interrupted`, or `unfinished` when an enclosing attempt
has no registered invocation to establish current ownership), not the validity
of findings or checks.
`started` and optional `ended` are timestamps; `elapsed_nanos` is the measured
monotonic duration only when an end was recorded. `legacy`, `incomplete`,
`truncated` and `omitted` preserve unavailable history. The journal keeps at most
4096 spans, never evicting unfinished spans to admit another. Trace retention
does not remove cumulative budget accounting. Reading does not export telemetry.

`cost.status` is `estimated` only when a configured versioned table covers all
measured local token consumption; then `total` is present. Otherwise status is
`unknown`, `total` is absent, and `known_subtotal` prices only known consumption.
`price_version`, `currency` and fixed `reasons` identify the basis: missing table,
legacy usage, missing model rate, incomplete usage or unknown cache split.
These are token estimates, excluding remote agent consumption, other fees and
taxes. No model name, prompt, diagnostic or tool payload is exposed by the spans
or cost projection. Rates are configured locally; see `CONFIG.md`.

`trace.export` is a passive preview: `enabled`, configured `destination`, its
`fingerprint`, `finished` and `unfinished` retained span counts, and an optional
fixed `reason` (`not_configured` or `no_finished_spans`). Legacy or unfinished-only
history cannot be exported. Destination/credential-reference changes invalidate
the destination fingerprint independently of `trace.snapshot`; the credential
value and environment variable name are never exposed by the preview.

`POST /agent-runs/{runID}/export` requires strict JSON containing `project_id`,
`project_revision`, `snapshot`, `destination_fingerprint` and `confirm: true`.
The service rechecks checkout identity, retained content and destination before
one send. Unconfirmed/invalid requests return 400, a missing run returns 404,
and changed scope/snapshot/destination returns 409 without sending. As with
other run routes, `worktree_path` selects its guarded checkout service.

The response is a source-free receipt with the project identity, `run_id`,
`snapshot`, `destination_fingerprint`, `state`, `sent`, optional `accepted` and
`rejected` counts, and a fixed optional `reason`. `sent` is the number of spans
in the attempted payload, not proof of delivery. A valid acknowledgement yields
`exported`, `partial`, or `warning`; `unknown` omits acceptance/rejection counts
because the collector may already have received the data. HTTP, cancellation,
timeout, oversized or malformed replies never become a successful receipt.
No automatic retry occurs. Reading history or a preview never exports.

Export follows the [OTLP HTTP/JSON envelope](https://opentelemetry.io/docs/specs/otlp/)
with hexadecimal trace/span IDs, numeric enums, decimal-string timestamps and
only host-projected attributes: fixed service/scope, snapshot hash, history
flags, omitted/unfinished span counts, outcome, monotonic duration and optional hashed run/operation
identities. It excludes source, prompts, tool values, raw errors, model names,
resource detection and ambient `OTEL_*` settings. Only finished spans with real
end times are included; incomplete/truncated history remains labeled. Request
body, response headers/body and duration are bounded as described in `CONFIG.md`.

An existing domain checkpoint is returned as `input`; superseded or removed
proposals have no input control. No raw SDK session, prompt or model response is
returned by default. Explicit `include_text=true` returns `text` with availability,
`partial_model_output` or `final_model_output` provenance, at most 64 KiB of UTF-8,
and `truncated`. Final output references its immutable completion artifact.
Partial text is transient (at most 64 invocation buffers) and may be unavailable
after eviction or restart. Text has no approval or execution authority.

The same explicit disclosure includes up to 16 `tool_records` from the selected
invocation's last 128 private events. Each record identifies its event, call,
tool name and kind (`arguments` or `result`); `value` is JSON text bounded to
2,048 UTF-8 bytes, with `truncated` identifying omitted text or nested values.
Names and call IDs are bounded to 128 bytes, and nesting to 24 levels.
`tools_truncated` identifies omitted history. Credential-bearing fields such as
authorization, API keys, passwords, tokens and private keys are redacted;
arbitrary prose may still contain private source and must remain an explicit,
local disclosure. Tool records are absent from default details and progress SSE.

`GET /agent-runs/events` returns a JSON replay batch, or SSE when Accept contains
`text/event-stream`. SSE frames use `event: agent_progress`, `id: <cursor>` and
the same JSON batch in `data`. The cursor has an opaque journal epoch and a
monotonic sequence. Pass it as `cursor` or `Last-Event-ID`; conflicting values
are rejected. Batches contain at most 128 events; `has_more` requests immediate
replay. With no cursor, a foreign/future cursor, or a cursor older than the 2,048
retained events, `reset=true` returns a current root snapshot and its new cursor,
without replaying older transitions over that snapshot. Paginate the snapshot's
`next_cursor` if needed. Subsequent batches contain only newer events. Consumers
deduplicate by epoch/sequence and replace state on reset. Detail views expose
retained earlier node transitions separately.

Events carry root `run_id`, native `invocation_id`, parent/node identity, project
revision, time, state, shared usage, input request identity and immutable evidence
references. They contain no source, prompts, arbitrary provider metadata or model
text. Durable terminal transitions survive missed broker notifications. Frames
are capped at 1 MiB, slow writes at ten seconds, and idle streams refresh every
25 seconds to recover dropped notifications. Disconnect/cancel releases the
subscription; reconnect rechecks project scope. Existing `/events` ready/update
subscribers retain their original contract.

`POST /agent-runs/{runID}/control` requires `project_id`, `project_revision` and
`action: cancel`. Only a current active root with `can_cancel=true` is eligible;
the shared budget is canceled and the worker owns cleanup. History cannot create
or retry arbitrary graphs. Waiting input is resolved through
`POST /agent-runs/{runID}/input` with the same `ChangeIdentity`, `workflow_id`,
`request_id` and `decision` as the workflow input endpoint. It routes to the
proposal owner, retaining all fresh source, check, scope and explicit Apply guards.
Missing invocation IDs return 404, stale identities 409, and unreadable journals
500; none is interpreted as empty successful history.

Workflow progress and advisory review are private source-bearing conversation
history. List entries add `workflow_status` without review contents. Schema-1
history without the optional `workflow` field remains readable. After a daemon
restart, unfinished workers read as interrupted and do not automatically dispatch.
Waiting input stays visible without resuming its graph. Resume clears check/review
authority and marks workflow evidence outdated. An unchanged approval checkpoint
can be answered after explicit fresh checks; a new workflow requires fresh
admission. A saved approval receipt never restores in-memory Apply authority.
Progress persistence failures remain
visible and cannot authorize Apply. No provider or project-code execution occurs
when reading progress, restoring history or configuring the workflow.

Grouped writes are journaled before source replacement. A failed write rolls back
when current hashes still match; an unrelated concurrent edit blocks recovery
instead of being overwritten. `prepared`, `undoing` and `recovery_required` receipts
expose interrupted/partial work and must be recovered before another grouped Apply.
A mutation receipt reports the actual state, index (when available), warnings and
Undo availability. Metadata failure after source changes does not erase the receipt.
Undo verifies post-Apply hashes and removes files that the proposal created. The
latest grouped change owns the Undo journal.

`POST /api/projects/current/changes/{sessionID}/verify` accepts `ChangeIdentity`:
current `project_id`/`project_revision` plus applied proposal `revision`/`hash`.
It requires the latest journal to be applied and every post-Apply target and
workspace hash to match. Go verification requires fresh execution trust, then
checks source parse/format and runs fixed `go test ./...` and `go vet ./...` in
an isolated copy. Markdown-only verification checks text/file identities without
executing code. It sends no model request and never changes source. Results carry
`session_id`, project identity, `proposal_hash`, `workspace_hash`, `status`, optional
`reason`, checks and timestamp. Status distinguishes `verified`, `failed`,
`unavailable` and `canceled`; later source/project changes expose evidence as `stale`.
Evidence is private journal metadata and appears in passive recovery receipts as
`verification`. Undo clears it. Failed checks leave applied source in place.
Verification proves only those checks; it does not certify all acceptance criteria
or automatically resolve findings. Explicit focused reanalysis uses the existing
file-analysis endpoint and its own Bug-scope provider consent.

Features use the configured Analyze scope. Generation/goals bodies carry current
project identity, `expected_hash`, `goals` (at most 4096 bytes), and generation's
`confirm_remote_provider`. Triage adds `status` (`open`, `saved`, `dismissed`).
Each generation requests at most five ideas; each includes benefit, evidence,
eligible affected paths, estimated effort and acceptance criteria. Ideas remain
advisory and never enter Bugs/Performance/Security finding counts. Standalone
generation via `POST /api/projects/current/features/generate` accepts an
optional `profile` (catalog model ID or legacy `analyze`, `bug`, `function`;
default `analyze`) to choose the model, and an optional `analysis_selection_id`. When the selection ID
is present, its fingerprint is validated (409 on mismatch) and its exclusions
are applied to the context and suggested paths.

Standalone generation is synchronous and uses the same workflow deadline as
analysis feature discovery: at least ten minutes, or the selected profile's
timeout when longer. The native client waits for the workflow result or caller
cancellation. The daemon suspends its ordinary response write deadline during
generation and restores the bounded write allowance before sending the result.

Every generation, from the Features routes or an analysis run, is additive. The
prompt receives a bounded list of existing idea titles and statuses, newest first
(at most 8 KiB of whole lines, with a count of omitted older titles), and the
token-budget check counts it with the context and goals. After the expected-hash,
project and source checks, the daemon appends only new ideas with status `open`.
A candidate is a duplicate when its `id` matches an existing idea, or its
normalized title matches any existing idea (dismissed included) or an earlier
candidate in the same response. Normalization lower-cases Unicode text, turns
each run of non-letter, non-digit characters into one space and trims; empty
keys never match. Existing ideas keep their order, IDs and statuses, so a
duplicate never reopens a dismissed idea. An empty or duplicate-only response is
a successful generation with no new ideas.

Feature history is `schema_version: 2`. `generations[]` records each successful
generation's `id`, `generated_at`, `project_revision`, `workspace_hash`,
`goals_hash` and `model` (`profile`, `model`, `provider_origin`,
`remote_provider`, or null when unknown); each idea has `generation_id`.
`last_generation` reports `generation_id`, `added_count` and `duplicate_count`
for the most recent successful generation. Records no longer referenced are
pruned on write, and unresolved or duplicate references are rejected as
unsupported history. Each idea's `freshness` is derived on read: it is `current`
only when its own generation's revision, workspace fingerprint and goals hash
match the current values. The report-level `freshness`, `project_revision`,
`workspace_hash` and `context_manifest` describe only the most recent generation,
so a later generation never revalidates older ideas. Version 1 history remains
readable: its ideas share a `legacy` generation built from the v1 report context,
with goals trusted only if v1 marked them current and a null model. The stored
`hash` is kept until the next successful write saves version 2.

The history stays bounded at 64 KiB. A generation that would exceed it adds no
ideas, leaves the file unchanged and fails with `feature history is full;
existing ideas were kept`; a v1 upgrade that would exceed it fails the same way
with the v1 file intact. Failed generation retains earlier ideas with an
explicit failure status. Its `failure` explains whether the provider request,
response format, or suggestion validation failed without storing raw provider
diagnostics or rejected output. If that status cannot fit, the generation error is
returned and the file is unchanged. Cancellation, project/source/goal changes,
a concurrent report change and persistence errors write nothing new.

The web Summary reads saved feature suggestions without generating them. Its
Features count includes open and saved ideas and links to the Features workspace.
Unavailable or ungenerated counts remain unknown; a successful empty report
shows zero. Stale and failed reports retain their status indicators and any
available saved counts. Features keeps the full ideas, estimated effort, error
details and passive handoff to Chat.

## Git and worktrees

Repository/workspace reads require current `project_id` and `project_revision`.
`available: false` represents a non-Git directory; a detached branch or unborn
HEAD stays distinguishable from read failures. Worktree status failures have an
`error` and omit `dirty`; unknown status is never treated as clean. Lists are
bounded to 100 worktrees. Completed entries are applied, committed change history.
Each worktree has `mergeable`, computed against the current branch: both checkouts
must be clean local branches without an operation in progress, the source must be
unlocked and present, and it must have commits not already in the target with a
shared ancestor. Saved tasks must all be applied and committed; unfinished or
failed tasks block merge. This passive eligibility check does not predict conflicts.
The full list remains available even when no worktree is eligible.
Passive status/diff reads disable configured clean/process filters, external diff
commands and text conversion; they do not grant project execution trust.

Git mutations carry that project identity and the displayed `head`/`branch`,
`confirm: true` and `confirm_execution: true`. They also require existing execution
trust for the current project revision, since checkout filters and merge drivers
can execute project code. Hooks, signing, editors and credential prompts are
suppressed. Switch/merge/abort cancel prepared jobs, invalidate review/execution
trust and reindex even after a Git command fails. A command failure can leave
source changes; its receipt preserves `error`, observed repository status and
index/warnings. Transport failures require status/recovery refresh before retry.

Creation adds `target_branch` and an unused absolute `path` directly inside the
current project's `.worktrees/` directory. The daemon creates that directory,
rejects symlinked parents, and adds `/.worktrees/` to Git's local `info/exclude`.
Project indexing, context policy and isolated checks also exclude `.worktrees`;
creating or editing an implementation does not change the original source revision.
It runs `git worktree add` from the captured HEAD; uncommitted content is not copied.
A local inventory marked AI-unavailable allows the new checkout to reopen without
model access. Creation returns its path without changing the active project.

Project endpoints accept an optional `worktree_path` query parameter, an absolute
path to another registered worktree of the active repository. Membership is
rechecked on each request; unrelated, missing and pruned paths are rejected.
`GET /api/projects/current?worktree_path=...` returns that checkout's project identity.
Subsequent requests use its own project ID/revision and ordinary proposal guards.
Each checkout has a separate manager, execution trust, proposal authority and
bounded workflow worker. Reads and navigation never start generation or transfer
consent. The client keeps the original branch selected while
separate implementations run; task Review/Apply/Commit/Undo target only their
captured checkout. Daemon shutdown and explicit project replacement stop these
workers; reindexing or switching branches in the original checkout does not
replace their task state. Existing initialized worktrees outside `.worktrees` remain readable.

Branch switching takes an existing `target_branch`; dirty worktrees and branches
checked out elsewhere block the switch. To return to a branch already checked
out elsewhere, explicitly restore that existing project path instead of switching
the current worktree to the same branch.

Merge preview and merge take the selected worktree `path` and `source_head`.
Preview returns source/target identities, incoming commits and a bounded read-only
diff; it requires no execution trust and never merges. Merge
rechecks both heads (and `source_branch` when supplied) and requires both worktrees
clean and all saved tasks applied and committed. Merge conflicts return
`state: conflicts`, `repository.merge_head` and `repository.conflicts` instead of
success. Resolve them using ordinary Git tools, or explicitly abort by sending
the displayed `merge_head`; abort discards conflict-resolution edits.

After a successful merge, the daemon marks findings linked to applied, committed
fix sessions Done in the original project. It captures the links before merging
and verifies each fix commit belongs to the captured source HEAD. Drafts, undone
fixes, failed commits, commits outside that history and other projects' targets
do not complete findings. Completion is persisted before worktree removal and
survives reindexing, stale analysis and restarts. `GET /api/projects/current/findings`
returns these references with their `commit_oid` in `completed`, independently of
finding filters. Semantic findings also use the existing `status: fixed`; explicit
reopen or dismissal clears their completion. Confidence, verification and freshness
are unchanged. The client labels completed findings **Done** in lists and details.
If saving completion fails, `state` remains `merged`, `warnings` explains the
failure, and the worktree and its task history are retained. Retrying the guarded
merge after fixing storage saves completion even if Git has already merged the HEAD.

The daemon then stops the source implementation worker,
rechecks the source identity and clean state, verifies its commit is in the current
branch, and removes the source checkout with `git worktree remove` without force.
After checkout removal, the source branch is deleted only if its captured commit
is integrated and no registered worktree still uses that branch. Ref deletion
compares the captured HEAD atomically, preserving concurrently advanced branches.
Private `.mini-orca/` task history is locally ignored for removal and deleted
with the checkout. Checkouts containing another
registered worktree are retained. Conflicted, failed and aborted merges never
complete findings or remove the source. Successful removal returns `removed_worktree`
with its absolute path, allowing the client to close the implementation and stop polling it. If
cleanup fails, the merge remains `state: merged` and `warnings` explains why the
worktree remains; `removed_worktree` is omitted. `removed_branch` independently
confirms branch deletion. If only branch deletion fails, the worktree removal
receipt is retained and a warning identifies the branch to inspect in Git.
`completed` contains the finding completions saved by this operation, including
when subsequent cleanup fails; clients must not infer completion from a worktree's
applied history alone. No push happens automatically.

`cleanup-merged` provides the same completion and removal path after an external
merge commit or an earlier cleanup warning, without running `git merge` again.
It requires the current project/revision, destination HEAD/branch, source path,
source HEAD/branch, both confirmations and execution trust. The destination must
be clean and have no active Git operation; the source must be an ancestor of its
HEAD. Unfinished task histories, changed, dirty, locked or nested source worktrees
remain protected. Successful reconciliation also refreshes project facts after
the external merge. After external source changes, explicitly reindex and grant
execution trust for the updated revision before requesting cleanup. Squash
merges and cherry-picks without ancestry are not treated as proof of integration.

Workspace reads add `integration` (`current`, `merged`, `up_to_date`, `behind`,
`unrelated`) and `overlap_paths` for paths changed on both sides of a divergent
branch. These are passive comparisons, without merging or running merge drivers.
Overlap is advisory, not proof of conflict. `merged` includes equal HEADs; it does
not claim a task was verified. The TUI gives unfinished task states precedence
over ancestry, so a failed task at the destination's HEAD is shown as `FAILED`.
Applied, committed work awaiting explicit integration is shown as `TO_MERGE` and
its linked issues are withheld from the issue lists until merged or rejected.
Unknown integration is omitted with `error`.
Refreshing never removes a worktree or completes findings.

Discard takes the selected worktree `path`, `source_head` and `source_branch` plus
the usual mutation identity, confirmation and execution trust. It rechecks the
clean source and branch, stops its worker, and removes the checkout and private
task history without merging or forcing deletion. The source branch and current
checkout remain unchanged. Locked, stale, dirty and nested checkouts are protected
by the same removal guards as merge cleanup. Success returns `state: discarded`
and `removed_worktree`; removal failure returns `state: failed` with `error`.
Discard reopens linked in-progress findings, including applied but unmerged fixes,
unless another task still owns the finding. Any status persistence failure is
reported in `warnings` without hiding the successful removal receipt.

### Managed MCP integrations

GET `/api/projects/current/integrations` requires `project_id` and
`project_revision`. It is passive and returns configured connection details,
configuration fingerprints and optional current inventories, with `no-store`.
States are `configured`, `ready` or `expired`; `ready` means a reviewed inventory
is retained, not a background connection. Credential fields contain environment
variable references only. Tool descriptions are untrusted external metadata.

POST `/api/projects/current/integrations/{integrationID}/control` strictly accepts
`project_id`, `project_revision`, `configuration`, `action`, optional `grant` and
`confirm_connect`. `action: discover` requires `confirm_connect: true`, approving
the displayed endpoint or installed process. Discovery opens and closes a bounded
native MCP session and returns a random grant, `expires_at` (15 minutes) and an
allowlisted inventory with its `fingerprint`, remote names, local aliases, input
schemas and wrapped output schemas. It sends no repository source.
`action: revoke` requires the current grant if one exists, cancels active
operations and returns the passive configured state. Stale configuration/grant
returns 409; missing credentials and rejected protocol requests return errors
without raw server diagnostics. Cancellation joins owned process/network work.

Plan investigations and declaration explanations accept an optional `integrations`
array of at most two unique `{id, grant, schema_fingerprint,
confirm_source_sharing: true}` selections. This explicit per-request consent is
separate from provider confirmation and covers the admitted prompt, source,
skill and memory context that tool arguments may disclose to each selected
server. Grant tokens and credentials are excluded from model prompts. CLI
completion-only profiles reject integration selections. Tool calls recheck scope,
credentials and the discovered inventory, consume shared tool budgets, dispatch
at most once per call, and close their connection before returning. Changed
schemas require fresh discovery and consent. Only configured `read_external`
effects are supported; returned text cannot authorize local changes or become
verified source evidence. Reindex, project replacement, revoke and restart
invalidate grants. The configuration reference documents transport, schema,
process and size limits.

The TUI's Settings / Connections view loads these inventories locally and keeps
main-checkout and selected-task-worktree grants separate. Discovery, incoming
access grants and revocation require explicit confirmation; browsing and refresh
do not contact external destinations or launch processes. A partial inventory
failure preserves the other connection kinds. Canceled or failed control requests
require local refresh to establish the actual grant outcome before reuse.

An editable task plan may select at most two ready MCP/API connections. These
selections are ephemeral and are not saved in proposals or inherited by new
worktrees. Investigation separately confirms provider destinations, connection
destinations and the captured source paths. Changed routing, inventory, grant,
checkout or plan input invalidates consent before dispatch. Expired grants and
adapters without advertised tool support are rejected; incoming access and remote
agent discovery never become investigation tool permissions.

### Curated API tools and external action receipts

GET `/api/projects/current/apis` requires `project_id` and `project_revision` and
returns passive API configurations with configuration fingerprints, `configured`,
`ready` or `expired` states, and optional inventory/grant/expiry. POST
`/api/projects/current/apis/{integrationID}/control` uses `IntegrationControl`
with the same strict identity, discover/revoke and confirmation fields as MCP.
Discovery reads the configured indexed JSON specification locally; it never
contacts the API. Inventories contain an exact fingerprint and selected operations
with ID, method, path, effect, parameters, input schema and successful response
schemas. Credentials remain environment variable references.

Plan/declaration `integrations` selections also accept API IDs under the same
combined two-integration limit and per-investigation source-sharing consent.
GET tools dispatch one bounded external read. Mutating tools return
`approval_required` and `action_id` after saving a candidate; they do not send the
write. The configuration guide defines the supported OpenAPI subset.

GET `/api/projects/current/api-actions` requires current project identity and
returns at most 512 private receipts with `no-store`. Each includes `id`,
`version`, project/revision, integration/inventory/scope identity, `operation_key`,
the exact `request` (operation ID/key, method, URL and optional JSON body), original
`arguments`, `request_hash`, state, timestamps, and current `can_approve`,
`can_reconcile` and `stale` flags. Response JSON/status/hash and user-reported
reconciliation fields appear when available. These records may contain source;
they never include resolved bearer credentials.

POST `/api/projects/current/api-actions/{actionID}/control` strictly accepts
`project_id`, `project_revision`, `action`, `version`, `request_hash`, `confirm`
and optional `reported_outcome`/`observation`. `action: approve` requires
`confirm: true`, the exact current receipt identity, live captured source authority
and current integration grant. Approval claims durable `pending` state before
one HTTP attempt. `completed` means a declared successful response was validated;
`not_sent` means dispatch was prevented; `unknown` means an attempted effect could
not be established. An HTTP 200 control response contains the receipt and does
not imply the external effect completed. Restart, cancellation or receipt-save
failure cannot authorize automatic replay; pending/unknown receipts block new
writes through the same configured operation.

`action: reconcile` requires `confirm: true`, current version/hash, an unknown
outcome (or expired pending lease with no live owner), `reported_outcome` of
`applied`, `not_applied` or `other`, and a nonempty observation up to 2048 bytes.
The user checks the external service before submitting this account. The resulting
`reconciled` receipt is explicitly user-reported; it never retries the request.
Stale/uncertain conflicts return 409, missing receipts 404, malformed requests 400
and journal failures 500. Existing recovery data is preserved on failures.

### Agent settings and skill validation

GET `/api/projects/current/agent-settings` requires current `project_id` and
`project_revision`. Its no-store response contains the configuration fingerprint,
settings `version` (zero before the first edit), `state` (`configured`, `overridden`
or `stale`), effective `roles`, `limits`, operator-configured `ceilings`, configured
skill directories with `enabled` flags, and non-secret configured model metadata.
This local read creates no journal, connection, process or provider request.

POST `/api/projects/current/agent-settings/control` requires the same project
identity, `configuration`, `version`, `confirm: true` and `action: replace` or
`reset`. Replacement requires complete `values`: `roles` with `creation`, `testing`
and `review` selected from `analyze`, `bug` and `function`; all seven `limits`
fields; and an explicit `disabled_skills` array drawn only from configured skill
directories. Positive limits must remain within the configured ceilings; `repairs`
may be zero. No provider, credential, tool, skill directory or retention permission
can be added. Reset omits `values` and follows current operator defaults while
advancing the version, so old editors cannot overwrite a reset.

Settings are private to the canonical checkout and survive restart. Mutation
requires current project/configuration/version identity. Operator configuration
changes mark retained overrides stale; new work then requires an explicit reset,
while retained data remains readable. Malformed requests return 400, stale scope
or version returns 409, and corrupt or failed persistence returns 500 without
replacing retained data. Existing runs keep their admitted roles, budget, skill
selection, context policy and configuration fingerprint. Explicit per-request
model choices still take precedence. A pending pure human-review handoff keeps
its captured settings but rechecks operator, project, policy and proposal identity.

POST `/api/projects/current/agent-settings/skills/validate` requires current
project/configuration/version identity and a configured `directory`. It reads the
indexed manifest and resources through current source/policy guards, uses the
native ADK parser, and returns `state: valid` with bounded metadata, required tool
names and resource hashes/sizes. It returns neither instructions nor resource
bodies, writes no validation grant and executes no scripts. Validation does not
enable a skill or grant its required tools; actual investigations revalidate
source and check tool availability in their own admitted scope. Changed, excluded,
malformed, oversized or unindexed resources remain explicit errors.

### Inbound agent access and optional A2A listener

GET `/api/projects/current/agent-access` requires current `project_id` and
`project_revision`. It returns only this project's configured principals,
credential environment references, capabilities, configuration fingerprints and
states (`disabled`, `configured`, `unavailable`, `authorized`, `expired`). Reads
never start a listener, provider call or execution. Responses are `no-store`.
Only the application instance owning the listener can grant access; separately
opened worktree services report it unavailable. Closing the listener revokes its
grants, and a replacement listener requires a fresh configuration fingerprint.

POST `/api/projects/current/agent-access/{principalID}/control` requires current
project identity, `configuration`, and `action: grant|revoke`. Grant requires
`confirm_connect: true`, `confirm_source_sharing: true`, and
`confirm_remote_provider: true` when a selected capability uses a remote model
(Function for analysis, Bug for proposals). Replacing/revoking a live grant also
requires its exact `grant`. A successful grant returns `state: authorized`, a
new grant identity and `expires_at` 15 minutes later. Scope, credential or model
changes invalidate it. Revocation cancels and joins its active operations;
missing consent returns 400 and stale identity returns 409.

The opt-in listener configured by `integrations.a2a_server` is separate from the
REST API. GET `/.well-known/agent-card.json` publishes generic metadata without
project information. POST `/a2a` requires a single `Authorization: Bearer ...`
header matching exactly one configured principal with a live local grant.
It supports A2A 1.0 JSON-RPC `SendMessage`, `SendStreamingMessage`, `GetTask`,
`SubscribeToTask` and `CancelTask`. Cards and navigation grant no access.

Send parameters contain only `message`, with a unique bounded `messageId`,
`role: ROLE_USER`, and one `{text: ...}` part. The text contains one strict JSON
object, for example:

```json
{
  "project_id": "current-project-id",
  "project_revision": "current-revision",
  "operation": "analysis",
  "question": "Explain the selected function and suggest a focused improvement.",
  "sources": [{"path": "main.go", "hash": "current-source-hash"}],
  "acceptance_criteria": ["Preserve existing behavior"],
  "confirm_remote_provider": false
}
```

`operation` is `analysis` or `proposal` and must be in the principal's configured
capabilities. Question/source limits and freshness checks match explicit external
context admission, with 1–8 existing indexed sources. Source policy, revision,
grant and provider confirmation are enforced before model dispatch and result
publication. Analysis returns an advisory investigation; proposal preparation
returns `advisory`, `proposal_id`, project identity, proposal `revision`/`hash`,
`state` and the local review next action. It never returns Apply/Undo/Merge
authority or invokes project checks. Normal local provider budgets and policy
remain in force.

No client-selected context/task IDs on sends, extension metadata, attachments,
push delivery, list-all-tasks or unrestricted session APIs are accepted. Task
reads, subscriptions and cancellation require the original authenticated grant
owner, including while the task is active. Get accepts `id` and optional
`historyLength` 0–1; subscribe/cancel accept only `id`. JSON-RPC IDs and message
IDs are bounded strings. Duplicate keys, trailing JSON and unknown fields fail
before native SDK dispatch. Invalid payloads return HTTP 400; missing credentials
or project grants return 401. SDK task errors use native JSON-RPC envelopes with
sanitized diagnostics. Failed model work stays failed, never an empty success.

HTTP requests are at most 16 KiB, message text 8 KiB, and result text 16 KiB.
Concurrency, deadlines and a maximum of 128 attempted message IDs bound private
in-memory SDK task/session retention. A duplicate message never dispatches again,
including after failure or disconnection. Disconnect/cancel/revoke/shutdown stop
owned work. Restart clears inbound grants and A2A history without replay; local
proposal/run journals remain available for human recovery. The listener retains
Host/Origin protections and is disabled unless configured explicitly.

### Outbound A2A delegation

GET `/api/projects/current/remote-agents` uses current project identity and lists
configured A2A connections without discovery. POST
`/api/projects/current/remote-agents/{integrationID}/control` uses the existing
`IntegrationControl` fields: `discover` requires `confirm_connect` and current
`configuration`; `revoke` requires any current `grant`. Discovery pins a bounded
A2A 1.0 JSON-RPC card and its exact configured endpoint for 15 minutes. Only
allowlisted text skills are available; required extensions are unsupported.

POST `/api/projects/current/delegations` requires `project_id`,
`project_revision`, `remote: {id, grant, schema_fingerprint,
confirm_source_sharing: true}`, `kind: analysis|review`, an advertised `skill`,
`question` (up to 2048 UTF-8 bytes) and `confirm_delegate: true`. Optional `sources`
contains up to eight explicit `{path, hash}` selections; optional `stream` requests
advertised SSE. The combined prompt is bounded by 64 KiB and the Function context
estimate. It contains only this admitted question/source context. Each operation
revalidates the card; it never obtains ambient credentials, history or memory.

A durable private receipt records message/attempt identity before a single send
and captures validated remote task/context IDs as they arrive. The response is
an advisory receipt, including explicit `not_sent`, `unknown`, `input_required`,
`auth_required`, `submitted`, `working`, `completed`, `failed`, `rejected` or
`canceled` state. A 200 receipt response does not imply remote completion.
Only bounded plain text is retained: at most four artifacts and 64 KiB combined,
with `complete` flags set by final artifact chunks or a completed task status;
interrupted streams retain incomplete flags. SSE has 512 KiB and 128-event limits. Remote internal model
and search call counts remain unknown. Optional valid token metadata is labelled
`remote_reported` and retains the latest supplied report, while absent metadata
is `unknown`. These reports do not establish independently measured cumulative
usage. Each card/RPC exchange
consumes a host tool allowance. Remote output cannot supply local tool actions,
permissions, verified findings, source Apply or Undo authority.

GET `/api/projects/current/delegations` returns project-scoped metadata, with
question, observation, text and artifact bodies omitted or empty. GET
`/api/projects/current/delegations/{delegationID}` explicitly discloses private
advisory text and locally rechecks source freshness. Both require current project
identity and send `Cache-Control: no-store`; neither contacts the remote service.
The list's `stale` flag checks the project revision; detail also checks actual
source hashes. `active` describes this daemon's current operation owner.

POST `/api/projects/current/delegations/{delegationID}/control` requires current
project identity, receipt `version`, `action` and `confirm: true`. Network actions
also require a live `remote` grant for the exact saved endpoint/card:

- `refresh` uses GetTask and `reconnect` uses SubscribeToTask; neither resends a
  message. `cancel` targets the retained remote task even if captured sources
  have since changed. These actions contain no reply, observation or stream flag.
- `input` is allowed only for `input_required` and unchanged source authority.
  It sends one new message with a nonempty `reply` up to 2048 bytes and fresh
  `remote.confirm_source_sharing`; optional `stream` requires advertised support.
- `reconcile` records a nonempty user-reported `observation` up to 2048 bytes for
  `unknown`, `auth_required`, or expired `pending` work. It makes no network call,
  cannot race a local owner, and preserves prior remote IDs and advisory data.

Canceling a request closes local transport and attempts bounded remote cleanup.
Unconfirmed outcomes remain unknown. One unfinished receipt per endpoint blocks
new sends; up to 128 receipts are retained per checkout. Restart never resumes or
replays pending work. Conflicts return 409; private persistence failures return
500 with existing recovery records preserved. See [configuration](../CONFIG.md)
for the supported protocol and credential subset.

### Grounded external research

GET `/api/projects/current/research` requires current project identity and returns
`state: available|disabled|unsupported_profile`, Function model ID and private
report metadata. It performs no provider request. POST to the same route accepts
`project_id`, `project_revision`, `question`, optional `sources: [{path, hash}]`,
`confirm_search`, `confirm_source_sharing` and `confirm_remote_provider`. Search and
sharing confirmations must be true; remote confirmation follows the configured
destination. The question is at most 2048 UTF-8 bytes and sources at most eight
current indexed files. Source/policy/revision checks run before dispatch and
publication. Disabled/unsupported search, invalid sources or missing consent fail
without provider work. Search runs separately through native ADK with no function
tools, retries or provider caching.

Success returns only `{id, project_id, project_revision, title, state, model,
created_at, stale}`. States `grounded`, `uncited` and `ungrounded` retain the
provider's attribution limits; none establishes repository findings. GET
`/api/projects/current/research/{reportID}/report`, with current project identity,
returns private HTML containing the answer, inline and aggregate citations,
reported queries, original Google Search suggestions and explicit unknown
provider-internal search usage. Model token usage is displayed only when known.
The report uses `no-store`, `nosniff`, `no-referrer`, a restrictive CSP and a
script-free sandbox for original suggestions. Unsupported active suggestion HTML
or invalid citation ranges prevent publication. No external images/resources are
loaded during preview; links are deliberate user navigation.

Report reads flag changed source/policy or project revisions as stale. The list's
`stale` field compares revisions only; opening a report also checks its admitted
sources. Generic run text for `grounding-v1` returns
`state: research_report_required` without the answer, preserving the citation and
Search-suggestion display contract. Missing reports return 404. All routes retain
the normal loopback and optional worktree scope policy.

## Mission plans and explicit review

Mission plans are private planning documents under
`/api/projects/current/missions`. Listing, reading, saving, revising and reviewing
plans do not call providers. Explicit generation uses the separate `/missions/plan`
operation described below. None of these operations execute project checks,
create worktrees, launch children or change project source.
Approval records review of a plan; it does not grant Apply or Undo authority.

GET list and detail require current `project_id` and `project_revision` query
parameters. The shared router consumes optional `worktree_path` before invoking
the selected checkout's handler. Every successful response uses `no-store`.
List returns the current identity, `catalog_revision` and at most 128 `missions`
summaries: ID, stored revision, goal, plan hash, child/review counts, approval and
update time. It omits child approaches and criteria. Its `source_status` is
`unchecked`; `stale` on a summary only compares stored and current revisions.

Detail returns `catalog_revision`, the private `mission`, `stale` and
`source_status`. The mission contains project/revision, context-policy and
effective-configuration fingerprints, instruction fingerprints for each path,
the complete `plan`, `plan_hash`, reviewed child IDs, approval and timestamps.
Detail rechecks its bindings: `current` means those checks passed, `changed`
means a captured identity no longer matches, and `unavailable` means a binding
could not be checked. Changed and unavailable details remain readable with
`stale: true`; they cannot be reviewed, revised or approved. After source or
settings changes, create a new plan using fresh captures.

A plan contains a nonempty `goal` of at most 4096 UTF-8 bytes, 1–64 distinct
`sources`, explicit `limits`, 1–16 `children`, and `integration_order`. Each
source contains a canonical relative `path`, `exists`, and a `sha256:` content
`hash` when present (empty when absent). Existing source must match the project
index and current disk content. Capture is bounded to 256 KiB total source;
source text is not stored in the mission catalog. Context exclusions, unsafe
paths and symlinks are rejected. Effective `AGENTS.md` identities are captured
for present and absent paths.

Each child contains an ASCII letter/digit/underscore/hyphen `id` of 1–64 bytes,
a `title` of at most 120 bytes, an `approach` of at most 4096 bytes, 1–16
`acceptance_criteria` of at most 512 bytes each, `dependencies`, `write_paths`,
`read_paths` and explicit `limits`. Each child has 1–8 writes and at most eight
reads, all distinct within that child and included in the captured inventory.
`integration_order` includes every child exactly once in topological order.
Unknown, repeated, cyclic and self dependencies are rejected. Children sharing
write/write or write/read access require a dependency path between them;
integration order alone does not authorize shared access. Reading an absent
path requires an ancestor that writes it. Disjoint children may be independent.

Limits require every field from `AgentLimits`, including an explicit zero when
repairs are disabled. Each child must fit the mission budget, and the sum of
child model calls, tool calls, input/output tokens and repairs must fit it.
Elapsed time and concurrency are ceilings, not summed across children. The
mission budget must fit the effective operator/project limits. Saving a plan
does not reserve or consume an execution budget.

POST uses strict JSON (rejecting duplicate keys, unknown fields and trailing
values) with `project_id`, `project_revision`,
`catalog_revision`, `plan_hash`, `action`, optional `plan` and `child_id`, and
`confirm` for review/approval. Actions are:

- `create` at the collection route: empty hash and a complete plan; use the
  current catalog revision (zero for an empty catalog). The server creates the
  mission ID and captures policy, instructions and settings.
- `revise` at the selected mission: exact saved hash, current catalog revision
  and a complete replacement plan. Every revision clears all reviews and
  approval, including a replacement with identical content.
- `review`: exact saved hash and current catalog revision, one `child_id` and
  `confirm: true`. A child can be reviewed only once per plan revision.
- `approve`: exact saved hash and current catalog revision, no child ID and
  `confirm: true`. Every child must already be reviewed. Approval is separate
  from the last child review.

All durable mutations recheck project, present/absent sources, context policy,
instructions and effective settings at publication. Successful mutations return
the same detail shape and increment the checkout-wide catalog revision, so a
concurrent change to any mission rejects an older editor. The plan hash covers
the complete plan and captured bindings, excluding timestamps and review state.
The private version-1 catalog is workspace-bound, stored atomically with mode
0600, and bounded to 8 MiB; each submitted mission is bounded to 256 KiB.
Corrupt or unknown-schema catalogs are preserved and reported as unavailable.

Invalid plans, action shapes or missing confirmation return 400. Excluded source
returns 403, missing missions 404, changed project/source or catalog/hash
identities 409, and private-catalog read/write failures 500. A canceled request
cannot publish a new plan or review. A successful response reports the saved
planning decision, never an executed child or a verified implementation.

### Mission execution preparation

`GET /api/projects/current/missions/{missionID}/execution` requires the current
`project_id` and `project_revision`, supports the selected `worktree_path`, and
returns 404 when the mission has no execution receipt. It reads private metadata
and Git identity without starting work. The response contains `execution`,
`stale`, `source_status` (`current`, `changed`, or `unavailable`),
`ready_children` and `owner_state` (`idle`, `active`, `waiting_for_owner` or
`recovery_required`). Readiness excludes reserved children and dependencies
whose proposals have not been explicitly integrated. Changed
source, project revision, instructions, policy, settings, catalog approval or Git
base keeps the frozen receipt inspectable with `stale: true`. Missing or corrupt
execution storage is not reported as a completed or empty execution.

`POST` at the same route accepts a strict object with `project_id`,
`project_revision`, `action`, `plan_hash`, and `confirm: true`:

- `prepare` additionally requires the approved `catalog_revision` and `base`
  containing the exact Git `head` and `branch`. `execution_id` must be absent or
  empty and `execution_revision` zero. Every child must be reviewed and the
  complete mission separately approved. The checkout must be a clean Git root
  on a committed local branch, without a merge, rebase or other pending operation.
- `start` requires the exact `execution_id` and `execution_revision`, zero
  `catalog_revision`, no `base`, and `confirm_execution: true`. The project
  check commands and workspace must already be trusted. `confirmed_profiles`
  contains only the selected role profiles that require remote confirmation;
  duplicate or unselected grants are rejected. Every child needs an explicit
  test write path. Start admits one ready wave and returns its durable receipt.
- `pause`, `cancel` and `reconcile` require `execution_id` and
  `execution_revision` from the current receipt. `catalog_revision` must be
  zero and `base` absent. They reject provider or execution grants.
  Pause stops an active wave and preserves unused pending children. Cancel
  permanently closes the execution, including while it is pausing.
  Cancellation uses the frozen plan hash and remains available after sources,
  settings or the editable catalog plan change. It still requires the current
  project identity and cannot control another checkout's receipt.

Preparation creates a receipt in a version-4 catalog with state `prepared`, an independent
`execution_revision` of 1, the approved `catalog_revision`, a full frozen
`mission`, the exact Git `base`, and `children` containing each child's ID,
`pending` state and fixed approved `limits`. The plan already bounds the sum of
child model calls, tool calls, input/output tokens and repairs. These are future
allocations, not measured usage. Preparation and inspection grant no provider,
project-code execution, worktree creation, Apply or merge authority. Existing
version-1 prepared/canceled, version-2 wave and version-3 integration receipts remain readable and
upgrade on the next explicit mutation without resetting allocations. The initial
`current` checkpoint copies the original revision, base, sources and instructions.

Start freezes the effective settings and runs a native `mission-graph-v1` ADK
wave. Before worktree creation, it records each child's `wave_id`, deterministic
`directory`, `branch` and `base_head`. The sum of selected whole-child
concurrency allocations cannot exceed the mission limit. Each child runs the
existing checked-change workflow with its own fixed calls/tokens/tools/repairs
allocation. Context contains only approved write/read targets and applicable
instructions; implicit sibling declarations and broader cached context are
excluded. A proposal's `mission_origin` binds its mission, execution, child and
plan hash. Ordinary model retries on that proposal cannot mint a new allocation.

Child states are `pending`, `reserved`, `running`, `awaiting_review`, `rejected`,
`failed`, `interrupted` or `integrated`. Child receipts retain project/session/workflow and
proposal identities when available, plus a bounded `reason`. A successful child
produces a checked proposal awaiting separate human diff review and Apply. Start
never applies source or merges a worktree. Dependent children remain pending
until an explicit integration operation records their prerequisites.

Execution states are `prepared`, `running`, `pausing`, `canceling`, `paused`,
`canceled`, `awaiting_review`, `blocked`, `interrupted`, `integrating`, `revalidating`, `ready` or
`completed`. An active `wave`
contains its opaque owner, selected children, start time and millisecond
allowance. `elapsed_millis` charges active wave time; waiting for human review
does not consume it. Failed or canceled attempts retain their full fixed child
allocations. Bounded `failure` text explains an incomplete wave without source
or provider diagnostics. Owned workers and their child processes are drained on
pause, cancellation and shutdown.

Restart never resumes provider work. Another owner's wave remains
`waiting_for_owner` until its bounded deadline expires; a stopped local owner
or expired lease permits explicit `reconcile`. Reconciliation inspects the
registered worktree, exact Git identity and matching proposal origin/workflow.
Complete, current proposal evidence is retained; missing, changed or uncertain
evidence becomes `interrupted`. Recovery charges the entire expired wave
allowance, retains all child reservations, and performs no provider requests,
checks, source writes or cleanup. Control requests always use the latest
execution revision, including revisions changed by child progress.

There is one retained execution per mission identity. Repeated preparation,
plan edits, cancellation and daemon restart cannot issue another allocation for
that mission. A different attempt requires a new mission with fresh review.
Canceling a prepared execution changes its state to `canceled`, increments its
execution revision, and retains the frozen plan and all child allocations.
Replaying an old preparation or cancellation returns 409.

### Mission integration and recovery

The execution route also accepts `integrate`, `reconcile_integration` and
`cleanup_integration`. Each requires the current project identity, frozen
`plan_hash`, exact `execution_id` and `execution_revision`, zero
`catalog_revision`, the displayed destination `base`, `confirm: true` and
`confirm_execution: true`. The destination must have current execution trust.
Provider grants are rejected. The `integration` object contains `child_id`,
`session_id`, `workflow_id`, `proposal_hash`, `proposal_revision`, canonical
registered worktree `path`, and `source` with the exact committed `head` and
`branch`. Recovery and cleanup additionally require `integration_id`; new
integration rejects it. Other execution actions reject this object.

Integration requires the next child in the approved integration order, checked
against `execution.current.base`. The exact proposal must have passed checks,
per-file human review and explicit Apply/commit. The single child commit must
have the recorded base as its parent and match the reviewed file contents and
modes, with no extra paths. A parallel child checked on an older base remains
blocked for explicit revalidation. Ordinary worktree merge, cleanup and discard
routes reject mission-owned checkouts, including reservations with no proposal.
Recorded branch ownership and retained proposal origins also prevent moving or
renaming a checkout from bypassing mission integration or discarding its evidence.

Before merging, the host saves the complete applied proposal as a protected
private artifact and records a pending integration intent. The artifact reference
in `integrations[].evidence` contains `key` (`session` and `name`), exact `version`
and content `hash`. This preserves checks, review, workflow, source and commit
provenance after worktree removal. It is source-bearing private evidence and is
not included in source-free run diagnostics. The merge is fast-forward only.

The original `mission`, plan hash, initial `base` and child allocations remain
unchanged. A verified merge records a new `current` checkpoint with its observed
`project_revision`, Git `base`, `sources` and `instructions`. Publication rechecks
source, catalog approval, settings, policy and Git identity. Only approved child
paths may change. The child becomes `integrated` and dependent children become
eligible only after this receipt is saved. Later starts use the new checkpoint
with the original budgets and fresh execution/provider consent. All children
integrated yields `completed`; cancellation still prevents subsequent starts.

Integration receipts have `pending`, `merged`, `failed`, `conflicts` or
`unavailable` state. A bounded owner lease prevents another daemon from recovering
an active operation. Restart never retries merge. Explicit reconciliation inspects
the actual clean destination, exact commit parent/tree and protected proposal;
only the recorded source commit can advance the checkpoint. An unchanged base
records `failed`; changed, conflicting or unavailable evidence remains blocked.
A missing worktree alone never proves integration. Resolve or explicitly abort
conflicts with the ordinary Git recovery controls before reconciliation.

Responses to integration controls may contain `git`, the observed Git mutation
result including its new index/repository and warnings. A successful Git effect
with failed receipt publication retains the checkout, blocks dependencies and
reports the unsaved receipt; it is never presented as a source no-op. Receipt
publication precedes cleanup. Cancellation after a merge still records the
observed commit, but defers checkout removal until explicit cleanup. `cleanup`
is `retained`, `pending`,
`worktree_removed`, `removed` or `failed`. Untracked edits, changed branches,
missing artifact evidence and write failures retain recoverable state. Explicit
`cleanup_integration` verifies ancestry and exact ownership before retrying only
cleanup. It never merges, invokes providers or resets allocations. Previously
removed checkouts and branches are reconciled from Git evidence, not assumed.

Preparation rechecks plan/catalog identity, current source hashes, instructions,
policy, effective settings and Git base at atomic publication. The private
workspace-bound execution catalog uses mode 0600, is bounded to 8 MiB and 128
receipts, and preserves corrupt or unsupported schemas. Reads and responses use
`Cache-Control: no-store`. Invalid action shapes or missing confirmation return
400, absent receipts 404, stale identities or concurrent controls 409, and
private storage failures 500. Canceled preparation cannot publish a new receipt;
an already attempted integration still records its observed Git outcome.

### Mission proposal revalidation

The same execution route accepts `revalidate` and `reconcile_revalidation` with
the current project identity, frozen `plan_hash`, exact `execution_id` and
`execution_revision`, current checkpoint `base`, `confirm: true` and
`confirm_execution: true`. `catalog_revision` must be zero; provider grants and
an `integration` object are rejected. The `revalidation` object contains
`previous`, using the integration proposal identity fields for the retained
child, canonical path and old checkout HEAD (a draft's base or its exact reviewed
commit). `previous.integration_id` is rejected. Recovery additionally requires
`revalidation_id`; new attempts reject it.

Only an owned `awaiting_review` child checked against an earlier mission base
can be revalidated. The current destination must still match the verified
mission checkpoint, sources, instructions, configuration and execution trust.
Unrelated external changes require a new reviewed mission. Before creating any
checkout, the service protects the previous complete proposal as an artifact
and reserves a bounded attempt with the old identity, source commit, current
checkpoint, owner and new candidate directory. Pending attempts block starts
and integrations. At most three attempts per child are retained; failed and
interrupted attempts count. Their elapsed time consumes the existing mission
allowance and is capped by the child's original elapsed limit. Model/tool/token
and repair allocations never change.

Each attempt creates a new owned `-rN` checkout and change session, copying only
proposal content and approved read/write scope. The original checkout, commit
and historical model evidence remain retained. The new origin includes the
exact `revalidation_id`; its `generation` is positive and `workflow_id` is
empty. No provider is invoked. Fresh tests and lint run in the existing isolated
check workspace under the current scoped execution grant. Checks, model-review
claims, plan approval and human review are never copied from the old proposal.
Opening and checking a candidate cannot apply source. The explicit final human
Review approves its newly captured plan only after current checks and per-file
review; Apply/commit and mission integration remain separate explicit actions.

`revalidations[]` records `pending`, `awaiting_review`, `failed` or `interrupted`,
the `previous` and `candidate` child receipts, old `source`, protected `evidence`,
`before` checkpoint, allowance, owner and timestamps. Only a successfully checked
candidate replaces the selected child. Failed candidates retain their own
session/check evidence without replacing the preceding proposal. Integration
then requires the exact selected candidate at the current base, including its
empty workflow ID. Historical and unsuccessful checkout reservations remain
protected against ordinary Git merge/discard controls.

Request cancellation, mission cancellation, reindex, project replacement and
shutdown revoke owned revalidation work. Reads never recreate candidates or
rerun checks. Explicit recovery is available after the local owner stops or a
foreign owner's allowance expires; it records `interrupted`, charges the full
reserved allowance when elapsed work is uncertain, and retains any existing
candidate and archive without promoting verification or executing anything.
Missing checkouts or incomplete session publication remain visible uncertainty.
Older execution catalogs are upgraded on read without writes; the next explicit
mutation persists version 4 and no allocation is reset.

### Explicit mission decomposition

`POST /api/projects/current/missions/plan` runs a cancelable native ADK planning
invocation using the Function profile. It requires `project_id`,
`project_revision`, current `catalog_revision`, `goal`, captured `sources`,
mission execution `limits`, separate `planning_limits`, and
`confirm_remote_provider` when required by that profile. The optional worktree
selector follows the other mission routes. JSON decoding is strict. The host
fixes the goal, inventory and execution limits; the model supplies only the
children and integration order.

Both budgets must fit effective operator/project limits. Planning additionally
permits at most eight model calls, 24 tool calls, 1,048,576 input tokens,
8,388,608 output tokens and 300 seconds, with `concurrent_calls: 1` and
`repairs: 0`. Every limit field is required. Provider retries and tool turns
share that planning budget; the Function timeout can shorten it. Planning cost
is separate from the future execution allocation and does not authorize child
execution. Unknown provider usage retains conservative reservation accounting.

For tool-capable profiles, the native agent can use only the captured repository
read tools: `read_snippet`, `search_repository`, `inspect_symbols`,
`inspect_imports` and `inspect_impact`. Completion-only profiles receive the
bounded captured source through their existing adapter. Applicable instruction
text accompanies either path. Initial prompt text must fit both the Function
context ceiling and 128 KiB; the captured source inventory retains its 256 KiB
bound. Planning never invokes project checks or external integration tools.

Project, source, instruction, policy, settings and catalog identities are
rechecked at dispatch, tool boundaries and publication. The entire returned
graph must pass the same domain validation as a manually authored mission,
including dependency reachability, absent reads and aggregate child budgets.
A successful response contains the normal mission detail plus `input_hash`,
`prompt_version: mission-planner-v1`, admitted `planning_limits`, cumulative
`usage` and `context_manifest`. It always saves a new, unapproved draft with no
reviewed children and increments the catalog revision.

Cancellation, changed identities, a full/unavailable catalog, malformed output,
truncation or an incomplete decomposition cannot publish a usable mission or
replace an existing draft. Invalid or incomplete decomposition returns 400 with
`mission planning incomplete`; stale catalog or captured identities return 409.
Provider/transport failures and request cancellation retain the normal error
contract. Retrying is a new explicit planning request against current captures.

### Explicit live sessions

`GET /api/projects/current/live-sessions` requires current `project_id` and
`project_revision`. It returns an exact binding (`project_id`, `project_revision`,
`configuration`, `settings_version`, `model`, `provider_origin`), `destination`,
shared `limits`, fixed live bounds and `sessions`. Availability is `available`,
`disabled`, `unsupported_profile` or `stale_settings`. The optional
`agent_runtime.live_model` defaults to empty; the conformed combination is Gemini
API `gemini-3.8-live` using the Function profile's explicit API key/destination.
No provider calls occur during list/detail or artifact disclosure.

`POST /api/projects/current/live-sessions` accepts that exact binding plus
`confirm_connect: true` and `confirm_remote_provider` (true for a remote Function
profile). It creates a durable, source-free receipt before opening an owned ADK
live runner. It grants no project tools, code execution or source changes.
Request cancellation during connection closes and joins its owner.

`POST /api/projects/current/live-sessions/{liveID}/control` accepts the exact
binding plus current `control_revision` and `action`:

- `send`: supply `text` or one `attachment: {id, version, hash}`, with
  `confirm_source_sharing: true` and applicable `confirm_remote_provider`.
  Text is 1–2048 UTF-8 bytes. Media is a current imported JPEG or raw little-endian
  16-bit mono PCM (`audio/pcm;rate=16000`), at most 64 KiB; no URLs, paths or implicit
  format conversion are accepted. Admission is repeated before each media frame.
- `interrupt`: explicitly interrupt the provider turn; it consumes one control
  turn and sends no conversation history.
- `reconnect`: requires `confirm_connect: true` and applicable remote confirmation,
  a still-owned `failed`, `disconnected` or `go_away` session and remaining budget.
  It creates a fresh connection without replay, resumption handles or refreshed
  lifetime/token limits. It does not resend the previous input.
- `close`: close and join the owner. No text or attachment is accepted for
  interrupt, reconnect or close. A repeated/stale control must refresh first.

Every accepted control claims its revision durably before its effect. A stale
checkout/settings/model/origin/revision or context policy is rejected. History
marks a changed binding or policy as `stale`; close remains available for an owned
session in the current checkout. Controls return the updated
session metadata. `GET /api/projects/current/live-sessions/{liveID}` adds
`output_artifacts` and `missing_artifacts` for explicit library disclosure.
Both reads and successful controls use `Cache-Control: no-store`.

Session metadata includes `id`, journal `version`, `control_revision`, immutable
binding and context `policy`, `state`, `owned`, `stale`, `can_reconnect`,
`retention_failed`, `connections`, attempted `turns`, provider `completed_turns`,
attempted `sent_bytes`, received payload bytes, cumulative `usage`, separately
reported partial token counts, ADK `run_ids`, artifact references and timestamps.
States are `connecting`, `connected`, `disconnected`, `go_away`, `closed`,
`interrupted`, `limit_reached` or `failed`. Output is advisory; a provider turn
completion does not establish verified project evidence.

The owner permits five minutes and two connections, shortened by shared run
limits. Per connection: eight send/interrupt controls, 128 KiB outgoing payload,
512 KiB incoming payload, 128 KiB per incoming frame, 256 incoming messages and
64 KiB retained text. Conservative byte-based input/output token reservations
include 16 KiB of transport allowance per direction and are charged as unknown
usage without a final receipt. `usage_reported` and `reported_*_tokens` are partial
provider observations, not final billable totals. A failed attempt still consumes
its reserved budget. Up to four sessions can be owned simultaneously per checkout.

Connection end saves private transcript and raw 24 kHz mono PCM output artifacts;
metadata never contains transcript, audio, credentials or provider resume handles.
Audio requires explicit local export, with no capture or playback. Output errors
retain failure state; successful input dispatch does not imply successful output
retention. Reindex, checkout replacement, settings changes, shutdown, policy/budget
failure and receipt persistence failure cancel the owner. Restarted active or
disconnected history is `interrupted`, unowned and never automatically resumed.
Only same-process owned sessions can explicitly reconnect. Up to 128 receipts are
retained locally; reaching capacity rejects new sessions without silently evicting
evidence.

Strict bodies reject unknown fields, duplicate keys, trailing JSON and bodies over
1 MiB. Invalid controls/consent return 400, missing detail 404, stale identity or
unowned control 409, oversized body 413, unavailable persistence 500 and transport
or malformed provider exchange 502. Failed requests never automatically retry.
The same routes are isolated through the existing `worktree_path` selector.

### Explicit multimodal attachments

`GET /api/projects/current/attachments` requires current `project_id` and
`project_revision`, with the usual optional registered `worktree_path`. This
passive, no-store overview returns `state: available|unsupported_profile|stale_settings`,
the configured Function `model`, `configuration`, `settings_version`, effective
run `limits`, and at most 128 newest imported `attachments`. `truncated` means
older versions remain accessible through the paginated artifact library. Prior
revision imports remain visible as stale and cannot be sent as current context.

`POST /api/projects/current/attachments` imports supplied bytes only. Its strict
JSON body is `{project_id, project_revision, name, mime, data, confirm_import}`;
`data` is base64 and `confirm_import` must be true. Names are plain display names,
not paths. No URL or daemon filesystem path is accepted. The 1 MiB HTTP body
limit remains in force. Decoded content is limited to 512 KiB. Supported MIME
values are `image/png`, `image/jpeg`, `application/pdf`, `audio/wav`, and
`audio/pcm;rate=16000`. Images must fully decode within 4096 pixels per side and
4 megapixels. PDFs require a supported header and complete EOF envelope; this
is not a semantic PDF validator. WAV must have complete RIFF chunks containing
16 kHz mono signed 16-bit PCM. Raw PCM must contain complete little-endian samples.
The receipt is an `AgentArtifact` with immutable ID/version/hash and project,
revision and context-policy provenance. Import performs no provider request.

`POST /api/projects/current/attachments/advice` accepts only:

```json
{
  "project_id": "current-project-id",
  "project_revision": "current-revision",
  "configuration": "configuration-from-overview",
  "settings_version": 0,
  "model": "gemini-2.5-flash",
  "provider_origin": "https://generativelanguage.googleapis.com",
  "question": "Explain the attached diagram.",
  "attachments": [{"id": "exact-artifact-id", "version": 1, "hash": "exact-sha256"}],
  "confirm_source_sharing": true,
  "confirm_remote_provider": true
}
```

The model and origin must match the overview; the configuration fingerprint binds
its full configured destination and credentials without disclosing them. The
question is bounded to 2048 UTF-8 bytes. Select one to four distinct explicitly
imported versions, at most 1 MiB combined. Source-sharing confirmation is always
required; remote-provider confirmation follows the configured Function profile.
Foreign, deleted, protected, stale, non-imported or altered artifacts are rejected.
Raw PCM imports cannot be used for completion advice; use WAV audio. Unsupported
profiles or MIME fail before provider work.

The daemon checks project/revision, context policy, settings version and exact
artifact ownership/hash before dispatch and before publishing. It makes one
native ADK completion with inline bytes and the shared run budget, without tools,
retries, memory, skills, compaction, provider caching, source writes or Files API
uploads. Reindex, project replacement, request cancellation and shutdown stop
owned work. The private journal uses `multimodal-v1` and hashes a media manifest;
binary payloads remain in artifacts and are omitted from progress metadata.

The receipt is `{state: "advisory", artifact: AgentArtifact, model, usage,
usage_known}`. `usage` contains `prompt_tokens`, `completion_tokens`, `total_tokens`
and optional cache-token details. Zero values with `usage_known: false` are not
measured zero usage. The answer and question are absent from the receipt. Explicit
artifact detail discloses bounded private text with selected-version provenance;
ordinary export retrieves the full private answer. This is model advice, never
verified evidence or an approved source change. Existing artifact deletion and
export apply; local deletion cannot retract already transmitted provider bytes.

Invalid input is 400, oversized HTTP bodies 413, missing/foreign versions 404,
changed identities 409, unavailable storage 500, and provider exchange failure 502.
Failed and canceled requests do not publish an advisory receipt. Private invocation
history may retain the actual earlier provider exchange for diagnosis. Reads,
inspection and export never reconnect, resubmit media or regenerate an answer.
