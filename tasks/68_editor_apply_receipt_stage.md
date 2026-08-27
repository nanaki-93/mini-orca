# 68 — Implement the Editor Apply and receipt stage

## Status

Pending

## Goal

Create the calm final decision surface and a clear post-Apply/Undo receipt without a
redundant generic confirmation dialog.

## Depends on

Task 67.

## Implementation

- Build the Apply stage from Direction B's plain-language hierarchy using Direction C colors.
- State “Nothing has changed yet,” show exact scope, validation, checks, draft identity,
  advisory impact, and read-only Git context.
- Use a single explicit target-naming action such as “Apply RegisterRoutes to router.go”.
- Keep Apply disabled with a visible reason unless the exact current draft is eligible.
- Remove the old generic Apply dialog; stage entry plus the explicit target-naming button is
  the deliberate two-step decision.
- After success, replace the decision card with applied revision/hash/audit evidence and
  conflict-safe Undo when available.
- Preserve source/index/findings/freshness refresh after Apply and Undo.
- Delete the remainder of the superseded draft review pane.

## Acceptance criteria

- No call to Apply is reachable from Target, Draft, or Verify.
- The Apply action mutates exactly the named bound file and is never automatic.
- Dirty/invalid/unchecked/stale/mismatched drafts remain blocked with an explanation.
- Receipt and Undo use the returned guarded identity and cannot reuse stale evidence.

## Verification

- Extend apply-copy, eligibility, fake-transport Apply/Undo, refresh, and conflict tests.
- Run `./desktop/gradlew -p desktop test`, `make check`, and `git diff --check`.

