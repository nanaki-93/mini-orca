# UI precision baseline

**Recorded:** 2026-09-04
**Scope:** Task 161 captures the pre-migration desktop evidence and validates the
Jewel foundation. It is not native-window acceptance and it does not migrate a
production UI component.

## Evidence boundary

The checked-in visual harness renders production composables with fixture-only
data into an offscreen Compose/Skia scene. It is useful for deterministic layout,
text, semantics, focus, and callback assertions, but it is not evidence of native
window composition, operating-system keyboard routing, screen-reader output, or
detached menu/dialog placement. Those remain Task 170 release checks.

The baseline command wrote ignored artifacts to
`desktop/build/reports/ui-precision/before/`; the post-toolchain run wrote the
same matrix to `desktop/build/reports/ui-precision/after/`. Reviewed representative
production-component renders include:

| Area | Fixture/state and evidence |
| --- | --- |
| Editor | `editor-1440.png`, `editor-1000.png`, `editor-999.png`, and `editor-candidate-800-1.3.png` |
| Analysis | `analysis-1440-1.0.png`, `analysis-1920-1.0.png`, `analysis-1000-1.0.png`, `analysis-999-1.0.png`, `analysis-800-1.0.png`, and empty/paused/failed/long-destination cases |
| Summary | `summary-dashboard-1440.png`, `summary-dashboard-800-1.3.png`, empty and expanded-detail/purpose cases |
| Performance | `performance-populated-1440.png` and `performance-empty-800-1.3.png` |
| Context and Review | `context-consent-800-1.3.png`, `review-failed-800-1.3.png`, `review-receipt-800-1.3.png`, and `assistant-stale-800-1.3.png` |
| Problems and menus | `problems-empty-800-1.3.png`, `bugs-failed-800-1.3.png`, `popup-surface-320-1.3.png`, `project-menu-open.png`, and `preview-menu-open.png` |

The matrix uses the fixture viewports encoded in its names: `1440x900`,
`1920x1080`, exactly `1000x760`, `999x760`, `800x650`, and the affected views at
1.3 text scale. It also covers the 120dp rail, 360/480dp narrow overlays, long
text, empty, paused, failed, stale, receipt, disclosure, and disabled states.
The `jewel-compatibility-spike/jewel-compatibility-spike.png` output was reviewed
while the temporary spike existed; it showed the real Jewel tab, tree disclosure,
text field, disabled action, and popup menu surface. It is ignored build output,
not a native screenshot.

## Current surface and density inventory

The pre-migration palette deliberately shares `appBackground` and `surface`
(`#1E1F22`), uses `chromeSurface` `#18191B`, `raisedSurface` `#2B2D30`, and
`border`/`strongSurface` `#323438`. Shared spacing is 4/8/12/16dp; shared shapes
are 6/6/8dp; default body text is 14sp/20sp, body2 12sp/16sp, caption 11sp/16sp,
and chrome controls have a 32dp minimum. `MiniOrcaPanel` still combines a 1dp
border with a rounded panel/card surface. These are source measurements and
visual-harness inputs, not native visual evidence.

| Ownership area | Current production components | Task owning replacement |
| --- | --- | --- |
| Theme and control bridge | `DesktopTheme.kt`, `DesktopApp.kt`, `ChromeControls.kt`, `DesktopIcons.kt`, `PreviewFeature.kt`, `CommandPalette.kt` | 162 then 169 |
| Rail, shell, docked panes, headers, bottom region | `IdeShell.kt`, `DesktopShell.kt`, `DesktopHeader.kt`, `DesktopStatusBar.kt` | 163–164, then 169 |
| Analysis and Bugs | `WorkspacePanes.kt`, `FindingsPresentation.kt`, `ProblemsToolWindow.kt` | 165 and 169 |
| Summary and Performance | `ProjectSummaryPane.kt`, `PerformanceWorkspace.kt`, `EngineeringInsightPanel.kt` | 166 |
| Files, editor, source/diff, breadcrumbs/tabs | `ExplorerPane.kt`, `EditorWorkspace.kt`, `SourceEditorPane.kt`, `DiffViewer.kt` | 167 |
| Context, Assistant, Review, workflow evidence | `ContextToolWindow.kt`, `AssistantToolWindow.kt`, `ReviewEvidencePane.kt`, `WorkflowToolWindows.kt` | 168 |
| Checks, Output, popups, dialogs, remaining Material controls | `BottomEvidenceToolWindows.kt`, `ChromeControls.kt`, `DesktopShell.kt`, `IdeShell.kt` | 169 |

Every current `androidx.compose.material` import is assigned above: Task 162 owns
`DesktopTheme.kt` and `DesktopApp.kt`; Task 163 owns `ChromeControls.kt`,
`DesktopHeader.kt`, `DesktopStatusBar.kt`, and `DesktopIcons.kt`; Task 164 owns
`IdeShell.kt` and `DesktopShell.kt`; Task 165 owns `WorkspacePanes.kt`,
`FindingsPresentation.kt`, and `ProblemsToolWindow.kt`; Task 166 owns
`ProjectSummaryPane.kt`, `PerformanceWorkspace.kt`, and
`EngineeringInsightPanel.kt`; Task 167 owns `ExplorerPane.kt`,
`EditorWorkspace.kt`, `SourceEditorPane.kt`, and `DiffViewer.kt`; Task 168 owns
`ContextToolWindow.kt`, `AssistantToolWindow.kt`, `ReviewEvidencePane.kt`, and
`WorkflowToolWindows.kt`; Task 169 owns `BottomEvidenceToolWindows.kt`,
`CommandPalette.kt`, and `PreviewFeature.kt`. They are the supported bridge during
the staged migration, not Jewel adoption by themselves. Task 169 removes the
obsolete Material control/theme layer unless a tested low-level interoperability
exception is explicitly documented.

