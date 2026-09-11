# IDE token contrast

Task 162 centralizes the desktop palette in `DesktopTheme.kt` and makes Jewel's
standalone `IntUiTheme` the production outer theme. Task 169 removed the temporary
Material theme bridge; remaining text primitives receive these explicit semantic
content roles directly.

| Pair | Tokens | Contrast |
| --- | --- | ---: |
| Primary text on tool window | `#F2F2F2` / `#1E1F22` | 14.72:1 |
| Primary text on editor canvas | `#F2F2F2` / `#2B2D30` | 12.33:1 |
| Primary text on overlay | `#F2F2F2` / `#26282C` | 13.19:1 |
| Secondary text on tool window | `#C4C7C5` / `#1E1F22` | 9.67:1 |
| Selected-content text on selection surface | `#A8C7FA` / `#2E436E` | 5.70:1 |
| Action label on active fill | `#0B0D10` / `#3574F0` | 4.55:1 |
| Keyboard focus on tool window | `#A8C7FA` / `#1E1F22` | 9.59:1 |
| Keyboard focus on overlay | `#A8C7FA` / `#26282C` | 8.59:1 |
| Success text on added diff | `#65D6A3` / `#18352C` | 7.38:1 |
| Error text on removed diff | `#FF8F98` / `#3A232B` | 6.62:1 |

`contrastRatio` resolves a foreground alpha over its actual background. Disabled action testing
therefore first blends the translucent button fill over the panel before measuring its text.
Normal text targets are at least 4.5:1 and focus cues at least 3:1. The muted selection surface
and bright focus/content colors deliberately differ, so selection is not the only focus signal.

## Final result and terminal roles — 2026-09-12

Measured from the current `DesktopTheme.kt` token values with sRGB relative
luminance. Result badges use the actual opaque 12% tint blended over the panel;
foreground alpha is resolved before calculating contrast. The existing automated
DesktopThemeTest covers these combinations.

| Meaningful text / actual background | Contrast |
| --- | ---: |
| Result accent / editor | 8.07:1 |
| Result accent / selected row | 5.72:1 |
| Primary / selected row | 8.75:1 |
| Secondary / selected row | 5.75:1 |
| Terminal text / terminal canvas | 12.33:1 |
| Terminal attention / panel | 9.68:1 |
| codeType label / its badge fill | 7.42:1 |
| success label / its badge fill | 7.14:1 |
| warning label / its badge fill | 7.45:1 |
| error label / its badge fill | 6.08:1 |
| secondaryText label / its badge fill | 7.41:1 |

All listed text exceeds 4.5:1. The focus indicator remains separate from selected
fill and exceeds 3:1 on panel, overlay and selection backgrounds. Named status,
severity and provenance labels carry meaning without color. Decorative separators
and tinted badge borders are not the sole signal of a state or required action.
