# 137 — Add bounded project performance jobs and API

## Status

Pending

## Goal

Run explicit sequential file reviews as a guarded project job with honest aggregate
coverage, local persistence, cancellation, and a complete loopback API contract.

## Depends on

Task 136.

## Required task commentary

- Before editing, post `Starting Task 137` and describe queue identity, lifecycle,
  concurrency, budgets, aggregate/API boundaries, and race/contract tests.
- After the commit, post `Task 137 complete` with results, exact commit hash, and
  Task 138 as the next task.

## Implementation

- Implement explicit Start/Pause/Resume/Cancel and read-only report/job/queue-preview
  operations using Task 136's file review. Import, restore, reindex, normal analysis,
  and cached reads must not start Performance work.
- Capture project ID/revision, canonical root, policy fingerprint, exact queued paths
  and file hashes, run limits, model/provider/prompt identity, and a unique job ID/
  generation. Reject changed preview inputs before starting or resuming.
- Reuse existing file-count defaults/caps (currently 100/500), with explicit limits
  and truthful excluded/oversized/outside-limit reasons. Default total run budget
  is 15 minutes and maximum 60 minutes; clamp every request to its remaining budget.
- Define cumulative budget accounting across Pause/Resume and restart: waiting while
  paused does not consume execution time, but resume cannot reset spent execution
  time or attempt counters. Persist accounting conservatively and test recovery.
- Run one file at a time with one owner for bounded retries/backoff. Pause permits
  the active file to finish and prevents the next; Cancel interrupts active work.
  Budget exhaustion, failures, and cancellation retain valid partial reports.
- Block a second project-wide background review while Analyze-all or Performance
  is active, symmetrically at both entry points. Do not cancel the other job or
  replace its state. Preserve normal Analyze-all behavior when Performance is idle.
- Guard persistence/publication with both captured source identity and job generation.
  A canceled old response cannot overwrite a new run at the same revision or write
  into another project's root. Stop stale source/policy/revision runs explicitly.
- Persist source-free progress at `.mini-orca/sessions/performance-job.json` using
  atomic storage/recovery patterns. On restart restore interrupted work as paused/
  interrupted, never auto-resume provider work. Resume is explicit and reconfirms
  remote scope; mismatched jobs require a new review.
- Derive project report and category totals from accepted/reused file report identities
  in the current queue, not a second persisted summary or another LLM request.
- Reconcile indexed, eligible, selected, reviewed, cached, failed, skipped, pending,
  and outside-limit counts. Separate completed queue from complete project coverage,
  and successful-empty from unavailable/failed/all-invalid output.
- Register the seven routes proposed in feature plan section 7.3: cached report,
  context preview, job read/start, and pause/resume/cancel. Guard control requests
  with expected project/job identity; enforce request/response/page bounds.
- Use existing structured errors and conflict conventions, strict JSON, loopback
  defaults, and request-specific remote confirmation for the entire previewed queue.
  Confirmation for another analysis/run/project is not authorization.
- Update API/OpenAPI, route registration tests, Go/Kotlin wire models and client
  methods together. Label routing as Performance using the Analysis model; update
  configuration comments/docs without adding a required profile or local config.
- Extract shared policy only at concrete common boundaries. Do not duplicate the
  Analyze-all controller wholesale or build a general-purpose job/agent engine.

## Acceptance criteria

- All new routes work and document the exact implemented payloads and errors.
- Read-only operations have no provider side effects, and every run/resume is
  explicitly scoped, bounded, cancelable, and safely restorable.
- Concurrent starts, retry exhaustion, pause/resume, project switches, late results,
  and source changes cannot corrupt reports or affect another job's authority.
- Coverage and status never imply measured performance or unreviewed project safety.
- Performance does not retire bug findings, overwrite normal caches, or add source
  writes, command execution, benchmark/profiler support, or a second Apply path.

## Verification

Use deterministic providers, fake/controlled clocks where needed, temporary roots,
and race-focused lifecycle tests. Cover budgets across resume/restart, idempotence/
conflicts, source/policy changes, canceled/replaced jobs, report identities, corrupt
storage, paged report consistency, privacy, route schemas, and client serialization.

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

## Commit

After checks pass, record evidence, mark this task Complete, move it under
`tasks/completed/`, update the index, inspect only Task 137 staged work, and create
exactly one commit:

```text
feat(performance): add bounded project review jobs
```

Do not amend, squash, tag, or push. Verify no Task 137 work remains uncommitted.
