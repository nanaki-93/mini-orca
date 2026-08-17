# 04 — Implement context privacy and inclusion policy

## Goal

Ensure project analysis and local-model prompts include only intentional, safe, bounded context.

## Depends on

Task 01.

## Implementation

- Add one `ContextPolicy` shared by import, indexing, summaries, and generation.
- Honor `.gitignore` and project-local `.mini-orcaignore`.
- Deny secrets and unsafe paths by name/pattern: `.env*`, credential/key/certificate files, tokens, and local overrides.
- Exclude generated, minified, dependency, and lock files by default; allow explicit project configuration overrides.
- Replace raw byte-only limits with token estimation plus hard byte/file limits.
- Make policy decisions explainable: include/exclude reason, truncation, and estimated tokens.
- Require a user confirmation signal for non-loopback LLM endpoints.

## Acceptance criteria

- Secret fixtures never reach a fake LLM server.
- The same policy governs all prompt-building paths.
- Included/excluded decisions are deterministic and testable.

## Verification

- Add policy table tests, `.gitignore` fixtures, and path/symlink regression tests.
