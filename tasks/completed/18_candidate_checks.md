# 18 — Run focused candidate validation checks

## Status

Complete

## Goal

Report whether an approved-scope candidate parses, formats, lints, and passes selected checks before Apply.

## Depends on

Task 17.

## Implementation

- Define check states: not-run, passed, failed, skipped, canceled, and unavailable.
- Run parser and formatter checks against an isolated candidate workspace or temporary file; never modify the real project.
- Select focused commands from detected project type with an explicit command preview and timeout.
- Keep tests optional/configurable; capture sanitized stdout/stderr and exit code.
- Return an aggregate `applicable` decision only when required checks pass.

## Acceptance criteria

- Candidate checks cannot write to the user project.
- A failed required check disables Apply and shows actionable output.

## Verification

- Add executor tests for cancellation, command failures, and project immutability.
