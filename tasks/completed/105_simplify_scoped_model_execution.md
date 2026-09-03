# 105 — Simplify scoped model execution

## Status

Complete

## Goal

Replace the generic agent hierarchy with one direct scoped LLM execution path owned by
the application workflow, then delete `internal/agent` completely.

## Depends on

Task 104.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 105` and name the live
  prompt/request behavior being moved, the agent files being removed, focused tests,
  and existing changes.
- After the commit, post a separate update beginning with `Task 105 complete` and
  report the new execution boundary, deletion result, commands/results, exact commit
  hash, and that Task 106 is next.

## Implementation

- Trace every production import of `internal/agent` and classify it as retained model
  behavior or retired role/orchestration behavior.
- Move declaration-edit prompt construction into a focused `internal/app` file that
  returns ordinary `[]llm.ChatMessage` values.
- Preserve the exact function-scope context, task pinning, remote confirmation,
  retries, cancellation, timeout, and response-content behavior required by the live
  workflow.
- Make the application model runtime call `llm.Client.Chat` directly through one small
  execution helper. Do not create a replacement `Agent` interface.
- Remove generic `Agent`, `Result`, `Phase`, `Registry`, `CoderAgent`, reviewer, tester,
  workflow, and prompt-registry types.
- Delete all of `internal/agent` and tests dedicated to its retired abstractions.
- Move only behavior assertions for the retained declaration prompt/model call into
  `internal/app` or `internal/llm` tests.
- Remove role skill strings and metadata that no retained path consumes. Task 107 owns
  the corresponding configuration-schema removal.
- Do not alter scope resolution or the public API in this task.

## Acceptance criteria

- No production or test import references `internal/agent`.
- There is one direct application-to-LLM request path and no generic role registry.
- Function prompts retain the selected project/file/symbol/task context and bounded
  context policy.
- Timeout, cancellation, retry, malformed response, and provider-error behavior remain
  covered.
- Preview generation still performs no source write.

## Verification

Run:

```text
go test ./internal/app ./internal/llm
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Inspect imports with `rg -n 'internal/agent|package agent' --glob '*.go' .` and confirm
there is no live result.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 105 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(app): remove generic agent layer
```

Do not amend, squash, tag, or push the commit.
