# Dark UI token contrast checks

**Recorded:** 2026-09-04  
**Task:** 141 — charcoal theme and icon system

Ratios use the WCAG relative-luminance formula on the opaque semantic color pairs
implemented in `DesktopTheme.kt`.

| Foreground / background | Ratio | Target | Result |
| --- | ---: | ---: | --- |
| Primary text `#E6EDF3` / app `#171B20` | 14.64:1 | 4.5:1 | Pass |
| Secondary text `#AAB6C3` / panel `#1C2229` | 7.78:1 | 4.5:1 | Pass |
| White action text `#FFFFFF` / action fill `#285FCB` | 5.85:1 | 4.5:1 | Pass |
| Focus cyan `#65D2EC` / panel `#1C2229` | 9.15:1 | 3:1 | Pass |
| Success `#65D6A3` / added-diff surface `#18352C` | 7.38:1 | 4.5:1 | Pass |
| Error `#FF8F98` / removed-diff surface `#3A232B` | 6.62:1 | 4.5:1 | Pass |

`DesktopThemeTest` asserts these token relationships. Integration of blended hover and
selected surfaces will be checked after those controls are fully restyled in Task 148.
