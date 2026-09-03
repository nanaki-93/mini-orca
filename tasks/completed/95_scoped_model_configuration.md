# 95 — Add scoped model configuration

## Status

 Complete

## Goal

Define one backward-compatible configuration contract that resolves effective
`analyze`, `bug`, and `function` model profiles without changing the application
workflow.

## Depends on

Task 94.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 95` and name the
  configuration, fallback, validation, example, and focused tests in scope.
- After verification, post a separate update beginning with `Task 95 complete` and
  state the behavior, commands/results, changed files, and that Task 96 is next.

## Implementation

- Add fixed scope identifiers for `analyze`, `bug`, and `function` in
  `internal/config`; do not model them as arbitrary agent names.
- Add `ModelScopesConfig` and `ModelProfileConfig` under the top-level
  `model_scopes` key. An explicit profile supports:
  - `api_base_url`;
  - `api_key`;
  - `model`;
  - optional `temperature`;
  - optional `max_tokens`;
  - optional `context_max_tokens`.
- Represent optional numeric values so omission is distinguishable from an explicit
  zero. Apply defaults in one resolver rather than scattering zero-value checks.
- Add one `ResolveModelProfiles` boundary returning complete immutable effective
  profiles for all three scopes.
- Apply the fallback rules from `PLAN.md`, including the established
  `agents.coder.model` precedence for an unconfigured `function` scope.
- Convert legacy host-style `llm.base_url` into the existing `/v1` API base only in
  the resolver. Do not change the serialized meaning of the legacy field.
- Validate explicit profiles narrowly:
  - an explicit `api_base_url` requires a nonempty model;
  - the URL must be absolute HTTP(S), have a host, and contain no user information,
    query, or fragment;
  - token values must be positive and bounded by documented application limits;
  - temperature must remain within the currently supported range.
- Keep `api_key` as a plain local configuration value. Do not add environment
  interpolation, Keychain, encryption, credential storage, or a settings UI.
- Update `config.example.yaml` with commented local and online examples using
  placeholders only. Never add a usable key.
- Remove no existing `llm`, `agents`, retry, or timeout behavior.

## Acceptance criteria

- A config without `model_scopes` resolves to current behavior for every scope.
- Each scope can independently select a different API base and model.
- `function` retains the existing coder-model override when no explicit function
  profile exists.
- An empty key is valid for local profiles; explicit online profiles can carry a key.
- Explicit zero temperature is preserved instead of replaced by the default.
- Malformed, partial, or unsafe endpoint configuration fails at daemon startup with a
  useful error that contains no API key.
- Config loading, saving, and JSON/YAML behavior remain compatible.

## Verification

- Extend `internal/config/config_test.go`, `default` tests, integration tests, and the
  tracked example-config test for:
  - legacy-only configuration;
  - one partial scope override;
  - three independent profiles;
  - coder-model precedence;
  - explicit zero temperature;
  - local empty-key behavior;
  - rejected URL and missing-model cases;
  - absence of secret values in errors.
- Run:

  ```text
  go test ./internal/config
  make fmt-check
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
