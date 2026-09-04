# 133 — Establish the insights and performance contract baseline

## Status

Pending

## Goal

Turn `docs/insights-performance/PLAN.md` into a verified implementation baseline
against the completed IDE shell before changing production behavior.

## Depends on

Task 132. Verify its completed state and implementation commit; retain any recorded
manual acceptance limitations without relabelling them as passed.

## Required task commentary

- Before editing, post `Starting Task 133` and describe the baseline, initial planning
  artifacts, likely tests, verification commands, and protected user-owned changes.
- After the commit, post `Task 133 complete` with results, exact commit hash, and
  Task 134 as the next task.

## Implementation

- Read the feature plan completely and inspect the finished IDE, API, model scopes,
  analysis/finding conversions, draft identity, caches, and job lifecycle.
- When execution starts, mark the feature plan In Progress and the index sequence
  active, replacing its preparation-only wording. Keep Tasks 134–139 Pending.
- Create `docs/insights-performance/BASELINE.md` with the starting HEAD, current
  behavior, relevant ownership boundaries, fixture matrix, and truthful automated/
  interactive check availability. Never persist user diffs, secrets, or raw prompts.
- Define test fixtures and acceptance cases for optional insights, same-call
  generation, disclosure-only UI, source-based performance, coverage, queue identity,
  provider confirmation, cache freshness, and exact optimization targets.
- Add missing behavior-focused characterization tests protecting current scopes,
  no-model-call navigation, stable findings/triage, draft invalidation, Apply/Undo,
  existing shortcuts, and the exact `1000dp` layout boundary. Reuse existing tests
  when they already cover the behavior; do not add redundant or source-text tests.
- Confirm every feature-plan requirement is owned by Tasks 134–139. Record concrete
  contract decisions in the feature plan where needed, including optional-field
  failure handling, request budgets, lifecycle states, and report pagination bounds.
- Keep prospective routes and schemas in design material until implemented. Do not
  advertise unregistered Performance routes in the live API/OpenAPI or change live
  response contracts solely to stage later work.
- This task owns validating and committing the initially uncommitted feature plan,
  Tasks 133–139, new execution prompt, and their README/index updates together with
  its baseline work. This is the only planning-artifact bootstrap commit.
- Do not add production insight generation, Performance services, new navigation,
  required configuration, or additional runtime dependencies in this task.

## Acceptance criteria

- Task 132 is complete and the actual IDE behavior is the baseline, not a stale mock.
- The plan, seven task files, index, README, and execution prompt agree on order,
  scope, safety, and the seven required commit subjects.
- Fixtures cover meaningful insights and trivial changes where insight is omitted,
  plus valid-empty, partial, failed, stale, and canceled performance outcomes.
- Existing product behavior is unchanged and characterized without speculative
  production scaffolding or failing placeholders for later tasks.
- Unrelated pre-existing edits remain unchanged and are excluded from staging.

## Verification

Run:

```text
make check
make quality
git diff --check
```

Check local documentation links, task numbering/dependencies, prompt subjects,
and the initial artifact ownership list. Record interactive unavailability in the
baseline; it must not be presented as an automated pass.

## Commit

After checks pass, record verification evidence in this task, mark it Complete,
move it to `tasks/completed/`, and update `tasks/INDEX.md`. Stage the exact initial
planning artifacts and Task 133 baseline work, inspect the full staged diff, and
create exactly one commit:

```text
test: establish insights and performance baseline
```

No planning-only, status-only, partial, or fixup commit. Do not amend, squash, tag,
or push. Record the commit hash in the conversation, not in its own committed file.
