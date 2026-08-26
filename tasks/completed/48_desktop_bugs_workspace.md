# 48 — Implement the Bugs workspace

## Status

Complete

## Goal

Make verified tool findings and AI suggestions searchable, triageable, and actionable without conflating their confidence.

## Depends on

Tasks 37, 43, 44, and 45.

## Implementation

- Render Verified/tool-reported issues and AI suggestions in separate labeled sections.
- Show severity, provenance/confidence, evidence, path, line/symbol, status, revision, and freshness.
- Add text search plus source, severity, freshness, and lifecycle filters.
- Support open-in-Editor and revision-guarded triage actions.
- Add explicit Run verified scan and Cancel scan controls with progress and warnings; never scan automatically.
- Make Prepare fix open the file and prefill chat without sending, generating, validating, or applying.

## Acceptance criteria

- AI findings cannot be visually or textually mistaken for verified/tool-reported issues.
- Stale or locationless findings cannot prepare an active fix until refreshed.
- No Bugs action writes source or starts generation without another explicit user action.

## Verification

- Add filter, classification, triage, scan-progress, finding-navigation, and Prepare-fix state tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
