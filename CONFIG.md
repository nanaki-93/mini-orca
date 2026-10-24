# Configuration Guide

Mini-Orca reads one YAML file, normally ignored `config.yaml`. Start with
`config.example.yaml`; do not commit provider credentials. Configuration is
validated once at daemon startup, and unknown or retired keys stop startup with
the full field path in the error. JSON configuration files are not supported.

The daemon configures three model profiles, used by default as follows:

| Scope | Used for |
| --- | --- |
| `analyze` | Project import, architectural summaries and source-based Performance review |
| `bug` | Selected-file and unified semantic analysis |
| `function` | Declaration proposals and explicit repairs |

The Chat agent workflow chooses a model independently for Creation, Testing
and Review. Their prompt scopes are `bug`, `function` and `analyze`, respectively. Choose
configured profiles or discovered Agy/Pi catalog models on **Models**. A captured run
keeps its selected model, timeout and retry budget; remote consent covers every
selected remote profile. These selections do not change other operations' defaults.

All three scopes are required. Each selects a `provider` (`openai`, `gemini`,
`vertex`, `agy`, or `pi`) and a `model`. Omitted `provider` retains the OpenAI-compatible HTTP
transport. HTTP profiles have a Chat Completions-compatible
`api_base_url`; an empty `api_key` is valid for a loopback local
server. `api_base_url` must be an absolute HTTP or HTTPS URL without user
information, a query string, or a fragment. A non-loopback scope still requires
an explicit confirmation on each request that sends prompt content. Configuration
does not authorize automatic scans, writes, multi-file edits, commits, or pushes.

```yaml
model_scopes:
  analyze:
    api_base_url: "https://api.openai.com/v1"
    api_key: "set-in-ignored-config.yaml"
    model: "analysis-model-id"
    reasoning_effort: "high" # Optional: none, minimal, low, medium, high, xhigh, max.
    temperature: 0.1 # Optional; defaults to 0.7.
    max_tokens: 16000 # Optional; defaults to 8192.
    context_max_tokens: 120000 # Optional; defaults by scope.
  bug:
    api_base_url: "http://localhost:11434/v1"
    api_key: ""
    model: "bug-model-id"
  function:
    api_base_url: "http://localhost:1234/v1"
    api_key: ""
    model: "function-model-id"
```

The default context budgets are 120000 tokens for `analyze`, 32000 for `bug`,
and 4000 for `function`. For HTTP profiles, `temperature` must be from 0 through 2;
`max_tokens` must be 1–65536; `context_max_tokens` must be 1–120000. API keys
are never returned, logged, or embedded in metadata. Mini-Orca does not perform
environment interpolation or use a vault or Keychain.

The ADK runtime supports bounded Chat Completions, typed function calls and text
streaming through the OpenAI-compatible adapter, plus native Gemini through the
pinned Google SDK. Existing proposal workflows publish only complete results.
Native Anthropic Messages, OpenAI Responses and HTTP model enumeration are not
implemented. Provider errors are sanitized; raw provider bodies are never diagnostics.

## Native Google providers

Replace a scope with `provider: gemini` for the Gemini Developer API:

```yaml
analyze:
  provider: gemini
  model: exact-google-model-id
  api_key: set-in-ignored-config.yaml
  max_tokens: 16000
  context_max_tokens: 120000
  reasoning_effort: high
```

The default endpoint is `https://generativelanguage.googleapis.com` using
`v1beta`. Use `provider: vertex` with an explicit `api_key` for Vertex Express,
or use a project-bound access token:

```yaml
analyze:
  provider: vertex
  model: exact-google-model-id
  access_token: set-in-ignored-config.yaml
  vertex_project: your-project-id
  vertex_location: global
```

Vertex uses `v1` at `https://aiplatform.googleapis.com` for Express/global, or
`https://LOCATION-aiplatform.googleapis.com` for a configured region. Tokens
must be refreshed in the local configuration and the daemon restarted when they
expire. Do not combine Express keys with a token or project/location. Native
profiles never authenticate with environment credentials or Application Default
Credentials. They cannot reuse an Antigravity account implicitly.

