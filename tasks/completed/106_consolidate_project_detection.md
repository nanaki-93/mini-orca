# 106 — Consolidate project detection and remove general tools

## Status

Complete

## Goal

Move the one live project/build-file detector to the project domain and delete the
unused general-purpose tool and source-writing executor stack.

## Depends on

Task 105.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 106` and name the live
  detector behavior, tool package deletion, safety boundary, likely files, and checks.
- After the commit, post a separate update beginning with `Task 106 complete` and
  report project detection coverage, removed executors, commands/results, exact commit
  hash, and that Task 107 is next.

## Implementation

- Characterize the project type and build-file results currently consumed by
  `internal/project/analysis.go`.
- Implement that behavior as a small project-domain detector with clear names and
  deterministic precedence.
- Add table-driven tests for supported project types, ambiguous roots, no recognized
  build file, nested ignored directories, and stable build-file reporting.
- Replace the active `internal/tools` import with the new project-domain code.
- Delete the complete `internal/tools` package: executor interfaces, file operations,
  shell execution, formatters, language executors, project detector, types, and tests.
- Do not preserve general command execution or write APIs under another name. Retained
  draft checks continue through their existing constrained check runner until Task 108.
- Remove dependencies made unused solely by the deleted package.

## Acceptance criteria

- Project analysis reports the same supported project type/build file for retained
  fixtures.
- `internal/tools` no longer exists and no production import references it.
- No general-purpose file writer, shell executor, formatter, or language-executor
  abstraction remains.
- Constrained validation/check behavior still works and does not gain a broader write
  capability.

## Verification

Run:

```text
go test ./internal/project ./internal/app
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Inspect `rg -n 'internal/tools|package tools' --glob '*.go' .` and the complete staged
deletion before committing.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 106 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(project): remove general tool executors
```

Do not amend, squash, tag, or push the commit.
