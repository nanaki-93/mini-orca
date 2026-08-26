# Mini-Orca implementation tasks

Source roadmap: [`../PLAN.md`](../PLAN.md)

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
| 37 | [Project overview, scan, and findings APIs](37_project_overview_findings_apis.md) | 34, 35, 36 | Pending |
| 38 | [Go declaration edit engine](38_go_declaration_edit_engine.md) | 32 | Pending |
| 39 | [Editable draft lifecycle](39_editable_draft_lifecycle.md) | 38 | Pending |
| 40 | [Draft validation, checks, Apply, and undo](40_draft_validation_checks_apply.md) | 39 | Pending |
| 41 | [File-scoped chat sessions](41_file_scoped_chat_sessions.md) | 38, 39, 40 | Pending |
| 42 | [Desktop shell decomposition](42_desktop_shell_decomposition.md) | 32 | Pending |
| 43 | [Desktop API contracts](43_desktop_api_contracts.md) | 37, 40, 41 | Pending |
| 44 | [Desktop workspace and workflow state](44_desktop_workspace_state.md) | 42, 43 | Pending |
| 45 | [Desktop top-level navigation](45_desktop_top_level_navigation.md) | 44 | Pending |
| 46 | [Desktop Project Summary](46_desktop_project_summary.md) | 37, 43, 44, 45 | Pending |
| 47 | [Desktop Project Analysis](47_desktop_analysis_workspace.md) | 43, 44, 45 | Pending |
| 48 | [Desktop Bugs workspace](48_desktop_bugs_workspace.md) | 37, 43, 44, 45 | Pending |
| 49 | [Desktop Editor file/symbol brief](49_desktop_editor_file_symbol_brief.md) | 44, 45 | Pending |
| 50 | [Desktop file-scoped chat](50_desktop_file_scoped_chat.md) | 41, 43, 44, 49 | Pending |
| 51 | [Desktop editable declaration draft](51_desktop_editable_declaration_draft.md) | 40, 43, 44, 50 | Pending |
| 52 | [Desktop draft review and Apply](52_desktop_draft_review_apply.md) | 40, 51 | Pending |
| 53 | [Desktop accessibility and responsive interaction](53_desktop_accessibility_responsive.md) | 45, 46, 47, 48, 49, 50, 51, 52 | Pending |
| 54 | [Desktop integration coverage](54_desktop_integration_coverage.md) | 46, 47, 48, 49, 50, 51, 52, 53 | Pending |
| 55 | [API, migration, and release documentation](55_api_docs_migration_release_notes.md) | 37, 41, 52, 54 | Pending |
| 56 | [Focused AI IDE release acceptance](56_release_acceptance.md) | 33–55 | Pending |

## Delivery boundaries

- Tasks 01–31 are the completed desktop-only safety foundation and release history.
- Tasks 32–33 restore a trustworthy validation, configuration, and version baseline.
- Tasks 34–37 deliver structured project intelligence, findings, and verified scans.
- Tasks 38–41 deliver Go declaration drafts and file-scoped chat backend contracts.
- Tasks 42–49 restructure Desktop and add Summary, Analysis, Bugs, and Editor.
- Tasks 50–52 deliver file chat, editable drafts, and the final review workflow.
- Tasks 53–56 complete accessibility, integration coverage, documentation, and release acceptance.

One Air implementation agent owns one ready task at a time. Read-only research or
review agents may work in parallel, but shared-worktree writers must not overlap.
No task may introduce direct source editing, autonomous multi-file edits, silent
writes, automatic commits, or background agents that mutate a project.
