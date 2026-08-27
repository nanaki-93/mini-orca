# 69 — Refactor the Project Summary visual hierarchy

## Status

Complete

## Goal

Make Summary a scan-friendly overview that clearly separates deterministic facts
from optional model interpretation.

## Depends on

Task 68.

## Implementation

- Recompose project metrics, language/build facts, revision, interpretation, coverage,
  finding counts, and next actions into shared themed sections/cards.
- Preserve explicit labels distinguishing deterministic data from model output.
- Use concise Open Analysis/Open Bugs actions and clear missing/stale/failed states.
- Remove any facts duplicated by the retired global metric strip.
- Keep long content scrollable and selectable where appropriate.

## Acceptance criteria

- Deterministic facts remain available when model analysis is missing/failed.
- Model interpretation and AI risks cannot be mistaken for verified facts.
- Coverage/finding counts and workspace actions remain accurate and keyboard reachable.
- Empty/importing/stale states use the shared visual language.

## Verification

- Extend Summary state/render-label tests where presentation logic is pure.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
