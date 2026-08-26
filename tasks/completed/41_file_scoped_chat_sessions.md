# 41 — Add file-scoped chat sessions and draft generation

## Status

Complete

## Goal

Turn one-shot generation into a conversation pinned to the open file and one replace/create symbol target.

## Depends on

Tasks 38, 39, and 40.

## Implementation

- Define a session bound to project ID/revision, open path, base hash, edit mode, and exact selected or requested new symbol.
- Reject attempts to retarget a session or use it after the file/project becomes stale.
- Change generation prompts/responses from whole-file candidates to one structured Go declaration plus required imports and a concise assistant explanation.
- Create a new draft for every successful assistant proposal and link revision requests to the preceding draft.
- Keep visible messages scoped to the file session while durable activity remains source-free and project-scoped.
- Preserve cancellation, retry, context inspection, remote-provider confirmation, and bounded one-file context.

## Acceptance criteria

- A chat message cannot generate a draft for any file other than its bound open path.
- Replace mode requires an exact selected symbol; create mode requires a valid absent name.
- Model output never directly determines a full-file write or Apply.
- Session and draft lineage support explicit “revise this proposal” conversation turns.

## Verification

- Add prompt, session isolation, stale revision, response parsing, cancellation, and lineage tests in `internal/app` and handlers.
- Run `go test ./internal/app ./internal/api/handlers ./internal/agent`, `make vet`, and `git diff --check`.
