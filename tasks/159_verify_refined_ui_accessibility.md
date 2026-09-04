# 159 — Verify responsive layout and accessible UI interactions

## Status

Pending

## Depends on

Task 158, completed and committed.

## Goal

Verify the complete redesigned surface matrix with production-component renders
and interaction checks, fixing remaining UI integration defects.

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

`DesktopVisualLayoutTest.kt`, `DesktopAccessibilityTest.kt`, keyboard/shell tests,
only production components needed for discovered defects, and
`desktop/UI_REFINEMENT_ACCEPTANCE.md` plus the keyboard/visual checklists.

## Implementation

- Expand the fixture matrix to Summary, Analysis, Editor, Performance, Problems,
  Assistant/Review, menus, disclosures, drawers, bottom overlays, and dialogs.
  Reuse representative states instead of a wasteful full Cartesian test matrix.
- Cover 1440×900, 1920×1080, 1000×760, 999×760, 800×650, and 130% text scale.
  Include long project/file/model names, long menus/details, empty/loading/stale/
  error states, expanded and collapsed content, and invalid/disabled actions.
- Assert meaningful layout bounds, no essential-label clipping, shared alignment,
  bounded controls, scroll reachability, and active/focused state visibility.
  Do not make exact platform font pixels a brittle cross-machine requirement.
- Test real keyboard event paths where possible: tab order, arrow navigation,
  activation, Escape for the topmost surface, focus restoration, drawer/workspace
  changes, resizing, and preference round-trips.
- Confirm semantic names, expanded/selected/disabled state, textual status, contrast,
  and preserved source/diff selection. Callback tests must prove local-only Preview
  and disclosure behavior, not just inspect source strings.
- Compare rendered output with the supplied mock and Task 150 baseline. Fix
  integration defects in the owning component; do not change fixtures to hide bugs.
- Perform available native keyboard/window and screen-reader checks. Record each
  result, capture provenance, and missing tool/environment capability precisely.
  Never call an offscreen fixture a native screenshot.
- Start the final acceptance record with a requirement-to-evidence matrix and
  explicit release follow-ups for any unavailable native checks.

## Required legacy removal

Remove duplicate test harnesses, stale screenshots/references where superseded,
unnecessary fixture-only production branches, and obsolete assertions. Generated
captures remain generated output, not manually edited assets or committed build trees.

## Acceptance criteria

- All required automated responsive, accessibility-semantics, interaction, and
  workflow-isolation checks pass on the actual production components.
- Every requested area has reviewed before/after evidence, including open menus and
  Summary, not just closed shell screenshots.
- No essential control clips or becomes unreachable at the required sizes/scales.
- Native checks are individually passed with evidence or explicitly unavailable;
  an observed native defect blocks completion until fixed. Unavailability may be
  recorded as a release follow-up, never as a native pass.

## Verification

Run:

```sh
./desktop/gradlew -p desktop spotlessCheck detekt test \
  -PvisualOutput="$PWD/desktop/build/reports/ui-refinement/after"
```

Verify every image claimed in the acceptance record was produced and inspected;
record representative state/viewport coverage and native limitations.

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
test(desktop): verify refined UI accessibility (task 159)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Not run — task is Pending. Replace this paragraph during execution with actual
checks, outcomes, legacy code removed, evidence paths, and remaining limitations.
