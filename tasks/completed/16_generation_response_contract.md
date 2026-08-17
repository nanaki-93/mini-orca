# 16 — Define structured generation response contract

## Status

Complete

## Goal

Make model output machine-checkable before any diff or Apply work begins.

## Depends on

Tasks 02, 06, and 07.

## Implementation

- Replace free-form complete-file output with a versioned structured response containing target path, target symbol, scope mode, candidate content, and optional rationale.
- Temporarily accept exactly one fenced code block for compatible models; reject all other free-form layouts.
- Capture base project revision and file hash before generation.
- Return generation id, effective model/profile, context manifest summary, and normalized candidate hash.
- Never persist or apply an unvalidated candidate.

## Acceptance criteria

- Multiple code blocks, extra paths, missing candidate content, and wrong target data are rejected.
- Cancellation leaves no successful candidate record.

## Verification

- Add parser and fake-model response matrix tests.
