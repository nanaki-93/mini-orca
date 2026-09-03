# 100 — Create strict bug task specifications

## Status

Complete

## Goal

Let the configured `bug` model attach one bounded, exact, machine-validated task
specification to a selected-file AI finding and carry it through the existing
`Prepare fix` action.

## Depends on

Tasks 97–99.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 100` and name the task
  spec schema, model-output validation, finding persistence, Prepare fix handoff, and
  tests.
- After verification, post a separate update beginning with `Task 100 complete` and
  state the behavior, commands/results, changed files, and that Task 101 is next.

## Implementation

- Add a versioned `BugTaskSpec` to the existing selected-file analysis/finding model.
  It contains:
  - exact target path;
  - exact target symbol;
  - deterministic target signature;
  - bounded acceptance criteria;
  - bounded non-goals;
  - optional bounded Go test candidate name and content.
- Extend the semantic-analysis prompt and strict JSON parser rather than making a
  second bug-model call.
- Treat model target metadata as untrusted:
  - force the analyzed file as target path;
  - require exactly one matching symbol from the current index;
  - require exact confidence and atomic edit eligibility;
  - copy the signature from the index;
  - reject unknown, duplicate, approximate, non-atomic, or cross-file targets.
- Bound item counts and lengths. Sanitize persisted task text with the existing finding
  sanitation rules.
- For an optional Go test candidate:
  - require the target file to be Go;
  - parse it as a Go test file;
  - require the package to match the target package;
  - require the named `TestXxx` function to exist exactly once;
  - reject oversized, malformed, ambiguous, or non-test content.
- Persist task specs with the existing per-file analysis and AI finding records. Do
  not create a new task database or source write.
- Preserve old cache records and AI findings without a task spec as readable advisory
  data. Only fresh exact specs enable `Prepare fix`.
- Update the Desktop finding model/card and `Prepare fix` handoff so the existing
  Editor session receives the complete structured requirement, target, acceptance
  criteria, and non-goals.
- Opening, filtering, triaging, or dismissing a finding must not call any model.

## Acceptance criteria

- One file-analysis call can return a validated task spec without another provider
  request.
- Target path and signature always come from deterministic current project state.
- Hallucinated or ambiguous model targets cannot become editable tasks.
- A malformed optional test does not become executable or writable.
- Fresh exact task specs enable the existing Prepare fix path; stale or project-wide
  suggestions remain advisory.
- The resulting chat target is still pinned to project, revision, file hash, mode, and
  one symbol.
- No test candidate or task spec is written into imported source files.

## Verification

- Extend semantic prompt/parser, file-analysis cache, finding-store, project workspace,
  handler contract, Desktop model, Bugs state/presentation, and Editor handoff tests.
- Cover valid replace target, unknown symbol, ambiguous symbol, approximate symbol,
  wrong package, malformed test, oversized fields, stale finding, cache reload, and
  no-second-model-call behavior.
- Run:

  ```text
  go test ./internal/project ./internal/app ./internal/api/...
  ./desktop/gradlew -p desktop test
  make fmt-check
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
