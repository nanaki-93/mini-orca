# 139 — Complete insights and performance acceptance

## Status

Complete

## Goal

Verify the complete feature plan, remove superseded paths, update maintained
documentation, and close the seven-task sequence with reproducible evidence.

## Depends on

Tasks 133–138, in numeric order.

## Required task commentary

- Before editing, post `Starting Task 139` and describe the full diff audit, safety/
  privacy/regression matrix, content/UX acceptance, docs, and commit ledger checks.
- After the commit, post `Task 139 complete` with all results/limitations, exact
  commit hash, and that Tasks 133–139 are finished.

## Implementation

- Review the entire feature diff from the Task 133 baseline as a senior engineer:
  remove obsolete prompts/parsers, duplicate validation/state/presentation logic,
  unused foundation helpers, debug output, stale comments, and transitional branches.
  Do not absorb unrelated cleanup or rewrite the completed IDE implementation.
- Verify each feature-plan requirement and the complete bug/analysis/proposal/Review
  insight path, including no insight, malformed optional content, disclosure state,
  manual draft edits, stale owners, and canceled/late results.
- Verify the full Performance preview/start/pause/resume/cancel/restart path, provider
  confirmation, policy/budget boundaries, separate caches/findings, honest coverage,
  empty/partial/failed results, report paging, and optimization handoff.
- Verify read-only source/diff, one project/file/declaration, explicit discard,
  current validation/check/hash identity, exact Apply/Undo, and unchanged keyboard/
  responsive behavior end to end.
- Inspect source-free persisted metadata and logs with sentinel fixtures for prompts,
  source, credentials, and sanitized errors. Never use real secrets for testing.
- Run the plan's human content-quality review on representative bugs, improvements,
  analysis, and performance examples when a configured provider is available:
  concise advanced observations, grounded conditions/trade-offs, no filler, and
  appropriate omission for trivial changes. Keep deterministic tests provider-free.
- Repeat wide, exactly `1000dp`, narrow, text-scaling, keyboard, focus, and long-content
  acceptance. No quiz/game/tracking/Learn UI may remain.
- Record actual commands/results and manual evidence in
  `docs/insights-performance/ACCEPTANCE.md`. Any environment-dependent manual check
  that could not run must be explicit with a release-operator follow-up, never a pass.
- Update README, Desktop usage, API/OpenAPI, configuration scope descriptions/example
  comments, keyboard checklist, release notes, task README/index, and plan status.
  No new required local configuration or migration is expected; explain any actual
  cache/schema refresh behavior without introducing automatic provider work.
- Verify six prior task commits before staging the final task. After this commit,
  verify exactly seven sequence commits with the required ordered subjects. Record
  hashes in the conversation/final report; do not create an extra ledger-only commit
  to place Task 139's own hash into its committed file.
- Mark the feature plan Complete only when implementation and automated acceptance
  are complete. Put any permitted manual limitation adjacent to that status. Do not
  modify root `plan.md` to conceal an older IDE acceptance limitation.

## Acceptance criteria

- Every feature-plan criterion is implemented and verified, or has a narrowly scoped
  explicitly permitted environment-dependent manual follow-up.
- All required automated checks pass; no safety failure or real product defect is
  dismissed as an unavailable manual check.
- Insights are concise optional in-page panels; Performance is independent,
  source-based and explicitly unmeasured. No runtime execution or mutation scope was
  added, and no legacy duplicate implementation remains.
- Live contracts/docs match implementation, old insight-less metadata remains usable,
  and privacy/freshness/target guards are intact.
- The sequence has exactly one isolated commit per task with no amended, combined,
  pushed, or unrelated-user-work commit.

## Verification

Run all of:

```text
make fmt-check
go test ./...
make test-race
make vet
./desktop/gradlew -p desktop spotlessCheck detekt test
make check
make quality
git diff --check
```

Validate local documentation links, task statuses/dependencies, API route/schema
agreement, and the staged diff. Record each manual check as passed, failed, or not
run with reason. Failed required checks block completion.

## Commit

After checks pass, record evidence, mark this task Complete, move it under
`tasks/completed/`, update index/README/plan status, and inspect the entire Task 139
staged diff. Create exactly one commit:

```text
chore: complete insights and performance acceptance
```

Do not amend, squash, tag, or push. After committing, verify all seven task-owned
changes are committed, unrelated work remains preserved, and no extra commit is
needed. Report the seven exact hashes and final acceptance/limitations.

## Completion evidence

- Audited the six prior task commits and verified their exact ordered subjects before
  staging this acceptance record.
- Recorded automated, contract/privacy, and manual-release-operator evidence in
  `docs/insights-performance/ACCEPTANCE.md`; updated plan, README, desktop usage,
  keyboard checklist, API schema, and task ledger status.
- Passed `make fmt-check`, `go test ./...`, `make test-race`, `make vet`,
  `./desktop/gradlew -p desktop spotlessCheck detekt test`, `make check`, and
  `git diff --check`. The user-authorized `make quality` deferral is documented in
  the acceptance record; its remaining failure is repository-wide complexity output.
