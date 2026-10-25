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

Give each workspace area a consistent accent and distinguish navigation, content
and selected rows with readable surfaces. The dashboard presents recorded counts,
selectable activities and a next-step panel when space permits. In short terminals,
prioritize its title and all four activity routes before supporting detail. `n`
opens an editable task prompt on the dashboard and Tasks page; `r` runs or resumes
analysis on the dashboard, and `:reanalyze` starts fresh analysis.

## Interaction and accessibility

Keep actions, current state, errors and recovery paths clear. Use consistent
keybindings, visible selection indicators (`▸`) and explicit focus in tables and
command prompts. Color supplements text labels and badges (`[CRITICAL]`,
`[TRUSTED]`, `[SURGICAL]`); never rely on color alone. Show progress while
requests or jobs run and prevent duplicate submission.

Preserve access to findings, explanations and diagnostics. Long paths and narrow
terminal viewports must not hide actions or corrupt layout. Model-authored text
remains inert; source and composed diffs stay read-only in review panes.
Keep text contrast at least 4.5:1 on the page, panel and selection backgrounds in
each palette. Nested text styles must restore the surrounding surface when they end.

At 100 columns and 20 rows or above, use the sidebar; below that, keep numbered
workspace tabs and the active view visible. Fit the real terminal dimensions, including
when showing a resize prompt below 52 columns or 16 rows. Small diff panes must
retain visible code, and split diffs must retain both sides. Palettes supply their
own backgrounds so the Paper palette remains readable in dark terminals too.

The command menu opens with `:` or `Ctrl+K` and supports search, completion,
editable arguments and session command history. Keep its input above the results
and the selected command's explanation visible, including in compact terminals.
Browsing or completing a command is passive. Enter explicitly runs the
chosen command; commands requiring arguments stay in the editor until supplied.
Pasted text is inert outside that editor. Keep existing guards on every command
route. Opening a project and planning a custom task have visible prompt shortcuts.
`Ctrl+K` preserves an already-open input draft and cannot bypass a confirmation.

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
Investigation is an explicit action (`i` / `:investigate`) using the Function
profile, with remote-provider confirmation when required. Keep it asynchronous
and cancelable (`x` / `:cancel-investigation`), retaining the draft on failure or
cancellation. Show pending approach, quoted evidence, assumptions, suggested
scope and criteria in the existing plan sections, labeled as model suggestions.
Acceptance (`a` / `:accept-investigation`) updates the editable draft and clears
prior approval; it does not generate code. Suggested paths require explicit scope
editing. Discard results after project or draft replacement, and keep navigation
and command-menu browsing passive. Remove accepted citations to paths removed
from scope; replanning against the current base clears captured citations so
old source evidence cannot block a fresh investigation.
Automatic repair is opt-in. Failed checks may suggest additional scope, but cannot
silently expand an approved plan. Stage retries must state which work is reused.
Stage details show the model budget alongside elapsed time. Testing generates
test files before verification; missing check results must not imply tests have
run. Busy stages do not advertise a duplicate retry. Timeout recovery identifies
the relevant model or check limit, and interrupted checks do not trigger code repair.

Mission plans share Work's plan surface. `:missions` reads local history;
`:mission-scope main|task` explicitly selects the checkout. Enter inspects a
saved plan and its binding freshness. Keep current and stored revisions,
dependency/integration order, captured paths, child allocations and review state
available through Overview, Children, Scope and Budget sections. History source
freshness is unchecked until detail inspection. Preserve history selection by ID.
`:plan` returns to the ordinary task plan without losing either draft.

`:mission-new <goal>` creates a local draft. `:mission-edit` edits goal, files,
integration order, or the selected child's title, approach, criteria, writes,
reads and dependencies (`depends`). Lists use semicolons. `:mission-child <id>`
selects or adds a child; `:mission-remove <id>` removes it locally, leaving any
dependent references for explicit correction. `:mission-budget
mission|planning|child <field> <integer>` edits the displayed allocations within
their effective ceilings. These edits are passive. `e` opens the relevant field
editor, and `j/k` selects children in the Children section. Saving (`s` /
`:mission-save`) validates the complete graph and clears previous reviews and
approval. Unsaved edits cannot reuse approval of the saved plan.

