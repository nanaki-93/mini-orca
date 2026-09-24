# Mini-Orca UI guidelines

Use the immutable [Overview](../design/ui-mocks/ide-reference-2026-09-16/01-overview.png),
[Performance](../design/ui-mocks/ide-reference-2026-09-16/02-performance.png),
[Bugs](../design/ui-mocks/ide-reference-2026-09-16/03-bugs.png),
[Security](../design/ui-mocks/ide-reference-2026-09-16/04-security.png) and
[Source](../design/ui-mocks/ide-reference-2026-09-16/05-source.png) captures as
the hierarchy reference. Build a dense, coherent developer tool with source,
context and evidence. [PLAN.md](../PLAN.md) owns implementation priorities; the
[dark UI direction](../docs/dark-ui/README.md) records the underlying workspace
ownership. Sample facts, direct-fix controls, text navigation and unsupported
controls in the images do not appear in production.

## Self-explanatory UI and copy

For every new or changed feature, make the task, current state and next action
clear through layout, grouping, familiar controls and direct labels. Simplify an
unclear interaction before adding instructions to explain it.

- **Titles are optional.** Use a short title when it identifies a pane, dialog or
  distinct content group. Omit headings that repeat the active tab, nearby label
  or the same information at another level. Do not add subtitles or introductory
  paragraphs by default.
- **Actions say what they do.** Prefer familiar verb/object labels such as
  **Open project**, **Start analysis** and **New function**. Keep names consistent
  across entry points; avoid generic labels when the outcome is unclear. Do not
  add a sentence telling users to click an already clear button.
- **Fields remain identifiable.** Keep concise persistent labels. Add a hint or
  example only for a non-obvious format or constraint; placeholders must not be
  the only label. Put validation beside the affected field when it is relevant.
- **Show state where it matters.** Use a compact status and the relevant action.
  Empty states need only the missing context and a useful next step when one
  exists; failures and blocked actions need a specific reason and recovery when
  available. Avoid repeating the same message in a banner, header and body.
- **Reveal optional detail on demand.** Put metadata, provenance detail, logs and
  longer explanations in an existing details view or labeled disclosure. Tooltips
  can clarify secondary controls or show shortcuts; essential instructions,
  errors and decisions must not depend on hover or opening optional help.
- **Keep decision-critical information visible.** Show the relevant target,
  consequence, consent and trust requirements before an action. Preserve distinct
  result states, evidence types and uncertainty. Do not add generic warnings or
  implementation details that do not affect the user's decision.
- **Keep controls accessible.** Retain visible labels for unfamiliar actions,
  accessible names/states for every control and visible keyboard focus. Icon-only
  controls are appropriate when familiar and unambiguous in context, with names
  available on hover/focus. Color alone cannot communicate meaning.
- **Preserve useful content.** Requested code explanations, findings, evidence
  and diagnostics are task content. Keep their meaning and full content available;
  remove repetitive interface scaffolding around them, without inventing a shorter
  result or silently discarding information.

Illustrative copy choices for future changes:

| Situation | Preferred presentation |
| --- | --- |
| Analysis already identified by navigation | **Start analysis** and current status; omit another Analysis heading and “Click Start analysis to begin.” |
| No project open | **Open project**; add a short empty-state label only if the surrounding view does not explain what is missing. |
| Apply blocked by an edited draft | A concise stale-validation reason beside the relevant action; optional diagnostics in Details. |

## One design system

