# Mini-Orca legacy cleanup and pragmatic refactor plan

**Status:** Complete — automated acceptance passed on 2026-09-03. Interactive
Desktop/provider and container acceptance remain release-operator checks because
this environment has no Compose window, model service, personal credentials, or
Docker daemon.

**Prepared:** 2026-09-03

**Scope:** Go daemon, Kotlin/Compose desktop client, configuration, tests, build tooling, and maintained documentation

## 1. Outcome

This plan removes code that is unreachable, obsolete, duplicated, or no longer part of Mini-Orca's product. It then refactors the remaining implementation around the product's actual workflow:

1. select one project;
2. select one file and symbol;
3. request a candidate;
4. inspect source, diff, findings, and checks;
5. explicitly apply or reject the candidate;
6. explicitly undo an applied candidate when allowed.

The goal is not a new framework or a generalized agent platform. The goal is a smaller, easier-to-read application whose package boundaries follow the behavior it currently ships.

### Non-negotiable invariants

- Preserve preview-before-apply. Generation must never write source files.
- Preserve explicit user confirmation for apply and undo.
- Preserve single-project, single-file, single-symbol scope.
- Preserve loopback-only daemon binding by default.
- Preserve revision, source-hash, draft-hash, and task-identity checks that prevent stale writes.
- Preserve read-only source and diff views, keyboard navigation, state labels, and responsive desktop behavior below 1000dp.
- Keep Go production code under `cmd/` and `internal/` and compatible with Go 1.22.
- Do not edit generated output in `build/`, `desktop/build/`, `desktop/.gradle/`, or `desktop/.kotlin/`.
- Do not introduce automatic commits, background source mutation, broad multi-file edits, credentials, or a checked-in local `config.yaml`.
- Replace old implementations in the same change that introduces their replacement. Do not leave compatibility layers or parallel paths behind.

## 2. Cleanup policy

Code is removable when all of the following are true:

- it is unreachable from the daemon or desktop production entry points;
- it is not part of the documented target API or configuration contract;
- its only callers are tests for that obsolete behavior; and
- deletion does not weaken a product invariant above.

Tests do not make obsolete production code live. Tests dedicated to deleted behavior must be deleted with that behavior. Useful assertions should be moved only when they characterize a retained rule.

This plan treats deprecated loopback API routes and legacy configuration fields as removable. Mini-Orca is local-first and has one maintained client. The cleanup will document the breaking contract change and provide a direct configuration migration, but it will not retain silent aliases or fallback parsing. If an undisclosed external API consumer must be supported, that requirement must be identified before Phase 4 begins.

## 3. Audit baseline

The repository was inspected from the daemon entry point, desktop call sites, package tests, route registration, configuration decoding, static analysis, clone detection, and maintained documentation.

### Current validation baseline

| Check | Baseline result |
| --- | --- |
| `make fmt-check` | Pass |
| `go test ./...` | Pass |
| `make test-race` | Pass |
| `go vet ./...` | Pass |
| `./desktop/gradlew -p desktop test` | Pass, 124 tests |
| Go statement coverage | 70.9% overall |
| Staticcheck | Findings remain; see below |
| `go mod tidy -diff` | Non-empty diff |
| Desktop build warnings | Compose/Gradle usage-attribute deprecation warning |

### Size and concentration

| Area | Production lines | Test lines | Main concern |
| --- | ---: | ---: | --- |
| Go | 14,512 | 11,919 | Large retired agent/tooling surface and oversized workflow functions |
| Kotlin | 5,032 | 2,125 | UI composition root owns state, effects, polling, and rendering |
| `internal/orchestrator` | 866 | 876 | Entire package is outside the runtime graph |
| `internal/agent` | 1,613 | 3,468 | Most abstractions serve the retired multi-agent workflow |
| `internal/tools` | 1,681 | 1,881 | Only project-type detection remains reachable |
| Tracked Markdown | 92 files / 7,280 lines | — | Completed task scaffolding obscures current documentation |
| `tasks/` | 76 files / 4,465 lines | — | 71 completed task files retained as working documentation |

The line counts are a baseline, not a target by themselves. Reduction is useful only when behavior and safety remain clear.

### Confirmed static findings

- Dead-code analysis reports 213 unreachable Go functions from production entry points.
- `internal/orchestrator` has no production path from `cmd/mini-orca`.
- Most of `internal/agent` and `internal/tools` is reachable only through obsolete abstractions or their tests.
- Staticcheck reports unreachable or unused helpers and invalid test assertions, including tautological assertions in `internal/agent/integration_test.go` and `internal/project/findings_test.go`.
- `internal/agent/workflow.go` contains a condition that can never be true.
- `internal/llm/client.go` contains a needless field-by-field conversion.
- `go mod tidy -diff` shows that `gopkg.in/yaml.v3` should be a direct dependency and that module sums are stale.
- Clone detection finds production duplication in HTTP request handling, file walking, job state transitions, persistence, candidate validation, and LLM request handling. Some reported import blocks are noise and will be excluded from the enforced threshold.

Dead-code results are evidence, not an automatic deletion list. Reflection, serialization, build-tag, and entry-point uses must still be checked before each removal.

