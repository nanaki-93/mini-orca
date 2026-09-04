# 146 — Restyle bottom tools and persistent status

## Status

Complete

## Depends on

Task 145.

## Goal

Match the reference's compact issue table, bottom tabs, and quiet status strip with
real current findings, checks, operation output, and project state.

## Implementation

- Restyle Bugs & Problems, Checks, and Output tabs with icons, text, counts/summary,
  selection edge, collapse/reopen, and existing bottom resize handling.
- Present severity, file, line, description, and lifecycle status in compact finding
  rows; make severity filters easy to scan without losing advanced filters or
  verified-versus-AI provenance. Use the same presentation in the full Bugs workspace.
- Keep counts consistent with the active filters and expose full text in details.
  Selecting a row navigates to its real location and never prepares/applies a fix implicitly.
- Restyle focused check output and operation summaries. Leave the Terminal panel to
  Task 147; do not relabel Output as a working terminal.
- Group persistent status by operation/project and current file/language/branch/
  daemon/provider. Show only evidence-backed values and retain priority on narrow widths.
- Keep textual disconnected, missing, stale, failed, and running states, plus
  accessible status details for truncated segments.

## Likely files

`ProblemsToolWindow.kt`, `FindingsPresentation.kt`, `BottomEvidenceToolWindows.kt`,
`IdeShell.kt`, `DesktopStatusBar.kt`, and corresponding existing tests.

## Acceptance criteria

- Compact bottom tools visually match the new shell and remain usable when collapsed.
- Filters, counts, lifecycle actions, evidence, and source locations remain coherent.
- Bottom and full-workspace findings do not duplicate business rules or state.
- No static connected/up-to-date/toolchain/encoding value is presented as live status.
- Checks and operation output preserve their semantics; neither starts terminal execution.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use `ProblemsToolWindowTest`, `BottomEvidenceToolWindowsTest`, and
`DesktopStatusBarTest`; verify long descriptions, empty/filtered results, stale evidence,
and disconnected status at wide/narrow sizes when native access is available.
Complete/move the task and update the index; no unrequested commit.
