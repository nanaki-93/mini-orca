# 64 — Implement the Editor Target stage

## Status

Complete

## Goal

Create one clear surface for choosing and understanding the exact file/declaration
scope before any prompt-bearing request.

## Depends on

Task 63.

## Implementation

- Keep the read-only source canvas central with selected symbol/finding line emphasis.
- Consolidate duplicated file facts, analysis freshness, symbol brief, replace/create
  mode, extracted symbol selection, and new declaration name in the Target context panel.
- Show exact/approximate/atomic target status with textual explanations.
- Keep Analyze/Refresh and remote-provider confirmation behavior where prompt content
  may be sent.
- Preserve prepared suggestion/finding navigation and explicit context selection.
- Delete superseded duplicate editor brief/target UI and call sites in this change.

## Acceptance criteria

- A valid replace/create target unlocks Draft; invalid/approximate input explains why not.
- Source stays selectable and read-only.
- Finding and Summary actions land on the correct file/symbol/line and prepared request.
- File/symbol facts appear once, in the stage where they are needed.

## Verification

- Extend target validation, navigation, source emphasis, and brief tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