Decomposition (`g` / `:mission-plan`) is a separate confirmed, asynchronous model
action. Disclose the checkout, current catalog, goal, every captured path/hash
or planned absent path, applicable instruction sharing, Function model and
destination, and both the planning budget and future mission allocation.
Recheck the captured draft and settings on confirmation and queued dispatch;
refresh local provider metadata before transport. Prevent duplicate submission.
`x` / `:mission-cancel` stops owned transport and retains the draft. Failures or
cancellation require local history refresh before retrying, because saving may
have completed before the receipt was lost. Reject late/replaced results, and
clear private mission state when its project or task checkout changes.
An inspected execution retains its owner when opening an exactly owned nested
child. An owner revision change invalidates action consent and requires explicit
refresh; it does not discard the retained mission or adopt unrelated source changes.

Review (`r` / `:mission-review`) shows the selected concrete child's approach,
criteria, paths, dependencies and budget in a scrollable confirmation bound to
the exact saved plan hash and catalog revision. Final approval (`a` /
`:mission-approve`) requires all children reviewed and a separate confirmation
of that hash. These decisions start no child execution and grant no Apply
authority. History, section navigation, command completion and local editing
never invoke providers or checks. Mission controls replace ordinary task
generation shortcuts while the mission view is active; `:mission-draft` returns
from history to the retained draft.

Enter on an approved plan or `:mission-execution` inspects execution in the same
four sections. `:mission-draft` returns to the retained plan. Keep navigation and
refresh passive, preserve local edits, selected child and scroll as progress arrives,
and show unavailable progress without replacing the last receipt. Local event
subscriptions reconnect with their acknowledged cursor, use bounded passive reads
for lease/effect changes, and stop on navigation, pending mutations or scope change.

Preparation (`p` / `:mission-prepare`) confirms the approved plan and clean Git
base without executing models or checks. Starting (`g` / `:mission-start`) confirms
the actual configured Creation, Testing and Review profiles, provider destinations,
shared context, check commands and fixed allocations for ready children. Check
trust (`t` / `:mission-trust`) is a separate confirmation for the exact owner;
recording it runs no commands. Pause (`P` / `:mission-pause`) interrupts the active
wave; Stop (`X` / `:mission-stop`) permanently cancels the mission. Neither control
resets consumed child allocations. Show unstarted allocations separately from
measured model usage, and recorded elapsed charges separately from active reservations.

Enter or `:mission-proposal` opens the selected owned child in the ordinary diff
inspector. Every changed file still needs `w` review and explicit Apply/commit.
Committing returns to the mission for `a` / `:mission-integrate`, bound to its
approved order, exact proposal, commit and current destination. A task-scoped
integration updates that task's owner and never replaces the top-level project.
Invalidate check trust and require current command/settings inspection after a
base advance. An older-base proposal uses `v` / `:mission-revalidate` to create a
fresh candidate and run consented checks without model calls or inherited approval.

Show failed/interrupted children, candidate generations, protected proposal
references, Git conflicts and cleanup states in the child detail. `:mission-candidate
0|1|2|3` inspects retained generations; `0` is the original proposal.
`:mission-model-history` opens the original child's execution as historical
evidence. Neither action promotes that generation into the selected candidate.
Independent model retries cannot reset a mission child's fixed allocation.

Recovery (`c`) selects the applicable explicit control: `:mission-reconcile` for
an abandoned wave, `:mission-recover-integration` for a pending Git outcome, or
`:mission-recover-revalidation` for an interrupted candidate check. Active owner
leases remain visible and cannot be stolen. Recovery never repeats model calls,
checks or merges. `:mission-cleanup` retries only a proven integrated child's
checkout/branch cleanup. After transport cancellation, `r` / `:mission-execution`
reads the same owner's current revision before inspecting retained receipts;
uncertain outcomes never become success or trigger automatic replay.

Task stages follow recorded node identities, including separate generation and
verification and parallel specialist reviews. Selection and scroll remain stable
as progress arrives. Unknown nodes remain inspectable, and interrupted or missing
evidence never becomes success. Native execution completion does not approve a
proposal. Older combined testing summaries are labeled Testing; unavailable native
history falls back to those summaries with an explicit availability message.
Inspection reads bounded, source-free progress for the exact project revision,
checkout and workflow. Private model text requires a separate explicit disclosure.

Use Left/Right in task detail for Overview, Timeline, Evidence, Usage and Recovery;
Tab continues to change workspace views. Timeline lists invocation IDs. Explicit
`:run-output <ID>` opens bounded private output and tool argument/result records
in Evidence; `:hide-output` closes it. Sanitize terminal control bytes and label
truncation. Updates preserve selected stages and scroll instead of following text.
Progress reconnects with the last acknowledged cursor and stops when inspection
ends or its project, checkout or workflow changes. A private disclosure refreshes
only while open. `:stream-output on|off` controls streaming for new workflows and
retries through the existing provider capability and consent checks.

