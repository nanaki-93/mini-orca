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

- [ ] Complete

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

- [ ] Complete

**Target files**
- `internal/tui/hub_agents.go`, `internal/tui/workflow_progress.go`, `internal/tui/domain/domain.go`, `internal/tui/domain/remediation.go`, `internal/tui/state.go`, `internal/tui/dashboard.go`, `internal/tui/view.go`, `internal/tui/theme_styles.go`.
- `internal/tui/domain/agent_runs.go`, `internal/tui/domain/agent_runs_test.go` — **new**.
- `internal/tui/workflow_progress_test.go`, `internal/tui/view_test.go`, `internal/tui/workspace_view_test.go`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 13; existing five workspaces, palettes, viewport behavior, and keyboard rules.

**Implementation rules**
- Replace hard-coded four-stage positions with stable stage IDs, typed states, and graph-derived progress. Render sequential and parallel work accurately without invented percentages.
- Present compact task rows and one clear next action; replace decorative fleet/mission terminology and repeated panels with plain task, stage, evidence, and recovery labels.
- Preserve all five workspaces, visible project/Git identity, command access, focus, and scroll state. At small sizes prioritize content and actions; support at least 52x16, 80x24, 100x20, and 160x40 plus the resize prompt.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: adapt the TUI to dynamic agent workflows`

## Task 15 — Add execution, evidence, and approval detail views

- [ ] Complete

**Target files**
- `internal/tui/agent_run_view.go`, `internal/tui/agent_run_actions.go`, `internal/tui/agent_run_test.go` — **new**.
- `internal/tui/sse_events.go`, `internal/tui/daemon_actions.go`, `internal/tui/keys.go`, `internal/tui/colon_commands.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `internal/tui/detail_view.go`, `internal/tui/state.go`, `internal/tui/workspace_scenarios_test.go`.

**Inputs / dependencies**
- Task 14; structured event/control/input API.

**Implementation rules**
- Provide Overview, Timeline, Evidence, Usage, and Recovery sections in task detail; expose tool arguments/results through bounded sanitized disclosures. Stream text without moving selection or forcing scroll-to-bottom.
- Wire pause, resume, cancel, stage retry, and pending-input decisions to their real guarded endpoints. Hide or explain unavailable actions; prevent duplicate submission.
- Pending approvals show exact scope and destination; keep per-file diff review and Apply separate. Test reconnects, old events, project switches, long content, terminal escape injection, and passive navigation.

**Verification command**
`make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add agent execution and approval detail views`

## Task 16 — Move resumable project analysis onto ADK workflows

- [ ] Complete

**Target files**
- `internal/app/agentruntime/analysis_graph.go`, `internal/app/agentruntime/analysis_graph_test.go`, `internal/app/agent_analysis_host.go` — **new**.
- `internal/app/analysis_run.go`, `internal/app/analysis_stage_runner.go`, `internal/app/analysis_run_progress.go`, `internal/app/analysis_run_store.go`, `internal/app/analysis_run_retry.go`, `internal/app/analysis_run_freshness.go`, `internal/app/analysis_run_features.go`.
- `internal/app/analysis_run_test.go`, `internal/app/analysis_run_recovery_test.go`, `internal/app/analysis_run_retry_test.go`, `internal/app/analysis_freshness_test.go`.

**Inputs / dependencies**
- Task 15; analysis selection/admission, resumable coverage, stage cache identity, and security intent.

**Implementation rules**
- Express per-file/stage analysis as bounded graph work with deterministic aggregation. Preserve deterministic security rules as distinct evidence and retain semantic/performance/security model scopes.
- Reuse only fresh matching results. Count every actual provider attempt across resume/retry, and preserve partial coverage and failed/canceled/unavailable distinctions.
- Keep selection, preview, filters, restore, and inspection passive. Project replacement/reindex cancels owned work and blocks stale publication. Retire the superseded dispatcher logic in the changed owner files.

