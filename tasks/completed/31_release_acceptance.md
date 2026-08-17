# 31 — Validate release acceptance and publish implementation evidence

## Status

Complete

## Goal

Prove the desktop-only milestone satisfies the roadmap before calling it complete.

## Depends on

Tasks 15 and 25–30.

## Implementation

- Create a representative Go fixture project with ignored secrets, source files, test files, malformed source, and Git/non-Git variants.
- Execute documented acceptance flows: import/reindex, file summary, symbol selection, context inspection, focused generation, rejected out-of-scope candidate, focused checks, apply/undo, analyze-all cancellation, comparison/export, and offline model failure.
- Verify no web UI files/routes remain and API documentation reflects the desktop-only daemon.
- Record command output, known limitations, supported language confidence, and any deferred parser work in release notes.

## Acceptance criteria

- Every definition-of-done item in the source roadmap has an automated test or documented reproducible manual check.
- No accepted flow violates one-project/one-file/one-symbol rules.
- Release notes state the active model/privacy behavior and known limitations accurately.

## Verification

- Run `make check`, desktop smoke suite, and the complete fixture acceptance checklist on a clean working tree.
