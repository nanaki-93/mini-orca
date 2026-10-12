# Mini-Orca UI guidelines

Design around the developer's next action. Keep the main content and primary
action prominent; disclose supporting metadata and explanations on demand.
Routine actions should execute from their buttons without redundant dialogs.
Palette, typography, navigation and layout may change as needed.

## Interaction and accessibility

Keep actions, current state, errors and recovery paths clear. Use accessible names,
keyboard operation and visible focus. Color supplements labels; check contrast
against actual backgrounds. Show progress while requests or jobs run and prevent
duplicate submission. Honor reduced-motion preferences.

Preserve access to findings, explanations and diagnostics. Long paths, narrow
windows and large text must not hide actions or clip content. Model-authored HTML,
links and remote images remain inert; source and composed diffs stay read-only.

## Product behavior

Navigation, disclosures and previews do not dispatch providers, execute project
code or write source. Keep provider consent and execution trust tied to the
relevant action and scope. Explicit Apply/Undo and stale-evidence checks remain
enforced. Never invent project facts or present suggestions as verified results.
Unknown, unavailable, stale, partial, failed and canceled states retain meaning.

## Verification

Use browser tests of production components and workflow tests at relevant viewport
and text sizes. Exercise loading, empty, populated and failure states, collapsed
details, keyboard access, action reachability and contrast. Inspect rendered
screenshots. These tests do not prove native focus, popup placement or screen-reader
behavior; native packaging and PTY lifecycle checks provide separate evidence.
