# 103 — Establish the cleanup contract and characterization baseline

## Status

Complete

## Goal

Freeze the behavior that the cleanup must preserve, make the maintained route surface
measurable, and add characterization coverage before deleting legacy packages.

## Depends on

Task 102.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 103` and name the
  contract inventory, safety characterizations, baseline evidence, likely files, and
  checks in scope.
- After the commit, post a separate update beginning with `Task 103 complete` and
  report behavior, commands/results, changed files, exact commit hash, and that Task
  104 is next.

## Implementation

- Re-read `PLAN.md` and turn its non-negotiable invariants into tests that do not
  depend on `internal/orchestrator`, generic agents, or legacy candidates.
- Record the current daemon route inventory and its named Desktop or operational
  consumer in a maintained cleanup baseline document.
- Make route registration inspectable from tests with the smallest local extraction;
  do not add a router framework or move business logic into `cmd/`.
- Add a contract test that fails when a maintained Kotlin client path has no registered
  daemon route or a non-operational daemon route has no named maintained consumer.
- Add or tighten direct characterization tests for:
  - project import and restore;
  - file/symbol selection identities;
  - analysis and context construction;
  - draft creation and replacement;
  - validation/checks without source mutation;
  - explicit Apply, stale Apply rejection, and explicit Undo;
  - cancellation and stale-response rejection; and
  - default loopback binding.
- Prefer extending current integration/contract tests over creating duplicate suites.
- Fix the tautological live finding assertion reported by static analysis. Delete no
  legacy package in this task and do not repair assertions whose sole subject will be
  deleted in Tasks 104–106.
- Record the baseline commands, counts, coverage, static findings, route set, and known
  Gradle warning without claiming that later quality gates already pass.

## Acceptance criteria

- Retained preview, check, Apply, and Undo invariants have tests independent of the
  packages scheduled for deletion.
- Generation, analysis, chat, validation, and checks are proven not to mutate source.
- The route/consumer inventory is complete enough for Task 110 to make deletions
  without guesswork.
- Baseline evidence is reproducible and distinguishes passing checks from known
  findings.
- No production behavior, route, configuration field, or persistence format changes.

## Verification

Run:

```text
make fmt-check
go test ./...
make test-race
make vet
./desktop/gradlew -p desktop test
git diff --check
```

Inspect the complete diff and confirm it contains characterization, route-inventory,
and baseline work only.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 103 changes, inspect the staged diff, and create
exactly one commit:

```text
test: characterize cleanup contract
```

Do not amend, squash, tag, or push the commit.
