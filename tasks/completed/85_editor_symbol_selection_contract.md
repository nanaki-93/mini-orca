# 85 — Define editor symbol selection and inspection contracts

## Status

Complete

## Goal

Add deterministic, presentation-only contracts for selecting an indexed declaration from
a source line, inspecting that declaration, and deciding whether it may become an edit
target before Compose interaction changes begin.

## Depends on

Task 84.

## Required task commentary

- Before editing, post a concise update beginning with `Starting Task 85` and name the
  selection helpers, derived inspector/edit state, likely tests, and any pre-existing
  changes that must be preserved.
- After verification, post a separate update beginning with `Task 85 complete` and state
  the contracts added, tests run, and that Task 86 is next.

## Implementation

- Inspect the current worktree before editing and preserve every pre-existing change as
  user-owned.
- Add a pure `symbolAtLine`-style helper that:
  - ignores invalid/non-containing ranges;
  - selects the narrowest containing declaration;
  - breaks ties in favor of exact atomic targets, then source order;
  - returns no symbol for an out-of-declaration line.
- Add a pure source-selection projection containing the clicked line and optional symbol.
- Update selection reduction only as needed so an editor line click can set the exact
  focused line and can explicitly clear a previously selected symbol.
- Add a small `SymbolInspectorUiState` (or equivalent) derived from selected file,
  analysis, symbols, selected symbol, provider state, and current edit identity. It must
  express:
  - file fallback versus selected-symbol state;
  - explanation/signature/range text;
  - missing/stale/running/failed/fresh analysis action;
  - exact atomic Go edit eligibility and a concise blocked reason;
  - whether another current draft is bound to a different target.
- Add a derived Inspect/Edit/Review/Receipt progress contract using existing
  chat/draft/validation/check/apply truth. Do not change the visible UI in this task.
- Keep all revision/hash/request identity checks unchanged. Do not add persisted UI state
  or a daemon/API contract.

## Acceptance criteria

- Selection is deterministic for disjoint, nested, overlapping, invalid, and empty
  symbol ranges.
- Clicking outside a declaration can clear stale symbol context without losing the
  clicked line.
- The inspector contract distinguishes every analysis lifecycle state with at most one
  contextual action.
- Only existing exact atomic Go functions/types are eligible for Replace editing.
- Inspection state and current draft identity are distinct; inspecting another symbol
  cannot retarget a draft.
- Existing command-palette/finding navigation behavior and stale-response guards remain
  unchanged.

## Verification

- Extend `DesktopStateTest`, `EditorBriefTest` or their focused replacements, and
  `EditorFlowStateTest` for the new pure contracts.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update `tasks/INDEX.md`. Do not stage or commit; this task does not authorize either.
