# UI precision acceptance evidence

## Task 170 status

Superseded by UI-04. UI-04 closed the attainable inherited native and assistive-technology
checks on 2026-09-07 and records unsupported combinations below. This record keeps deterministic
production-component evidence separate from native evidence. An offscreen render is never used
as evidence for native menu, dialog, window-edge, or screen-reader behavior.

## Environment

- Host: macOS 26.6.2 (25G83), arm64.
- Desktop runtime: JBR 25.0.4+1-b508.27-nomod.
- Deterministic UI data: only the production test fixture (`go-shop · fixture`, `Visual fixture · no
  backend`) was rendered; no fixture capture includes a daemon, provider, user project,
  credential, or live provider data. The resumed UI-04 run below used a separate disposable native
  fixture and loopback daemon.

## FND-01 baseline reconciliation — 2026-09-06

The starting repository identity was `d0a7cc9` (`docs: establish Mini-Orca improvement
autopilot`). Before this task began, the coordinator had changed only the FND-01 status in
`PLAN.md` from Pending to Running. That status edit is not FND-01 evidence. The Task 170 test
source and retained execution record are tracked at that baseline; its generated PNGs are ignored
build output. This task adds no product or configuration behavior.

The current reproduction used the existing `DesktopVisualLayoutTest` production-component
renderer with local fixture data. It ran with JBR `25.0.4.1+1-583.48-jcef`, which is JBR 25 but
not the exact historical Task 170 `25.0.4+1-b508.27-nomod` SDK. The runtime difference is
recorded here rather than treated as equivalent native acceptance.

| View | 1440×900, font/density scale 1.0 | 999×760, font/density scale 1.0 | Source and classification |
| --- | --- | --- | --- |
| Editor | `editor-1440.png` | `editor-999.png` | `EditorVisualFixture`; offscreen production-component render |
| Analysis | `analysis-1440-1.0.png` | `analysis-999-1.0.png` | `AnalysisVisualFixture`; offscreen production-component render |
| Review, ready to apply | `review-ready-1440-900-1.0.png` | `review-ready-999-760-1.0.png` | `ReviewToolWindow` with a current passed required check; offscreen production-component render |
| Performance, populated | `performance-populated-1440-900-1.0.png` | `performance-populated-999-760-1.0.png` | `PerformanceWorkspacePane` with a populated source hypothesis; offscreen production-component render |

All eight artifacts were written to the ignored
`desktop/build/reports/ui-precision/fnd-01/` directory by:

```sh
./desktop/gradlew -p desktop test \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/fnd-01"
```

The run passed: 22 visual-layout, 7 accessibility, and 7 keyboard-navigation tests. PNG
dimensions were checked with `sips`; representative Editor, Analysis, Review, and Performance
renders were visually inspected. The renderer does not open a native window, so this reproduction
does not update native, popup-placement, operating-system focus, or screen-reader evidence.

## UI-04 execution attempt — 2026-09-07

The candidate was `0f1362efa32bc5ec5290ff9542f39b8f9dfef83e`. Before the attempt, the only
worktree change was the coordinator's UI-04 Pending-to-Running transition in `PLAN.md`; it is not
UI evidence. The host was macOS 26.6.2 (25G83), arm64. The Gradle launcher was Temurin
21.0.11+10 and the downloaded, ignored toolchain was the exact documented JBR
25.0.4+1-b508.27-nomod.

The full Desktop gate passed with the documented launcher/toolchain split:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh spotlessCheck detekt test
```

A forced focused run then passed 22 visual-layout, 8 accessibility, and 7
keyboard-navigation tests with no skipped, failed, or errored cases. It generated 66 PNGs under
the ignored `desktop/build/reports/ui-precision/ui-04/` directory:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh test --rerun-tasks \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/ui-04"
```

