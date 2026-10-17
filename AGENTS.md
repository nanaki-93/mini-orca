# Mini-Orca agent instructions

These instructions apply to the whole repository. Before editing, read the
relevant area guide below, even when starting from the repository root. Area
guides add local detail; keep shared rules here instead of copying them.

| Work area | Read before editing |
| --- | --- |
| Go daemon and evaluation CLI (`cmd/`, `internal/`) | [internal/AGENTS.md](internal/AGENTS.md) |
| Desktop client (`desktop/`) | [desktop/AGENTS.md](desktop/AGENTS.md) |
| Python/shell tooling, Makefile and CI | [scripts/AGENTS.md](scripts/AGENTS.md) |
| Documentation and API descriptions | [docs/AGENTS.md](docs/AGENTS.md) |

## Quick commands & single-test execution

- Bootstrap & entrypoints (`justfile` / `Makefile`): `just setup`, `just dev`, `just test`, `just check` (or `make build`, `make dev`, `make test`, `make check`, `make quality`).
- Build daemon: `go build ./cmd/daemon` (or `just build`).
- Run a single Go test: `go test ./internal/app -run TestName -v` (or `just test-one TestName`).
- Run a single frontend test file: `npm --prefix desktop/frontend run build && node desktop/frontend/tests/redesign.mjs`.

## Architecture boundaries & ownership

Mini-Orca is a local-first coding assistant: a Go 1.22 daemon, a Wails/React
desktop client and a loopback HTTP API. It supports project-wide analysis and
checked chat proposals across up to eight captured paths, with diff review,
explicit Apply and guarded Undo.

- `cmd/` owns startup, configuration wiring and command entry points.
- `internal/api/` owns HTTP decoding, local request policy and error responses. Handlers must never import `internal/storage` or `internal/llm` directly; always call `internal/app` service methods.
- `internal/app/` owns workflow orchestration, model calls and guarded changes.
- `internal/project/` owns project facts, indexing, context policy and composition. It must never import `internal/app` or `internal/api`.
- `internal/storage/` owns durable file primitives; domain stores own their schemas.
- `internal/llm/` owns provider transport; `internal/config/` owns configuration.
- `internal/insighteval/` owns the evaluation runner, separate from normal app use.
- `desktop/` owns presentation, client state and user-operated terminal sessions.

## Workspace & commit conventions

- Before editing or creating files in a subdirectory or submodule, check for a local `AGENTS.md`; local instructions override root rules for module style and tests.
- Maintain atomic commits per submodule; never mix submodule and root commits.
- All execution tasks must end with a clean git commit once verification passes. Never leave unstaged or uncommitted changes after completing a task unless explicitly requested otherwise.

## Before changing code

1. Understand the requested outcome and the smallest complete change that delivers it. Trace the entry point, state owner, callers and tests. Search with `rg`.
2. Inspect `git status --short` and staged/unstaged diffs. Preserve the user's existing work; do not reset, stash, discard or reformat unrelated edits.
3. Identify observable behavior, failure cases and checks that establish completion. For a bug, reproduce it or add a regression case that exposes it.
4. Resolve routine choices from the code and request. Ask only when missing information materially changes product behavior, compatibility or scope.

Current behavior is documented in the README, desktop guides and API contract. Do not infer authorization from old task queues or `.mini-orca/autopilot/` worktrees.

## Implementation, error handling & concurrency standard

- Keep the diff limited to the requested behavior and its necessary callers, tests and docs. Fix the underlying cause in its owning layer.
- Prefer straightforward control flow, precise domain names, cohesive functions and explicit inputs/results. Model distinct states explicitly instead of loosely related booleans or ambiguous null/empty values.
- Reuse existing package boundaries, error patterns and shared controls. Validate external input at its boundary and preserve domain checks in their owner.
- **Error handling**: Handle failures explicitly with useful context (`fmt.Errorf("action: %w", err)`). Never turn a failure into success, an empty result or a default value unless that fallback is a defined, tested behavior.
- **Concurrency & cancellation**: Propagate `context.Context` timeouts and cancellation across all I/O, child processes and model calls. Guard shared state with explicit mutex ownership and deterministic synchronization in tests; never leak background goroutines after a job or test completes.
- Replace superseded implementations and remove obsolete helpers, branches, imports, call sites and tests. Finish the requested path end to end without placeholders, inert controls, fabricated data, swallowed errors or TODOs.
- Comments explain intent or constraints. Do not broaden a fix into a dependency upgrade, general refactor or formatting sweep, and never weaken checks or quality thresholds.

## UI, UX & product boundaries

- Follow [desktop/UI_DESIGN_GUIDELINES.md](desktop/UI_DESIGN_GUIDELINES.md) for interaction and verification expectations. Keep actions, states, errors, scope, consent and recovery clear in both GUI and TUI modes.
- Source and composed diffs are read-only except guided `AGENTS.md` instruction text. Source mutation requires explicit diff Review/Apply and guarded Undo with current project, proposal and source identity/hash checks.
- Navigation, disclosures, previews and local restore must not trigger provider requests, project-code execution or source writes. Keep remote-provider consent, Security review intent and execution trust tied to the relevant scope.
- Keep model suggestions, verified findings and measured benchmarks distinct. Failed, unavailable, partial, stale and canceled results retain their meaning.
- Keep the daemon loopback-only by default. Never commit credentials, provider tokens, source-bearing diagnostics, local `config.yaml`, or generated outputs (`build/`, `desktop/build/`, `.mini-orca/autopilot/`).

## Validation, diff review & handoff

| Changed area | Required checks when the environment permits |
| --- | --- |
| Go code | `go fmt ./...`, `make fmt-check`, `go test ./...`, `make test-race`, `make vet`, `./scripts/quality.sh --go-only` |
| Desktop web client/host | `make web-test`; native packaging changes also require `make web-build` on macOS arm64 |
| Python/shell tooling | `python3 -m unittest discover -s scripts/tests -p 'test_*.py'` and syntax checks for changed shell scripts |
| Prose-only documentation/instructions | `git diff --check`; verify referenced paths, commands and consistency with current code |
| Cross-stack changes or full validation request | `./scripts/validate.sh` (or `just check`) |

- Tests must establish behavior and cover meaningful failure or boundary cases using temporary projects, fake providers and deterministic synchronization.
- Review the final diff (`git diff --check`), remove unrelated churn or dead code, and report what changed, the checks run, and any remaining limits.
