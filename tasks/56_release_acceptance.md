# 56 — Validate focused AI IDE release acceptance

## Status

Pending

## Goal

Prove the complete roadmap satisfies its safety, usability, and validation contract before release.

## Depends on

Tasks 33–55.

## Implementation

- Extend the release fixture with Go replace/create targets, verified parser/vet/test findings, AI suggestions, ignored secrets, stale revisions, and Git/non-Git variants.
- Exercise Summary, Analysis, Bugs, Editor brief, explicit scans, Analyze-all, file chat, manual draft editing, validation rejection, checks, Apply, undo, comparison, and export.
- Verify manual edits invalidate old evidence, stale sessions/drafts cannot apply, and every mutation affects exactly one expected file.
- Verify remote/offline model behavior, cancellation, compact layout, keyboard operation, and source/diff read-only behavior.
- Record automated and reproducible manual evidence plus intentionally deferred non-Go safe editing.

## Acceptance criteria

- Every definition-of-done item in `PLAN.md` has automated or documented reproducible evidence.
- No accepted flow violates project, file, symbol, context, credential, or explicit-Apply boundaries.
- The repository is clean except for the intended implementation and documentation changes.

## Verification

- Run `make check`, the desktop integration/semantics suite, and the complete release fixture checklist.
- Run `git diff --check` and record all results in release notes before marking Complete.
