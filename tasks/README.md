# Mini-Orca Cleanup Tasks

Individual task files for the Mini-Orca cleanup project. Each task is derived from `TASKS.md` and verified against `CLEANUP_PLAN.md`.

## Task Organization

### Phase 1: Remove Session System (7 tasks)
| Task | File | Status |
|------|------|--------|
| 1.1 | [TASK-1.1.md](TASK-1.1.md) | Delete session type definitions |
| 1.2 | [TASK-1.2.md](TASK-1.2.md) | Delete session HTTP handlers |
| 1.3 | [TASK-1.3.md](TASK-1.3.md) | Delete session API types |
| 1.4 | [TASK-1.4.md](TASK-1.4.md) | Delete gate store |
| 1.5 | [TASK-1.5.md](TASK-1.5.md) | Simplify state store |
| 1.6 | [TASK-1.6.md](TASK-1.6.md) | Update orchestrator |
| 1.7 | [TASK-1.7.md](TASK-1.7.md) | Update main.go |

### Phase 2: Remove Project System (5 tasks)
| Task | File | Status |
|------|------|--------|
| 2.1 | [TASK-2.1.md](TASK-2.1.md) | Delete project handlers |
| 2.2 | [TASK-2.2.md](TASK-2.2.md) | Delete project types |
| 2.3 | [TASK-2.3.md](TASK-2.3.md) | Delete project files handler |
| 2.4 | [TASK-2.4.md](TASK-2.4.md) | Update render handler |
| 2.5 | [TASK-2.5.md](TASK-2.5.md) | Update file tree handler |

### Phase 3: Remove Skills System (5 tasks)
| Task | File | Status |
|------|------|--------|
| 3.1 | [TASK-3.1.md](TASK-3.1.md) | Delete skills directory |
| 3.2 | [TASK-3.2.md](TASK-3.2.md) | Delete skills API handler |
| 3.3 | [TASK-3.3.md](TASK-3.3.md) | Delete skills API types |
| 3.4 | [TASK-3.4.md](TASK-3.4.md) | Simplify config |
| 3.5 | [TASK-3.5.md](TASK-3.5.md) | Update agent registry |

### Phase 4: Simplify UI Components (7 tasks)
| Task | File | Status |
|------|------|--------|
| 4.1 | [TASK-4.1.md](TASK-4.1.md) | Delete unused component templates |
| 4.2 | [TASK-4.2.md](TASK-4.2.md) | Delete unused phase templates |
| 4.3 | [TASK-4.3.md](TASK-4.3.md) | Delete unused editor template |
| 4.4 | [TASK-4.4.md](TASK-4.4.md) | Delete unused CSS files |
| 4.5 | [TASK-4.5.md](TASK-4.5.md) | Simplify base template |
| 4.6 | [TASK-4.6.md](TASK-4.6.md) | Simplify IDE template |
| 4.7 | [TASK-4.7.md](TASK-4.7.md) | Simplify template engine functions |

### Phase 5: Simplify Orchestrator (5 tasks)
| Task | File | Status |
|------|------|--------|
| 5.1 | [TASK-5.1.md](TASK-5.1.md) | Simplify orchestrator Run() |
| 5.2 | [TASK-5.2.md](TASK-5.2.md) | Remove complex phase router |
| 5.3 | [TASK-5.3.md](TASK-5.3.md) | Simplify history tracking |
| 5.4 | [TASK-5.4.md](TASK-5.4.md) | Simplify retry logic |
| 5.5 | [TASK-5.5.md](TASK-5.5.md) | Update human gate |

### Phase 6: Add Chat API (3 tasks)
| Task | File | Status |
|------|------|--------|
| 6.1 | [TASK-6.1.md](TASK-6.1.md) | Create chat types |
| 6.2 | [TASK-6.2.md](TASK-6.2.md) | Create chat handler |
| 6.3 | [TASK-6.3.md](TASK-6.3.md) | Register chat routes |

### Phase 7: Update UI for Chat (4 tasks)
| Task | File | Status |
|------|------|--------|
| 7.1 | [TASK-7.1.md](TASK-7.1.md) | Add chat panel to IDE layout |
| 7.2 | [TASK-7.2.md](TASK-7.2.md) | Create chat UI components |
| 7.3 | [TASK-7.3.md](TASK-7.3.md) | Add chat JavaScript |
| 7.4 | [TASK-7.4.md](TASK-7.4.md) | Add review modal |

### Phase 8: Final Cleanup & Testing (6 tasks)
| Task | File | Status |
|------|------|--------|
| 8.1 | [TASK-8.1.md](TASK-8.1.md) | Update base template functions |
| 8.2 | [TASK-8.2.md](TASK-8.2.md) | Remove test files for deleted code |
| 8.3 | [TASK-8.3.md](TASK-8.3.md) | Rewrite remaining tests |
| 8.4 | [TASK-8.4.md](TASK-8.4.md) | Build and test |
| 8.5 | [TASK-8.5.md](TASK-8.5.md) | Clean up build artifacts |
| 8.6 | [TASK-8.6.md](TASK-8.6.md) | Update documentation |

## Task Structure

Each task file contains:
- **Phase**: Which cleanup phase this belongs to
- **Status**: Pending / In Progress / Complete
- **Goal**: What needs to be accomplished
- **CLEANUP_PLAN Verification**: How this task fulfills the CLEANUP_PLAN requirements
- **Files**: Files to create, edit, or delete
- **Impact**: What areas are affected
- **Dependencies**: Task ordering (Before/After)
- **Risk**: Risk level (Low/Medium/High)
- **Checklist**: Step-by-step verification items

## Execution Order

Tasks must be executed in order within each phase, and phases must be completed sequentially:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7 → Phase 8
   ↓          ↓          ↓          ↓          ↓          ↓          ↓         ↓
 1.1-1.7    2.1-2.5    3.1-3.5    4.1-4.7    5.1-5.5    6.1-6.3    7.1-7.4   8.1-8.6
```

## Total Tasks: 42
