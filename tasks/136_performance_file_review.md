# 136 — Implement bounded source-based performance file review

## Status

Pending

## Goal

Build the policy-safe review operation and separate file-report cache that the
project Performance job will use, without running project code or claiming metrics.

## Depends on

Task 135.

## Required task commentary

- Before editing, post `Starting Task 136` and name the performance schema, context
  budget, provider route, cache/freshness boundary, and focused verification.
- After the commit, post `Task 136 complete` with results, exact commit hash, and
  Task 137 as the next task.

## Implementation

- Implement cohesive performance report/finding types, strict model output parsing,
  bounded field validation, and a single-file review operation under current
  `internal/project` and `internal/app` ownership conventions.
- Cover CPU/complexity, memory, I/O, concurrency, caching, and UI/rendering when
  supported by the supplied source. Include workload conditions, observed pattern,
  recommendation, trade-off, verification plan, confidence, potential impact,
  location, and optional shared Engineering insight.
- Distinguish source observations from model hypotheses. Do not infer actual hot
  paths from file size, loops alone, names, signatures, or correctness test outcomes.
- Use the existing `analyze` model profile explicitly, with actual provider/model/
  reasoning/timeout provenance and existing confirmation checks. No new required
  scope or silent cross-scope fallback.
- Reuse eligible indexed paths, source traversal, ContextPolicy, and safe resolution.
  Build one file plus bounded source-free project facts per call; the manifest
  must describe exactly what can be sent. No arbitrary neighboring source expansion.
- Bound source to 64 KiB and configured context capacity after reserving instruction/
  output budget. Skip oversized/ineligible files with reasons; do not silently
  truncate them while claiming a complete review.
- Bound output to 64 KiB, five findings, and stricter provider limits. Validate
  locations/symbols against supplied source inventory and bind path/hash/identity
  server-side. Never accept a model-directed target file.
- A valid empty array means no opportunities identified in this reviewed file.
  Dropped invalid findings produce a partial-output warning; an all-invalid nonempty
  response fails the review rather than becoming an empty success.
- Validate/discard malformed optional insight using Task 134's shared policy, not
  a new implementation. Do not relax required performance finding validation.
- Persist sanitized source-free reports atomically under
  `.mini-orca/performance/files/<safe-path-hash>.json`, with prompt/schema/model/
  provider/reasoning/context-policy and source identity freshness inputs.
- Provide safe cached reads, explicit refresh semantics, corrupt-cache recovery,
  and immutable snapshots. Do not overwrite or reconcile the bug-analysis cache or
  `UnifiedFinding`'s generic `ai` source.
- Recheck source identity before accepting a response. Keep review preparation,
  generation, and guarded publication separable enough for Task 137 to reject a
  canceled job's late result without persisting it over a newer one.
- Set one retry owner, using the existing bounded policy without nested multiplied
  retries. Task 137 supplies the remaining run deadline and attempt budget.
- This is an internal foundation: no registered Performance routes, background
  project worker, new UI navigation, benchmark/profiler execution, or optimization
  generation in this task. Keep the foundation small and testable, not a framework.

## Acceptance criteria

- A review uses only allowed bounded context and yields honest source-based findings
  with valid anchors, known provenance, and no measured-performance claims.
- Empty, partial, invalid, canceled, changed-source, unsupported, and oversized input
  states are distinct and tested.
- Cache/report storage never includes raw source, prompts, credentials, or unsanitized
  output, and cannot overwrite ordinary analysis or bug findings.
- Existing application behavior and live routes remain usable and unchanged.

## Verification

Test parser/field limits, Unicode insight limits, real source anchors, unsafe paths,
policy exclusions and symlinks, remote routing/confirmation, prompt/output budgets,
retry counts, cancellation, source changes, cache invalidation, old/missing/corrupt
metadata, cloning, and sanitized persistence using fake providers/temp fixtures.

```text
make fmt-check
go test ./...
make test-race
make vet
make quality
git diff --check
```

## Commit

After checks pass, record evidence, mark this task Complete, move it under
`tasks/completed/`, update the index, inspect only Task 136 staged work, and create
exactly one commit:

```text
feat(performance): add source-based file review
```

Do not amend, squash, tag, or push. Verify no Task 136 work remains uncommitted.
