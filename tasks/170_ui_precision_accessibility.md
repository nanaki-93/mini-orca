# 170 — Verify responsive layout, accessibility, and native visuals

## Status

Pending

## Depends on

Task 169, including its verified local commit.

## Goal

Establish that the completed design works in the actual desktop UI, not only in source or offscreen fixtures.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`DesktopVisualLayoutTest.kt`, `DesktopAccessibilityTest.kt`, `DesktopKeyboardNavigationTest.kt`, shell/layout/interaction tests; narrowly affected production components; `desktop/VISUAL_REVIEW.md`, `desktop/KEYBOARD_SMOKE_CHECKLIST.md`; new `desktop/UI_PRECISION_ACCEPTANCE.md`.

## Implementation

1. Regenerate and inspect matching production-component before/after captures for the full plan matrix. Record scale, viewport, fixture state, component coverage, and image provenance; keep generated output in ignored build locations.
2. Exercise 1440×900, 1920×1080, 1000×760, 999×760, 800×650, and 1280×600; check 100/125/150% text and 1×/2× density where supported. Test long paths/errors, populated/empty/stale states, compact headers, and no overlapping actions.
3. Test keyboard-only traversal of rail, Files tree, tabs, breadcrumbs where actionable, run actions, disclosures, splitters, drawers, menus, palette, and dialogs. Verify names, selected/expanded state, disabled behavior, Escape, and focus restoration.
4. Launch the actual native app on the supported runtime with a disposable fixture. Capture pane hierarchy and real menus/dialogs at window edges; verify source/diff selection and native resizing. Label all native versus offscreen evidence precisely.
5. Run an available supported screen reader through core navigation, run controls, provider confirmation, and guarded Review. Record OS/runtime/assistive technology and observed names/states; do not infer a pass from semantics tests alone.
6. Fix observed UI regressions within desktop scope and add deterministic reproductions. Reuse relevant Task 149 evidence but leave that historical task Pending unless its acceptance is separately authorized and satisfied.

## Acceptance criteria

- [ ] All three priority gates are visible in the real app: Jewel-backed consistent controls, layered panes with 1dp boundaries, and compact header-owned actions.
- [ ] The full responsive/interaction matrix has no material clipping, hidden essential controls, ambiguous active/focus state, or preview-first regression.
- [ ] Actual native popup/dialog placement, OS keyboard/focus behavior, and supported screen-reader results are recorded. Missing material evidence keeps this task In Progress and blocks Task 171.
- [ ] Contrast checks cover all actual text/background/focus pairs. Screenshot fixtures contain no real user secrets or live provider data.

## Verification

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`, plus the documented native/assistive-technology matrix. Report unsupported combinations explicitly. Request operator help when required native evidence cannot be gathered; do not mark Complete or commit a passing acceptance record prematurely.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
test(desktop): verify UI precision and native interactions
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

