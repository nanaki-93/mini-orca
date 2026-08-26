# 46 — Implement the Project Summary workspace

## Status

Pending

## Goal

Give users a factual project overview and a clearly separated structured model interpretation.

## Depends on

Tasks 37, 43, 44, and 45.

## Implementation

- Render project type, build metadata, language distribution, file/source counts, line totals, entry points, and revision from deterministic overview data.
- Render purpose, architecture, components, flows, risks, and next steps as separate structured sections.
- Display analysis freshness/status, coverage, and verified/AI finding totals with links to Analysis and Bugs.
- Preserve deterministic content when AI analysis is unavailable, failed, missing, or stale.
- Avoid exposing the absolute imported-project path when a project-relative label is sufficient.

## Acceptance criteria

- Deterministic facts and model interpretation are visually and textually distinct.
- Missing AI output never produces an empty project page.
- Stale analysis is labeled and cannot appear current after a revision change.

## Verification

- Add presentation-state tests for complete, stale, failed, unavailable, and deterministic-only overview data.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
