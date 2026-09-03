# 107 — Enforce one scoped configuration and LLM boundary

## Status

Complete

## Goal

Replace configuration fallback eras with one strict `analyze`, `bug`, and `function`
profile contract and leave exactly one provider-neutral LLM request boundary.

## Depends on

Task 106.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 107` and name the
  breaking configuration migration, LLM consolidation, documentation, focused tests,
  and pre-existing changes.
- After the commit, post a separate update beginning with `Task 107 complete` and
  report the final schema/client boundary, migration, commands/results, exact commit
  hash, and that Task 108 is next.

## Implementation

- Make `model_scopes.analyze`, `model_scopes.bug`, and `model_scopes.function` the only
  model-profile source of truth.
- Remove the flat `llm` fallback, `agents.coder`, `agents.tester`, `agents.reviewer`,
  role skills, and unused role timeout configuration.
- Decode YAML with unknown-field rejection and return actionable full field paths for
  missing, misspelled, partial, or retired configuration.
- Validate profiles once at startup, including API base safety, required model,
  temperature/token bounds, and secret-free errors.
- Preserve local empty-key behavior and remote-provider confirmation rules. Do not add
  environment interpolation, a vault, Keychain, native provider SDKs, or a settings UI.
- Keep exactly one production `llm.Client` constructor/request flow. Remove unused
  host-style constructors, JSON config loading, scope fallback helpers, and options
  used only by deleted agent code.
- Centralize URL joining, request construction, timeout/cancellation, non-success body
  decoding, and missing-content validation in `internal/llm`.
- Remove provider model enumeration if Task 103 proved it has no consumer; otherwise
  record the concrete consumer and keep only that minimal method.
- Update `config.example.yaml`, `CONFIG.md`, Docker configuration examples, startup
  tests, and migration guidance together. Never edit ignored local `config.yaml`.
- Run `go mod tidy`; make YAML a direct dependency and remove newly unused module data.

## Acceptance criteria

- Exactly one documented and decoded scoped configuration schema exists.
- Unknown/retired keys fail at startup instead of silently falling back.
- Local and online OpenAI-compatible profiles work through one LLM boundary without
  leaking API keys into errors, logs, responses, or metadata.
- Restore, navigation, validation, checks, Apply, and Undo still make no model call.
- The migration guide maps every removed legacy field to a new field or “removed.”
- `go mod tidy -diff` is empty.

## Verification

Run:

```text
go test ./internal/config ./internal/llm ./internal/app
make fmt-check
go test ./...
make test-race
make vet
go mod tidy -diff
git diff --check
```

Automated provider tests must use local `httptest` servers only.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 107 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(config): enforce scoped model profiles
```

Do not amend, squash, tag, or push the commit.
