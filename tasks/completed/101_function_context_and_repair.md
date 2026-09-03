# 101 — Add bounded function context and explicit repair

## Status

Complete

## Goal

Give the configured `function` model a small declaration-focused prompt, verify an
optional bug test only in the isolated check workspace, and support at most three
explicit check-driven repair revisions.

## Depends on

Task 100.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 101` and name function
  context, task-spec pinning, temporary test checks, repair limits, and regression
  tests.
- After verification, post a separate update beginning with `Task 101 complete` and
  state the behavior, commands/results, changed files, and that Task 102 is next.

## Implementation

- Add scope-aware options to the existing context builder. Preserve policy decisions,
  canonical paths, manifests, deterministic ordering, and truncation reporting.
- Build function context from:
  - the pinned task specification when present;
  - the selected declaration or create-symbol stub;
  - package and import facts;
  - directly relevant indexed signatures;
  - prior declaration draft only for an explicit revision request.
- Enforce the resolved `function.context_max_tokens` budget. Do not include unrelated
  complete source files merely because the configured model has a larger context.
- Keep the existing declaration-only JSON response and daemon-side file composition.
- Pin an optional task spec to the existing chat session and generated draft identity.
  Reject a spec whose revision, file hash, target path, or symbol is stale.
- Extend explicit draft checks for a validated task test candidate:
  - use the existing temporary copied workspace;
  - choose a non-conflicting generated `_test.go` filename in the target package;
  - run the test against the captured base and require it to fail for the intended
    assertion;
  - run the same test against the composed candidate and require it to pass;
  - never copy the generated test back to the imported project;
  - return bounded sanitized evidence in the check report.
- Add an explicit Desktop `Revise with check output` action when the latest task-bound
  draft has failed checks. It sends the sanitized evidence as the next message in the
  existing pinned session and parent-draft chain.
- Limit check-driven repair revisions to three per task/session. Normal user-authored
  conversation revisions retain their existing behavior unless they use the repair
  action.
- Keep provider transport retries separate from repair revisions. Do not start a model
  call automatically after checks fail.
- A passing check returns to the current human review and explicit Apply flow. Do not
  add a commit action.

## Acceptance criteria

- A function request stays within its configured token budget and contains no
  unrelated complete source.
- Direct manual file chat still works without a bug task spec.
- Task-bound sessions cannot be retargeted or reused after project/file state changes.
- The optional generated test is created only in temporary workspaces and cannot
  overwrite an existing project test.
- The test must fail on the base and pass on the candidate before its dedicated check
  is successful.
- Check errors are sanitized and bounded before an explicit repair message.
- At most three repair actions reach the function provider; a fourth is disabled and
  rejected by the daemon.
- No check starts a provider call by itself, and no draft/check/test writes imported
  source before explicit Apply.

## Verification

- Extend context, chat-session, draft lifecycle, candidate-check, handler, API client,
  Desktop controller, editor action, and integration tests.
- Cover replace/create context, truncation, missing spec, stale spec, baseline test
  unexpectedly passing, candidate test failing/passing, test filename collision,
  cancellation, sanitized evidence, three-repair limit, and no automatic provider
  request.
- Run:

  ```text
  go test ./internal/project ./internal/app ./internal/api/...
  ./desktop/gradlew -p desktop test
  make fmt-check
  make vet
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
