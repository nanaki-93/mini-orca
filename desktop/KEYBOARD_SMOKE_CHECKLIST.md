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
5. In each result page, verify the three category boxes name their destinations,
   mark the current category and navigate with Enter/Space without starting work.
   At a width on each side of the list/detail reflow boundary, filter and scroll
   the populated list, select a row, then resize across the boundary and back.
   Check the query, filter, selection and applicable list position survive;
   keyboard-reveal and scroll both stacked list and detail, including long evidence.
   Repeat with empty/filtered results and check that unavailable/failed states do
   not become zero findings. Inspect the selected result's severity, exact source
   location and evidence. Selection remains local and does not prepare or apply
   a fix. Only **Prepare fix** prefills Assistant, and its disabled reason
   remains visible when the source identity is ineligible. Performance hypotheses
   never claim measured speedup; Security rule matches and model hypotheses remain
   distinct in their evidence. Bugs keeps separately trusted verified Go scans.
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

Use Escape to dismiss only the top transient surface before canceling a request.
With no transient surface or active request, Escape leaves source unchanged.
Record runtime, viewport, text/density scale, input method and actual observations
when reporting a native check. Use an enabled screen reader to inspect spoken
names, focus/selection states and order through stacked panes, dialogs, Review
and dock controls; do not mark this or popup/focus/real-PTY checks passed based on
`DesktopVisualLayoutTest` or other offscreen tests.
