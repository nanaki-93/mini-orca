# Prompt template — execute all tasks sequentially

Copy this prompt to execute the complete task backlog in dependency order.

```text
You are executing the complete Mini-Orca improvement backlog, one task at a time.

Read, in this order:
1. Plan.md
2. tasks/README.md
3. tasks/INDEX.md
4. Every task file listed in tasks/INDEX.md, in phase and dependency order

Objective:
Deliver the Plan.md scope while keeping the repository simple, well structured, and clean:
- one Go daemon remains the backend and LLM integration point;
- imported projects are safely analyzed into `.mini-orca/analysis.md`;
- generation is constrained to one selected function/class in one selected file while receiving bounded project-wide context;
- the Kotlin Compose Desktop app remains one window, one API client, and no database or unnecessary layers.

Execution protocol:
1. Inspect the worktree before starting. Preserve all unrelated user changes.
2. Build a dependency-aware queue from tasks/INDEX.md.
3. Execute exactly one ready task at a time. Do not parallelize tasks.
4. Before each task, read its dependencies and inspect the source/tests it names.
5. For a Pending or In Progress task, implement only the documented scope.
6. For a Complete task, verify its acceptance criteria without reimplementing it. Repair only demonstrated regressions.
7. Run the task's focused verification commands immediately after that task.
8. Mark the task Complete and update tasks/INDEX.md only after its checks pass.
9. Proceed to the next ready task only when the preceding task is complete and clean.
10. Stop on a real blocker: report the task, evidence, failed command, and required user decision. Do not skip or fabricate completion.

Code-quality rules:
- Prefer deletion and simplification over new abstractions.
- Keep changes small and local; do not introduce a new framework, service, database, global mutable state, or duplicated API client.
- Use idiomatic Go and Kotlin, explicit error handling, focused functions, and meaningful names.
- Preserve canonical path and symlink protections, exact safe-shell matching, and preview-only code generation.
- Do not make generated code write automatically to user projects.
- Add a focused regression test for each bug fixed or behavior changed.
- Do not perform unrelated formatting, refactoring, dependency upgrades, or cleanup.

Verification gates:
- After Go changes: `go fmt ./...`, relevant `go test`, and `go vet` for affected packages.
- After concurrency, filesystem, executor, or security changes: relevant `go test -race`.
- After Compose Desktop changes: `gradle -p desktop test`.
- Before final completion: run every Phase 5 command, including `go test -race ./...`, `go vet ./...`, the desktop test, and `git diff --check`.

Status and reporting:
- Keep task files and tasks/INDEX.md truthful at all times.
- Do not mark a task Complete merely because most code exists; its acceptance criteria and checks must pass.
- At each task boundary, report the completed task, files changed, and verification result in one short progress update.
- At the end, provide a compact phase-by-phase summary, the final test results, and any intentionally deferred work.
```
