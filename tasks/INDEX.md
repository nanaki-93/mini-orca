# Mini-Orca implementation tasks

Active plan: [UI precision](../Plan.md).
Execution: [PROMPT_EXECUTE_UI_PRECISION.md](PROMPT_EXECUTE_UI_PRECISION.md).

## Planned UI precision — Tasks 161–171

Strict numeric order; one implementation writer, one verified local commit per task,
and no push. Tasks 161–164 are complete; later tasks remain Pending.

| Order | Task | Depends on | Status |
| ---: | --- | --- | --- |
| 161 | [Establish the baseline and prove Jewel compatibility](161_ui_precision_baseline.md) | Current baseline | Complete |
| 162 | [Integrate Jewel and the semantic IDE theme](162_jewel_theme_integration.md) | 161 | Complete |
| 163 | [Build shared dense headers, toolbars, and dividers](163_dense_ide_primitives.md) | 162 | Complete |
| 164 | [Layer the shell and replace heavy pane separation](164_layered_pane_shell.md) | 163 | Complete |
| 165 | [Flatten Analysis and move run controls into its header](165_flat_analysis_toolbar.md) | 164 | Pending |
| 166 | [Apply dense flat sections to Summary and Performance](166_dense_summary_performance.md) | 165 | Pending |
| 167 | [Refine the Files tree, editor tabs, and breadcrumbs](167_precise_editor_navigation.md) | 166 | Pending |
| 168 | [Flatten Context, Assistant, and Review safely](168_flat_context_review.md) | 167 | Pending |
| 169 | [Finish bottom panes, global controls, and migration cleanup](169_complete_ide_surface_migration.md) | 168 | Pending |
| 170 | [Verify responsive layout, accessibility, and native visuals](170_ui_precision_accessibility.md) | 169 | Pending |
| 171 | [Complete acceptance, documentation, and commit ledger](171_ui_precision_acceptance.md) | 170 | Pending |

A dependency includes its verified task commit. Never skip an incomplete task.
Task 161 owns reviewed uncommitted planning/cleanup artifacts under the prompt's
bootstrap scope. It does not create an additional planning commit.

## Outstanding historical acceptance

| Task | Scope | Status |
| ---: | --- | --- |
| [149](149_dark_ui_acceptance.md) | Dark-UI native/repository acceptance after completed Tasks 140–148 | Pending |

Task 149 is not a dependency of 161–171. Reuse valid evidence where useful without
silently completing the historical acceptance.

## Completed history

Completed individual task files and superseded completed plans/prompts were removed
at the user's request. Git contains their full records; do not recreate a parallel
archive of completed task documents.

| Range | Completed delivery |
| --- | --- |
| 01–102 | Preview-first product, desktop/API, model-scope and workflow foundation |
| 103–117 | Legacy cleanup and cohesive scoped architecture |
| 118–132 | IDE shell, editor, context, bottom tools, keyboard/layout foundation |
| 133–139 | Engineering insights and source-based Performance review |
| 140–148 | Dark theme, navigation, workspaces, Preview controls, responsive checks |
| 150–160 | Navigation cleanup, shared tokens, width/density/copy refinements and component acceptance |

[Previous UI acceptance](../desktop/UI_REFINEMENT_ACCEPTANCE.md) preserves the
commit/evidence ledger and native limitations. Implementation completion does not
imply Task 149 or full native release acceptance passed.