An optional `api_base_url` selects an explicit gateway root; the adapter appends
the API version and model path. Native endpoints require
HTTPS, except loopback HTTP for local gateways. Credentials are attached only to
that endpoint. Redirects, SDK retries, and SDK continuation requests are blocked;
each model attempt permits one HTTP exchange and retains the application's deadline.
Requests and responses are bounded to 4 MiB. Incomplete, blocked, malformed, or
tool-bearing completion results cannot become proposals.

Native profiles support `temperature`, `top_p`, `top_k`, `presence_penalty`,
`max_tokens`, and the existing context budget. `reasoning_effort` accepts `none`
(zero thinking budget), `minimal`, `low`, `medium`, or `high`; support depends on
the selected model. `min_p`, `repeat_penalty`, `xhigh`, and `max` are rejected.
Policy messages must precede the conversation for native Google requests.

Model metadata includes adapter capabilities for completion, JSON Schema,
typed tools, and streaming. This describes implemented protocols, not account
entitlements or model availability. Attachments, grounding, and live sessions
remain unavailable until their guarded workflows are implemented. Reading
configured metadata is passive; only explicit CLI catalog refresh performs
discovery. Native models are selected by their configured identifier.

The native provider paths are covered with fake HTTP providers, including Vertex
authentication, cancellation and tool-signature round trips. No live Google
account or model has been used for this validation. See the
[Google ADK model documentation](https://adk.dev/agents/models/google-gemini/).

## CLI providers

Install and sign in to [Antigravity CLI](https://www.antigravity.google/docs/cli/install/)
or [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md)
as the user running the daemon. Choose a model supported by that CLI (`agy models`
or `pi --list-models`). The daemon invokes the executable directly, without a
login shell. Make it available on the daemon's `PATH`, or set `cli_path` to an
absolute executable path. It is not a shell command and accepts no extra arguments.

For example, replace the three scope blocks with:

```yaml
model_scopes:
  analyze:
    provider: agy
    model: gemini-3.8-flash-high
    reasoning_effort: high
  bug:
    provider: pi
    model: provider/exact-model-id
    context_max_tokens: 32000
  function:
    provider: pi
    model: provider/exact-model-id
    reasoning_effort: high
    # cli_path: /absolute/path/to/pi
```

CLI and HTTP scopes can be mixed. CLI profiles use the CLI's own credentials
and settings; omit `api_base_url`, `api_key`, `temperature`, `max_tokens`, and
sampling options. Unsupported settings fail startup instead of being ignored.
Context budgets still limit Mini-Orca's supplied context. Pi maps reasoning
`none` to `off`; agy supports `low`, `medium`, `high`, `xhigh`, and `max`.
Use exact model identifiers, not Pi fuzzy patterns or thinking suffixes.
For agy, copy the slug from `agy models`: for example, `gemini-3.8-flash-high`
selects Gemini 3.8 Flash (High). Display names such as `Gemini 3.8 Flash` are
invalid; names containing spaces fail configuration validation with instructions
to use a slug. Model availability depends on the CLI account. Restart the daemon
after changing its configuration.

Analysis and workflow stages can also select models from the live Agy catalog or Pi's available-model
catalog without adding them to a scope in `config.yaml`. The picker reads
`agy models` for exact slugs and display names, including model variants, and
Pi's `get_available_models` RPC catalog, including custom local models from Pi's
`models.json`. For each CLI it uses the first matching configured `cli_path`
(Analyze, Bug, then Function), or `agy`/`pi` on the daemon's `PATH` if no scope
uses that CLI. No prompt or project context is sent during discovery. Agy may
contact its catalog service; Pi discovery uses its offline inventory.

Use **Models → choose a model → Agy** to browse Agy directly. **Refresh models**
reloads both catalogs after account or model changes. Each CLI's discovery
failure is reported separately and leaves configured choices and the other
CLI's catalog available. Local catalog entries still require prompt consent
because the CLI controls its downstream destination. Catalog assignments use
the operation's context/dispatch limits and CLI defaults, with no inherited
reasoning override from a configured scope. Agy variants retain their exact slug.

For Gemini through an Antigravity account, use Agy directly. Pi removed built-in
Google Antigravity and Gemini CLI support in
[0.71.0](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/CHANGELOG.md#0710---2026-04-30).
Pi still supports the Google Gemini API with its own authentication, such as
`GEMINI_API_KEY`; signing in to Agy does not populate that Pi provider. See
[Pi providers](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/providers.md).
Mini-Orca disables Pi extensions for both discovery and inference, so providers
added only by a Pi extension are not included.

Each request starts a separate process in a private temporary directory, using
Mini-Orca's supplied source context. Pi runs in JSON print mode with tools,
extensions, MCP, skills, context-file discovery and session saving disabled. Its
temporary project settings disable automatic retries and compaction. Agy uses
a temporary `mini-orca` agent with file, command, subagent and MCP tools disabled,
slash expansion disabled, and JSON stdin/stdout. Agy also keeps `finish` disabled:
native schema completion depends on that tool, which user hooks can reject.
These adapters require CLI versions supporting those options and the documented
event formats.
See the [Pi CLI reference](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/cli.md)
and [agy headless reference](https://www.antigravity.google/docs/cli/headless/).

Only a successful final assistant response is accepted. Both CLI providers
receive the JSON Schema in their system prompt and must return a JSON answer.
Mini-Orca validates the final response against that schema locally, with no
unconstrained fallback. Failed, truncated, tool-bearing or incomplete responses
fail the request.
CLI stderr and raw protocol errors are not returned or logged. Output is bounded
to 4 MiB and stderr to 64 KiB. HTTP and CLI transports honor the workflow's
bounded deadline, including the longer feature discovery allowance. Direct
transport callers without a deadline receive a five-minute fallback. Owned process
groups are stopped on completion or cancellation, and temporary prompt files are removed. CLI providers currently
require Unix process-group support; HTTP providers retain their existing platforms.

Both CLI providers always require remote-provider confirmation, even if Pi is
configured with a local model: Mini-Orca cannot independently verify the CLI's
downstream destination. Metadata shows `cli://agy` or `cli://pi`, with
`temperature` and `max_tokens` set to `null` because those controls are unavailable.
Existing Review, explicit Apply, and Undo still own source changes. Mini-Orca
does not configure the CLI's credential storage or upstream retention; agy may
retain its own conversation records under its normal settings. Application
retry limits count CLI invocations; upstream retries internal to a CLI or its
service are outside Mini-Orca's transport accounting.

## Agent roles and run limits

`agent_runtime.roles` supplies Creation, Testing and Review profiles when an API
request omits a model choice. Defaults are `bug`, `function`, and `analyze`.
Explicit catalog selections take precedence. Resolved roles are frozen at
admission, saved with the workflow, and require the same profile-specific remote
consent as explicit choices. The runtime never silently switches providers.

```yaml
agent_runtime:
  roles:
    creation: bug
    testing: function
    review: analyze
  max_model_calls: 1024
  max_tool_calls: 4096
  max_input_tokens: 134217728
  max_output_tokens: 67108864
  max_elapsed_seconds: 86400
  max_concurrent_calls: 4
  max_repairs: 2
```

Model calls, tokens, concurrency and elapsed time are shared by child stages and
provider retries within an admitted workflow or analysis execution window.
Standalone operations receive their own budget. Existing scope deadlines and
analysis attempt allowances still apply; the first exhausted limit stops work.
Limits are positive; omitted or zero values use the displayed defaults, except
`max_repairs`, where an explicit zero disables repairs. Repairs remain opt-in and
cannot exceed two. Maximums are 4096 model calls, 16384 tool calls, 2^40 tokens per
token limit, seven days, and 32 concurrent model calls. Completion agents admit
no tools; their ADK policy hook rejects tool dispatch.

Before dispatch, the runtime reserves input capacity using encoded request bytes
plus framing allowance, and output capacity using the configured `max_tokens`
(4 MiB for an opaque CLI). Reported usage settles reservations; Gemini thinking
tokens count as output. Missing usage and failed/interrupted calls keep the full
reservation and are marked unknown. A provider usage overrun stops the run and
prevents final-result publication. These are local execution limits, not a billing
guarantee: upstream internal retries and unreported consumption remain unknown.
No reservation or usage record grants source-write or execution authority.

Project-specific role, budget and skill preferences are available through the
guarded [agent settings API](docs/api-contract.md#agent-settings-and-skill-validation).
They are saved privately in the canonical checkout's agent journal and apply to
future admissions. Budgets can be reduced up to the configured ceilings; role
choices reference the existing scopes, and skill controls can only disable or
reenable configured directories. Active runs retain their captured configuration.
Explicit request model choices and TUI model presets remain authoritative.
Changing operator configuration makes saved overrides stale and requires an
explicit reset before new work; reset follows current configuration defaults.

### Scoped project skills

`agent_runtime.skills` is an explicit list of project-relative skill directories;
the default is an empty list. For example:

```yaml
agent_runtime:
  skills:
    - project-skills/review-code
```

Each directory must contain an indexed `SKILL.md` with valid ADK YAML frontmatter
and nonempty instructions. Its `name` must match the directory name. Configure
at most eight distinct, non-overlapping directories. Reindex after adding or
changing a skill. Excluded files and escaping symlinks cannot become skill context.
Loading configuration and browsing project state do not run skill tools.

Tool-capable Function profiles use the native ADK `list_skills`, `load_skill`, and
`load_skill_resource` tools during plan investigation and declaration explanation.
Only names and descriptions enter the initial prompt. Instructions and indexed
resources under `references/`, `assets/`, and `scripts/` load on demand. Completion-only
profiles retain their existing behavior and do not advertise these tools.

Each manifest/resource must be 1–8192 bytes of UTF-8 text; each skill can have up
to 16 resources, and the combined inventory is limited to 128 KiB. Descriptions
are at most 512 bytes; serialized metadata is at most 2048 bytes. The existing
16 KiB tool result, 128 KiB aggregate retrieval, model context, and shared run
budgets also apply. Oversized encoded results fail explicitly instead of truncating
instructions silently. Frontmatter `allowed-tools`, when supplied, must name only
the tools already admitted to that investigation; unsupported tools fail before
a model call.

Skill text is advisory and cannot override the task, repository instructions, or
grants. The Go SkillToolset has no script execution tool. Loading a script displays
its text; it cannot install packages, run commands, or write source. Explicitly
trusted fixed checks continue through the checked workflow's sandbox.

Manifests and loaded resources are checked against their captured hashes before
model dispatch and publication. Context manifests identify contributing skill
files, and private run tool records retain bounded file/hash provenance. Native
session events also record loaded-file identities. These source-bearing records
require explicit private-output disclosure and do not enter public progress events.
Conformance tests exercise native ADK tools with fake compatible and Google providers;
they do not establish live-provider performance.

### Optional context compaction and caching

All three optimizations are disabled by default. Configure them under
`agent_runtime.context`:

```yaml
agent_runtime:
  context:
    compaction: false
    token_threshold: 8000
    retain_events: 4
    summary_max_tokens: 1024
    local_cache: false
    cache_ttl_seconds: 600
    provider_cache: false
    provider_ttl_seconds: 0
```

Compaction uses ADK Go's native summarizer and token-threshold tail retention
inside investigations. In proposal conversations it summarizes older assistant
discussion while retaining every original user message and the recent tail.
Approved plans, acceptance criteria, source, instruction text and proposal
identity stay outside lossy summaries. The stored conversation remains intact.
The reconstructed request must still fit the selected context budget; oversized
mandatory material produces an error before summarization. Summaries are advisory,
and model suggestions never become verified findings through compaction.

`token_threshold` accepts 512–128000 estimated tokens; `retain_events` accepts
2–64; `summary_max_tokens` accepts 128–8192 and must be below the threshold.
Summary calls use the same admitted profile, source authority and shared budget,
including the investigation's eight-model-turn limit. They have a 30-second
timeout within the owning operation's deadline. A compaction or persistence
failure stops dispatch. No retry is hidden inside the summarizer.

`local_cache` reuses deterministic captured source composition for proposals.
Its TTL accepts 60–86400 seconds. Keys include project/revision, workspace and
source hashes, instructions, policy, schema and configuration. Current source
authority is checked again on every hit and model dispatch.

Private cache data and paid-operation receipts live in
`.mini-orca/agents/<workspace-hash>/context-cache.json` with mode `0600`.
The catalog permits 1024 records, 1 MiB per value and 32 MiB total. Expired local
composition is pruned during writes. Completed summaries remain reusable; an
uncertain summary or remote-cache creation remains pending across restarts and
cannot silently incur another call. A full or corrupt catalog fails explicitly.
To intentionally discard receipts, stop the affected workflow and remove that
checkout's cache file after reviewing the prior run; subsequent requests may pay
for summarization or cache creation again. Removing local metadata does not
delete provider-held content.

`provider_cache` enables a Mini-Orca adapter over the GenAI Go cache API; ADK Go
does not provide `ContextCacheConfig`. It requires an explicit
`provider_ttl_seconds` of 60–3600 and selected Gemini API or Vertex profiles in the
Gemini 2.5/3 Flash/Pro families. Unsupported profiles fail locally; model/account
availability remains subject to the provider. Gemini 4 and Interactions explicit
caching are unsupported. Existing profile-specific remote consent is still required.

Only a complete initial text turn of at least 16 KiB is eligible, once a request
also contains a subsequent turn and has captured source authority. Small or
single-turn requests proceed normally. Cached prefixes include their system/tool
configuration; resource identity, selected model, destination, expiry and source
authority are checked before reuse. Cache creation consumes an additional shared
model-call reservation; cached input still counts toward local context/token
limits. Failed or uncertain creation is not retried automatically. This does not
guarantee upstream cost savings or change a provider's implicit caching policy.
Explicitly cached source and memory may remain remotely retained until the chosen
expiry even after local edits or forgetting; they cannot be reused under stale
local authority. Adapter behavior is covered by offline HTTP fixtures, not a live
provider billing benchmark.

## Other settings

```yaml
project_path: "."

retry:
  max_retries: 3
  backoff_base: 1000 # milliseconds
  backoff_max: 30000 # milliseconds

timeouts:
  import_seconds: 300
  analysis_seconds: 300
  generation_seconds: 300
  workflow_model_seconds: 900
  focused_check_seconds: 60

logging:
  level: "info"
  format: "json"
  filename: ""
  sensitive_keys: ["api_key", "password"]
```

The operation timeouts bound the request contexts; retries only cover explicit
prompt requests. Restore, navigation, validation, checks, Apply, and Undo do
not make a model call.

`workflow_model_seconds` gives each Creation, Testing and Review model call at
least 15 minutes by default, including its provider retries. Longer generation
or selected-scope timeouts still take precedence. Existing configurations that
omit this field receive the new default; standalone generation retains its own
`generation_seconds` limit. Testing generates complete test files before running
checks, so its model budget is separate from each command's
`focused_check_seconds` limit.

The overall workflow deadline includes the remaining model budgets, check and
preparation allowances, and up to two repairs when explicitly enabled. It is at
least 30 minutes and grows with configured limits, subject to the shared
`agent_runtime.max_elapsed_seconds` cap. To adjust a slow workflow,
set `workflow_model_seconds`, restart the daemon, and use `:retry test` on the
saved proposal to reuse Creation. A retry still validates the current source,
plan, consent and execution trust.

## Configuration changes

Only the current YAML schema is supported. For retired configurations, create a
fresh local file from `config.example.yaml`; unknown keys fail startup instead
of invoking compatibility fallbacks. `MINI_ORCA_CONFIG` selects the file path
(default `config.yaml`). Restart the daemon after changes.

## Local metadata and manual cleanup

Configuration stays separate from project metadata. Mini-Orca stores its local
index, analysis, findings, shared proposals and grouped recovery data
under the imported project's `.mini-orca/` directory; see
[README.md](README.md#project-intelligence) for the current paths. These files are
application metadata, separate from configuration.
Do not delete Apply receipts or Undo backups as a routine cache reset.

ADK session state, input history, final events, invocation status and shared budget
checkpoints live in `.mini-orca/agents/<checkout-hash>/journal.json`. The hash binds
the canonical checkout path; worktrees have separate records. Journals use atomic
replacement, Unix file/directory modes 0600/0700, and a process lock. Windows
access follows the checkout's filesystem ACLs. Input and response
history can contain project source, so keep this directory out of version control
and public diagnostics. No journal is uploaded automatically.

Budget reservations are saved before provider dispatch; usage and final events
must be saved before a successful result is published. A persistence failure stops
the shared run. Session app/user namespaces stay inside that checkout, temporary
state is not persisted, and partial streaming previews are not durable events.
Each journal is bounded to 64 MiB, 4096 sessions, 8192 invocations and 2048 budget
records. Corrupt, oversized or unknown-version data is preserved and rejected.
Opening a project reads records passively; it never resumes unfinished invocations
or grants source-write, remote-provider, or execution consent.

Versioned completion results and proposal/check evidence use the adjacent private
`artifacts.json` catalog. Each version records its SHA-256 hash, byte size, media
type, ownership and provenance. Content is limited to 4 MiB per version, 128
retained versions per artifact, 4096 artifact identities and 64 MiB per catalog.
Identical proposal/check snapshots reuse a version. A failed evidence write blocks
publication; saved artifacts never grant permission to Apply or Undo.

Artifact reads/export return verified bytes without accepting a project filesystem
destination. Deletion and retention are explicit, preserve version counters, and
reject evidence retained by an unfinished run or change recovery. Generic ADK
artifact operations cannot delete recovery-pinned proposal/check snapshots. There
is no automatic cleanup, source write or external upload.

Explicit declaration explanations use an ADK investigation with the selected
Function profile when it supports tools. Search, snippet, symbol, import and
heuristic impact tools admit only the selected declaration's source span and
indexed metadata. Other paths or line ranges return `scope_required`; they do
not widen consent. The current-check tool returns source-free, proposal-bound
check states; absent or stale evidence never implies a pass. It does not execute
checks. Completion-only CLI profiles continue using the supplied bounded context.

Each investigation retains its tool contracts, scope identity and tool events in
the private journal. Limits survive provider retries: eight model turns, 24 tool
calls, 16 KiB per tool payload and 128 KiB of aggregate tool output. Retrieval
returns at most 20 evidence items and 8 KiB of text, with file/hash/span provenance.
Source reads are capped at 1 MiB. Model context is capped at 128 KiB and the
Function profile's context token estimate (four bytes per token), whichever is
smaller. Source, policy, instructions and project identity are rechecked at
dispatch and publication; a changed scope stops the investigation.

`integrations.mcp` configures at most eight named MCP servers. Each entry selects
`streamable_http` with an HTTPS `endpoint` (literal loopback HTTP is permitted),
or `stdio` with an absolute installed `command` and fixed `args`. HTTP bearer
authentication uses `bearer_token_env`; stdio `environment` maps child variable
names to explicit credential variable names. Missing credentials are unavailable,
never discovered implicitly. Arguments must not contain secrets. Configure one
to four exact tool names with the operator-owned `effect: read_external`.
Mutations, filesystem tools and arbitrary command execution are not admitted;
server annotations cannot change this classification.

Configuration and list views are passive. Explicit discovery approves the exact
configured endpoint or process and returns an in-memory inventory valid for 15
minutes. Each plan/declaration investigation separately supplies the current grant,
schema fingerprint and `confirm_source_sharing: true`. This consent covers that
investigation's admitted prompt, source, skill and memory context reaching the
selected server through tool arguments. Provider confirmation is independent.
At most two integrations can be selected per investigation, within the combined
16-tool limit for repository, skill and integration tools. Revoke, reindex,
project replacement and shutdown cancel in-flight connections and discard grants;
grants are not restored on daemon restart. See the [API contract](docs/api-contract.md)
for discovery and revocation.

Each discovery/call owns a short-lived native ADK MCP connection, closes it before
returning, and rechecks the selected inventory before dispatch. Automatic reconnect,
tool retry, OAuth prompts, roots, sampling and elicitation are disabled. Credential
changes invalidate grants. Discovery permits four pages, 64 total tools and 64 KiB;
protocol frames and HTTP responses are capped at 64 KiB. Tool arguments/results use
the investigation's 16 KiB bounds and shared budgets. Schemas must be inline JSON
objects without references; results support text or structured objects, not media
or resource downloads. Incoming schema/result integers must have absolute values
below 2^53 because the MCP SDK decodes JSON numbers as floating point; represent
larger identifiers as strings. Validated outbound argument JSON retains its
original numeric precision. `timeout_seconds` defaults to 30 and accepts 1–60; at most
16 integration operations may be active in a service.

Stdio runs only the configured installed executable, with no shell interpolation
or automatic installation. Its private temporary working directory is also HOME
and TMPDIR; PATH is `/usr/bin:/bin`, and only configured credential references are
added. Cleanup terminates the owned process group, joins the process and removes
the directory. This is process ownership, not OS filesystem confinement: approve
only trusted installed MCP servers and correctly classify their allowed tools.
External answers remain advisory and cannot supply verified repository citations,
checks, source writes, Apply or Undo authority.

`integrations.openapi` configures at most eight curated HTTP APIs, with IDs unique
across MCP, API and A2A entries. Each supplies an indexed project-relative `spec_path`,
a fixed HTTPS `endpoint` (literal loopback HTTP is permitted), an optional
`bearer_token_env`, `timeout_seconds` (1–60; default 30), and one to four exact
`operations: [{name: operationId, effect: read_external}]`. GET operations require
`read_external`; POST, PUT, PATCH and DELETE require `mutate_external`. Configuration
owns the destination and credentials; document `servers` and `security` fields
do not select endpoints or acquire authentication.

API discovery reads the local specification without sending HTTP. It creates a
15-minute grant using the same explicit discovery/source-sharing controls as MCP.
The supported subset is OpenAPI 3.0.0–3.0.4 JSON, at most 64 KiB, with inline
schemas, scalar path/query parameters, optional or required JSON object bodies,
and one to eight explicit successful JSON object responses or empty 204 responses.
Path placeholders occupy whole segments. Parameters use `simple` path or `form`
query serialization; headers, cookies, callbacks, references, polymorphic schemas,
nullable values, formats, multipart bodies and arbitrary schema extensions are
rejected for selected operations. Inline schemas support type, object properties,
required fields, boolean additional-properties policy, array items, enums, patterns
and numeric/string/collection bounds, with depth 12 and 64 properties per object.
Schema numeric bounds must be below 2^53 in magnitude; argument JSON preserves
integer precision. Arguments and results use the shared 16 KiB tool limit.

Mutating tools only save a private candidate containing the exact method, URL and
body. Explicit approval of its current version/hash claims a durable pending
receipt before one HTTP attempt. Reads and writes disable redirects, proxies,
cookies, retries and connection reuse. An uncertain write is recorded as `unknown`;
pending/unknown receipts block more writes through that operation. Reconciliation
records the user's externally checked outcome without replaying the request.
Restart, expiry, changed sources or revocation invalidate candidate authority;
existing receipts remain readable. Review is bounded to 128 live candidates and
512 retained receipts per checkout. Receipt and report data is private and may
contain admitted source text; credentials are excluded.

`integrations.google_search: true` enables a separate explicit research action
using the Function profile. It requires native Gemini or Vertex and a supported
text model: `gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-2.5-flash-lite`,
`gemini-3.8-flash`, `gemini-3.6-flash`, `gemini-3.5-flash-lite`,
`gemini-3.1-pro-preview` or `gemini-3-flash-preview`. Unknown models remain
unavailable until their capability is verified. Each request separately confirms
search, source sharing and any remote-provider destination. A question is limited
to 2048 UTF-8 bytes, with up to eight explicit path/hash selections and a combined
64 KiB context within the configured Function token estimate. Search never joins
planning JSON or function tools and does not use provider caching or retries.
The private report displays validated inline/aggregate citations and the original
Google Search suggestions in a sandboxed, script-free frame. Search metadata is
advisory; provider-internal search usage is unknown. One admitted search invocation
consumes one model-call and one tool-call allowance, independently of unknown
provider-internal searches. Configuration, report listing and report reads do not
dispatch provider requests. See the [API contract](docs/api-contract.md).

`integrations.a2a` configures up to eight remote agents. Each entry supplies an
`id`, exact same-origin `card_url` and `endpoint`, optional `bearer_token_env`,
one to four allowlisted `skills`, and `timeout_seconds` (1–60; default 30).
URLs require HTTPS or literal loopback HTTP, without query strings, redirects or
escaped paths. Explicit discovery fetches a bounded card and grants its exact
fingerprint for 15 minutes. The supported interface is A2A 1.0 JSON-RPC, including
SSE when advertised. Cards must advertise exactly the configured interface and
text/plain for selected skills; required extensions are rejected. Icons,
documentation, extended cards and artifact URLs are never fetched. Credentials
come only from the configured environment reference. Replacing or revoking a
grant cancels and joins admitted operations; restart retains no connection grant.

Delegation requires explicit analysis/review intent, a selected advertised skill,
a question up to 2048 UTF-8 bytes, source-sharing confirmation and up to eight
current path/hash selections. The combined prompt is bounded by 64 KiB and the
Function context estimate. Ambient memory, skills and history are excluded.
Each card/RPC request consumes a tool-call allowance; the total operation uses
the configured timeout. Remote model/search call counts remain unknown. Optional
valid ADK token metadata is labelled `remote_reported`, never measured locally.

The private journal saves a message ID and exclusive pending lease before each
send. Initial sends and input replies are attempted once. A retained task can be
explicitly refreshed, subscribed to again, or canceled; these never resend a
message. Input requires unchanged source/card authority and fresh reply consent.
Cancellation closes local transport and attempts bounded remote cleanup.
Unknown outcomes and expired pending receipts require explicit recovery; an
operator can record a user-reported reconciliation without network activity.
One unfinished delegation per endpoint blocks additional sends, and a checkout
retains at most 128 receipts. Responses are advisory plain text, at most four
artifacts and 64 KiB combined. Streams are bounded to 512 KiB and 128 events.
Partial artifacts, input/auth requests, failure, cancellation and uncertainty
retain separate states. Remote output cannot grant local permissions, execute
project code or change source. See the [API contract](docs/api-contract.md).

`integrations.a2a_server` optionally exposes Mini-Orca's native ADK A2A service.
It is disabled by default. `bind` defaults to `127.0.0.1:9091` and must use a
literal loopback IP; `timeout_seconds` is 1–120 (default 60), and `max_concurrent`
is 1–4 (default 2). Configure one to eight `principals`, each with a distinct
`id`, `bearer_token_env`, fixed `project_id` and `capabilities: [analysis]`,
`[proposal]` or `[analysis, proposal]`. Tokens must be 32–4096 printable ASCII bytes
without whitespace; keep them in the referenced environment variables. Reused token
values across principals are rejected at authentication. No ambient credentials
or project permissions are inferred from this configuration.

The separate listener publishes only a generic card at
`/.well-known/agent-card.json` and authenticated JSON-RPC at `/a2a`. Its card
contains no project IDs, principal configuration, history or source. It reuses
the daemon's Host/Origin checks and does not expose the development UI or REST
API. Before any RPC can access a project, explicitly grant its principal access
through the local `agent-access` controls described in the
[API contract](docs/api-contract.md). A grant requires source-sharing and the
applicable provider consent, lasts 15 minutes, and binds the current revision,
credential and model configuration. Revoke, reindex, replacement and shutdown
invalidate grants and cancel owned work. Grants belong to the service instance
owning the listener; a separate worktree service reports it unavailable.
Opening configuration or a card is passive. TUI access controls are part of the
planned Settings integration.

Requests carry explicit project/revision identity, a question, one to eight
current source hashes and applicable per-request provider confirmation. Analysis
uses the normal investigation workflow; proposal preparation returns a private
draft identity for local checks and review. Neither operation applies source,
runs project checks, grants execution trust, or merges work. Payloads are bounded
to 16 KiB per HTTP request, 8 KiB for the request's text part and 16 KiB of result
text. At most 16 HTTP requests and the configured number of executions are active
at once. A process retains at most 128 attempted message IDs/tasks; duplicate
messages are never dispatched again. A2A task/session history is private and
process-local. Restart removes it and its grants; normal local run/proposal
journals remain durable. After an uncertain outcome, inspect those journals
before issuing any new request. Request disconnection, explicit task cancellation,
timeout and daemon shutdown terminate owned execution.

Execution trust decisions are private user settings, outside project metadata.
They are stored in `mini-orca/execution-trust/` under the OS user configuration
directory (on macOS, `~/Library/Application Support/`). `MINI_ORCA_TRUST_DIR`
can select another absolute, user-controlled directory. Each decision is bound
to the canonical project path and command scope; copying `.mini-orca/` to another
checkout does not transfer consent. Use `:trust` to grant or revoke trust.