## 4. Principal findings

### 4.1 Retired autonomous-agent architecture

The repository still contains an autonomous orchestration design that the shipped product no longer uses:

- reviewer, tester, registry, workflow, phase, and generic agent abstractions;
- an `internal/orchestrator` package with a write-oriented multi-step workflow;
- prompt registries for those retired roles;
- general-purpose file, shell, formatter, and language executors under `internal/tools`.

This code increases the apparent architecture, duplicates request execution, and exposes write-capable primitives contrary to the deliberately narrow preview-first product. The active workflow only needs scoped model calls and project-type detection.

### 4.2 Candidate compatibility layer after the draft migration

The current draft flow is backed by an older whole-file generation/candidate model. Comments explicitly describe compatibility methods that were meant to survive only until the route migration completed. The result is duplicate vocabulary and validation paths:

- generation preview versus declaration draft;
- candidate checks versus draft checks;
- legacy candidate storage versus current task/draft storage;
- whole-file parsing/validation versus declaration composition;
- deprecated compare/export/check routes with no desktop production caller.

The retained domain should speak in terms of tasks, drafts, checks, apply, and undo. "Candidate" may remain in user-facing copy where it is meaningful, but it should not name a second internal storage model.

### 4.3 Multiple configuration eras

Configuration currently combines:

- a flat `llm` profile;
- scoped model profiles;
- `agents.coder`, `agents.tester`, and `agents.reviewer` settings;
- skill-like prompt configuration; and
- a `skills` example section that is not decoded by the Go configuration type.

Only the scoped generation functions and a subset of coder configuration are live. Silent fallback and ignored fields make configuration look supported when it is not.

### 4.4 API surface exceeds the maintained client

The daemon registers retired aliases and source-free history endpoints. Confirmed examples include:

- `POST /api/chat/message`, which always returns 410;
- `GET /api/chat/history`, an alias;
- candidate compare, export, and compatibility-check routes;
- durable activity history that the desktop never reads;
- read endpoints for transient chat/draft state that have no recovery semantics and no desktop caller.

The loopback API should be the smallest contract needed by the desktop and operational health checks. A route should not remain merely because it has a handler test.

### 4.5 Active Go duplication and oversized control flow

Important live behavior is difficult to review because validation, loading, execution, and persistence occur in the same functions. Current hotspots include context building, semantic response parsing, apply, file analysis, draft checks, finding validation, and declaration composition.

Recurring duplicated behavior includes:

- strict JSON decoding and trailing-token checks;
- project revision validation and domain-error mapping;
- project/file/draft identity comparisons;
- recursive eligible-file walks;
- atomic JSON persistence and corrupt-file recovery;
- analysis and scan job transition handling; and
- LLM request lifecycle handling.

The correct response is a few narrow shared helpers and cohesive state owners, not a middleware framework, repository hierarchy, or generic workflow engine.

### 4.6 Desktop composition and state ownership

`MiniOrcaApp.kt` is approximately 934 lines and currently acts as composition root, effect runner, poller, workflow controller, state coordinator, and renderer. `DesktopShell` takes 53 parameters; other panes take 16–28. `DesktopState.reduce` and several composables have high cyclomatic complexity. There are unused callbacks and theme functions, weakly typed API body construction, and broad exception handling.

The UI needs one clear owner for asynchronous workflow state, smaller feature-level view models/actions, and composables that render state rather than coordinate network effects.

### 4.7 Stale artifacts and build hygiene

- Completed task plans remain in the main documentation tree and are described as the active roadmap.
- API and configuration documentation describe more than one contract generation.
- `.mini-orca/analysis.md` duplicates the canonical JSON analysis report and is not read at runtime.
- `.mini-orca/sessions/activity.json` is produced for an unused activity API.
- The runtime Docker image copies Go source it does not need.
- Module metadata is not tidy.
- The desktop build emits a Gradle compatibility warning that will become an upgrade blocker.

## 5. Target architecture

The target dependency direction is intentionally small:

```text
desktop UI
  -> desktop presenter/controller
    -> typed loopback API client

cmd/mini-orca
  -> HTTP route registration and handlers
    -> application service and feature state owners
      -> project domain / LLM client / atomic storage / configuration
```

The following legacy dependency branches disappear:

```text
internal/orchestrator   DELETE
internal/agent          DELETE after the one live prompt path is moved
internal/tools          DELETE after project detection is moved
internal/workflow       DELETE after retained declaration types are made local
internal/errors         DELETE after transport errors are consolidated
```

### Target responsibilities

| Component | Responsibility | Explicitly not responsible for |
| --- | --- | --- |
| `cmd/mini-orca` | Load validated config, construct concrete dependencies, register routes, manage process lifetime | Business logic or compatibility routing |
| `internal/api` and handlers | HTTP decoding, status mapping, response encoding, route-level preconditions | Persistence, workflow state, or duplicate domain validation |
| `internal/app` | Coordinate import, analysis, chat, draft, checks, apply, and undo | Generic agents or general-purpose tool execution |
| `internal/project` | Safe project paths, source inventory, analysis, findings, symbols, patches, language/project detection | HTTP concerns or model-provider details |
| `internal/llm` | One provider-neutral chat request boundary and its DTOs | Agent roles, prompt registries, or UI scope fallback |
| `internal/storage` | Small atomic file-write and corrupt-file recovery primitives | Generic repositories or domain policy |
| `desktop` presenter/controller | Own asynchronous operations, polling, stale-response rejection, and immutable UI workflow state | Compose layout details |
| Compose screens/components | Render feature models and emit named user intents | Direct network calls or job ownership |

