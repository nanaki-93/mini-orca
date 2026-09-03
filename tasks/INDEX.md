# Mini-Orca implementation tasks

Current cleanup plan: [`../PLAN.md`](../PLAN.md)

## Historical ledger: Tasks 01–102

Tasks 01–102 are completed implementation history. Their detailed task files,
obsolete execution prompts, and retired UI-mock material were consolidated in
Task 116. Git history remains the full, immutable archive; this ledger records
the delivery sequence without presenting pre-cleanup work as the active roadmap.

| Range | Completed delivery |
| --- | --- |
| 01–31 | Preview-first single-project Desktop safety foundation, API, configuration, and release baseline. |
| 32–56 | Restored validation/configuration hygiene; added project analysis, findings, scans, Go declaration drafts, checks, Apply/Undo, and Desktop workspaces. |
| 57–79 | Completed the focused Desktop presentation, responsive shell, Editor review flow, and visual acceptance. |
| 80–94 | Refined compact, accessible Desktop controls and direct selected-symbol editing. |
| 95–102 | Added strict scoped-model configuration, OpenAI-compatible routing, task repair, and scoped-model acceptance. |

## Active cleanup execution record

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

Tasks 103–117 execute strictly in numeric order through
[`PROMPT_EXECUTE_LEGACY_CLEANUP.md`](PROMPT_EXECUTE_LEGACY_CLEANUP.md). Each
task has one implementation writer, required user-facing start/completion
commentary, and exactly one isolated verified commit. The workflow never
authorizes direct source editing, automatic writes, commits, or pushes outside
the explicit reviewed Apply/Undo boundary.
