# Mini-Orca

A local-first coding assistant with a pure Go **Terminal UI (TUI)** and a loopback Go daemon, built to analyze repositories, surface verified and semantic **Issues**, synthesize structured **Docs**, suggest goal-aligned **Features**, and prepare guarded **AI Changes** with explicit human review.

## Purpose of the Project

Mini-Orca helps engineers understand and improve local codebases without surrendering control over source mutations or provider boundaries:

- **Analyze projects to find Issues**: Coordinates deterministic static/security rules alongside scoped semantic, performance, and advisory security analysis.
- **Create structured Docs**: Synthesizes a language-agnostic engineering specification covering project purpose, how it pursues its goal, layered architecture, main dependencies, runtime scenarios, ADRs, and operational runbooks.
- **Suggest Features**: Generates goal-aware feature ideas grounded in the repository's indexed architecture and user-defined project goals.
- **Plan and review guarded changes**: Prepares and verifies changes across up to eight captured Go/Markdown paths (or isolated Git worktrees) after an editable plan review, then through a 4-stage pipeline—**Architect & Code → Verify & Test → Critic Review → Human Review**—with explicit diff Apply and guarded Undo.

## How It Pursues the Goal

1. **Deterministic Indexing & Context Policy**: Indexes repository files, symbols, and imports under `.mini-orca/index.json` while enforcing `.mini-orcaignore` and path exclusions before any prompt is constructed.
2. **Scoped Multi-Stage Analysis**: Runs resumable analysis (`analysis/run.json`) across configured model scopes (`analyze`, `bug`, `function`). Separates verified static findings from AI suggestions and preserves explicit remote-provider consent and security-review intent.
3. **4-Stage Verified Change & Agent Pipeline**:
   - **Step 1 · Architect & Code**: Generates a proposal from an explicitly approved approach, write scope, read context and acceptance criteria.
   - **Step 2 · Verify & Test**: Compiles and runs scoped package checks and pinned regression tests (`go test`, syntax/vet checks) with automatic repair off by default; opt in to at most two automatic retries.
   - **Step 3 · Critic Review**: Evaluates the checked diff for regressions, boundary violations, and clean-code precision before human sign-off.
   - **Step 4 · Human Precision Gate**: Presents read-only unified, split and hunk diffs with per-file review for explicit **Accept & Commit**, worktree **Merge** (closing grouped findings), or **Undo**.
4. **Keyboard-First Pure Go TUI**: Provides a 5-hub Terminal UI (`cmd/tui`, `internal/tui`) backed by the loopback API (`127.0.0.1:9090`), with modal vim navigation, `:command` execution, and 4 color palettes (`tokyo`, `crt`, `amber`, `paper`).

## Reviewing a task in the TUI

Selecting a fix or feature opens **Task Plan**. Edit the approach with `e`, write
scope with `f`, and criteria with `t`. Use `:context path; path` for read-only
context, `:boundaries item; item` for non-goals, and `:repairs on|off` for repair
policy. `s` saves a captured draft; `g` approves it and generates code. A custom
`:agent <request>` requires explicit scope and criteria. Use `:plan` to return to
a draft or prepare a revised approach. Paths are never silently dropped to fit
the eight-file write limit.

Use `PgUp/PgDn` in issue, feature, plan and workflow detail views to inspect all
evidence, criteria and diagnostics. Causes remain hypotheses until supported by
reproduction; a test that already passed or failed to compile on the original
source is labeled separately from a reproduced failure.

In the diff, `PgUp/PgDn` scroll explanations while keeping code visible, `h/l` scroll long lines, `w` marks the current file reviewed, `f/F`
moves between files, `e` shows rationale and criterion references, and `z` compares
against the last complete review. `P` opens the plan. Apply shows its commit/merge
destination and requires review of every changed file; changed proposals clear
review progress. `:retry test` reuses implementation, while `:retry review`
reuses current passing checks. Attempts, stage timings and failure reasons remain
visible in workflow details.

