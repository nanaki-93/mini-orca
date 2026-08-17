# 19 — Add explicit Apply, Undo, and audit history

## Goal

Safely write one validated candidate and allow immediate recovery.

## Depends on

Tasks 07, 17, and 18.

## Implementation

- Implement Apply with matching project revision, base file hash, validated candidate id, and explicit user confirmation.
- Re-read the file before write; reject conflicts.
- Write temp file, fsync where supported, atomically rename, and retain a project-scoped backup.
- Implement single-step Undo guarded by post-apply content hash.
- Record audit entry: action, target, hashes, validation/check results, timestamps, model profile, and outcome; exclude source and secrets.

## Acceptance criteria

- Apply never silently changes another file.
- Undo restores only the immediately applied unmodified file.
- Interrupted writes leave either original or complete candidate, never partial data.

## Verification

- Add conflict, write-failure, undo, restart, and audit-redaction tests.
