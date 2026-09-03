# 117 — Complete cleanup quality and acceptance

## Status

Pending

## Goal

Install reproducible quality gates, close every remaining cleanup finding, run full
automated and manual acceptance, and truthfully complete the plan.

## Depends on

Tasks 103–116.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 117` and name the
  quality tools, residual audit, full validation matrix, manual smoke test, final
  metrics, and commit-history verification.
- After the commit, post a separate update beginning with `Task 117 complete` and
  report final architecture/outcome, all command results, manual limitations, metrics,
  exact commit hash, and that the sequence is finished.

## Implementation

- Add a reproducible, version-pinned project quality command for Go staticcheck,
  production dead-code analysis, and production clone detection. Keep tool dependencies
  out of the runtime graph and make installation/cache behavior explicit.
- Add focused Detekt and Kotlin formatting tasks compatible with the retained build.
  Enforce correctness, unused code, unsafe casts, exception handling, meaningful
  complexity, and Compose-relevant structure.
- Exclude generated output and import-only clone noise. Do not hide real findings in a
  broad baseline or blanket suppression.
- Run the tools and fix every in-scope production finding. Delete code proven unused;
  do not create dummy callers or suppressions solely to make a tool pass.
- Confirm the final runtime package graph has no `internal/orchestrator`,
  `internal/agent`, `internal/tools`, `internal/workflow`, or `internal/errors` branch.
- Confirm no legacy route, configuration key, persistence writer, compatibility DTO,
  dead client method, or obsolete current-documentation claim identified in `PLAN.md`
  remains.
- Record before/after production/test line counts, packages, routes, dead functions,
  duplication, complexity hotspots, Go coverage, Desktop test count, and container size
  where reproducible.
- Run the full automated matrix and the disposable-project manual smoke checklist from
  `PLAN.md`. Never use a real user project for destructive acceptance.
- Record unavailable environment-dependent manual steps accurately with their risk;
  never fabricate a pass. Resolve automatable failures before completion.
- Update release acceptance and set `PLAN.md` to Complete only when definition-of-done
  evidence is present. If a permitted manual limitation remains, state it adjacent to
  the status and in the final report.
- Verify Tasks 103–117 have exactly one sequential commit each with the required
  subjects and no combined, amended, or pushed task commit.

## Acceptance criteria

- Staticcheck, vet, formatting, dead-code, clone, and Detekt gates have no unexplained
  production findings.
- Production duplication is below 1% excluding imports/generated code and no known
  domain rule is duplicated.
- No non-test function exceeds cyclomatic complexity 15 and no production file exceeds
  500 lines without a narrow written justification reviewed in this task.
- Remaining live-package coverage does not regress from the Task 103 baseline; Apply,
  Undo, stale-write, cancellation, corrupt-metadata, and late-response paths have
  direct coverage.
- The full supported validation passes and the manual smoke result is truthful.
- The repository contains no task-owned uncommitted/staged change after the commit and
  no unrelated user work was included.

## Verification

Run at minimum:

```text
make fmt-check
go test ./...
make test-race
make vet
go mod tidy -diff
./desktop/gradlew -p desktop test
make check
make quality
git diff --check
```

Run the Kotlin quality/format tasks added by this backlog, validate documentation links,
inspect the final dependency graph and route inventory, and complete the `PLAN.md`
manual smoke checklist on a disposable fixture.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, set `PLAN.md` to Complete with truthful evidence, stage only Task 117
changes, inspect the complete staged diff, and create exactly one commit:

```text
chore: complete legacy cleanup acceptance
```

Do not amend, squash, tag, or push the commit. After committing, confirm nothing is
staged and no Task 117 change remains uncommitted.
