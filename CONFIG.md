# Configuration Guide

Mini-Orca is configured using a YAML file, typically named `config.yaml`. This document provides a detailed reference for all available configuration options.

Keep `config.yaml` on the local machine; it is intentionally ignored by Git.
Start from `config.example.yaml` and do not add provider credentials to tracked
files. The daemon binds to loopback by default. Every configured non-loopback
model scope requires explicit confirmation for its own prompt request. That
confirmation is part of the request, not a configuration switch that silently
enables remote delivery.

Prompt-bearing requests are project import/analysis, one-file semantic
analysis, Analyze-all, and file-scoped chat messages. Deterministic reindexing,
verified Go scans, validation, focused checks, Apply, and Undo do not send a
prompt. Confirming a remote provider does not authorize automatic scans,
automatic writes, multi-file edits, commits, or pushes.

## LLM Configuration (`llm`)

Defines the LLM provider settings. This is a flat configuration (no nested providers).

- `base_url` (string, required): The base URL of the provider's API (e.g., `http://localhost:1234` for LM Studio).
- `api_key` (string, optional): The API key for the provider.
- `model` (string, optional): The default model to use.
- `temperature` (float, optional): Sampling temperature (0.0 to 1.0). Default: `0.7`.
- `max_tokens` (int, optional): Maximum number of tokens to generate. Default: `8192`.

## Scoped model profiles (`model_scopes`)

`model_scopes` optionally resolves three fixed profiles at daemon startup:
`analyze` for import, `bug` for one-file analysis and Analyze-all, and
`function` for declaration proposals and explicit check-driven repairs. Each
explicit profile needs `api_base_url` and `model`; an empty `api_key` is valid
for local servers. `temperature`, `max_tokens`, and `context_max_tokens` are
optional. The default context budgets are 120000, 32000, and 4000 tokens in
scope order.

An omitted `analyze` or `bug` scope inherits the complete legacy `llm` profile.
An omitted `function` scope inherits `llm`, except that `agents.coder.model`
continues to override its model. A partial scope is invalid rather than silently
mixing endpoints. Restart the daemon after changing configuration.

All profiles use one small OpenAI Chat Completions-compatible contract. Use
placeholder IDs and a local personal config, for example:

```yaml
model_scopes:
  analyze: # OpenAI-compatible example
    api_base_url: "https://api.openai.com/v1"
    api_key: "replace-in-local-config"
    model: "replace-with-analysis-model-id"
  bug: # Claude-compatible or Gemini-compatible OpenAI endpoint
    api_base_url: "https://api.anthropic.com/v1" # or https://generativelanguage.googleapis.com/v1beta/openai
    api_key: "replace-in-local-config"
    model: "replace-with-bug-model-id"
  function: # Ollama, LM Studio, or a custom compatible gateway
    api_base_url: "http://localhost:11434/v1" # LM Studio: http://localhost:1234/v1
    api_key: ""
    model: "replace-with-local-function-model-id"
```

Native Anthropic Messages, Gemini `generateContent`, OpenAI Responses,
streaming, tool calls, vendor SDKs, and provider-specific reasoning controls
are not supported by this compatibility layer. A provider that lacks compatible
Chat Completions or `/models` support can still be used for generation; model
listing is informational only.

API keys are read only from ignored local configuration. Mini-Orca never logs
or returns them, but it does not provide a vault, encryption, Keychain,
rotation, account management, or credential UI.

## Agents Configuration (`agents`)

Configure the active `coder` profile. Tester and reviewer settings are retained
for optional focused checks but are not an automatic generation pipeline. The
workflow remains one file-scoped chat session and one editable declaration draft
at a time; profiles cannot authorize automatic writes or multi-file changes.

### Agent Configuration

Each agent supports the following fields:

- `skills` (list of strings, optional): A list of skill names assigned to the agent. Skills must be defined in the `skills` section.
- `model` (string, optional): Override the default model for this specific agent.
- `timeout_seconds` (int, optional): Maximum duration for requests made with this profile. Default: `300`.

Example:
```yaml
agents:
  coder:
    skills: ["go", "file-system"]
    model: "qwen3-coder-30b"
    timeout_seconds: 300
```

## Skills Configuration (`skills`)

Define the knowledge base and tools available to agents.

- `knowledge` (map): Prompt templates that provide agents with specialized knowledge or instructions.
- `tools` (map): Prompt templates that describe how to use specific tools (e.g., shell, formatter).

Example:
```yaml
skills:
  knowledge:
    go-best-practices: "Follow effective Go programming practices..."
    testing-strategies: "Use table-driven tests and property-based testing..."
  tools:
    shell: "Execute shell commands safely..."
    formatter: "Format code using gofmt or equivalent..."
```

## Retry Configuration (`retry`)

Control how the system handles failed LLM requests or tool executions.

- `max_retries` (int): Maximum number of attempts for a failed operation. Default: `3`.
- `backoff_base` (int): The base delay for exponential backoff in milliseconds. Default: `1000`.
- `backoff_max` (int): The maximum delay between retries in milliseconds. Default: `30000`.

## Operation Timeouts (`timeouts`)

Each desktop request carries its cancellation context through daemon work. These
values cap the daemon operation even when the client remains connected.

- `import_seconds` (int): Project import and architectural analysis deadline. Default: `300`.
- `analysis_seconds` (int): One-file semantic analysis deadline. Default: `300`.
- `generation_seconds` (int): File-scoped declaration proposal deadline. Default: `300`.
- `focused_check_seconds` (int): Isolated formatter/parser/check deadline. Default: `60`.

## Logging Configuration (`logging`)

Mini-Orca uses structured logging via Go's `slog` package.

- `level` (string): Log level (`debug`, `info`, `warn`, `error`). Default: `info`.
- `format` (string): Output format (`json` or `text`). Default: `json`.
- `filename` (string, optional): Path to a file where logs should be written. If empty, logs go to stdout.
- `sensitive_keys` (list of strings, optional): A list of keys whose values should be redacted from the logs (e.g., `password`, `api_key`).

## Full Example

```yaml
llm:
  base_url: "http://localhost:1234"
  api_key: ""
  model: "qwen3-coder-30b"
  temperature: 0.7
  max_tokens: 8192

agents:
  coder:
    skills: ["go", "file-system"]
    model: "qwen3-coder-30b"
    timeout_seconds: 300

skills:
  knowledge:
    go-best-practices: "Follow effective Go programming practices..."
  tools:
    shell: "Execute shell commands safely..."
    formatter: "Format code using gofmt or equivalent..."

retry:
  max_retries: 3
  backoff_base: 1000
  backoff_max: 30000

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

## Environment Variables

- `MINI_ORCA_CONFIG`: Path to the configuration file (defaults to `config.yaml`).
