# Mini-Orca implementation tasks

Active feature plan: [UI refinement](../desktop/UI_REFINEMENT_PLAN.md).

Completed plans: [IDE-style desktop UI](../plan.md) and
[Engineering insights and project performance analysis](../docs/insights-performance/PLAN.md).

## Planned UI refinement: Tasks 150–160

Execute only when requested, using
[PROMPT_EXECUTE_UI_REFINEMENT.md](PROMPT_EXECUTE_UI_REFINEMENT.md).
The sequence requires one implementation agent, strict numeric order, verification,
and exactly one isolated local commit per completed task. No pushes.

| Order | Task | Depends on | Status |
| ---: | --- | --- | --- |
| 150 | [Establish the UI refinement baseline](completed/150_ui_refinement_baseline.md) | 148 + current baseline | Complete |
| 151 | [Remove duplicate Project navigation](completed/151_remove_project_navigation.md) | 150 | Complete |
| 152 | [Unify IDE design tokens and control states](completed/152_unify_ide_design_tokens.md) | 151 | Complete |
| 153 | [Correct Analysis and shared workspace widths](completed/153_correct_workspace_widths.md) | 152 | Complete |
| 154 | [Refine popup menus and menu-row controls](completed/154_refine_popup_menus.md) | 153 | Complete |
| 155 | [Refine disclosures, drawers, and tool-window controls](155_refine_disclosures_and_drawers.md) | 154 | Pending |
| 156 | [Redesign Summary as a concise project dashboard](156_redesign_summary_dashboard.md) | 155 | Pending |
| 157 | [Simplify workspace copy and information density](157_simplify_workspace_copy.md) | 156 | Pending |
| 158 | [Polish Editor, workflow surfaces, and global UI copy](158_polish_editor_workflow_surfaces.md) | 157 | Pending |
| 159 | [Verify responsive layout and accessible UI interactions](159_verify_refined_ui_accessibility.md) | 158 | Pending |
| 160 | [Complete UI refinement acceptance and cleanup](160_ui_refinement_acceptance.md) | 159 | Pending |

Task 150 verifies the current implementation after Tasks 140–148 and the subsequent
visual correction. Task 149 remains a separate outstanding acceptance record, not
a hard dependency or a task to silently complete within this new sequence.
The execution prompt owns the exact commit subjects, bootstrap artifact list,
staging safeguards, resume behavior, and verification policy.

## Prior dark desktop redesign: Tasks 140–149

| Order | Task | Depends on | Status |
| ---: | --- | --- | --- |
| 140 | [Record the dark UI reference and behavior baseline](completed/140_dark_ui_reference_baseline.md) | 139 | Complete |
| 141 | [Implement the charcoal theme and icon system](completed/141_dark_theme_and_icons.md) | 140 | Complete |
| 142 | [Restyle the shell, toolbar, and navigation](completed/142_dark_shell_and_navigation.md) | 141 | Complete |
| 143 | [Restyle the project tree and source editor](completed/143_dark_explorer_and_editor.md) | 142 | Complete |
| 144 | [Reorganize AI Context and restyle project workspaces](completed/144_dark_context_and_workspaces.md) | 143 | Complete |
| 145 | [Restyle the Assistant and candidate review](completed/145_dark_assistant_and_candidate_review.md) | 144 | Complete |
| 146 | [Restyle bottom tools and persistent status](completed/146_dark_bottom_tools_and_status.md) | 145 | Complete |
| 147 | [Add unsupported-feature UI previews](completed/147_ui_preview_features.md) | 146 | Complete |
| 148 | [Verify responsive and accessible dark UI behavior](completed/148_dark_ui_responsive_accessibility.md) | 147 | Complete |
| 149 | [Complete dark UI visual and regression acceptance](149_dark_ui_acceptance.md) | 140–148 | Pending |

