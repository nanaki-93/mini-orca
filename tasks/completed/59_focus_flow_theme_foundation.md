# 59 — Install the Focus Flow theme foundation

## Status

Complete

## Goal

Create one semantic Compose theme using the selected Direction C color system and
the shared visual primitives needed by the refactor.

## Depends on

Task 58.

## Implementation

- Replace scattered global styling with `MiniOrcaTheme`, semantic color roles,
  typography, shapes, spacing, and code-token colors.
- Use the approved palette: background `#080917`, surfaces `#101225`, `#171A31`,
  and `#222640`, border `#292D49`, text `#F2F2FB`/`#9297B6`, violet `#9B8CFF`,
  cyan `#62D8EF`, success `#55DDB0`, warning `#FFC86E`, error `#FF7F9F`.
- Add cohesive primitives only for repeated needs: panel/card surfaces, section
  labels, textual status badges, labeled icon buttons, focus treatment, empty/error/
  loading states, and code lines.
- Keep system UI/monospace fonts and avoid runtime font downloads or blur-heavy effects.
- Make code highlighting consume semantic code colors while preserving source text.
- Prevent new one-off color literals outside the theme.

## Acceptance criteria

- The current Desktop UI renders from the Focus Flow semantic palette.
- Success, warning, error, selected, focused, and disabled states remain distinct
  with text as well as color.
- Highlighting returns byte-for-byte identical source text.
- Shared primitives reduce real duplication without becoming a speculative design framework.

## Verification

- Extend theme/highlighting/status helper tests.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