**Verification command**
`go test -race ./internal/app ./internal/app/agentruntime ./internal/project && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: execute project analysis through ADK workflows`

## Task 17 — Integrate documentation, features, and explanations

- [ ] Complete

**Target files**
- `internal/app/agentruntime/knowledge_graph.go`, `internal/app/agentruntime/knowledge_graph_test.go` — **new**.
- `internal/app/feature_suggestions.go`, `internal/app/declaration_explanation.go`, `internal/app/semantic_review.go`, `internal/app/performance_review.go`, `internal/app/security_review.go`, `internal/app/service.go`.
- `internal/project/project_overview.go`, `internal/project/architecture_overview_report.go`, `internal/project/architecture_overview_report_test.go`, `internal/project/handbook/handbook.go`, `internal/project/handbook/handbook_test.go`.
- `internal/app/feature_suggestions_test.go`, `internal/app/declaration_explanation_test.go`, `internal/app/project_analysis_contract_test.go`.

**Inputs / dependencies**
- Task 16; current report schemas, provenance, goals, and deterministic handbook facts. `internal/project/project_overview.go` owns the current analyzer; inject a provider-neutral adapter there.

**Implementation rules**
- Route all remaining model-backed knowledge operations through the shared runtime, tools, budget, and event infrastructure. Keep project facts and handbook composition independent of ADK.
- Publish only validated domain reports with anchored evidence and compatible history. Distinguish repository facts, suggestions, and unverified explanations; do not synthesize benchmark claims.
- Preserve explicit user actions and current provider scopes. Existing report readers and the Docs/Features TUI remain usable throughout migration.

**Verification command**
`go test ./internal/app ./internal/app/agentruntime ./internal/project/... && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate knowledge workflows with ADK`

## Task 18 — Add curated persistent project memory

- [ ] Complete

**Target files**
- `internal/app/agentstate/memory.go`, `internal/app/agentstate/memory_test.go`, `internal/app/agentruntime/memory.go`, `internal/app/agentruntime/memory_test.go`, `internal/app/agent_memory.go`, `internal/app/agent_memory_test.go` — **new**.
- `internal/app/change_pipeline_stages.go`, `internal/app/agentruntime/investigator.go`, `internal/app/agentruntime/knowledge_graph.go`, `internal/app/agentruntime/runtime.go`.

**Inputs / dependencies**
- Task 17; existing harness lessons and instruction graduation workflow.

**Implementation rules**
- Store accepted decisions, verified outcomes, and explicitly retained lessons with provenance, source revisions, scope, timestamps, and invalidation rules. Use a bounded local search implementation behind ADK's memory interface.
- Separate candidate memories from accepted memory; model text cannot grant itself authority or override AGENTS.md. A stale memory is advisory and visibly stale.
- Retrieve only admitted relevant records; support update, forget, retention, and project isolation. Avoid automatically retaining entire conversations or secrets; deletion removes retrieval eligibility immediately.

**Verification command**
`go test -race ./internal/app/agentstate ./internal/app/agentruntime ./internal/app && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add curated local agent memory`

## Task 19 — Bound context with compaction and safe cache reuse

- [ ] Complete

**Target files**
- `internal/app/agentruntime/compaction.go`, `internal/app/agentruntime/compaction_test.go`, `internal/app/agentstate/context_cache.go`, `internal/app/agentstate/context_cache_test.go` — **new**.
- `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/session.go`, `internal/app/agentruntime/budget.go`, `internal/app/change_context.go`, `internal/config/agent_runtime.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 18; [Go compaction support](https://adk.dev/context/compaction/) and durable session events.

**Implementation rules**
- Configure bounded history/tail retention and persist compaction records so restart cannot repeatedly summarize/bill the same events. Charge summarization to the same run and authorized model scope.
- Keep approved plans, hard constraints, grants, IDs, and source fingerprints outside lossy summaries. Revalidate reconstructed context; fail clearly if mandatory context exceeds budget.
- Cache deterministic retrieval/context by project, source, policy, instructions, schema, and configuration fingerprints. Provider-side prompt caching is capability-gated and opt-in with explicit remote retention/TTL; it never makes stale evidence current.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app/agentstate ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add bounded context compaction and cache reuse`

