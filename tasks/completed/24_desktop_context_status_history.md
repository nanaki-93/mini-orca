# 24 — Add context, local-model status, and project activity UI

## Status

Complete

## Goal

Make model usage, privacy boundaries, and prior focused work visible in the desktop app.

## Depends on

Tasks 05, 07, 20, and 21.

## Implementation

- Add Context Inspector dialog/pane with included paths, exclusion reasons, token estimate, truncation, and remote-provider warning.
- Display daemon connection, model name/profile, endpoint locality, context window when known, latency, and token-use metadata.
- Display project-scoped activity/audit history with filters and no source-code leakage.
- Add cancel/retry behavior for model calls and actionable offline/error banners.

## Acceptance criteria

- The user can verify what will be sent before requesting a model call.
- Project activity does not cross project boundaries or show secrets.

## Verification

- Test privacy redaction and status transitions in the view model.
