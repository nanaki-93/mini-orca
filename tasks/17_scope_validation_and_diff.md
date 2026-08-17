# 17 — Validate atomic scope and generate diffs

## Goal

Prove that a candidate changes only the selected symbol and allowed imports in one file.

## Depends on

Tasks 09 and 16.

## Implementation

- Build `GenerationValidator` around original/candidate content and selected target metadata.
- Parse Go originals and candidates; verify same target identity, no duplicate/removed symbols, and unchanged non-target declarations.
- Allow only minimal required import changes in `symbol_plus_imports` mode.
- Add a generic conservative validator for non-Go files that refuses ambiguous candidates.
- Produce structured validation diagnostics and a unified diff with line metadata.

## Acceptance criteria

- Editing a second function, renaming the target, or changing another file is rejected.
- Invalid syntax yields diagnostics and no applicable candidate.
- UI receives `strict_symbol` or `symbol_plus_imports` validation state.

## Verification

- Add Go AST diff fixtures for valid and forbidden changes.
