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

Reviewed Task 170 captures include
`analysis-1280-600-100-1x.png`, `analysis-1280-600-125-1x.png`,
`analysis-1280-600-150-1x.png`, and `analysis-1280-600-100-2x.png`. The 150% and
2× captures retain named Preview/Pause/Cancel controls, their textual state, and the layered
pane boundaries without action overlap. These images are not native-window screenshots.

## Native and assistive-technology evidence

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
