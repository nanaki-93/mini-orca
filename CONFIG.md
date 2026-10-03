# Configuration Guide

Mini-Orca reads one YAML file, normally ignored `config.yaml`. Start with
`config.example.yaml`; do not commit provider credentials. Configuration is
validated once at daemon startup, and unknown or retired keys stop startup with
the full field path in the error. JSON configuration files are not supported.

Every prompt-bearing operation has one fixed model scope:

| Scope | Used for |
| --- | --- |
| `analyze` | Project import, architectural summaries and source-based Performance review |
| `bug` | Selected-file analysis and Analyze-all suggestions |
| `function` | Declaration proposals and explicit repairs |

All three scopes are required. Each selects a `provider` (`openai`, `agy`, or
`pi`) and a `model`. Omitted `provider` retains the OpenAI-compatible HTTP
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
environment interpolation, use a vault or Keychain, call native provider SDKs,
or provide a settings UI.

The HTTP provider boundary is one non-streaming OpenAI-compatible Chat Completions
request. Native Anthropic Messages, Gemini `generateContent`, OpenAI Responses,
tool calls, model enumeration, and vendor SDKs are not supported. Provider
errors are reduced to a status code so provider bodies cannot leak a key.

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
    model: replace-with-agy-model-slug
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

Each request starts a separate process in a private temporary directory, using
Mini-Orca's supplied source context. Pi runs in JSON print mode with tools,
extensions, skills, context-file discovery and session saving disabled. Its
temporary project settings disable automatic retries and compaction. Agy uses
a temporary `mini-orca` agent with an empty tool allowlist and command execution
off, slash expansion disabled, and JSON stdin/stdout. These adapters require
CLI versions supporting those options and the documented event formats.
See the [Pi CLI reference](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/cli.md)
and [agy headless reference](https://www.antigravity.google/docs/cli/headless/).

Only a successful final assistant response is accepted. Agy receives native
JSON Schema constraints; Pi receives the schema in its system prompt. Mini-Orca
validates both outputs against the schema locally, with no unconstrained
fallback. Failed, truncated, tool-bearing or incomplete responses fail the request.
CLI stderr and raw protocol errors are not returned or logged. Output is bounded
to 4 MiB, stderr to 64 KiB, and the process uses the shorter of the operation
deadline and five minutes. Owned process groups are stopped on completion or
cancellation, and temporary prompt files are removed. CLI providers currently
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

## Configuration changes

Only the current YAML schema is supported. For retired configurations, create a
fresh local file from `config.example.yaml`; unknown keys fail startup instead
of invoking compatibility fallbacks. `MINI_ORCA_CONFIG` selects the file path
(default `config.yaml`). Restart the daemon after changes.

## Local metadata and manual cleanup

Configuration stays separate from project metadata. Mini-Orca stores its local
index, analysis, findings, Analyze-all state, Apply audit, and Undo backup data
under the imported project's `.mini-orca/` directory; see
[README.md](README.md#project-intelligence) for the current paths. These files are
application metadata, separate from configuration.
Do not delete Apply receipts or Undo backups as a routine cache reset.
