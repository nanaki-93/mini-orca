# 164 — Layer the shell and replace heavy pane separation

## Status

Complete

## Depends on

Task 163, including its verified local commit.

## Goal

Give the activity rail, tool windows, and editor immediately distinguishable surfaces and precise continuous boundaries.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`IdeShell.kt`, `DesktopShell.kt`, `DesktopHeader.kt`, `DesktopStatusBar.kt`, `DesktopLayoutState.kt` only if required; `DesktopShellTest.kt`, `DesktopLayoutStateTest.kt`, `DesktopVisualLayoutTest.kt`.

## Implementation

1. Apply #18191B to activity/outer chrome, #1E1F22 to tool windows, and #2B2D30 to the editor/content canvas via semantic roles. Ensure the editor/gutter do not sit inside a card.
2. Replace pane gaps, decorative rounded wrappers, and nested borders with a single 1dp divider at each owned boundary. Carry lines cleanly through header/content intersections without double strokes.
3. Keep existing resizing and temporary pane clamping. Preserve a wider invisible splitter hit region, hover cursor, accessible label, keyboard resizing, and stored dimensions; do not turn splitters into a 1dp-only interaction target.
4. Use shared flat headers for docked/drawer containers. Preserve reopen/collapse entry points and focus restoration.
5. Retain the exact 1000dp breakpoint, 360dp docked editor target, rail destinations, and state transitions when leaving Editor. Narrow layouts may overlay drawers, not compress every pane into an unusable column.

## Acceptance criteria

- [x] Wide Editor captures clearly show three surface levels with thin continuous boundaries and no heavy permanent gutters or whole-pane rounded containers.
- [x] Each boundary has one divider owner. Resize targets remain practical with pointer and keyboard; resizing does not overwrite stored preferences during temporary clamping.
- [x] Layouts at 1000dp and 999dp select the correct docked/drawer mode, with no lost selected file, symbol, draft, or hidden reopen control.
- [x] Surface and splitter changes are present in production shell scenes, not only a control showcase.

## Verification

Extend shell/layout tests for boundary widths, resizing, focus, and minimum editor space; render wide, breakpoint, and narrow scenes. Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
feat(desktop): separate IDE panes with layered surfaces
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Completed 2026-09-05.

- Reassigned the activity rail, docked tool windows, editor canvas, and restrained chrome to
  their semantic surfaces. Docked panes now meet through one owned separator; the rail has a
  single boundary separator, while each 8dp splitter owns its centered 1dp line.
- Kept render-time pane clamping separate from stored preferences. Added the rail separator to
  the width calculation while retaining the 360dp editor floor at the 1000dp breakpoint.
  Splitters now expose resize cursor, accessible name, focus, and arrow-key resizing.
- Updated the Editor visual fixture to compose the production shell primitives. Inspected
  component-rendered captures at 1440dp, 1000dp, and 999dp:
  `desktop/build/reports/ui-precision/task-164/editor-1440.png`,
  `desktop/build/reports/ui-precision/task-164/editor-1000.png`, and
  `desktop/build/reports/ui-precision/task-164/editor-999.png`. The 999dp scene retains the
  selected source and explicit Files/Context reopen actions; native-window evidence remains
  owned by Task 170.
- Passed focused shell/layout/visual tests under JBR 25. Passed
  `./desktop/gradlew -p desktop spotlessCheck detekt test` using JDK 21 to launch Gradle with
  the JBR 25 toolchain path, and passed JBR 25
  `./desktop/gradlew -p desktop packageDistributionForCurrentOS`. JBR 25 directly cannot run
  the current Detekt task (`25.0.4`); this is the established Detekt launcher limitation, not a
  production runtime failure. Skiko restricted-native-access and Jewel `Unsafe` warnings were
  non-fatal and unchanged.
- Ran `git diff --check`. No configuration or migration action is required.
