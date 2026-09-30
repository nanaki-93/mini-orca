# Desktop keyboard smoke checklist

Use a disposable Go project, a fake provider and isolated preferences. This is an
operator procedure. [UI guidelines](UI_DESIGN_GUIDELINES.md) cover interaction
and accessibility.

Launch, restore and resize the native window at 1600×1000, 1440×900, 1024×768,
800×650 and 1280×600 where the host permits. Check action reachability, header
wrapping and footer alignment. Repeat affected surfaces at 125% and 150% text;
record viewport, display density and text scale separately. Compare native and
component observations without treating offscreen captures as proof of OS focus,
popup placement or spoken screen-reader output. The native observations below
remain pending until performed and recorded on a host; offscreen fixture results
are not a substitute. Record each tested size/scale, input method and any
unavailable check separately rather than marking the whole checklist complete.

For changed flows, also apply the [UI review](UI_DESIGN_GUIDELINES.md#verification):
identify the task, state and next action. Verify that labels, blocked reasons
and required consent remain clear
with keyboard focus.

1. Before opening a project, verify Open project and Cmd/Ctrl+O work. Cancel the
   chooser and confirm the landing state remains. When entering a disposable path,
   confirm its native field is visible before operating it; record an offscreen
   computer-use failure as pending native evidence. Project shortcuts must have no
   action until a project opens. Restore must not call a provider or start a shell.
2. Verify the scrollable rail shows all six icon-and-label destinations in order:
   Summary, Analysis, Bugs, Performance, Security, Editor, then the separated
   Terminal, Commands and Models actions. At small heights and 150% text, scroll
   and keyboard-focus/reveal every entry; labels must not require hovering or be
   truncated. Check that selected and focused workspace entries remain distinct.
   With the workspace group focused, arrows move focus without selecting;
   Enter/Space selects once. Tab to each utility and activate it with Enter/Space;
   arrows/activation in the workspace group must not steal utility input.
   Cmd/Ctrl+1–4 select Summary, Analysis, Bugs and Editor. Cmd/Ctrl+Tab cycles all
   six workspaces. Commands offers View Performance results and View Security results;
   navigation preserves the selected file, draft and loaded results. Merely focusing
   rail entries or switching workspaces must not start a shell or request a model.
3. In Analysis, Start analysis opens a whole-project preview with file/stage scope,
   exclusions, cache use and request bounds. At 800×650 and 1280×600 with 150%
   text, scroll the preview body to the complete destinations and Security intent;
   Tab/Shift+Tab to each checkbox, cancellation and start decision. Confirm
   every required remote destination and explicit Security intent before starting;
   merely focusing or selecting a checkbox must not start the run. Admission is
   one operation; starting
   a result page must show the whole category and never launch another analysis.
   The daemon's dispatch bounds remain enforced without Run limits controls.
4. Verify Analysis contains progress, per-section states, failures and result
   links. Pause settles at a stage boundary, Cancel stops further requests, and
   Resume requires a fresh preview. After restart, retained progress is interrupted
   or paused without dispatch authority. Check partial, failed, unavailable,
   canceled and completed-empty evidence retain their distinct labels/counts.
5. From Summary's result cards or Analysis's Saved results category panels,
   open Bugs, Performance and Security with keyboard activation; use the six-entry
   workspace rail (or Cmd/Ctrl+Tab) to switch categories. Check the selected
   workspace indication and confirm navigation does not start analysis, a scan
   or a provider request; result pages have a **View analysis** action, not three
   category boxes. At widths on each side of the list/detail reflow boundary,
   search by title/path, use severity facets (including unknown), scroll a
   populated list, and select a row with Enter/Space. Resize across the boundary
   and back; check query, facet, selection and applicable list position survive.
   Keyboard-reveal and independently scroll the stacked list and detail, including
   long paths and evidence at 125% and 150% text. Verify visible focus, spoken
   names and selected/disabled states with an enabled screen reader. Select and
   copy the exact location and long diagnostics/evidence; verify text remains
   read-only and complete after disclosure and reflow. Repeat with completed-empty
   and filtered-no-match results; use **Clear filters** to recover loaded rows.
   With a saved-result read failure, check **Retry loading results** and retained
   rows/diagnostic; retry should read saved details without starting analysis.
   Unavailable, stale, partial and canceled results must keep distinct labels;
   unknown counts must not become zero. Inspect severity, full title, location,
   origin, message, and **Evidence and fix criteria** in Bugs. Row selection and
   disclosure must not open source, prepare a fix or triage. Verify **Open source**
   navigates to the exact indexed path/positive line without prefill, including a
   stale or taskless finding with a valid path; an absent indexed path explains
   why it cannot open. Return and verify **Prepare fix** is disabled with a
   specific visible reason for an ineligible task/source. For an eligible current
   exact Go declaration, activate it with keyboard and check Editor/Assistant
   shows the target and prefilled request without Send, checks or source write.
   With a different-target draft or Assistant input, try each action: cancel and
   Escape the discard dialog and check draft, input, evidence and target survive;
   confirm an unchanged intent and check only the chosen action proceeds. Change
   project, finding or input while confirmation is pending and check old approval
   cannot open or prepare the new target. Same-target source inspection should
   retain the draft. Check that failed source reads cannot prefill. Keep triage
   separate. Performance hypotheses never claim measured speedup; Security rule
   matches and model hypotheses remain distinct in their evidence. For Bugs
   execution and diagnostics, also perform the separate
   [Verified Go scan native review](#verified-go-scan-native-review) below.

   In Performance, keyboard-select a typed opportunity and scroll the detail
   independently of the result list. At 800×650 and 1280×600, 125%/150% text,
   and with **Report metadata** collapsed, reach and copy the full path, positive
   line (no `:0` for an absent line), observed pattern, qualitative impact,
   confidence disclaimer, recommendation, workload, trade-offs, verification,
   warnings and engineering insight. Expand metadata and copy a long value;
   confirm model prose is read-only and no content or action is clipped. Check
   completed-empty, no-match, retained-error, partial and stale rows without
   treating unknown counts as zero. Tab to **Open source** and **Prepare fix**;
   verify focus and enabled/disabled announcements. Open source on current and
   retained stale typed evidence with an indexed path: check the exact path and
   supplied positive line in read-only Editor without an Assistant prefill. With
   a missing/ambiguous indexed file, check the visible failure instead. Check
   blocked preparation reasons for stale, changed/missing hash, ambiguous or
   inexact declaration and unsupported source. For an exact current Go target,
   Prepare fix should prefill Assistant with the supplied hypothesis context but
   never Send, execute or write. Cancel and Escape each target-changing discard
   prompt with a draft and with Assistant input; confirm both remain intact.
   Confirm an unchanged intent, then change project/result/draft/input while a
   prompt is pending and verify the old approval cannot act. Same-file Open
   source must retain the draft. A failed source/symbol read must not prefill.

   Inspect a semantic Performance row separately: check its shared evidence and
   task-based blocked reason. Tab to its **Open source** control and activate it
   with Enter/Space; verify the indexed path and supplied positive line open in
   read-only Editor without preparing a request. With no indexed path, verify
   the control is disabled and its reason is visible. Reject missing, duplicate
   and wrong-project findings without navigating or preparing. Record this
   native source-action check as pending until performed; presenter and
   component tests alone cannot establish native behavior.

   Also perform the [benchmark discovery and admission native review](#benchmark-discovery-and-admission-native-review)
   below. It covers explicit read-only listing/refresh, keyboard selection,
   command/scope inspection and copying, project/revision trust, recovery and
   invalidation during admission. Record those native observations separately
   from fake-transport tests and offscreen fixtures; a measured candidate must
   not label the selected Performance hypothesis measured.

   In Security, use disposable saved source-rule and AI results (and, if
   available, a mismatched source/evidence pair). At compact width and 125%/150%
   text, Tab to **Review Security intent**, **Open source**, **Prepare fix** and
   **Report metadata** on a typed row. Check visible focus, keyboard activation,
   spoken names and expanded/disabled states with an enabled screen reader; verify
   that wrapped actions and their blocked reasons remain reachable with metadata
   collapsed. Check **Source rule** describes a pattern rather than a verified
   vulnerability, **Model hypothesis** remains unverified and an unknown pairing
   says **Evidence type unavailable**. Inspect a completed-empty result: the
   no-safety-assurance explanation must remain visible. For a failed, partial or
   unavailable report with no rows, read its reason without selecting a finding;
   check retained rows and saved-result read errors independently. Search/filter
   and select both typed and semantic rows with Enter/Space, including a long path;
   copy the complete path, positive range, condition, remediation, preconditions,
   verification idea and engineering insight. Expand/collapse **Report metadata**
   from the keyboard, copy long scope/hash/model values, and verify disclosure
   does not trigger source reads, scans, provider requests or writes.

   On a typed Security row, Tab to **Open source** and **Prepare fix**; test current
   and retained stale evidence with a valid indexed path. Open source should show
   the supplied positive line in read-only Editor without Assistant prefill; an
   invalid or ambiguous anchor should report a failure, not jump to a nearby
   declaration. Same-file inspection must retain the draft. With an unsupported,
   stale, changed-hash or inexact target, check Prepare fix is disabled with a
   readable reason and its state is announced. With a current exact Go declaration,
   Prepare fix should open an unsent Assistant request; check no checks or source
   write occurred. For each target-changing action with draft or Assistant input,
   inspect the discard dialog: Tab through **Keep draft**/**Discard draft** (or
   **Keep work**/**Discard work** for input alone), cancel using the keep action and
   Escape in separate attempts, and verify target, draft and input remain. After
   each dismissal, check *native focus* returns to the surviving Security action
   opener; reopen the dialog and confirm only an unchanged intent. Change the
   selection or input while approval is pending and verify old approval cannot
   act. A failed read must retain existing work and never prefill.

   Activate **Review Security intent** by keyboard and verify it only opens
   Analysis: no preview, scan or run. Then explicitly choose **Start analysis**
   or a valid **Resume → fresh preview**, inspect whole-project scope and
   destinations, and check the separate **Include AI Security review** checkbox
   and each required remote confirmation before admitting the run. Dismiss the
   admission dialog with Escape and check native focus returns to a surviving
   Analysis opener; no preview review or dismissal may dispatch a model request.
   Record native focus, screen-reader and copy observations as pending until
   performed; automated production renders and fake-call tests do not establish
   native behavior.

6. Use Cmd/Ctrl+P to choose a file and Cmd/Ctrl+Shift+O to choose a declaration.
   Source and diff must remain selectable/read-only. Relative paths disambiguate
   equal basenames. Source drag selects text without changing the draft target.
   At 125% and 150% text, select and copy source and Current/Candidate diff text,
   try typing to verify neither changes, and horizontally scroll the source and
   each diff column independently; vertical diff rows should remain synchronized.
   Resize while scrolled and confirm the selected diff mode and applicable scroll
   position persist. Copy a long path using its accessible text/local scrolling,
   without relying on hover.
7. Resize Editor across the measured wide/compact boundary and back, at 100%,
   125% and 150% text. Check visible Files, source/diff canvas and
   Context/Assistant/Review side by side when they fit and in a bounded vertical
   stack otherwise; hidden panes stay hidden. Keyboard-focus a file, Editor tab,
   draft action and right-tool tab on both sides: focus should stay on a surviving
   control, with its stacked pane scrolled into view. Focus a side splitter before
   stacking; verify focus moves to a surviving Editor control without activating
   it. Repeat while a dialog is open and while Terminal owns focus: neither should
   lose focus. Check selected file/right tab and draft text, caret and selection
   survive wide → compact → wide without a save or unsolicited work. Drag and
   arrow-resize a constrained splitter: the first delta must start at the displayed
   width, and only explicit input commits a new preference. Check source/diff
   viewport and scroll access. Activate rail Commands to open the actions palette;
   header search opens file search. With the window narrow and near a screen edge,
   open the file and commands palettes from keyboard and pointer, plus a dialog;
   inspect native placement, clipping, keyboard focus and Escape dismissal. Close
   and Escape each palette from its own opener (keyboard and pointer) and verify
   *native focus* returns to that surviving control, not just its region. Activate rail Models and footer counts separately:
   each opens the same configured model details, even when unavailable, without
   changing workspace or probing a provider. Close/Escape must return native focus
   to the corresponding rail or footer opener. Repeat after switching projects or
   closing one: focus must land in a valid region or landing, not a removed control.
   Check selected source after resize; Cmd/Ctrl+P remains available.
8. Open a Go file containing only `package main`. Select New function in the file
   header or Context; Assistant focuses the name field without a model request.
   New Go function and New Go type remain in Commands. Reject keywords, duplicate
   names and invalid identifiers before sending. With an existing draft, cancel
   a target-change discard prompt and verify the original draft is preserved.
9. Send an explicit request in Assistant. Check readable model headings, code,
   summary and expandable details. Edit only the declaration/import draft. Use
   Cmd/Ctrl+Shift+V to validate and Cmd/Ctrl+Shift+C for eligible trusted checks.
   Validation/check failures remain available in Review; request failures stay
   beside the matching Assistant request. Copy long diagnostics without clipping.
10. Review the exact diff, current validation and required checks. Apply names the
    file and declaration and is unavailable for stale evidence. Edit draft clears
    previous approval evidence. After explicit Apply/Undo, source refreshes and
    Undo is limited to the immediately preceding unchanged Apply.
    At reduced heights and 150% text, with optional help collapsed, scroll both
    evidence and decision areas: inspect complete target path/declaration, failed
    diagnostics, required checks, Apply scope and recovery action. Tab to the
    enabled action and activate it exactly once only with current evidence; with
    stale evidence, confirm blocked reason and no Apply. Verify confirmation and
    cancellation remain reachable by keyboard without triggering a check or source
    write from reflow. Expand Check details and Project context without dispatching
    a request.
    During a rerun, show Running even when the previous report passed.
11. Confirm Terminal and its shell tabs share one bottom bar. Explicitly activate
    rail Terminal, collapsed-dock Terminal or Ctrl+Shift+T: each opens/focuses the
    dock and may start a real shell in the project if none exists. Repeating rail
    activation must not create another session or collapse the dock. Use the
    expanded dock's Terminal control to collapse it; collapse does not close a tab.
    Use + to create independent shells, switch tabs, and × to close one without
    stopping others. Collapse/reopen, workspace changes and resizing preserve each
    shell's PID, history and scrollback. With an expanded dock, shrink the native
    window to a short height and back: verify the dock bar, tabs and collapse
    control remain reachable above the footer, the workspace can still be
    scrolled, and the preferred height returns when room allows. Resize the dock
    explicitly and verify real PTY rows/columns change without losing content or
    hiding the canvas; passive window reflow must not open, close or replace a
    session. Record session PID and PTY dimensions before/after separately from
    component-render evidence.
12. With terminal focus, verify typing, Unicode paste, selection/copy, shell history,
    Ctrl+C, scrolling and a disposable full-screen program. App shortcuts must not
    steal ordinary shell input. Ctrl+Shift+F12 returns to Editor; Cmd/Ctrl+P then
    opens the application palette. Verify focus returns to Editor without changing
    or dismissing the dock.
13. Change the selected temporary file from the shell. Return to Editor/Review:
    source refreshes and old draft/check/analysis evidence becomes stale. Reindex
    is explicit for added/removed/renamed files. Project switch requires closing an
    active shell; cancel preserves every tab. Closing a tab stops its children; +
    starts a new session. Closing the last tab leaves + available without restarting
    it. Application exit and confirmed project switching clean up all shells.
14. Inspect idle, starting, running, exited, failed, closed and cleanup-pending
    terminal labels. Failures remain visible and retry/close controls reachable.
    At reduced windows and large text, verify ordinary scrolling and access to long
    paths/errors, headings, badges, disclosure controls, row actions and status details;
    record any clipping or unreachable actions. Color
    must supplement text labels for selection, severity and meaningful lifecycle
    states.

## Benchmark discovery and admission native review

This is a procedure, not completed native evidence. Use the
[delivered Performance behavior](README.md#performance-hypotheses-and-benchmark-evidence)
as the expected contract. Use a disposable root Go module, isolated preferences
and a fake provider for any import or draft generation. Add a Go declaration to
edit and at least two `func BenchmarkName(b *testing.B)` benchmarks in a same-package
`*_test.go` file. For a long-catalog case, add at least 25 distinct benchmarks and
include a long benchmark name; repeat with a long Unicode package/target path.
Use only code and tests you trust, with no effects outside disposable data:
copied workspaces are not a security sandbox. Prepare an isolated declaration
draft in Editor and explicitly validate it; do not Apply during this procedure.

Repeat at the sizes, text scales and densities listed above, especially 800×650
and 1280×600 at 125%/150% text and on both sides of the result-browser reflow
boundary. Record actual runtime, viewport, density, text scale and input method.
Keep **Report metadata** and measurement details collapsed for required-information
checks. For request/late-response cases, use a recording proxy or controlled test
daemon if available, pointing the client at it with `MINI_ORCA_URL` as described
in the [desktop guide](README.md). Record the setup and response delays used;
there is no in-app delay/failure-injection control. Without controlled responses
or request evidence, mark the corresponding observations pending rather than
inferring safety from appearance. Ordinary automated tests remain fake-transport
or action-counter tests, without real project execution.

1. Without a draft, Tab to **Explore benchmark evidence**, activate with Enter
   and Space in separate attempts, and verify expanded state, visible focus,
   scrolling and **Benchmark comparison** reachability. Read the blocked reason
   and disabled List action. Repeat with an edited, validating, invalid or stale
   candidate where safely reproducible. These states must not request a catalog
   or auto-validate. Switch result/project and check disclosure resets. With a
   valid unchanged candidate, navigate away/back and select another finding:
   retain candidate evidence; navigation/disclosure must not discover or execute.
2. With the exact current draft validated, Tab to **List compatible benchmarks**
   and activate once. Observe only a guarded catalog GET under the
   [API contract](../docs/api-contract.md#live-routes): no trust POST, comparison
   POST, provider request or source write. **Listing · read-only discovery**
   must not imply project-code execution. Repeat activation during a delayed
   lookup: only one lookup should be admitted. On success, check no choice is
   selected. Tab/Shift+Tab through every choice, including the last of the long
   catalog; focus must be visible and revealed by scrolling without selecting
   or making requests. Activate a choice with Enter, then another with Space:
   each selects once, with distinct focused and Selected/Not selected states.
   Use a screen reader to check choice names and selection announcements; do
   not credit offscreen semantics as spoken output.
3. After selection, inspect and copy the complete benchmark name, project
   ID/revision, target path, validated draft revision, **Package working directory**
   and **Opaque scope guard (identity metadata)**. Repeat with root-level and
   nested targets: `.` means project root, not the opaque scope. Tab to **Copy
   argv** (accessible name **Copy selected benchmark argv**) and activate it;
   paste into a separate scratch document and compare every indexed JSON-quoted
   argument with the returned catalog, including the final argument. Check copy
   feedback and read-only command focus, then select/copy the scope and long
   path text. No dedicated scope-copy button is expected. Verify selection and
   copying across reflow, native focus indication and screen-reader Read-only
   state; typing must not edit argv or metadata. Clipboard observations require
   an actual paste, not just a copied-message assertion. With optional details
   collapsed, scroll to every command/scope value, execution warning, trust
   explanation, blocked reason and run/recovery action at constrained sizes.
4. Activate **Refresh compatible benchmarks** after selecting a choice. Check
   catalog/selection authority is cleared immediately and the replacement requires
   explicit selection again, even for the same benchmark. Exercise empty,
   unavailable, lookup-failed and invalidated discovery where safely available;
   read the current reason and keyboard-reach Refresh without a hidden optional
   disclosure. With a missing/stale candidate, Refresh stays blocked until source
   and draft are current and validated. A malformed name/scope/argv or foreign
   catalog must never enable execution. Use controlled responses for states the
   real daemon cannot reproduce, or record them as unobserved on the host.
5. Before execution, read/copy the warning about imported project code, possible
   external file/network effects and copied workspaces not being a sandbox.
   Inspect the separate trust contract `go test ./...`: permission is broader
   than benchmark-only, applies to this project/revision in the daemon session,
   and granting it does not execute that command. Explicitly activate **Trust
   and run selected benchmark** once on the disposable project. With recording
   available, verify trust GET → validated trust POST (`confirm: true`) → comparison
   POST for the exact selected name/scope and current candidate guards. Trust
   responses must match project/revision, with the trust command list exactly
   `[["go", "test", "./..."]]`; the acknowledgment must be trusted. Observe
   **Admitting · execution trust** / **Checking execution trust…** separately
   from **Running · explicit local
   execution** / **Comparing benchmark…**. Run and discovery controls must be
   disabled while active; repeated activation must not admit another operation.
   Refresh explicitly, reselect, and repeat with catalog-reported trust: **Run
   selected benchmark** must compare directly without silently granting trust.
6. With controlled delays, repeat from a fresh validated candidate/selection,
   holding trust GET, trust POST and comparison responses in separate attempts.
   During each pending stage, edit the declaration/imports, restart validation,
   change the selected benchmark, or explicitly discard/change target/project.
   Confirm old authority and active indicators are revoked. Release the held
   response: obsolete work must not initiate the next stage or publish evidence
   for the replacement candidate/choice. Selecting a different choice must not
   run it. A trust POST already sent may have granted trust; a comparison already
   sent may have started code. Local stopping is not daemon-confirmed cancellation.
   Record the stage, invalidating action, request order and late-response outcome
   separately. Without deterministic delay control, leave those stage checks
   pending; manually racing a fast response does not qualify them.
7. Safely exercise trust rejection/expiry, changed scope, stale-draft conflict
   and lookup/comparison transport failure using controlled responses where
   needed. Trust request rejection must not reach comparison or retry automatically.
   A mismatched/untrusted acknowledgment, daemon-reported expired trust or changed
   scope revokes selection and requires explicit Refresh/reselection/re-admission,
   not automatic trust renewal or substitution of a returned scope. A stale-draft
   conflict requires source refresh and current validation. A comparison timeout
   must not claim code never began or invent measurements. Retain an older
   completed comparison while making a new request: pending/failure copy must
   take precedence and **Prior measurement details** must remain clearly prior,
   not execution authority or measured confirmation of a model hypothesis.
   Inspect retained unavailable, failed, canceled, stale and inconclusive evidence
   separately when supplied; absent cases are unobserved, not native passes.

Throughout, observe request methods/routes, an execution log and disposable source
hashes where available. Finding selection, focus movement, disclosure, copying,
selection and resizing must produce no trust grant, comparison, provider request
or source write; only explicit List/Refresh may request the catalog. Record each
native focus, screen-reader, clipboard and side-effect check with its evidence path
and result; unavailable recording or injection checks remain pending with a reason.
Passing `PerformanceWorkspaceTest`, `DesktopKeyboardNavigationTest`, benchmark
workflow tests or production-component renders is separate automated evidence,
not a record of these native observations.

## Verified Go scan native review

This is a procedure to perform, not a record of completed observations. Use the
[delivered scan behavior](README.md#verified-go-scan) as the expected contract.
Use only a disposable root Go module whose code and tests you trust, isolated
preferences and a fake provider for any import/Analysis work; no live provider is
needed for the scan. Include an intentional test failure, a slow cancelable test
and long diagnostic output in disposable cases. A copied workspace is not a
sandbox: do not use untrusted code or tests with effects outside disposable data.

Repeat the affected Bugs surface at standard/wide sizes and approximately
800×400 and 800×650, at 100%, 125% and 150% text where the host permits. Record
actual viewport, display density, text scale, runtime and input method; these are
review conditions, not new app settings. Test with **Command and output** both
collapsed and expanded, and scroll each nested diagnostic region at short height.

1. Open Bugs without running checks. Read/copy the project ID/revision,
   whole-project parser scope, `go vet ./...`, `go test ./...` and copied-workspace
   execution warning. Exclude a file in **Analysis → Files** and return: the scan
   still describes whole-project scope, not selected-file scope. Scope, warning,
   status, blocked reasons and recovery must remain reachable with details
   collapsed. Repeat with non-Go/unknown project metadata: the trust-and-run
   action is disabled with a root-Go-module explanation, not an apparently valid
   Start. If missing-identity or unread-status states cannot be produced on the
   host, record them unobserved rather than treating fixture coverage as native.
2. Tab/Shift+Tab to enabled scan actions and **Command and output**. Check visible
   focus, Enter/Space activation and, with an enabled screen reader, names and
   expanded/disabled states. Inspect retained diagnostics even when starting is
   blocked. Expand/collapse by keyboard; switch project or replace the report and
   check that obsolete expansion resets. No disclosure or focus change should
   activate Start, Cancel, fix preparation or source editing.
3. Select a finding, search/filter, navigate away/back, expand diagnostics and
   select/copy output without activating execution. Observe request methods/routes
   and an execution log when available: these passive actions must produce no
   execution-trust POST, scan POST/DELETE, provider request or source write. Status
   polling for a previously active scan may continue; read-only GETs are not a new
   admission. Record how side effects were checked; if no transport/execution
   evidence is available, mark that check unverified, not passed by appearance.
4. Explicitly activate **Trust project-code execution & run checks** once. With
   request recording available, verify execution-trust GET → execution-trust POST
   (`confirm: true`) → scan POST for the current revision, with trust responses
   matching project/revision and exactly `["go", "test", "./..."]`. No separate
   checkbox or mock formatting control is expected. Check **Starting** immediately,
   including with an old completed report, and no duplicate admission on repeated
   activation. While running, keyboard-reach **Cancel checks**, activate once and
   inspect **Cancellation requested** until a matching terminal report arrives;
   do not label cancellation confirmed merely because DELETE returned.
5. Inspect the intentional failure and canceled/partial evidence. **Completed**
   must not hide a failed phase. Expand **Command and output**, inspect every
   returned phase/state/argv, and select/copy the full available sanitized command
   and output. Parser inspection must have no invented command exit result;
   empty output and absent phases must not claim success. Use **Show full available
   output**, reach the end by scrolling, copy text beyond the preview and return
   with **Show preview**. Distinguish the UI preview limit from daemon truncation:
   the latter's omitted text cannot be recovered. Long paths, commands and all
   available text must remain reachable after resize. Tool evidence must remain
   distinct from model suggestions and carry no general safety assurance.
6. Where failures can be induced safely, inspect **Start unconfirmed**,
   **Cancellation unconfirmed**, **Status unavailable** and **Live status
   unavailable** beside retained evidence. Keyboard-activate **Refresh scan
   status** when offered: it must read status, not grant trust, start again or
   resend Cancel. Matching active status resumes polling; terminal status can
   trigger a read-only findings refresh. Check enrichment failure retains rows
   with an unavailable/stale warning. Record unknown phases, skipped outcomes,
   workspace failures and foreign/obsolete responses as unobserved on the host
   unless actually supplied and inspected; automated fake-response tests cover
   these states separately without establishing native interaction.

Record each actual observation and evidence path separately from offscreen
renders and fake-transport tests. Mark unavailable native focus, clipboard,
screen-reader, failure-injection or request-recording checks pending with their
reason. Do not report a live-daemon, native-window or provider observation solely
because fixtures rendered.

Use Escape to dismiss only the top transient surface before canceling a request.
With no transient surface or active request, Escape leaves source unchanged.
Record runtime, viewport, text/density scale, input method and actual observations
when reporting a native check. Use an enabled screen reader to inspect spoken
names, focus/selection states and order through stacked panes, dialogs, Review
and dock controls; do not mark this or popup/focus/real-PTY checks passed based on
`DesktopVisualLayoutTest` or other offscreen tests.
