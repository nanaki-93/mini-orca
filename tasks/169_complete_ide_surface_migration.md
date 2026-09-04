# 169 — Finish bottom panes, global controls, and migration cleanup

## Status

Complete

## Depends on

Task 168, including its verified local commit.

## Goal

Eliminate inconsistent chrome outside the main workspaces and remove the temporary migration bridge.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`ProblemsToolWindow.kt`, `BottomEvidenceToolWindows.kt`, `DesktopHeader.kt`, `DesktopStatusBar.kt`, `CommandPalette.kt`, `PreviewFeature.kt`, `DesktopApp.kt`, `DesktopTheme.kt`, `ChromeControls.kt`, build dependencies; related test suites and visual fixtures.

## Implementation

1. Use flat bottom-pane headers/tabs with inline filter/count/action rows, thin top boundaries, and dense Problems/Checks/Output content. Remove padded output panels that merely duplicate pane structure.
2. Preserve full diagnostics, check freshness, advisory labels, selectable output, and navigation to the real selected finding. Keep Terminal explicitly Preview and inert.
3. Migrate remaining global project/Preview menus, command palette, landing controls, consent/discard dialogs, and status controls to the same Jewel theme and density rules. Real dialogs may remain bounded surfaces.
4. Verify menu overflow, long labels, disabled reasons, tooltips, keyboard traversal, Escape/outside dismissal, and focus return. Avoid experimental native popup modes unless separately justified and verified.
5. Audit all desktop imports/call sites for obsolete Material controls, card wrappers, duplicated token values, and temporary bridge paths. Remove dead dependencies/theme adapters once their last caller migrates.
6. Document any strictly necessary low-level interop primitive with a concrete reason and tests; it must consume the single active design system and cannot retain a parallel legacy UI.

## Acceptance criteria

- [x] Bottom and global surfaces match the new pane hierarchy and header-action rules, including empty/loading/error and Preview states.
- [x] Standard controls use Jewel; the obsolete Material theme and temporary migration bridge are removed, with only explicitly styled platform interaction primitives remaining.
- [x] Menus/dialogs retain functional semantics and callback isolation; offscreen component coverage is retained without treating it as proof of native popup placement.
- [x] No copied palette, obsolete card-based replacement, dead helper, or forgotten workspace remains in the component inventory.

## Verification

Run Problems/bottom output, command palette, Preview, status, menu, consent, and dismissal tests. Inspect the remaining-surface fixture matrix. Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
refactor(desktop): complete Jewel surface migration
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

- Flattened Problems, Checks, and Output into dense pane sections with 8dp insets, shared headers, and 1dp separators. Diagnostics, check freshness, selectable command output, and real finding navigation remain unchanged; Preview controls remain local and inert.
- Replaced the remaining Material dialog and root-surface bridge with shared IDE controls. `MiniOrcaTheme` now applies the standalone Jewel theme directly; the retired Material theme colors, typography, shapes, surfaces, alerts, checkbox, and progress controls have no production callers.
- Retained only two low-level Compose Desktop interaction primitives: `ModalDrawer` owns responsive drawer gestures/dismissal while taking explicit semantic palette colors, and `DropdownMenu` owns popup placement/dismissal/keyboard handling while its surface and rows use shared IDE controls. Bounded `Dialog` uses the standard Compose scene-layer dialog path; Task 170 owns native placement evidence.
- Passed focused Problems, bottom output, command palette, Preview, status, shell, and theme tests with `env JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.BottomEvidenceToolWindowsTest' --tests 'io.miniorca.desktop.ProblemsToolWindowTest' --tests 'io.miniorca.desktop.CommandPaletteTest' --tests 'io.miniorca.desktop.PreviewFeatureTest' --tests 'io.miniorca.desktop.DesktopStatusBarTest' --tests 'io.miniorca.desktop.DesktopShellTest' --tests 'io.miniorca.desktop.DesktopThemeTest' -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home`.
- Passed `env JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop spotlessCheck detekt test -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home`; detekt analyzed 41 Kotlin files with zero code smells.
- Passed `env JAVA_HOME=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home ./desktop/gradlew -p desktop test --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' -PvisualOutput=/Users/marcoandreose/DEV/lab/mini-orca/desktop/build/reports/ui-precision/task-169 -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home`; inspected the Checks failure, Output running/error, and remote-consent renders in `desktop/build/reports/ui-precision/task-169/`.
- The JBR emitted its known restricted-native-access and Jewel `Unsafe` deprecation warnings during visual tests. No runtime or configuration changes were made.
