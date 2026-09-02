# Mini-Orca implementation tasks

Current roadmap: [`../PLAN.md`](../PLAN.md)

Tasks 57–74 retain the completed Desktop visual-refactor history documented in
[`../design/ui-mocks/IMPLEMENTATION_PLAN.md`](../design/ui-mocks/IMPLEMENTATION_PLAN.md).

## Execution order

| Order | Task | Depends on | Status |
|---:|---|---|---|
| 01 | [Product and workflow contract](completed/01_product_workflow_contract.md) | — | Complete |
| 02 | [Configured application service](completed/02_configured_application_service.md) | 01 | Complete |
| 03 | [API contract and documentation](completed/03_api_contract_and_documentation.md) | 01, 02 | Complete |
| 04 | [Context policy](completed/04_context_policy.md) | 01 | Complete |
| 05 | [Context inspector data](completed/05_context_inspector_data.md) | 04 | Complete |
| 06 | [Cancellation and timeouts](completed/06_cancellation_and_timeouts.md) | 02 | Complete |
| 07 | [Project revision and state](completed/07_project_revision_and_state.md) | 02 | Complete |
| 08 | [Deterministic index persistence](completed/08_deterministic_index_persistence.md) | 04, 07 | Complete |
| 09 | [Go symbols](completed/09_go_symbol_extraction.md) | 08 | Complete |
| 10 | [Generic symbols](completed/10_generic_symbol_extraction.md) | 08 | Complete |
| 11 | [Index and symbol APIs](completed/11_index_and_symbol_apis.md) | 08, 09, 10 | Complete |
| 12 | [File-analysis cache](completed/12_file_analysis_cache.md) | 04, 07, 08 | Complete |
| 13 | [Semantic file analysis](completed/13_semantic_file_analysis.md) | 02, 05, 09, 10, 12 | Complete |
| 14 | [File-analysis APIs](completed/14_file_analysis_apis.md) | 12, 13 | Complete |
| 15 | [Sequential analyze-all](completed/15_sequential_analyze_all.md) | 14 | Complete |
| 16 | [Generation response contract](completed/16_generation_response_contract.md) | 02, 06, 07 | Complete |
| 17 | [Scope validation and diff](completed/17_scope_validation_and_diff.md) | 09, 16 | Complete |
| 18 | [Candidate checks](completed/18_candidate_checks.md) | 17 | Complete |
| 19 | [Apply, undo, audit](completed/19_apply_undo_audit.md) | 07, 17, 18 | Complete |
| 20 | [Desktop API client and state](completed/20_desktop_api_client_state.md) | 03, 11, 14, 16, 19 | Complete |
| 21 | [Desktop shell and explorer](completed/21_desktop_shell_explorer.md) | 20 | Complete |
| 22 | [Desktop summaries and symbol actions](completed/22_desktop_summaries_symbols.md) | 20, 21 | Complete |
| 23 | [Desktop generation and diff review](completed/23_desktop_generation_diff_review.md) | 20, 21, 22 | Complete |
| 24 | [Desktop context, status, history](completed/24_desktop_context_status_history.md) | 05, 07, 20, 21 | Complete |
| 25 | [Explain and prompt templates](completed/25_explain_and_prompt_templates.md) | 13, 20, 22 | Complete |
| 26 | [Impact preview and Git status](completed/26_impact_preview_git_status.md) | 08, 20, 22 | Complete |
| 27 | [Candidate comparison and report export](completed/27_candidate_comparison_report_export.md) | 13, 17, 20, 23 | Complete |
| 28 | [Desktop usability and accessibility](completed/28_desktop_usability_accessibility.md) | 21, 22, 23, 24 | Complete |
| 29 | [Retire the web UI](completed/29_retire_web_ui.md) | 03, 20, 21, 22, 23, 24, 28 | Complete |
| 30 | [Tooling and verification](completed/30_tooling_and_verification.md) | 03, 06, 11, 14, 19, 20, 29 | Complete |
| 31 | [Release acceptance](completed/31_release_acceptance.md) | 15, 25, 26, 27, 28, 29, 30 | Complete |
| 32 | [Restore green validation baseline](completed/32_restore_green_validation_baseline.md) | 31 | Complete |
| 33 | [Repository configuration and version hygiene](completed/33_repository_config_version_hygiene.md) | 32 | Complete |
| 34 | [Structured project analysis](completed/34_structured_project_analysis.md) | 32, 33 | Complete |
| 35 | [Unified finding model and store](completed/35_unified_finding_model_store.md) | 34 | Complete |
| 36 | [Go verified project scan](completed/36_go_verified_project_scan.md) | 35 | Complete |
| 37 | [Project overview, scan, and findings APIs](completed/37_project_overview_findings_apis.md) | 34, 35, 36 | Complete |
| 38 | [Go declaration edit engine](completed/38_go_declaration_edit_engine.md) | 32 | Complete |
| 39 | [Editable draft lifecycle](completed/39_editable_draft_lifecycle.md) | 38 | Complete |
| 40 | [Draft validation, checks, Apply, and undo](completed/40_draft_validation_checks_apply.md) | 39 | Complete |
| 41 | [File-scoped chat sessions](completed/41_file_scoped_chat_sessions.md) | 38, 39, 40 | Complete |
| 42 | [Desktop shell decomposition](completed/42_desktop_shell_decomposition.md) | 32 | Complete |
| 43 | [Desktop API contracts](completed/43_desktop_api_contracts.md) | 37, 40, 41 | Complete |
| 44 | [Desktop workspace and workflow state](completed/44_desktop_workspace_state.md) | 42, 43 | Complete |
| 45 | [Desktop top-level navigation](completed/45_desktop_top_level_navigation.md) | 44 | Complete |
| 46 | [Desktop Project Summary](completed/46_desktop_project_summary.md) | 37, 43, 44, 45 | Complete |
| 47 | [Desktop Project Analysis](completed/47_desktop_analysis_workspace.md) | 43, 44, 45 | Complete |
| 48 | [Desktop Bugs workspace](completed/48_desktop_bugs_workspace.md) | 37, 43, 44, 45 | Complete |
| 49 | [Desktop Editor file/symbol brief](completed/49_desktop_editor_file_symbol_brief.md) | 44, 45 | Complete |
| 50 | [Desktop file-scoped chat](completed/50_desktop_file_scoped_chat.md) | 41, 43, 44, 49 | Complete |
| 51 | [Desktop editable declaration draft](completed/51_desktop_editable_declaration_draft.md) | 40, 43, 44, 50 | Complete |
| 52 | [Desktop draft review and Apply](completed/52_desktop_draft_review_apply.md) | 40, 51 | Complete |
| 53 | [Desktop accessibility and responsive interaction](completed/53_desktop_accessibility_responsive.md) | 45, 46, 47, 48, 49, 50, 51, 52 | Complete |
| 54 | [Desktop integration coverage](completed/54_desktop_integration_coverage.md) | 46, 47, 48, 49, 50, 51, 52, 53 | Complete |
| 55 | [API, migration, and release documentation](completed/55_api_docs_migration_release_notes.md) | 37, 41, 52, 54 | Complete |
| 56 | [Focused AI IDE release acceptance](completed/56_release_acceptance.md) | 33–55 | Complete |
| 57 | [Editor flow presentation contract](completed/57_editor_flow_presentation_contract.md) | 56 | Complete |
| 58 | [Retire legacy candidate Desktop UI](completed/58_retire_legacy_candidate_desktop_ui.md) | 57 | Complete |
| 59 | [Focus Flow theme foundation](completed/59_focus_flow_theme_foundation.md) | 58 | Complete |
| 60 | [Desktop top bar and workspace rail](completed/60_desktop_top_bar_workspace_rail.md) | 59 | Complete |
| 61 | [Responsive Desktop scaffold](completed/61_responsive_desktop_scaffold.md) | 60 | Complete |
| 62 | [Workbench project explorer](completed/62_workbench_project_explorer.md) | 61 | Complete |
| 63 | [Editor workspace stage navigation](completed/63_editor_workspace_stage_navigation.md) | 62 | Complete |
| 64 | [Editor Target stage](completed/64_editor_target_stage.md) | 63 | Complete |
| 65 | [Editor Draft stage](completed/65_editor_draft_stage.md) | 64 | Complete |
| 66 | [Selectable diff viewer](completed/66_selectable_diff_viewer.md) | 65 | Complete |
| 67 | [Editor Verify stage](completed/67_editor_verify_stage.md) | 66 | Complete |
| 68 | [Editor Apply and receipt stage](completed/68_editor_apply_receipt_stage.md) | 67 | Complete |
| 69 | [Project Summary visual refactor](completed/69_project_summary_visual_refactor.md) | 68 | Complete |
| 70 | [Analysis workspace visual refactor](completed/70_analysis_workspace_visual_refactor.md) | 69 | Complete |
| 71 | [Bugs workspace visual refactor](completed/71_bugs_workspace_visual_refactor.md) | 70 | Complete |
| 72 | [Command, dialog, and system states](completed/72_command_dialog_system_states.md) | 71 | Complete |
| 73 | [Accessibility, responsive, and performance polish](completed/73_accessibility_responsive_performance_polish.md) | 72 | Complete |
| 74 | [UI refactor integration acceptance](completed/74_ui_refactor_integration_acceptance.md) | 57–73 | Complete |
| 75 | [Analysis and Editor UX execution baseline](completed/75_analysis_editor_ux_execution_baseline.md) | 74 | Complete |
| 76 | [Analyze-all summary presentation](completed/76_analysis_run_summary_presentation.md) | 75 | Complete |
| 77 | [Analysis workspace summary UI](completed/77_analysis_workspace_summary_ui.md) | 76 | Complete |
| 78 | [Editor-only explorer and file navigation](completed/78_editor_only_explorer_navigation.md) | 77 | Complete |
| 79 | [Analysis and Editor UX acceptance](completed/79_analysis_editor_ux_acceptance.md) | 75–78 | Complete |
| 80 | [Compact semantic Desktop controls](completed/80_desktop_compact_semantic_controls.md) | 79 | Complete |
| 81 | [Exclusive no-project landing state](completed/81_desktop_no_project_landing.md) | 80 | Complete |
| 82 | [Technical metadata and copy simplification](completed/82_desktop_metadata_copy_simplification.md) | 81 | Complete |
| 83 | [Workspace density and visual-priority polish](completed/83_desktop_workspace_density_polish.md) | 82 | Complete |
| 84 | [Desktop UX refinement acceptance](completed/84_desktop_ux_refinement_acceptance.md) | 80–83 | Complete |
| 85 | [Editor symbol selection and inspection contracts](completed/85_editor_symbol_selection_contract.md) | 84 | Complete |
| 86 | [Interactive source and symbol inspector](completed/86_interactive_source_symbol_inspector.md) | 85 | Complete |
| 87 | [Direct selected-symbol editing](completed/87_direct_selected_symbol_editing.md) | 86 | Complete |
| 88 | [Contextual Editor Review flow](completed/88_contextual_editor_review_flow.md) | 87 | Complete |
| 89 | [Direct-symbol Editor UX acceptance](completed/89_direct_symbol_ux_acceptance.md) | 85–88 | Complete |
| 90 | [Active Editor file header](completed/90_editor_active_file_header.md) | 89 | Complete |
| 91 | [Remove Editor source instruction](completed/91_remove_editor_source_instruction.md) | 90 | Complete |
| 92 | [Hide empty Required imports](completed/92_hide_empty_required_imports.md) | 91 | Complete |
| 93 | [Group Bugs by priority](completed/93_group_bugs_by_priority.md) | 92 | Complete |
| 94 | [Desktop UX refinement acceptance](completed/94_desktop_ux_refinement_acceptance.md) | 90–93 | Complete |

