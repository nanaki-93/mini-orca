# Configuration Guide

Mini-Orca is configured using a YAML file, typically named `config.yaml`. This document provides a detailed reference for all available configuration options.

## LLM Configuration (`llm`)

Defines the LLM provider settings. This is a flat configuration (no nested providers).

- `base_url` (string, required): The base URL of the provider's API (e.g., `http://localhost:1234` for LM Studio).
- `api_key` (string, optional): The API key for the provider.
- `model` (string, optional): The default model to use.
- `temperature` (float, optional): Sampling temperature (0.0 to 1.0). Default: `0.7`.
- `max_tokens` (int, optional): Maximum number of tokens to generate. Default: `8192`.

## Agents Configuration (`agents`)

Configure the active `coder` profile. Tester and reviewer settings are retained
for optional focused checks but are not an automatic generation pipeline.

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
- `generation_seconds` (int): Focused candidate generation deadline. Default: `300`.
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
