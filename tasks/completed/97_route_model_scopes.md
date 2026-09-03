# 97 — Route existing operations by model scope

## Status

 Complete

## Goal

Wire the three resolved model profiles into the existing application service so every
prompt-bearing operation uses the intended scope and every non-model operation stays
model-free.

## Depends on

Tasks 95 and 96.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 97` and name service
  routing, retry reuse, provenance/cache freshness, and focused tests.
- After verification, post a separate update beginning with `Task 97 complete` and
  state the behavior, commands/results, changed files, and that Task 98 is next.

## Implementation

- Replace the single-provider fields in `app.Service` with three small resolved runtime
  values owned by the same service. Do not add a second service or orchestration layer.
- Route existing methods exactly as defined in `PLAN.md`:
  - project architectural analysis to `analyze`;
  - selected-file analysis and Analyze-all to `bug`;
  - declaration generation, file chat, and chat revisions to `function`.
- Keep deterministic restore/reindex, verified scans, draft validation/checks, Apply,
  and Undo free of model calls.
- Generalize the existing provider retry helper to accept the selected runtime or
  execution callback. Preserve the global retry/backoff configuration and caller
  context behavior; do not add nested scope retries.
- Stop wrapping the semantic-analysis prompt in coder skills. Use a small shared model
  execution boundary while retaining the coder skill prompt for function generation.
- Record scope, configured/returned model, and sanitized provider origin in project
  analysis, file analysis, and generated declaration-draft provenance.
- Include provider origin, model, scope, prompt version, and context-policy version in
  cache freshness matching. Old records remain readable but are stale when they do not
  prove a match with an explicit scoped profile.
- Keep public model metadata free of API keys and URL user information, query strings,
  and fragments.
- Update startup logging to report one source-free line per effective scope. Do not
  log the full configured URL or key.

## Acceptance criteria

- Three `httptest` providers observe only the calls assigned to their scope.
- Analyze-all reuses `bug` and remains sequential with its current pause/resume/cancel
  behavior.
- File analysis records `bug`, not `coder`, and no coder skills enter its prompt.
- Function proposals retain the current one-session, one-file, one-symbol target and
  structured declaration response.
- A model/provider change makes affected project/file analysis stale without deleting
  deterministic facts or unrelated tool findings.
- Manual draft edits keep original function-model provenance and invalidate validation
  and checks exactly as before.
- No new operation writes source or starts another model scope implicitly.

## Verification

- Extend service, file-analysis, Analyze-all, project-analysis, chat-session, cache,
  and draft-lifecycle tests for routing and provenance.
- Add negative assertions that scan, reindex, checks, Apply, and Undo make no provider
  request.
- Run:

  ```text
  go test ./internal/app ./internal/project ./cmd/daemon
  make fmt-check
  make vet
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