`sips` confirmed the expected pixels for the component Analysis matrix: 1440×900, 1920×1080,
1000×760, 999×760, 800×650, and 1280×600 at 100%, 125%, and 150% text plus the 100% 2×
density case. `analysis-1280-600-150-1x.png`, `editor-1000.png`, and
`review-ready-800-700-1.3.png` were visually inspected. Their named actions and state remained
readable, but these are offscreen production-component renders only. The rendered
`project-menu-open.png` cannot paint the Desktop scene popup layer, so it supplies no native menu
placement evidence.

For the native attempt, a disposable copy of `internal/project/testdata/release-fixture/` and an
isolated desktop preference root were created only under ignored `desktop/build/ui-04/`. A
loopback daemon used that copied project and local unreachable fixture model destinations; no
provider, credential, or user project was used. The distributable was rebuilt by Gradle running
on the exact JBR and launched from its `.app` executable. `jcmd` directly reported the live
Mini-Orca VM as `OpenJDK 64-Bit Server VM version 25.0.4+1-b508.27`, and the computer-use
inventory reported the running bundle as `Mini-Orca` (`io.miniorca.desktop`).

The automation surface could not bind to that app by either display name or bundle identifier.
Both attempts returned that macOS Accessibility and Screen Recording permissions were still
pending. Those permissions were not granted or reconfigured. Consequently no native screenshot,
window size, edge popup/dialog placement, operating-system focus order, keyboard traversal,
selection, Escape/focus restoration, splitter/drawer resize, text/density scaling, or preference
recovery result is claimed. VoiceOver was not running, and the blocked accessibility surface could
not expose another reader, so no reader name/state observation is claimed. The temporary app and
daemon were stopped after the attempt.

That attempt remained blocked on the complete native and assistive-technology operator matrix
below.

## UI-04 resumed native execution — 2026-09-07

The resumed starting identity was `df426547dd6f35888658d203c4914e8cef7f1630`. The only
pre-existing worktree change was the coordinator's UI-04 status update in `PLAN.md` to
`Running — native permissions restored`; it is not UI evidence. The same exact ignored JBR,
disposable fixture, loopback daemon, and unreachable local fixture-model destinations from the
attempt above were reused. No real provider, credential, or user project was used. A small
ignored `java.util.prefs` test harness stored application preferences only in
`desktop/build/ui-04/prefs/ui-04.properties`; it did not alter macOS preferences.

Accessibility and Screen Recording access worked through the computer-use surface. It exposed
the running `Mini-Orca` window, its accessibility tree, keyboard and pointer input, and
window-only JPEG captures. Capture dimensions were parsed directly from those buffers. The
surface did not provide a file-export API. A targeted `screencapture -l 2615` attempt from the
terminal returned `could not create image from window`, so no native screenshot file is claimed.
The directly observed facts are also summarized in the ignored
`desktop/build/ui-04/logs/native-observations.json` execution artifact.

| Requested native window | Direct result |
| --- | --- |
| 1000×760 | Exact capture; populated/error Summary and docked Editor were readable with no observed clipping. |
| 999×760 | Exact capture; populated/error Summary and compact Editor drawer/bottom-overlay arrangement were readable with no observed clipping. |
| 800×650 | Exact Summary capture; the compact Summary remained readable with the failed-AI state and bottom-tools opener visible. |
| 1280×600 | Exact capture; Summary and docked bottom tabs remained readable with no observed clipping. |
| 1440×900 | Unsupported by the available host work area; the direct resize attempt was clamped to 1361×768. |
| 1920×1080 | Unsupported by the available host work area; the direct resize attempt was clamped to 1383×768. |

The native fixture exposed a populated factual inventory of 10 indexed files and 45 lines plus an
explicit failed AI-analysis state with 10 missing analyses. In Editor, selecting `main.go`
displayed its full read-only source and accessibility descriptions for the file, relative path,
and each source line. A pointer drag visibly selected `Run` source text without selecting a
declaration or changing the file. At compact width, longer fixture names were visually ellipsized
while their complete relative paths remained present in the accessibility names.

