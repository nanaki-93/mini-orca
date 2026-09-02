# 80 — Add compact semantic desktop controls

## Status

Pending

## Goal

Create one reusable button and single-line input system that makes action purpose and
priority visible while moderately reducing control size across the Desktop client.

## Depends on

Task 79.

## Required task commentary

- Before editing, post a concise user-facing commentary update beginning with
  `Starting Task 80` and name the control primitives, likely files, and focused tests.
- After verification, post a concise update beginning with `Task 80 complete` and state
  the behavior changed, tests run, and that Task 81 is next.
- During long-running work, continue posting brief progress commentary at least every
  60 seconds. These are user-facing task updates, not source-code comments.

## Implementation

- Inspect the current uncommitted Desktop changes before editing. Treat every pre-existing
  hunk as user-owned and preserve it.
- Replace the shared button's primary/neutral-only contract with one small semantic action
  role, such as:
  - Primary — violet;
  - Navigation — cyan;
  - Positive — mint;
  - Attention — amber;
  - Destructive/interruption — rose;
  - Neutral — the existing raised/strong surface.
- Define the role and its enabled, disabled, selected, pressed, border, and content colors
  in `DesktopTheme.kt`. Reuse the approved Focus Flow palette rather than introducing
  screen-local colors.
- Add a small shared control-density choice for standard and top-bar/toolbar buttons.
  Target:
  - 34–36dp standard button height;
  - 30–32dp toolbar button height;
  - 10dp/4dp standard content padding;
  - 8dp compact horizontal padding;
  - 11sp default button text.
- Remove the superseded `primary: Boolean` API and migrate every supported
  `FocusFlowButton` call to an explicit or correct default semantic role. Do not keep
  parallel old/new button APIs.
- Use these action-role assignments consistently:
  - Open project, Analyze, Start, Send, and Validate use Primary;
  - workspace/view/file-opening actions use Navigation;
  - Continue to Apply and an eligible Apply use Positive;
  - Pause and Undo use Attention;
  - Cancel and Dismiss use Destructive/interruption;
  - Close, Inspect, and secondary utilities use Neutral.
- Keep destructive/interruption controls outlined or tinted where a filled treatment
  would visually overpower the primary action.
- Add one shared compact single-line field wrapper using existing Material/Compose
  behavior. Target 44–46dp height and compact internal padding without clipping labels,
  placeholders, cursor, or focus indication.
- Migrate single-line controls in Explorer, Command Palette, Analysis limits, Bugs search
  and filters, Target symbol entry, and Draft imports to the compact wrapper.
- Keep multiline chat and declaration editors content-driven. Do not force them through
  the single-line compact height.
- Preserve text labels, keyboard focus, selected/disabled semantics, contrast, and
  screen-reader descriptions. Color must not be the only state signal.
- Do not change workflows, API calls, project identity guards, layout visibility, copy,
  or filter behavior in this task.

## Acceptance criteria

- Every supported Desktop button renders through one semantic action-role system.
- Primary, navigation, positive, attention, destructive/interruption, and neutral actions
  are visibly distinct and use only shared palette tokens.
- The old boolean primary-button API and obsolete per-call color overrides are removed.
- Standard and toolbar controls follow the compact size targets without clipped text or
  focus indicators.
- Every supported single-line input uses the shared compact field behavior.
- Multiline message and draft editors retain usable content height.
- Disabled, selected, warning, and error state remain explicit in text/semantics and are
  not conveyed by color alone.
- No product behavior, API request, revision/hash guard, or responsive breakpoint changes.

## Verification

- Extend `DesktopThemeTest` for the action-role palette and density mappings.
- Extend affected accessibility/presentation tests for enabled, disabled, selected, and
  destructive labels where practical.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

- Inspect the complete Task 80 diff for old `primary =` call sites, ad-hoc button colors,
  duplicate compact field implementations, and unrelated changes.

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update its link/status in `tasks/INDEX.md`. Do not create a commit; this task definition
does not authorize commits.
