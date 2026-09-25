# Desktop keyboard smoke checklist

Use a disposable Go project, a fake provider and isolated preferences. This is an
operator procedure. [UI guidelines](UI_DESIGN_GUIDELINES.md) cover interaction
and accessibility.

Launch, restore and resize the native window at 1600×1000, 1440×900, 1024×768,
800×650 and 1280×600 where the host permits. Check action reachability, header
wrapping and footer alignment. Repeat affected surfaces at 125% and 150% text;
record viewport, display density and text scale separately. Compare native and
component observations without treating offscreen captures as proof of OS focus,
popup placement or spoken screen-reader output.

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
   exclusions, cache use and request bounds. Confirm every required remote
   destination and explicit Security intent. Admission is one operation; starting
   a result page must show the whole category and never launch another analysis.
   The daemon's dispatch bounds remain enforced without Run limits controls.
4. Verify Analysis contains progress, per-section states, failures and result
   links. Pause settles at a stage boundary, Cancel stops further requests, and
   Resume requires a fresh preview. After restart, retained progress is interrupted
   or paused without dispatch authority. Check partial, failed, unavailable,
   canceled and completed-empty evidence retain their distinct labels/counts.
5. In each result page, verify the three category boxes name their destinations,
   mark the current category and navigate with Enter/Space without starting work.
   Select a result and inspect severity, exact source location and its
   evidence. Selection
   remains local and does not prepare or
   apply a fix. Only **Prepare fix** prefills Assistant, and its disabled reason
   remains visible when the source identity is ineligible. Performance hypotheses
   never claim measured speedup; Security rule matches and model hypotheses remain
   distinct in their evidence. Bugs keeps separately trusted verified Go scans.
6. Use Cmd/Ctrl+P to choose a file and Cmd/Ctrl+Shift+O to choose a declaration.
   Source and diff must remain selectable/read-only. Relative paths disambiguate
   equal basenames. Source drag selects text without changing the draft target.
7. Resize Editor and check access to Files and Context/Assistant/Review in the
   chosen layout. Activate rail Commands and check that it opens the actions palette;
   header search opens file search. Close and Escape each palette from its own
   opener (keyboard and pointer) and verify *native focus* returns to that surviving
   control, not just its region. Activate rail Models and footer counts separately:
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
    At reduced heights and 150% text, verify scrolling exposes the complete Apply
    scope and recovery action. Expand Check details and Project context without dispatching a request.
    During a rerun, show Running even when the previous report passed.
11. Confirm Terminal and its shell tabs share one bottom bar. Explicitly activate
    rail Terminal, collapsed-dock Terminal or Ctrl+Shift+T: each opens/focuses the
    dock and may start a real shell in the project if none exists. Repeating rail
    activation must not create another session or collapse the dock. Use the
    expanded dock's Terminal control to collapse it; collapse does not close a tab.
    Use + to create independent shells, switch tabs, and × to close one without
    stopping others. Collapse/reopen, workspace changes and resizing preserve each
    shell's PID, history and scrollback. Resize the terminal and verify real PTY
    dimensions change without losing content or hiding the canvas.
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
when reporting a native check.