The project menu, command palette, macOS project chooser, Files drawer, Context drawer, and
bottom-tools overlay were opened in the native app. Their labels and available/selected states
were present in the accessibility tree and the visible surfaces stayed within the window. Escape
closed one of those transient layers at a time without changing the workspace or selected file.
Tab visibly focused the Project and Search controls, and Return activated the focused control.
Project-menu and Files-drawer dismissal returned activation to their openers.

Direct native inspection found three defects. The Summary dashboard initially exposed Editor as
the selected rail destination; the rail now derives selection from the rendered workspace. The
command palette and compact bottom-tools overlay initially returned focus only to their broad
regions; their actual opener controls now receive focus. In the rebuilt JBR application, Escape
followed by Return reopened each corrected trigger. Deterministic assertions cover the complete
workspace-to-rail mapping and prove that the exact Search and Open tools triggers can receive
focus. The dismissal and trigger-restoration sequence is direct native evidence only.

After the final fixes and launcher correction, the full Desktop run passed 291 tests with zero
Detekt findings:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh spotlessCheck detekt test
```

The final forced focused run passed 24 visual-layout, 8 accessibility, and 7 keyboard-navigation
tests with no skipped, failed, or errored cases. It generated 70 PNGs under the ignored
`desktop/build/reports/ui-precision/ui-04-resumed/` directory:

```sh
MINI_ORCA_JDK21_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem \
MINI_ORCA_JBR25_HOME="$PWD/desktop/build/ui-04/toolchains/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home" \
  ./scripts/desktop-gradle.sh test --rerun-tasks \
  --tests 'io.miniorca.desktop.DesktopVisualLayoutTest' \
  --tests 'io.miniorca.desktop.DesktopAccessibilityTest' \
  --tests 'io.miniorca.desktop.DesktopKeyboardNavigationTest' \
  -PvisualOutput="$PWD/desktop/build/reports/ui-precision/ui-04-resumed"
