# 98 — Expose scoped models and enforce scoped confirmation

## Status

 Complete

## Goal

Make the daemon API disclose non-secret effective model information for all scopes and
enforce remote-provider confirmation against the actual scope used by each request.

## Depends on

Task 97.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 98` and name the
  additive model catalog, confirmation routing, API compatibility, and contract tests.
- After verification, post a separate update beginning with `Task 98 complete` and
  state the behavior, commands/results, changed files, and that Task 99 is next.

## Implementation

- Extend `GET /api/models/current` with a `scopes` object keyed by `analyze`, `bug`, and
  `function`.
- Preserve the current top-level fields as a compatibility projection of the effective
  `function` profile.
- Each scope returns only:
  - scope/profile name;
  - model;
  - sanitized provider origin;
  - `remote_provider`;
  - temperature;
  - maximum output tokens;
  - context budget;
  - effective timeout and retry limit.
- Never serialize the API key, configured URL path/query, request headers, or any
  prompt.
- Change remote confirmation to accept a fixed scope and evaluate that scope's
  provider locality.
- Apply it at the existing prompt-bearing handlers:
  - import uses `analyze`;
  - selected-file analysis plus Analyze-all start/resume use `bug`;
  - chat messages and compatibility generation use `function`.
- Keep the existing `confirm_remote_provider` request field. Do not add a token,
  remembered server-side approval, or a new confirmation endpoint.
- Keep the daemon as the final authority even when a client sends an incorrect flag.
- Update OpenAPI, the route contract, and handler tests in the same task so the live
  contract never diverges.

## Acceptance criteria

- Existing clients can still decode the top-level effective function model.
- New clients can identify model and locality for all three scopes.
- A remote scope rejects an unconfirmed prompt request before any provider call.
- Confirming one scope does not bypass the flag required on another request.
- A local scope proceeds without confirmation even if another scope is remote.
- Restore, reindex, scans, checks, Apply, and Undo require no provider confirmation.
- API responses and errors contain no API key.

## Verification

- Extend model-handler, project-handler, chat-handler, candidate-handler, daemon route,
  and OpenAPI/route-contract consistency tests.
- Use mixed local/remote `httptest` configurations and assert provider request counts.
- Run:

  ```text
  go test ./internal/api/... ./internal/app ./cmd/daemon
  make fmt-check
  make vet
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
