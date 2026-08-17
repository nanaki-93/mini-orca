# 07 — Add project revisions and project-scoped state

## Goal

Prevent stale candidates and cross-project history from affecting the active project.

## Depends on

Task 02.

## Implementation

- Give each imported project a stable id and revision derived from canonical root plus index/content state.
- Require project revision and base file hash for candidate, apply, and undo operations.
- Move chat/activity/audit data to project-scoped `.mini-orca/sessions/` storage with atomic writes.
- Return conflict responses when project or file state changed during a request.
- Define retention and safe recovery behavior for incomplete session files.

## Acceptance criteria

- Importing project B cannot expose project A's activity.
- A candidate from an older file hash cannot be applied.

## Verification

- Add concurrent import/generate tests and restart persistence tests.
