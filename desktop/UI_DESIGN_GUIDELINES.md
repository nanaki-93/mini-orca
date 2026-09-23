# Mini-Orca UI guidelines

Previous Overview, Performance, Bugs, Security and Source mockups illustrate
an earlier direction, not a required layout or palette. Concepts under
`design/ui-mocks/`, when available, can be used as starting points. The UI may
be redesigned across the shell, workspaces, navigation, typography, colors,
shapes and copy.
Treat mock content as illustrative, not as real project data or supported actions.

## Self-explanatory UI and copy

Make the task, current state and next action understandable from the interface.
Choose labels, headings, explanatory text and disclosures to suit the specific
flow; no previous wording or heading hierarchy is mandatory. Avoid redundant
copy, but keep information needed to identify content, act, decide or recover.

- Keep fields identifiable, controls named for assistive technology, keyboard
  focus visible and essential information available without hover. Color alone
  must not communicate state. Check contrast on actual backgrounds and states;
  [UI_CONTRAST.md](UI_CONTRAST.md) records measurements of an earlier palette,
  not a palette requirement.
- Keep the relevant target, consequence, consent and trust requirements clear
  before an action. Distinguish verified findings from model suggestions, measured
  performance from estimates, and unknown, stale, partial, canceled and failed
  results from successful or empty results.
- Preserve the full meaning and availability of findings, explanations, errors
  and diagnostics even when changing their presentation.

## Implementation boundaries

Use the existing workflow and state owners for requests, eligibility and guarded
changes. Navigation, tabs, disclosures and previews are local interactions; they
must not silently request a model response, run project code or modify source.
Remote-provider consent, execution trust, Review/Apply/Undo and stale-evidence
checks remain explicit. Source and composed diffs stay read-only; only the
isolated declaration/import draft is editable. Do not invent project facts,
findings, scores or working controls without behavior behind them.

The current client uses Jewel, `DesktopTheme.kt`, `ChromeControls.kt` and
`DesktopIcons.kt`. Changes to visual primitives should be coordinated so the
interface remains coherent; the existing token values, component shapes and
pane positions are not design constraints. Keep semantic state and focus
indicators distinct and test any new palette's contrast. The current
`ModelResultContent` handles selectable freeform model prose with a bounded
literal fallback; if its presentation changes, keep the content complete and
model-supplied links, HTML and images inert.

## Verification

Review changed flows with optional detail collapsed and at representative wide,
compact and short-window sizes, larger text, long content and relevant empty,
loading, populated and failure states. Check reachability, clipping, contrast,
keyboard focus and accessible names/states. Compare rendered production
components with the *chosen* visual direction, not solely with earlier captures.
Use [component reproduction](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks)
and the [keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md) when applicable.
Offscreen Compose renders cannot prove OS focus, popup placement or screen-reader
behavior; report native evidence separately in
[release acceptance](../docs/RELEASE_ACCEPTANCE.md).
