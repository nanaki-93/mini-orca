# 05 — Expose context-inspector data

## Status

Complete

## Goal

Provide the desktop client with an exact preview of what a model request will receive.

## Depends on

Task 04.

## Implementation

- Define a `ContextManifest` with included files/snippets, exclusions and reasons, token estimate, byte limit, and truncation state.
- Refactor context building to return manifest plus prompt material without logging source text.
- Add a read-only endpoint for the pending file/symbol/action context.
- Redact content in diagnostics; return paths, sizes, hashes, reasons, and counts by default.

## Acceptance criteria

- The manifest reflects the exact context used by generation and summary calls.
- Sensitive file content is never returned by the inspector.

## Verification

- Compare fake LLM request bodies to the manifest in integration tests.
