# 152 — Unify IDE design tokens and control states

## Status

Complete

## Depends on

Task 151, completed and committed.

## Goal

Apply the guideline palette, typography, and interaction hierarchy once in the
shared design system, without screen-specific themes.

## Execution contract

Follow `desktop/UI_REFINEMENT_PLAN.md`, `desktop/UI_DESIGN_GUIDELINES.md`, and
`tasks/PROMPT_EXECUTE_UI_REFINEMENT.md`. Paths in this paragraph are repository-root
relative. This task runs only after explicit execution is requested. Use one
implementation agent, finish and commit this task before starting the next, preserve
user-owned changes, and keep the preview-first workflow intact.

## Likely files

Production/test filenames without a directory are under the corresponding
`desktop/src/main/kotlin/io/miniorca/desktop/` or
`desktop/src/test/kotlin/io/miniorca/desktop/` directory.

`DesktopTheme.kt`, `ChromeControls.kt`, `DesktopIcons.kt`, `DiffViewer.kt`,
affected shared controls/tests, and current contrast/design documentation.

## Implementation

- Map semantic surface roles to the guideline values: workspace #1E1F22,
  navigation #18191B, raised panels #2B2D30, accent #3574F0, muted selection
  #2E436E, and separators #323438.
- Keep text, disabled, success/warning/error, syntax, diff, and focus tokens
  separate. Do not use the accent indiscriminately for text, fills, and borders.
  If an action/text pairing fails contrast, use a documented semantic derivative
  at that role rather than weakening the contrast target.
- Consolidate spacing on the 4dp grid, 11–12sp secondary chrome, explicit readable
  line heights, 1dp separators, and 6dp contained-control corners. Font scaling
  may grow controls; never solve overflow by shrinking text.
- Refine shared buttons, tabs, badges, fields, and quiet chrome for default, hover,
  pressed, selected, focused, and disabled states. Focus must differ from selection.
- Preserve readable monospaced source/diff content, textual state and +/- markers,
  accessible names, hit areas, and the current Mini-Orca mark.
- Update current contrast evidence and shared-control visual fixtures. Adopt one
  token system; later tasks consume it rather than restyling each popup independently.
- Do not introduce Jewel, a custom titlebar, a second theme, or a theme toggle here.

## Required legacy removal

Remove superseded colors, shapes, styling branches, dead density values, and
unused aliases once callers have migrated. Retain genuinely distinct semantic roles;
do not collapse them merely because two current values match.

## Acceptance criteria

- Shared components use the guideline's surface/selection hierarchy; no remaining
  workspace switches to a competing palette or stock light defaults.
- Actual rendered text pairs meet 4.5:1 for normal text and essential focus/control
  cues meet 3:1; check blended backgrounds, not only opaque token values.
- Keyboard focus remains visibly distinct; selected/disabled/error states are
  readable without color alone, including at 130% text scale.
- Syntax and diff styling leave source bytes, selection, and workflow state intact.

## Verification

Extend `DesktopThemeTest`, `DesktopAccessibilityTest`, `DiffViewerTest`, and
shared-control fixture coverage for rendered states, contrast, long labels, and
font scaling. Update current contrast documentation with measured pairs.

For every task, run `./desktop/gradlew -p desktop spotlessCheck detekt test` and
`git diff --check` before committing; checks already included above need not run
twice. Add behavior-focused tests where coverage is missing. Record actual results
below, including visual evidence and native checks deferred to Task 159/160.
Required automated failures block the task subject only to the explicitly stated
repository-baseline exception in Tasks 150 and 160.

## Commit

After acceptance passes, prepare Complete status, move this file to
`tasks/completed/`, and update its index link/status in the same isolated commit.
Follow the prompt's staging/review protocol. Use exactly this subject:

```text
style(desktop): unify IDE design tokens (task 152)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Passed `./desktop/gradlew -p desktop spotlessCheck detekt test -PvisualOutput="$PWD/desktop/build/reports/ui-refinement/task-152"` and `git diff --check` on 2026-09-04. Replaced the superseded charcoal palette, 4dp contained-button shape, fixed-height shared buttons, and accent-as-text usages with one guideline-aligned semantic system; the new `UI_CONTRAST.md` records resolved contrast pairs. Visual review passed for `analysis-1440-1.0.png` and `shared-controls-130.png`, including readable selected, disabled, and focus states without clipping. Native window and screen-reader checks were unavailable because this session has no enabled native Mini-Orca surface; Task 159/160 retain those operator checks.
