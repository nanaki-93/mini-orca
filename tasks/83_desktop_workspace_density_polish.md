# 83 — Refine workspace density and visual priority

## Status

Pending

## Goal

Finish the visual hierarchy by consolidating connection/status presentation, reducing
filter and control clutter, and grouping related actions responsively.

## Depends on

Task 82.

## Required task commentary

- Before editing, post a concise user-facing commentary update beginning with
  `Starting Task 83` and name the status, filters, action groups, stage navigation,
  likely files, and focused tests.
- After verification, post a concise update beginning with `Task 83 complete` and state
  the density/priority improvements, responsive evidence, tests run, and that Task 84 is
  next.
- During long-running work, continue posting brief progress commentary at least every
  60 seconds. These are user-facing task updates, not source-code comments.

## Implementation

- Replace duplicate connection presentation with one compact, text-labeled colored state
  in the project-open top bar:
  - Connected uses Success;
  - Connecting/attention uses Warning;
  - Disconnected uses Error;
  - model/locality/latency detail is on-demand rather than always visible.
- Show Reconnect only while disconnected or after a connection failure. Keep it absent
  from the no-project landing state and from healthy project-open state.
- Remove connection/model/version duplication from the persistent bottom status bar.
  Either remove the bar when idle or reduce it to concise transient operation/error
  feedback. Do not create a second permanent connection indicator.
- If workspace navigation needs attention state, use only a concise text badge such as
  `Running` or `Failed`; do not restore numeric file/finding/draft counters.
- Add one small responsive action-group helper or a local repeated pattern:
  - related actions share one row when width permits;
  - the group changes to a vertical/wrapped layout before labels clip;
  - preserve logical focus order and spacing;
  - avoid experimental abstractions if a `BoxWithConstraints` branch is sufficient.
- Apply responsive grouping to Analyze/Refresh/Cancel, Analyze-all Pause/Cancel or
  Resume/Cancel, finding Open/Prepare/Triage, and Apply/Undo surfaces where both actions
  can coexist.
- Simplify Bugs filtering:
  - keep one compact search field visible;
  - add a concise Filters disclosure;
  - keep Source, Severity, Freshness, and Lifecycle controls hidden until requested;
  - indicate active filtering in text without an unnecessary numeric badge;
  - preserve the existing pure filter behavior and keyboard order.
- Put Analyze-all file and retry limits in one compact responsive row. Preserve current
  bounds/defaults and remote-provider confirmation.
- Shorten visible Editor stage buttons to Target, Draft, Verify, and Apply.
- Show the current stage once in a concise visible marker and retain full current/locked
  state in semantics. Disabled forward stages must remain understandable without relying
  only on color.
- Reduce excessive panel/action padding to the 8–12dp and 4–6dp rhythm from `PLAN.md`.
  Do not shrink code, diagnostics, or important status copy.
- Keep one dominant filled action per panel. Supporting, attention, and interruption
  actions must use the Task 80 semantic treatment.
- Preserve the 1000dp breakpoint, saved Editor pane widths, Editor-only drawers, keyboard
  navigation, read-only views, and all workflow guards.

## Acceptance criteria

- The project-open UI has one connection indicator and no duplicated
  endpoint/model/latency/version string.
- Reconnect appears only when it can resolve a disconnected/error state.
- Idle status presentation does not consume a permanent verbose row.
- Bugs opens with one search field; advanced filters are available on demand and preserve
  filtering behavior.
- Analyze-all limits share a compact responsive row without clipping or changing bounds.
- Related actions stay visually grouped, keep logical focus order, and remain usable on
  both sides of the 1000dp breakpoint.
- Editor stage buttons use short labels and expose one concise current-stage marker plus
  complete state semantics.
- Each panel has a clear dominant action and semantic colors consistently express
  navigation, success, attention, interruption, and neutral support.
- No numeric workspace counters, raw identity values, redundant helper paragraphs, or
  preview-first safety regression is reintroduced.

## Verification

- Extend `DesktopShellTest` for connection/status visibility and wide/narrow action
  layout decisions.
- Extend `DesktopAccessibilityTest` and `EditorWorkspaceTest` for concise visible
  stage labels, full semantics, focus order, and disabled stage behavior.
- Extend Bugs and Analysis workspace tests for filter disclosure, active-filter text,
  compact limit layout, and unchanged pure filtering/bounds.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

- Inspect the complete Task 83 diff for duplicate connection/status UI, clipped fixed-width
  action rows, restored numeric badges, behavior changes, and unrelated edits.

## Completion

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, and
update its link/status in `tasks/INDEX.md`. Do not create a commit; this task definition
does not authorize commits.
