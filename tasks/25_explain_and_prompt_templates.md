# 25 — Add explain actions and focused prompt templates

## Goal

Offer repeatable one-file/one-symbol assistance without relying on ad-hoc chat wording.

## Depends on

Tasks 13, 20, and 22.

## Implementation

- Add templates: Explain file, Explain symbol, Fix bug, Refactor, Add validation, Add documentation, and Generate test.
- Map each template to allowed action/scope requirements from Task 01.
- For Explain symbol, include deterministic signature, enclosing file facts, and bounded relevant callers/callees when known.
- For Generate test, require a selected test file and test symbol; refuse source-and-test dual-file changes.
- Persist template id and inputs in activity/audit records; allow editable request text before send.

## Acceptance criteria

- A template pre-fills but never submits a request.
- Every template preserves one-file/one-symbol limits.

## Verification

- Add action validation and template rendering tests.
