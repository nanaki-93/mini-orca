# 115 — Simplify Compose and API client contracts

## Status

Pending

## Goal

Remove Compose parameter explosion, unused UI contracts, and weakly typed request
construction while preserving the approved Desktop interaction and visual behavior.

## Depends on

Task 114.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 115` and name the large
  composables, typed models/actions, API request DTOs, accessibility checks, and
  pre-existing changes.
- After the commit, post a separate update beginning with `Task 115 complete` and
  report the simplified contracts, removed unused code, commands/results, exact commit
  hash, and that Task 116 is next.

## Implementation

- Split `DesktopShell` and oversized panes along stable visual/feature boundaries; do
  not create a separate composable for every trivial element.
- Replace long primitive/callback parameter lists with small immutable feature models
  and named action groups. Avoid one catch-all state object or global callback bag.
- Target roughly 6–8 parameters per composable and keep exceptions only where Compose
  slot APIs are clearer.
- Ensure composables render already-derived state and emit named intents; they must not
  assemble project/file/draft identity or make API decisions.
- Remove unused `onCancelAll`, unused theme helpers such as `badgeColor`, unreachable
  states, and tests that only preserve removed contracts.
- Replace `jsonBody(vararg Any?)`, unchecked list casting, and ad hoc request maps with
  serializable request DTOs and one serialization path.
- Keep one concrete `ApiClient` unless a split demonstrably reduces coupling. Group
  retained methods by feature and delete any method with no production caller.
- Centralize repeated success/error decoding while preserving server status and error details.
- Preserve text labels for state, keyboard focus/traversal, screen-reader semantics,
  selectable read-only source/diff, enabled/disabled action rules, and responsive
  drawer behavior below 1000dp.
- Do not redesign the approved UI, change daemon behavior, or add a UI framework.

## Acceptance criteria

- `DesktopShell` and feature panes have cohesive, reviewable contracts with no unused callback.
- No unchecked or vararg JSON request construction remains.
- Every production `ApiClient` method has a caller and a registered daemon route.
- Accessibility, keyboard, compact layout, source/diff read-only behavior, draft edit,
  validation, checks, Apply, and Undo tests pass.
- UI extraction does not duplicate presenter state or workflow rules.

## Verification

Run:

```text
./desktop/gradlew -p desktop test
make check
git diff --check
```

Run any configured formatting/static task already present. Inspect parameter counts,
unchecked casts, unused callbacks, and broad exception catches in production Kotlin.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 115 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(desktop): simplify UI and API contracts
```

Do not amend, squash, tag, or push the commit.
