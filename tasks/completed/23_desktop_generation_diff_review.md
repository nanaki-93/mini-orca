# 23 — Deliver focused generation and diff review

## Status

Complete

## Goal

Replace raw generated text with an explicit scope-aware preview and Apply workflow.

## Depends on

Tasks 20–22.

## Implementation

- Build Focused Action form with file, symbol picker/manual fallback, action type, request, scope chip, and Generate Preview.
- Show Context Inspector entry before sending and cancellable generation progress while running.
- Render unified or side-by-side diff, candidate hash, scope validation, and focused-check states.
- Add Discard, Ask for revision, Run focused checks, Apply, and Undo controls with correct enablement.
- Preserve activity history as a collapsible supporting panel rather than primary chat UI.

## Acceptance criteria

- Apply remains disabled until base hash, scope validation, and required checks pass.
- All actions visibly name the one file and one symbol they affect.

## Verification

- Add end-to-end desktop smoke: import → summary → symbol → generate → validate → apply → undo.
