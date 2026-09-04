# 156 — Redesign Summary as a concise project dashboard

## Status

Pending

## Depends on

Task 155, completed and committed.

## Goal

Replace Summary's stacked prose cards with real project metrics, concise
interpretation, and expandable details in Analysis's visual language.

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

`ProjectSummaryPane.kt`, its presentation model, `ContextToolWindow.kt` shared
consumer, existing project overview models as read-only inputs, and Summary tests.

## Implementation

- Use the shared available-width page rules and compact heading with project
  identity, language, and build metadata.
- Show indexed files, total lines, verified findings, and AI suggestions as a
  compact metric strip. Use real overview metrics and existing deterministic
  fallback facts; distinguish absent values from actual zero counts.
- Present analysis coverage compactly with labeled fresh/stale/missing/running/
  failed counts. Do not combine verified findings and model suggestions or
  synthesize a project health score.
- Show a short purpose preview with AI interpretation and freshness labels. Clamp
  presentation only, providing explicit access to the exact full returned text.
  Do not request a new model summary or persist a truncated replacement.
- Place architecture, components, entry points, flows, risks, and next steps in
  the shared expandable detail treatment. Preserve all returned content and risk
  severity; keep empty sections quiet.
- Keep deterministic facts visible for missing/running/stale/failed interpretation.
  Preserve actionable failure details and render missing data honestly.
- Replace the large navigation action panel with concise Analysis/Bugs links.
- Refactor the existing shared presentation model and AI Context consumer together,
  keeping source/freshness rules at one boundary. Avoid duplicate presenters or
  introducing a new backend model solely for layout.

## Required legacy removal

Remove old stacked Summary panels, redundant sentence fields and list helpers,
and obsolete phrase-based tests. Remove fields only after all Summary/Context
consumers migrate; retain real analysis data and useful existing policy tests.

## Acceptance criteria

- At 1440×900 the initial populated Summary shows identity, key metrics, coverage,
  and concise purpose without scrolling through long interpretation lists.
- Every full interpretation section remains reachable, including long purpose,
  empty lists, risk labels, and stale/error details.
- Null overview/analysis/counts and genuine zeros are visibly different; fallback
  indexed facts remain available without inventing model results.
- Analysis/Bugs links preserve existing navigation and perform no provider calls.
- Summary and AI Context agree on data source and freshness.

## Verification

Update `ProjectSummaryPaneTest`, `ContextToolWindowTest`, and visual fixtures for
no project, deterministic-only, missing/running/fresh/stale/failed AI, zero counts,
absent counts, long purpose/lists, expansion, and navigation isolation. Render wide,
narrow, and 130% text scale.

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
feat(desktop): redesign Summary dashboard (task 156)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Not run — task is Pending. Replace this paragraph during execution with actual
checks, outcomes, legacy code removed, evidence paths, and remaining limitations.
