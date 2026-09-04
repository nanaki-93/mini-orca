# IDE token contrast

Task 152 centralizes the desktop palette in `DesktopTheme.kt`. The token values below are the
authoritative visual roles; Compose Material colors only receive those mapped values.

| Pair | Tokens | Contrast |
| --- | --- | ---: |
| Primary text on workspace | `#F2F2F2` / `#1E1F22` | 14.72:1 |
| Secondary text on workspace | `#C4C7C5` / `#1E1F22` | 9.67:1 |
| Selected-content text on selection surface | `#A8C7FA` / `#2E436E` | 5.70:1 |
| Action label on active fill | `#0B0D10` / `#3574F0` | 4.55:1 |
| Keyboard focus on workspace | `#A8C7FA` / `#1E1F22` | 9.59:1 |
| Success text on added diff | `#65D6A3` / `#18352C` | 7.38:1 |
| Error text on removed diff | `#FF8F98` / `#3A232B` | 6.62:1 |

`contrastRatio` resolves a foreground alpha over its actual background. Disabled action testing
therefore first blends the translucent button fill over the panel before measuring its text.
Normal text targets are at least 4.5:1 and focus cues at least 3:1. The muted selection surface
and bright focus/content colors deliberately differ, so selection is not the only focus signal.