### Deliberate non-goals

- No dependency-injection framework.
- No generic repository layer.
- No event bus or workflow DSL.
- No generalized agent, tool, or plugin runtime.
- No speculative multi-project or multi-file model.
- No compatibility façade for deleted internal packages or private routes.
- No rewrite of working parsers solely to adopt a different style.
- No target based only on maximizing abstraction count or minimizing line count.

## 6. Contract decisions

### 6.1 Configuration

Use one strict configuration schema. Scoped model profiles are the source of truth for `analyze`, `bug`, and `function` work. Remove:

- the flat `llm` fallback;
- `agents.coder`, `agents.tester`, and `agents.reviewer`;
- role timeouts that no live operation consumes;
- prompt "skills" whose only behavior is prepending a generic sentence; and
- undocumented or undecoded `skills` example entries.

Decode YAML with unknown-field rejection so misspelled or retired keys fail at startup instead of silently doing nothing. Keep only shared defaults that genuinely apply to every scoped profile; otherwise make each scope explicit.

The migration guide must map the old fields before they are removed:

| Old source | New source |
| --- | --- |
| `llm.provider`, `llm.base_url`, `llm.api_key`, `llm.model` | Repeat or deliberately specialize under `model_scopes.analyze`, `.bug`, and `.function` |
| `agents.coder.model` | `model_scopes.function.model` |
| `agents.coder.timeout` | The retained function-request timeout, if timeout remains configurable |
| `agents.tester` / `agents.reviewer` | Removed; no live equivalent |
| `skills` and role `skills` | Removed; no live equivalent |

No local `config.yaml` is modified automatically. Update `config.example.yaml`, `CONFIG.md`, Docker examples, startup validation tests, and error messages together.

### 6.2 Loopback API

Keep only routes used by the maintained desktop workflow or process operations. Preserve `/health` for container health checks and `/status` for desktop connectivity.

Delete the following compatibility or unused surface after the route-consumer test is in place:

| Route or capability | Decision |
| --- | --- |
| `POST /api/chat/message` | Delete retired 410 endpoint |
| `GET /api/chat/history` | Delete alias |
| Candidate compare/export/compatibility checks | Delete routes, DTOs, client methods, app methods, and tests |
| Activity history | Delete GET route, record calls, store, DTOs, and `.mini-orca/sessions/activity.json` writer |
| Source-free audit history | Delete route/store if only the apply response consumes audit metadata |
| Unused current-project projection | Delete if the final route inventory confirms no startup/restore consumer |
| Unused transient chat/draft reads and draft review | Delete; apply remains the authoritative server-side safety check |
| Unused analysis-delete route | Delete unless a retained desktop reset action is identified |
| Top-level effective-model compatibility projection | Delete; return only the scoped model catalog |

Update `docs/openapi.yaml`, the canonical human API guide, Kotlin `ApiClient`, handler tests, and contract tests in the same phase. Mark the result as a deliberate local API contract revision in release notes. Do not return 410 shims.

### 6.3 Persistent project metadata

- Keep `.mini-orca/project-analysis.json` as the canonical analysis report.
- Stop generating `.mini-orca/analysis.md` and remove its projection field.
- Remove the unused activity/session store and stop creating `.mini-orca/sessions/activity.json`.
- Preserve findings, file-analysis, scan, apply-audit, and undo metadata that serve the current workflow.
- Do not delete old files from users' projects automatically. Document them as safe manual cleanup after upgrade.
- Preserve atomic replacement, file permissions, and corrupt-file recovery for retained files.

## 7. Execution plan

Each phase must be reviewable on its own. Every phase ends with obsolete code and tests removed; none may introduce a second implementation "for later." Run focused tests while iterating and the listed exit checks before moving on.

### Phase 0 — Freeze the live contract and make deletion measurable

**Purpose:** Establish guardrails before structural removal.

Actions:

1. Record a machine-readable inventory of registered daemon routes and production Kotlin client calls.
2. Add or tighten characterization tests for the retained critical path:
   - project import/restore;
   - file and symbol selection;
   - analysis and context building;
   - draft creation and revision replacement;
   - validation/check execution without source mutation;
   - apply with explicit identity checks;
   - stale apply rejection;
   - undo eligibility and undo;
   - cancellation and stale-response rejection; and
   - loopback/default bind behavior.
3. Add a source-mutation assertion to generation, analysis, chat, validation, and check tests.
4. Fix invalid live test assertions. Delete invalid assertions belonging solely to code scheduled for deletion rather than repairing obsolete tests.
5. Add a route-consumer test: every maintained route must have a named desktop or operational consumer, and every production client call must resolve to a registered route.
6. Capture the baseline metrics from Section 3 in the implementation change log.

