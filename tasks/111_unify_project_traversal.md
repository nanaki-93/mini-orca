# 111 — Unify project source traversal

## Status

Pending

## Goal

Replace equivalent recursive project scans with one deterministic eligible-source
walker while preserving every path, ignore, limit, symlink, and cancellation rule.

## Depends on

Task 110.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 111` and name the
  duplicated walkers, policy differences, intended shared boundary, tests, and checks.
- After the commit, post a separate update beginning with `Task 111 complete` and
  report the consolidated policy, behavior preservation, commands/results, exact
  commit hash, and that Task 112 is next.

## Implementation

- Inventory every production file walk used by import, indexing, analysis, context,
  project report, and scans; classify semantic differences before extracting code.
- Implement one project-domain eligible-source walker for callers whose rules are
  actually equivalent.
- Make inputs explicit: canonical root, ignored directories, allowed extensions,
  symlink behavior, maximum files, deterministic ordering, and cancellation context.
- Return domain-relevant results and errors; do not expose `filepath.Walk` callbacks to
  application or handler callers.
- Preserve maximum-count behavior, partial-result/error semantics, path normalization,
  root escape protection, hidden/metadata exclusions, and stable ordering.
- Keep genuinely different traversal semantics local and document why they cannot use
  the shared walker.
- Remove `scan`, `listContextFiles`, and other exact duplicate/dead helpers after callers
  migrate.
- Do not add a source inventory cache or filesystem watcher.

## Acceptance criteria

- Equivalent scans use one deterministic implementation and one policy definition.
- Import, index, context, analysis, and verified scans report the same eligible files
  for existing fixtures.
- Symlink escapes, ignored/generated directories, unsupported extensions, limits,
  permission errors, and cancellation remain covered.
- No broad callback abstraction, cache, or speculative project scanner is introduced.

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

Use focused fixtures to compare old characterization expectations to the new walker;
do not retain the old implementation only for comparison after the task completes.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 111 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(project): unify source traversal
```

Do not amend, squash, tag, or push the commit.
