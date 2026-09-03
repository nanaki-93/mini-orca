# Cleanup contract baseline

This record freezes the Mini-Orca contract before the Tasks 104–117 cleanup.
It is a characterization baseline, not a second API specification.

## Route and consumer inventory

`cleanup-baseline-routes.json` is the machine-readable inventory. The daemon
test compares it with the registered route catalog. Entries marked `retained`
have a current Desktop `ApiClient` method or an operational consumer. After
Task 110, the current route count is 32 and every route is retained.

The maintained public contract remains [the API guide](api-contract.md) and
[the OpenAPI document](openapi.yaml). Their route tables are also tested against
the daemon registrations during this transition.

## Preserved workflow

The retained workflow is one project, one selected file, one selected symbol,
one declaration draft, and an explicit review before a source mutation. Model
generation, project/file analysis, chat, validation, and checks are preview
operations: they must not modify selected source. Only explicit Apply and Undo
may change source, and both preserve their identity guards.

The baseline tests directly cover:

- project import and restore, deterministic file/symbol identity, and default
  loopback binding;
- selected-file and project analysis, bounded context construction, and model
  cancellation;
- declaration draft creation/replacement, validation, isolated checks, explicit
  Apply, stale Apply rejection, and explicit Undo; and
- stale Desktop file-response rejection and project-switch clearing.

## Reproducible evidence

Recorded before cleanup implementation (2026-09-03):

| Metric | Baseline |
| --- | ---: |
| Go production lines (`cmd/` + `internal/`) | 14,512 |
| Go test lines (`cmd/` + `internal/`) | 11,919 |
| Desktop production Kotlin lines | 5,032 |
| Desktop test Kotlin lines | 2,125 |
| Registered HTTP routes | 45 |
| Go statement coverage | 70.9% |
| Desktop tests | 124 |

The verified baseline commands are `make fmt-check`, `go test ./...`, `make
test-race`, `make vet`, and `./desktop/gradlew -p desktop test`. Before Task
103, static analysis still reported the known legacy/dead-code findings listed
in [PLAN.md](../PLAN.md), `go mod tidy -diff` was non-empty, and the desktop
build emitted its Gradle usage-attribute compatibility warning. Later cleanup
tasks must not represent these as already fixed by this baseline.