## Task 20 — Integrate project skills through ADK

- [ ] Complete

**Target files**
- `internal/app/agentruntime/skills.go`, `internal/app/agentruntime/skills_test.go`, `internal/app/agent_skills.go`, `internal/app/agent_skills_test.go` — **new**.
- `internal/config/agent_runtime.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agent_tools.go`, `internal/project/instructions.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 19; [Go SkillToolset](https://adk.dev/skills/), context policy, and trusted execution.

**Implementation rules**
- Load configured skills lazily through validated manifests and scoped filesystem sources. Bound metadata, instructions, resources, paths, and hashes; expose active skill provenance in run events.
- Preserve repository instruction precedence and explicit task scope. Skills cannot expand grants, install packages, or run scripts automatically.
- If a skill needs execution, route an explicitly admitted, pinned command through the existing sandbox policy. Reject changed resources, escaping symlinks, invalid manifests, and unavailable tools with useful diagnostics.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/project ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate scoped ADK skills`

## Task 21 — Add managed MCP connections and tools

- [ ] Complete

**Target files**
- `internal/config/integrations.go`, `internal/config/integrations_test.go`, `internal/app/agent_integrations.go`, `internal/app/agent_integrations_test.go`, `internal/app/agentruntime/mcp.go`, `internal/app/agentruntime/mcp_test.go` — **new**.
- `internal/config/config.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/tools.go`, `internal/app/service.go`, `cmd/daemon/main.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 20; [ADK MCP toolsets](https://adk.dev/tools-custom/mcp-tools/), shared policy, and process ownership.

**Implementation rules**
- Support explicitly configured stdio and Streamable HTTP servers with tool allowlists, endpoint-bound authentication, bounded discovery, timeouts, and owned connection/process cleanup.
- Inspecting configuration stays passive. Connection/discovery and tool execution require appropriate grants; metadata cannot self-declare a mutating tool safe. Pin discovered tool schemas for a run and invalidate approval on changes.
- Validate every call and result through Mini-Orca budgets, source policy, and effect classification. Cancellation closes sessions and owned children. Never auto-install an MCP server or expose arbitrary filesystem/shell capabilities.

**Verification command**
`go test -race ./internal/config ./internal/app/agentruntime ./internal/app ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: integrate managed MCP connections and scoped tools`

## Task 22 — Add curated OpenAPI and grounding tools

- [ ] Complete

**Target files**
- `internal/app/agentruntime/openapi.go`, `internal/app/agentruntime/openapi_test.go`, `internal/app/agentruntime/grounding.go`, `internal/app/agentruntime/grounding_test.go` — **new**.
- `internal/app/agent_integrations.go`, `internal/app/agent_tools.go`, `internal/config/integrations.go`, `internal/config/integrations_test.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 21; approved endpoint/tool inventory and Gemini provider capabilities.

