# Mini-Orca TUI design guidelines

Design around the developer's next action. Keep the main content and primary
action prominent; disclose supporting metadata and explanations on demand.
Routine actions execute from their keyboard shortcuts or colon commands without
redundant dialogs. Palette, typography, navigation and layout may evolve as needed.

The workspace uses five areas: **Project**, **Work**, **Code**, **Docs** and
**Settings**. The dashboard links to issues, tasks, analysis and checks without
starting work. Keep project identity, connection and Git state visible above the
content; keep contextual actions and command access below it. Use short page
titles, restrained borders and explicit text states. Counts summarize recorded
data; an empty list does not prove the project is clean.

## Interaction and accessibility

Keep actions, current state, errors and recovery paths clear. Use consistent
keybindings, visible selection indicators (`▸`) and explicit focus in tables and
command prompts. Color supplements text labels and badges (`[CRITICAL]`,
`[TRUSTED]`, `[SURGICAL]`); never rely on color alone. Show progress while
requests or jobs run and prevent duplicate submission.

Preserve access to findings, explanations and diagnostics. Long paths and narrow
terminal viewports must not hide actions or corrupt layout. Model-authored text
remains inert; source and composed diffs stay read-only in review panes.

At 100 columns and 20 rows or above, use the sidebar; below that, keep numbered
workspace tabs and the active view visible. Fit the real terminal dimensions, including
when showing a resize prompt below 52 columns or 16 rows. Small diff panes must
retain visible code, and split diffs must retain both sides. Palettes supply their
own backgrounds so the Paper palette remains readable in dark terminals too.

The colon menu supports search, completion, editable arguments and session command
history. Browsing or completing a command is passive. Enter explicitly runs the
chosen command; commands requiring arguments stay in the editor until supplied.
Pasted text is inert outside that editor. Keep existing guards on every command
route. Opening a project and planning a custom task have visible prompt shortcuts.

Use Page Up/Down for overflowing pages, diagnostics, help and dialogs; Home/End
reaches the beginning/end. Keep confirmation actions visible while its message
scrolls. Error overlays expose recovery and command access and consume shortcuts
that would otherwise act on the obscured page. Source views support paging and
horizontal panning without exposing diff-review actions.

## Product behavior

Navigation, disclosures and previews do not dispatch providers, execute project
code or write source. Keep provider consent and execution trust tied to the
relevant action and scope. Explicit Apply/Undo and stale-evidence checks remain
enforced. Never invent project facts or present suggestions as verified results.
Unknown, unavailable, stale, partial, failed and canceled states retain meaning.

Task selection opens an editable plan without generation. Make approach, evidence,
write/read scope, criteria and non-goals inspectable before explicit approval.
Automatic repair is opt-in. Failed checks may suggest additional scope, but cannot
silently expand an approved plan. Stage retries must state which work is reused.

Long detail content scrolls with Page Up/Down; long diff lines scroll horizontally.
Keep full-proposal review distinct from comparison against the previous review.
Mark changed files individually, invalidate progress when the proposal changes,
and show Apply's commit/merge destination. Model rationale and criterion references
are advisory; distinguish reproduced failures, tests already passing, and checks
that could not execute. Progress timings are diagnostics, not measured speedups.

Keep lists focused on selection and next actions. Task stages, feature evidence,
and worktree recovery belong in detail views. Plans separate approach, evidence,
scope and acceptance; issue details separate problem, evidence and solution.
`Tab` / `Shift+Tab` always switch sidebar views, including from task plans and
detail pages. Left/Right (or `h`/`l`) switch detail sections without requests or
mutations. Preserve full content through scrolling. Keep a compact model summary
in the sidebar at every supported sidebar height. Detailed routing, settings and
workspace statistics belong on their own pages. Diff explanations are initially
collapsed and remain reachable with `e`.

Model lists show assignments and access boundaries; full parameters and preset
routing belong to the selected item's scrollable details. Worktree details show
the recovery action for the current state rather than every possible workflow.

Inspecting a failed task never retries it. Starting a second task in an occupied
write scope requires finishing or discarding the existing task. Worktree refresh
is passive; cleanup is explicit and must wait for the daemon's removal receipt.
Keep integration, merge conflicts and cleanup warnings distinct. External merge
resolution must remain recoverable through refresh and guarded cleanup.
Updating the index after external changes is a separate explicit action; it stops
outdated destination checks and workflows. Remember project trust across reopening
and reindexing, rebinding execution authority to current validated sources. Keep
revocation available and report failures to save or restore trust.

## Verification

Use deterministic Go tests (`go test -race ./internal/tui/... ./cmd/tui/...`)
across all 5 hubs, sub-views, palettes and terminal dimensions. Exercise
loading, empty, populated and failure states, keyboard navigation, colon
commands, action reachability and late-response identity guards.
