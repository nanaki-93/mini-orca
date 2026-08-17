# 01 — Define the focused workflow contract

## Goal

Make the product promise explicit: one active project, one file, one selected symbol, one reviewed candidate.

## Depends on

None.

## Implementation

- Decide and document that v4.2 is a single-coder preview/apply workflow; tester and reviewer remain optional focused checks, not an automatic multi-agent pipeline.
- Define action modes: `analyze_file`, `explain_symbol`, `fix`, `refactor`, `document`, and `generate_test`.
- Define scope modes: `strict_symbol` and `symbol_plus_imports`; only the latter permits minimal import edits.
- Specify when an action is read-only, candidate-producing, or mutating.
- Record non-goals: autonomous multi-file edits, auto-commits, and silent writes.

## Acceptance criteria

- README, API contract, UI labels, and generated prompts use the same terms.
- Every candidate-producing action requires a target file and target symbol.
- Generate-test targets one symbol in an already selected test file.

## Verification

- Add a small contract test or fixtures for valid/invalid action and scope combinations.
