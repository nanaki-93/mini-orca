# 08 — Build and persist the deterministic project index

## Status

Complete

## Goal

Index every eligible project file without an LLM call.

## Depends on

Tasks 04 and 07.

## Implementation

- Define index models for project revision, files, hashes, language, size, line count, modification time, imports, symbols, and analysis freshness.
- Scan through `ContextPolicy`; do not index excluded or binary content beyond metadata required for explanation.
- Write `.mini-orca/index.json` using temp-file plus rename.
- Reuse unchanged file entries by content hash and reindex only changed files.
- Rebuild on import and expose a manual reindex operation.

## Acceptance criteria

- Import creates a deterministic index without calling the LLM.
- Changing one source file changes only its index entry and revision.
- Corrupt cache data triggers a safe rebuild.

## Verification

- Add fixture-project, persistence, and atomic-write tests.
