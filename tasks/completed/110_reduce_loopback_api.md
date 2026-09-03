# 110 — Reduce the loopback API contract

## Status

Complete

## Goal

Remove deprecated and unconsumed loopback routes and make transport validation
consistent without introducing a routing or middleware framework.

## Depends on

Task 109.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 110` and cite the Task
  103 consumer proof, exact route/DTO/store removals, contract migration, and checks.
- After the commit, post a separate update beginning with `Task 110 complete` and
  report the final route set, breaking changes, commands/results, exact commit hash,
  and that Task 111 is next.

## Implementation

- Use the Task 103 route/consumer inventory as the deletion authority. Preserve
  `/health`, `/status`, and every route with a current Desktop or operational consumer.
- Delete the retired `POST /api/chat/message` 410 endpoint and `GET /api/chat/history`
  alias.
- Delete confirmed-unconsumed current-project projections, transient draft/chat reads,
  draft review, analysis delete, model listing, and audit-history operations. If the
  inventory identifies a real retained consumer, document it and keep only that route.
- Delete matching handler methods, app methods, DTOs, Kotlin client methods, tests,
  OpenAPI operations, and documentation in the same change.
- Remove activity-history recording, route, DTOs, manager methods, and the writer for
  `.mini-orca/sessions/activity.json`.
- Remove the top-level effective-model compatibility projection and return the scoped
  model catalog only.
- Use Go 1.22 method-aware `ServeMux` patterns consistently; remove redundant handler
  method switches.
- Add only small package-local helpers for strict one-object/size-limited JSON decode,
  required project revision parsing, and domain-error-to-status mapping.
- Move the transport-only API error type under `internal/api` and delete
  `internal/errors` when imports reach zero.
- Reject trailing JSON, oversized requests, invalid enums, and missing revisions in a
  consistent error body without exposing secrets or filesystem details.
- Revise the local API contract version/release note. Do not retain aliases or 410
  shims for the removed surface.

## Acceptance criteria

- Every registered route has a named current consumer or explicit health/operations purpose.
- Every production Kotlin client method maps to a registered route and is called.
- No activity history, deprecated route, alias, compatibility projection, or thin
  error-wrapper package remains.
- Equivalent malformed requests receive consistent status codes and JSON errors.
- The desktop's maintained import/analyze/chat/draft/check/Apply/Undo workflow passes.

## Verification

Run:

```text
go test ./internal/api/... ./internal/app ./internal/project
./desktop/gradlew -p desktop test
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Run the route/consumer contract test and inspect `docs/openapi.yaml` against the actual
registration table before committing.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 110 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(api): remove legacy loopback routes
```

Do not amend, squash, tag, or push the commit.
