# Prompt template — execute one task

Copy this prompt and replace `{TASK_FILE}` with a task file from [`INDEX.md`](INDEX.md).

```text
You are implementing one task in the Mini-Orca repository.

Task file: tasks/{TASK_FILE}

Read, in this order:
1. Plan.md
2. tasks/README.md
3. tasks/INDEX.md
4. tasks/{TASK_FILE}
5. The source files and tests named by the task

Follow the task's dependencies. If a dependency is not complete or its acceptance criteria fail, stop and report the blocker; do not work around it with a duplicate implementation.

First inspect the current worktree and the existing implementation. The task status is evidence, not a substitute for verification:
- If the task is Pending or In Progress, implement only its stated scope.
- If the task is Complete, verify its acceptance criteria. Change code only when verification identifies a real regression.

Implementation rules:
- Keep Mini-Orca simple: use the existing Go daemon, handlers, project package, and Compose Desktop module. Do not introduce a new service, database, framework, state store, or abstraction unless the task explicitly requires it.
- Prefer the smallest clear change. Keep functions focused, names descriptive, error handling explicit, and comments limited to non-obvious decisions.
- Preserve the existing worktree. Do not overwrite, reset, revert, or remove unrelated user changes.
- Respect exact task boundaries, especially canonical project-path validation and the one-symbol/one-file generation constraint.
- Do not expand the task into unrelated cleanup or redesign.
- Add or adjust focused regression tests when behavior changes.

Verification rules:
- Run every verification command in the task that applies to the changed code.
- Run gofmt/go fmt for changed Go code, then go vet for affected packages.
- For concurrency or path/security changes, run the relevant race tests.
- For desktop changes, run `gradle -p desktop test`.
- Check `git diff --check` before completion.

Completion rules:
- Only mark the task Complete after all its acceptance criteria and verification checks pass.
- Update tasks/INDEX.md only if its status or dependency information changed.
- If blocked by missing authority, unavailable infrastructure, or a failing unrelated dependency, do not claim completion. State the exact blocker and the next safe action.

Finish with a concise report containing:
1. task identifier and final status;
2. files changed;
3. acceptance criteria verified;
4. commands run and their results;
5. any remaining blocker or follow-up.
```

Example task value: `02_04_generate_ai_analysis.md`.
