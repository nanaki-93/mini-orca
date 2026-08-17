# 27 — Compare candidates and export focused reports

## Goal

Make review choices and handoff useful without changing project files.

## Depends on

Tasks 13, 17, 20, and 23.

## Implementation

- Allow two explicitly requested candidates for the same base hash, target file, target symbol, and action.
- Validate each candidate independently and present a comparison of diff size, scope, checks, model metadata, and user notes.
- Add Markdown export for file summary, selected symbol, findings, candidate diff metadata, checks, and audit reference.
- Redact secrets, excluded-file content, and full prompt/source material from exports by default.

## Acceptance criteria

- Comparison does not create or apply either candidate.
- Export is a readable Markdown artifact with no secret content.

## Verification

- Add comparison conflict and export-redaction tests.
