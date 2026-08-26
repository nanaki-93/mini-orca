# 50 — Implement file-scoped desktop chat

## Status

Complete

## Goal

Let users converse about one replace/create target without allowing chat to leave the open file.

## Depends on

Tasks 41, 43, 44, and 49.

## Implementation

- Create or resume a conversation bound to project revision, open path, base hash, edit mode, and selected/new symbol.
- Support Replace selected function/type and Create new function/type modes with clear validation before send.
- Show the bound file and target above the composer and keep context inspection, cancellation, and provider confirmation.
- Render user/assistant turns for the active file session; do not substitute global source-free activity for conversation history.
- Allow findings and analysis suggestions to prefill the composer while keeping Send explicit.
- Render each assistant code proposal as a new draft card linked to prior revisions.

## Acceptance criteria

- The UI cannot submit a message for a file other than the currently open file.
- Replace requires an exact selected symbol; Create requires a valid new absent name.
- Changing file/revision stales the previous conversation and prevents its draft from applying.
- Receiving a message or draft never writes project source.

## Verification

- Add controller and transport tests for binding, mode validation, prefill, cancellation, stale sessions, and lineage.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