## Published Jewel combination

The selected implementation path is standalone Jewel adoption:

| Layer | Selected version/evidence |
| --- | --- |
| Jewel | `org.jetbrains.jewel:jewel-int-ui-standalone:0.40.0-262.10315.125`, a released Maven Central artifact; its [published POM](https://repo1.maven.org/maven2/org/jetbrains/jewel/jewel-int-ui-standalone/0.40.0-262.10315.125/jewel-int-ui-standalone-0.40.0-262.10315.125.pom) identifies JetBrains, `jewel-ui` at the same version, JNA 5.17.0, and IntelliJ icon artifacts with build `262.10315.125`. |
| Jewel compatibility | The [official Jewel README](https://github.com/JetBrains/intellij-community/blob/master/platform/jewel/README.md) documents the standalone coordinate and JBR requirement. The [release matrix](https://github.com/JetBrains/intellij-community/blob/master/platform/jewel/RELEASE%20NOTES.md) records Jewel 0.40 with Compose Multiplatform 1.11.0 and the JDK 25/Kotlin 2.3.20 generation. |
| Build plugins | Kotlin JVM/serialization/Compose compiler plugin 2.3.20 and Compose Multiplatform 1.11.0. Gradle 9.1.0 is required for Java 25 toolchain support by the [Gradle compatibility matrix](https://docs.gradle.org/current/userguide/compatibility.html). |
| Render runtime | Compose resolves Skiko `0.144.6` for the tested macOS arm64 runtime. The temporary Jewel test runtime also resolved `jewel-ui` and `jewel-foundation` at `0.40.0-262.10315.125`, plus JNA 5.17.0. The IntelliJ icon artifacts select Kotlin stdlib 2.4.0 at runtime; that transitive resolution is recorded rather than hidden. |
| JBR | JetBrains Runtime `25.0.4+1-b508.27` (JBR SDK archive `jbrsdk-25.0.4-osx-aarch64-b508.27`) verified with SHA-512 `1d42308d0afea8d5be2ad166bc43d1a68ef2dcda2e925444c341df6b462118c6b0361c3201301dc1ed7798367ca21cca859c56dfbdb97ff2d577986ef5eef100`. The [JetBrains Runtime release](https://github.com/JetBrains/JetBrainsRuntime/releases/tag/jbr-release-25.0.4b508.27) publishes current macOS, Linux, and Windows x64/aarch64 archives. |

Jewel is Apache-2.0 according to its POM. Package assembly must bundle the matching
JBR distribution only in accordance with the JetBrains Runtime distribution terms;
Task 171 validates the actual package and records its platform artifact. No JBR
archive, machine-specific path, or credential is committed here.

The temporary, test-only spike added this exact dependency, rendered `IntUiTheme`,
`DefaultButton` (including focus and disabled semantics), `TabStrip`, `LazyTree`,
`PopupMenu`, and input-state `TextField`, then removed the dependency and test
source. It passed under JBR 25. The JBR 25 run emitted upstream restricted-native-
access notices from Skiko and Jewel's `UnsafeAccessing`; these did not fail the
test and are not suppressed. The existing offscreen adapter cannot prove Escape
routing to a detached popup after the Compose 1.11 upgrade, so its test now asserts
semantic dismissal and focus restoration; native Escape behavior remains Task 170.

## Reproducible local setup and results

Jewel must launch and package with JBR 25. For ordinary desktop compilation and
tests, point `JAVA_HOME` at an unpacked matching JBR SDK/JDK home:

```text
JAVA_HOME=/path/to/jbrsdk-25.0.4-<platform>-b508.27/Contents/Home \
  ./desktop/gradlew -p desktop test
```

Stable Detekt 1.23.8 does not parse a JDK 25 runtime and accepts JVM targets only
through 22. The project therefore compiles application source to JVM 22 while
using JBR 25 as its Gradle toolchain/runtime. Run Detekt with a JDK 21 Gradle
launcher and explicitly expose the JBR 25 toolchain until Detekt supports JDK 25:

```text
JAVA_HOME=/path/to/jdk-21 \
  ./desktop/gradlew -p desktop spotlessCheck detekt test \
  -Porg.gradle.java.installations.paths=/path/to/jbrsdk-25.0.4-<platform>-b508.27/Contents/Home
```

This is not a system-JDK installation requirement: callers choose their local JBR
location. A direct JBR 25 launcher was used for `make check`; the Gradle 9.1.0
wrapper reported JBR `25.0.4+1-b508.27` as both launcher and daemon JVM.

| Check | Result |
| --- | --- |
| Baseline `./desktop/gradlew -p desktop spotlessCheck detekt test` before migration | Passed on the original Kotlin 2.0.21/Compose 1.7.0/Gradle 8.6/JVM 21 baseline; generated captures are under `ui-precision/before`. |
| Baseline `make check` | Passed. |
| Baseline `make quality` | Failed before desktop static checks at unchanged Go complexity: `(*Service).reviewPerformanceFile` (19), `validPerformanceJob` (19), `validPerformanceFinding` (18), `(*Service).StartPerformanceJob` (18), and `(*Service).StartAnalyzeAll` (16). The later clone stage was skipped. |
| Temporary Jewel wrapper spike | Passed with the exact standalone artifact and JBR 25; temporary source and dependency removed afterwards. |
| Post-migration `spotlessCheck detekt test` | Passed using the JDK 21 launcher/JBR 25 toolchain command above; generated captures are under `ui-precision/after`. |
| Post-migration `make check` | Passed with JBR 25 as `JAVA_HOME`. |

No production UI component, daemon behavior, configuration file, JDK installation,
custom window decoration, or experimental popup flag was introduced by this task.