Exit criteria:

- Critical preview/apply/undo behavior has direct tests independent of retired agent abstractions.
- The target route set is explicit and contract-tested.
- All baseline validation still passes.

### Phase 1 — Remove the autonomous agent and general-purpose tools stack

**Purpose:** Delete the largest confirmed legacy subsystem first.

Actions:

1. Move the live declaration-edit prompt construction into a focused file under `internal/app`; return ordinary `[]llm.ChatMessage` values.
2. Make the function-scope runtime call `llm.Client.Chat` directly and return the response content through one small helper.
3. Remove generic `Agent`, `Result`, `Phase`, registry, reviewer, tester, coder, workflow, and prompt-registry types.
4. Move the small live project/build-file detection behavior into `internal/project`, with table-driven tests for supported project types.
5. Delete all shell, write, formatter, executor, and language-tool implementations that are not reachable from the retained workflow.
6. Delete `internal/orchestrator`, `internal/agent`, `internal/agent/prompts`, and `internal/tools` after imports reach zero.
7. Delete their dedicated tests and fixtures. Move only retained project-detection and prompt assertions to the new owners.
8. Re-run dead-code analysis and inspect every remaining result.

Exit criteria:

- No production or test import references `internal/orchestrator`, `internal/agent`, or `internal/tools`.
- Scoped model calls still produce the same request content and error behavior.
- The daemon contains no general-purpose source-writing executor.
- `go test ./...`, `make test-race`, and staticcheck pass for affected packages.

### Phase 2 — Establish one strict configuration and LLM boundary

**Purpose:** Remove fallback eras and duplicated model execution.

Actions:

1. Implement the schema decision in Section 6.1 and reject unknown YAML fields.
2. Validate required scoped profile fields once during startup and return actionable field paths in errors.
3. Remove unused configuration loaders and lookup helpers, including JSON loading and fallback scope lookup.
4. Keep one production `llm.Client` constructor. Remove host-style constructors and options that only support deleted agent code.
5. Remove the unused model-list endpoint/client DTOs if the route inventory confirms no consumer.
6. Centralize request creation, non-success decoding, timeout/cancellation handling, and response-content validation inside the LLM boundary.
7. Apply the staticcheck simplification in `internal/llm/client.go`.
8. Update examples and migration documentation in the same change.
9. Run `go mod tidy` and require a clean `go mod tidy -diff`.

Exit criteria:

- Exactly one configuration schema and one model-call path exist.
- Unknown and retired configuration keys fail clearly.
- `config.example.yaml` contains only decoded and used fields.
- LLM tests cover success, provider errors, malformed bodies, missing content, timeout, and cancellation.
- Module metadata is tidy.

### Phase 3 — Make the draft domain native

**Purpose:** Remove the legacy whole-file generation model beneath the current workflow.

Actions:

1. Define focused internal values for:
   - project snapshot identity;
   - selected file identity;
   - task identity;
   - draft revision/hash identity;
   - composed declaration edit; and
   - validation/check result.
2. Use these values to replace repeated ID, revision, path, source-hash, and draft-hash conditionals. Keep wire JSON stable until the API phase; map at the handler boundary.
3. Refactor apply into named stages: decode/precondition, snapshot verification, draft revalidation, atomic write, audit persistence, and response construction.
4. Refactor draft checks into named stages: identity validation, command selection, execution, normalization, and result storage.
5. Make the check runner private to the draft workflow; remove the exported candidate wrapper.
6. Replace the legacy `GenerationPreview` backing store with a draft-native stored record.
7. Move the few still-live hash/ID helpers into a focused identity file.
8. Delete whole-file generation parsing, candidate compatibility storage, template setters, legacy comparison/export logic, and their tests.
9. Move any retained declaration-validation types out of `internal/workflow`; then delete that package.
10. Remove source-free draft audit state if no retained route consumes it. Keep apply audit required by the apply/undo result.

Exit criteria:

- There is one draft representation from generation through apply.
- Apply performs all stale-identity and source-integrity checks immediately before writing.
- Generation, validation, and checks cannot mutate source.
- `internal/app/generation.go`, legacy comparison code, candidate bridges, and `internal/workflow` are gone.
- Focused tests cover hash/revision mismatch, changed source, replaced draft, failed checks, successful apply, and undo.

### Phase 4 — Reduce the API to the maintained contract

**Purpose:** Remove deprecated and unconsumed transport surface.

Actions:

1. Delete the routes and capabilities approved in Section 6.2.
2. Delete matching handler methods, request/response DTOs, app methods, Kotlin client methods, tests, and OpenAPI operations in the same change.
3. Remove activity recording from retained handlers and delete the activity/session store.
4. Remove compatibility fields from the scoped model response.
5. Use Go 1.22 method-aware `ServeMux` patterns consistently and remove redundant method switches from handlers.
6. Add three package-local transport helpers only:
   - strict one-object JSON decoding with size limits and trailing-token rejection;
   - required project revision parsing; and
   - consistent domain-error-to-status mapping.
7. Consolidate the transport-only API error type under `internal/api`; delete the thin `internal/errors` package if no domain caller remains.
8. Keep route registration explicit and grouped. Do not add a routing framework.
9. Update the route-consumer contract test and API documentation.

