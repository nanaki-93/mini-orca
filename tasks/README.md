# Mini-Orca improvement tasks

This folder decomposes [`Plan.md`](../Plan.md) into independently executable implementation tasks.

## Task format

Every task includes:

- phase, status, priority, and risk;
- exact dependencies and affected files;
- implementation steps;
- acceptance criteria;
- focused verification commands.

Statuses reflect the current worktree. The plan implementation is complete, so all tasks are marked **Complete**. Reopen a task by changing its status to **Pending** when its acceptance criteria fail or its behavior must change.

## Execution rules

1. Follow dependencies in [`INDEX.md`](INDEX.md).
2. Keep the Go daemon as the only backend and LLM integration point.
3. Preserve the one-symbol/one-file generation boundary.
4. Do not weaken canonical path or symlink validation.
5. Run focused checks for each task and the complete Phase 5 verification before release.

## Prompt templates

- [Execute one task](PROMPT_EXECUTE_TASK.md)
- [Execute all tasks sequentially](PROMPT_EXECUTE_ALL_TASKS.md)

## Phases

| Phase | Theme | Tasks |
|---|---|---:|
| 1 | Correctness and security | 4 |
| 2 | Project analysis backend | 5 |
| 3 | Atomic code generation | 4 |
| 4 | Kotlin Compose Desktop client | 4 |
| 5 | Verification and documentation | 3 |
| **Total** |  | **20** |
