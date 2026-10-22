# Mini-Orca TUI design guidelines

Design around the developer's next action. Keep the main content and primary
action prominent; disclose supporting metadata and explanations on demand.
Routine actions execute from their keyboard shortcuts or colon commands without
redundant dialogs. Palette, typography, navigation and layout may evolve as needed.

## Interaction and accessibility

Keep actions, current state, errors and recovery paths clear. Use consistent
keybindings, visible selection indicators (`▸`) and explicit focus in tables and
command prompts. Color supplements text labels and badges (`[CRITICAL]`,
`[TRUSTED]`, `[SURGICAL]`); never rely on color alone. Show progress while
requests or jobs run and prevent duplicate submission.

Preserve access to findings, explanations and diagnostics. Long paths and narrow
terminal viewports must not hide actions or corrupt layout. Model-authored text
remains inert; source and composed diffs stay read-only in review panes.

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
`Tab` / `Shift+Tab` switch these sections without requests or mutations. Preserve
full content through scrolling. Avoid repeated workspace statistics, model routing
and settings panels outside their own pages. Diff explanations are initially
collapsed and remain reachable with `e`.

Inspecting a failed task never retries it. Starting a second task in an occupied
write scope requires finishing or discarding the existing task. Worktree refresh
is passive; cleanup is explicit and must wait for the daemon's removal receipt.
Keep integration, merge conflicts and cleanup warnings distinct. External merge
resolution must remain recoverable through refresh and guarded cleanup.
Updating the index after external changes is a separate explicit action; it stops
outdated destination checks and workflows and clears execution trust.

## Verification

Use deterministic Go tests (`go test -race ./internal/tui/... ./cmd/tui/...`)
across all 5 hubs, sub-views, palettes and terminal dimensions. Exercise
loading, empty, populated and failure states, keyboard navigation, colon
commands, action reachability and late-response identity guards.
