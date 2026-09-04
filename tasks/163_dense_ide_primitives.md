# 163 — Build shared dense headers, toolbars, and dividers

## Status

Complete

## Depends on

Task 162, including its verified local commit.

## Goal

Make the three visual priorities reusable and consistent before migrating individual panes.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`ChromeControls.kt`, `DesktopTheme.kt`, `DesktopIcons.kt`, `DesktopAccessibility.kt`; focused shared-control tests and `DesktopVisualLayoutTest.kt`. Add a small single-purpose component file only if the existing boundaries need it.

## Implementation

1. Replace shared button/tab/disclosure implementations with the verified Jewel equivalents or narrow Jewel-styled wrappers. Preserve callback signatures when practical; remove replaced implementations in the same task.
2. Provide one flat header composition with optional title/icon/state/disclosure, trailing actions, and collapse/overflow slots. Standardize 28–32dp minimum header/action targets, 16dp action icons, and 4–8dp gaps.
3. Implement compact transparent toolbar actions, tooltips, accessible names, enabled/disabled states, hover/pressed treatment, and separately visible keyboard focus. Ambiguous actions may retain short text.
4. Separate disclosure activation from trailing actions: an action click must not expand/collapse the section. Expose expansion semantics, keyboard activation, and stable focus behavior.
5. Provide shared horizontal/vertical 1dp separator styling and zero-elevation, square pane structure. Keep the visual line independent of the wider splitter interaction target.
6. Apply shared typography roles and active-tab underline styling. Permit rows to grow at larger text scale rather than forcing a clipped fixed height.

## Acceptance criteria

- [x] A rendered control fixture demonstrates default, hover, pressed, selected, disabled, focused, and expanded/collapsed states without rounded section-card chrome.
- [x] Keyboard users can activate named actions and disclosures independently; disabled controls dispatch zero callbacks.
- [x] One implementation owns each migrated primitive; remaining screens receive the same tokens without copied per-screen styles.
- [x] Targets fit the default density table and remain legible/unclipped at 150% text scale. Selected tabs and focus are visually different.

## Verification

Add behavior-focused control tests and production-component visual fixtures. Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`; inspect fixture images, not just test exit codes.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
refactor(desktop): unify dense IDE chrome primitives
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Implemented shared Jewel-themed control policy through `IdeActionSurface`: flat transparent chrome
actions, contained workflow buttons, selected tabs, and disclosures now use one interaction, focus,
disabled-state, and typography implementation. `IdePaneHeader` provides the common flat header with
optional icon/state/disclosure plus trailing action, overflow, and collapse slots. The docked tool-window
header and workspace headers use it. Shared 1dp horizontal and vertical separator components now render
inside the existing wider resizable splitter targets.

`ChromeTab` now uses a 32dp target, an active fill, and a 2dp bottom accent underline. Headers and
actions use 4dp gaps and 16dp icons; header content may grow to two lines rather than clipping at
enlarged text scale. Tooltip and accessible-name support is supplied by the shared control; icon-only
disclosure and close controls provide names. Existing callbacks and preview-first workflow rules remain
unchanged.

Added `ChromeControlsTest` for visual-state precedence and production-component tests for independent
keyboard action/disclosure activation, disabled callback suppression, focus restoration, and 150% text.
The rendered state fixture was inspected at:

- `desktop/build/reports/ui-precision/task-163/shared-chrome-states-150.png`
- `desktop/build/reports/ui-precision/task-163/shared-chrome-actions-150.png`
- `desktop/build/reports/ui-precision/task-163/shared-chrome-actions-expanded-150.png`

Verification passed with the Task 161 JDK 21 launcher and Task 162 JBR 25 toolchain:

```text
./desktop/gradlew -p desktop test --tests io.miniorca.desktop.ChromeControlsTest \
  --tests io.miniorca.desktop.DesktopVisualLayoutTest.sharedChromeKeepsNamedActionsAndDisclosureActivationIndependent \
  --tests io.miniorca.desktop.DesktopVisualLayoutTest.sharedChromeFixtureShowsInteractionStatesAndKeepsTextLegibleAtOneHundredFiftyPercent \
  -Porg.gradle.java.installations.paths=<JBR 25> \
  -PvisualOutput=desktop/build/reports/ui-precision/task-163

./desktop/gradlew -p desktop spotlessCheck detekt test \
  -Porg.gradle.java.installations.paths=<JBR 25> \
  -PvisualOutput=desktop/build/reports/ui-precision/task-163

git diff --check
```

The first full run exposed one stale `PreviewFeatureTest` expectation for the intentionally reduced
menu row target (36dp to 32dp); the behavior-focused expectation was updated and the full suite then
passed. JBR emitted its known Skiko and Jewel `Unsafe` warnings; no runtime/package configuration changed.
