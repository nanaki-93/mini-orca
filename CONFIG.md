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

Execution trust decisions are private user settings, outside project metadata.
They are stored in `mini-orca/execution-trust/` under the OS user configuration
directory (on macOS, `~/Library/Application Support/`). `MINI_ORCA_TRUST_DIR`
can select another absolute, user-controlled directory. Each decision is bound
to the canonical project path and command scope; copying `.mini-orca/` to another
checkout does not transfer consent. Use `:trust` to grant or revoke trust.