Recovery shows the exact pending proposal scope, hashes and destination.
`:run-input approve|reject|cancel` records a guarded decision; per-file diff review
and Apply remain separate. `:run-cancel` cancels an active native root. Task pause
and resume are unavailable; use cancellation and a guarded stage retry. Analysis
retains its own pause/resume controls. Usage displays the shared run budget once,
with unavailable token measurements explicitly identified.
Timeline also shows bounded local spans shared by retries and child work, with
parent identities, execution outcomes and measured durations. Label omitted and
unfinished spans; span completion does not imply a passing check or approval.
Usage distinguishes reported tokens from conservative charges and shows the
version/currency of a configured price table. If measurements or rates are
incomplete, show an unknown total and a separately labeled known subtotal.
Cost remains an estimate, not an invoice. These panels stay passive and fit the
minimum terminal size in every palette using the existing detail scroller.

`:run-export` opens a scrollable confirmation for the current retained snapshot.
Show the checkout and current/stored revisions, invocation, exact destination,
destination fingerprint and snapshot hash, plus finished, unfinished and omitted
span counts. Browsing or opening this confirmation sends nothing. Revalidate the
captured scope, snapshot and destination both on confirmation and before queued
dispatch. Receipts distinguish accepted, partially rejected, warning and unknown
delivery; unknown acceptance is never displayed as zero. Ignore late receipts
for a replaced request or checkout. Exports do not grant proposal approval or
Apply authority, and no export retries automatically.

Long detail content scrolls with Page Up/Down; long diff lines scroll horizontally.
Keep full-proposal review distinct from comparison against the previous review.
Mark changed files individually, invalidate progress when the proposal changes,
and show Apply's commit destination. Worktree Apply commits the reviewed proposal
and moves it from Tasks to Worktrees as `TO_MERGE`; it never merges automatically.
Withhold linked issues from the issue lists while awaiting that decision. Explicit
Merge marks them Done and removes the worktree and its branch. Reject returns
them to TODO and retains the unmerged branch. Model rationale and criterion references
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

Keep Models, Roles & Budgets, and Skills as separate Settings lists. Show the
selected checkout path, settings version and configured/overridden/stale state.
The main checkout and selected task worktree have separate overrides. Navigation
and selection remain passive; explicit refresh reads only local settings.
Budget and role edits and skill enablement require a scoped confirmation, affect
future runs, and preserve active runs' captured settings. Skill validation reports
captured metadata and hashes without execution or tool grants. Required tools
are declarations; availability is checked for each invocation. Preserve the
selected skill by directory across refreshes. Canceling a save leaves its outcome
unknown until refresh. Display stale and unavailable states without inventing
defaults. Provider confirmation identifies selected workflow models, destinations
and captured paths; changed routing, settings or task input invalidates consent.

Connections uses one compact list with explicit MCP, OpenAPI, A2A outbound and
A2A inbound labels. Selected details show the checkout, destination, credential
references, permissions and expiry without displaying grant tokens. Keep failed
requests and unavailable inventories distinct from empty configuration, expired
grants and confirmed disconnection. Refresh reads local metadata. Discovery and
incoming access require scoped confirmation; cancellation leaves the outcome
unavailable until refresh. OpenAPI schema readiness does not imply endpoint
availability. Preserve selection by kind and ID, including partial refreshes.
When Settings exceeds the sidebar height, window its subviews around the active
view and retain all hub shortcuts and all three model summaries.

Selecting up to two MCP/API connections for a draft does not start an
investigation or share source. Before dispatch, separately confirm provider and
tool destinations with the captured plan scope, and invalidate changed grants,
inventories, routing or draft input. Show the selected connections on the plan.
Incoming access and outbound remote agents remain separate capabilities; neither
grants local Apply, merge or project execution permission.

External Work keeps API actions, retained research reports and remote advisory
tasks in one scoped history. Local refresh and private receipt inspection are
passive. API dispatch requires inspection of the complete persisted request and
a separate confirmation bound to its current version and hash. Uncertain delivery
never offers retry; reconciliation records an externally checked user observation.
Cancellation or failed control transport requires local history refresh before
another action. Preserve selection by receipt kind/ID and healthy history groups
when another group is unavailable.

