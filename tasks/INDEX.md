# Mini-Orca Simplification — Task Index

## Overview
**Goal**: Simplify mini-orca from a 6-phase plan-driven pipeline to a 4-phase single-feature workflow.
**Current**: `coding → testing → review → human_review → completed`
**Target**: `coding → testing → review → human_gate → done`

## Workflow Change
```
OLD: User sends goal → Plan generated → Plan reviewed → Units coded one-by-one → Test → Review → Human approve
NEW: User sends (project_path + feature prompt) → Code generated → Test → Review → Human approve
```

## Task List

### Phase 1: Data Model Simplification
| # | Task File | Description | Depends On |
|---|-----------|-------------|------------|
| 1.1 | `01_01_remove_planning_types.md` | Remove Plan, PlanUnit, AtomicUnit from state/session.go | — |
| 1.2 | `01_02_simplify_phase_types.md` | Remove planning phases from phase_router.go and session.go | 1.1 |
| 1.3 | `01_03_simplify_state_store.md` | Remove plan-related methods from state/store.go | 1.1, 1.2 |

### Phase 2: Orchestrator Refactoring
| # | Task File | Description | Depends On |
|---|-----------|-------------|------------|
| 2.1 | `02_01_rewrite_orchestrator.md` | Rewrite orchestrator.go for single-feature flow | 1.1, 1.2, 1.3 |
| 2.2 | `02_02_update_phase_router.md` | Simplify phase_router.go handlers and transitions | 1.2, 2.1 |
| 2.3 | `02_03_update_human_gate.md` | Verify human_gate.go works with new flow | 2.2 |

### Phase 3: Agent & Prompt Updates
| # | Task File | Description | Depends On |
|---|-----------|-------------|------------|
| 3.1 | `03_01_update_agent_orchestrator.md` | Update agent/orchestrator.go for prompt-based input | 2.1 |
| 3.2 | `03_02_update_coder_prompt.md` | Update prompts/coder.go for user request input | 3.1 |
| 3.3 | `03_03_update_reviewer_prompt.md` | Update prompts/reviewer.go to use prompt vs plan | 3.1 |

### Phase 4: API Layer Changes
| # | Task File | Description | Depends On |
|---|-----------|-------------|------------|
| 4.1 | `04_01_update_session_api.md` | Update api/session.go for new session creation | 3.1 |
| 4.2 | `04_02_update_gate_api.md` | Update api/gate.go for new flow | 2.2 |
| 4.3 | `04_03_update_main_daemon.md` | Simplify cmd/daemon/main.go server setup | 4.1, 4.2 |

### Phase 5: HTMX Frontend Updates
| # | Task File | Description | Depends On |
|---|-----------|-------------|------------|
| 5.1 | `05_01_update_htmx_render_handler.md` | Update htmx_render.go phase tracker and dashboard | 2.2 |
| 5.2 | `05_02_update_project_handler.md` | Update project handler if needed | — |
| 5.3 | `05_03_update_templates.md` | Update/delete HTML templates | 5.1 |

### Phase 6: Testing & Cleanup
| # | Task File | Description | Depends On |
|---|-----------|-------------|------------|
| 6.1 | `06_01_update_unit_tests.md` | Update all unit tests | 2.1 |
| 6.2 | `06_02_update_integration_tests.md` | Update integration tests | 6.1 |
| 6.3 | `06_03_update_documentation.md` | Update README, API docs, config docs | — |
| 6.4 | `06_04_final_cleanup.md` | Final cleanup, build, verify | 6.1, 6.2, 6.3 |

## Execution Order
```
Phase 1: 1.1 → 1.2 → 1.3
Phase 2: 2.1 → 2.2 → 2.3
Phase 3: 3.1 → 3.2 → 3.3
Phase 4: 4.1 → 4.2 → 4.3
Phase 5: 5.1 → 5.2 → 5.3
Phase 6: 6.1 → 6.2 → 6.3 → 6.4
```

Each task file contains:
- **Goal**: What this task accomplishes
- **Files to Modify**: Exact file paths
- **Files to Create/Delete**: New or removed files
- **Detailed Steps**: Step-by-step implementation instructions
- **Code Examples**: Before/after code snippets where helpful
- **Verification**: How to verify the task is complete
