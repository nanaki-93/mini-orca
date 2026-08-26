# 43 — Extend desktop API contracts

## Status

Pending

## Goal

Give Compose Desktop typed access to every project workspace, finding, scan, chat-session, and editable-draft capability.

## Depends on

Tasks 37, 40, and 41.

## Implementation

- Add serializable models for structured project analysis, overview, index diagnostics, findings, scan progress, and Analyze-all job progress.
- Add models for file chat sessions/messages, replace/create modes, draft revisions/states, validation, checks, and review eligibility.
- Add `ApiClient` methods for every new live endpoint, including finding triage, scan start/cancel, chat creation/message, draft update/validate/check, and draft Apply.
- Handle optional responses and Analyze-all/scan `204 No Content` without decoding an empty body.
- Keep unknown-field compatibility and structured API error handling.

## Acceptance criteria

- Desktop code performs no untyped parsing for new contracts.
- Transport tests cover success, structured failure, optional collections, and no-content responses.
- Source content appears only in selected-file/draft responses where explicitly required, never in findings or job metadata.

## Verification

- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