Jewel standalone is already adopted. Use `DesktopTheme.kt`, `ChromeControls.kt`
and `DesktopIcons.kt`; do not create another theme or copy styles into each pane.
Small app-owned source/diff components remain appropriate. Low-level text/layout
primitives must use the shared semantic roles, not stock Material/Swing appearance.
Runtime and dependency setup lives in [README.md](README.md#runtime-and-build).

| Role | Tokyo Midnight target |
| --- | --- |
| Activity rail / outer chrome | `#171821` |
| Tool windows / sidebars / bottom panes | `#1D1E2B` |
| Editor / input canvas | `#1A1B26` |
| Section headers / overlay | `#303145` / `#232436` |
| Contained neutral controls / hover | `#303145` / `#34364B` |
| Selection edge / primary action | `#7DCFFF` / `#7AA2F7` |
| Information / explanation / running | `#7DCFFF` |
| Success / warning / failure | `#9ECE6A` / `#E0AF68` / `#FF8FA3` |
| Identity / selected surface / selected text | `#BB9AF7` / `#292E45` / `#C0CAF5` |
| Pane separator / control outline | `#9299BE` (1dp) |

Keep text, disabled, success/warning/error, diff and focus as separate semantic
roles. Use tinted fills and matching outlines for semantic actions and badges. Keep
primary action fills opaque in every interaction state so their dark labels retain
contrast. Verify actual backgrounds, including hover, press, selection and disabled
fills.
Normal meaningful text targets 4.5:1; focus/control indicators target 3:1. Current
measurements are in [UI_CONTRAST.md](UI_CONTRAST.md). Color never replaces labels.

## Geometry and hierarchy

- Keep structural panes without elevation, with a rounded outer perimeter. Use the
  shared 10dp corners for controls, inputs, file rows and tooltips; 14dp for
  sections and category/result surfaces; and 18dp for structural workspace panes,
  popups and dialogs.
  Use pill-shaped badges and progress tracks, and 4dp corners for small checkbox
  indicators. Clip child fills to the same shape as their containing surface.
  Keep selection strokes inset with rounded ends. Do not wrap ordinary sections
  in repeated decorative cards.
  Keep an 8dp frame inset beside the rail, along the trailing edge and above/below
  the workspace. Adjacent panes have one 8dp resize gutter with a centered handle;
  its hover/focus treatment remains distinct from selection. Avoid full-height
  divider strokes across the rounded perimeter.
- Align to a 4dp grid; normal content inset 8dp and internal gaps 4–8dp. Avoid
  nested 16–20dp padding. Headers/actions default to 28–32dp; rows to 24–28dp.
- Summary and Bugs, Performance and Security workspaces use 14sp body / 22sp line
  height, 16sp section headings and 13sp metadata. Their page inset is 24dp;
  result cards and details use 16dp padding. Controls retain 12–13sp body / 18–20sp
  line height; secondary chrome 11–12sp, section labels 12sp semibold, and
  breadcrumbs 12sp. Source/diff stay monospaced and readable. Grow at 125/150%
  text scale rather than shrinking the font to fit.
- Use headings only where they add orientation; keep labeled state. Headers own
  their actions, especially Start/Pause/Resume/Cancel. Trailing actions never toggle
  an adjacent disclosure.
  Use short labels/tooltips/accessibility names when an icon is ambiguous.
- Selection has a blue fill and accent edge, including the file tree; keyboard
  focus is independently visible. Focused buttons add a dark inner keyline so the
  light focus outline remains visible on bright actions. Tool-window and source
  tabs share that policy.
- Prefer a quiet page hierarchy: project identity and search lead the toolbar;
  an active run appears once as a compact strip; category summaries use neutral
  surfaces with small semantic accents; and Architecture sits beside Flows. Result
  pages use one unboxed heading, local filters beneath it, then a readable
  list/detail split. Source keeps Explorer, code and Context as resizable peers.
  These layout rules must not hide the source-safety, consent, trust, stale or
  evidence state needed for a decision.

## Shell and task flow

Keep the 48dp icon-only activity rail with hover labels and accessible names,
primary workspace, Editor-only Files/inspector
panes and integrated status bar. Keep this docked-pane composition at every
window size. The native window requests maximized placement at startup but remains
resizable; smaller windows may clip or fit awkwardly. Do not substitute drawers,
overlays or other viewport-specific layouts, and do not constrain the window with
a minimum size. Preserve saved pane widths across window resizing. The terminal spans beneath all
workspace panes with its own 8dp gap and rounded perimeter; the rail and status
bar share the continuous frame.
Native window controls remain native.

The expanded Terminal canvas has an 8dp side/bottom inset. Its Swing host cannot
inherit the Compose clipping shape, so the inset keeps native pixels inside the
rounded dock while preserving the existing PTY resize and focus owners.

The main toolbar has a 56dp minimum height. Its product mark leads directly into
project/branch context and then a left-aligned search control up to 420dp wide; do
not repeat the product wordmark there. The remaining space keeps separate labeled
analysis and daemon statuses at the trailing edge without turning passive state into
another outlined control. Keep the full-size status placement; text may clip at
reduced window sizes. Analysis labels come from the run owner: a completed run does
not by itself prove coverage is up to date.

Files uses real project-relative paths and one active file. Breadcrumbs expose
real path/symbol identity; do not invent navigation callbacks or tabs. Source and
composed diff remain selectable/read-only. Only the declaration/import draft edits.

Context explains the selected code, Assistant prepares the request/draft, and
Review shows the exact candidate with current validation/checks, Apply, receipt
and Undo. Use one next valid action and a concise blocked reason; expand technical
details on demand. Never duplicate daemon eligibility rules in a visual helper.

Selected declarations show a short description with explicit explanation and Refactor
actions in one compact view. Omit technical breakdowns, provenance and teaching copy.
Use the cached description when available; requesting or refreshing one stays explicit.
File context starts on Actions after navigation, with metadata and collapsed project
context in Details. Navigation must never request an explanation or prepare an edit.

Keep empty, loading, stale, failed, partial, canceled and unavailable states
explicit. Unknown metrics are not zero. Daemon connectivity is not provider
connectivity. No fabricated findings, project facts or scores in normal usage.

Model results use primary body text; where needed, headings use 13sp semibold
and labels use 12sp semibold. These styles do not require a heading or label on
every block. Keep the supported summary, severity/state and useful action visible;
place metadata and optional technical details behind a labeled disclosure. Use
the cyan result accent for explanation identity, not as a severity verdict.
Badges always include a text label and wrap rather than clipping their meaning.
Headers grow for long titles and larger fonts; do not shrink the text to fit.

Use `ModelResultContent` for freeform model prose. Its selectable text supports
paragraphs, simple emphasis, flat lists, inline code and triple-backtick code
fences. Unsupported or unfinished markup stays literal; links, HTML and images
are never activated. Formatting is bounded to 32,768 characters and 512 lines; larger
responses remain complete as plain text. An eight-line preview offers an explicit
keyboard-focusable **Show full response** control when it overflows. Expanding
retains all content, and a new response resets the preview. The containing pane
owns vertical scrolling; do not add a scroll area inside each result. Structured
failures and required next actions belong outside optional disclosures.

UI-01 removed the inert Preview utilities. Keep unsupported controls out of the
current interface. No navigation, disclosure,
preview or preset may send a provider request, execute code or mutate source.
Fresh AI requests require explicit scope-specific consent where applicable.

## Verification

Review each changed flow with optional help and details collapsed:

- Can a user identify the content, scope, current state and next action from the
  controls and layout, without reading introductory instructions?
- Does each title, subtitle, hint and sentence add information needed to act,
  decide or recover? Remove repetition; fix unclear interactions before adding copy.
- Are required labels, errors, consequences and consent still available at the
  point of action, including with keyboard focus and without relying on color?

Compare rendered production components to the references for every substantive
visual change. Check full-size views and, when relevant, reduced window sizes for
clipping, as well as large text, long paths/errors and affected empty/stale/populated
states. Check alignment, separators, keyboard focus and names/states; reduced
windows retain the fixed layout and are not expected to fit every action.

Use [component reproduction](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks) for fixture reproduction and the retained
[keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md) for native checks. Offscreen
Compose renders cannot prove OS focus, popup placement or screen-reader behavior.
Record those separately in [release acceptance](../docs/RELEASE_ACCEPTANCE.md).
