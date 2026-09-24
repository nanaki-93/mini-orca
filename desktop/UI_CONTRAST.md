# Tokyo Midnight token contrast

The shared Tokyo Midnight palette separates indigo outer chrome, tool windows, the
darker source canvas, section headers and outlined controls. Blue identifies
primary actions, cyan information and running work, and lavender product identity;
green, amber and coral identify success, warnings and failures. Every state retains
its text label.

Use the [UI copy rules](UI_DESIGN_GUIDELINES.md#self-explanatory-ui-and-copy) for
concise labels; reducing copy does not relax contrast or state-label requirements.

Colors are owned by [DesktopTheme.kt](src/main/kotlin/io/miniorca/desktop/DesktopTheme.kt).
These measurements use its current sRGB values and resolved backgrounds. Meaningful
text, including muted and disabled labels, targets 4.5:1. Essential control and
focus indicators target 3:1; decorative pane separators are supplementary boundaries.

| Pair | Foreground / background | Contrast |
| --- | --- | ---: |
| Primary text / outer frame | `#C0CAF5` / `#171821` | 10.94:1 |
| Secondary text / outer frame | `#A9B1D6` / `#171821` | 8.37:1 |
| Muted text / outer frame | `#A9B1D6` / `#171821` | 8.37:1 |
| Resize handle / outer frame | `#9299BE` / `#171821` | 6.33:1 |
| Hovered resize handle / outer frame | `#7DCFFF` / `#171821` | 10.29:1 |
| Primary text / tool window | `#C0CAF5` / `#1D1E2B` | 10.22:1 |
| Primary text / source and terminal | `#C0CAF5` / `#1A1B26` | 10.59:1 |
| Secondary text / header and overlay | `#A9B1D6` / `#303145` | 6.02:1 |
| Muted text / selection | `#A9B1D6` / `#292E45` | 6.33:1 |
| Selected text / selection | `#C0CAF5` / `#292E45` | 8.28:1 |
| Primary action label / default | `#161824` / `#7AA2F7` | 7.01:1 |
| Primary action label / hover and press | `#161824` / `#9AB8FF` | 8.97:1 |
| Primary action label / selected | `#161824` / `#6783C3` | 4.71:1 |
| Apply label / default | `#161824` / `#9ECE6A` | 9.65:1 |
| Apply label / hover and press | `#161824` / resolved 90% success over `#1A1B26` (`#91BC63`) | 8.05:1 |
| Apply label / selected | `#161824` / resolved 82% success over `#1A1B26` (`#86AE5E`) | 6.91:1 |
| Control outline / hovered control | `#9299BE` / `#34364B` | 4.24:1 |
| Selection edge / selection fill | `#7DCFFF` / `#292E45` | 7.79:1 |
| Focus / dark inner keyline | `#D7DEFF` / `#171821` | 13.27:1 |
| Success text / added diff | `#9ECE6A` / `#26382F` | 6.80:1 |
| Failure text / removed diff | `#FF8FA3` / `#402632` | 6.30:1 |

Primary actions have opaque default, hover/pressed and selected fills. This avoids
the former hover label dropping to 3.66:1 when its blue fill became translucent.
The primary Apply action uses its positive tone; its hover/pressed and selected
fills are resolved alpha blends over the editor canvas, with the measured ratios
above. The dark label and existing focus keyline remain readable. Positive badges and secondary actions
retain their tinted treatment. Other semantic actions use tint fills resolved over the tool window, so placing them
inside an overlay or selected row does not change their label contrast. Disabled
controls use a neutral fill and muted border. Focus uses a light outline with a
dark inner keyline, which remains visible against a bright primary action.

Badges use an opaque 16% tint over the tool-window surface (`#1D1E2B`) and a
stronger matching outline. Their resolved fills and foreground measurements are
listed below. The flat toolbar keeps passive analysis and daemon statuses unboxed
on the outer frame (`#171821`): running uses information, attention uses warning,
neutral analysis uses secondary text, and connection labels use success or failure.
Each status keeps its text label, and daemon status remains separate from provider
state.

| Toolbar status / outer frame `#171821` | Contrast |
| --- | ---: |
| Running / information `#7DCFFF` | 10.29:1 |
| Attention / warning `#E0AF68` | 8.83:1 |
| Neutral analysis / secondary `#A9B1D6` | 8.37:1 |
| Connected / success `#9ECE6A` | 9.66:1 |
| Connection failure / error `#FF8FA3` | 8.17:1 |

| Badge label / resolved fill over `#1D1E2B` | Contrast |
| --- | ---: |
| Information / running `#2C3A4D` | 6.73:1 |
| Success / fresh `#323A35` | 6.41:1 |
| Warning / stale `#3C3535` | 5.99:1 |
| Failure `#41303E` | 5.65:1 |
| Unavailable / neutral `#333646` | 5.66:1 |

[DesktopContrastTest](src/test/kotlin/io/miniorca/desktop/DesktopContrastTest.kt)
checks all action tones in default, hover/press, selected and disabled states on
the supported host surfaces; text and syntax on dark, selected and raised surfaces;
and rendered action/badge labels at 100% and 150% text scale. A pixel check verifies
that both focus keylines survive rendering. The existing layout, accessibility and
keyboard suites retain coverage of 1000/999dp docking, 800×650, short 1280×600
windows, larger text, and empty/stale/failed/populated result states.

Visual evidence is generated with the [component reproduction procedure](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks).
Post-refresh Tokyo Midnight production-component renders are retained under
`desktop/build/reports/tokyo-midnight/`, including `chrome-review/`,
`shell-step-2-2/`, `step-2-3/` and `task-3-review/`. These are offscreen production
component renders; their presence does not establish native-window interaction,
OS focus, popup placement or screen-reader behavior. Native behavior was not
verified as part of those renders.

The older captures under `desktop/build/reports/analysis-rounded/` predate the
Tokyo Midnight refresh and are historical, not evidence for the current palette.

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
