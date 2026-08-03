# Configuration Guide

Mini-Orca is configured using a YAML file, typically named `config.yaml`. This document provides a detailed reference for all available configuration options.

## Models Configuration (`models`)

This section defines the LLM providers and how they are used across different execution phases.

- `active_provider` (string, required): The name of the provider to use by default. Must match a key in the `providers` map.
- `providers` (map): Definitions for LLM providers.
    - `base_url` (string, required): The base URL of the provider's API (e.g., `http://localhost:1234` for LM Studio).
    - `api_key` (string, optional): The API key for the provider.
- `phases` (map): Phase-specific model settings. Keyed by phase name: `coding`, `testing`, `review`, `human_review`.
    - `provider` (string): The provider to use for this phase.
    - `model` (string): The specific model ID to use.
    - `temperature` (float): Sampling temperature (0.0 to 1.0).
    - `max_tokens` (int): Maximum number of tokens to generate.

## Agents Configuration (`agents`)

Configure the three core agents: `coder`, `tester`, and `reviewer`.

- `skills` (list of strings): A list of skill names assigned to the agent. Skills must be defined in the `skills` section.
- `model` (string, optional): Override the default model for this specific agent.

## Skills Configuration (`skills`)

Define the knowledge base and tools available to agents.

- `knowledge` (map): Prompt templates that provide agents with specialized knowledge or instructions.
- `tools` (map): Prompt templates that describe how to use specific tools (e.g., shell, formatter).

## Retry Configuration (`retry`)

Control how the system handles failed LLM requests or tool executions.

- `max_retries` (int): Maximum number of attempts for a failed operation.
- `backoff_base` (int): The base delay for exponential backoff in milliseconds.
- `backoff_max` (int): The maximum delay between retries in milliseconds.

## Logging Configuration (`logging`)

Mini-Orca uses structured logging via Go's `slog` package.

- `level` (string): Log level (`debug`, `info`, `warn`, `error`). Default is `info`.
- `format` (string): Output format (`json` or `text`). Default is `json`.
- `filename` (string, optional): Path to a file where logs should be written. If empty, logs go to stdout.
- `sensitive_keys` (list of strings, optional): A list of keys whose values should be redacted from the logs (e.g., `password`, `api_key`).

## Environment Variables

- `MINI_ORCA_CONFIG`: Path to the configuration file (defaults to `config.yaml`).
