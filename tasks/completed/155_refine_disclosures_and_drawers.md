# 155 — Refine disclosures, drawers, and tool-window controls

## Status

Complete

## Depends on

Task 154, completed and committed.

## Goal

Make expandable sections and collapsible tool windows visually consistent inside
and outside their expanded surfaces.

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

`EngineeringInsightPanel.kt`, `IdeShell.kt`, `DesktopShell.kt`,
`FindingsPresentation.kt`, `ContextToolWindow.kt`, affected dialog/header controls,
and disclosure/drawer/accessibility tests.

## Implementation

- Introduce a narrow reusable disclosure header using the shared tokens and a
  proper vector chevron, title, optional count/state, and clear expanded semantics.
- Replace Engineering insight's text chevrons, repeated advisory labels, and
  redundant internal Close insight button with one coherent toggle header.
  Keep AI interpretation and stale state explicit and all returned prose reachable.
- Preserve the existing useful disclosure preference behavior, focus on close,
  and no content when optional insight is absent. Expand/collapse remains local.
- Migrate filter disclosures and other expandable sections from the baseline
  inventory. Align fields and internal actions without nested card/close-button
  stacks. Do not place actionable descendants inside another clickable button.
- Refine docked headers, Files/Context drawer headers, bottom overlay tabs,
  reopen/collapse controls, and affected dialogs using the same spacing and states.
  Keep distinct modal/non-modal semantics rather than forcing one container type.
- Ensure large content has bounded scroll regions, close/reopen remains reachable,
  and transitions do not lose selected file, draft, evidence, or saved pane sizes.
- Preserve Escape handling for the topmost surface, focus restoration, tab order,
  and exact 999dp/1000dp behavior. Apply warnings and consent must not be hidden
  merely to make a disclosure smaller.

## Required legacy removal

Remove replaced disclosure branches, text-glyph arrows, redundant Close buttons,
unused handlers/imports, and old local surface overrides. Preserve required local
state/preferences; no second implementation for old rendering remains.

## Acceptance criteria

- Every scoped disclosure/tool window has a compact consistent header in both
  states, with aligned inner controls and no redundant nested chrome.
- Complete insight content, freshness, filter state, and selected workflow context
  survive closing/reopening and viewport changes.
- Keyboard/focus/expanded semantics work, and no toggle initiates backend requests,
  applies code, or modifies analysis/candidate authority.
- Long content and larger text do not obscure the dismissal or next workflow action.

## Verification

Cover insight null/stale/expanded states, persistence, keyboard toggle and focus
return; extend shell/accessibility/visual tests for drawers, bottom overlay, filters,
and nested-surface Escape. Assert unchanged callbacks and draft/evidence state.

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
style(desktop): refine disclosures and tool windows (task 155)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Passed `./desktop/gradlew -p desktop spotlessCheck detekt test
-PvisualOutput="$PWD/desktop/build/reports/ui-refinement/task-155"` and `git diff
--check` on 2026-09-04. Replaced the Engineering insight text-glyph toggle and
redundant Close insight control with the shared semantic disclosure header,
preserving its persisted expansion preference, explicit AI/stale state, local-only
behavior, focus restoration, and complete bounded prose. The same header now owns
advanced finding filters; their 520dp threshold stacks fields before 130% text can
overlap content, and the Problems view uses one scroll region so expanded filters
and findings remain reachable. Drawer headers now provide explicit close controls,
and bottom pane/overlay headers share vector-icon open/collapse and title treatment.
Visual review passed for `engineering-insight-collapsed-720-1.3.png`,
`engineering-insight-expanded-720-1.3.png`,
`engineering-insight-narrow-360-1.3.png`,
`findings-filters-expanded-480-1.3.png`, and
`tool-window-controls-360-1.3.png`. The production semantic harness exercises
keyboard toggle/focus return, no-op disclosure/filter interactions, drawer close,
and bottom-overlay dismissal. Detached dialog window rendering is not fully
composited by the offscreen raster; native keyboard/window and screen-reader checks
remain deferred to Tasks 159/160.
