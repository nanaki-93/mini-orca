# Mini-Orca UI guidelines

Visual design is open to change. The retained
[Tokyo Midnight concept](../.mockups/screens/visual-refresh/option-3a-tokyo-midnight.html)
is an optional reference, not a required target for palette, typography,
component shapes, spacing, shell, navigation, pane arrangement or responsive
behavior. The current implementation is not a design requirement.

## Interaction and accessibility

Choose copy, hierarchy, controls and presentation for the task at hand. Keep
users able to identify the current state, available actions, errors and recovery
paths. Keep necessary labels and accessible names/states, keyboard operation and
visible focus. Do not rely on color alone to convey meaning. Check text and
indicator contrast against the backgrounds used by the chosen design, including
interaction states and larger text. Preserve full access to findings, explanations
and diagnostics even if their presentation changes.

## Product behavior

Visual changes must not silently change the workflow: navigation, disclosures and
previews do not request a provider response, run project code or write source.
Remote-provider consent, execution trust, Review/Apply/Undo and stale-evidence
checks remain explicit. Source and composed diffs stay read-only; only the
isolated declaration/import draft is editable. Do not invent project facts,
findings, scores or working controls without behavior behind them. Distinguish
verified results from model suggestions, measured performance from estimates,
and unknown, stale, partial, canceled and failed results from successful results.

The current client uses Jewel and shared controls, but their existing styles and
tokens are not mandated for redesign. Coordinate implementation changes so the
interface remains coherent. `ModelResultContent` currently handles freeform model
prose; if its presentation changes, keep content available and model-supplied
links, HTML and images inert.

## Verification

Test changed flows at relevant viewport and text sizes and across relevant result
states. Check action reachability, clipping, keyboard focus, accessible names,
contrast and clear consent/error states. Compare rendered production components
against the *chosen* design. Use `DesktopVisualLayoutTest` component renders
and the [keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md) when applicable.
Offscreen renders cannot prove native focus, popup placement or screen-reader
behavior; report native observations separately.
