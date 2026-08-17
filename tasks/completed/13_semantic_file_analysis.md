# 13 — Generate semantic summaries one file at a time

## Status

Complete

## Goal

Use the local model to explain one selected file while separating facts from interpretation.

## Depends on

Tasks 02, 05, 09, 10, and 12.

## Implementation

- Build a versioned prompt from file content, deterministic facts, project summary, relevant signatures, and context manifest.
- Require structured JSON for purpose, responsibilities, side effects, risks, suggestions, and per-symbol explanations.
- Validate model output against schema, size limits, and selected-file scope.
- Label model-derived content as suggestions; never overwrite deterministic facts.
- Cache successes and actionable failures; honor cancellation.

## Acceptance criteria

- An analysis explains purpose, symbols, dependencies, side effects, risks, suggestions, and freshness.
- Only one target file is sent for semantic analysis, aside from allowed compact project facts.

## Verification

- Add fake-LLM JSON success, malformed JSON, empty, timeout, and prompt-scope tests.