Exit criteria:

- Every registered route has a maintained consumer or an explicit health/operations purpose.
- No production Kotlin client method is uncalled.
- No deprecated route, alias, compatibility field, activity store, or 410 shim remains.
- Malformed JSON, stale revision, missing project, cancellation, and domain conflicts return consistent statuses and bodies.

### Phase 5 — Consolidate project traversal and persistence

**Purpose:** Remove duplication in active project code without hiding domain rules.

Actions:

1. Build one deterministic eligible-file walker used by project import, inventory, analysis, and context discovery where policies are identical.
2. Keep policy inputs explicit: root, ignored directories, supported extensions, maximum file count, symlink policy, and cancellation.
3. Do not introduce a cache unless profiling proves repeated walking is a material bottleneck.
4. Introduce a small `internal/storage` atomic-write primitive for retained JSON/text metadata:
   - create in the destination directory;
   - write and sync;
   - apply the intended permissions;
   - close;
   - rename atomically; and
   - clean up the temporary file on error.
5. Add one corrupt-file recovery helper used only where current behavior already recovers corrupt metadata. Do not create a generic repository layer.
6. Migrate findings, file-analysis, project-report, scan-state, and apply-audit persistence where their semantics match.
7. Keep source apply/undo replacement in its domain-specific path because its validation and recovery semantics differ from metadata writes.
8. Stop generating `.mini-orca/analysis.md`; retain canonical JSON only.
9. Delete dead project methods such as write-path resolvers, old analyzer constructors, unused finding reconciliation, duplicate context listing, and unused activity/report helpers.
10. Replace five-or-more related analyzer constructor arguments with one small provenance/profile value when it makes the call site clearer.
11. Break context building and semantic parsing into validation, selection, parsing, and normalization functions with single responsibilities.

Exit criteria:

- One eligible-file traversal policy exists for equivalent scans.
- One atomic metadata-write implementation exists.
- No retained code reads or writes the obsolete Markdown analysis or activity file.
- Path traversal, symlink escape, file limits, corrupt metadata, cancellation, and partial-write behavior remain directly tested.
- Race tests pass.

### Phase 6 — Give Go workflow state clear owners

**Purpose:** Reduce the application service's mutex/state sprawl while staying concrete and simple.

Actions:

1. Keep `app.Service` as the orchestration façade used by handlers.
2. Move mutable state into cohesive concrete owners, for example:
   - scoped model catalog/runtime;
   - analysis jobs;
   - Go scan job;
   - draft store; and
   - chat session store.
3. Give each owner its own mutex and invariants. Do not expose locks or duplicate defensive clones at the handler layer.
4. Never hold a lock during model requests, command execution, filesystem walks, or HTTP response writing.
5. Share job-state transition helpers only where analysis and scan semantics are truly identical; retain separate job types otherwise.
6. Keep clone functions where they protect concurrent ownership. Delete only unused clones and exact duplicate implementations; do not replace them with reflection or generic deep-copy machinery.
7. Decompose the remaining high-complexity functions into named stages. Prefer early validation and straight-line success paths.
8. Remove dead logging, error, project, config, and app helpers surfaced after the state move.

Exit criteria:

- Every mutable map/job/session has one named owner.
- Lock boundaries are visible and covered by race tests.
- No function mixes transport decoding, model execution, state mutation, and persistence.
- Staticcheck and dead-code analysis report no unexplained production findings.

### Phase 7 — Refactor the desktop workflow boundary

**Purpose:** Move asynchronous behavior out of the Compose root and make stale work easy to reason about.

Actions:

1. Reduce `MiniOrcaApp` to dependency construction, lifecycle hookup, top-level state observation, and root composition.
2. Introduce one concrete desktop presenter/controller that owns:
   - `ApiClient`;
   - coroutine jobs and polling;
   - selected project/file/symbol identities;
   - task and draft identities;
   - stale-response rejection;
   - operation errors; and
   - immutable workflow UI state.
3. Keep temporary presentation state—drawers, filters, focus, palette visibility—near the relevant composable when it has no workflow meaning.
4. Replace the monolithic reducer with feature-sized pure transitions or clear named presenter methods. Do not introduce Redux/MVI infrastructure solely for terminology.
5. Use explicit value types for project, file, task, and draft identities so equality checks cannot accidentally omit a revision or hash.
6. Cancel superseded work and ignore late responses using request identity, not only boolean loading flags.
7. Treat coroutine cancellation separately from user-visible failures. Map API protocol failures and connectivity failures deliberately; remove broad exception catches where a narrower type is available.
8. Add presenter tests for rapid project/file/symbol changes, cancellation, stale poll responses, daemon disconnect/reconnect, apply, and undo.

Exit criteria:

- No composable owns a daemon polling loop or performs direct API orchestration.
- `MiniOrcaApp` is a small composition root.
- Workflow transitions are unit-testable without Compose rendering.
- Stale results cannot overwrite a newer project, file, symbol, task, or draft.

### Phase 8 — Simplify Compose screens and the Kotlin API client

**Purpose:** Remove parameter explosion, unsafe request construction, and UI duplication.

