# 108 — Introduce draft-native identities and Apply stages

## Status

Complete

## Goal

Make draft identity and stale-write safety explicit domain concepts, decompose checks
and Apply into auditable stages, and remove candidate terminology from the retained
Apply boundary.

## Depends on

Task 107.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 108` and name the
  identity values, Apply/check stages, source-safety tests, likely files, and checks.
- After the commit, post a separate update beginning with `Task 108 complete` and
  report the safety structure, commands/results, exact commit hash, and that Task 109
  is next.

## Implementation

- Introduce small internal values for project snapshot, selected file, task target,
  and draft revision/hash identity. Constructors validate once; comparison methods
  express the full matching rule without boolean parameter lists.
- Keep HTTP wire JSON stable in this task and map request DTOs to validated identity
  values at the application boundary.
- Rename the retained Go application action from candidate Apply to draft Apply; update
  call sites and tests and remove the old method name in the same change.
- Decompose Apply into named stages:
  1. request and confirmation validation;
  2. current project/file snapshot loading;
  3. stored draft identity verification;
  4. declaration revalidation and check-policy enforcement;
  5. atomic source replacement;
  6. audit/undo persistence; and
  7. response construction.
- Re-read the source and repeat all identity/hash checks immediately before the write;
  an earlier preview or check is never sufficient authorization.
- Decompose draft checks into identity validation, constrained command selection,
  execution, normalization, and result storage.
- Make the task-aware draft check runner the one retained implementation. Remove
  exported generic candidate-check wrappers and duplicate option validation.
- Ensure check commands run in the intended project directory, honor cancellation and
  timeout, and cannot introduce a general-purpose write interface.
- Give the stored draft record one clear owner and preserve defensive copying at the
  concurrency boundary.

## Acceptance criteria

- Project revision, file path/hash, task identity, draft revision, and draft hash are
  all checked by explicit values rather than repeated partial conditionals.
- Apply is the only retained source-write path and requires explicit confirmation.
- A changed project, source file, target, stored draft, failed required check, or late
  request is rejected before any source write.
- Analysis, chat, generation, validation, and checks leave the selected source byte-for-byte unchanged.
- Successful Apply preserves permissions, audit data, and Undo eligibility.
- No old Apply method or duplicate check runner remains.

## Verification

Run:

```text
go test ./internal/app ./internal/project ./internal/api/handlers
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Tests must cover every identity mismatch, an external source change between check and
Apply, cancellation, failed checks, successful Apply, and Undo.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 108 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(app): make draft apply identities explicit
```

Do not amend, squash, tag, or push the commit.
