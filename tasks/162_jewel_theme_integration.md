# 162 — Integrate Jewel and the semantic IDE theme

## Status

Complete

## Depends on

Task 161, including its verified local commit.

## Goal

Install one verified desktop component foundation and expose distinct surface roles without rewriting workflow state.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`desktop/build.gradle.kts`, `desktop/settings.gradle.kts`, `desktop/gradle/wrapper/`, `Main.kt`, `DesktopApp.kt`, `DesktopTheme.kt`; `DesktopThemeTest.kt`, rendering test adapter; `desktop/README.md`, `desktop/UI_COMPONENT_DECISION.md`, `desktop/UI_CONTRAST.md`.

## Implementation

1. Apply exactly the dependency/runtime matrix proved in Task 161. Keep the JVM baseline unless the selected supported pairing requires a change; document that change. Use the Gradle wrapper task for any wrapper upgrade.
2. Configure application launch, tests, and distributable runtime consistently for JBR. Avoid machine-specific absolute JDK paths, global environment mutation, or system installation; document reproducible local setup and any operator prerequisite.
3. Wrap production UI in Jewel's standalone theme and map activityRail, toolWindow, editorCanvas, paneSeparator, overlay, text, selection, focus, and semantic evidence roles centrally. Start with the plan's specified colors.
4. Introduce the 12–13sp body/18–20sp line-height and 11–12sp section typography roles. Preserve separate source/diff and editable-draft typography; avoid scattering Material typography references through new controls.
5. Keep only the minimal single-token bridge needed for unmigrated call sites through Task 169. Inventory those call sites explicitly; do not preserve an alternative theme toggle or legacy shell.
6. Adapt test harness APIs to the aligned Compose version without losing semantics or pretending detached windows are composited. Verify theme rendering and native distribution creation for the host.

## Acceptance criteria

- [x] The actual app and tests use the pinned standalone Jewel dependency and selected runtime; compilation is not satisfied only by an unused library.
- [x] Rail, tool-window, canvas, and overlay roles are distinct, documented, and tested. Meaningful text meets contrast targets on its actual destination surfaces.
- [x] All existing tests pass on the supported pairing; app initialization and host packaging work, or the task remains incomplete with a precise blocker.
- [x] No default Material/Swing appearance or duplicate palette is introduced. Source, diff, provider consent, and Apply/Undo domain behavior are unchanged.

## Verification

Run theme and workflow tests while iterating, then `./desktop/gradlew -p desktop spotlessCheck detekt test createDistributable` and `git diff --check`. Use a disposable fixture for a launch smoke check; distinguish process startup from native visual verification. Do not manually edit distribution output.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
feat(desktop): adopt Jewel and semantic IDE theme
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Started after verified Task 161 commit
`8e6f691face88366a08d7e5deaf5b48e54dbbd83`.

- Added the exact verified production dependency
  `org.jetbrains.jewel:jewel-int-ui-standalone:0.40.0-262.10315.125`. `MiniOrcaTheme`
  now wraps every production window and the rendering test fixture in Jewel
  `IntUiTheme(isDark = true)`, with one centrally mapped Material bridge for
  controls that later tasks own.
- Replaced the old overlapping palette names with central activity-rail
  (`#18191B`), tool-window (`#1E1F22`), editor-canvas (`#2B2D30`), overlay
  (`#26282C`), and pane-separator (`#323438`) roles. Added 13sp/20sp body,
  12sp/18sp compact, and 11sp/16sp section typography roles. Source/diff and
  editable-draft typography and all workflow behavior are unchanged.
- `DesktopThemeTest` now verifies semantic role distinction, dense typography,
  and meaningful-text/focus contrast on tool-window, canvas, and overlay
  destinations. Reviewed full production-component renders are ignored output in
  `desktop/build/reports/ui-precision/task-162/`; they remain offscreen evidence,
  not native screenshots.
- The first `createDistributable` launch smoke exposed omitted modules in Compose's
  trimmed runtime: Jewel required `jdk.unsupported` for `sun.misc.Unsafe`, then
  Mini-Orca's daemon transport required `java.net.http`. Declaring both modules
  in `nativeDistributions` produced a final macOS arm64 bundle with JBR 25.0.4,
  all Jewel jars, and a clean six-second isolated launch smoke. No app data was
  used; each smoke run had a disposable `user.home`.
- `spotlessCheck detekt test createDistributable` passed with the JDK 21 launcher
  and JBR 25 toolchain. The final distribution was then rebuilt directly with
  JBR 25 (the packager derives its image from the launcher JVM) and smoke-tested.
  Upstream JDK-25 restricted-native-access notices from Skiko/Jewel remain
  observed but non-fatal. Runtime setup, module rationale, artifact evidence,
  and the native-verification boundary are documented in `desktop/README.md`,
  `desktop/UI_COMPONENT_DECISION.md`, and `desktop/UI_CONTRAST.md`.

The final `git diff --check` passed before staging this task's reviewed files.
