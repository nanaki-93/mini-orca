# 166 — Apply dense flat sections to Summary and Performance

## Status

Complete

## Depends on

Task 165, including its verified local commit.

## Goal

Carry the same visual rhythm into the other project workspaces without inventing data or unsupported actions.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`ProjectSummaryPane.kt`, `PerformanceWorkspace.kt`, `EngineeringInsightPanel.kt`; `ProjectSummaryPaneTest.kt`, `PerformanceWorkspaceTest.kt`, `EngineeringInsightPanelTest.kt`, visual fixtures.

## Implementation

1. Convert remaining metric cards and padded containers to flat aligned rows, compact lists, and shared disclosure sections. Keep Summary's deterministic facts visually distinct from advisory interpretation.
2. Move Performance's existing review/job controls into owning headers using the shared toolbar. Do not invent Pause/Resume when the Performance job contract only supports other actions.
3. Keep useful metrics grouped by labels rather than decorative boxes. Show unknown, skipped, stale, partial, budget-limited, cancelled, and failed coverage explicitly.
4. Retain Source-based review / Not measured labeling, finding evidence, optimization preparation, and their existing guards; no performance score or measured-speed claim.
5. Restyle Engineering insight through its shared component so owner identity, close/reopen, and availability behavior remain consistent wherever used.
6. Use the shared typography/spacing roles and remove obsolete workspace panel helpers or styling branches.

## Acceptance criteria

- [x] Summary and Performance use the same 8dp insets, compact heading rhythm, and inherited section surfaces as Analysis; no generic rounded status-card stacks remain.
- [x] Existing scope, coverage, stale/advisory labeling, and safe optimization handoff remain unchanged and behavior-tested.
- [x] Only real actions appear, with textual state and named icon controls. Each operation has one primary action location.
- [x] Populated/empty/error scenes remain usable at wide/narrow widths and 150% text scale; long finding text can be read in full.

## Verification

Update the three workspace/panel test suites and representative production renders. Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
refactor(desktop): flatten Summary and Performance sections
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Completed 2026-09-05.

- Flattened Summary facts, coverage, interpretation, and shared Engineering Insight sections;
  kept indexed facts distinct from advisory model output.
- Moved the existing Performance lifecycle controls into the Performance header. Preview, remote
  confirmation, start, pause, resume, cancel, editor navigation, and guarded optimization
  preparation retain their prior workflow callbacks. Review coverage now explicitly labels
  unknown, partial, skipped, failed, remaining, stale/canceled, and budget-limited states from
  existing job/report fields; it makes no performance-score or measured-speed claim.
- Ran `spotlessApply`, focused workspace/panel and visual tests, then
  `JAVA_HOME=/Users/marcoandreose/.sdkman/candidates/java/21.0.11-tem ./desktop/gradlew -p desktop spotlessCheck detekt test -Porg.gradle.java.installations.paths=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home` successfully. Detekt requires the JDK 21 Gradle launch; the supported JBR 25 path remains the Kotlin toolchain.
- Ran `JAVA_HOME=/private/tmp/mini-orca-jbr-TP5kFo/jbrsdk-25.0.4-osx-aarch64-b508.27/Contents/Home ./desktop/gradlew -p desktop packageDistributionForCurrentOS` successfully, and `git diff --check` passed.
- Inspected production fixture evidence: `desktop/build/reports/ui-precision/task-166/summary-dashboard-1440.png`, `summary-dashboard-800-1.3.png`, `performance-populated-1440.png`, `performance-empty-800-1.3.png`, and `engineering-insight-expanded-720-1.3.png`. These are offscreen Compose renders; native-window, OS keyboard, and screen-reader verification remains for Task 170. No configuration or runtime migration is required.
