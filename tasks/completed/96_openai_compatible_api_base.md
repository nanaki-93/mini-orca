# 96 — Support OpenAI-compatible API bases

## Status

 Complete

## Goal

Let resolved profiles call local or online OpenAI-compatible Chat Completions API
bases, including GPT, Claude compatibility, Gemini compatibility, Ollama, and LM
Studio, without native provider adapters.

## Depends on

Task 95.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 96` and name API-base
  joining, compatibility behavior, cancellation, error handling, and tests.
- After verification, post a separate update beginning with `Task 96 complete` and
  state the behavior, commands/results, changed files, and that Task 97 is next.

## Implementation

- Keep the existing `llm.Client` request/response structs and Chat Completions JSON
  contract.
- Add an API-base-aware constructor or equivalent explicit option. It accepts an API
  prefix such as `/v1` or `/v1beta/openai` and joins exactly
  `chat/completions` without duplicating or discarding path segments.
- Preserve `NewClient` host-style behavior for current callers and tests. Migrate
  configured scoped runtimes to the API-base-aware construction path in Task 97.
- Use the same bearer `Authorization` header for configured online compatibility
  endpoints. Do not add provider SDKs or vendor-specific authentication flows.
- Retain request cancellation, HTTP timeouts, response-size bounds, and sanitized
  non-OK errors. API keys and provider response bodies must not enter errors or logs.
- Make model listing optional and non-blocking:
  - join `models` against the API base when requested;
  - do not prevent configured chat use when a compatibility provider lacks that route;
  - do not add model discovery as a prerequisite for a request.
- Do not add Responses API, Anthropic Messages, Gemini `generateContent`, streaming,
  tool calls, structured-output flags, or reasoning-specific payload fields.

## Acceptance criteria

- API bases ending in `/v1`, `/v1/`, and `/v1beta/openai` produce the correct chat
  URL.
- Existing host-style local configurations continue calling `/v1/chat/completions`.
- Model, messages, temperature, maximum tokens, authorization, and cancellation reach
  the provider as before.
- A missing or unsupported `/models` response does not invalidate a configured model.
- Provider error bodies and keys remain absent from returned errors.
- No test calls the public internet or requires a real provider account.

## Verification

- Extend `internal/llm/client_test.go` and relevant agent integration tests with local
  `httptest` servers representing OpenAI/Claude-style `/v1` and Gemini-style
  `/v1beta/openai` paths.
- Cover trailing slashes, authorization present/absent, cancellation, oversized
  response, non-OK response sanitization, and optional model listing.
- Run:

  ```text
  go test ./internal/llm ./internal/agent
  make fmt-check
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
