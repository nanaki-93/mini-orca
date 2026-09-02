# 76 — Add the Analyze-all summary presentation model

## Status

Pending

## Goal

Provide one tested, pure source of truth for Analysis coverage, current/last-run totals,
and failures so Compose does not classify job-file states itself.

## Depends on

Task 75.

## Implementation

- Add a cohesive presentation type in `AnalysisWorkspaceState.kt` for:
  - project coverage totals from `AnalysisCoverage`;
  - job status and configured file/retry limits;
  - candidate, completed, failed, running, and remaining counts;
  - deterministic failure rows containing relative path, attempts, and display error.
- Recognize the daemon job-file states `pending`, `running`, `completed`, and `failed`.
- Treat a nonblank error as a failure even when status is blank or unknown.
- Use a safe fallback such as `Analysis failed` only when a failed row has no error text.
- Clamp derived remaining counts to zero and retain backend file order.
- Update lifecycle presentation wording so it refers to summary/error visibility rather
  than successful per-file results “below.”
- Keep run-option bounds and polling behavior unchanged.
- Do not change serialized models or daemon/API contracts.

## Acceptance criteria

- Coverage counts and job-run counts cannot be conflated.
- Empty/no-job, running, paused, completed, canceled, stale, and failed jobs have stable
  textual presentation.
- Mixed completed/failed/running/pending input yields exact totals.
- Error-bearing unknown states appear in the failure list.
- Successful, running, and pending files without errors do not appear as failures.

## Verification

- Extend `AnalysisWorkspaceStateTest` for every classification and empty-state rule.
- Run `./desktop/gradlew -p desktop test`.
- Run `git diff --check` and inspect the task diff.

## Commit

After all criteria pass, move this task to `tasks/completed/`, update its index row, stage
only Task 76 hunks, inspect `git diff --cached`, and create exactly this commit:

```text
feat(desktop): summarize analyze-all results
```
