# UI precision acceptance evidence

## Task 170 status

In progress. This record separates deterministic production-component evidence from the
required native-window and assistive-technology checks. An offscreen render is never used as
evidence for native menu, dialog, window-edge, or screen-reader behavior.

## Environment

- Host: macOS 26.6.2 (25G83), arm64.
- Desktop runtime: JBR 25.0.4+1-b508.27-nomod.
- Deterministic UI data: only the production test fixture (`go-shop · fixture`, `Visual fixture · no
  backend`) was rendered; no fixture capture includes a daemon, provider, user project,
  credential, or live provider data. The native launch could not be inspected or captured.

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

UI-04 remains blocked on the complete native and assistive-technology operator matrix below.

## Deterministic component evidence

`DesktopVisualLayoutTest`, `DesktopAccessibilityTest`, and
`DesktopKeyboardNavigationTest` pass with output in the ignored
`desktop/build/reports/ui-precision/task-170/` directory.

| Coverage | Viewports / scales | Evidence classification |
| --- | --- | --- |
| Shell and Analysis hierarchy | 1440×900, 1920×1080, 1000×760, 999×760, 800×650 | Offscreen production-component render |
| Compact short window | 1280×600 at 100%, 125%, 150% text; 100% at 1× and 2× density | Offscreen production-component render |
| Errors, stale/empty/populated evidence | Problems, Checks, Output, Analysis, Context, Assistant, Review, Performance fixtures | Offscreen production-component render and semantics assertions |
| Keyboard / state semantics | Rail, tabs, command palette, popup dismissal/focus return, disclosures, responsive drawer boundary, provider confirmation | Deterministic Compose keyboard and semantics tests |
| Contrast | Primary, secondary, selected, focus, disabled, diff success, and diff error pairs | `DesktopThemeTest` and `DiffViewerTest` token assertions |

The FND-01 table above is the current minimum baseline for the four principal populated views.
The broader Task 170 matrix remains historical component evidence, with its native limitations
unchanged.

Reviewed Task 170 captures include
`analysis-1280-600-100-1x.png`, `analysis-1280-600-125-1x.png`,
`analysis-1280-600-150-1x.png`, and `analysis-1280-600-100-2x.png`. The 150% and
2× captures retain named Preview/Pause/Cancel controls, their textual state, and the layered
pane boundaries without action overlap. These images are not native-window screenshots.

## Prior native and assistive-technology evidence

The app was launched with the pinned JBR runtime and a live
`io.miniorca.desktop.MainKt` process was observed. The available computer-use inventory reported
no native applications both before and while the app was running, so this environment cannot
inspect the window, capture native menus/dialogs, resize it, drive OS focus behavior, or read its
accessibility tree. The temporary process was stopped after that launch check.

No supported screen reader was running or exposed to the automation surface. `AccessibilityUIServer`
alone is an operating-system service, not evidence that VoiceOver or another reader has exercised
Mini-Orca. No screen-reader result is claimed.

## Required operator evidence before completion

On supported macOS/JBR 25 hardware with a disposable fixture, capture and record:

1. Native screenshots at 1440×900, 1920×1080, 1000×760, 999×760, 800×650, and 1280×600;
   repeat the short-window view at 100%, 125%, and 150% text where supported.
2. Window-edge placement and dismissal for Project/Preview menus, command palette, Preview and
   consent/discard dialogs, Files/Context drawers, status details, and bottom-tools overlay.
3. Keyboard focus traversal and restoration for rail, Files tree, editor tabs/breadcrumbs,
   header actions, disclosures, splitters, drawers, menus, palette, dialogs, provider consent,
   and guarded Review. Exercise Escape one transient surface at a time.
4. VoiceOver or another supported reader through core navigation, run controls, provider
   confirmation, and guarded Review; record OS/runtime/reader version and observed names,
   selected/expanded/disabled states, and focus order.

Task 170 remains incomplete until that material native and assistive evidence is attached here.
UI-04 owns native-window, popup, operating-system focus, and reader evidence; REL-01 owns the
release-level provider, lifecycle, package, and distribution checks in the canonical
[release acceptance](../docs/RELEASE_ACCEPTANCE.md).
