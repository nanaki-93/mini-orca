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
| POST | `/api/projects/current/files/explanation` | Explicitly explain one exact indexed Go declaration using transient Function-scope context. |
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
| POST | `/api/projects/current/apply` | Apply one validated, checked declaration draft only after `confirm: true`. |
| POST | `/api/projects/current/undo` | Restore only the immediately preceding unchanged apply after `confirm: true`. |

Declaration explanation requests bind `project_id`, `project_revision`, `base_file_hash`, `target_path`, and `target_symbol`. The target must resolve to one exact atomic declaration in an eligible indexed Go file. A non-loopback Function provider also requires `confirm_remote_provider: true` on that request. The daemon rechecks the project and file identity after provider work before returning the bounded explanation, source line anchor, optional engineering insight, and `ContextManifest` provenance.

Explanation responses are transient. This route does not create chat sessions or drafts, persist chat or analysis history, run checks, or change Apply/Undo state. Cancellation, malformed provider output, and stale request identity return an error without publishing a result.

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
