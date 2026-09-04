# 154 — Refine popup menus and menu-row controls

## Status

Pending

## Depends on

Task 153, completed and committed.

## Goal

Give actual open popup contents the same deliberate styling and interaction
quality as their toolbar triggers.

## Execution contract

Follow `desktop/UI_REFINEMENT_PLAN.md`, `desktop/UI_DESIGN_GUIDELINES.md`, and
`tasks/PROMPT_EXECUTE_UI_REFINEMENT.md`. Paths in this paragraph are repository-root
relative. This task runs only after explicit execution is requested. Use one
implementation agent, finish and commit this task before starting the next, preserve
user-owned changes, and keep the preview-first workflow intact.

## Likely files

Production/test filenames without a directory are under the corresponding
`desktop/src/main/kotlin/io/miniorca/desktop/` or
`desktop/src/test/kotlin/io/miniorca/desktop/` directory.

`DesktopHeader.kt`, `PreviewFeature.kt`, `ChromeControls.kt`, affected popup
consumers, `desktop/UI_COMPONENT_DECISION.md`, and popup/preview tests.

## Implementation

- Evaluate Jewel using current primary JetBrains sources before changing
  dependencies. Record standalone support, relevant coordinates/version,
  compatibility with the pinned toolchain, accessibility, and the decision in
  `desktop/UI_COMPONENT_DECISION.md`.
- Prefer the existing stack if Jewel requires a broader migration. Do not upgrade
  Kotlin/Compose/Gradle as incidental styling work. If a compatible narrow adoption
  is justified, record it and remove the replaced component implementation.
- Implement the smallest shared popup/menu-row treatment needed by real menus.
  Keep mature popup placement and keyboard semantics; do not build a generic menu
  framework or lose behavior by replacing it with a raw clickable Column.
- Style popup surface, subtle border/shadow, corners, row padding, separators,
  optional leading icons, shortcut/status columns, and hover/focus/disabled states.
  Target 32–36dp rows at normal scale, growing for text accessibility.
- Migrate project actions and Preview menus and other actual dropdown consumers
  identified by Task 150. Keep concise labels and group related actions.
- Bound popup dimensions to the viewport, scroll long contents, and anchor correctly
  near edges. Preserve conditional Reconnect and disabled project-dependent actions.
- Keep Preview badges and local-only dialogs honest; opening a popup/dialog never
  calls a provider. Only explicit live action selection invokes its existing callback.
- Restore focus on dismissal and after preview dialogs. Support keyboard traversal,
  Enter/Space activation, Escape, and outside dismissal as appropriate.

## Required legacy removal

Remove duplicate popup styling, obsolete item wrappers, and unused preview
labels/helpers after all callers migrate. No stock-looking alternate menu path or
parallel Jewel/custom replacement remains for the same component.

## Acceptance criteria

- Project and Preview popup rows match the shared IDE surface/spacing system in
  every interaction state, not only when closed.
- Long labels, disabled items, and edge-anchored popups remain usable at narrow
  widths and 130% text scale.
- Each live action invokes its callback once; disabled items and Preview flows
  invoke no backend/workflow actions.
- Keyboard dismissal returns focus predictably; the library decision is documented
  without an unapproved toolchain migration.

## Verification

Add rendered popup interaction/layout coverage using actual production components.
Extend `PreviewFeatureTest` and `DesktopVisualLayoutTest` with open menu, disabled
item, keyboard focus/dismissal, long-content, and callback-isolation cases. If the
existing harness cannot inspect a popup, extend the test adapter rather than fake
its layout or claim native verification.

For every task, run `./desktop/gradlew -p desktop spotlessCheck detekt test` and
`git diff --check` before committing; checks already included above need not run
twice. Add behavior-focused tests where coverage is missing. Record actual results
below, including visual evidence and native checks deferred to Task 159/160.
Required automated failures block the task subject only to the explicitly stated
repository-baseline exception in Tasks 150 and 160.

## Commit

After acceptance passes, prepare Complete status, move this file to
`tasks/completed/`, and update its index link/status in the same isolated commit.
Follow the prompt's staging/review protocol. Use exactly this subject:

```text
style(desktop): refine popup menus (task 154)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Not run — task is Pending. Replace this paragraph during execution with actual
checks, outcomes, legacy code removed, evidence paths, and remaining limitations.