## Architecture

Mini-Orca enforces strict layered ownership across the Go daemon and Go TUI:

| Layer / Path | Responsibility |
| --- | --- |
| `cmd/daemon`, `cmd/tui`, `cmd/engineering-insight-eval` | Daemon startup, terminal UI CLI entry point, and evaluation runner. |
| `internal/api` | Loopback HTTP decoding, browser-origin protection, local request policy, and structured error responses. |
| `internal/app` | Workflow orchestration, analysis runs, 4-stage change/agent sessions, Git worktree isolation, and guarded source mutation. |
| `internal/project` | Repository facts, symbol/import indexing, context composition, structured `ProjectDocsReport` synthesis, and `AGENTS.md` instructions. |
| `internal/storage` | Atomic durable file primitives under `.mini-orca/`; domain packages own their persisted schemas. |
| `internal/llm`, `internal/config` | Provider transport (OpenAI-compatible HTTP, `agy`, `pi`), timeouts, and 3-scope model configuration. |
| `internal/tui` | Guarded daemon client, 5-hub TUI state machine, keyboard/colon-command handlers, and ANSI terminal renderer. |

## Main Dependencies

| Dependency | Scope | Purpose |
| --- | --- | --- |
| **Go 1.22+** (`go.mod`, `gopkg.in/yaml.v3`) | Daemon, TUI & CLI | Core HTTP daemon, terminal UI, AST/symbol indexing, analysis orchestration, and atomic `.mini-orca/` storage. |
| **Model Providers** (`HTTP` / `agy` / `pi`) | AI Transport | Pluggable local or remote LLM execution for `analyze`, `bug`, and `function` scopes. |
| **Go Quality & Race Gates** | Verification | Deterministic TUI/daemon verification (`make check`) and static/reachability/complexity/clone checks (`make quality`). |

## Run Locally

Read [CONFIG.md](CONFIG.md) before the first launch.

```sh
cp config.example.yaml config.yaml
just tui
# Or run daemon and TUI separately:
go run ./cmd/daemon
go run ./cmd/tui
```

The daemon defaults to `127.0.0.1:9090`; the TUI connects to `http://127.0.0.1:9090` unless `MINI_ORCA_URL` or `-url` is set. Keep `config.yaml` local. Every prompt-bearing request to a non-loopback HTTP or CLI provider requires scope-specific consent.

### Project Intelligence & Storage (`.mini-orca/`)

Project-local `.mini-orca/` stores `index.json`, `project-analysis.json`, `project-docs.json`, `file-analysis/`, `findings.json`, `performance/files/`, and unified progress in `analysis/run.json`. Private `changes/` files retain conversations, captured source and proposals, feature goals/ideas, the latest grouped recovery journal, and verification evidence. `.mini-orca/autopilot/` holds isolated Git worktrees for autonomous agent tasks. Restore uses local persisted analysis without triggering model requests. See the [API guide](docs/api-contract.md) and [workflow migration](docs/api-contract.md#retired-workflows) for current contracts and boundaries.

## Development and documentation

Follow [AGENTS.md](AGENTS.md). TUI contributions follow the [TUI design guidelines](docs/UI_DESIGN_GUIDELINES.md). Run focused tests while editing. The full local gate is:

```sh
./scripts/validate.sh
```

The checks use fakes and temporary directories, do not start the daemon, and do not require a provider, key, or project configuration.

- `make check`: Formatting, Go unit/race/vet checks, Python tooling, and Go TUI tests.
- `make quality`: Pinned Go static, reachability, complexity, and clone checks.
- [Interactive technical docs](docs/technical-docs.html) · [TUI design guidelines](docs/UI_DESIGN_GUIDELINES.md)
- [API guide](docs/api-contract.md) · [OpenAPI](docs/openapi.yaml) · [configuration](CONFIG.md)
- [Docker](DOCKER.md) · [release notes](RELEASE_NOTES.md) · [LICENSE](LICENSE)