Actions:

1. Split `DesktopShell` and large panes by stable visual responsibility, not one function per trivial element.
2. Replace long primitive/callback parameter lists with small feature-level immutable models and named action groups. Avoid one global "all callbacks" object.
3. Target no more than roughly 6–8 parameters per composable; justify exceptions where Compose slot APIs are clearer.
4. Remove unused `onCancelAll`, unused theme helpers, unreachable UI states, and matching tests.
5. Replace `jsonBody(vararg Any?)`, unchecked lists, and ad hoc maps with typed request DTOs and centralized serialization.
6. Keep one concrete `ApiClient` unless a split clearly reduces coupling. Organize methods by feature rather than introducing interfaces with one implementation.
7. Centralize repeated error/status decoding and preserve server error details.
8. Preserve semantic labels, keyboard traversal, read-only source/diff behavior, and the sub-1000dp drawer layout.
9. Add or retain UI tests for enabled/disabled actions, labels, keyboard focus order, and compact layout.

Exit criteria:

- Large composables render feature state and emit intents; they do not assemble domain identities.
- No production client method or callback is unused.
- No unchecked request-body construction remains.
- Desktop tests pass with the same accessibility and responsive behavior.

### Phase 9 — Remove stale documentation and build residue

**Purpose:** Make the repository describe only the maintained system.

Actions:

1. Replace the pre-cleanup completed task-file archive with durable architecture, configuration, API, and contributor documentation. Git history remains the archive.
2. Delete obsolete pre-cleanup task plans and execution prompts after any still-current decisions are captured in maintained docs. Retain Tasks 103–117 as the auditable execution record for this cleanup backlog.
3. Remove outdated UI mock artifacts that no longer serve an active design workflow.
4. Choose one human API guide plus `docs/openapi.yaml`; turn duplicate top-level API docs into a short pointer or delete them.
5. Update README, configuration, Docker, API, desktop, and release-acceptance docs to match the final route and configuration contracts.
6. Remove source copying from the runtime Docker stage.
7. Validate the current Compose/Kotlin/Gradle compatibility matrix, then align wrapper/plugins to eliminate the usage-attribute deprecation warning. Do this as a focused build change, not mixed into UI behavior.
8. Add standard ignores for local/editor artifacts only when they are not already covered. Never delete untracked user files as part of the refactor.
9. Check all maintained relative documentation links.

Exit criteria:

- No completed task roadmap is presented as current work.
- Documentation contains one configuration contract and one API contract.
- The runtime image contains the binary and runtime assets only.
- Desktop tests run without the identified Gradle compatibility warning.

### Phase 10 — Enforce quality and complete the cleanup

**Purpose:** Prove that the smaller architecture is safer and maintainable.

Actions:

1. Add a reproducible, version-pinned quality command for Go staticcheck, dead-code analysis, and production clone detection. Keep tooling out of the runtime dependency graph.
2. Add a focused Detekt configuration for correctness, unused code, unsafe casts, exception handling, complexity, and Compose-relevant structure. Add formatting enforcement with a Kotlin/Gradle-compatible formatter.
3. Exclude generated content and import-only clone noise; do not suppress real production findings with a broad baseline file.
4. Record final line counts, package graph, duplication, complexity hotspots, coverage, binary/image size, and desktop test count.
5. Perform a final manual smoke test using a disposable fixture project.
6. Run the complete validation matrix below.

Exit criteria:

- No unexplained dead production functions.
- Staticcheck, vet, formatting, and Detekt have zero production findings.
- Production duplication is below 1%, excluding imports/generated code, with no known duplicated domain rule.
- No non-test function exceeds cyclomatic complexity 15 without a written, local justification.
- No production source file exceeds 500 lines without a written, local justification.
- Remaining live-package test coverage does not regress from the recorded baseline; critical apply/undo and stale-write paths have direct coverage.
- No legacy package, route, configuration key, persistence writer, compatibility DTO, or obsolete document identified in this plan remains.

## 8. File-level removal ledger

This ledger is intentionally explicit. Exact test filenames may change as useful characterization assertions are moved.

### Confirmed removal after dependencies are moved

- `internal/orchestrator/**`
- `internal/agent/reviewer.go`
- `internal/agent/tester.go`
- `internal/agent/registry.go`
- `internal/agent/workflow.go`
- `internal/agent/prompts/**`
- remaining generic files in `internal/agent/**` after direct scoped model execution replaces them
- `internal/tools/**` after project detection moves to `internal/project`
- `internal/workflow/**` after retained declaration edit types move to their owning package
- old whole-file generation parsing in `internal/app/generation.go`
- legacy candidate comparison/export implementation
- candidate compatibility methods marked for post-route-migration removal
- the unused activity/session persistence implementation
- `internal/errors/**` after the transport error type moves into `internal/api`
- tests and fixtures whose sole subject is any deleted behavior

### Remove only after the named proof

