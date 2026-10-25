# Full ADK integration and Go migration

Migrate Mini-Orca to the latest stable Go toolchain, then make Google ADK the execution framework for its coding, analysis, documentation, and integration workflows. Preserve local ownership of source, consent, verification, and recovery while presenting the expanded capabilities through a restrained, professional Go TUI.

This is an implementation specification, not a claim that the features below already exist. Execute tasks in order, with one verified commit per task; the user explicitly requested both the full implementation and a commit after every step.

## Decisions and execution contract

- Versions verified on 2026-10-24: [Go 1.27.3](https://go.dev/dl/) is the latest stable Go release; [ADK Go v2.6.0](https://github.com/google/adk-go/releases/tag/v2.6.0) is the latest stable v2 release. Its [module](https://github.com/google/adk-go/blob/v2.6.0/go.mod) requires Go 1.26.6. Pin reviewed versions, including CI and container builds; do not use a moving `latest` dependency.
- Use native Go ADK. `internal/app` retains workflow admission, current project/proposal/source identities, consent, execution trust, and Apply/Undo. A new leaf package, `internal/app/agentruntime`, owns ADK-specific agents, adapters, graph execution, tools, plugins, and session interfaces. Inject host operations into it; it must never import its parent package. `internal/app/agentstate` owns durable runtime records through existing `internal/storage` primitives. Keep SDK types out of HTTP DTOs, TUI state, and `internal/project`.
- Integrate all relevant available Go capabilities: typed model/tool interactions, streaming, sequential/parallel/loop/conditional workflows, human input, durable sessions/checkpoints, artifacts, memory, compaction, skills, MCP, A2A, tracing, evaluation, and optional remote services. OpenAPI adapters and task evaluation are Mini-Orca implementations where the Go SDK does not provide the required facility. A capability is supported only after its pinned SDK/provider path has a passing conformance test.
- Keep the existing `analyze`, `bug`, and `function` profiles valid. Add named role assignments and explicit capability metadata without silently selecting another provider. Retain HTTP, `agy`, and `pi` compatibility; CLI providers remain completion-only unless a separately validated protocol supports tools. No implicit local-to-remote fallback.
- Treat source, tool output, skill text, remote agent cards, and memory as untrusted inputs. Domain checks execute at the actual model/tool/storage boundary, even when an SDK callback is skipped. ADK confirmation is an input mechanism; it never substitutes for Mini-Orca's reviewed proposal authority.
- Preserve the eight-file write limit and explicit read-scope admission. Larger missions consist of separately approved, checked, reviewed tasks. No unrestricted shell, automatic merge, source writes from navigation, or replay of an uncertain mutation after a crash.
- Default to local storage and local operation. MCP processes, external endpoints, grounding, cloud storage, live media, and ambient execution require explicit configuration and the appropriate per-scope grant. Implement and test their boundaries with fakes; configuration is not permission to send repository source. Missing credentials leave an honest unavailable state, not fabricated results.
- Runtime state and artifact content are private source-bearing data with restrictive permissions; ordinary logs, traces, and evaluation receipts are source-free. Never store provider secrets in ADK session state. Retention, deletion, and export must have explicit behavior.
- Preserve five workspaces: Project, Work, Code, Docs, Settings. Main lists show task, state, progress, and next action. Detailed execution, evidence, tools, usage, and recovery appear in scrollable detail views. Use stable stage IDs, restrained borders, consistent spacing, text status labels, keyboard focus, and the existing four palettes. Avoid turning the home screen into an SDK console.
- Before each task, inspect status/diffs and local instructions. Change only the listed files and the task checkbox; necessary corrections to the plan are explicit planning edits. New files are marked **new**. Do not add unused scaffolding: every introduced production capability needs a real caller in the same commit.
- For every Go task, format with `go fmt ./...`, run its focused command and `./scripts/validate.sh`, and review `git diff --check`. The maintained validation script includes formatting, full Go tests, race tests, vet, quality, Python tooling tests, and TUI tests. Run shell syntax checks when shell files change. Do not weaken a gate or overwrite historical fixtures to obtain a pass.
- Mark a task complete only after its acceptance criteria and verification pass. Stage explicit paths, commit with the suggested message, and verify a clean working tree before proceeding. The user's commit requirement overrides the builder skill's default prohibition on commits. Never commit generated binaries, local configuration, credentials, runtime stores, or downloaded SDK sources.

## Task 01 — Migrate the supported toolchain to Go 1.27.3

- [x] Complete

**Target files**
- `go.mod`, `go.sum` — supported Go version and necessary dependency metadata.
- `.mise.toml`, `.github/workflows/validate.yml`, `Dockerfile` — aligned development, CI, and builder toolchains.
- `scripts/quality.sh`, `scripts/tests/test_quality.py` — update a pinned checker only if compatibility requires it.
- `AGENTS.md`, `internal/AGENTS.md`, `README.md` — supported runtime instructions.

**Inputs / dependencies**
- None. Current module/CI target Go 1.22; the inspected local executable is Go 1.27.1. Use Go's managed toolchain download rather than replacing the system installation.

**Implementation rules**
- Set the module minimum to `go 1.27.3`; pin mise and the container builder to that patch. Let CI read the module's Go version. Verify the builder image exists before naming it.
- Review [Go 1.27 changes](https://go.dev/doc/go1.27), especially timers, HTTP, filesystem/process behavior, and tooling compatibility. Make no unrelated dependency or source modernization changes.
- Keep deliberately old fixture module versions. Preserve CGO-free daemon/TUI release builds and the existing quality thresholds.

**Verification command**
`go version && go mod verify && ./scripts/validate.sh && make build && make build-linux && git diff --check`

**Commit**: `build: migrate supported toolchain to Go 1.27.3`

## Task 02 — Put existing model execution behind a native ADK boundary

- [x] Complete

**Target files**
- `go.mod`, `go.sum` — pin ADK v2.6.0.
- `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/model.go`, `internal/app/agentruntime/runtime_test.go` — **new**, typed runtime and completion adapter.
- `internal/app/service.go`, `internal/app/service_test.go`, `internal/app/model_runtime.go` — wire existing model calls through the adapter and retain usage/finish metadata.
- `internal/app/archgate/archgate.go`, `internal/app/archgate/archgate_test.go`, `internal/AGENTS.md` — enforce the new acyclic package boundary.

**Inputs / dependencies**
- Task 01; existing `llm.Client`, strict response schemas, retry admission, and workflow tests.

**Implementation rules**
- Run existing completion behavior through an ADK runner with a narrow Mini-Orca-owned request/result interface. SDK conversion belongs entirely in the leaf package.
- Preserve complete conversations, schema enforcement, CLI isolation, refusals, empty-result policy, deadlines, and before-attempt identity checks. SDK adapters must not introduce an additional unaccounted retry loop.
- Add conformance tests for the pinned runner, model adapter, event termination, cancellation, and errors. The current application remains functional at this commit.

**Verification command**
`go test ./internal/app/agentruntime ./internal/app ./internal/llm && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: run model operations through the ADK adapter`

## Task 03 — Support typed tool calls and bounded text streaming

- [x] Complete

**Target files**
- `internal/llm/client.go`, `internal/llm/client_test.go` — preserve completion compatibility.
- `internal/llm/generation.go`, `internal/llm/generation_test.go`, `internal/llm/stream.go`, `internal/llm/stream_test.go` — **new**, provider-neutral content, tool calls/results, and bounded SSE decoding.
- `internal/app/agentruntime/model.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/runtime_test.go` — translate native ADK requests/results and consume streaming events.

**Inputs / dependencies**
- Task 02; existing HTTP size, timeout, redirect, and credential policies.

**Implementation rules**
- Support declared function schemas, stable call IDs, validated JSON arguments, tool responses, terminal status, usage, and incremental text. Preserve tool call/result ordering and parallel call identity.
- Reject unknown calls, malformed or oversized arguments, truncated streams, missing terminal events, and unsupported provider parameters. Cancellation closes the response body and worker lifetime.
- A partial stream is never a completed proposal; only validated final content may enter publication. CLI adapters expose completion-only capabilities and never enable their own filesystem or shell tools.

**Verification command**
`go test -race ./internal/llm ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add typed model tools and streaming transport`

## Task 04 — Add explicit provider capabilities and native Gemini access

- [x] Complete

**Target files**
- `internal/config/config.go`, `internal/config/model_profiles.go`, `internal/config/cli_profiles.go`, `internal/config/config_test.go`, `internal/config/config_example_test.go`.
- `internal/app/agentruntime/gemini.go`, `internal/app/agentruntime/gemini_test.go`, `internal/llm/capabilities.go`, `internal/llm/capabilities_test.go` — **new**.
- `internal/app/agentruntime/provider.go`, `internal/app/agentruntime/gemini_transport.go` — **new**, immutable provider adapter and a bounded endpoint guard; `internal/app/agentruntime/model.go` selects native execution.
- `internal/app/model_catalog.go`, `internal/app/analysis_models.go`, `internal/app/model_choices.go`, `internal/app/service.go`.
- `internal/app/service_test.go` — native-provider admission and import-analysis compatibility.
- `internal/api/handlers/model_handler_test.go`, `internal/tui/client/models.go`, `CONFIG.md`, `config.example.yaml`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 03; [Go provider documentation](https://adk.dev/agents/models/openai/) and the pinned Gemini adapter.

**Implementation rules**
- Add explicit Gemini Developer API and Vertex backend configuration, endpoint-bound credentials, capability reporting, and validation. Keep existing three-profile YAML valid; unknown keys remain errors.
- Expose tool, schema, streaming, attachment, grounding, and live-session support only when verified. Model discovery is explicit and bounded; viewing cached metadata sends nothing.
- Inject guarded HTTP transport into SDK clients; prevent environment credential fallback to a different endpoint, redirect leakage, and SDK-owned retries outside the app budget. Keep local OpenAI-compatible parameter support intact.
- Use a provider adapter with the existing completion and optional-empty contracts, including import analysis. Preserve native thought signatures inside the SDK boundary. Support explicit Developer API keys, Vertex Express keys, or project/location-bound Vertex access tokens; never discover credentials implicitly.

**Verification command**
`go test ./internal/config ./internal/llm ./internal/app/agentruntime ./internal/app ./internal/api/handlers ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add capability-aware Gemini and compatible providers`

## Task 05 — Enforce shared run budgets, routing, and policy

- [x] Complete

**Target files**
- `internal/config/agent_runtime.go`, `internal/config/agent_runtime_test.go` — **new**, validated limits and role assignments.
- `internal/config/config.go`, `internal/config/default.go`, `internal/config/config_test.go`, `CONFIG.md`, `config.example.yaml`.
- `internal/app/agentruntime/policy.go`, `internal/app/agentruntime/budget.go`, `internal/app/agentruntime/policy_test.go` — **new**.
- `internal/app/agentruntime/runtime.go`, `internal/app/service.go`, `internal/app/analysis_models.go`, `internal/app/change_pipeline_timeout.go`, `internal/app/change_pipeline_timeout_test.go`.
- `internal/app/agentruntime/provider.go`, `internal/app/agentruntime/model.go`, `internal/app/agentruntime/gemini.go`, `internal/app/agentruntime/runtime_test.go`, `internal/app/agentruntime/gemini_test.go` — enforce policy at actual provider dispatch and preserve known versus estimated usage.
- `internal/app/agent_limits.go`, `internal/app/agent_limits_test.go` — **new**, application-owned admission for shared run limits.
- `internal/app/change_pipeline.go`, `internal/app/change_pipeline_types.go`, `internal/app/change_pipeline_test.go`, `internal/app/analysis_run.go`, `internal/app/analysis_run_test.go`, `internal/app/service_test.go` — bind budgets to worker lifetimes and freeze resolved role choices.
- `internal/api/handlers/change_pipeline_handler_test.go`, `docs/api-contract.md`, `docs/openapi.yaml` — document and test configured defaults for omitted workflow role selections.

**Inputs / dependencies**
- Task 04; existing profile confirmation and inclusive retry budgets.

**Implementation rules**
- Add role-to-profile routing with the three current scopes as compatible defaults. Freeze assignments and destination grants at admission; explicit authorized fallback choices cannot cross consent boundaries.
- Start with the existing Creation, Testing and Review roles. Omitted workflow choices resolve to these configured defaults; explicit catalog choices retain precedence and require the same profile-specific consent. Later graph tasks add roles when their callers exist.
- Enforce cumulative model calls, tool calls, input/output tokens, elapsed time, concurrent branches, and repair limits across children and retries. Unknown usage is explicit; use reservations to prevent overspending through concurrency.
- Use ADK plugins for observations and policy hooks, while repeating essential checks at dispatch. Authority/cancellation/persistence failures are terminal. Tool repair cannot retry a mutation or amplify SDK/transport retries.

**Verification command**
`go test -race ./internal/config ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: enforce shared agent budgets and scoped routing`

## Task 06 — Persist sessions and run state locally

- [x] Complete

**Target files**
- `internal/app/agentstate/store.go`, `internal/app/agentstate/session.go`, `internal/app/agentstate/store_test.go` — **new**.
- `internal/app/agentruntime/session.go`, `internal/app/agentruntime/session_test.go` — **new**, ADK session service adapter.
- `internal/app/agentruntime/runtime.go`, `internal/app/service.go`, `internal/app/project_lifecycle.go`, `internal/app/worktree_service.go`, `internal/app/archgate/archgate.go`, `internal/app/archgate/archgate_test.go`.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/lock_unix.go`, `internal/app/agentstate/lock_windows.go`, `internal/app/agentstate/lock_other.go`, `internal/app/agentruntime/persistence.go` — **new**, versioned invocation/budget records, cancellable process locks and runtime persistence binding.
- `internal/app/agentruntime/budget.go`, `internal/app/agentruntime/policy_test.go`, `internal/app/agent_limits.go`, `internal/app/agent_limits_test.go`, `internal/app/service_test.go`, `internal/app/worktree_service_test.go`, `internal/app/analysis_run.go`, `internal/app/analysis_run_test.go` — bind canonical checkout identity, checkpoint reservations before dispatch, propagate persistence failures and verify passive restoration.
- `CONFIG.md`, `go.mod`, `go.sum` — document private session records and retain required direct dependency declarations.

**Inputs / dependencies**
- Task 05; [ADK session service obligations](https://github.com/google/adk-go/blob/v2.6.0/session/service.go) and existing atomic storage.

**Implementation rules**
- Store versioned records below private `.mini-orca/agents/`, separated by canonical project/worktree, session, and run. Bind revisions, source/policy/config fingerprints, schema identity, and cumulative budgets.
- Implement session state namespaces, durable event IDs, ordered appends, compaction actions, and copy-safe reads. Reject traversal, oversized/corrupt records, and unknown versions; never turn an I/O error into a missing session.
- Serialize each checkout's session namespaces, invocation records and cumulative budget checkpoints through a bounded atomic journal with a cancellable process lock. Persist reservations before model dispatch and usage before final publication; failed persistence terminates the run. Completion invocations retain private input state and explicit terminal status. Opening or restoring records does not resume them.
- Preserve existing change/analysis stores as domain authorities. Restore is passive; no model, tool, or process dispatch on opening a project. Exercise ADK's session conformance suite and concurrent append/write failures.

**Verification command**
`go test -race ./internal/app/agentstate ./internal/app/agentruntime ./internal/app ./internal/storage && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: persist local ADK sessions and run records`

## Task 07 — Add versioned local artifacts

- [x] Complete

**Target files**
- `internal/app/agentstate/artifacts.go`, `internal/app/agentstate/artifacts_test.go`, `internal/app/agentruntime/artifacts.go`, `internal/app/agentruntime/artifacts_test.go`, `internal/app/change_store_test.go` — **new**.
- `internal/app/agentruntime/runtime.go`, `internal/app/change_store.go`, `internal/app/change_checks.go`.
- `CONFIG.md` — document artifact privacy, bounds, retention and recovery protection.

**Inputs / dependencies**
- Task 06; existing proposal/check persistence and [ADK artifact interfaces](https://adk.dev/artifacts/).

**Implementation rules**
- Persist immutable artifact versions with hashes, MIME type, size, provenance, and session ownership. Attach proposal evidence and bounded check reports through an actual runtime consumer.
- Use a separate bounded atomic artifact catalog under the checkout's private agent directory and its existing process lock. Store completion artifacts through the ADK runner's artifact service and event references. Save deduplicated proposal/check snapshots before publishing domain history; snapshots remain supporting evidence, never Apply or Undo authority.
- Artifact writes stay inside private metadata; saving a generated artifact never changes project source. Validate names, symlinks, size limits, ownership, and permissions.
- Implement retrieval, listing, explicit deletion, retention, and export primitives. Protect artifacts referenced by active runs/recovery; report cleanup failures. No automatic external upload.
- Export returns verified bytes to an admitted caller; it does not accept a destination filesystem path. Retention is bounded and explicit, with immutable version counters that survive deletion. Recovery-pinned proposal evidence cannot be deleted through the generic SDK artifact service.

**Verification command**
`go test -race ./internal/app/agentstate ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add private versioned agent artifacts`

## Task 08 — Give agents bounded repository investigation tools

- [x] Complete

**Target files**
- `internal/app/agent_tools.go`, `internal/app/agent_tools_test.go`, `internal/app/agentruntime/tools.go`, `internal/app/agentruntime/tools_test.go` — **new**.
- `internal/project/agent_context.go`, `internal/project/agent_context_test.go` — **new**, pure indexed retrieval helpers.
- `internal/project/context_policy.go`, `internal/project/declaration_context.go`, `internal/app/change_context.go`, `internal/app/declaration_explanation.go`.
- `internal/app/agentruntime/tool_agent.go`, `internal/app/agentruntime/tool_agent_test.go` — **new**, bounded native ADK LLM-agent tool turns; `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/model.go`, `internal/app/agentruntime/policy.go`, `internal/app/agentruntime/policy_test.go`, `internal/app/agentruntime/persistence.go`, `internal/app/agentstate/run.go` — explicit tool admission, per-dispatch authority, private tool-schema provenance and replayable events.
- `internal/app/service.go`, `internal/app/service_test.go`, `internal/app/declaration_explanation_test.go`, `CONFIG.md`, `go.mod`, `go.sum` — reuse the existing retry owner for typed investigation requests, retain direct dependency declarations and document read-only tool bounds.

**Inputs / dependencies**
- Task 07; context policy, source hashes, symbol/import index, and instruction resolution.

**Implementation rules**
- Provide typed search, scoped file/snippet read, symbol/import inspection, impact lookup, and current-check-evidence tools. Reuse application/domain validation; the SDK never receives an unrestricted root filesystem tool.
- Apply exclusion, symlink containment, byte/result/token budgets, and cancellation on each call. Return path/hash/span provenance and mark heuristic relationships explicitly.
- Wire tools into explicit declaration investigation. Retrieval outside admitted scope produces a scope request; it cannot silently become source sent to a remote provider or a write target.
- Use the selected Function profile's verified tool protocol through an ADK LLM agent; completion-only CLI profiles retain bounded supplied-context explanations. Do not switch providers or grant a wider scope. A declaration investigation admits its exact source span and indexed metadata; every additional turn rechecks project, policy, instructions and source identity.
- Validate named tool schemas and results at dispatch, charge each actual tool call once, and cap an investigation at eight model turns with bounded aggregate retrieval. Keep tool-schema/scope identity and tool events in the private session. Proposal check evidence is read-only, source-free and explicitly associated with its checked proposal; unavailable or stale evidence never means tests passed.

**Verification command**
`go test -race ./internal/project ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add guarded repository investigation tools`

## Task 09 — Add an evidence-backed planning agent

- [x] Complete

**Target files**
- `internal/app/agent_investigation.go`, `internal/app/agent_investigation_test.go`, `internal/app/agentruntime/investigator.go` — **new**.
- `internal/app/change_plan.go`, `internal/app/change_plan_test.go`, `internal/api/handlers/change_handler.go`, `internal/api/handlers/change_plan_test.go`, `cmd/daemon/main.go`.
- `internal/tui/client/client.go`, `internal/tui/client/models.go`, `internal/tui/client/change_plan_test.go`, `internal/tui/plan.go`, `internal/tui/plan_actions.go`, `internal/tui/plan_test.go`, `internal/tui/keybinds_help.go`.
- `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md`.
- `internal/app/agent_tools.go`, `internal/app/agentruntime/tool_agent.go`, `internal/app/agentruntime/tool_agent_test.go` — reuse admitted repository tools and add structured final responses without SDK-owned output tools or a second retry loop.
- `internal/tui/plan_investigation.go`, `internal/tui/plan_investigation_test.go` — **new**, asynchronous investigation, cancellation and explicit acceptance; `internal/tui/models.go`, `internal/tui/terminal_loop.go`, `internal/tui/daemon_actions.go`, `internal/tui/focused_pages.go`, `internal/tui/command_menu.go` — connect the existing action loop, plan sections and command menu.

**Inputs / dependencies**
- Task 08; captured plans and explicit remote-provider confirmation.

**Implementation rules**
- Add an explicit Investigate action to the editable plan. Produce structured evidence, assumptions, suggested scope, approach, and acceptance criteria; validate cited hashes and locations.
- The user edits and approves the resulting plan before generation. Investigation does not approve its own write scope or execute project code.
- Preserve an existing draft on failure/cancellation, discard late results from another project/plan, and invalidate approval when accepted evidence or scope changes. Opening the plan remains passive.
- Expose a stateless, guarded `POST /api/projects/current/plans/investigate` operation. Bind the editable plan, explicit read/write paths, indexed source hashes, project revision, policy and instructions before dispatch. Missing new files carry explicit absence; changes to either existing or missing paths invalidate the request. Investigation creates only private run evidence, never a change session, worktree, source write or execution-trust grant.
- Return bounded typed citations, assumptions, suggested paths, approach and criteria. Check every citation against captured source hashes and line ranges before publication. Preserve completion-only CLI profiles and the selected Function profile's remote-consent requirement. Accepted citations and assumptions become optional plan fields included in its approval hash, with existing histories remaining readable.
- Add explicit Investigate, Cancel and Accept suggestions actions. Keep pending results separate from the editable plan, show them in the existing plan sections, reject late or edited-draft results, and retain draft content on failure. Acceptance updates evidence, approach and criteria; suggested scope remains advisory until the user edits it. Navigation and command-menu browsing remain passive.

**Verification command**
`go test ./internal/app ./internal/api/handlers ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add evidence-backed task investigation and planning`

## Task 10 — Migrate checked changes to an ADK graph

- [x] Complete

**Target files**
- `internal/app/agentruntime/change_graph.go`, `internal/app/agentruntime/change_graph_test.go`, `internal/app/agent_change_host.go`, `internal/app/agent_change_host_test.go` — **new**.
- `internal/app/change_pipeline.go`, `internal/app/change_pipeline_stages.go`, `internal/app/change_pipeline_types.go`, `internal/app/change_pipeline_resume.go`, `internal/app/change_pipeline_control.go`, `internal/app/change_pipeline_test.go`.
- `internal/app/change_checks.go`, `internal/app/change_baseline.go`, `internal/app/change_verify.go`.
- `internal/app/agentruntime/persistence.go`, `internal/app/agentruntime/artifacts.go`, `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go` — share private invocation-session setup, explicitly version graph records, and preserve completion/investigation history compatibility.
- `internal/app/worktree_service_test.go` — assert graph and nested completion journals remain isolated to the admitted worktree.
- `docs/api-contract.md` — describe graph execution and the retained domain approval boundary.

**Inputs / dependencies**
- Task 09; existing proposal parsing, pinned tests, baseline checks, isolated execution, and review contracts.

**Implementation rules**
- Implement explicit generation, test generation, baseline/verification, critic, and human-review nodes. Use typed node inputs/results and stable IDs; app callbacks retain the only access to domain mutations.
- Proposal tools may return only validated captured edits. Verification uses trusted controlled argv and the existing sandbox; never run model-authored commands.
- Preserve test-only generation, reproducibility labels, required checks, cancellation, selected models, and stage retry semantics. Replace the superseded sequential loop rather than running two engines for one request.
- Run native ADK sequential nodes with stable IDs `create`, `generate_tests`, `verify`, `review`, and `human_review`. The verification node retains the existing baseline/candidate check operation and controlled argv. Preserve the four current public stage summaries until the dynamic TUI task; the test summary spans test generation and verification.
- Carry typed project, workflow, conversation and proposal identities through every node. Revalidate at the host mutation boundary; read-only nodes cannot change proposal identity, and a model review never grants human approval. Resume skips only explicitly admitted prior stages with current evidence.
- Store bounded, source-free graph node progress and results in the private ADK journal using a distinct `change-graph-v1` schema and the shared root budget. Persist node start before calling the host, stop on cancellation/error/persistence failure, and never replay domain mutations from restored SDK events. Keep application-owned automatic repair behavior until the following specialist/repair task replaces its orchestration.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/app/sandbox && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: execute checked changes as an ADK graph`

## Task 11 — Add specialist review and bounded repair branches

- [x] Complete

**Target files**
- `internal/app/agentruntime/review_graph.go`, `internal/app/agentruntime/review_graph_test.go` — **new**.
- `internal/app/agentruntime/change_graph.go`, `internal/app/change_pipeline.go`, `internal/app/change_pipeline_stages.go`, `internal/app/change_pipeline_types.go`, `internal/app/change_pipeline_test.go`, `internal/app/change_pipeline_timeout_test.go`.
- `internal/app/agentruntime/graph_runtime.go`, `internal/app/agentruntime/repair_graph.go`, `internal/app/agentruntime/repair_graph_test.go`, `internal/app/agent_change_review.go`, `internal/app/agent_change_review_test.go` — **new**, shared durable graph execution, conditional repair orchestration, and host-owned specialist admission/publication.
- `internal/app/agent_change_host.go`, `internal/app/agent_change_host_test.go`, `internal/app/change_pipeline_resume.go`, `internal/app/change_pipeline_control.go`, `internal/app/change_pipeline_timeout.go`, `internal/app/change_plan_test.go`, `internal/app/change_checks_test.go`, `internal/app/change_checks.go` — propagate immutable review routing, preserve historical workflow compatibility, budget queued reviewers, and require explicit recovery for scope expansion.
- `internal/app/worktree_service_test.go` — include the repair controller's journal in isolated worktree execution assertions.
- `internal/app/agentstate/store.go`, `internal/app/agentstate/artifacts.go`, `internal/app/agentruntime/session.go`, `internal/app/agentruntime/session_test.go`, `internal/app/agentruntime/artifacts.go`, `internal/app/agentruntime/artifacts_test.go`, `internal/app/agentruntime/persistence.go`, `internal/app/agentruntime/policy_test.go` — distinguish canceled sibling persistence from storage failure so parallel failure cleanup cannot relabel or cancel the admitted root run.
- `internal/app/agentruntime/budget.go` — keep canceled child reservations from canceling siblings while retaining terminal handling for real persistence and budget failures.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go`, `internal/api/handlers/change_pipeline_handler_test.go`, `internal/tui/client/models.go`, `internal/tui/client/client_test.go`, `docs/api-contract.md`, `docs/openapi.yaml` — version specialist/repair journals and document/test the additive specialist evidence contract.

**Inputs / dependencies**
- Task 10; shared admission/budgets and immutable proposal snapshots.

**Implementation rules**
- Route to correctness, architecture, test-quality, security, and performance specialists when applicable. Security requires the existing explicit review intent. Parallel reviewers read the same immutable checked proposal.
- Join structured verdicts deterministically; missing/failed reviewers remain incomplete, and performance advice remains unmeasured without a benchmark.
- Retain opt-in repair capped at two attempts. Only actionable failed checks or critic findings can cause repair; cancellation, timeout, unavailable checks, authority errors, and scope expansion require the relevant recovery action.
- Select correctness and test-quality reviews for checked code changes, architecture for Go or multi-source changes, security only for explicitly confirmed security tasks, and performance only for performance tasks. Freeze this routing at admission. All branches use the selected Review profile and its existing destination consent; they share the root budget and bounded concurrency.
- Persist source-free specialist node progress before dispatch and publish bounded per-specialist evidence against the same proposal identity. Join in a canonical order; retain incomplete/failed branch states without publishing an aggregate approval. Legacy workflows without specialist fields remain readable.
- Replace the application's repair loop with a native conditional ADK graph with at most three attempts. The host alone determines repairability, reserves each repair before dispatch, refreshes current identity, and prepares the feedback. Persist each transition; no SDK retries or implicit write-scope expansion.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add specialist review and bounded graph repairs`

## Task 12 — Persist human input and guarded checkpoints

- [x] Complete

**Target files**
- `internal/app/agentruntime/confirmation.go`, `internal/app/agentruntime/confirmation_test.go`, `internal/app/agent_confirmation.go`, `internal/app/agent_confirmation_test.go` — **new**.
- `internal/app/agentstate/input.go`, `internal/app/agentstate/input_test.go` — **new**, SDK-independent waiting checkpoints and atomic input receipts.
- `internal/app/agentstate/session.go`, `internal/app/change_pipeline_control.go`, `internal/app/change_pipeline_resume.go`, `internal/app/change_pipeline_recovery_test.go`.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go`, `internal/app/agentstate/artifacts.go` — explicitly version confirmation runs and protect waiting checkpoints from deletion.
- `internal/app/agent_change_host.go`, `internal/app/change_pipeline_types.go`, `internal/app/change_checks.go`, `internal/app/change_session.go`, `internal/app/agent_change_host_test.go`, `internal/app/change_checks_test.go` — create the approval checkpoint at the checked handoff, bind existing Review to its receipt, and preserve passive recovery with fresh checks.
- `internal/api/handlers/change_pipeline_handler.go`, `internal/api/handlers/change_pipeline_handler_test.go`, `cmd/daemon/main.go`, `internal/tui/client/client.go`, `internal/tui/client/models.go`, `docs/api-contract.md`, `docs/openapi.yaml`.
- `internal/tui/client/client_test.go`, `cmd/daemon/main_test.go` — cover the guarded input route and additive checkpoint wire contract.

**Inputs / dependencies**
- Task 11; [graph human input](https://adk.dev/graphs/human-input/) and existing Apply/Undo authority.

**Implementation rules**
- Model pending input explicitly with a request ID, run/node ID, schema, exact action/scope, expiry, and proposal/source identity. Handle approve, reject, cancel, expiry, and duplicate responses idempotently.
- Resume only after fresh admission; persist the input receipt and checkpoint before further dispatch. A forged tool response or session state value cannot grant authority.
- Distinguish pausing graph execution from canceling a process. Reconcile interrupted effects before restart; never replay Apply, Git operations, or uncertain external writes automatically.
- At an approving checked-change handoff, create a native single-node ADK human-input graph with a unique request, a 24-hour deadline, and a `confirmation-v1` journal. Persist its waiting state and release the worker. The checkpoint contains only identity, captured path hashes, and the fixed `review_proposal` action.
- Resume this pure handoff graph only through the host's current project/configuration/policy binding and exact proposal/scope checks. Persist a decision receipt before native resume; retry after partial storage failure safely. Responses cannot invoke providers, checks, source mutation, or Git operations.
- Expose `workflow.input` and an explicit `/workflow/input` response route for approve, reject, and cancel. Existing Review records the same approval receipt after its current guards; existing cancellation also cancels waiting input. Preserve legacy workflows without checkpoints. Approval requires live passing checks and existing per-file review requirements; passive restore never reinstates in-memory Apply authority.

**Verification command**
`go test -race ./internal/app ./internal/app/agentruntime ./internal/app/agentstate ./internal/api/handlers ./cmd/daemon ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add durable human input and guarded checkpoints`

## Task 13 — Publish structured run history and resumable events

- [x] Complete

**Target files**
- `internal/app/agent_runs.go`, `internal/app/agent_runs_test.go`, `internal/api/handlers/agent_run_handler.go`, `internal/api/handlers/agent_run_handler_test.go` — **new**.
- `internal/app/agent_run_types.go`, `internal/app/agent_events.go`, `internal/app/agent_events_test.go` — **new**, source-free run DTOs, live ownership and bounded private text.
- `internal/app/agentstate/history.go`, `internal/app/agentstate/history_test.go`, `internal/app/agentruntime/events.go`, `internal/app/agentruntime/events_test.go` — **new**, durable progress cursors, hierarchy and SDK event projection.
- `internal/app/events.go`, `internal/app/agentstate/store.go`, `internal/app/agentruntime/runtime.go`, `internal/api/handlers/project_lifecycle_handler.go`, `cmd/daemon/main.go`, `cmd/daemon/worktree_router_test.go`.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/session.go`, `internal/app/agentstate/input.go`, `internal/app/agentstate/store_test.go`, `internal/app/agentruntime/persistence.go`, `internal/app/agentruntime/session.go`, `internal/app/agentruntime/graph_runtime.go`, `internal/app/agentruntime/confirmation.go` — persist parent relationships and progress at the same durable boundaries as execution and human input.
- `internal/app/service.go`, `internal/app/agent_limits.go`, `internal/app/agent_confirmation.go`, `internal/app/change_pipeline.go`, `internal/app/change_pipeline_types.go`, `internal/app/change_pipeline_resume.go`, `internal/app/change_pipeline_test.go` — connect lifecycle observation and cancellation to the owning service, and admit optional streaming without bypassing provider capabilities or consent.
- `internal/tui/client/agent_models.go`, `internal/tui/client/agent_runs_test.go` — **new**; `internal/tui/client/client.go`, `internal/tui/models.go`, `docs/api-contract.md`, `docs/openapi.yaml`.
- `internal/tui/client/agent_events.go` — **new**, bounded SSE decoding with cancellation and cursor validation; `internal/tui/client/models.go` — preserve the captured streaming choice.

**Inputs / dependencies**
- Task 12; existing loopback policy and project/worktree routing.

**Implementation rules**
- Add bounded list/detail/control/input routes below `/api/projects/current/agent-runs`. Creation remains through admitted domain actions, never arbitrary caller-supplied graphs.
- Emit source-free events with project/run/node/attempt IDs, sequence cursors, time, state, usage, and evidence references. Support reconnect, duplicate suppression, stale cursors, gap/snapshot recovery, and backpressure.
- Preserve existing project update subscribers during migration. Terminal state is durable and recoverable even if live delivery is dropped. Partial model text stays in an explicit private detail channel and is inert.
- Persist a bounded source-free progress journal with an epoch and monotonic sequence, retaining 2,048 events. Keep native session payloads private. Record native invocation parent IDs so a checked-change root groups its attempts, specialist reviews and waiting input. Older journals without these fields remain readable.
- Bind list/detail/events requests to the current project and checkout; paginate history and event replay. Expired or foreign cursors trigger an explicit snapshot reset, while duplicate events are suppressed by the client. Bound stream frames and slow writes; cancellation releases subscribers. Preserve the existing project event contract.
- Only active root invocations advertise cancel; cancel their shared budget through the owning service and let workers finish cleanup. Waiting confirmation inputs route through the existing domain response method with exact request/proposal identity. Historical state cannot restart work; retries still use admitted domain actions.
- Keep default metadata source-free. An explicit private detail request may read bounded model output and transient partial text with provenance and truncation state. Add opt-in `stream_output` to checked workflows; reject unsupported selected providers before dispatch and preserve the choice in workflow history. Viewing progress never starts streaming or model work.

**Verification command**
`go test -race ./internal/app ./internal/api/handlers ./cmd/daemon ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: expose durable agent runs and structured event streams`

## Task 14 — Adapt the TUI to dynamic workflows with restrained layout

- [x] Complete

**Target files**
- `internal/tui/hub_agents.go`, `internal/tui/workflow_progress.go`, `internal/tui/domain/domain.go`, `internal/tui/domain/remediation.go`, `internal/tui/state.go`, `internal/tui/dashboard.go`, `internal/tui/view.go`, `internal/tui/theme_styles.go`.
- `internal/tui/domain/agent_runs.go`, `internal/tui/domain/agent_runs_test.go` — **new**.
- `internal/tui/agent_progress.go`, `internal/tui/agent_progress_test.go` — **new**, guarded native progress reads and stable stage selection.
- `internal/tui/domain.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/daemon_actions.go`, `internal/tui/sse_events.go` — connect task rows, inspection and existing job polling to current workflow identity without starting model work.
- `internal/tui/domain/domain_test.go`, `internal/tui/state_test.go` — update stage/navigation expectations and preserve guarded task behavior.
- `internal/tui/workflow_progress_test.go`, `internal/tui/view_test.go`, `internal/tui/workspace_view_test.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 13; existing five workspaces, palettes, viewport behavior, and keyboard rules.

**Implementation rules**
- Replace hard-coded four-stage positions with stable stage IDs, typed states, and graph-derived progress. Render sequential and parallel work accurately without invented percentages.
- Resolve native history by exact project, checkout, change-session and workflow identity. Read bounded source-free detail on task inspection and refresh it with the existing active-job poll; private text remains opt-in in Task 15. Preserve truthful legacy summaries when native history is absent, and show unavailable history as unavailable. Ignore late results after scope or workflow replacement.
- Derive stage selection from stable node IDs, including separate test generation/verification and admitted parallel reviewers. Preserve the selected node and scroll position as progress changes; unknown stages remain inspectable rather than borrowing a positional label. Native invocation completion is execution metadata, never proposal approval.
- Present compact task rows and one clear next action; replace decorative fleet/mission terminology and repeated panels with plain task, stage, evidence, and recovery labels.
- Preserve all five workspaces, visible project/Git identity, command access, focus, and scroll state. At small sizes prioritize content and actions; support at least 52x16, 80x24, 100x20, and 160x40 plus the resize prompt.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: adapt the TUI to dynamic agent workflows`

## Task 15 — Add execution, evidence, and approval detail views

- [x] Complete

**Target files**
- `internal/tui/agent_run_view.go`, `internal/tui/agent_run_actions.go`, `internal/tui/agent_run_test.go` — **new**.
- `internal/tui/agent_run_stream.go`, `internal/tui/agent_run_stream_test.go` — **new**, owned cursor-based streaming and cancellation/reconnect tests.
- `internal/tui/terminal_loop.go`, `internal/tui/hub_agents.go`, `internal/tui/agent_progress.go` — connect asynchronous details to the existing event loop and preserve stage selection and scroll.
- `internal/app/agentruntime/private_events.go`, `internal/app/agentruntime/private_events_test.go` — **new**, bounded SDK event projection for explicitly requested tool arguments/results.
- `internal/app/agent_run_private.go`, `internal/app/agent_run_private_test.go` — **new**, selected-invocation private detail assembly.
- `internal/app/agent_runs.go`, `internal/app/agent_run_types.go`, `internal/tui/client/agent_models.go`, `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md` — extend the existing explicit private-output disclosure with bounded tool records; default history remains source-free.
- `internal/app/change_pipeline_control.go`, `internal/app/change_pipeline_read_test.go` (**new** test) — preserve a valid running snapshot when its known worker finishes between disk load and recovery classification; retain failure and orphan recovery semantics.
- `internal/tui/sse_events.go`, `internal/tui/daemon_actions.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/detail_view.go`, `internal/tui/state.go`, `internal/tui/workspace_scenarios_test.go`.

**Inputs / dependencies**
- Task 14; structured event/control/input API.

**Implementation rules**
- Provide Overview, Timeline, Evidence, Usage, and Recovery sections in task detail; expose tool arguments/results through bounded sanitized disclosures. Stream text without moving selection or forcing scroll-to-bottom.
- Wire pause, resume, cancel, stage retry, and pending-input decisions to their real guarded endpoints. Hide or explain unavailable actions; prevent duplicate submission.
- Keep pause/resume tied to the admitted analysis owner: native checked-change runs support cancel and guarded stage retry, not arbitrary pause or replay. Explain that limit in task recovery. Subscribe only while inspecting the current run; cancel and join the stream on scope change or TUI exit, reconnect with its last acknowledged cursor, and never dispatch model work from a refresh.
- Private text and tool records require explicit invocation selection. Keep arguments/results bounded, redact credential-bearing keys, sanitize terminal control characters, and show truncation. Opt-in output streaming applies to new workflows and retries through existing provider capability and consent checks; private text remains separate from source-free progress events.
- Pending approvals show exact scope and destination; keep per-file diff review and Apply separate. Test reconnects, old events, project switches, long content, terminal escape injection, and passive navigation.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add agent execution and approval detail views`

## Task 16 — Move resumable project analysis onto ADK workflows

- [x] Complete

**Target files**
- `internal/app/agentruntime/analysis_graph.go`, `internal/app/agentruntime/analysis_graph_test.go`, `internal/app/agent_analysis_host.go` — **new**.
- `internal/app/agent_analysis_host_test.go` — **new**, native history, cancellation, bounded window capture, and admission regression tests.
- `internal/app/agentruntime/events.go` — project bounded analysis node identities and truthful stage outcomes into source-free progress.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go` — admit the versioned analysis graph journal while preserving rejection of unknown schemas.
- `internal/app/agentstate/store.go`, `internal/app/agentstate/journal_read_test.go` (**new** test) — handle atomic journal replacement during reads with bounded retries, retaining private-file, symlink, size, and corruption checks under concurrent graph/feature work.
- `internal/app/analysis_run.go`, `internal/app/analysis_stage_runner.go`, `internal/app/analysis_run_progress.go`, `internal/app/analysis_run_store.go`, `internal/app/analysis_run_retry.go`, `internal/app/analysis_run_freshness.go`, `internal/app/analysis_run_features.go`.
- `internal/app/analysis_run_test.go`, `internal/app/analysis_run_recovery_test.go`, `internal/app/analysis_run_retry_test.go`, `internal/app/analysis_freshness_test.go`.

**Inputs / dependencies**
- Task 15; analysis selection/admission, resumable coverage, stage cache identity, and security intent.

**Implementation rules**
- Express per-file/stage analysis as bounded graph work with deterministic aggregation. Preserve deterministic security rules as distinct evidence and retain semantic/performance/security model scopes.
- Reuse only fresh matching results. Count every actual provider attempt across resume/retry, and preserve partial coverage and failed/canceled/unavailable distinctions.
- Keep selection, preview, filters, restore, and inspection passive. Project replacement/reindex cancels owned work and blocks stale publication. Retire the superseded dispatcher logic in the changed owner files.
- Capture at most 500 unfinished files and 2,000 ordered stage nodes per admitted window. Persist each native start before host dispatch; retain cumulative host attempt reservations and report publication. Native cancellation must settle the domain run as canceled; time/budget exhaustion remains resumable. Completed graph windows do not imply complete project coverage or successful findings.

**Verification command**
`go test -race ./internal/app ./internal/app/agentruntime ./internal/project && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: execute project analysis through ADK workflows`

## Task 17 — Integrate documentation, features, and explanations

- [x] Complete

**Target files**
- `internal/app/agentruntime/knowledge_graph.go`, `internal/app/agentruntime/knowledge_graph_test.go` — **new**.
- `internal/app/agent_knowledge.go`, `internal/app/agent_knowledge_test.go` — **new**, host integration and native-history/publication regressions.
- `internal/app/agent_limits.go`, `internal/app/agentruntime/events.go`, `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go` — bind import runs to the scanned checkout and register versioned, source-free knowledge progress.
- `internal/llm/generation.go`, `internal/llm/generation_test.go` — preserve typed refusal/incomplete-response diagnostics across the native adapter without exposing provider text.
- `internal/app/feature_suggestions.go`, `internal/app/declaration_explanation.go`, `internal/app/semantic_review.go`, `internal/app/performance_review.go`, `internal/app/security_review.go`, `internal/app/service.go`.
- `internal/project/project_overview.go`, `internal/project/architecture_overview_report.go`, `internal/project/architecture_overview_report_test.go`, `internal/project/handbook/handbook.go`, `internal/project/handbook/handbook_test.go`.
- `internal/app/feature_suggestions_test.go`, `internal/app/declaration_explanation_test.go`, `internal/app/project_analysis_contract_test.go`.
- `internal/project/project_test.go`, `internal/project/ignore_rules_test.go`, `internal/project/reindex_metrics_test.go`, `internal/app/analysis_freshness_test.go` — migrate fixtures to explicit factual capture and remove the superseded combined analyzer entry point.
- `internal/api/handlers/project_handler_test.go` — migrate the handler's imported-project fixture to the same explicit scan/interpretation boundary.
- `docs/openapi.yaml`, `docs/api-contract.md` — document the evidence-preserving handbook composition version and knowledge run behavior.

**Inputs / dependencies**
- Task 16; current report schemas, provenance, goals, and deterministic handbook facts. `internal/project/project_overview.go` owns the current analyzer; inject a provider-neutral adapter there.

**Implementation rules**
- Route all remaining model-backed knowledge operations through the shared runtime, tools, budget, and event infrastructure. Keep project facts and handbook composition independent of ADK.
- Publish only validated domain reports with anchored evidence and compatible history. Distinguish repository facts, suggestions, and unverified explanations; do not synthesize benchmark claims.
- Preserve explicit user actions and current provider scopes. Existing report readers and the Docs/Features TUI remain usable throughout migration.
- Capture import identity from a deterministic scan before native admission, recheck source/policy before publication, and preserve inventory with honest failed reports on provider timeout/refusal. Journal/policy failures remain errors. Keep host retries and failed feature-history semantics intact; do not add nested dispatchers to stages already owned by the analysis graph.
- Remove fabricated handbook dependencies, scenario steps, guards and decisions. Anchor component paths to the admitted inventory and label interpretations as advisory. Version the changed composition; rebuild old derived handbooks passively from retained reports/indexes without modifying historical model reports or sending a provider request.

**Verification command**
`go test ./internal/app ./internal/app/agentruntime ./internal/project/... && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate knowledge workflows with ADK`

## Task 18 — Add curated persistent project memory

- [x] Complete

**Target files**
- `internal/app/agentstate/memory.go`, `internal/app/agentstate/memory_test.go`, `internal/app/agentruntime/memory.go`, `internal/app/agentruntime/memory_test.go`, `internal/app/agent_memory.go`, `internal/app/agent_memory_test.go` — **new**.
- `internal/app/agent_memory_context.go`, `internal/app/agent_memory_context_test.go`, `internal/api/handlers/agent_memory_handler.go`, `internal/api/handlers/agent_memory_handler_test.go` — **new**, admitted context retrieval and explicit local curation routes.
- `internal/app/change_pipeline_stages.go`, `internal/app/agentruntime/investigator.go`, `internal/app/agentruntime/knowledge_graph.go`, `internal/app/agentruntime/runtime.go`.
- `internal/app/agentruntime/policy.go`, `internal/app/agent_knowledge.go`, `internal/app/agent_investigation.go`, `internal/app/change_session.go`, `internal/app/feature_suggestions.go`, `internal/app/declaration_explanation.go`, `internal/app/semantic_review.go`, `internal/app/security_review.go`, `internal/app/performance_review.go` — attach bounded, captured memory scopes at actual model owners and revalidate retrieval before transport dispatch.
- `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `docs/api-contract.md`, `docs/openapi.yaml` — register and document guarded memory list/detail/create/edit/accept/forget operations; TUI management follows in Task 26.
- `internal/app/analysis_run_test.go` — wait on the analysis worker's completion using its admitted window deadline, and cancel/join it on a failed wait. A generic three-second signal timeout is not the budget for a durable multi-stage workflow; preserve all completion, coverage, call-count and recovery assertions.

**Inputs / dependencies**
- Task 17; existing harness lessons and instruction graduation workflow.

**Implementation rules**
- Store accepted decisions, verified outcomes, and explicitly retained lessons with provenance, source revisions, scope, timestamps, and invalidation rules. Use a bounded local search implementation behind ADK's memory interface.
- Separate candidate memories from accepted memory; model text cannot grant itself authority or override AGENTS.md. A stale memory is advisory and visibly stale.
- Retrieve only admitted relevant records; support update, forget, retention, and project isolation. Avoid automatically retaining entire conversations or secrets; deletion removes retrieval eligibility immediately.
- Use the pinned [Go memory service interface](https://adk.dev/sessions/memory/) with bounded local search. Reject automatic session ingestion. Limit records, text, result counts and retention; keep private atomic storage and compare-and-swap versions. Editing accepted content makes it a candidate again; acceptance and forgetting require an explicit guarded user action.
- Scope memories to captured file hashes and optional declaration spans. Excluded, expired, changed-source or out-of-scope records cannot enter model context. Revision-only historical advice remains explicitly stale. Revalidate selected record versions and scope at the transport boundary, and charge retrieval/context to the same run budget. Verified-outcome text must come from current proposal check receipts, never an arbitrary claim supplied by a model.

**Verification command**
`go test -race ./internal/app/agentstate ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add curated local agent memory`

## Task 19 — Bound context with compaction and safe cache reuse

- [x] Complete

**Target files**
- `internal/app/agentruntime/compaction.go`, `internal/app/agentruntime/compaction_test.go`, `internal/app/agentstate/context_cache.go`, `internal/app/agentstate/context_cache_test.go` — **new**.
- `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/session.go`, `internal/app/agentruntime/budget.go`, `internal/app/change_context.go`, `internal/config/agent_runtime.go`, `CONFIG.md`, `config.example.yaml`.
- `internal/app/agentruntime/context.go`, `internal/app/agentruntime/context_test.go`, `internal/app/agent_context.go`, `internal/app/agent_context_test.go` — **new**, scoped context configuration, local composition reuse, and lossless user constraints around compacted advisory history.
- `internal/app/agent_limits.go`, `internal/app/change_session.go`, `internal/app/service.go`, `internal/config/agent_runtime_test.go`, `internal/config/config.go` — wire configuration and captured change authority into actual dispatch; preserve existing analysis attempt reservations and extend the strict YAML field allowlist.
- `internal/app/agentruntime/investigator.go`, `internal/app/agentruntime/tool_agent.go`, `internal/app/agentruntime/policy.go`, `internal/app/agentruntime/persistence.go` — protect the original admitted prompt, charge summaries to the same investigation/run, and persist native compaction events with source guards.
- `internal/app/agentruntime/provider_cache.go`, `internal/app/agentruntime/provider_cache_test.go` — **new**, a guarded GenAI cache adapter; `internal/app/agentruntime/gemini.go` — reuse validated cached prefixes in native generation.

**Inputs / dependencies**
- Task 18; [Go compaction support](https://adk.dev/context/compaction/) and durable session events.

**Implementation rules**
- Configure bounded history/tail retention and persist compaction records so restart cannot repeatedly summarize/bill the same events. Charge summarization to the same run and authorized model scope.
- Keep approved plans, hard constraints, grants, IDs, and source fingerprints outside lossy summaries. Revalidate reconstructed context; fail clearly if mandatory context exceeds budget.
- Cache deterministic retrieval/context by project, source, policy, instructions, schema, and configuration fingerprints. Provider-side prompt caching is capability-gated and opt-in with explicit remote retention/TTL; it never makes stale evidence current.
- Use native ADK token-threshold tail retention and its summarizer interface. Keep original investigation instructions/source and every user conversation message outside lossy summaries. Native compaction failures must stop dispatch, including failures that the SDK otherwise treats as optional bookkeeping.
- Persist a claim before a paid summary or cache creation, release file locks before model I/O, and retain completed receipts. An uncertain claim cannot silently trigger another paid request after restart. Revalidate authority before cache use and publication; reject corrupt/oversized state without overwriting it.
- ADK Go does not provide `ContextCacheConfig`. Implement explicit provider caching through the pinned GenAI Go client only for tested compatible `generateContent` model families. Default it off, require a bounded explicit TTL, pin resource identity to the configured destination/model, and reject unsupported configurations without a provider call. Do not assume Gemini 4/Interactions support explicit caches; [current provider documentation](https://ai.google.dev/gemini-api/docs/caching) excludes them.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app/agentstate ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add bounded context compaction and cache reuse`

## Task 20 — Integrate project skills through ADK

- [x] Complete

**Target files**
- `internal/app/agentruntime/skills.go`, `internal/app/agentruntime/skills_test.go`, `internal/app/agent_skills.go`, `internal/app/agent_skills_test.go` — **new**.
- `internal/config/agent_runtime.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agent_tools.go`, `internal/project/instructions.go`, `CONFIG.md`, `config.example.yaml`.
- `internal/config/config.go`, `internal/config/agent_runtime_test.go` — strict, immutable configured skill directories.
- `internal/app/agent_investigation.go`, `internal/app/agent_investigation_test.go`, `internal/app/declaration_explanation.go` — pass the caller context into skill admission and publish validated skill provenance in the existing context manifest.
- `internal/app/agentruntime/tools.go`, `internal/app/agentruntime/investigator.go`, `internal/app/agentruntime/tool_agent.go`, `internal/app/agentruntime/persistence.go` — adapt native skill tools into the guarded investigation, freeze manifests in private session identity, and retain loaded-file provenance in private run events.

**Inputs / dependencies**
- Task 19; [Go SkillToolset](https://adk.dev/skills/), context policy, and trusted execution.

**Implementation rules**
- Load configured skills lazily through validated manifests and scoped filesystem sources. Bound metadata, instructions, resources, paths, and hashes; expose active skill provenance in run events.
- Preserve repository instruction precedence and explicit task scope. Skills cannot expand grants, install packages, or run scripts automatically.
- If a skill needs execution, route an explicitly admitted, pinned command through the existing sandbox policy. Reject changed resources, escaping symlinks, invalid manifests, and unavailable tools with useful diagnostics.
- Use the native Go SkillToolset's `list_skills`, `load_skill`, and `load_skill_resource` through existing tool budgets and schema checks. Advertise metadata only in tool-capable plan/declaration investigations; instructions and resources enter context on demand. Require indexed hashes for configured project-relative manifests and resources, and revalidate loaded content before dispatch and publication.
- The pinned Go SkillToolset has no script execution tool. Loading `scripts/` text does not execute it or grant shell access. Existing explicitly trusted, fixed workflow checks remain the execution path; reject manifests requiring tools outside the admitted inventory. Do not inherit the SDK's default instruction to follow skill text exactly over repository/task authority.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/project ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate scoped ADK skills`

## Task 21 — Add managed MCP connections and tools

- [x] Complete

**Target files**
- `internal/config/integrations.go`, `internal/config/integrations_test.go`, `internal/app/agent_integrations.go`, `internal/app/agent_integrations_test.go`, `internal/app/agentruntime/mcp.go`, `internal/app/agentruntime/mcp_test.go` — **new**.
- `internal/config/config.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/tools.go`, `internal/app/service.go`, `cmd/daemon/main.go`, `CONFIG.md`, `config.example.yaml`.
- `internal/app/agentruntime/mcp_transport.go`, `internal/app/agentruntime/mcp_protocol.go`, `internal/api/handlers/agent_integrations_handler.go`, `internal/api/handlers/agent_integrations_handler_test.go` — **new**, bounded native transports/protocol policy and explicit discovery/revocation routes.
- `internal/app/agent_integrations_process.go` — **new**, host-owned process groups, private working directories and credential references; `internal/app/project_lifecycle.go`, `internal/app/change_pipeline_control.go` — cancel and join in-flight integration operations during replacement, reindex and shutdown.
- `internal/app/sandbox/process_unix.go`, `internal/app/sandbox/process_unsupported.go`, `internal/app/sandbox/runner_test.go` — verify that an owned process group is absent before accepting a cleanup signal race after its leader exits.
- `internal/app/agent_tools.go`, `internal/app/agent_investigation.go`, `internal/app/declaration_explanation.go`, `internal/tui/client/models.go`, `docs/api-contract.md`, `docs/openapi.yaml`, `go.mod`, `go.sum` — per-investigation integration grants, production callers and wire/dependency contracts.
- `internal/tui/client/change_plan_test.go` — preserve integration grant fields through the existing investigation client contract.

**Inputs / dependencies**
- Task 20; [ADK MCP toolsets](https://adk.dev/tools-custom/mcp-tools/), shared policy, and process ownership.

**Implementation rules**
- Support explicitly configured stdio and Streamable HTTP servers with tool allowlists, endpoint-bound authentication, bounded discovery, timeouts, and owned connection/process cleanup.
- Inspecting configuration stays passive. Connection/discovery and tool execution require appropriate grants; metadata cannot self-declare a mutating tool safe. Pin discovered tool schemas for a run and invalidate approval on changes.
- Validate every call and result through Mini-Orca budgets, source policy, and effect classification. Cancellation closes sessions and owned children. Never auto-install an MCP server or expose arbitrary filesystem/shell capabilities.
- Use short-lived connections for explicit discovery and each admitted call; retain only expiring schema inventories between requests. Reconnect for a call only under that run's explicit grant, re-discover the selected schema, and close the connection before returning. Prevent SDK reconnect/retry from dispatching a second call. Allow only operator-classified external read tools in this step; reject mutating/execution classifications instead of trusting server annotations. The following task supplies exact-action approval and receipts for curated HTTP mutations.

**Verification command**
`go test -race ./internal/config ./internal/app/agentruntime ./internal/app ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate managed MCP connections and scoped tools`

## Task 22 — Add curated OpenAPI and grounding tools

- [x] Complete

**Target files**
- `internal/app/agentruntime/openapi.go`, `internal/app/agentruntime/openapi_test.go`, `internal/app/agentruntime/grounding.go`, `internal/app/agentruntime/grounding_test.go` — **new**.
- `internal/app/agent_integrations.go`, `internal/app/agent_tools.go`, `internal/config/integrations.go`, `internal/config/integrations_test.go`, `CONFIG.md`, `config.example.yaml`.
- `internal/app/agentruntime/openapi_schema.go`, `internal/app/agentruntime/openapi_http.go` — **new**, strict inline schema/parameter support and single-attempt HTTP execution.
- `internal/app/agent_openapi.go`, `internal/app/agent_openapi_test.go`, `internal/app/agent_api_actions.go`, `internal/app/agent_api_actions_test.go`, `internal/app/agentstate/actions.go`, `internal/app/agentstate/actions_test.go` — **new**, captured API inventories, exact-action review, durable dispatch claims and outcome reconciliation; `internal/app/agentstate/store.go` — journal ownership and validation.
- `internal/app/agent_research.go`, `internal/app/agent_research_test.go`, `internal/app/agent_research_report.go` — **new**, explicit grounded research and a private report containing inline/aggregate citations and the returned Search suggestions.
- `internal/app/agent_runs.go` — route grounded answer disclosure through the report instead of the generic plain-text output view; `go.mod`, `go.sum` — declare the already-pinned HTML tokenizer as a direct dependency for isolating Search suggestion fragments.
- `internal/app/agentruntime/gemini.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/persistence.go`, `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go`, `internal/llm/capabilities.go`, `internal/llm/capabilities_test.go`, `internal/config/config.go` — native search admission, grounded response metadata, persisted invocation identity and supported capability/configuration contracts.
- `internal/api/handlers/agent_apis_handler.go`, `internal/api/handlers/agent_apis_handler_test.go`, `internal/api/handlers/agent_research_handler.go`, `internal/api/handlers/agent_research_handler_test.go` — **new**, passive inventory/report reads, explicit API discovery, approval/reconciliation and research requests; `cmd/daemon/main.go`, `docs/api-contract.md`, `docs/openapi.yaml` — route and wire contracts. Add TUI client methods with their Settings callers in Task 25.

**Inputs / dependencies**
- Task 21; approved endpoint/tool inventory and Gemini provider capabilities.

**Implementation rules**
- Wrap explicitly selected OpenAPI operations as typed tools using a documented supported schema subset. Pin operation/schema identity, method, destination, auth reference, and effect class; reject unsupported schema constructs before dispatch.
- Keep HTTP argument mapping, response limits, redirection, credential isolation, and idempotency explicit. Mutating operations require exact-action approval and an outcome receipt; unknown outcomes require reconciliation.
- Add optional Google Search grounding only for supported profiles with explicit network/source consent. Preserve external citations and uncertainty; external text cannot become repository facts or executable instructions.
- OpenAPI discovery reads an explicitly configured, indexed project JSON specification through source policy. Use a documented OpenAPI 3.0 inline schema subset and exact operation allowlists; configuration owns destinations, credential references and effect classification. Discovery/grant expiry, source changes and revoke invalidate admitted calls.
- Mutating tools create reviewable candidates only. Explicit human approval claims a durable pending receipt before one HTTP attempt. Candidates retain an in-memory source authority tied to the originating investigation and expiring integration grant; restart or revoke requires a new candidate. Pending/unknown outcomes block further writes through that operation until explicit external reconciliation, which is labelled user-reported. Preserve receipts after cancellation or project changes.
- Grounded research is a separate, explicit search-only ADK invocation using the configured Function profile. Do not mix search with planning JSON/function tools or silently change providers. Return a private report reference; render the answer with its validated citations and original Search suggestions in an isolated, script-free report. Keep model dispatch accounting distinct from unknown provider-internal search usage. Follow the [pinned GenerateContent grounding contract](https://ai.google.dev/gemini-api/docs/generate-content/google-search) and [display requirements](https://firebase.google.com/docs/ai-logic/grounding-google-search).

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add curated API and grounded research tools`

## Task 23 — Consume remote agents through A2A

- [x] Complete

**Target files**
- `internal/app/agentruntime/a2a_client.go`, `internal/app/agentruntime/a2a_client_test.go`, `internal/app/agent_delegation.go`, `internal/app/agent_delegation_test.go` — **new**.
- `internal/config/integrations.go`, `internal/app/agent_integrations.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentstate/store.go`, `CONFIG.md`, `config.example.yaml`.
- `internal/config/a2a.go`, `internal/config/a2a_test.go`, `internal/app/agent_remote_connections.go`, `internal/app/agent_remote_connections_test.go` — **new**, passive endpoint configuration, explicit card discovery and expiring grants; `internal/config/config.go` — strict configuration keys.
- `internal/app/agentruntime/a2a_transport.go`, `internal/app/agentruntime/a2a_events.go`, `internal/app/agentruntime/a2a_events_test.go` — **new**, fixed-destination single-attempt JSON-RPC/SSE transport and advisory-only event conversion; `internal/app/agentruntime/graph_runtime.go` — optional native streaming mode for the shared runner.
- `internal/app/agentstate/delegations.go`, `internal/app/agentstate/delegations_test.go`, `internal/app/agent_delegation_control.go` — **new**, durable message/task identity, exclusive dispatch leases, explicit refresh/reconnect/input/cancel and uncertain-outcome reconciliation; `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go` — version native delegation journals.
- `internal/app/agent_external_context.go` — **new**, shared bounded explicit source capture for external requests; `internal/app/agent_research.go`, `internal/app/agent_research_test.go` — reuse this owner while preserving research consent and report behavior.
- `internal/api/handlers/agent_delegation_handler.go`, `internal/api/handlers/agent_delegation_handler_test.go` — **new**, passive remote-agent/delegation views and explicit network actions; `cmd/daemon/main.go`, `docs/api-contract.md`, `docs/openapi.yaml`, `go.mod`, `go.sum` — production routes, contract and the A2A SDK version already selected by ADK. TUI methods join their Settings callers in Task 25.

**Inputs / dependencies**
- Task 22; [Go A2A support](https://adk.dev/a2a/) and external endpoint admission.

**Implementation rules**
- Fetch and pin trusted configured agent cards only on explicit connection. Delegate bounded analysis/review tasks with minimum admitted context, cancellation, deadlines, durable remote task IDs, and actual usage/unknown-usage states.
- Remote findings/artifacts remain untrusted advisory inputs. Remote agents cannot grant local permissions, enlarge write scope, apply changes, or request additional credentials implicitly.
- Handle streaming reconnect, input-required, auth failure, malformed artifacts, and uncertain outcomes. Do not retry a remote side effect without an established idempotency contract.
- Use native ADK `remoteagent/v2` with the pinned A2A SDK, an explicit A2A 1.0 JSON-RPC interface, and configured card/endpoint URLs on the same origin. Discovery is bounded and returns a 15-minute card fingerprint/grant; unsupported required extensions, endpoint changes or credential changes reject admission. Never fetch icons, documentation, arbitrary artifact URLs or additional credentials.
- Expose explicit analysis/review delegation with an allowlisted advertised skill, a bounded question and up to eight captured path/hash selections. Share only this admitted context, without ambient skills, memory or other session history. Require fresh delegation and source-sharing consent. Each network operation consumes a shared tool allowance and bounded elapsed time; remote internal model/search usage remains unknown unless valid usage metadata is explicitly labelled remote-reported.
- Persist a unique outgoing message identity and pending lease before dispatch; retain validated remote task/context IDs as events arrive. Return only bounded plain-text advisory messages/artifacts with separate incomplete, input-required, auth-required, failed, canceled and unknown states. Custom conversion cannot populate local function calls, transfer actions, permissions or source mutation.
- Initial sends and input replies make one attempt. Stream reconnect is an explicit subscription to a retained task, never a resend. Refresh queries task status; cancel targets the retained task. Input requires the current receipt version, live source/card authority and explicit reply consent. Cancellation joins local transport and attempts bounded remote cleanup, retaining unknown status when confirmation is unavailable. A pending/unknown operation cannot be replayed; explicit user-reported reconciliation clears uncertainty without network dispatch. Passive reads and daemon restart never resume work.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add guarded A2A delegation`

## Task 24 — Expose a guarded Mini-Orca agent endpoint

- [x] Complete

**Target files**
- `internal/app/agent_remote_service.go`, `internal/app/agent_remote_service_test.go`, `internal/app/agentruntime/a2a_server.go`, `internal/app/agentruntime/a2a_server_test.go` — **new**.
- `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `internal/config/integrations.go`, `internal/api/local_policy.go`, `internal/api/request_test.go`, `CONFIG.md`, `docs/api-contract.md`, `docs/openapi.yaml`.
- `internal/config/a2a_server.go`, `internal/config/a2a_server_test.go` — **new**, disabled-by-default literal-loopback listener and explicit per-project principal/capability configuration; `internal/config/config.go`, `config.example.yaml` — strict keys and passive defaults.
- `internal/app/agent_remote_work.go` — **new**, bounded request decoding and normal investigation/proposal admission; `internal/app/agent_integrations.go` — temporary inbound grants and lifecycle cancellation.
- `internal/app/agentruntime/a2a_server_guard.go`, `internal/app/agentruntime/a2a_server_lifecycle.go` — **new**, bounded native JSON-RPC admission, authenticated task ownership, duplicate-message rejection and joined execution cleanup.
- `internal/api/handlers/agent_remote_service_handler.go`, `internal/api/handlers/agent_remote_service_handler_test.go` — **new**, passive principal inventory and explicit local grant/revoke actions. TUI callers join Task 25.
- `cmd/daemon/a2a_server.go`, `cmd/daemon/a2a_server_test.go` — **new**, optional listener construction, existing Host/Origin policy and shutdown wiring.

**Inputs / dependencies**
- Task 23; existing local request policy, run lifecycle, and SDK authentication/authorization facilities.

**Implementation rules**
- Add an opt-in A2A listener on loopback by default with explicit authenticated principals and per-project capabilities. Its agent card advertises only implemented operations.
- Route requests through normal application admission. Allow analysis and proposal preparation; source Apply/Undo/Merge remain local human actions.
- Separate public metadata from private sessions/artifacts. Test authentication, project isolation, revoked grants, resource limits, cancellation, and absence of default exposure. Do not mount the unrestricted ADK REST/dev server onto the daemon.
- Use the pinned native ADK A2A executor and A2A 1.0 JSON-RPC handler. Accept only bounded text requests for implemented skills, with strict project/revision and 1–8 source hashes; forbid client-selected session/context IDs, metadata, attachments, push delivery and source mutation operations. Explicitly check authenticated task ownership before read, subscription and cancellation.
- Configuration authenticates a principal but grants no project access. Local grant/revoke controls bind a 15-minute permission to the current project, capabilities, credential and provider configuration; require source-sharing and relevant remote-provider consent. Reindex, project replacement, revocation and shutdown cancel owned work. Provider confirmation is also required on each applicable request.
- Bound request/output size, active executions and retained task/message identities. Never replay a message ID. A2A task/session state is private and process-local; restart removes grants and task access, while ordinary local proposal/run journals remain durable. Browsing cards or local inventory never dispatches work. Disconnected requests, explicit cancel and shutdown terminate and join execution; failure details cannot expose raw provider diagnostics.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/api ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: expose a guarded local A2A service`

## Task 25A — Persist guarded settings and freeze admitted configuration

- [x] Complete

**Target files**
- `internal/api/handlers/agent_settings_handler.go`, `internal/api/handlers/agent_settings_handler_test.go`, `internal/app/agent_settings.go`, `internal/app/agent_settings_test.go` — **new**.
- `internal/app/agent_settings_runtime.go`, `internal/app/agent_settings_skills.go`, `internal/app/agentstate/settings.go`, `internal/app/agentstate/settings_test.go`, `internal/app/agentruntime/skill_inspection.go`, `internal/app/agentruntime/skill_inspection_test.go` — **new**, private versioned settings, immutable admission snapshots and explicit local skill inspection.
- `internal/app/agentstate/store.go`, `internal/app/agent_limits.go`, `internal/app/agent_limits_test.go`, `internal/app/agent_context.go`, `internal/app/agent_skills.go`, `internal/app/agent_investigation.go`, `internal/app/declaration_explanation.go`, `internal/app/agent_change_review.go`, `internal/app/change_pipeline.go`, `internal/app/change_pipeline_stages.go`, `internal/app/change_pipeline_timeout.go`, `internal/app/change_pipeline_timeout_test.go` — capture effective settings before investigation/workflow preparation and reuse them throughout nested calls, budgets, repair loops and parallel reviews.
- `internal/app/agent_confirmation.go`, `internal/app/agent_confirmation_test.go` — preserve the captured settings fingerprint when resolving a pure human-input handoff; still reject changed operator configuration, project, policy or proposal identity.
- `cmd/daemon/main.go`, `docs/api-contract.md`, `docs/openapi.yaml`, `CONFIG.md`.

**Inputs / dependencies**
- Task 24; provider capability catalog, configured role/budget ceilings, explicit skill directories, private checkout journal and current-project guards.

**Implementation rules**
- Store source-free project overrides for role scopes, budgets and disabled configured skills using existing private atomic storage, version compare-and-swap and configuration fingerprints. Missing overrides preserve configured defaults; corrupt or stale settings cannot silently become defaults. Explicit reset restores current configuration. Budgets cannot exceed operator-configured ceilings, and settings cannot add skill directories, credentials, providers or tools.
- Expose passive local inventory, explicit confirmed mutation and explicit bounded local skill validation through strict HTTP contracts. Skill validation describes metadata and required tools without executing resources or granting tools. Keep validation, enablement and actual run-specific tool availability distinct.
- Freeze settings once per admission, including role defaults, skill selection, repair/concurrency limits and journal fingerprint. Nested work reuses that snapshot. Changed settings apply to subsequent admission and survive restart; copied checkout state, stale revisions/versions, write failures and cancellation remain explicit failures.
- Keep existing model presets and explicit per-request model selections authoritative. No provider requests, process execution, source writes or grant acquisition occur through these settings controls.

**Verification command**
`go test -race ./internal/app/agentstate ./internal/app/agentruntime ./internal/app ./internal/api/handlers ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: persist guarded agent settings for future runs`

## Task 25B — Add professional role, budget and skill settings views

- [x] Complete

**Target files**
- `internal/tui/client/agent_settings.go`, `internal/tui/client/agent_settings_test.go`, `internal/tui/agent_settings.go`, `internal/tui/agent_settings_test.go`, `internal/tui/agent_settings_commands.go`, `internal/tui/agent_settings_actions.go` — **new**, wire contracts, compact lists, selected-item details and explicit controls.
- `internal/tui/agent_settings_workflow.go` — **new**, role-aware workflow model selection and destination-bound remote confirmation. `internal/tui/plan.go`, `internal/tui/plan_test.go`, `internal/tui/workflow_progress.go`, `internal/tui/workflow_progress_test.go` — preserve plan/retry identity while confirming the selected workflow destinations.
- `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/hub_settings.go`, `internal/tui/hub_overview.go`, `internal/tui/domain/models.go`, `internal/tui/model_details_test.go`, `internal/tui/state.go`, `internal/tui/state_test.go`, `internal/tui/daemon_actions.go`, `internal/tui/sse_events.go`, `internal/tui/terminal_loop.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/colon_commands_actions.go`, `internal/tui/command_menu.go`, `internal/tui/command_menu_test.go`, `internal/tui/keybinds_help.go`, `internal/tui/view_test.go`.
- `docs/UI_DESIGN_GUIDELINES.md`, `CONFIG.md`.

**Inputs / dependencies**
- Task 25A; existing Settings navigation, model presets, scope assignments and confirmation patterns.

**Implementation rules**
- Keep Models, Roles/Budgets and Skills as compact Settings lists with selected-item details, bounded sanitized text and stable selection. Present configured, overridden, stale, enabled, disabled, valid and unavailable states accurately.
- Wire role assignment, individual budget editing, skill validation/enable/disable and explicit reset to guarded backend controls. Show the future-run effect before mutation, preserve model presets/scope assignments and use role defaults when constructing workflow model selections and corresponding provider confirmations.
- Opening or selecting entries is passive. Handle unavailable daemon data, stale results, narrow terminals, all palettes and keyboard help without introducing inert controls.
- Label the canonical checkout being edited and allow explicit selection of the main checkout or selected task worktree. Their private overrides remain separate. Load local settings without provider discovery, bind asynchronous results and mutations to the selected checkout/project/version, and make cancellation preserve an honest unknown-save outcome until refresh. Remote workflow confirmation discloses the selected role models/destinations and captured source scope; changing the task or model selection invalidates the confirmation.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add role budget and skill settings views`

## Task 25C — Connect scoped integrations through explicit TUI controls

- [x] Complete

**Target files**
- `internal/tui/client/integrations.go`, `internal/tui/client/integrations_test.go`, `internal/tui/agent_connections.go`, `internal/tui/agent_connections_test.go`, `internal/tui/agent_connections_actions.go`, `internal/tui/agent_connections_commands.go` — **new**, MCP/OpenAPI/outbound A2A/inbound principal inventory and guarded controls.
- `internal/tui/agent_connections_plan.go` — **new**, draft-scoped tool selection and destination-bound source-sharing confirmation. `internal/tui/plan.go`, `internal/tui/agent_settings_actions.go` — carry ephemeral investigation selections and reuse checkout identity checks without copying grants into saved plans or new worktrees. `internal/tui/view_test.go`, `internal/tui/command_menu_test.go` — cover the new navigation and action affordances.
- `internal/tui/view.go` — window the expanded Settings subnavigation around the active view while retaining every hub and all three model summaries at the minimum sidebar height.
- `internal/tui/client/client.go`, `internal/tui/client/models.go`, `internal/tui/hub_settings.go`, `internal/tui/hub_overview.go`, `internal/tui/state.go`, `internal/tui/state_test.go`, `internal/tui/daemon_actions.go`, `internal/tui/sse_events.go`, `internal/tui/terminal_loop.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/plan_investigation.go`, `internal/tui/plan_investigation_test.go`, `internal/tui/client/change_plan_test.go`.
- `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 25B; implemented MCP/OpenAPI/A2A configuration and short-lived grants, existing asynchronous TUI action ownership and captured plan investigations.

**Implementation rules**
- Add a compact Connections list with destination, configured permissions, credential references and grant expiry in selected details. Preserve distinct configured, disconnected, unavailable, expired and failed states; distinguish inbound principals from outbound remotes.
- Connect/discover and revoke/disconnect require explicit user actions and relevant confirmations. External I/O is asynchronous and cancellable, stale results cannot replace a new project/revision or selection, and browsing/local refresh cannot initiate external network or process work.
- Select at most two currently granted MCP/API connections for an explicit plan investigation. Capture tool grants and source-sharing consent with the same project, revision, source hashes and plan input. Never attach connections to unrelated requests or treat inbound authorization as source Apply permission.
- Label main/task checkout scope explicitly, keep their connection grants separate, and preserve selection by connection kind and ID on refresh. Partial inventory failures and canceled discovery/grant outcomes require local refresh before reuse. Bind investigation confirmation to both provider destinations and selected connection inventories; changed selections, grants, routing or draft input require fresh consent. OpenAPI schema discovery does not establish endpoint availability.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add explicit scoped connection controls`

## Task 25D — Review external actions and remote work from the TUI

- [x] Complete

**Target files**
- `internal/tui/agent_external_work.go`, `internal/tui/agent_external_work_test.go`, `internal/tui/agent_external_actions.go`, `internal/tui/agent_external_commands.go`, `internal/tui/browser.go`, `internal/tui/browser_test.go` — **new**, exact external action review, research and delegation actions with explicit report viewing.
- `internal/tui/agent_external_capture.go`, `internal/tui/agent_external_controls.go` — **new**, immutable source capture and destination consent, separate from exact receipt controls. `internal/tui/client/external_models.go`, `internal/tui/client/external_work.go`, `internal/tui/client/external_work_test.go` — **new**, SDK-free receipt DTOs and guarded local API contracts; keep connection inventory types focused on connections.
- `internal/tui/client/client.go`, `internal/tui/hub_overview.go`, `internal/tui/state_test.go`, `internal/tui/view_test.go`, `internal/tui/agent_connections_commands.go` — register exact routes and External Work navigation, with compact-layout and keyboard coverage and a visible transition from connection details.
- `internal/tui/client/integrations.go`, `internal/tui/client/integrations_test.go`, `internal/tui/agent_connections.go`, `internal/tui/agent_connections_actions.go`, `internal/tui/hub_settings.go`, `internal/tui/state.go`, `internal/tui/daemon_actions.go`, `internal/tui/sse_events.go`, `internal/tui/terminal_loop.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/plan_investigation.go`.
- `docs/UI_DESIGN_GUIDELINES.md`, `CONFIG.md`.

**Inputs / dependencies**
- Task 25C; exact API action journal, retained Google research reports, A2A delegation receipts and existing captured plan/source scope.

**Implementation rules**
- Review the exact persisted API request and current version/hash before one confirmed dispatch. Preserve unknown external outcomes, require explicit reconciliation, and never offer an automatic retry after uncertain delivery.
- Start research or A2A analysis/review only from explicit bounded captured input, with destination/source-sharing/provider consent and configured grants. Show advisory output and remote-reported usage distinctly. Expose explicit refresh, reconnect, cancel, input and reconciliation controls only when the current receipt permits them.
- Browsing local history is passive; external actions own their cancellation and late-result checks. Open a retained research HTML report only on an explicit action, using the guarded loopback report URL so its citations and required suggestions retain their rendered form. Never shell-interpolate model content or claim arbitrary HTML was rendered in the terminal.
- Capture the current plan's existing source hashes through a local index read before consent, or require an explicit question-only choice. Keep research capability, provider destination and remote card/grant identity bound through confirmation and queued dispatch. Preserve partial history failures and selection by receipt kind/ID; uncertain control outcomes require local refresh before another action.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: review external actions and remote agent work in the TUI`

## Task 26A — Expose a scoped artifact library with guarded export and deletion

- [x] Complete

**Target files**
- `internal/app/agent_library.go`, `internal/app/agent_library_test.go`, `internal/api/handlers/agent_library_handler.go`, `internal/api/handlers/agent_library_handler_test.go` — **new**.
- `internal/app/agentstate/artifacts.go`, `internal/app/agentstate/artifacts_test.go` — metadata catalog access and exact version/hash deletion with authorization rechecked under the journal lock and at persistence.
- `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 25D; local artifact and curated memory stores.

**Implementation rules**
- List only artifacts with host-recorded project ownership, with bounded pagination tied to the catalog snapshot. Return metadata, versions, provenance/source ownership, revision freshness, size and protection; payload disclosure is a separate passive detail read.
- Provide bounded text previews and binary metadata, explicit export and exact version/hash deletion. Export writes only a new private file in the checkout's guarded `.mini-orca/exports/artifacts` directory and returns its actual path/hash; it cannot target project source or arbitrary caller-supplied paths.
- Preserve protected evidence and artifacts referenced by unfinished runs. Recheck project/revision and exact content identity for mutations, including under persistence guards. Report failed storage and changed catalogs instead of silently returning empty data. Reuse the existing curated memory API in the subsequent TUI step.

**Verification command**
`go test ./internal/app/agentstate ./internal/app ./internal/api/handlers ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: expose guarded artifact library operations`

## Task 26B — Add artifact and memory management to the TUI

- [x] Complete

**Target files**
- `internal/tui/agent_library.go`, `internal/tui/agent_library_actions.go`, `internal/tui/agent_library_commands.go`, `internal/tui/agent_library_test.go`, `internal/tui/client/agent_library.go`, `internal/tui/client/agent_library_test.go` — **new**, scoped inventory, private disclosure and explicit library mutations.
- `internal/tui/agent_library_capture.go`, `internal/tui/agent_library_controls.go` — **new**, captured source/proposal provenance and exact version confirmations, separate from transport and navigation.
- `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/state.go`, `internal/tui/state_test.go`, `internal/tui/hub_settings.go`, `internal/tui/hub_overview.go`, `internal/tui/hub_agents.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/daemon_actions.go`, `internal/tui/sse_events.go`, `internal/tui/terminal_loop.go`, `internal/tui/view_test.go`, `docs/UI_DESIGN_GUIDELINES.md`, `docs/api-contract.md`.

**Inputs / dependencies**
- Task 26A; guarded artifact library and existing curated memory/lesson stores.

**Implementation rules**
- Add artifact/evidence navigation within Code and memory management beside Lessons within Settings. Show versions, source links, freshness, ownership, size, retention, and candidate/accepted state. Keep metadata reads and selection passive; disclose retained text explicitly and use the existing scrollable read-only text layout, with binary metadata instead of unsupported media rendering.
- Implement explicit artifact preview/export/delete and memory create/edit/accept/reject/forget using current checkout, record version/hash and complete confirmations. Rejecting a candidate removes it through guarded forget; accepted advice remains a separate explicit forget action. Creation/editing capture selected current source hashes; retaining proposal lessons/check outcomes must use their existing provenance guards.
- Preserve protected/active recovery references, reject changed scope or stale confirmations, sanitize terminal text, and keep cancellation/partial reads/late replies honest. Export displays the daemon's private-file receipt and never grants source Apply. Maintain stable selection and compact keyboard navigation.
- Capture manual advice from the selected plan or open file, and recapture edited records' source hashes. Retaining a proposal lesson or check outcome derives text through the existing receipt owner. Source links explicitly read the selected path with index/scope and content-hash checks; opening a source never changes library authority.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add artifact and memory library views`

## Task 27A — Distinguish reported usage from conservative budget charges

- [x] Complete

**Target files**
- `internal/app/agentstate/usage.go`, `internal/app/agentstate/usage_test.go`, `internal/app/agentruntime/usage.go`, `internal/app/agentruntime/usage_test.go` — **new**, validated source-free accounting and provider receipt projection.
- `internal/app/agentstate/run.go`, `internal/app/agentruntime/budget.go`, `internal/app/agentruntime/persistence.go`, `internal/app/agentruntime/policy.go`, `internal/app/agentruntime/provider.go`, `internal/app/agentruntime/gemini.go`, `internal/app/agentruntime/model.go`, `internal/app/agentruntime/policy_test.go`, `internal/app/agentruntime/gemini_test.go`.
- `internal/llm/client.go`, `internal/llm/generation.go`, `internal/llm/generation_test.go`, `internal/tui/agent_run_view.go`, `internal/tui/agent_run_test.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/agent_runs_test.go`, `CONFIG.md`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 26B; durable budget checkpoints and the existing native/compatible provider adapters.

**Implementation rules**
- Keep enforced token charges cumulative across failed attempts, retries and summarization. Separately record reported input/output/cache tokens, unknown usage, failed calls and conservative estimates, grouped by hashes of configured provider/model identities; never persist endpoints, keys or provider diagnostics in accounting.
- Preserve valid usage received before a failed native completion. Missing usage must remain unknown; a successful response without usage must not become a failed call. Validate cached tokens as a subset of reported input, retain exact integer counts and charge once under the existing budget mutex.
- Version the accounting projection, read historical records without inventing measured usage, validate monotonic checkpoints, and show reported versus estimated consumption in the existing Usage panel. Remote delegation's separately reported or unknown usage remains explicitly remote and must not be counted as local provider measurements.

**Verification command**
`go test -race ./internal/llm ./internal/app/agentstate ./internal/app/agentruntime && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: distinguish reported agent usage from budget estimates`

## Task 27B — Add bounded local traces and versioned cost estimates

- [x] Complete

**Target files**
- `internal/app/agentruntime/telemetry.go`, `internal/app/agentruntime/telemetry_test.go`, `internal/app/agentstate/traces.go`, `internal/app/agentstate/traces_test.go`, `internal/app/agent_usage.go`, `internal/app/agent_usage_test.go`, `internal/app/agent_usage_models.go`, `internal/config/agent_telemetry.go`, `internal/config/agent_telemetry_test.go`, `internal/tui/agent_telemetry_view.go` — **new**.
- `internal/app/agentstate/store.go`, `internal/app/agentstate/history.go`, `internal/app/agentstate/run.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/persistence.go`, `internal/app/agentruntime/events.go`, `internal/app/agentruntime/budget.go`, `internal/app/agentruntime/graph_runtime.go`, `internal/app/agentruntime/change_graph.go`, `internal/app/agentruntime/analysis_graph.go`, `internal/app/agentruntime/review_graph.go`, `internal/app/agentruntime/repair_graph.go`, `internal/app/agentruntime/knowledge_graph.go`, `internal/app/agentruntime/tools.go`, `internal/app/agentruntime/policy.go`, `internal/app/agentruntime/compaction.go`, `internal/app/agentruntime/a2a_client.go`.
- `internal/app/agent_limits.go`, `internal/app/service.go`, `internal/app/agent_settings_runtime.go`, `internal/app/agent_runs.go`, `internal/app/agent_run_types.go`, `internal/config/agent_runtime.go`, `internal/config/config.go`, `internal/api/handlers/agent_run_handler_test.go`, `internal/app/agentruntime/policy_test.go`.
- `internal/app/agentstate/store_test.go`, `internal/app/agentstate/usage_test.go`, `internal/app/agentstate/input_test.go`, `internal/app/agentstate/artifacts_test.go`, `internal/app/agentstate/history_test.go`, `internal/app/agent_events_test.go`, `internal/app/agent_runs_test.go` — update existing journal fixtures for atomic trace admission/completion.
- `internal/app/service_test.go` — synchronize provider dispatch before simulating deadline expiry in the existing overview-inventory regression; the previous 50 ms setup deadline also fails against the unchanged baseline.
- `internal/tui/agent_run_view.go`, `internal/tui/agent_run_test.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/agent_runs_test.go`, `CONFIG.md`, `config.example.yaml`, `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 27A; [ADK Go tracing](https://adk.dev/observability/traces/) and the pinned Go telemetry implementation. SDK telemetry setup reads ambient exporter/resource configuration, so use an explicitly projected source-free export boundary.

**Implementation rules**
- Correlate bounded run, node, tool, provider, retry, summary, delegation and check spans with opaque identities, fixed status categories and elapsed times. Retain failures and interrupted states; report truncated history without erasing cumulative accounting.
- Local source-free summaries remain passive, versioned and compatible with legacy journals. Keep fixed span kinds/status categories, opaque IDs, hashed operation identities and bounded timing; never retain raw SDK payloads. Trace retention must not erase cumulative accounting. Preserve incomplete/interrupted observations and fail explicitly on journal errors.
- Co-commit invocation/provider trace transitions with their existing run/budget checkpoints to avoid extra journal round trips on deadline-sensitive provider dispatch. A failed atomic write must expose neither half of the transition.
- Monetary estimates require a versioned user-configured price table and matching model hashes. Missing measurements/rates/cache knowledge keep total cost unknown; label any known subtotal and price version. Use existing Usage/Timeline panels, with small-terminal and all-palette verification.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app/agentstate ./internal/app ./internal/config ./internal/api/handlers ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add local agent traces and versioned cost estimates`

## Task 27C — Export a confirmed source-free telemetry snapshot

- [x] Complete

**Target files**
- `internal/app/agentruntime/telemetry_export.go`, `internal/app/agentruntime/telemetry_export_test.go`, `internal/app/agent_usage_export.go`, `internal/app/agent_usage_export_test.go`, `internal/tui/agent_telemetry_actions.go` — **new**.
- `internal/app/agent_usage.go`, `internal/app/agent_usage_models.go`, `internal/config/agent_telemetry.go`, `internal/config/agent_telemetry_test.go`, `internal/config/config.go`, `internal/api/handlers/agent_run_handler.go`, `internal/api/handlers/agent_run_handler_test.go`, `cmd/daemon/main.go`, `cmd/daemon/main_test.go`.
- `internal/tui/agent_run_actions.go`, `internal/tui/agent_telemetry_view.go`, `internal/tui/agent_run_test.go`, `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/agent_runs_test.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `CONFIG.md`, `config.example.yaml`, `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 27B; [OTLP HTTP/JSON specification](https://opentelemetry.io/docs/specs/otlp/). Keep the SDK's ambient exporter/resource setup disabled.

**Implementation rules**
- Optional bounded OTLP export requires a configured exact destination and explicit confirmation of project/revision, run, destination fingerprint and source-free snapshot hash. Recheck these identities immediately before sending; browsing must never export.
- Whitelist projected OTLP fields, reject redirects and URL credentials/query strings, bound request/response sizes and transport duration, propagate cancellation, and report partial/failed/unknown delivery honestly. Never infer destinations, resources or credentials from ambient OTEL settings; no automatic retries.
- The TUI confirmation discloses destination, checkout/run and the exact retained snapshot, including incomplete/truncated history. Reuse owned async actions and reject stale or late responses without granting source Apply authority. Use only fake loopback collectors in tests.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config ./internal/api/handlers ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: export confirmed source-free agent telemetry`

## Task 28 — Add task and tool-trajectory evaluation

- [x] Complete

**Target files**
- `cmd/agent-eval/main.go`, `cmd/agent-eval/main_test.go`, `internal/agenteval/runner.go`, `internal/agenteval/runner_test.go`, `internal/agenteval/scoring.go`, `internal/agenteval/fixtures_test.go`, `internal/agenteval/testdata/tasks.json` — **new**.
- `internal/app/agent_evaluation.go`, `internal/app/agent_evaluation_test.go` — **new**, bounded evaluation entry point.
- `scripts/quality.sh`, `scripts/tests/test_quality.py`, `README.md`, `internal/AGENTS.md`.

**Inputs / dependencies**
- Task 27C; existing engineering-insight evaluation conventions. [ADK evaluation documentation](https://adk.dev/evaluate/) is Python-specific; preserve a native Go harness.

**Implementation rules**
- Evaluate fixed task fixtures, tool-call correctness, scope compliance, independent regression tests, unsupported claims, cancellation, tokens, latency, and repair counts. Version corpus, prompt, model, runtime, and scoring identities.
- Provide fake-provider/replay mode as the deterministic default. Live evaluation requires an explicit command, source/destination disclosure, and request/token budget; never run it in ordinary tests or CI.
- Keep receipts source-free and failures/incomplete runs visible. Add the new CLI to the maintained deadcode roots, without excluding production packages from checks.

**Verification command**
`go test -race ./internal/agenteval ./cmd/agent-eval ./internal/app && python3 -m unittest discover -s scripts/tests -p 'test_*.py' && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add native agent task and trajectory evaluation`

## Task 29A — Persist bounded mission plans and explicit review

- [x] Complete

**Target files**
- `internal/app/agent_mission.go`, `internal/app/agent_mission_test.go`, `internal/app/agentstate/missions.go`, `internal/app/agentstate/missions_test.go` — **new**.
- `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go` — **new**.
- `internal/tui/client/missions_test.go` — **new**, wire contract tests.
- `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 28; captured source/instruction validation, private atomic catalogs, strict scoped handlers, and guarded client routes.

**Implementation rules**
- Provide real scoped API operations to create, list, inspect, revise and review a private mission plan. Saving and editing never dispatch providers, execute checks, create worktrees or write project source.
- Bound missions to 16 children and 64 captured paths. Each child has an approach, acceptance criteria, dependencies, 1–8 write paths, at most eight distinct read paths and explicit limits within a cumulative mission budget. Validate integration order, missing/cyclic dependencies, and concurrent write/read conflicts; an absent read path needs an ancestor that creates it.
- Capture project/revision, present/absent source identities, context policy and instruction identities. Recheck these at durable mutation boundaries. Reject invalid paths, excluded/symlinked source, stale revisions and changed source. Keep stale historical plans inspectable without making them executable.
- Bind child review and final mission approval to the exact plan hash and catalog revision. Revising the plan clears review/approval. Approval is planning authority only; no children launch and no Apply/Undo authority is granted.
- Use bounded, private, versioned atomic storage. Preserve corrupt files, reject unknown schemas, handle concurrent stale edits and surface write failures. Cover API/client success and rejection behavior with fakes.

**Verification command**
`go test -race ./internal/app ./internal/app/agentstate ./internal/api/handlers ./cmd/daemon ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: persist scoped mission plans and explicit review`

## Task 29B — Decompose missions through the native ADK planner

- [x] Complete

**Target files**
- `internal/app/agentruntime/mission_planner.go`, `internal/app/agentruntime/mission_planner_test.go`, `internal/app/agent_mission_planning.go`, `internal/app/agent_mission_planning_test.go` — **new**.
- `internal/app/agent_mission.go`, `internal/app/agent_mission_test.go`, `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go`, `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/missions_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 29A; bounded native investigation tools, current Function profile, remote consent, shared budgets and captured source/instruction checks.

**Implementation rules**
- Add an explicit, cancelable planning operation that decomposes the supplied goal only within the captured path inventory and mission budget. Use native ADK execution with read-only repository tools where supported; preserve the existing bounded completion path for completion-only profiles.
- Keep model suggestions unapproved. Strictly validate the entire returned DAG, scope, dependencies, acceptance criteria and budget before saving a draft; reuse task 29A's domain validator rather than trusting schema-constrained output alone.
- Revalidate project, sources, instructions and settings at dispatch and publication. Reject late, stale, incomplete or malformed output without publishing a usable mission. Cancellation must stop provider work and retain existing drafts. Never start a child or modify source.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/api/handlers ./cmd/daemon ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: decompose bounded missions with the native ADK planner`

## Task 29C — Review and edit missions in the Work workspace

- [x] Complete

**Target files**
- `internal/tui/mission_actions.go`, `internal/tui/mission_commands.go`, `internal/tui/mission_view.go`, `internal/tui/mission_test.go`, `internal/tui/client/missions.go` — **new**.
- `internal/tui/plan.go`, `internal/tui/focused_pages.go`, `internal/tui/workspace_actions.go`, `internal/tui/plan_actions.go`, `internal/tui/plan_test.go`, `internal/tui/state.go`, `internal/tui/terminal_loop.go`, `internal/tui/daemon_actions.go`, `internal/tui/sse_events.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/client.go`, `internal/tui/client/missions_test.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 29B; existing Work plan/detail views, owned async actions, guarded checkout selection and scrollable confirmation patterns.

**Implementation rules**
- Add explicit mission commands and a restrained plan/detail view within Work. Show goal, current/stored scope, dependency/integration order, child write/read paths, criteria, budget, review state and next action using existing palettes and scrolling.
- Opening, browsing and local editing remain passive. Model decomposition is a separate confirmed async action with explicit source/destination disclosure and cancellation. Capture exact draft, checkout and project identities; reject replaced or late results and prevent duplicate submission.
- Save revisions through the scoped contract, clear approval when content changes, review each concrete child, and confirm the exact mission hash before final approval. This step has no child-execution control; execution arrives in task 30.
- Keep ordinary single-task planning intact. Verify all four palettes and small terminal sizes, sanitization, keyboard cancellation, stale edits, client request ownership and changed-project cleanup.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: review bounded missions in the Work workspace`

## Task 30A — Prepare durable mission execution records and fixed allocations

- [x] Complete

**Target files**
- `internal/app/agentstate/mission_executions.go`, `internal/app/agentstate/mission_executions_test.go`, `internal/app/agent_mission_run.go`, `internal/app/agent_mission_run_test.go` — **new**.
- `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go`, `cmd/daemon/main.go`.
- `internal/tui/client/agent_models.go`, `internal/tui/client/client.go`, `internal/tui/client/missions_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 29C; approved mission plans, guarded Git identity reads and private atomic storage.

**Implementation rules**
- Add explicit prepare, read and cancel operations for a mission execution receipt. Preparation freezes the fully reviewed plan, exact clean Git base and each child's approved budget allocation. It performs no provider dispatch, project-code execution, worktree creation or source mutation.
- Keep one durable execution per mission identity so retries, plan edits, cancellation and restart cannot reset allocations. Preserve the frozen plan and canceled evidence. Later execution uses this receipt, never a mutable plan from the catalog.
- Recheck current project, plan/catalog identity, source hashes, instructions, policy, settings and Git base at preparation publication. Use an independent execution revision for exact control requests. Read-only inspection reports changed or unavailable bindings; cancellation remains possible after source/settings changes.
- Reject corrupt, foreign, oversized or unknown-schema records without replacing evidence. Cover concurrent preparations, replay, cancellation at publication, storage failure, stale base and passive reads in store/app/HTTP/client tests.

**Verification command**
`go test -race ./internal/app ./internal/app/agentstate ./internal/api/handlers ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: prepare durable mission execution allocations`

## Task 30B — Schedule bounded mission children through native ADK workflows

- [x] Complete

**Target files**
- `internal/app/agent_mission_scheduler.go`, `internal/app/agent_mission_scheduler_test.go`, `internal/app/agent_mission_child.go`, `internal/app/agent_mission_child_test.go`, `internal/app/agentruntime/mission_graph.go`, `internal/app/agentruntime/mission_graph_test.go` — **new**.
- `internal/app/agent_mission_run.go`, `internal/app/agent_mission_run_test.go`, `internal/app/agent_mission.go`, `internal/app/agentstate/mission_executions.go`, `internal/app/agentstate/mission_executions_test.go`.
- `internal/app/service.go`, `internal/app/project_lifecycle.go`, `internal/app/worktree_service.go`, `internal/app/git_worktrees.go`, `internal/app/git_mutation.go`, `internal/app/change_pipeline.go`, `internal/app/change_pipeline_control.go`, `internal/app/change_pipeline_types.go`, `internal/app/agent_limits.go`, `internal/app/agent_settings_runtime.go`, `internal/app/agent_runs.go`.
- `internal/app/change_types.go`, `internal/app/change_session.go`, `internal/app/change_store.go`, `internal/app/change_context.go`, `internal/app/agent_context.go`, `internal/app/change_pipeline_stages.go`, `internal/app/change_checks.go`, `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go`, `internal/app/agentruntime/events.go`, `internal/app/agentruntime/events_test.go`, `internal/tui/client/models.go`.
- `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/missions_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 30A; existing checked-change graph, workflow ownership and guarded worktree creation.

**Implementation rules**
- Admit an explicit start/advance against the execution receipt with fresh provider and project execution consent. Use native ADK to run a bounded ready wave; parallelize only disjoint children whose dependencies are explicitly integrated. Record writer ownership and pending worktree effects before dispatch.
- Give each child a private workflow admission constrained to its fixed allocation, approved write/read scope and captured settings. The sum of allocations bounds mission calls, tokens, tools and repairs; reserve an allocation once and never silently recycle it after failure, cancellation or restart. Bound aggregate active execution time and concurrent calls across waves; human review waiting does not consume execution time.
- Persist a mission origin on each child proposal. Ordinary model-driven retries cannot reset its reserved budget; only its privately admitted workflow can dispatch. Compose context from its explicit write/read paths and instructions without implicit sibling declarations. Register the mission graph schema and source-free progress in the existing ADK journal.
- Each child creates a scoped worktree and checked proposal through existing workflow owners. Return awaiting human review without Apply or merge authority. Changed sources/settings/base fail before the affected dispatch; editing a catalog plan never retargets an active job.
- Pause/cancel cascades to owned children and processes. Restart reconciles recorded worktrees and completed evidence without automatic provider requests; uncertain attempts remain interrupted and conservatively charged. Surface blocked, failed and rejected children honestly.

**Verification command**
`go test -race ./internal/app ./internal/app/agentruntime ./internal/app/agentstate ./internal/api/handlers && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: schedule bounded mission children with native ADK`

## Task 30C1 — Integrate checked mission children with durable Git evidence

- [x] Complete

**Target files**
- `internal/app/agent_mission_integration.go`, `internal/app/agent_mission_integration_test.go`, `internal/app/agentstate/mission_integrations.go`, `internal/app/agentstate/mission_integrations_test.go` — **new**.
- `internal/app/agent_mission_run.go`, `internal/app/agent_mission_run_test.go`, `internal/app/agent_mission_scheduler.go`, `internal/app/agent_mission_scheduler_test.go`, `internal/app/agent_mission_child.go`, `internal/app/agent_mission_child_test.go`, `internal/app/agent_mission.go`.
- `internal/app/agentstate/mission_executions.go`, `internal/app/agentstate/mission_executions_test.go`, `internal/app/git_merge.go`, `internal/app/git_worktree_cleanup.go`, `internal/app/project_lifecycle.go`, `internal/app/agent_runs.go`.
- `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/missions.go`, `internal/tui/client/missions_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.
- `Makefile` — bound the expanded application race suite at fifteen minutes; retain every existing check and operation deadline.

**Inputs / dependencies**
- Task 30B; existing guarded Apply/commit/merge, registered worktree identity checks, and protected private artifact storage.

**Implementation rules**
- Add explicit confirmed integration and reconciliation against the exact execution revision, child, proposal, committed worktree and destination Git identity. Preserve ordinary human diff Review and Apply/commit. Require the approved integration order, completed dependencies and the current recorded base; a child checked against an older base stays blocked for explicit revalidation.
- Persist merge intent before Git effects. Archive the complete checked proposal and provenance as protected private evidence before cleanup, and retain its exact artifact reference in the execution receipt. Ordinary Git routes must not bypass mission integration admission or erase owned evidence.
- Preserve the original approved mission and initial base. Record a separate current integration checkpoint with the resulting base, project revision, source hashes and instruction bindings. Only the approved child changes may advance that checkpoint; subsequent children use those observed bindings without changing their fixed allocations or the original plan hash.
- Publish the verified resulting checkpoint before releasing dependencies or removing worktrees. Reconcile interrupted merges and cleanup from exact commit ancestry, tree/source evidence and retained intent; missing worktrees alone never prove success. Report conflicts, unavailable evidence, partial persistence and cleanup failures honestly. Never retry uncertain Git effects automatically.
- Keep cleanup cancelable across the integration's own post-merge reindex. Persist an already observed merge after cancellation, retain its checkout for explicit later cleanup, and preserve normal cancellation of mission waves on project reindex.
- Version the execution schema deliberately and retain older prepared, active and completed child records. Cover serial dependency execution through explicit integration, out-of-order/foreign/stale requests, mutation races, persistence failures, restart and retained evidence after cleanup.

**Verification command**
`go test -race -timeout=15m ./internal/app ./internal/app/agentstate ./internal/api/handlers ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate mission children with durable Git receipts`

## Task 30C2 — Revalidate retained mission proposals against the current base

- [x] Complete

**Target files**
- `internal/app/agent_mission_revalidation.go`, `internal/app/agent_mission_revalidation_test.go` — **new**.
- `internal/app/agentstate/mission_revalidations.go`, `internal/app/agentstate/mission_revalidations_test.go` — **new**, bounded candidate lineage and reservation owner.
- `internal/app/agent_mission_integration.go`, `internal/app/agent_mission_integration_test.go`, `internal/app/agent_mission_run.go`, `internal/app/agent_mission_child.go`, `internal/app/agent_mission_child_test.go`, `internal/app/agent_mission_scheduler.go`, `internal/app/agent_confirmation.go`.
- `internal/app/agentstate/mission_integrations.go`, `internal/app/agentstate/mission_integrations_test.go`, `internal/app/agentstate/mission_executions.go`, `internal/app/agentstate/mission_executions_test.go`.
- `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go`, `internal/tui/client/agent_models.go`, `internal/tui/client/missions.go`, `internal/tui/client/missions_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 30C1; parallel children may retain useful proposals checked against an earlier recorded mission base.

**Implementation rules**
- Provide explicit bounded revalidation of an owned retained proposal against the current verified mission checkpoint. Capture the previous proposal/commit and new base before worktree effects; preserve old branches and protected proposal evidence.
- Create a fresh owned review candidate through existing change-session and worktree owners, limited to the approved child write/read paths. Carry forward only proposal content and provenance. Invalidate prior checks, model-review claims and human approval wherever the base or proposal identity changes; run fresh checks only with scoped execution consent.
- Revalidation never invokes a provider or resets the child's fixed model/tool/token/repair allocation. The candidate retains its mission origin and requires fresh per-file human review and explicit Apply/commit before integration. Show original model history as historical evidence, not current verification.
- Bind the new session origin to its exact revalidation ID. Its final explicit human Review approves the fresh captured plan only after every changed file is reviewed and current checks pass; opening or checking a candidate never grants plan or Apply approval.
- Bound candidate generations and persist their lineage and effect ownership. Cancellation, failed checks, missing worktrees or restart retain recoverable state without automatic recreation, execution or cleanup. Unrelated external changes to the integration checkpoint require a new reviewed mission instead of silent adoption.
- Charge each attempt to the existing elapsed allowance, preserve historical candidates in ownership guards, and cancel revalidation effects on reindex while preserving synchronous integration observation. Upgrade older execution catalogs passively; never attach current model-review authority to carried content.
- Cover two disjoint children generated in parallel and integrated sequentially after current-base revalidation, stale/replayed candidate requests, failed checks, cancellation and recovery without provider dispatch or source Apply.

**Verification command**
`go test -race -timeout=15m ./internal/app ./internal/app/agentstate ./internal/api/handlers ./internal/tui/client && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: revalidate mission proposals against the current base`

## Task 30C3 — Review and integrate mission executions in the TUI

- [x] Complete

**Target files**
- `internal/tui/mission_execution.go`, `internal/tui/mission_execution_test.go` — **new**.
- `internal/tui/mission_execution_actions.go`, `internal/tui/mission_execution_view.go`, `internal/tui/mission_execution_stream.go`, `internal/tui/mission_execution_stream_test.go` — **new**, transport admission, detail rendering and owned passive progress.
- `internal/tui/hub_agents.go`, `internal/tui/agent_run_actions.go`, `internal/tui/worktree_pending.go`, `internal/tui/worktree_lifecycle_test.go`, `internal/tui/colon_commands_actions.go`, `internal/tui/mission_actions.go`, `internal/tui/mission_commands.go`, `internal/tui/mission_view.go`, `internal/tui/mission_test.go`, `internal/tui/terminal_loop.go`, `internal/tui/sse_events.go`, `internal/tui/keybinds_help.go`, `internal/tui/command_menu.go`.
- `internal/tui/client/mission_executions.go`, `internal/tui/client/mission_executions_test.go` — **new**, typed execution controls and checkpoint/candidate receipt validation.
- `internal/tui/client/agent_models.go`, `internal/tui/client/missions.go`, `internal/tui/client/missions_test.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 30C2; all execution, revalidation, integration and recovery controls have verified backend behavior.

**Implementation rules**
- Expose preparation, start/advance, pause/cancel, per-child progress, current-base revalidation and explicit integration in the existing mission detail surface. Show concrete scope, allocations, destinations and confirmations. Keep navigation and detail reads passive; reject late results after scope, plan, execution, candidate or provider identity changes.
- Open an exact owned child proposal through ordinary diff review and explicit Apply/commit. Route its merge through mission integration admission in the approved order. Preserve the mission selection across resulting project revisions without carrying stale consent or approval.
- Preserve the captured owner when opening its nested child. Apply an integration index only to that owner; a task-scoped receipt must never replace the top-level project. After canceled transport, explicit refresh may recover the same owner's current revision through passive reads before inspecting the durable execution.
- Disclose the actual configured creation, testing and review profiles and check commands. Recheck settings, source and execution identities immediately before dispatch. Show allocated model/tool/token ceilings separately from measured usage; never fabricate remaining usage.
- Show pending effects, failed/rejected/interrupted children, retained proposal artifacts, conflicts and cleanup failures with the applicable recovery action. Provider retries cannot reset mission allocations. Refresh progress from local events without dispatching work or replacing local edits.
- Verify complete keyboard journeys, selected worktree ownership, cancellation and restart recovery at compact/wide dimensions and all palettes. Keep dependencies, remaining allocations, current base and recovery actions readable without increasing dashboard density.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: review and integrate scoped missions in the TUI`

## Task 31A — Import explicit attachments and request multimodal advice

- [x] Complete

**Target files**
- `internal/llm/attachments.go`, `internal/llm/attachments_test.go` — **new**, bounded media types and content validation.
- `internal/llm/capabilities.go`, `internal/llm/capabilities_test.go` — advertise only conformed native multimodal models.
- `internal/app/agentruntime/attachments.go`, `internal/app/agentruntime/attachments_test.go` — **new**, admitted native multimodal completion.
- `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/gemini.go`, `internal/app/agentruntime/persistence.go` — immutable media capture, strict native parts, and request identity without copying binary data into metadata.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go` — register and validate the explicit `multimodal-v1` journal schema while preserving rejection of unknown schemas.
- `internal/app/agent_attachments.go`, `internal/app/agent_attachments_test.go`, `internal/app/agent_attachment_advice.go`, `internal/app/agent_attachment_advice_test.go` — **new**, local import, version/hash selection, destination disclosure, and private advisory artifacts.
- `internal/api/handlers/agent_attachment_handler.go`, `internal/api/handlers/agent_attachment_handler_test.go` — **new**, strict loopback endpoints.
- `cmd/daemon/main.go`, `internal/tui/client/attachment_models.go`, `docs/api-contract.md`, `docs/openapi.yaml`, `CONFIG.md` — routes, wire DTOs (**new** client model file), and supported behavior.

**Inputs / dependencies**
- Task 30C3; existing private artifact inventory/export/deletion, Function profile, run admission, and integration lifecycle cancellation.
- The pinned native SDK accepts inline media parts. [Google artifact documentation](https://adk.dev/artifacts/) and the model capability contract distinguish inline bytes from provider-fetched file URLs.

**Implementation rules**
- Import explicitly supplied PNG/JPEG images, PDF documents, and WAV/PCM audio into immutable private artifacts. Validate content structure, canonical MIME, display name, size (at most 512 KiB each), project/revision, and explicit local import confirmation. Keep the existing 1 MiB HTTP request bound; do not read arbitrary daemon paths or accept remote URLs.
- An explicit advisory request selects one to four imported artifact versions by ID/version/hash, at most 1 MiB combined, plus a bounded question. It binds the current settings identity, Function model and destination, and source-sharing/remote consent. Reject foreign, stale, deleted, altered, unsupported or oversized selections before provider work.
- Run one native ADK completion with captured inline bytes and the existing cumulative budget. No implicit memory, skills, provider caching, compaction, retries, tools, source changes or extra uploads. Retain the source-free media manifest in the private invocation identity, while the artifact store owns payloads.
- Recheck scope, policy, settings and artifact identity at dispatch and publication; cancel on project replacement/reindex/shutdown. Save the answer as a private advisory artifact with provenance. Existing passive library inspection/export/deletion remains the recovery surface; model advice is never verified project evidence or a change proposal.
- Exercise real ADK/native fake-HTTP requests, binary privacy, unsupported profiles, malformed media, stale ownership, changed settings, cancellation, persistence failures and late results. TUI actions are completed in Task 31C; this commit has functional HTTP callers and no dormant runtime helpers.

**Verification command**
`go test -race -timeout=15m ./internal/llm ./internal/app/agentruntime ./internal/app/agentstate ./internal/app ./internal/api/handlers ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add explicit multimodal attachment advice`

## Task 31B — Own bounded ADK live session lifecycles

- [x] Complete

**Target files**
- `internal/app/agentruntime/live.go`, `internal/app/agentruntime/live_transport.go`, `internal/app/agentruntime/live_input.go`, `internal/app/agentruntime/live_events.go`, `internal/app/agentruntime/live_test.go`, `internal/app/agentruntime/live_transport_test.go`, `internal/app/agentruntime/live_events_test.go` — **new**, ADK runner/session bridge, explicit input validation, private output recording and bounded provider transport.
- `internal/app/agent_live.go`, `internal/app/agent_live_control.go`, `internal/app/agent_live_lifecycle.go`, `internal/app/agent_live_test.go`, `internal/app/agentstate/live.go`, `internal/app/agentstate/live_test.go` — **new**, durable receipts, owned controller and restart interpretation.
- `internal/app/agentstate/run.go`, `internal/app/agentstate/store_test.go` — register the explicit `live-v1` invocation schema and retain rejection of unknown versions.
- `internal/config/config.go` — admit only the explicit `agent_runtime.live_model` key through strict YAML validation.
- `internal/app/service.go`, `internal/app/project_lifecycle.go`, `internal/app/agent_integrations.go`, `internal/app/agent_settings.go`, `internal/config/agent_runtime.go`, `internal/config/agent_runtime_test.go`, `internal/llm/capabilities.go`, `internal/llm/capabilities_test.go`.
- `internal/api/handlers/agent_live_handler.go`, `internal/api/handlers/agent_live_handler_test.go`, `internal/tui/client/live_models.go` — **new**, strict controls and wire DTOs.
- `cmd/daemon/main.go`, `go.mod`, `go.sum`, `docs/api-contract.md`, `docs/openapi.yaml`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 31A; exact imported media references and shared policy/budget primitives.
- SDK inspection: ADK v2.6.0 `Runner.RunLive` supports custom `agent.LiveSession` agents. GenAI v1.74.0's default live connector uses a process-global WebSocket dialer, so use an application-owned bounded transport behind the ADK runner rather than replacing global transport state or weakening cancellation.
- [Gemini 3.8 Live](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-live) and the [WebSocket protocol](https://ai.google.dev/gemini-api/docs/live-api/get-started-websocket) define explicit text/image/PCM input, audio output with transcription, and interruption behavior. Account entitlement remains a runtime result.

**Implementation rules**
- Add an optional configured live model using the Function profile's explicit native destination and credentials; leave it disabled by default and preserve the ordinary Function model. Advertise only tested model/backend combinations; reject unsupported credentials, URLs and features before connecting.
- Use the actual ADK live runner with private sessions/artifacts. Bind open/send/interrupt/reconnect/close to exact checkout, settings, model/destination, session identity and control revision. Repeat admission at actual connect/send/tool boundaries. Live advisory sessions grant no source mutation or project-code execution; reject ungranted tool requests.
- Bound lifetime, connections, turns, incoming/outgoing bytes and tokens within the admitted cumulative budget. Charge uncertain consumption conservatively and label reported usage separately. Explicit reconnect retains elapsed/cumulative limits, makes no automatic replay or upload, and requires fresh consent. Provider errors, go-away and dropped transport retain distinct states.
- Retain bounded private transcripts/audio artifacts and source-free receipts; never log provider frames, credentials, resume handles or transcripts. No microphone/camera capture, terminal playback or provider-fetch URLs. Reject unsupported PDF/WAV-as-realtime input instead of silently converting it.
- Give every connection and goroutine a controller; cancellation, project replacement, worktree disposal, shutdown, budget exhaustion and persistence failure close and join it. Restart inspection reports interrupted sessions without reconnecting or replenishing budgets. Expose passive list/detail and explicit controls through the loopback API.
- Start with the conformed Gemini API `gemini-3.8-live` model and explicit API-key authentication. Bound a session to five minutes and two connections; each connection has at most eight input/control turns, 128 KiB outbound and 512 KiB inbound payload, with smaller effective bounds when the shared budget requires them. Reserve conservative byte-based token charges before connecting, retain partial reported usage separately, and never treat a missing final usage receipt as measured zero. An empty live-model setting preserves existing configuration identities.
- Test the runner over fake WebSockets, admission at every send, exact headers/route, interruption, partial/malformed output, blocked I/O cancellation, reconnect accounting, restart, stale commands and storage failure. Promote only the already pinned WebSocket dependency from indirect to direct if needed.

**Verification command**
`go test -race -timeout=15m ./internal/app/agentruntime ./internal/app/agentstate ./internal/app ./internal/config ./internal/llm ./internal/api/handlers ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add bounded ADK live sessions with explicit controls`

## Task 31C — Present attachments and live sessions in the TUI

- [x] Complete

**Target files**
- `internal/tui/agent_media.go`, `internal/tui/agent_media_actions.go`, `internal/tui/agent_media_view.go`, `internal/tui/agent_media_test.go`, `internal/tui/agent_media_stream.go`, `internal/tui/agent_media_stream_test.go` — **new**, focused media/session surface and owned passive updates.
- `internal/tui/client/attachments.go`, `internal/tui/client/attachments_test.go`, `internal/tui/client/live.go`, `internal/tui/client/live_test.go` — **new**, typed controls and receipt validation.
- `internal/tui/focused_pages_test.go`, `internal/tui/state_test.go` — include Media in the complete passive Work submenu keyboard cycle.
- `internal/tui/client/client.go`, `internal/tui/client/attachment_models.go`, `internal/tui/client/live_models.go`, `internal/tui/client/agent_models.go`, `internal/tui/state.go`, `internal/tui/keys.go`, `internal/tui/hub_overview.go`, `internal/tui/daemon_actions.go`, `internal/tui/terminal_loop.go`, `internal/tui/colon_commands_actions.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/agent_library.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 31B; backend attachment and live contracts pass fake-provider conformance.

**Implementation rules**
- Keep the five workspaces. Add a focused Work detail surface for selected attachments, advisory answers, live connection state, transcript disclosure, usage and recovery; keep the dashboard restrained and existing library ownership intact.
- Read a bounded, explicitly selected local attachment only after its path is supplied. Reject symlinks, non-regular files and escapes from the selected checkout. Show exact name/MIME/size/hash before local import, then select immutable imported versions for a separately confirmed provider request.
- Capture checkout, current settings, provider origin/model, selected artifact versions and question in confirmations. Revalidate before dispatch and reject late results after scope/selection/settings changes. Main and task worktree responses remain isolated.
- Expose open, send, interrupt, close and explicit reconnect with clear pending/disconnected/failed/interrupted/stale states. Navigation, transcript disclosure and local refresh never connect or send. Leaving the view stops passive watchers without silently closing a separately owned session; make active-session state and explicit close discoverable.
- Display audio as private artifact metadata/export, with no automatic playback or capture. Unsupported profiles and missing live configuration provide actionable text and no inert action. Preserve drafts and acknowledged progress through transport errors without automatic resend.
- Verify complete keyboard journeys, compact/wide layouts, all four palettes, ANSI handling, stale confirmations, privacy, cancellation and passive navigation with real client/handler fixtures.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: present explicit attachments and live sessions in the TUI`

## Task 32A — Own opt-in ambient analysis with durable grants and budgets

- [ ] Complete

**Target files**
- `internal/config/agent_triggers.go`, `internal/config/agent_triggers_test.go` — **new**, bounded polling, debounce, quiet, grant and window ceilings.
- `internal/app/agent_triggers.go`, `internal/app/agent_trigger_lifecycle.go`, `internal/app/agent_trigger_analysis.go`, `internal/app/agent_triggers_test.go`, `internal/app/agent_trigger_lifecycle_test.go` — **new**, scoped controls, owned watcher and admitted semantic analysis.
- `internal/app/agentstate/triggers.go`, `internal/app/agentstate/triggers_test.go` — **new**, private versioned trigger state and conservative window reservations.
- `internal/api/handlers/agent_triggers_handler.go`, `internal/api/handlers/agent_triggers_handler_test.go`, `internal/tui/client/trigger_models.go` — **new**, passive availability and explicit control contracts.
- `internal/config/agent_runtime.go`, `internal/config/config.go`, `internal/app/service.go`, `internal/app/project_lifecycle.go`, `internal/app/change_pipeline_control.go`, `internal/app/agent_settings.go`, `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `CONFIG.md`, `config.example.yaml`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 31C; existing ADK file-analysis admission, context authority, shared budgets, private stores and lifecycle cancellation. The main checkout is the ambient scope; task-worktree requests must be rejected explicitly.

**Implementation rules**
- Disabled is the default. Explicit watch configuration selects file-change or interval mode and one to eight eligible indexed paths. Capture bounded current source facts through project containment/context policy, debounce changes, deduplicate snapshots and enforce a quiet period between attempts. Detection alone only records a suggestion.
- Automatic semantic analysis requires a separate expiring grant naming the project, selected paths and filtered project context, current semantic model/destination (the Bug profile unless an explicit analysis selection applies), configuration, settings and policy, plus fixed run/window limits. Use the existing `AnalyzeFile` ADK knowledge workflow; no separate completion path, project-code execution, automatic Apply/Merge, feature generation or Security AI review.
- Reserve each attempt's worst-case model/token/time allowance durably before dispatch, enforce the same allowance at the actual provider boundary, and retain charges after cancellation, errors or uncertain delivery. Preserve window consumption across settings changes and restarts. Recheck the current grant, destination, source and settings on dispatch; no implicit provider fallback.
- Own and join watchers/model calls on disable, revocation, project replacement and shutdown. Skip competing active work before updating a changed source index. Watches remain inactive after daemon restart or project replacement until an explicit watch/grant control. Every activation establishes a fresh baseline and future eligibility; it never replays missed ticks or unfinished provider work. Persistence failure stops automatic work and remains visible.
- Expose passive state with last attempt/result, next eligibility, reason and remaining window allocation. Configuration alone never starts a watcher or grants source sharing. Cover fake-provider dispatch, no-grant detection, debounce/deduplication, budget exhaustion, restart/uncertain receipts, stale guards, path safety, corrupt state, cancellation and write failures. TUI controls follow in Task 32B.

**Verification command**
`go test -race -timeout=15m ./internal/app ./internal/app/agentstate ./internal/config ./internal/api/handlers ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add opt-in bounded ambient analysis`

## Task 32B — Review and control ambient analysis in the TUI

- [ ] Complete

**Target files**
- `internal/tui/agent_triggers.go`, `internal/tui/agent_trigger_actions.go`, `internal/tui/agent_trigger_view.go`, `internal/tui/agent_triggers_test.go`, `internal/tui/client/triggers.go`, `internal/tui/client/triggers_test.go` — **new**, explicit watch/grant controls and passive status.
- `internal/tui/client/trigger_models.go`, `internal/tui/client/client.go`, `internal/tui/state.go`, `internal/tui/keys.go`, `internal/tui/hub_settings.go`, `internal/tui/dashboard.go`, `internal/tui/daemon_actions.go`, `internal/tui/terminal_loop.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/focused_pages_test.go`, `internal/tui/state_test.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 32A; real scoped HTTP controls and owned cancellation are verified.

**Implementation rules**
- Keep five workspaces. Provide a focused Settings view for disabled/watch-only/granted/stale/failed states, exact selected paths, model/destination, expiry, conservative window consumption, last result, next eligibility and reason.
- Watch setup and provider permission are separate explicit confirmations. Bind both to the current checkout, configuration and control revision; grant confirmation names filtered context sharing and fixed limits. Refresh, navigation, drafts and disclosures remain passive.
- Keep revoke and disable easy to find. Cancel pending local requests without claiming a lost receipt had no effect; refresh before retrying. Preserve drafts and healthy status on errors and reject late responses after scope or selection changes. Avoid repeated notifications for unchanged state.
- Exercise real client/handler controls, keyboard flows, all palettes, compact/wide layouts, stale confirmations, unknown outcomes and passive navigation.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add explicit ambient analysis controls to the TUI`

## Task 33 — Add optional cloud-backed agent services

- [ ] Complete

**Target files**
- `internal/app/agentruntime/cloud_services.go`, `internal/app/agentruntime/cloud_services_test.go`, `internal/app/agent_data_policy.go`, `internal/app/agent_data_policy_test.go` — **new**.
- `internal/config/integrations.go`, `internal/app/agent_settings.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/session.go`, `internal/app/agentruntime/artifacts.go`, `internal/app/agentruntime/memory.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 32B; explicit remote grants. Real Google Cloud project, region, identities, and storage resources are deployment inputs supplied by the operator; tests use fakes.

**Implementation rules**
- Support configured ADK Vertex session/memory and GCS artifact adapters where pinned SDK conformance passes. Keep local services the default and application mutation authority local.
- Bind data scope, principal, project, region, retention, and deletion semantics to the selected service. Enabling a backend never migrates or uploads existing histories automatically.
- Implement explicit validated export/migration with receipts and recoverable failures; reject unsupported checkpoint/confirmation combinations. Report remote deletion or availability failures honestly and redact diagnostics.

**Verification command**
`go test -race -timeout=15m ./internal/app/agentruntime ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add optional consented cloud agent services`

## Task 34 — Package an authenticated remote worker and developer inspector

- [ ] Complete

**Target files**
- `cmd/agent-worker/main.go`, `cmd/agent-worker/main_test.go`, `cmd/agent-inspect/main.go`, `cmd/agent-inspect/main_test.go`, `Dockerfile.agent-worker`, `docs/agent-deployment.md` — **new**.
- `internal/app/agent_remote_service.go`, `internal/app/agentruntime/a2a_server.go`, `scripts/quality.sh`, `scripts/tests/test_quality.py`, `Makefile`, `README.md`, `internal/AGENTS.md`.

**Inputs / dependencies**
- Task 33; guarded A2A service, source-free event records, and container build conventions.

**Implementation rules**
- Provide a worker entry point suitable for Cloud Run or other container hosts with mandatory authenticated project-scoped requests, bounded jobs, shutdown, and no writable access to a user's local checkout.
- Provide a local inspector for recorded graphs/events/artifacts. ADK development UI, if offered, runs only in an explicit isolated debug session with the same guarded host interface; it must not add unrestricted browser routes to the daemon.
- Add build/run documentation and offline container validation. Do not deploy infrastructure, enable billing, publish images, or open network access as an implementation side effect. Add both command roots to quality analysis.

**Verification command**
`go test ./cmd/agent-worker ./cmd/agent-inspect ./internal/app ./internal/app/agentruntime && python3 -m unittest discover -s scripts/tests -p 'test_*.py' && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: package guarded agent workers and local inspection`

## Task 35 — Finish compatibility migration and remove superseded execution paths

- [ ] Complete

**Target files**
- `internal/app/service.go`, `internal/app/model_runtime.go`, `internal/app/change_pipeline.go`, `internal/app/change_pipeline_control.go`, `internal/app/change_pipeline_resume.go`, `internal/app/change_pipeline_stages.go`, `internal/app/analysis_run.go`, `internal/app/analysis_stage_runner.go`, `internal/app/change_store.go`, `internal/app/analysis_run_store.go`.
- `internal/app/agent_migration.go`, `internal/app/agent_migration_test.go`, `internal/app/agentstate/migration.go`, `internal/app/agentstate/migration_test.go` — **new**.
- `internal/app/change_pipeline_recovery_test.go`, `internal/app/analysis_restart_test.go`, `internal/app/analysis_run_store_test.go`, `cmd/daemon/main.go`, `internal/tui/workflow_progress.go`, `internal/tui/domain/remediation.go`.

**Inputs / dependencies**
- Task 34; every model-backed application workflow has a tested ADK path.

**Implementation rules**
- Make ADK the single execution path; remove obsolete runner loops, unused wrappers, old stage-index assumptions, and transitional switches. Preserve domain guard and storage ownership.
- Migrate compatible historical records without erasing provenance; keep old completed history readable. Mark incompatible active records interrupted and require fresh admission rather than inventing resumable SDK state.
- Exercise corrupt history, failed migrations, downgrade rejection, cancellation during migration, and recovery after partial persistence. Restoring legacy state never dispatches work.

**Verification command**
`go test -race -timeout=15m ./internal/app ./internal/app/agentstate ./internal/app/agentruntime ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `refactor: complete the ADK runtime migration`

## Task 36 — Complete TUI accessibility and interaction verification

- [ ] Complete

**Target files**
- `internal/tui/view.go`, `internal/tui/theme_styles.go`, `internal/tui/theme/graphics.go`, `internal/tui/viewport.go`, `internal/tui/hub_agents.go`, `internal/tui/hub_settings.go`, `internal/tui/agent_run_view.go`, `internal/tui/agent_library.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`.
- `internal/tui/view_test.go`, `internal/tui/workspace_view_test.go`, `internal/tui/workspace_scenarios_test.go`, `internal/tui/command_menu_test.go`, `internal/tui/agent_run_test.go`, `internal/tui/agent_library_test.go`, `internal/tui/theme/graphics_test.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 35; every new interaction has a real backend and explicit error/recovery behavior.

**Implementation rules**
- Verify all five hubs and four palettes at compact/wide dimensions, including loading, empty, populated, pending-input, disconnected, failed, partial, stale, and canceled states.
- Keep contrast at least 4.5:1, semantic text labels, visible focus and actions, unclipped long paths, stable scrolling, and safe ANSI handling. Avoid repeated borders, excessive badges, technical SDK labels, and expanding dashboard density.
- Test complete keyboard journeys from investigation through approval, checks, review, Apply, worktree integration, and recovery. Navigation, disclosures, completion, and history inspection must dispatch no model/tool/source effects.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `test: verify professional agent TUI interactions and accessibility`

## Task 37 — Validate the full integration and document supported behavior

- [ ] Complete

**Target files**
- `internal/agenteval/runner_test.go`, `internal/agenteval/fixtures_test.go`, `internal/agenteval/testdata/tasks.json`, `internal/app/agent_integration_test.go`, `internal/tui/agent_journey_test.go` — final two files **new**.
- `README.md`, `CONFIG.md`, `config.example.yaml`, `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md`, `docs/agent-deployment.md`, `docs/technical-docs.html`, `AGENTS.md`, `internal/AGENTS.md`.

**Inputs / dependencies**
- Task 36; frozen corpus and implemented capability matrix. Live provider, media, and cloud smoke tests remain dependent on explicit operator invocation and credentials.

**Implementation rules**
- Run deterministic end-to-end scenarios for tools, workflow branches, persistence failures, approval replay, provider rejection, restart, source changes, worktree conflict, integration disconnect, and terminal recovery. Verify negative cases have no unauthorized side effects.
- Compare recorded baseline and ADK runs using identical fixtures/models/budgets where real measurements exist; never claim quality/cost improvement from fake-provider tests. Report unavailable live evidence separately.
- Document actual commands, supported providers/features, defaults, budgets, data flows, migration, recovery, and optional services. Verify every new route and command. Remove stale four-stage-only and completion-only descriptions where superseded.
- Build daemon, TUI, evaluation, worker, and inspector binaries with the pinned toolchain; confirm CGO-free release targets. Commit only after all required gates pass and review the final cumulative diff for unrelated churn.

**Verification command**
`go mod verify && ./scripts/validate.sh && make build && make build-linux && go build ./cmd/agent-eval ./cmd/agent-worker ./cmd/agent-inspect && git diff --check`

**Commit**: `docs: finalize and validate the full ADK integration`
