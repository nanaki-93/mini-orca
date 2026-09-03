# Configuration Guide

Mini-Orca reads one YAML file, normally ignored `config.yaml`. Start with
`config.example.yaml`; do not commit provider credentials. Configuration is
validated once at daemon startup, and unknown or retired keys stop startup with
the full field path in the error. JSON configuration files are not supported.

Every prompt-bearing operation has one fixed model scope:

| Scope | Used for |
| --- | --- |
| `analyze` | Project import and architectural summaries |
| `bug` | Selected-file analysis and Analyze-all suggestions |
| `function` | Declaration proposals and explicit repairs |

All three scopes are required. Each has an OpenAI Chat Completions-compatible
`api_base_url` and a `model`; an empty `api_key` is valid for a loopback local
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
and 4000 for `function`. `temperature` must be from 0 through 2;
`max_tokens` must be 1–65536; `context_max_tokens` must be 1–120000. API keys
are never returned, logged, or embedded in metadata. Mini-Orca does not perform
environment interpolation, use a vault or Keychain, call native provider SDKs,
or provide a settings UI.

The provider boundary is one non-streaming OpenAI-compatible Chat Completions
request. Native Anthropic Messages, Gemini `generateContent`, OpenAI Responses,
tool calls, model enumeration, and vendor SDKs are not supported. Provider
errors are reduced to a status code so provider bodies cannot leak a key.

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

## Migration from legacy configuration

Retired keys are errors, not fallbacks. Migrate each legacy key before restart:

| Removed key | Replacement |
| --- | --- |
| `llm.base_url` | Copy to each `model_scopes.<scope>.api_base_url` (include the compatible API prefix such as `/v1`) |
| `llm.api_key` | Copy to each `model_scopes.<scope>.api_key` that uses the key |
| `llm.model` | Set each `model_scopes.<scope>.model` explicitly |
| `llm.temperature` | Set each `model_scopes.<scope>.temperature`, or omit for the default |
| `llm.max_tokens` | Set each `model_scopes.<scope>.max_tokens`, or omit for the default |
| `agents.coder.model` | `model_scopes.function.model` |
| `agents.coder.timeout_seconds` | `timeouts.generation_seconds` |
| `agents.coder.skills`, `agents.tester.*`, `agents.reviewer.*` | Removed; there are no role profiles or role skills |
| `skills.knowledge`, `skills.tools` | Removed |
| JSON loading/saving | Removed; use YAML only |

`MINI_ORCA_CONFIG` may select the YAML file path (default `config.yaml`); it
does not interpolate values inside that file. Restart the daemon after editing
configuration.
