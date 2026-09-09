# Mini-Orca UI guidelines

Use the [dark reference](../design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png)
and [dark UI direction](../docs/dark-ui/README.md). Build a dense, coherent developer
tool with source, context and evidence. [PLAN.md](../PLAN.md) owns implementation
priorities; the screenshot's sample facts and unsupported controls are illustrative.

## One design system

Jewel standalone is already adopted. Use `DesktopTheme.kt`, `ChromeControls.kt`
and `DesktopIcons.kt`; do not create another theme or copy styles into each pane.
Small app-owned source/diff components remain appropriate. Low-level text/layout
primitives must use the shared semantic roles, not stock Material/Swing appearance.
Runtime and dependency setup lives in [README.md](README.md#runtime-and-build).

| Role | Current target |
| --- | --- |
| Activity rail / outer chrome | `#18191B` |
| Tool windows / sidebars / bottom panes | `#1E1F22` |
| Editor / content canvas | `#2B2D30` |
| Overlay | `#26282C` |
| Active indicator/action accent | `#3574F0` |
| Selected surface | `#2E436E` |
| Separator | `#323438`, 1dp |

Keep text, disabled, success/warning/error, diff and focus as separate semantic
roles. Verify actual blended backgrounds, including selection and disabled fills.
Normal meaningful text targets 4.5:1; focus/control indicators target 3:1. Current
measurements are in [UI_CONTRAST.md](UI_CONTRAST.md). Color never replaces labels.

## Geometry and hierarchy

- Flat square pane surfaces, no elevation, no repeated rounded section cards.
  One owner per 1dp boundary; keep larger invisible splitter hit targets and keys.
  Reserve 4–6dp corners for inputs, contained actions, popups and dialogs.
- Align to a 4dp grid; normal content inset 8dp and internal gaps 4–8dp. Avoid
  nested 16–20dp padding. Headers/actions default to 28–32dp; rows to 24–28dp.
- Body 12–13sp with explicit 18–20sp line height; secondary chrome 11–12sp;
  breadcrumbs 12sp; source/diff monospaced and readable. Grow at 125/150% text
  scale rather than clipping or shrinking the font to fit.
- Keep restrained titles and labeled state. Headers own their actions, especially
  Start/Pause/Resume/Cancel. Trailing actions never toggle an adjacent disclosure.
  Use short labels/tooltips/accessibility names when an icon is ambiguous.
- Selection has an active fill and accent edge; keyboard focus is independently
  visible. Tool-window and source tabs share that policy.

## Shell and task flow

Keep the labeled activity rail, primary workspace, Editor-only Files/inspector
panes and integrated status bar. At ≥1000dp use resizable docked panes; below it
use labeled Files/Context drawers and a bounded bottom overlay. Preserve saved
widths when temporarily clamping and keep essential source/actions reachable in
short windows. Native window controls remain native.

Files uses real project-relative paths and one active file. Breadcrumbs expose
real path/symbol identity; do not invent navigation callbacks or tabs. Source and
composed diff remain selectable/read-only. Only the declaration/import draft edits.

Context explains the selected code, Assistant prepares the request/draft, and
Review shows the exact candidate with current validation/checks, Apply, receipt
and Undo. Use one next valid action and a concise blocked reason; expand technical
details on demand. Never duplicate daemon eligibility rules in a visual helper.

Keep empty, loading, stale, failed, partial, canceled and unavailable states
explicit. Unknown metrics are not zero. Daemon connectivity is not provider
connectivity. No fabricated findings, project facts or scores in normal usage.

UI-01 removes current inert Preview utilities. Until that task is implemented,
any retained Preview stays labeled and local-only. No navigation, disclosure,
preview or preset may send a provider request, execute code or mutate source.
Fresh AI requests require explicit scope-specific consent where applicable.

## Verification

Compare rendered production components to the references for every substantive
visual change. Check wide views, 1000/999dp, 800×650, 1280×600, large text, long
paths/errors and affected empty/stale/populated states. Check alignment, clipping,
action reachability, separators, keyboard focus and names/states.

Use [component reproduction](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks) for fixture reproduction and the retained
[keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md) for native checks. Offscreen
Compose renders cannot prove OS focus, popup placement or screen-reader behavior.
Record those separately in [release acceptance](../docs/RELEASE_ACCEPTANCE.md).