| Item | Required proof |
| --- | --- |
| Unused current-project/read routes | Route-consumer inventory confirms no desktop or operational use |
| Draft/chat read routes | No restart/resume behavior depends on in-memory state retrieval |
| Audit-history route | Apply/undo response and retained audit persistence meet all UI and recovery needs |
| Model-list support | No desktop selection or diagnostics feature consumes provider enumeration |
| Analysis-delete route | No retained reset/reanalysis workflow calls it |
| UI mock assets | Maintained screenshots/design docs no longer link to them |

"Remove only after proof" does not mean keep indefinitely. Phase 0 supplies the proof; the owning phase then removes the item or records the concrete retained consumer in this plan.

## 9. Refactoring rules

Use these rules during every phase:

1. **Name by product behavior.** Prefer `DraftStore`, `ProjectSnapshot`, or `AnalysisJob` over `Manager`, `Context`, or `Processor` when the narrower name is accurate.
2. **One reason to change.** A function should validate, load, execute, persist, or render—not all five.
3. **Straight-line control flow.** Validate early, return useful errors, and keep the successful path visually obvious.
4. **Share rules, not incidental syntax.** Extract duplicated identity checks, atomic writes, and file policies. Do not generalize two unrelated jobs because both have a status string.
5. **Concrete before abstract.** Introduce an interface only when there are multiple real implementations or a boundary must be substituted in a test. Prefer a small function seam otherwise.
6. **Keep invariants near state.** The type that owns a mutable draft/job/session owns its validation and cloning.
7. **Keep wire types at the edge.** HTTP and JSON DTOs map to domain values in handlers. Do not pass unvalidated strings and maps through the application.
8. **No boolean soup.** Use explicit state or intent types where combinations can be invalid.
9. **No silent compatibility.** Unknown config keys and invalid enum values fail with actionable errors.
10. **No speculative caching.** Measure first; deterministic recomputation is preferable while project scope remains small.
11. **Comments explain why.** Delete comments that narrate syntax or refer to completed migration tasks.
12. **Tests follow behavior.** Prefer table tests for policy and focused integration tests for workflow; avoid tests that merely mirror implementation structure.

## 10. Validation strategy

### Required after every Go behavior phase

```sh
make fmt-check
go test ./...
go vet ./...
```

Run focused package tests during iteration. Run `make test-race` after any mutation, cancellation, job, store, or lock change.

### Required after every desktop behavior phase

```sh
./desktop/gradlew -p desktop test
```

Also run the configured Kotlin formatting and Detekt tasks after Phase 10 introduces them.

### Required before final handoff

```sh
make check
make test-race
go mod tidy -diff
git diff --check
```

Also run the pinned Go quality command and desktop quality tasks. If an environment-dependent check cannot run, record the exact command, reason, and remaining risk; do not report the phase complete silently.

### Manual smoke checklist

Use a disposable fixture repository and verify:

1. start with default loopback binding;
2. import and restore a project;
3. browse files and symbols with keyboard navigation;
4. run analysis and inspect findings;
5. start, pause/resume where supported, cancel, and finish long-running jobs;
6. create a chat task and request a declaration draft;
7. replace a draft and confirm an older response cannot overwrite it;
8. inspect read-only source and diff;
9. run validation/checks and confirm source hash is unchanged;
10. attempt apply after externally changing the source and confirm rejection;
11. apply an unchanged valid draft explicitly;
12. undo it explicitly;
13. resize below and above 1000dp and verify drawer behavior;
14. restart/disconnect the daemon and verify desktop recovery/error state;
15. confirm no activity file or Markdown analysis report is newly created.

## 11. Risk controls

| Risk | Control |
| --- | --- |
| Dead-code tool misses dynamic use | Confirm entry points, route inventory, serialization, build tags, and desktop call sites before deletion |
| Deleting tests hides a retained rule | Add critical-path characterization first; move behavior assertions before removing obsolete suites |
| API cleanup breaks an unknown client | Treat the API revision as deliberate, document removed operations, and stop before Phase 4 if an external consumer is declared |
| Strict config breaks local setups | Provide an exact field migration and actionable unknown-key errors; do not rewrite user config automatically |
| Apply refactor weakens stale-write safety | Make identity and source validation explicit values and test every mismatch immediately before atomic write |
| State-owner split introduces races | Keep one mutex per owner, avoid locks across I/O, run race tests, and test cancellation/late responses |
| UI extraction changes behavior | Separate state/effects before layout, retain UI tests, and complete the responsive/keyboard smoke checklist |
| Shared helpers become a new framework | Limit helpers to demonstrated repeated semantics; reject generic repositories, middleware stacks, and workflow DSLs |
| Historical files are useful to a user | Delete only tracked obsolete project artifacts; leave local/untracked files untouched and rely on Git history |

## 12. Delivery order and review boundaries

The implementation backlog maps the phases to these review and commit boundaries:

