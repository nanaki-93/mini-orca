# 138 — Add the Performance workspace and optimization handoff

## Status

Complete

## Goal

Add a separate Performance page beside Analysis for reviewing project opportunities
and explicitly preparing a safe single-declaration optimization.

## Depends on

Task 137.

## Required task commentary

- Before editing, post `Starting Task 138` and describe navigation, provider/coverage
  states, finding details, scoped handoff, responsiveness, and verification.
- After the commit, post `Task 138 complete` with results, exact commit hash, and
  Task 139 as the next task.

## Implementation

- Add Performance adjacent to Analysis in finished IDE navigation and command search.
  Keep it separate from Bugs/Problems and keep all right/bottom tab responsibilities.
- Extend workspace/layout/presenter state coherently. Composables render immutable
  state and emit intents; they do not start network work on composition/selection.
- Preserve existing `Cmd/Ctrl+1`–`4` destinations with explicit mappings. Update
  workspace cycling, command availability, navigation semantics, and project-close/
  switch state clearing without relying on changed enum ordinals.
- Show `Source-based review · Not measured` and `Performance · Analysis model` with
  actual model/destination/locality. Preview the queued context/limits before Start
  or Resume and obtain explicit remote confirmation for that run where needed.
- Provide compact Analyze performance, Pause/Resume/Cancel, run-limits disclosure,
  current/last run state, elapsed budget, coverage and errors. Poll only read-only
  progress while needed, with lifecycle cancellation and late-response rejection.
- Display reconciled eligible/selected/reviewed/reused/skipped/failed/outside-limit
  coverage, separating partial-project coverage from selected-queue completion.
- Render bounded/paged opportunities with category, potential impact, confidence,
  project-relative location, and title. Implement category/impact/path filters and
  stable impact/confidence/path/line/ID ordering consistent across page fetches.
- Row selection shows inline details: observed pattern, workload conditions,
  recommendation, trade-off, verification plan, freshness, and optional shared
  EngineeringInsightPanel. Do not invent measured metrics or a project score.
- Distinguish not-started, running, paused/interrupted, canceled, budget-limited,
  stale, failed, completed-with-errors, and successful-empty states. Do not present
  missing/failed review as no opportunities or imply all files were reviewed.
- Add explicit `Open in Editor`. `Prepare optimization` is enabled only for a fresh
  finding with one exact eligible Go declaration, and only prefills the existing
  bound request/composer after normal target/discard checks. It cannot generate,
  run checks, or apply automatically.
- Keep cross-file/non-Go/approximate opportunities analysis-only. Correctness checks
  and Apply never label an optimization a verified performance gain.
- Fit narrow layouts and text scaling with wrapped controls and selected details
  below the list. Reuse palette, focus/row/disclosure patterns and status summaries;
  do not add another docking system or stretch the source mutation scope.
- Update Desktop usage and keyboard/manual smoke instructions for the shipped page.

## Acceptance criteria

- Performance is independently discoverable without changing existing navigation or
  initiating work on entry. Cached and in-progress results remain readable.
- Provider confirmation matches the full bounded run; stale preview/results and
  project switches cannot operate against the wrong project/job.
- Findings, filters, pagination, honest coverage, and insight details are consistent,
  compact, keyboard reachable, and usable below/at/above `1000dp`.
- Optimization preparation uses existing exact-target safeguards and remains an
  explicit preview-first workflow with no measured-performance claims.

## Verification

Test presenter intent/cancellation ownership, status/filter/page mappings, scope
confirmation, stale/late results, keyboard mappings, disclosure, responsive states,
coverage labels, and eligibility/discard behavior. Verify navigation/display actions
make zero generation/Apply/check calls.

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
make check
git diff --check
```

Perform keyboard, viewport, scaling, long-path, provider, partial-run and optimization
handoff smoke checks when available. Record unavailable interactive steps for final
acceptance; do not fabricate success or bypass provider confirmation to test them.

## Commit

After checks pass, record evidence, mark this task Complete, move it under
`tasks/completed/`, update the index, inspect only Task 138 staged work, and create
exactly one commit:

```text
feat(desktop): add Performance workspace
```

Do not amend, squash, tag, or push. Verify no Task 138 work remains uncommitted.

## Completion evidence

- Added a separate, command-search-discoverable Performance workspace with explicit
  queue preview, Analysis-model confirmation, read-only progress polling, honest
  source-based coverage, filters, selected rationale, and insight disclosure.
- Preserved `Cmd/Ctrl+1`–`4`; Performance participates only in explicit workspace
  cycling and command navigation. The handoff opens the existing Editor/composer and
  remains limited to one exact Go declaration.
- Added finding-ID-to-path report metadata so an opportunity can open its validated
  project-relative location without exposing source text in the project report.
- Passed `./desktop/gradlew -p desktop spotlessCheck detekt test`, `make check`, and
  `git diff --check`. Interactive wide/narrow/scale smoke checks remain recorded in
  the keyboard checklist for final acceptance. `make quality` still exits nonzero on
  the repository-wide existing complexity threshold; the Task 136 quality deferral
  remains in effect.
