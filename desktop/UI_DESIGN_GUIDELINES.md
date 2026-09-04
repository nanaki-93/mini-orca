# Mini-Orca UI design guidelines

Recorded from the user's design direction on 2026-09-04 and updated with the UI
precision requirements. Apply these guidelines to future desktop UI improvements.
The active [Plan.md](../Plan.md) specifies Tasks 161–171; this document does not
itself implement that plan or replace the native window frame.

## Visual direction

Build a cohesive, dense developer tool in the spirit of JetBrains Fleet/IntelliJ,
not a generic dark Material application. Follow the proportions and hierarchy in
the [supplied dark mock](../design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png).
The mock's sample project facts and unsupported controls are not product requirements.

Color alone is insufficient. Typography, surface hierarchy, borders, alignment,
selection, hover, focus, and pane proportions must work together. Prefer shared,
purpose-built components over styling isolated screens independently.

## Design tokens

Use the following target palette, expressed through semantic tokens in the shared
theme rather than hexadecimal colors scattered through composables:

| Role | Target |
| --- | --- |
| Activity rail / outer chrome | `#18191B` |
| Tool windows / sidebars / bottom panes | `#1E1F22` |
| Editor and main content canvas | `#2B2D30` |
| Active indicator/action accent | `#3574F0` |
| Muted selected/highlight surface | `#2E436E` |
| Subtle border/separator | `#323438` |
| Secondary section headings | Start at `#8A8D93`; verify actual background contrast |

The existing charcoal values need precise surface ownership, not another color-only
pass. Reconcile their semantic roles centrally, updating contrast tests and
documentation together; do not mix old and target roles screen by screen. Keep
distinct semantic tokens for text, disabled content, success/warning/error, diff additions/removals,
and keyboard focus. Selection and focus must remain distinguishable. Verify
contrast against the actual background, including blended overlays.

Use continuous 1dp separators with one owner per pane boundary. Pane surfaces are
square and flat, without elevation or heavy gutters. Keep wider invisible resize
targets behind splitter lines. Reserve 4dp–6dp corners for contained controls,
inputs, dialogs, or popups; section grouping is not a reason to add a rounded card.

## Typography and density

- Align spacing to a 4dp grid, usually using 4dp–8dp gaps within dense controls.
  Use larger spacing deliberately between sections, not arbitrary padding on
  every nested container.
- Use 11sp–12sp micro-labels for secondary IDE chrome. Keep primary content
  readable, source/diffs monospaced, and headings restrained but distinct.
- Use 12sp–13sp body text with explicit 18sp–20sp line height, 12sp breadcrumbs,
  and semibold or short uppercase section headings. Default content insets are
  8dp; avoid nested 16dp–20dp padding. Default headers/actions are 28dp–32dp and
  tree/list rows 24dp–28dp, growing with text scale rather than clipping.
- Use explicit, compact line heights without clipping at increased text scale.
  These sizes are logical Compose units, not fixed physical pixels.
- Use small status badges/chips where state needs emphasis, and quiet text where
  it is secondary. Preserve textual state labels; never depend on color alone.
- Keep controls compact without sacrificing keyboard navigation, focus visibility,
  accessible names, or usable hit targets. Do not shrink text to hide overflow.

## Shell and specialized components

Keep a stable multi-pane shell: activity/navigation rail, primary workspace, and
context inspector, with collapsible tool windows and an integrated status strip.
Preserve the existing drawers below 1000dp and resizable docked panes above it.
Do not replace this structure with a stack of full-width cards.

- Activity rail: consistent line icons and labels, quiet hover, and one narrow
  active indicator/pill; inactive items must not look like primary buttons.
- Tool windows: explicit compact headers, selected tabs, subtle separators, and
  discoverable collapse/reopen controls. Put actions inside their owning header:
  especially Start/Pause/Resume/Cancel, not in a separate Run controls card. Use
  named/tooltipped icons or short text toolbar actions. Trailing actions must not
  toggle the adjacent disclosure. Keep Apply/Undo explicitly labeled and guarded.
- Tabs and breadcrumbs: active background plus a 2dp accent underline, with a
  separate keyboard-focus treatment. Use subtle folder/file/symbol icons in real
  path segments. Do not turn illustrative multiple files into fake or live tabs.
- Toolbar/titlebar: visually integrated project identity, command search, and
  essential state. Group secondary utilities in menus rather than oversized
  outlined controls. A custom integrated titlebar is a future design target;
  evaluate native window dragging, controls, resizing, shortcuts, and accessibility
  before replacing platform decorations.
- Source/review: custom gutters and compact editor chrome; inline diff chunks use
  restrained green/red fills plus textual markers. Keep source/diff selectable
  and read-only, and show only actual candidate evidence.
- Status bar: compact contextual information chips and quiet separators, backed
  by real state. Daemon connectivity must not imply model/provider connectivity.

Keep chrome, workflow actions, status badges, and content surfaces visually
distinct. Reuse the shared theme and components (`DesktopTheme.kt`,
`ChromeControls.kt`, and `DesktopIcons.kt`) instead of introducing a second,
competing theme or copying styles into each workspace.

## Kotlin/Compose implementation choices

Do not rely on stock Material 3, Material, or Swing appearance. Material primitives
may remain implementation infrastructure, but colors, typography, shapes, borders,
and interaction states must come from the dedicated IDE design system.

The active plan prioritizes standalone JetBrains Jewel (`org.jetbrains.jewel`) and
explicitly includes necessary compatible Kotlin/Compose/wrapper/JBR migration.
Verify a published release, runtime distribution, accessibility, and test harness
before production adoption. The old Task 154 no-upgrade boundary is historical,
not a reason to defer this sequence. Adding these guidelines does not upgrade
dependencies.

Use one semantic design system mapped into Jewel, not parallel Material and Jewel
themes. Keep small app-owned components for source/diff or genuinely specialized
behavior. If a demonstrated integration blocker requires the user's unified-token
alternative, document evidence and request direction before changing the plan.
Remove replaced implementations and preserve workflow/state boundaries.

## Visual acceptance

For each substantive visual change, compare rendered production components with
the supplied mock; compilation and state tests alone do not establish visual quality.
Review the wide layout, both sides of the 1000dp breakpoint, narrower windows,
and enlarged text. Check clipping, alignment, density, interaction states, and
surface hierarchy, including empty, loading, error, and populated states affected
by the change.

Use [VISUAL_REVIEW.md](VISUAL_REVIEW.md) to reproduce the explicitly labeled fixture
renders and [KEYBOARD_SMOKE_CHECKLIST.md](KEYBOARD_SMOKE_CHECKLIST.md) for native
checks. Distinguish offscreen rendering evidence from native-window verification.

Visual polish must not introduce automatic source writes, extra provider requests,
unguarded Apply/Undo, or fake functionality. Unsupported controls stay clearly
marked Preview and local-only. Preserve one project, one file, one symbol, and
explicit candidate review before applying changes.
