# 12 — Create versioned file-analysis cache

## Status

Complete

## Goal

Persist semantic summaries safely and invalidate them exactly when inputs change.

## Depends on

Tasks 04, 07, and 08.

## Implementation

- Add `FileAnalysis`, finding, suggestion, and status models.
- Store entries under `.mini-orca/file-analysis/` keyed by normalized path hash; never store source content.
- Include content hash, project revision, schema version, prompt version, model/profile, and context-policy version.
- Implement fresh, missing, stale, failed, and running states.
- Use atomic write/read recovery and delete invalid entries safely.

## Acceptance criteria

- A source edit makes only its own analysis stale.
- A model or policy change invalidates affected summaries.
- Cache files contain no original code or prompt body.

## Verification

- Add round-trip, corruption, invalidation, and concurrent-read/write tests.