## Delivery boundaries

- Tasks 01–31 are the completed desktop-only safety foundation and release history.
- Tasks 32–33 restore a trustworthy validation, configuration, and version baseline.
- Tasks 34–37 deliver structured project intelligence, findings, and verified scans.
- Tasks 38–41 deliver Go declaration drafts and file-scoped chat backend contracts.
- Tasks 42–49 restructure Desktop and add Summary, Analysis, Bugs, and Editor.
- Tasks 50–52 deliver file chat, editable drafts, and the final review workflow.
- Tasks 53–56 complete accessibility, integration coverage, documentation, and release acceptance.
- Tasks 57–58 lock the staged presentation contract and remove the retired parallel Desktop UI.
- Tasks 59–62 install the Focus Flow visual system and Workbench shell/explorer.
- Tasks 63–68 deliver Target, Draft, Verify, and Apply as one guarded Editor flow.
- Tasks 69–72 bring every workspace, command surface, dialog, and system state into the design.
- Tasks 73–74 complete accessibility, responsive/performance polish, documentation, and acceptance.
- Task 75 records the approved Analysis/Editor UX contract and safe per-task commit workflow.
- Tasks 76–78 implement the Analysis summary, failures-only results, Editor-only chrome,
  and consistent file navigation.
- Task 79 completes regression, responsive, documentation, and commit-history acceptance.
- Task 80 installs the compact semantic action and single-line input foundation.
- Task 81 makes the no-project landing state exclusive and gates non-project shortcuts.
- Task 82 removes routine counts, revisions, hashes, and redundant explanatory copy.
- Task 83 consolidates status, filters, stage navigation, and responsive action groups.
- Task 84 completes regression, accessibility, documentation, and visual acceptance.
- Task 85 defines deterministic source-line selection, selected-symbol inspection, edit
  eligibility, and workflow progress contracts.