The earlier [PROMPT_EXECUTE_DARK_UI.md](PROMPT_EXECUTE_DARK_UI.md) applies only when
that earlier sequence is explicitly requested and does not authorize commits.
Tasks 150–160 are the current UI refinement backlog. They preserve the existing
single-file review workflow and Preview isolation; they do not assert that Task 149's
native/repository acceptance is complete.

## Historical ledger: Tasks 01–102

Tasks 01–102 are completed implementation history. Their detailed task files,
obsolete execution prompts, and retired UI-mock material were consolidated in Task
116. Git history remains the full, immutable archive.

| Range | Completed delivery |
| --- | --- |
| 01–31 | Preview-first single-project Desktop safety foundation, API, configuration, and release baseline. |
| 32–56 | Project analysis, findings, scans, Go declaration drafts, checks, Apply/Undo, and Desktop workspaces. |
| 57–79 | Focused Desktop presentation, responsive shell, Editor review flow, and visual acceptance. |
| 80–94 | Compact accessible Desktop controls and direct selected-symbol editing. |
| 95–102 | Strict scoped-model configuration, OpenAI-compatible routing, task repair, and scoped-model acceptance. |

## Completed cleanup sequence: Tasks 103–117

| Order | Task | Depends on | Status |
| ---: | --- | --- | --- |
| 103 | [Cleanup contract and characterization baseline](completed/103_cleanup_contract_baseline.md) | 102 | Complete |
| 104 | [Retire autonomous orchestration](completed/104_retire_autonomous_orchestration.md) | 103 | Complete |
| 105 | [Simplify scoped model execution](completed/105_simplify_scoped_model_execution.md) | 104 | Complete |
| 106 | [Consolidate project detection and remove tools](completed/106_consolidate_project_detection.md) | 105 | Complete |
| 107 | [Enforce one scoped configuration and LLM boundary](completed/107_strict_scoped_configuration.md) | 106 | Complete |
| 108 | [Introduce draft-native identities and Apply stages](completed/108_draft_identities_apply.md) | 107 | Complete |
| 109 | [Remove candidate and workflow compatibility](completed/109_remove_candidate_compatibility.md) | 108 | Complete |
| 110 | [Reduce the loopback API contract](completed/110_reduce_loopback_api.md) | 109 | Complete |
| 111 | [Unify project source traversal](completed/111_unify_project_traversal.md) | 110 | Complete |
| 112 | [Unify metadata persistence](completed/112_unify_metadata_persistence.md) | 111 | Complete |
| 113 | [Isolate Go workflow state](completed/113_isolate_go_workflow_state.md) | 112 | Complete |
| 114 | [Extract the Desktop workflow presenter](completed/114_extract_desktop_presenter.md) | 113 | Complete |
| 115 | [Simplify Compose and API client contracts](completed/115_simplify_desktop_ui_contracts.md) | 114 | Complete |
| 116 | [Clean documentation and build residue](completed/116_clean_docs_and_build.md) | 115 | Complete |
| 117 | [Complete cleanup quality and acceptance](completed/117_cleanup_acceptance.md) | 103–116 | Complete |

The previous sequence remains reproducible through
[`PROMPT_EXECUTE_LEGACY_CLEANUP.md`](PROMPT_EXECUTE_LEGACY_CLEANUP.md).

## Completed IDE-style UI sequence: Tasks 118–132

