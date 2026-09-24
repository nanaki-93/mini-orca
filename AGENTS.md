# Mini-Orca agent instructions

These instructions apply to the whole repository. Before editing, read the
relevant area guide below, even when starting from the repository root. Area
guides add local detail; keep shared rules here instead of copying them.

| Work area | Read before editing |
| --- | --- |
| Go daemon and evaluation CLI (`cmd/`, `internal/`) | [internal/AGENTS.md](internal/AGENTS.md) |
| Kotlin/Compose client (`desktop/`) | [desktop/AGENTS.md](desktop/AGENTS.md) |
| Python/shell tooling, Makefile and CI | [scripts/AGENTS.md](scripts/AGENTS.md) |
| Documentation and API descriptions | [docs/AGENTS.md](docs/AGENTS.md) |

## Project and ownership

Mini-Orca is a local-first coding assistant: a Go 1.22 daemon, a Kotlin/Compose
Desktop client and a loopback HTTP API. It supports project-wide analysis and
one deliberate Go declaration edit at a time, with an editable draft, checks,
Review, explicit Apply and guarded Undo.

- `cmd/` owns startup, configuration wiring and command entry points.
- `internal/api/` owns HTTP decoding, local request policy and error responses.
- `internal/app/` owns workflow orchestration, model calls and guarded changes.
- `internal/project/` owns project facts, indexing, context policy and composition.
- `internal/storage/` owns durable file primitives; domain stores own their schemas.
- `internal/llm/` owns provider transport; `internal/config/` owns configuration.
- `internal/insighteval/` owns the evaluation runner, separate from normal app use.
- `desktop/` owns presentation, client state and user-operated terminal sessions.
# Workspace Conventions

## Submodule & Nested Directory Protocol
- Before editing or creating files in a subdirectory or submodule, check if that directory contains an `AGENTS.md` file.
- Always check if the working directory or submodule contains a local `AGENTS.md`.
- When operating inside a submodule, local `AGENTS.md` instructions override root instructions for code style and module tests.
- Maintain atomic commits per submodule: do not mix submodule git commits with root repository commits in a single `git commit`.

## Commit Requirements
- All execution tasks must end with a clean git commit once verification passes.
- Never leave unstaged or uncommitted changes after completing a task unless explicitly requested otherwise.

## Before changing code

1. Understand the requested outcome and the smallest complete change that delivers
   it. Trace the affected entry point, state owner, callers and tests. Read nearby
   implementations before introducing a new pattern. Search with `rg`.
2. Inspect `git status --short` and relevant staged/unstaged diffs. Preserve the
   user's existing work, including changes in files you need to edit. Do not reset,
   stash, discard or reformat unrelated edits. If another writer changes your
   target, inspect the overlap before continuing.
3. Identify the observable behavior, failure cases and checks that will establish
   completion. For a bug, reproduce it or add a regression case that exposes it.
   A brief plan is enough for a small edit; do not create a task ledger by default.
4. Resolve routine implementation choices from the code and the user's request.
   Ask only when missing information materially changes the product behavior,
   compatibility or authorized scope. Continue independent work in the meantime.

Current behavior is documented in the README, desktop guides and API contract.
Do not infer authorization from old task queues or generated
`.mini-orca/autopilot/` worktrees.

## Implementation standard

- Keep the diff limited to the requested behavior and its necessary callers,
  tests and documentation. Fix the underlying cause in its owning layer. A
  complete repository change may touch several files; Mini-Orca's one-file Apply
  constraint governs the product's source-editing workflow.
- Prefer straightforward control flow, precise domain names, cohesive functions
  and explicit inputs/results. Model distinct states explicitly instead of adding
  loosely related booleans or ambiguous null/empty values.
- Reuse the existing package boundaries, error patterns and shared controls.
  Extract shared logic when it represents the same maintained rule. Do not add
  generic utilities, pass-through layers, configurable frameworks or new
  dependencies for hypothetical future use.
- Validate external input at its boundary and preserve domain checks in their
  owner. Do not duplicate eligibility, identity or validation rules in handlers,
  presentation helpers or adapters.
- Handle failures explicitly with useful context. Never turn a failure into
  success, an empty result or a default value unless that fallback is a defined,
  tested part of the behavior. Preserve cancellation and timeout semantics.
- Replace superseded implementations and remove obsolete helpers, branches,
  imports, call sites and tests. Keep compatibility code only for an existing
  supported contract or persisted format; explain and test that requirement.