- Task 86 makes source clicks drive the contextual symbol inspector and removes the
  duplicate File analysis and all-symbol surfaces.
- Task 87 routes eligible selected symbols directly into guarded Replace editing and
  moves Create declaration into Commands.
- Task 88 replaces persistent Editor navigation buttons with contextual progress and a
  consolidated Review/check/Apply surface.
- Task 89 completes pointer, keyboard, responsive, accessibility, documentation, and
  regression acceptance for the direct-symbol workflow.
- Task 90 replaces visible Editor progress presentation with a prominent active-file
  header while retaining workflow routing state internally.
- Task 91 removes the source instruction subtitle without changing pointer, selection,
  keyboard, or accessibility behavior.
- Task 92 hides the Required imports control only when the editable draft has no imports.
- Task 93 makes high-to-low severity the default Bugs grouping while retaining
  card-level provenance.
- Task 94 completes integration, responsive, accessibility, documentation, and full
  regression acceptance for the focused Desktop UX refinement.

One implementation agent owns one ready task at a time. Read-only research or review
agents may work in parallel, but shared-worktree writers must not overlap. The current
Tasks 85–89 execute strictly in numeric order using
[`PROMPT_EXECUTE_ALL_TASKS.md`](PROMPT_EXECUTE_ALL_TASKS.md). The execution prompt must
post a user-facing start and completion commentary update for every task.
After Task 89 is Complete and its overlapping work has a stable commit boundary, Tasks
90–94 execute strictly in numeric order using
[`PROMPT_EXECUTE_DESKTOP_UX_REFINEMENTS.md`](PROMPT_EXECUTE_DESKTOP_UX_REFINEMENTS.md),
with exactly one reviewed local commit after each completed task.
No task may introduce direct source editing, autonomous multi-file edits, silent
writes, background fixes, or background agents that mutate a project. Tasks 90–94
authorize only their specified local commits and never authorize a push.
