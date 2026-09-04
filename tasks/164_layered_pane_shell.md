# 164 — Layer the shell and replace heavy pane separation

## Status

Pending

## Depends on

Task 163, including its verified local commit.

## Goal

Give the activity rail, tool windows, and editor immediately distinguishable surfaces and precise continuous boundaries.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`IdeShell.kt`, `DesktopShell.kt`, `DesktopHeader.kt`, `DesktopStatusBar.kt`, `DesktopLayoutState.kt` only if required; `DesktopShellTest.kt`, `DesktopLayoutStateTest.kt`, `DesktopVisualLayoutTest.kt`.

## Implementation

1. Apply #18191B to activity/outer chrome, #1E1F22 to tool windows, and #2B2D30 to the editor/content canvas via semantic roles. Ensure the editor/gutter do not sit inside a card.
2. Replace pane gaps, decorative rounded wrappers, and nested borders with a single 1dp divider at each owned boundary. Carry lines cleanly through header/content intersections without double strokes.
3. Keep existing resizing and temporary pane clamping. Preserve a wider invisible splitter hit region, hover cursor, accessible label, keyboard resizing, and stored dimensions; do not turn splitters into a 1dp-only interaction target.
4. Use shared flat headers for docked/drawer containers. Preserve reopen/collapse entry points and focus restoration.
5. Retain the exact 1000dp breakpoint, 360dp docked editor target, rail destinations, and state transitions when leaving Editor. Narrow layouts may overlay drawers, not compress every pane into an unusable column.

## Acceptance criteria

- [ ] Wide Editor captures clearly show three surface levels with thin continuous boundaries and no heavy permanent gutters or whole-pane rounded containers.
- [ ] Each boundary has one divider owner. Resize targets remain practical with pointer and keyboard; resizing does not overwrite stored preferences during temporary clamping.
- [ ] Layouts at 1000dp and 999dp select the correct docked/drawer mode, with no lost selected file, symbol, draft, or hidden reopen control.
- [ ] Surface and splitter changes are present in production shell scenes, not only a control showcase.

## Verification

Extend shell/layout tests for boundary widths, resizing, focus, and minimum editor space; render wide, breakpoint, and narrow scenes. Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
feat(desktop): separate IDE panes with layered surfaces
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

