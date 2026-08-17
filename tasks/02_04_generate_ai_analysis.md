# Task 2.4 — Generate and persist the AI analysis file

**Phase:** 2 — Project analysis backend  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Medium

## Goal

Run an architectural LLM analysis during import and persist a durable, factual project report.

## Plan coverage

- Ask the configured LLM for an architectural summary.
- Generate and persist `.mini-orca/analysis.md`.

## Dependencies

- Task 2.2
- Task 2.3

## Files

- `internal/project/analysis.go`
- `internal/project/project_test.go`

## Implementation steps

1. Define a narrow chat-client interface so analysis is unit-testable.
2. Send the project context with a software-architect system instruction.
3. Request concise Markdown covering purpose, architecture, entry points, flow, risks, and next steps.
4. Reject fabricated context through prompt wording and retain deterministic scanner facts separately.
5. Record AI status as `complete`, `failed`, or `unavailable`.
6. If the LLM is unavailable, keep import useful by writing the factual inventory with a clear status instead of losing all results.
7. Write `.mini-orca/analysis.md` with timestamp, type, build file, counts, languages, AI summary, and complete inventory.

## Acceptance criteria

- A valid import always attempts the configured AI task.
- Successful AI Markdown appears in the persisted analysis file and API result.
- LLM failure produces a factual analysis file with non-success AI status.
- The analysis directory is created inside, never outside, the imported project.

## Verification

```bash
go test ./internal/project -run TestAnalyzerWritesAnalysisAndContextIncludesInventory
```