| Order | Task | Depends on | Status |
| ---: | --- | --- | --- |
| 118 | [Establish the IDE UI contract baseline](completed/118_ide_ui_contract_baseline.md) | 117 | Complete |
| 119 | [Add Desktop IDE layout state](completed/119_desktop_layout_state.md) | 118 | Complete |
| 120 | [Build the IDE shell foundation](completed/120_ide_shell_foundation.md) | 119 | Complete |
| 121 | [Rebuild Project as a navigation tool window](completed/121_project_tool_window.md) | 120 | Complete |
| 122 | [Add active-file editor chrome](completed/122_editor_chrome.md) | 121 | Complete |
| 123 | [Improve the source gutter and viewport](completed/123_source_gutter_and_viewport.md) | 122 | Complete |
| 124 | [Add the Context tool window](completed/124_context_tool_window.md) | 123 | Complete |
| 125 | [Add Assistant and Review tool windows](completed/125_assistant_review_tool_windows.md) | 124 | Complete |
| 126 | [Add the Problems tool window](completed/126_problems_tool_window.md) | 125 | Complete |
| 127 | [Add Checks and Output tool windows](completed/127_checks_output_tool_windows.md) | 126 | Complete |
| 128 | [Simplify the toolbar and command search](completed/128_toolbar_and_command_search.md) | 127 | Complete |
| 129 | [Add the persistent Desktop status bar](completed/129_persistent_status_bar.md) | 128 | Complete |
| 130 | [Harden responsive and accessible IDE navigation](completed/130_responsive_accessible_ide_shell.md) | 129 | Complete |
| 131 | [Unify workspace density and visual states](completed/131_workspace_density_polish.md) | 130 | Complete |
| 132 | [Complete IDE UI acceptance](completed/132_ide_ui_acceptance.md) | 118–131 | Complete |

### Plan coverage

| Plan phase | Implementation tasks |
| --- | --- |
| Phase 0 — Baseline and UI contract | 118 |
| Phase 1 — IDE shell foundation | 119–120 |
| Phase 2 — Project navigation and editor chrome | 121–123 |
| Phase 3 — Context, Assistant, and Review | 124–125 |
| Phase 4 — Problems, checks, and output | 126–127 |
| Phase 5 — Toolbar, commands, and status | 128–129 |
| Phase 6 — Responsive behavior and visual polish | 130–131 |
| Phase 7 — Validation and rollout | 132 |

Tasks 118–132 execute strictly in numeric order through
[`PROMPT_EXECUTE_IDE_UI.md`](PROMPT_EXECUTE_IDE_UI.md). Each task has one
implementation writer, required user-facing start/completion commentary, focused
verification, and exactly one isolated commit. No task authorizes a push or weakens
the preview-first Apply/Undo boundary.

## Completed insights and performance sequence: Tasks 133–139

| Order | Task | Depends on | Status |
| ---: | --- | --- | --- |
| 133 | [Establish the insights and performance contract baseline](completed/133_insights_performance_contract_baseline.md) | 132 | Complete |
| 134 | [Generate and transport contextual engineering insights](completed/134_engineering_insight_generation.md) | 133 | Complete |
| 135 | [Add the collapsible in-page engineering insight panel](completed/135_inline_engineering_insight_panel.md) | 134 | Complete |
| 136 | [Implement bounded source-based performance file review](completed/136_performance_file_review.md) | 135 | Complete |
| 137 | [Add bounded project performance jobs and API](completed/137_project_performance_job.md) | 136 | Complete |
| 138 | [Add the Performance workspace and optimization handoff](completed/138_performance_workspace.md) | 137 | Complete |
| 139 | [Complete insights and performance acceptance](completed/139_insights_performance_acceptance.md) | 133–138 | Complete |

Execute through
[PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md](PROMPT_EXECUTE_INSIGHTS_PERFORMANCE.md).
Tasks 133–139 are complete. Execution used one implementation agent and one verified
local commit per task. No push was authorized.

### Feature plan coverage

| Feature-plan area | Implementation tasks |
| --- | --- |
| Completed-IDE baseline, contracts, fixtures, and initial planning artifacts | 133 |
| Optional expert insight generation, owner identity, transport, and storage | 134 |
| Compact close/reopen panel in existing pages and tool windows | 135 |
| Source-based performance finding model, file review, policy, and cache | 136 |
| Explicit project job, coverage, cancellation/restart, budgets, and API | 137 |
| Separate Performance page, navigation, details, and safe optimization preparation | 138 |
| Full acceptance, privacy/regressions, cleanup, documentation, and commit ledger | 139 |