- Finish the requested path end to end. Do not ship placeholders, inert controls,
  fabricated production data, swallowed errors or TODOs in place of working behavior.
- Comments explain intent or constraints. Avoid narration of obvious code,
  generic docstrings, speculative documentation and summaries that merely repeat
  the diff. Update the existing owner of a fact when its behavior changes.
- Do not broaden a fix into a dependency/toolchain upgrade, general refactor or
  formatting sweep. Never weaken checks, raise quality thresholds or bless new
  baselines just to make the change pass.

## UI and UX standard

- Visual design is not fixed by previous mockups or documentation. Palette,
  typography, shapes, spacing, layout, navigation and copy may change.
- Keep UI behavior understandable and accessible: actions, states, errors, scope,
  consent and recovery remain clear. Do not hide useful requested content.
- Follow [desktop/UI_DESIGN_GUIDELINES.md](desktop/UI_DESIGN_GUIDELINES.md)
  for interaction and verification expectations, not as a visual specification.

## Product boundaries to preserve

- Source and composed diffs are selectable/read-only; only the isolated
  declaration/import draft is editable. Source mutation uses the existing
  explicit Review/Apply and Undo paths, with current identity and hash checks.
- Navigation, disclosures, previews and local restore must not trigger provider
  requests, project-code execution or source writes. Keep remote-provider consent,
  Security review intent and execution trust tied to the relevant scope.
- Keep model suggestions, verified findings and measured benchmarks distinct.
  Failed, unavailable, partial, stale and canceled results must retain their
  meaning; unknown counts are not zero and daemon health is not provider health.
- Keep the daemon loopback-only by default; external binding requires explicit
  configuration. Preserve browser-origin and local request protections.
- Do not add credentials, provider tokens, source-bearing diagnostics or local
  `config.yaml` to Git. Use `config.example.yaml` for configuration examples.
- Do not hand-edit generated build output, caches, dependency trees or local
  runtime evidence, including `build/`, `desktop/build/`, `desktop/.gradle/`,
  `desktop/.kotlin/` and `.mini-orca/autopilot/`.

## Validation

Use focused tests while iterating, then the relevant gates below before handoff.
Commands run from the repository root. Toolchain setup belongs to
[README.md](README.md#development-and-documentation); do not hardcode local paths.

| Changed area | Required checks when the environment permits |
| --- | --- |
| Go code | `go fmt ./...`, `make fmt-check`, `go test ./...`, `make test-race`, `make vet`, `./scripts/quality.sh --go-only` |
| Desktop code/build | `./scripts/desktop-gradle.sh test spotlessCheck detekt` |
| Python/shell tooling | `python3 -m unittest discover -s scripts/tests -p 'test_*.py'` and syntax checks for changed shell scripts |
| Prose-only documentation/instructions | `git diff --check`; verify referenced paths, commands and consistency with current code |
| Cross-stack changes or full validation request | `./scripts/validate.sh` |

`make check` covers formatting, Go tests/races/vet, Python tooling tests and
desktop tests. `make quality` adds the maintained static, reachability, complexity,
clone and desktop formatting checks. `./scripts/validate.sh` combines the full
local gates and is also used by CI. Documentation that changes a contract or
build procedure needs the corresponding contract tests or procedure validation.

Tests must establish behavior: cover the changed path and meaningful failure or
boundary cases. Prefer temporary projects, fake providers and deterministic
synchronization. Do not require a running daemon, provider key or live model for
ordinary validation. Do not add tests that merely restate constants, grep source
or mirror private implementation details when behavior can be exercised directly.
Reuse passing results only while the tested inputs remain unchanged.

## Diff review and handoff

- Review the final diff, including new files. Check ownership, error paths,
  cancellation, stale state, compatibility and test coverage. Remove unrelated
  churn, debug output, duplication and dead code; run `git diff --check`.
- Fix failures caused by this change. If a check is blocked or fails outside the
  changed scope, report the exact command, result and reason. Do not describe an
  unrun, skipped or failed check as passing, or silently reduce its scope.
- Report what behavior changed, the checks actually run, any remaining limits
  and required configuration/migration steps. Keep the handoff concise.
- Do not create commits, push or publish unless explicitly requested for the
  current work. Do not run Docker cleanup or destructive make targets unless
  explicitly requested.
