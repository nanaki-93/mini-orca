# 131 — Unify workspace density and visual states

## Status

Pending

## Goal

Bring Summary, Analysis, Bugs, empty/error states, and routine controls into the same
compact visual language as the new IDE shell while retaining the Focus Flow palette.

## Depends on

Task 130.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 131` and name the
  workspace presentations, token reuse, visual states, likely files, and checks.
- After the commit, post a separate update beginning with `Task 131 complete` and
  report the density changes, palette preservation, tests/results, exact commit hash,
  and that Task 132 is next.

## Implementation

- Restyle Summary as dense key/value sections with restrained raised cards reserved
  for important interpretation, error, and empty states.
- Restyle Analysis controls as a compact toolbar/form and present progress/failures in
  readable rows without changing job semantics.
- Restyle the detailed Bugs workspace using the shared Problems presentation, compact
  rows, filters, and a details region instead of duplicated large cards.
- Standardize headers, rows, fields, badges, panels, empty/loading/error/stale states,
  receipts, hover, pressed, selected, disabled, and focus treatment using shared theme
  tokens.
- Preserve every existing palette value and code-token color; do not add a second
  theme or copy JetBrains assets.
- Remove unnecessary nested cards and repeated all-caps headings where hierarchy is
  already supplied by a tool-window or editor header.
- Keep visible labels for dominant, navigation, positive, attention, destructive, and
  neutral actions.
- Delete obsolete visual helpers and parallel workspace presentations after migration.

## Acceptance criteria

- All retained screens share the same density, spacing, typography, focus, and state
  language.
- The existing palette values are unchanged and remain the recognizable product theme.
- Summary, Analysis, and Bugs behavior, filters, actions, and advisory/verified labels
  are unchanged in meaning.
- Long content scrolls or truncates with full accessible text; essential controls do
  not clip.
- No duplicate findings, validation, or workflow policy is introduced for appearance.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Repeat the documented screenshot matrix when interactive Desktop access is available
and compare hierarchy, density, clipping, focus, and state labels to the baseline.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 131 changes, inspect the staged diff, and create
exactly one commit:

```text
style(desktop): unify IDE workspace presentation
```

Do not amend, squash, tag, or push the commit.
