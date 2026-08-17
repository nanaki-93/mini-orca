# Mini-Orca desktop-only implementation tasks

Source roadmap: [`../docs/NEXT_FUNCTIONS_FIXES_UI.md`](../docs/NEXT_FUNCTIONS_FIXES_UI.md)

## Execution order

| Order | Task | Depends on | Status |
|---:|---|---|---|
| 01 | [Product and workflow contract](completed/01_product_workflow_contract.md) | — | Complete |
| 02 | [Configured application service](completed/02_configured_application_service.md) | 01 | Complete |
| 03 | [API contract and documentation](completed/03_api_contract_and_documentation.md) | 01, 02 | Complete |
| 04 | [Context policy](completed/04_context_policy.md) | 01 | Complete |
| 05 | [Context inspector data](completed/05_context_inspector_data.md) | 04 | Complete |
| 06 | [Cancellation and timeouts](06_cancellation_and_timeouts.md) | 02 | Pending |
| 07 | [Project revision and state](07_project_revision_and_state.md) | 02 | Pending |
| 08 | [Deterministic index persistence](08_deterministic_index_persistence.md) | 04, 07 | Pending |
| 09 | [Go symbols](09_go_symbol_extraction.md) | 08 | Pending |
| 10 | [Generic symbols](10_generic_symbol_extraction.md) | 08 | Pending |
| 11 | [Index and symbol APIs](11_index_and_symbol_apis.md) | 08, 09, 10 | Pending |
| 12 | [File-analysis cache](12_file_analysis_cache.md) | 04, 07, 08 | Pending |
| 13 | [Semantic file analysis](13_semantic_file_analysis.md) | 02, 05, 09, 10, 12 | Pending |
| 14 | [File-analysis APIs](14_file_analysis_apis.md) | 12, 13 | Pending |
| 15 | [Sequential analyze-all](15_sequential_analyze_all.md) | 14 | Pending |
| 16 | [Generation response contract](16_generation_response_contract.md) | 02, 06, 07 | Pending |
| 17 | [Scope validation and diff](17_scope_validation_and_diff.md) | 09, 16 | Pending |
| 18 | [Candidate checks](18_candidate_checks.md) | 17 | Pending |
| 19 | [Apply, undo, audit](19_apply_undo_audit.md) | 07, 17, 18 | Pending |
| 20 | [Desktop API client and state](20_desktop_api_client_state.md) | 03, 11, 14, 16, 19 | Pending |
| 21 | [Desktop shell and explorer](21_desktop_shell_explorer.md) | 20 | Pending |
| 22 | [Desktop summaries and symbol actions](22_desktop_summaries_symbols.md) | 20, 21 | Pending |
| 23 | [Desktop generation and diff review](23_desktop_generation_diff_review.md) | 20, 21, 22 | Pending |
| 24 | [Desktop context, status, history](24_desktop_context_status_history.md) | 05, 07, 20, 21 | Pending |
| 25 | [Explain and prompt templates](25_explain_and_prompt_templates.md) | 13, 20, 22 | Pending |
| 26 | [Impact preview and Git status](26_impact_preview_git_status.md) | 08, 20, 22 | Pending |
| 27 | [Candidate comparison and report export](27_candidate_comparison_report_export.md) | 13, 17, 20, 23 | Pending |
| 28 | [Desktop usability and accessibility](28_desktop_usability_accessibility.md) | 21, 22, 23, 24 | Pending |
| 29 | [Retire the web UI](29_retire_web_ui.md) | 03, 20, 21, 22, 23, 24, 28 | Pending |
| 30 | [Tooling and verification](30_tooling_and_verification.md) | 03, 06, 11, 14, 19, 20, 29 | Pending |
| 31 | [Release acceptance](31_release_acceptance.md) | 15, 25, 26, 27, 28, 29, 30 | Pending |

## Delivery boundaries

- Tasks 01–19 create the safe backend and persistence contract.
- Tasks 20–24 deliver the core desktop-only workflow.
- Tasks 25–28 add the roadmap's focused follow-on functions and polish.
- Tasks 29–31 remove the legacy web interface and verify the release.

No task may introduce autonomous multi-file edits, silent writes, automatic commits, or background agents that mutate a project.
