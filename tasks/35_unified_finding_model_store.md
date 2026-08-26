# 35 — Create the unified finding model and store

## Status

Pending

## Goal

Represent verified tool output and AI suggestions with stable provenance, location, freshness, and lifecycle state.

## Depends on

Task 34.

## Implementation

- Define finding source, confidence, severity, status, location, evidence, revision/hash, detected time, and freshness fields with bounded values.
- Generate deterministic finding IDs from stable source/rule/location/message inputs so unchanged reruns preserve triage.
- Persist source-free or sanitized findings atomically under `.mini-orca/` without storing prompts, credentials, or source files.
- Preserve open/dismissed/fixed triage for unchanged findings; mark revision/hash mismatches stale and retire findings no longer reported.
- Adapt fresh project/file AI risks into `suggested` findings while retaining their originating analysis and file path.

## Acceptance criteria

- AI findings cannot be represented as verified/tool-reported findings.
- Triage survives an unchanged rerun and stale findings cannot be offered as current fixes.
- Corrupt finding storage recovers safely without affecting project source.

## Verification

- Add table-driven persistence, ID, triage, sanitization, and invalidation tests in `internal/project`.
- Run `go test ./internal/project -race`, `make vet`, and `git diff --check`.
