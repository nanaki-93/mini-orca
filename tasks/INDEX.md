# Mini-Orca improvement — task index

## Objective

Implement the complete scope in [`Plan.md`](../Plan.md): secure project importing and AI analysis, project/file information, symbol-scoped generation with project-wide context, and a minimal Compose Desktop client matching the original IDE style.

## Task list

### Phase 1 — Correctness and security

| # | Task | Status | Depends on |
|---|---|---|---|
| 1.1 | [Fix human-gate concurrency and server timeouts](01_01_fix_concurrency_and_timeouts.md) | Complete | — |
| 1.2 | [Enforce canonical project path boundaries](01_02_secure_project_paths_and_file_reads.md) | Complete | — |
| 1.3 | [Harden shell and project executors](01_03_harden_shell_and_executors.md) | Complete | — |
| 1.4 | [Fix detection, history, and web interaction defects](01_04_fix_detection_history_and_web_bugs.md) | Complete | 1.1, 1.2, 1.3 |

### Phase 2 — Project analysis backend

| # | Task | Status | Depends on |
|---|---|---|---|
| 2.1 | [Create the thread-safe active-project manager](02_01_create_active_project_manager.md) | Complete | 1.2 |
| 2.2 | [Scan projects and collect metadata](02_02_scan_project_metadata.md) | Complete | 1.2 |
| 2.3 | [Build bounded project-wide context](02_03_build_project_context.md) | Complete | 2.2 |
| 2.4 | [Generate and persist the AI analysis file](02_04_generate_ai_analysis.md) | Complete | 2.2, 2.3 |
| 2.5 | [Expose project and file-information APIs](02_05_add_project_api.md) | Complete | 2.1, 2.4 |

### Phase 3 — Atomic code generation

| # | Task | Status | Depends on |
|---|---|---|---|
| 3.1 | [Require target file and symbol in requests](03_01_define_atomic_generation_contract.md) | Complete | 2.1, 2.5 |
| 3.2 | [Create the one-symbol generation prompt](03_02_create_atomic_generation_prompt.md) | Complete | 2.3, 3.1 |
| 3.3 | [Guarantee safe one-file output handling](03_03_enforce_single_file_output.md) | Complete | 1.2, 1.3, 3.2 |
| 3.4 | [Update the web IDE for symbol-scoped generation](03_04_update_web_atomic_controls.md) | Complete | 3.1, 3.2 |

### Phase 4 — Kotlin Compose Desktop client

| # | Task | Status | Depends on |
|---|---|---|---|
| 4.1 | [Create the Gradle/Compose scaffold and API client](04_01_create_desktop_scaffold_and_api_client.md) | Complete | 2.5, 3.1 |
| 4.2 | [Build the desktop shell and project import flow](04_02_build_desktop_shell_and_import.md) | Complete | 4.1 |
| 4.3 | [Add project explorer, viewer, and metadata](04_03_add_desktop_explorer_and_info.md) | Complete | 4.2, 2.5 |
| 4.4 | [Add the atomic generation panel](04_04_add_desktop_atomic_generator.md) | Complete | 4.2, 3.2 |

### Phase 5 — Verification and documentation

| # | Task | Status | Depends on |
|---|---|---|---|
| 5.1 | [Add and run backend regression coverage](05_01_verify_backend.md) | Complete | Phases 1–3 |
| 5.2 | [Build and verify the desktop module](05_02_verify_desktop.md) | Complete | Phase 4 |
| 5.3 | [Update documentation and version information](05_03_update_documentation_and_version.md) | Complete | 5.1, 5.2 |

## Dependency flow

```text
Phase 1 ──► Phase 2 ──► Phase 3 ──► Phase 4
   └───────────────► Phase 5 ◄───────────┘
```

Phase 1 tasks may be worked in parallel. Within later phases, follow the explicit dependency column.