```

The Compose `run` task had launched the Java 22-bytecode application on Gradle's Java 21 runtime,
causing `UnsupportedClassVersionError`. The application now resolves `javaHome` from the Java 25
toolchain, and `scripts/desktop-gradle.sh` restricts discovery to an explicitly supplied JBR 25.
A direct Java 21 Gradle launch started `MainKt` on the discovered SDKMAN JBR 25; the documented
script started it on the exact ignored `25.0.4+1-b508.27` JBR confirmed by `jcmd`. The repository
`make check` gate passed with that launcher/toolchain split.

The native pane splitters were then resized from the keyboard. The isolated preferences recorded
an Explorer width of `520.0` and bottom-pane height of `220.0`; a restart restored both dimensions
with the disposable project. The stored layout named Editor, while the restarted workspace
correctly opened and announced Summary under the current navigation-restoration policy. The
restart also exposed the empty Editor state before a file was selected.

The connected Q2789 was detected as a non-mirrored 2560×1440 1× display. The user placed the
Gradle-run `MainKt` window there, but that unbundled Java application was not bindable by either
its display name or `com.jetbrains.jbr.java` identifier. The bindable packaged fixture remained
on the main Retina display: attempts to drag its title bar returned `noWindowsAvailable`, and the
display picker treated Q2789 as an offscreen accessibility element because the external display
has a negative origin. No native wide-display visual result is inferred from the connected
monitor or the unbindable process.

VoiceOver 10 (build 993) was enabled through System Settings, and its Essentials collection was
completed so the `scrod` output service and Braille translation service were active. With those
services running, the packaged Mini-Orca accessibility tree exposed explicit names and state for
the project menu, search, daemon status, rail selection, Analyze-all control, analysis coverage,
failed AI interpretation, source metadata, bottom tabs and pane-resize control. Control-Option
navigation commands were sent through the automation surface, but that surface did not expose a
reader cursor or speech transcript; no spoken wording or reader focus sequence is claimed.

The host's text scale and density were not changed; 125%, 150%, and alternate-density coverage
remains deterministic component evidence only. Loading, stale, generated-diff, consent/discard,
provider-confirmation and guarded-Review states were not produced by this native disposable
fixture and are not claimed as native observations. Those combinations are explicit limitations,
while their layout and semantics remain covered by deterministic production-component tests.

## Deterministic component evidence

`DesktopVisualLayoutTest`, `DesktopAccessibilityTest`, and
`DesktopKeyboardNavigationTest` pass with output in the ignored
`desktop/build/reports/ui-precision/task-170/` directory.

| Coverage | Viewports / scales | Evidence classification |
| --- | --- | --- |
| Shell and Analysis hierarchy | 1440×900, 1920×1080, 1000×760, 999×760, 800×650 | Offscreen production-component render |
| Compact short window | 1280×600 at 100%, 125%, 150% text; 100% at 1× and 2× density | Offscreen production-component render |
| Errors, stale/empty/populated evidence | Problems, Checks, Output, Analysis, Context, Assistant, Review, Performance fixtures | Offscreen production-component render and semantics assertions |
| Keyboard / state semantics | Rail/tab navigation and selected state, command-palette arrow/Enter interaction, disclosure state, responsive region policy, provider confirmation, and Search/Open tools opener focusability | Deterministic Compose keyboard and semantics tests; exact dismissal/restoration is native evidence above |
| Contrast | Primary, secondary, selected, focus, disabled, diff success, and diff error pairs | `DesktopThemeTest` and `DiffViewerTest` token assertions |

The FND-01 table above is the current minimum baseline for the four principal populated views.
The broader Task 170 matrix remains historical component evidence, with its native limitations
unchanged.

Reviewed Task 170 captures include
`analysis-1280-600-100-1x.png`, `analysis-1280-600-125-1x.png`,
`analysis-1280-600-150-1x.png`, and `analysis-1280-600-100-2x.png`. The 150% and
2× captures retain named Preview/Pause/Cancel controls, their textual state, and the layered
pane boundaries without action overlap. These images are not native-window screenshots.

## Prior blocked native and assistive-technology evidence

The app was launched with the pinned JBR runtime and a live
`io.miniorca.desktop.MainKt` process was observed. The available computer-use inventory reported
no native applications both before and while the app was running, so this environment cannot
inspect the window, capture native menus/dialogs, resize it, drive OS focus behavior, or read its
accessibility tree. The temporary process was stopped after that launch check.

No supported screen reader was running or exposed to the automation surface. `AccessibilityUIServer`
alone is an operating-system service, not evidence that VoiceOver or another reader has exercised
Mini-Orca. No screen-reader result is claimed.

## Recorded limitations and release follow-up

UI-04 is complete because the material native checks available to the operator surface passed,
the three defects they exposed were fixed, and unsupported combinations are explicit. It closes
the attainable native-window, popup, operating-system focus and reader-name/state requirements
inherited from Tasks 149 and 170.

The following are unclaimed release limitations rather than inferred passes:

1. Exact native 1440×900 and 1920×1080 captures on Q2789; the component matrix covers both sizes,
   while the bindable native fixture could not cross the negative-origin display boundary.
2. Native 125%/150% text and alternate-density runs; deterministic production-component coverage
   exists for those combinations.
3. Native loading, stale, generated-diff, consent/discard, provider-confirmation and guarded-Review
   states; deterministic layout, semantics and interaction tests cover them.
4. A VoiceOver speech transcript and observable reader-cursor focus sequence; VoiceOver services
   were active and native names/states were inspected, but the computer-use surface exposed
   neither speech output nor the reader cursor.

REL-01 owns any release-level provider, lifecycle, package and distribution checks that require
those states or a different operator surface. The canonical status remains in
[release acceptance](../docs/RELEASE_ACCEPTANCE.md).