| Task | Review boundary | Required commit subject |
| ---: | --- | --- |
| 103 | Contract inventory and characterization baseline | `test: characterize cleanup contract` |
| 104 | Autonomous orchestrator deletion | `refactor: remove autonomous orchestration` |
| 105 | Direct scoped execution and agent deletion | `refactor(app): remove generic agent layer` |
| 106 | Project detection migration and tools deletion | `refactor(project): remove general tool executors` |
| 107 | Strict scoped configuration and one LLM boundary | `refactor(config): enforce scoped model profiles` |
| 108 | Draft identities and Apply/check decomposition | `refactor(app): make draft apply identities explicit` |
| 109 | Whole-file candidate/workflow compatibility deletion | `refactor(app): remove candidate compatibility` |
| 110 | Loopback API and activity cleanup | `refactor(api): remove legacy loopback routes` |
| 111 | Eligible-source traversal consolidation | `refactor(project): unify source traversal` |
| 112 | Atomic metadata persistence consolidation | `refactor(storage): unify metadata persistence` |
| 113 | Go workflow state ownership and residual cleanup | `refactor(app): isolate workflow state` |
| 114 | Desktop presenter and asynchronous state ownership | `refactor(desktop): extract workflow presenter` |
| 115 | Compose and typed API-client simplification | `refactor(desktop): simplify UI and API contracts` |
| 116 | Documentation, historical artifacts, Docker, and build cleanup | `chore: remove legacy docs and build residue` |
| 117 | Quality gates, final metrics, and acceptance | `chore: complete legacy cleanup acceptance` |

Execute Tasks 103–117 strictly in order with
[`tasks/PROMPT_EXECUTE_LEGACY_CLEANUP.md`](tasks/PROMPT_EXECUTE_LEGACY_CLEANUP.md).
Each task must leave the repository buildable, tested, and free of the obsolete
implementation it replaced. The user has explicitly authorized exactly one isolated
commit per completed task; do not combine, amend, squash, tag, or push those commits.

## 13. Definition of done

## 14. Final execution evidence

Task 117 completed the cleanup sequence on 2026-09-03. The historical baseline
above remains intentionally as the pre-cleanup audit; the maintained contract is
the README, configuration guide, API guide/OpenAPI pair, and release acceptance
record.

| Metric | Task 103 baseline | Final result |
| --- | ---: | ---: |
| Go production/test lines | 14,512 / 11,919 | 9,483 / 5,425 |
| Desktop Kotlin production/test lines | 5,032 / 2,125 | 8,109 / 3,838 |
| Runtime Go packages | retired branches present | 10 (`cmd/daemon` plus nine retained internal packages) |
| Registered routes | 45 | 32, each catalogued with a maintained consumer or operational purpose |
| Go statement coverage | 70.9% | 75.6% |
| Desktop tests | 124 | 135 |
| Production dead-code findings | 213 pre-cleanup; 8 residual before Task 117 | 0 |
| Production clone lines | 0.37% residual before Task 117 | 0.00% |
| Go cyclomatic hotspot | 25 | all production functions at or below 15 |

`make quality` pins Staticcheck 0.7.0, `deadcode` from x/tools 0.40.0,
gocyclo 0.6.0, and jscpd 4.0.5 outside the runtime graph, then runs checked-in
Spotless and Detekt tasks. It passed with no production finding. Detekt uses a
65-branch ceiling for top-level Compose screen routers; their child composables
and the presenter remain independently tested rather than hidden by a baseline
or suppression.

The Task 117 review also recorded the production files over 500 lines:

- `internal/app/analyze_all.go` owns the bounded, pauseable analysis-job state
  and its persistence; `internal/project/go_declaration_edit.go` owns the
  parser-backed declaration composition safety boundary.
- `DesktopShell.kt`, `WorkspacePanes.kt`, `ReviewEvidencePane.kt`, and
  `DesktopApp.kt` are cohesive Compose layout/rendering boundaries;
  `DesktopState.kt` is the typed reducer boundary; and
  `DesktopWorkflowPresenter.kt` owns asynchronous request, cancellation, and
  stale-response coordination. They are intentionally separate from one
  another and verified by the Desktop suite.

The full supported automated matrix passed: formatting, unit tests, race tests,
vet, module-tidiness check, quality gates, Desktop tests, `make check`, route
and documentation contract tests, and `git diff --check`. A local loopback
daemon started with `config.example.yaml`; `GET /health` and `GET /status`
passed before it was shut down. The disposable-fixture provider/UI flow and the
container image size/health check are not recorded as passed: this environment
has no running local model, personal provider credentials, interactive Compose
Desktop session, Docker daemon, or Compose plugin. A release operator must run
those documented manual checks before release.

The cleanup is complete only when all of the following are true:

- The runtime package graph matches Section 5.
- Retired agent, orchestrator, tool-executor, workflow, candidate-compatibility, activity, and error-wrapper packages/paths are deleted.
- Every registered API route has a maintained consumer or an explicit operational purpose.
- There is one strict configuration schema and one scoped model execution path.
- There is one draft representation and one apply safety path.
- Equivalent file walks and metadata writes have one implementation each.
- Mutable jobs, drafts, and sessions have named owners with race-tested lock boundaries.
- Compose screens render models and emit intents; the app root does not coordinate network workflows.
- Static analysis has no unexplained findings, production duplication is below the agreed threshold, and the full validation suite passes.
- Current documentation describes the resulting system and includes configuration/API migration notes.
- No generated directories, credentials, user source files, local config, or untracked user artifacts were modified.

The expected result is materially less code, fewer concepts, a narrower API and configuration surface, and stronger safety around the small set of behaviors Mini-Orca actually supports.
