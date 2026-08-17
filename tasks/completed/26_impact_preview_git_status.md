# 26 — Add impact preview and read-only Git status

## Status

Complete

## Goal

Help users judge a focused change without enabling repository-wide mutation.

## Depends on

Tasks 08, 20, and 22.

## Implementation

- Use index imports and symbol references to show likely dependent files/symbols with confidence labels.
- Add a read-only Git adapter that reports repository availability, branch, target-file status, and candidate-relevant diff state.
- Keep impact results advisory; do not expand generation context or scope automatically.
- Gracefully support non-Git projects and unavailable Git executable.

## Acceptance criteria

- Impact preview never sends extra files to the model unless the user explicitly accepts them in Context Inspector.
- Git view cannot stage, commit, push, reset, or mutate repository state.

## Verification

- Add Git fixture tests and non-Git fallback tests.