Research and delegation capture the current plan's indexed source hashes before
confirmation. List every shared path/hash and explicitly identify planned paths
without indexed content as unshared. An entirely unavailable source selection
requires correction or an explicit `--question-only` choice. Confirm the provider
or remote destination, search/delegation intent and source sharing; changed plans,
models, grants, cards or checkout invalidate pending consent. Keep remote usage
labeled as remotely reported and output as advisory, including partial artifacts.
Offer task refresh, stream reconnection, input, cancellation and reconciliation
only when the current receipt and connection permit them. A remote status refresh
is a separate confirmed network action from refreshing local history.

Open a retained research report only on Enter or `:report-open`. Use its guarded
loopback URL and a system browser argument vector, preserving the HTML report's
citations and original search suggestions. The terminal shows report metadata;
navigation does not launch a browser or contact a provider.

Artifacts lives in Code; Memory sits beside Lessons in Settings. Both share an
explicit main-checkout or task-worktree scope. Lists show local metadata, versions,
ownership, source references, freshness and retention without reading payloads.
Enter explicitly discloses retained text in the scrollable read-only details;
binary artifacts remain metadata. Mark bounded previews as truncated. Research
artifacts link to the cited HTML report and open it only on an explicit action.

Artifact export and deletion each require a confirmation naming the checkout,
exact version and hash. Show the actual private export path/hash from its receipt.
Protected recovery evidence and unfinished-run references cannot be deleted.
Pagination stays bound to a catalog snapshot; changed catalogs require refresh.
These pages never inherit proposal Apply/Undo shortcuts.

Create/edit advice by capturing current plan, open-file or retained-record source
hashes before confirmation. Show uncaptured planned files explicitly; retain at
least one current source. Edits create a candidate again. Accept only inspected,
current, unexpired candidates; rejecting a candidate and forgetting accepted
advice are separate confirmed actions. Retained proposal lessons and check outcomes
must derive their text through the existing proposal authority owner. Display
accepted memory as advisory context, never permission or current verification.
Explicit source links verify checkout and content identity and remain read-only.

Library requests own cancellation and reject changed scope or record identity.
Preserve healthy inventory when another group fails and keep selection by record
ID. Canceled or failed mutations require refresh before another action; exported
copies may already exist if transport lost the receipt. Hide private previews on
request, and never claim an unavailable inventory is empty.

Work / Media (`:media`) groups Attachments, Live sessions and Outputs without
adding a workspace. `:media-scope main|task` selects the checkout; `a/l/o` changes
the section. Refresh (`r`) reads local metadata. It neither contacts a provider
nor reads files from a guessed path. An unavailable catalog stays unavailable;
retained rows are labeled as the last known state.

Import (`n` / `:media-import <path>`) captures at most 512 KiB from an explicitly
selected regular file inside that checkout, rejecting symlinks and path escapes.
Review the captured name, MIME, byte count and hash before private local import.
PNG, JPEG, PDF, supported WAV and raw 16 kHz PCM have no implicit conversion.
Space selects up to four immutable imported versions totaling at most 1 MiB.
Ask (`q` / `:media-ask <question>`) separately confirms the exact question,
versions, checkout, settings and Function provider destination. Answers remain
advisory. Enter explicitly discloses private retained text; Esc hides it.
Binary/audio outputs expose metadata and a confirmed private export (`e`), with
no automatic playback. Older outputs remain available in Code / Artifacts.

`:live-open` confirms the configured live destination, conservative budget and
bounded lifetime. Missing `agent_runtime.live_model` or an unsupported Function
profile explains the configuration required. Inspect a session with Enter before
acting. `s` / `:live-send <text>` confirms each text input; `:live-send-media`
confirms exactly one selected JPEG or raw PCM version of at most 64 KiB.
`:live-interrupt` and `:live-close` are explicit controls. Reconnect is a separate
confirmation using the remaining lifetime and budget; it never replays input.
Show acknowledged progress, reported partial usage and conservative charges as
distinct values. Transcript/audio artifacts appear after a connection ends.

The visible session uses a cancelable passive metadata watcher. Leaving Media
stops that watcher while the daemon retains its bounded session owner; other
pages show a reminder to return to Media and explicitly close it. Cancel (`x`)
stops the pending local request, retaining its draft. A failed or canceled
operation may have a retained outcome and requires refresh before another
mutation; close remains available. Confirmation and dispatch bind the exact
checkout, selection, settings and control revision. Replaced or late responses
cannot update a different view or authorize another request.

Inspecting a failed task never retries it. Failed tasks remain restorable in Tasks
with their check errors, even when the worktree has no new commits. Equal Git HEADs
alone must not label such a task integrated. Starting a second task in an occupied
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
