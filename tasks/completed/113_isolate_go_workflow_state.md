# 113 — Isolate Go workflow state

## Status

Complete

## Goal

Give every mutable Go job, draft, and chat collection one cohesive owner, reduce
application-service complexity, and remove dead helpers exposed by the cleanup.

## Depends on

Task 112.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 113` and name the
  current state/mutex inventory, target owners, complexity hotspots, race tests, and
  existing changes.
- After the commit, post a separate update beginning with `Task 113 complete` and
  report ownership/lock boundaries, dead-code cleanup, commands/results, exact commit
  hash, and that Task 114 is next.

## Implementation

- Inventory every mutable map, current job, session, draft, and cached model runtime
  field on `app.Service`, including which goroutines read/write it and which clone
  functions guard ownership.
- Keep `app.Service` as the concrete orchestration façade used by handlers.
- Move mutable state into cohesive concrete owners for scoped runtimes, analysis jobs,
  Go scan state, drafts, and chat sessions where those concerns are currently mixed.
- Give each owner one mutex and enforce its invariants through methods. Do not expose
  locks or mutable internal records to handlers.
- Never hold a lock while calling a model, running a command, walking the filesystem,
  persisting data, or writing an HTTP response.
- Preserve defensive copies that prevent concurrent mutation. Remove only unused or
  exact duplicate clone functions; do not add reflection or generic deep-copy code.
- Share job transition helpers only where analysis and scan states have identical
  semantics. Keep separate types where pause/resume/cancel/progress rules differ.
- Decompose remaining high-complexity app functions into named validate, load,
  execute, normalize, persist, and publish stages with straight-line success paths.
- Remove dead app, project, config, logging, and LLM helpers revealed after Tasks
  104–112. Remove `logging.Debug`, `With`, `GetLogger`, or placeholder context helpers
  only after confirming no live caller.
- Do not add interfaces for concrete state owners or a generic job framework.

## Acceptance criteria

- Every retained mutable collection/job has one named owner and one visible lock boundary.
- No lock is held across external I/O or command/model execution.
- Pause, resume, cancel, progress, stale-response, and terminal-state behavior remains covered.
- Static analysis reports no unexplained unused helper in the changed packages.
- The application service coordinates features without becoming a forwarding-only
  abstraction hierarchy.

## Verification

Run:

```text
go test ./internal/app ./internal/project ./internal/api/handlers
make fmt-check
go test ./...
make test-race
make vet
git diff --check
```

Run the available staticcheck/dead-code commands from the Task 103 baseline and inspect
all changed ownership code for lock-across-I/O paths.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 113 changes, inspect the staged diff, and create
exactly one commit:

```text
refactor(app): isolate workflow state
```

Do not amend, squash, tag, or push the commit.
