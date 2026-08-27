# 71 — Refactor the Bugs workspace

## Status

Pending

## Goal

Make verified tool findings and AI suggestions easy to filter, compare, triage, and
open without weakening their provenance distinction.

## Depends on

Task 70.

## Implementation

- Replace the stacked text-field layout with a compact themed search/filter toolbar.
- Render verified, suggested, and unclassified findings as structured cards/list rows
  with source, confidence, severity, location, lifecycle, freshness, revision, and evidence.
- Keep verified/tool-reported findings visually and textually distinct from AI suggestions.
- Preserve explicit scan Start/Cancel, filtering, triage, Open in Editor, and Prepare fix guards.
- Use lazy rendering for large finding collections and retain keyboard focus/selection.
- Provide clear empty, running, canceled, stale, and warning states.

## Acceptance criteria

- Filters return the same findings as current pure filter logic.
- Only fresh located findings can Prepare fix.
- Provenance/confidence/freshness remain understandable without color.
- Open/Prepare actions navigate to the exact safe indexed location.

## Verification

- Extend finding filter/classification/action/semantics tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.

