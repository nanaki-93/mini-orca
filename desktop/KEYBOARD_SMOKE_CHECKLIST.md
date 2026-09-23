# Desktop keyboard smoke checklist

Use a disposable Go project, a fake provider and isolated preferences. This is an
operator procedure, not passing evidence. [Release acceptance](../docs/RELEASE_ACCEPTANCE.md)
owns observed results and host limitations; [UI guidelines](UI_DESIGN_GUIDELINES.md)
owns the visual rules.

Check wide, exactly 1000dp, 999dp, 800×650 and 1280×600 windows. Repeat affected
surfaces at 125% and 150% text. Record native versus component observations
separately; native accessibility names do not establish spoken reader behavior.

For changed flows, also apply the [UI copy review](UI_DESIGN_GUIDELINES.md#verification):
with optional details collapsed, identify the task, state and next action from
the visible controls. Check for redundant headings and helper paragraphs; verify
that labels, blocked reasons and required consent remain clear
with keyboard focus. Record this review separately from historical observations.

1. Before opening a project, verify Open project and Cmd/Ctrl+O work. Cancel the
   chooser and confirm the landing state remains. When entering a disposable path,
   confirm its native field is visible before operating it; record an offscreen
   computer-use failure as pending native evidence. Project shortcuts must have no
   action until a project opens. Restore must not call a provider or start a shell.
2. Verify Project, Results and Editing groups have distinct labeled destinations.
   Cmd/Ctrl+1–4 select Summary, Analysis, Bugs and Editor. Cmd/Ctrl+Tab cycles all
   workspaces. Commands offers View Performance results and View Security results;
   navigation preserves the selected file, draft and loaded results.
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
   Select a rounded result row and inspect severity, exact source location and its
   evidence disclosure. At narrow widths, **Back to results** restores the list;
   wide details have no Clear selection action. Row selection must not prepare or
   apply a fix. Only **Prepare fix** prefills Assistant, and its disabled reason
   remains visible when the source identity is ineligible. Performance hypotheses
   never claim measured speedup; Security rule matches and model hypotheses remain
   distinct in their evidence. Bugs keeps separately trusted verified Go scans.
6. Use Cmd/Ctrl+P to choose a file and Cmd/Ctrl+Shift+O to choose a declaration.
   Source and diff must remain selectable/read-only. Relative paths disambiguate
   equal basenames. Source drag selects text without changing the draft target.
7. Editor retains Files and Context/Assistant/Review docks at 1000dp and above.
   Below that width, Files and Context drawers are labeled. Open and dismiss each
   drawer, leave Editor with one open, and verify predictable focus restoration.
   Cmd/Ctrl+P must work again after dismissal; selected source stays intact.
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
    At short heights and 150% text, scroll to the complete Apply scope and recovery
    action. Expand Check details and Project context without dispatching a request.
    During a rerun, show Running even when the previous report passed.
11. Confirm Terminal and its shell tabs share one bottom bar. Selecting Terminal
    or Ctrl+Shift+T immediately opens a real shell in the project. Use + to create
    independent shells, switch tabs, and × to close one without stopping others.
    Collapse/Enter reopen, workspace changes and resizing through 1000/999dp preserve
    the docked presentation, PID, history and scrollback.
    Resize the dock and verify real PTY dimensions change without losing content.
    Inspect the real Swing canvas: its side/bottom inset must preserve the rounded
    terminal perimeter when expanded, resized and reopened.
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
    At short windows and large text, verify long paths/errors, headings, badges,
    disclosure controls, row actions and status details remain readable. Color
    must supplement text labels for selection, severity and meaningful lifecycle
    states.

Use Escape to dismiss only the top transient surface before canceling a request.
With no transient surface or active request, Escape leaves source unchanged.
Record runtime, viewport, text/density scale, input method and actual observations
in the acceptance ledger. Historical qualification limits remain unchanged.