**Implementation rules**
- Wrap explicitly selected OpenAPI operations as typed tools using a documented supported schema subset. Pin operation/schema identity, method, destination, auth reference, and effect class; reject unsupported schema constructs before dispatch.
- Keep HTTP argument mapping, response limits, redirection, credential isolation, and idempotency explicit. Mutating operations require exact-action approval and an outcome receipt; unknown outcomes require reconciliation.
- Add optional Google Search grounding only for supported profiles with explicit network/source consent. Preserve external citations and uncertainty; external text cannot become repository facts or executable instructions.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add curated API and grounded research tools`

## Task 23 — Consume remote agents through A2A

- [ ] Complete

**Target files**
- `internal/app/agentruntime/a2a_client.go`, `internal/app/agentruntime/a2a_client_test.go`, `internal/app/agent_delegation.go`, `internal/app/agent_delegation_test.go` — **new**.
- `internal/config/integrations.go`, `internal/app/agent_integrations.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentstate/store.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 22; [Go A2A support](https://adk.dev/a2a/) and external endpoint admission.

**Implementation rules**
- Fetch and pin trusted configured agent cards only on explicit connection. Delegate bounded analysis/review tasks with minimum admitted context, cancellation, deadlines, durable remote task IDs, and actual usage/unknown-usage states.
- Remote findings/artifacts remain untrusted advisory inputs. Remote agents cannot grant local permissions, enlarge write scope, apply changes, or request additional credentials implicitly.
- Handle streaming reconnect, input-required, auth failure, malformed artifacts, and uncertain outcomes. Do not retry a remote side effect without an established idempotency contract.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add guarded A2A delegation`

## Task 24 — Expose a guarded Mini-Orca agent endpoint

- [ ] Complete

**Target files**
- `internal/app/agent_remote_service.go`, `internal/app/agent_remote_service_test.go`, `internal/app/agentruntime/a2a_server.go`, `internal/app/agentruntime/a2a_server_test.go` — **new**.
- `cmd/daemon/main.go`, `cmd/daemon/main_test.go`, `internal/config/integrations.go`, `internal/api/local_policy.go`, `internal/api/request_test.go`, `CONFIG.md`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 23; existing local request policy, run lifecycle, and SDK authentication/authorization facilities.

**Implementation rules**
- Add an opt-in A2A listener on loopback by default with explicit authenticated principals and per-project capabilities. Its agent card advertises only implemented operations.
- Route requests through normal application admission. Allow analysis and proposal preparation; source Apply/Undo/Merge remain local human actions.
- Separate public metadata from private sessions/artifacts. Test authentication, project isolation, revoked grants, resource limits, cancellation, and absence of default exposure. Do not mount the unrestricted ADK REST/dev server onto the daemon.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/api ./cmd/daemon && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: expose a guarded local A2A service`

## Task 25 — Add professional model, skill, and integration settings

- [ ] Complete

**Target files**
- `internal/api/handlers/agent_settings_handler.go`, `internal/api/handlers/agent_settings_handler_test.go`, `internal/app/agent_settings.go`, `internal/app/agent_settings_test.go` — **new**.
- `cmd/daemon/main.go`, `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/hub_settings.go`, `internal/tui/domain/models.go`, `internal/tui/model_details_test.go`, `internal/tui/state.go`, `internal/tui/daemon_actions.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`.
- `docs/api-contract.md`, `docs/openapi.yaml`, `docs/UI_DESIGN_GUIDELINES.md`.

**Inputs / dependencies**
- Task 24; provider capability catalog, role budgets, skills, MCP/OpenAPI/A2A configuration.

**Implementation rules**
- Use compact Settings lists for Models, Roles/Budgets, Skills, and Connections; place parameters and scopes in selected-item details. Show configured, disconnected, unavailable, and failed distinctly.
- Wire connect/disconnect, validate, enable/disable, and revoke to explicit guarded actions. Opening or selecting an entry causes no network or process activity.
- Display credential references/redacted status only. Freeze active-run configuration; changes apply to subsequent admission unless an explicit cancellation/restart is chosen. Preserve current presets and scope assignments.

**Verification command**
`go test ./internal/app ./internal/api/handlers ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add agent capability and integration settings`

## Task 26 — Add artifact and memory management to the TUI

- [ ] Complete

**Target files**
- `internal/app/agent_library.go`, `internal/app/agent_library_test.go`, `internal/api/handlers/agent_library_handler.go`, `internal/api/handlers/agent_library_handler_test.go` — **new**.
- `internal/tui/agent_library.go`, `internal/tui/agent_library_test.go` — **new**.
- `cmd/daemon/main.go`, `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/state.go`, `internal/tui/hub_settings.go`, `internal/tui/keys.go`, `internal/tui/command_menu.go`, `internal/tui/keybinds_help.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 25; local artifact and curated memory stores.

**Implementation rules**
- Add artifact/evidence navigation within Code and memory/lessons management within Settings. Show versions, source links, freshness, ownership, size, retention, and whether a record is candidate or accepted.
- Implement preview, accept/reject memory, forget, and explicit artifact export/delete with accurate receipts. Reuse source/diff viewers for text; render binary metadata without pretending the terminal can display arbitrary media.
- Preserve active recovery references, reject stale mutations, sanitize terminal text, and keep browsing passive. Export never writes project source through the artifact API.

**Verification command**
`go test ./internal/app ./internal/api/handlers ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add artifact and memory library views`

## Task 27 — Add traceable usage and optional telemetry export

- [ ] Complete

**Target files**
- `internal/app/agentruntime/telemetry.go`, `internal/app/agentruntime/telemetry_test.go`, `internal/app/agent_usage.go`, `internal/app/agent_usage_test.go` — **new**.
- `internal/app/agentruntime/runtime.go`, `internal/app/agentstate/store.go`, `internal/config/agent_runtime.go`, `internal/tui/agent_run_view.go`, `internal/tui/client/agent_models.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 26; [ADK Go tracing](https://adk.dev/observability/traces/) and durable event/usage records.

**Implementation rules**
- Correlate run, node, tool, provider, retry, and check spans. Aggregate reported input/output/cache tokens, call counts, elapsed time, and failures without losing failed-attempt consumption.
- Keep local source-free trace summaries as the default. Optional OTLP export requires a configured destination and explicit grant; scrub content, arguments, URLs with secrets, and provider diagnostics before persistence/export.
- Monetary amounts require a versioned user-configured price table; otherwise show tokens and unknown cost. Display estimates and measured usage distinctly and account for summarization/delegation.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add source-free agent tracing and usage accounting`

## Task 28 — Add task and tool-trajectory evaluation

- [ ] Complete

**Target files**
- `cmd/agent-eval/main.go`, `cmd/agent-eval/main_test.go`, `internal/agenteval/runner.go`, `internal/agenteval/runner_test.go`, `internal/agenteval/scoring.go`, `internal/agenteval/fixtures_test.go`, `internal/agenteval/testdata/tasks.json` — **new**.
- `internal/app/agent_evaluation.go`, `internal/app/agent_evaluation_test.go` — **new**, bounded evaluation entry point.
- `scripts/quality.sh`, `scripts/tests/test_quality.py`, `README.md`, `internal/AGENTS.md`.

**Inputs / dependencies**
- Task 27; existing engineering-insight evaluation conventions. [ADK evaluation documentation](https://adk.dev/evaluate/) is Python-specific; preserve a native Go harness.

**Implementation rules**
- Evaluate fixed task fixtures, tool-call correctness, scope compliance, independent regression tests, unsupported claims, cancellation, tokens, latency, and repair counts. Version corpus, prompt, model, runtime, and scoring identities.
- Provide fake-provider/replay mode as the deterministic default. Live evaluation requires an explicit command, source/destination disclosure, and request/token budget; never run it in ordinary tests or CI.
- Keep receipts source-free and failures/incomplete runs visible. Add the new CLI to the maintained deadcode roots, without excluding production packages from checks.

**Verification command**
`go test -race ./internal/agenteval ./cmd/agent-eval ./internal/app && python3 -m unittest discover -s scripts/tests -p 'test_*.py' && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add native agent task and trajectory evaluation`

## Task 29 — Plan larger missions as separately approved tasks

- [ ] Complete

**Target files**
- `internal/app/agent_mission.go`, `internal/app/agent_mission_test.go`, `internal/app/agentstate/missions.go`, `internal/app/agentstate/missions_test.go`, `internal/app/agentruntime/mission_planner.go` — **new**.
- `internal/api/handlers/agent_mission_handler.go`, `internal/api/handlers/agent_mission_handler_test.go` — **new**.
- `cmd/daemon/main.go`, `internal/tui/client/client.go`, `internal/tui/client/agent_models.go`, `internal/tui/plan.go`, `internal/tui/plan_actions.go`, `internal/tui/plan_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 28; investigator, approved plans, worktree ownership, and durable task identities.

**Implementation rules**
- Decompose an explicit project goal into a validated acyclic dependency graph of bounded tasks with acceptance criteria, read/write scope, budgets, and integration order.
- Keep every child within the existing eight-file limit and require review of the concrete mission/child plans. Reject cycles, overlapping simultaneous writers, unreachable dependencies, and unbounded task counts.
- Mission inspection and editing are passive; planning never launches children. Expose the plan in Work using the existing focused plan/detail patterns.

**Verification command**
`go test -race ./internal/app ./internal/app/agentstate ./internal/app/agentruntime ./internal/api/handlers ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add scoped multi-task mission planning`

## Task 30 — Execute mission dependencies with guarded worktree integration

- [ ] Complete

**Target files**
- `internal/app/agent_mission_run.go`, `internal/app/agent_mission_run_test.go`, `internal/app/agentruntime/mission_graph.go`, `internal/app/agentruntime/mission_graph_test.go` — **new**.
- `internal/app/agent_mission.go`, `internal/app/agentstate/missions.go`, `internal/app/worktree_service.go`, `internal/app/git_worktrees.go`, `internal/app/git_merge.go`, `internal/app/git_worktree_cleanup.go`, `internal/app/agent_runs.go`.
- `internal/tui/hub_agents.go`, `internal/tui/agent_run_actions.go`, `internal/tui/worktree_pending.go`, `internal/tui/worktree_lifecycle_test.go`.

**Inputs / dependencies**
- Task 29; checked change graph and current guarded worktree lifecycle.

**Implementation rules**
- Schedule ready children with cumulative mission budgets and explicit writer ownership. Parallelize only disjoint work; default integration order follows the approved dependency graph.
- Each child produces a checked proposal for human review. Apply commits only that reviewed child; merge remains an explicit action. Rebase/changed base requires fresh checks and review where identity changes.
- Pause/cancel cascades deterministically; restart reconciles owned worktrees and pending effects. Surface blocked dependencies, conflicts, rejected children, and cleanup failures without losing completed evidence.

**Verification command**
`go test -race ./internal/app ./internal/app/agentruntime ./internal/app/agentstate && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: execute scoped missions with guarded worktree integration`

## Task 31 — Support explicit multimodal attachments and live sessions

- [ ] Complete

**Target files**
- `internal/app/agentruntime/live.go`, `internal/app/agentruntime/live_test.go`, `internal/app/agent_attachments.go`, `internal/app/agent_attachments_test.go` — **new**.
- `internal/llm/generation.go`, `internal/llm/capabilities.go`, `internal/app/agentruntime/gemini.go`, `internal/app/agentstate/artifacts.go`, `internal/app/agent_runs.go`, `internal/config/agent_runtime.go`.
- `internal/api/handlers/agent_run_handler.go`, `internal/api/handlers/agent_run_handler_test.go`, `internal/tui/agent_run_actions.go`, `internal/tui/agent_run_view.go`, `internal/tui/agent_run_test.go`, `docs/api-contract.md`, `docs/openapi.yaml`, `CONFIG.md`.

**Inputs / dependencies**
- Task 30; artifact privacy and verified provider capability. Pinned Go SDK includes `agent.LiveSession`; test the actual runner/provider path rather than assuming parity with Python examples.

**Implementation rules**
- Allow explicitly selected bounded images/documents/audio artifacts for supported models. Validate MIME/content, size, ownership, scope, and destination; reject arbitrary provider fetch URLs and implicit uploads.
- Implement bounded bidirectional text/media input through an explicit session lifecycle, with interrupt/close/reconnect states and the same tool/consent budget enforcement at dispatch. Live callbacks alone are insufficient as policy gates.
- The TUI shows attachments, transcript, and connection state; no automatic microphone/camera capture or terminal media playback. Unsupported capabilities fail before network dispatch. External capture/playback clients are a separate product decision, not an inert TUI control.

**Verification command**
`go test -race ./internal/llm ./internal/app ./internal/app/agentruntime ./internal/api/handlers && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add explicit attachments and bounded live agent sessions`

## Task 32 — Add opt-in ambient analysis triggers

- [ ] Complete

**Target files**
- `internal/app/agent_triggers.go`, `internal/app/agent_triggers_test.go`, `internal/app/agentstate/triggers.go`, `internal/app/agentstate/triggers_test.go` — **new**.
- `internal/config/agent_runtime.go`, `internal/app/project_lifecycle.go`, `internal/app/service.go`, `internal/app/agent_settings.go`, `internal/api/handlers/agent_settings_handler.go`, `cmd/daemon/main.go`.
- `internal/tui/hub_settings.go`, `internal/tui/dashboard.go`, `internal/tui/agent_run_test.go`, `CONFIG.md`, `config.example.yaml`, `docs/api-contract.md`, `docs/openapi.yaml`.

**Inputs / dependencies**
- Task 31; reusable analysis admission, persisted limits, and lifecycle cancellation.

**Implementation rules**
- Offer explicit per-project file-change/interval triggers with debounce, deduplication, quiet periods, and persisted per-window budgets. Disabled is the default.
- Read-only change detection may suggest a run; automatic provider work requires a separately recorded trigger grant naming scope, provider, and limits. Never auto-apply or auto-merge.
- Revocation stops pending work; project replacement and shutdown cleanly stop owned watchers/timers. Restoring settings does not replay missed triggers. Show last run, next eligible time, reason, and disable action without noisy repeated notifications.

**Verification command**
`go test -race ./internal/app ./internal/app/agentstate ./internal/config ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

**Commit**: `feat: add opt-in bounded ambient analysis`

## Task 33 — Add optional cloud-backed agent services

- [ ] Complete

**Target files**
- `internal/app/agentruntime/cloud_services.go`, `internal/app/agentruntime/cloud_services_test.go`, `internal/app/agent_data_policy.go`, `internal/app/agent_data_policy_test.go` — **new**.
- `internal/config/integrations.go`, `internal/app/agent_settings.go`, `internal/app/agentruntime/runtime.go`, `internal/app/agentruntime/session.go`, `internal/app/agentruntime/artifacts.go`, `internal/app/agentruntime/memory.go`, `CONFIG.md`, `config.example.yaml`.

**Inputs / dependencies**
- Task 32; explicit remote grants. Real Google Cloud project, region, identities, and storage resources are deployment inputs supplied by the operator; tests use fakes.

**Implementation rules**
- Support configured ADK Vertex session/memory and GCS artifact adapters where pinned SDK conformance passes. Keep local services the default and application mutation authority local.
- Bind data scope, principal, project, region, retention, and deletion semantics to the selected service. Enabling a backend never migrates or uploads existing histories automatically.
- Implement explicit validated export/migration with receipts and recoverable failures; reject unsupported checkpoint/confirmation combinations. Report remote deletion or availability failures honestly and redact diagnostics.

**Verification command**
`go test -race ./internal/app/agentruntime ./internal/app ./internal/config && ./scripts/validate.sh && git diff --check`

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
`go test -race ./internal/app ./internal/app/agentstate ./internal/app/agentruntime ./cmd/daemon && make tui-test && ./scripts/validate.sh && git diff --check`

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
