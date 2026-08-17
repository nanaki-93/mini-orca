# 22 — Add summary and symbol-selection experiences

## Goal

Make each selected file understandable and make real symbols the default generation targets.

## Status

Complete

## Depends on

Tasks 20–21.

## Implementation

- Render deterministic file facts separately from model-generated summary, findings, and suggestions.
- Add Analyze, Refresh, Cancel, and stale/missing states to the Summary tab.
- Render symbols with kind, signature, line range, confidence, and atomic-target state.
- Clicking a symbol fills Focused Action; clicking a suggestion pre-fills an action/request without sending it.
- Add Explain symbol as a read-only action tied to one selected symbol.

## Acceptance criteria

- Users can understand a file and select a target without manually typing its name.
- Model suggestions are clearly labeled and cannot execute automatically.

## Verification

- Test screen state for fresh, stale, failed, approximate, and empty symbol cases.
