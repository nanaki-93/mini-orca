# Go daemon and evaluation code

Read [../AGENTS.md](../AGENTS.md) first. These rules cover `internal/` and are also
the backend guide for changes to `cmd/`.

## Boundaries and contracts

- Target Go 1.22; check `go.mod` before using newer language or library features.
  Keep production Go code in `cmd/` and `internal/`. Entry points wire dependencies;
  handlers decode/validate requests, call workflow owners and map errors.
- Reuse `internal/api` request decoding and structured errors. Preserve size
  limits, unknown-field/trailing-value rejection and local request policy. Use
  typed errors or `errors.Is`/`errors.As` where available, with useful `%w` context.
- Keep shared workflow policy in `app` and project facts/composition in `project`.
  Avoid placing business rules in HTTP handlers or expanding `Service` with
  unrelated responsibilities when an existing cohesive owner fits.
- For a wire-contract change, update Go request/response types, daemon route
  registrations, Kotlin serialization/client models, `docs/openapi.yaml` and
  `docs/api-contract.md` as applicable. Cover both successful and rejected requests
  in handler/client contract tests. Never silently relax guards for convenience.

## Identity, mutation and persistence

- Reuse project/path policy for containment, symlinks, exclusions and context
  limits; `filepath.Join` alone does not establish that a path is safe.
- Carry project ID/revision, file hash and draft/run identity through asynchronous
  operations. Recheck before publishing results or changing source; a late result
  must not become evidence for a different project, file, draft or run.
- Preserve the guarded Apply/Undo workflow in `app/apply.go`. Apply consumes a
  stored, validated draft identity, not arbitrary request source. Editing a draft
  invalidates its previous checks; Undo requires the expected post-Apply hash.
- Use existing atomic storage primitives and domain stores. Preserve permissions,
  backup/audit behavior and useful persistence errors. A failure after a source
  write must not be reported as if no mutation occurred.
- When changing persisted data or cache meaning, handle existing versions
  deliberately. Invalidate incompatible evidence; keep required historical
  records readable. Change the relevant schema/prompt/policy identity when its
  semantics change, and cover migration or rejection with fixtures.

## Models, jobs and execution

- Use configured `analyze`, `bug` and `function` scopes through existing clients.
  Preserve scope-specific remote confirmation and context filtering. Treat model
  output as untrusted data: validate it before composition or publication.
- Whole-project analysis uses the existing admission and job lifecycle. Keep
  previews free of dispatch and result filters free of scope changes. Resume
  requires fresh admission; retain cumulative attempt accounting and partial
  coverage across interruptions.
- Give every goroutine/process an owner and bounded lifetime. Carry cancellation
  through synchronous request work; background jobs use their existing lifecycle
  owner. Preserve deadlines, request/output limits and inclusive retry budgets.
  Avoid holding locks during provider or subprocess I/O.
- Project-code checks and benchmarks require the existing execution-trust guard
  and isolated workspace. Use controlled argv, bounded diagnostics and process
  cleanup; never execute model text as a shell command. The desktop's user-operated
  terminal does not authorize model-driven commands.
- Keep source, secrets and transcripts out of metadata/logs that are defined as
  source-free. Preserve provenance and freshness instead of presenting advisory
  output as verified evidence.

## Verification

Follow the root Go validation gates. Start with the affected packages; extend
existing test fixtures and fakes rather than copying workflow setup. For changed
guards, include rejection/no-side-effect assertions. For asynchronous changes,
exercise cancellation, replacement and late completion using deterministic
synchronization. For persistence changes, exercise corrupt data and write failure.

Tests must use temporary projects and fake HTTP providers. Live evaluations,
runtime startup and model downloads are separate work requiring an explicit
request; building or testing the evaluation CLI does not require a campaign.
