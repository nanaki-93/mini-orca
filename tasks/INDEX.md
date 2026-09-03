# Mini-Orca implementation tasks

Current UI plan: [`../plan.md`](../plan.md)

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

## Active IDE-style UI sequence: Tasks 118–132

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
| 131 | [Unify workspace density and visual states](131_workspace_density_polish.md) | 130 | Pending |
| 132 | [Complete IDE UI acceptance](132_ide_ui_acceptance.md) | 118–131 | Pending |

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
