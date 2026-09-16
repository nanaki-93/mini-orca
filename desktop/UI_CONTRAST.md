# IDE token contrast

The shared charcoal theme separates the teal outer frame, tool windows, the darker
source canvas, section headers and outlined controls. Blue identifies actions and
selection; cyan identifies information and running work; green, amber and coral
identify success, warnings and failures. Every state retains its text label.

Use the [UI copy rules](UI_DESIGN_GUIDELINES.md#self-explanatory-ui-and-copy) for
concise labels; reducing copy does not relax contrast or state-label requirements.

Colors are owned by [DesktopTheme.kt](src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt).
These measurements use its current sRGB values and resolved backgrounds. Meaningful
text, including muted and disabled labels, targets 4.5:1. Essential control and
focus indicators target 3:1; decorative pane separators are supplementary boundaries.

| Pair | Foreground / background | Contrast |
| --- | --- | ---: |
| Primary text / outer frame | `#F5F7FA` / `#203238` | 12.43:1 |
| Secondary text / outer frame | `#CCD4DF` / `#203238` | 8.92:1 |
| Muted text / outer frame | `#A9B4C3` / `#203238` | 6.35:1 |
| Resize handle / outer frame | `#8E9EAF` / `#203238` | 4.86:1 |
| Hovered resize handle / outer frame | `#73ABFF` / `#203238` | 5.71:1 |
| Primary text / tool window | `#F5F7FA` / `#24282F` | 13.78:1 |
| Primary text / source and terminal | `#F5F7FA` / `#1B1E23` | 15.57:1 |
| Secondary text / header and overlay | `#CCD4DF` / `#303640` | 8.13:1 |
| Muted text / selection | `#A9B4C3` / `#263F62` | 5.08:1 |
| Selected text / selection | `#C9DFFF` / `#263F62` | 7.86:1 |
| Primary action label / default | `#101722` / `#78ACFF` | 7.83:1 |
| Primary action label / hover and press | `#101722` / `#98C1FF` | 9.77:1 |
| Primary action label / selected | `#101722` / `#619AFF` | 6.49:1 |
| Apply label / default | `#101722` / `#74E0AC` | 11.13:1 |
| Apply label / hover and press | opaque 90% success over editor | 9.30:1 |
| Apply label / selected | opaque 82% success over editor | 7.91:1 |
| Control outline / hovered control | `#8E9EAF` / `#414E5F` | 3.09:1 |
| Selection edge / selection fill | `#73ABFF` / `#263F62` | 4.57:1 |
| Focus / dark inner keyline | `#D3E4FF` / `#203238` | 10.36:1 |
| Success text / added diff | `#74E0AC` / `#193E30` | 7.33:1 |
| Failure text / removed diff | `#FF929E` / `#482730` | 6.13:1 |

Primary actions have opaque default, hover/pressed and selected fills. This avoids
the former hover label dropping to 3.66:1 when its blue fill became translucent.
The primary Apply action has its own opaque positive tone, keeping its dark label
and the existing focus keyline readable. Positive badges and secondary actions
retain their tinted treatment. Other semantic actions use tint fills resolved over the tool window, so placing them
inside an overlay or selected row does not change their label contrast. Disabled
controls use a neutral fill and muted border. Focus uses a light outline with a
dark inner keyline, which remains visible against a bright primary action.

Badges use an opaque 16% tint over the tool window and a stronger matching outline.
The flat toolbar keeps its passive analysis and daemon statuses unboxed on the outer
frame: running uses information (8.19:1), attention uses warning (9.24:1), neutral
analysis uses secondary text (8.92:1), and connection labels use success (8.26:1) or
failure (6.26:1). Each status keeps its text label, and the daemon status remains
separate from provider state.

| Badge label / its resolved fill | Contrast |
| --- | ---: |
| Information / running | 6.23:1 |
| Success / fresh | 6.27:1 |
| Warning / stale | 6.83:1 |
| Failure | 5.11:1 |
| Unavailable / neutral | 6.60:1 |

[DesktopContrastTest](src/test/kotlin/io/miniorca/desktop/DesktopContrastTest.kt)
checks all action tones in default, hover/press, selected and disabled states on
the supported host surfaces; text and syntax on dark, selected and raised surfaces;
and rendered action/badge labels at 100% and 150% text scale. A pixel check verifies
that both focus keylines survive rendering. The existing layout, accessibility and
keyboard suites retain coverage of 1000/999dp docking, 800×650, short 1280×600
windows, larger text, and empty/stale/failed/populated result states.

Visual evidence is generated with the [component reproduction procedure](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks).
Earlier palette comparison captures are under `desktop/build/reports/ui-contrast/`.
They are offscreen production-component renders. Native-window interaction and
screen-reader behavior were not verified in that earlier check because the
computer-use tool could not attach to the running Java app.

## Rounded frame verification — 2026-09-16

The frame changes are checked by `DesktopThemeTest` against the actual chrome,
control and focus colors, and by `DesktopVisualLayoutTest` using filled production
panes, rendered corner pixels, gutter bounds and keyboard/pointer splitters.
The current frame comparisons are under `desktop/build/reports/mockup-ui/mock-01/`.
They cover 1600×1000, 1440×900, 1000/999×760, 800×650 and 1280×600 at 100/125/150%
text, with the terminal expanded/collapsed in docked layouts and collapsed in
narrow layouts. The existing terminal tests cover the separate overlay. These are
offscreen component renders. Final MOCK-06 native and packaged evidence is recorded
separately in [rounded mockup acceptance](../docs/RELEASE_ACCEPTANCE.md#rounded-mockup-acceptance--2026-09-16).
The final 493-test gate includes the existing complete contrast checks; the native
terminal inset introduces no new color or interaction-state fill.
