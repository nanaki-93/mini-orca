# 49 — Build the Editor and always-visible file/symbol brief

## Status

Pending

## Goal

Keep a concise file and selected-symbol explanation visible while the user reads source and chats.

## Depends on

Tasks 44 and 45.

## Implementation

- Preserve the explorer and read-only selectable source view as the Editor center.
- Add line numbers and selected-symbol/finding location emphasis without enabling direct source edits.
- Place the file/symbol brief above chat in the wide right pane and retain a compact brief above source below 1000dp.
- Always show deterministic path, language, size/lines, content hash/freshness, symbol signature, kind, and line range.
- Enrich with cached purpose, responsibilities, dependencies, side effects, symbol explanation, and advisory impact when available.
- Provide Analyze, Refresh, Retry, and symbol selection without navigating away from Editor.

## Acceptance criteria

- Every open file has a useful visible brief even when model analysis is missing or failed.
- Selecting a symbol updates the brief and advisory impact without hiding source.
- Source remains read-only, selectable, and keyboard navigable in both layouts.

## Verification

- Add brief-state, symbol-selection, optional-load, and responsive-layout helper tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
