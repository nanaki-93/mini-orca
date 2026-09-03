# 130 — Harden responsive and accessible IDE navigation

## Status

Complete

## Goal

Make every new shell region usable below `1000dp`, at supported text scaling, and
through a complete keyboard/focus path.

## Depends on

Task 129.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 130` and name the
  responsive modes, drawer/overlay behavior, focus traversal, scaling cases, likely
  files, and checks.
- After the commit, post a separate update beginning with `Task 130 complete` and
  report responsive/accessibility behavior, manual limitations, tests/results, exact
  commit hash, and that Task 131 is next.

## Implementation

- Apply one explicit layout policy: docked left/right/bottom regions at and above
  `1000dp`; Project and right-side tool windows as modal drawers below it; bottom tools
  as a compact summary that expands into a bounded overlay/drawer.
- Preserve editor priority and close incompatible drawers when leaving Editor without
  losing the selected file, declaration, draft, or evidence.
- Define deterministic toolbar overflow and breadcrumb truncation for narrow widths
  and supported text scaling.
- Complete focus traversal across toolbar, tool-window bar, Project tree, editor tabs,
  source, right tabs, bottom tabs, and status items.
- Add arrow-key navigation within trees/tab groups, visible cyan focus treatment, and
  focus restoration after dialogs/drawers.
- Ensure actions/states retain text or accessible names and never depend only on color,
  hover, animation, or an unlabeled glyph.
- Keep pointer hit targets usable while retaining compact visual dimensions.
- Prefer short functional transitions and avoid large motion when reduced motion is
  requested or can be inferred from platform settings.

## Acceptance criteria

- Exactly `1000dp` uses the stable wide layout; widths below it use drawers/overlay.
- No essential action or state clips in the documented viewport and scaling matrix.
- Every workflow is keyboard reachable and `Escape` closes only the topmost transient
  surface or active cancellable operation.
- Focus returns predictably and never triggers an action merely by moving.
- Screen-reader semantics identify tool windows, active tabs, selected file/symbol,
  workflow states, and source/diff read-only status.

## Verification

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Complete the responsive, keyboard, text-scaling, and semantics sections of
`desktop/KEYBOARD_SMOKE_CHECKLIST.md` where interactive access is available; record
unavailable checks truthfully.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 130 changes, inspect the staged diff, and create
exactly one commit:

```text
feat(desktop): harden responsive IDE navigation
```

Do not amend, squash, tag, or push the commit.
