# 66 — Build the selectable unified and side-by-side diff viewer

## Status

Complete

## Goal

Implement Direction C's high-clarity Before/Proposed comparison using deterministic,
testable projections of the existing unified diff.

## Depends on

Task 65.

## Implementation

- Add pure `DiffRow`/`DiffCell` projection helpers for context, additions, removals,
  paired replacements, and old/new line numbers.
- Build selectable read-only Unified and Side-by-side views with explicit Before and
  Proposed labels, scope framing, line numbers, and semantic added/removed descriptions.
- Preserve exact diff text; do not normalize, edit, or reconstruct source beyond display rows.
- Keep large diffs scrollable and avoid loading a second editable text model.
- Replace the current private composed-diff rendering with the shared viewer; remove the
  superseded renderer in the same change.

## Acceptance criteria

- Unified and side-by-side views represent the same `UnifiedDiff` without lost/reordered lines.
- Selection/copy works and no diff surface accepts edits.
- Additions/removals remain understandable without color.
- Empty/unavailable/invalid diff states explain the next action.

## Verification

- Add projection tests for context, insert, delete, replace, and mixed hunks.
- Extend source-preservation and semantics tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
