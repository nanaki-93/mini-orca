# 10 — Add safe generic symbol extraction

## Status

Complete

## Goal

Support useful file facts in non-Go projects without claiming parser-level certainty.

## Depends on

Task 08.

## Implementation

- Create a language-extractor interface with capability and confidence fields.
- Implement conservative fallback extraction for imports and likely declarations in Kotlin, Java, TypeScript, Python, Rust, and unknown text.
- Mark fallback targets as `approximate`; allow manual target entry only with a visible warning.
- Keep exact parser integrations as incremental follow-up work, beginning with languages users actually import.

## Acceptance criteria

- Unsupported syntax never causes an index failure.
- UI can distinguish exact Go targets from approximate targets.

## Verification

- Add mixed-language fixtures and false-positive regression tests.
