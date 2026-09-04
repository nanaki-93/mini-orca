# 165 — Flatten Analysis and move run controls into its header

## Status

Pending

## Depends on

Task 164, including its verified local commit.

## Goal

Remove the strongest remaining dashboard-card pattern and make run state/actions read like an IDE tool window.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`WorkspacePanes.kt`, `AnalysisWorkspaceState.kt` only for presentation needs; `AnalysisWorkspaceStateTest.kt`, `DesktopVisualLayoutTest.kt`, existing presenter callback tests.

## Implementation

1. Replace AnalysisRunCard and the separate Run controls card/column with a flat Analysis header and one compact toolbar. Remove their obsolete helpers and sizing rules instead of hiding them.
2. Bind Start/Pause/Resume/Cancel to existing state and callbacks. Render only valid active actions, retain textual progress/run state, and preserve pending-request disablement.
3. Move advanced options and any necessary explanatory help into a compact flat disclosure. Keep Cancel reachable at narrow widths; secondary actions can overflow into a named menu.
4. Present Coverage as compact aligned metric rows/strip on the inherited surface. Convert Analysis errors to an icon/title/count disclosure with full selectable error details and retry only where already supported.
5. Preserve available-width use, readable progress, current job identity, partial/stale/failed coverage, and honest no-data state. Do not reduce missing coverage to a misleading zero or remove safety-related copy.

## Acceptance criteria

- [ ] No standalone Run controls card or equivalent boxed replacement remains. The active job has exactly one action toolbar in its header.
- [ ] Idle, starting, running, pausing, paused, cancelling, cancelled, failed, and completed states preserve the existing domain transitions; no duplicate callback dispatch.
- [ ] Coverage and Analysis errors have dense, flat structure with the required micro-typography. Error details remain reachable and selectable.
- [ ] Long project names, progress text, and errors do not overlap actions at 999dp/800dp or enlarged text. Cancel never disappears into an inaccessible layout.

## Verification

Exercise toolbar enablement/callbacks and disclosures with deterministic job fixtures, including rapid repeated activation and error cases. Run `./desktop/gradlew -p desktop spotlessCheck detekt test`, inspect matching before/after Analysis renders, and run `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
feat(desktop): integrate Analysis actions into pane header
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

