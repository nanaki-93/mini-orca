# Mini-Orca

A local-first coding assistant with a dual **GUI / TUI** interface and a loopback Go daemon, built to analyze repositories, surface verified and semantic **Issues**, synthesize structured **Docs**, suggest goal-aligned **Features**, and prepare guarded **AI Changes** with explicit human review.

## Purpose of the Project

Mini-Orca helps engineers understand and improve local codebases without surrendering control over source mutations or provider boundaries:

- **Analyze projects to find Issues**: Coordinates deterministic static/security rules alongside scoped semantic, performance, and advisory security analysis.
- **Create structured Docs**: Synthesizes a language-agnostic engineering specification covering project purpose, how it pursues its goal, layered architecture, main dependencies, runtime scenarios, ADRs, and operational runbooks.
- **Suggest Features**: Generates goal-aware feature ideas grounded in the repository's indexed architecture and user-defined project goals.
- **Execute Guarded Changes & Autonomous Agents**: Prepares and verifies changes across up to eight captured Go/Markdown paths (or isolated Git worktrees) through a 4-stage pipeline—**Architect & Code → Verify & Test → Critic Review → Human Review**—with explicit diff Apply and guarded Undo.

## How It Pursues the Goal

1. **Deterministic Indexing & Context Policy**: Indexes repository files, symbols, and imports under `.mini-orca/index.json` while enforcing `.mini-orcaignore` and path exclusions before any prompt is constructed.
2. **Scoped Multi-Stage Analysis**: Runs resumable analysis (`analysis/run.json`) across configured model scopes (`analyze`, `bug`, `function`). Separates verified static findings from AI suggestions and preserves explicit remote-provider consent and security-review intent.
3. **4-Stage Verified Change & Agent Pipeline**:
   - **Step 1 · Architect & Code**: Generates a unified proposal or isolated worktree fix from findings, feature ideas, or chat instructions.
   - **Step 2 · Verify & Test**: Compiles and runs scoped package checks and pinned regression tests (`go test`, syntax/vet checks) with bounded self-repair (up to 3 repair attempts).
   - **Step 3 · Critic Review**: Evaluates the checked diff for regressions, boundary violations, and clean-code precision before human sign-off.
   - **Step 4 · Human Precision Gate**: Presents read-only unified/split diffs for explicit **Accept & Commit**, worktree **Merge** (closing grouped findings), or **Undo**.
4. **Dual GUI / TUI Parity**: Provides both a Wails/React graphical studio and a full-featured keyboard-driven Terminal UI (`TUI`) backed by the same workspace state and loopback API, enabling a seamless transition to a pure TUI workflow.

## Architecture

Mini-Orca enforces strict layered ownership across the Go daemon and the desktop client:

| Layer / Path | Responsibility |
| --- | --- |
| `cmd/daemon`, `cmd/eval` | Startup, configuration wiring, loopback server binding, and CLI evaluation entry points. |
| `internal/api` | Loopback HTTP decoding, browser-origin protection, local request policy, and structured error responses. |
| `internal/app` | Workflow orchestration, analysis runs, 4-stage change/agent sessions, Git worktree isolation, and guarded source mutation. |
| `internal/project` | Repository facts, symbol/import indexing, context composition, structured `ProjectDocsReport` synthesis, and `AGENTS.md` instructions. |
| `internal/storage` | Atomic durable file primitives under `.mini-orca/`; domain packages own their persisted schemas. |
| `internal/llm`, `internal/config` | Provider transport (OpenAI-compatible HTTP, `agy`, `pi`), timeouts, and 3-scope model configuration. |
| `desktop/` | Wails native bridge (`desktop/internal/bridge`) and React/TypeScript frontend (`desktop/frontend/src`) hosting both the GUI and TUI views. |

## Main Dependencies

| Dependency | Scope | Purpose |
| --- | --- | --- |
| **Go 1.22+** (`go.mod`, `gopkg.in/yaml.v3`) | Daemon & CLI | Core HTTP daemon, AST/symbol indexing, analysis orchestration, and atomic `.mini-orca/` storage. |
| **Wails v2** (`github.com/wailsapp/wails/v2`) | Desktop Host | Native macOS arm64 window host, loopback bridge, and local PTY terminal management (`github.com/creack/pty`). |
| **React 18 + TypeScript 5 + Vite 5** | Frontend (`desktop/frontend`) | Unified state machine (`workspace.ts`), GUI studio views, and keyboard-first Terminal UI (`tui.tsx`). |
| **Model Providers** (`HTTP` / `agy` / `pi`) | AI Transport | Pluggable local or remote LLM execution for `analyze`, `bug`, and `function` scopes. |
| **Playwright + Go Quality Gates** | Verification | Headless component/workflow verification (`make web-test`) and static/complexity/clone checks (`make quality`). |

## Other Project Info & Persistence

### Run Locally

Use Go 1.25+, Node.js 22+, and Xcode command-line tools for the macOS arm64 desktop application. Read [desktop setup](desktop/README.md#build-and-run) and [CONFIG.md](CONFIG.md) before the first launch.

```sh
cp config.example.yaml config.yaml
go run ./cmd/daemon
# In a second terminal:
make desktop-run
```

The daemon defaults to `127.0.0.1:9090`; the desktop connects to `http://127.0.0.1:9090` unless `MINI_ORCA_URL` is set. Keep `config.yaml` local. Every prompt-bearing request to a non-loopback HTTP or CLI provider requires scope-specific consent.

### Project Intelligence & Storage (`.mini-orca/`)

Project-local `.mini-orca/` stores `index.json`, `project-analysis.json`, `project-docs.json`, `file-analysis/`, `findings.json`, `performance/files/`, and unified progress in `analysis/run.json`. Private `changes/` files retain conversations, captured source and proposals, feature goals/ideas, the latest grouped recovery journal, and verification evidence. `.mini-orca/autopilot/` holds isolated Git worktrees for autonomous agent tasks. Restore uses local persisted analysis without triggering model requests. See the [API guide](docs/api-contract.md) and [workflow migration](docs/api-contract.md#retired-workflows) for current contracts and boundaries.

## Development and documentation

Follow [AGENTS.md](AGENTS.md). UI contributions follow the [UI interaction and accessibility guidelines](desktop/UI_DESIGN_GUIDELINES.md). Run focused tests while editing. The full local gate is:

```sh
./scripts/validate.sh
```

The checks use fakes and temporary directories, do not start the daemon, and do not require a provider, key, or project configuration.

- `make check`: Formatting, Go tests/races/vet, Python tooling, and desktop workflow/bridge tests.
- `make quality`: Pinned Go static, reachability, complexity, and clone checks.
- [Desktop setup and usage](desktop/README.md) · [UI guidelines](desktop/UI_DESIGN_GUIDELINES.md)
- [API guide](docs/api-contract.md) · [OpenAPI](docs/openapi.yaml) · [configuration](CONFIG.md)
- [Docker](DOCKER.md) · [release notes](RELEASE_NOTES.md) · [LICENSE](LICENSE)
