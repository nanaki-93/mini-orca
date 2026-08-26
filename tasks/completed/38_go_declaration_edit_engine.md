# 38 — Build the Go declaration edit engine

## Status

Complete

## Goal

Safely compose a complete candidate file from one replacement or new Go declaration.

## Depends on

Task 32.

## Implementation

- Define `replace_symbol` and `create_symbol` edit modes for exact parser-backed Go functions, methods, and types.
- Parse an isolated declaration plus requested imports, reject multiple declarations, package clauses, unknown targets, invalid names, and unsupported kinds.
- Replace exactly one existing declaration or insert exactly one new top-level declaration at a deterministic location.
- Format the composed Go file and validate that all unrelated declarations and disallowed imports remain unchanged.
- Return the normalized declaration, candidate content/hash, validation diagnostics, and a real insertion-aware unified diff.

## Acceptance criteria

- Replace mode requires one matching symbol before and after.
- Create mode requires the symbol to be absent before and present exactly once after.
- A declaration cannot modify, remove, reorder, or duplicate unrelated declarations.
- Inserted lines do not make every following line appear changed in the diff.

## Verification

- Add parser/composition/diff tests covering functions, methods, types, comments, imports, duplicates, malformed input, and out-of-scope changes.
- Run `go test ./internal/project`, `make fmt-check`, `make vet`, and `git diff --check`.
