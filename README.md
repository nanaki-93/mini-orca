# Mini-Orca

A local-first coding assistant with a pure Go **Terminal UI (TUI)** and a loopback Go daemon, built to analyze repositories, surface verified and semantic **Issues**, synthesize structured **Docs**, suggest goal-aligned **Features**, and prepare guarded **AI Changes** with explicit human review.

## Purpose of the Project

Mini-Orca helps engineers understand and improve local codebases without surrendering control over source mutations or provider boundaries:

- **Analyze projects to find Issues**: Coordinates deterministic static/security rules alongside scoped semantic, performance, and advisory security analysis.
- **Create structured Docs**: Synthesizes a language-agnostic engineering specification covering project purpose, how it pursues its goal, layered architecture, main dependencies, runtime scenarios, ADRs, and operational runbooks.
- **Suggest Features**: Generates goal-aware feature ideas grounded in the repository's indexed architecture and user-defined project goals.
- **Plan and review guarded changes**: Investigates repository evidence, prepares an editable plan, and runs a native ADK workflow across up to eight captured Go/Markdown paths per task. Generation, test generation, verification, specialist review and human input retain separate outcomes. Source changes require explicit diff Apply and guarded Undo.

## How It Pursues the Goal

1. **Deterministic Indexing & Context Policy**: Indexes repository files and imports under `.mini-orca/index.json` while enforcing `.mini-orcaignore` and path exclusions before any prompt is constructed.
2. **Scoped Multi-Stage Analysis**: Runs resumable analysis (`analysis/run.json`) across configured model scopes (`analyze`, `bug`, `code`). Separates verified static findings from AI suggestions and preserves explicit remote-provider consent and security-review intent.
3. **Native ADK Workflows with Host-Owned Guards**: Sequential change stages, parallel specialist reviews, conditional analysis and bounded repair loops share admitted budgets. Generation and test generation precede isolated checks; model advice and human approval remain separate. Repairs are off by default and can be explicitly enabled for at most two iterations. Review every changed file before **Apply**, then explicitly integrate a worktree or use guarded **Undo**.
4. **Keyboard-First Pure Go TUI**: Provides a responsive workspace (`cmd/tui`, `internal/tui`) backed by the loopback API (`127.0.0.1:9090`), with a project dashboard, searchable commands, arrow/Vim navigation, and 4 color palettes (`tokyo`, `crt`, `amber`, `paper`).

## Navigating the workspace

The dashboard shows issues, tasks, analysis coverage and verification status.
Use `↑/↓` to select an activity and `Enter` to open it. Checks and analysis that
have not run are labeled separately from successful results. Press `o` on the
dashboard or Repository page to enter a project path; paths may contain spaces.
Press `n` on the dashboard or Tasks page to plan a new task. On the dashboard,
`r` runs or resumes analysis; use `:reanalyze` for a fresh analysis.

| Key | Workspace | Contents |
| --- | --- | --- |
| `1` | Project | Dashboard, repository inventory, tests and check output |
| `2` | Work | Tasks and execution details, mission plans, issues, features, analysis, file scope and media |
| `3` | Code | Read-only files and diffs, worktrees, saved proposals and artifacts |
| `4` | Docs | Context, dependencies, components, decisions and architecture |
| `5` | Settings | Trust, models, roles, skills, connections, external work, bootstrap, lessons, memory, ambient analysis and cloud copies |

`Tab` / `Shift+Tab` cycle views within a workspace. Use `↑/↓` or `j/k` to
select rows, `PgUp/PgDn` to read long pages, and `Esc` to return from details.
`Backspace` or `Ctrl+O` goes back to the previous view with its selection, and
each view remembers the row you left it on. `:messages` shows recent status
updates and errors. The bottom bar shows actions for the current view. `?` opens
the keyboard reference, `p` changes the palette, and `Ctrl+C` quits. Set
`NO_COLOR=1` for a monochrome theme.

Long daemon actions never freeze the terminal: the status bar shows what is
running and for how long, keys you type are queued and replayed afterwards, and
`Esc` cancels read-only work such as opening a file. Apply, merge, Undo and trust
changes always run to completion.

Press `Ctrl+K` or `:` to search commands by name or type a command directly.
The input sits above the results and the selected command's explanation.
`↑/↓` selects a suggestion, `Tab` completes it, and `Enter` runs it. Commands
needing a path or request open an editable prompt first. Use `←/→`, `Home/End`
and `Delete` to edit; `Ctrl+P/N` recalls session history and `Ctrl+U` clears the input. `Esc`
cancels without running anything. Confirmation dialogs keep keyboard focus until
you confirm or cancel them.

`:roles` opens Roles & Budgets; `:skills` opens Skills. Settings initially use the
main project checkout. Use `:agent-settings task` to select the inspected task's
worktree, or `:agent-settings main` to switch back. `main` and `task` are fixed
checkout choices. Saved edits affect future runs in that checkout.

Workspace areas have distinct accents, with filled panels and selection highlights
across Tokyo, CRT, Amber and Paper. Wide terminals use a sidebar with a compact
model summary; narrow terminals use navigation tabs. The dashboard adds count
cards and a next-step panel when space permits, prioritizing activity navigation
in smaller terminals. Frames fit the actual terminal size, with a resize prompt below
52 columns or 16 rows. A larger terminal is recommended for reviewing diffs.

## Browser workspace

Run `:web` in the TUI to open the local browser workspace with the selected
checkout, proposal and file. Use `:web files`, `:web tasks`, `:web issues`,
`:web git` or `:web diff` to choose a view directly.

The browser workspace is a lightweight IDE served by the daemon at `/ui`:

| Key | Page | What it is for |
| --- | --- | --- |
| `1` | **Home** | The single next step, workspace counts, the active task pipeline and recent tasks. |
| `2` | **Explorer** | Folder tree with fuzzy filter, tabbed read-only source, issue markers, Explain and New task. |
| `3` | **Tasks** | Tasks grouped by what needs attention; the three-step **Describe → Choose files → Confirm** wizard. |
| `4` | **Review** | Changed-file tree with review state, split/unified diff with collapsed context, and the steps **Changes · Checks · Agents · Refine · Apply**. |
| `5` | **Issues** | Filters by status, category and severity; detail pane with evidence, insight and Create fix task. |
| `6` | **Docs** | The generated handbook: overview, architecture, components, scenarios, decisions, dependencies, runbook, glossary. |
| `7` | **Checks** | Go scan phases and project analysis coverage, with run, pause and cancel. |
| `8` | **Git** | Current checkout, agent worktrees, merge preview (incoming commits and diff) and guarded merge. |
| `9` | **Settings** | Theme, density, diff layout, checkout scope, execution trust and models. |

The **Agents** step starts the native ADK workflow (create → tests → specialist
reviews) after an explicit confirmation that lists every remote destination,
follows each stage live and records your approval for the exact proposal hash.
**Apply** can also commit the reviewed files through a private index. Apply,
Undo, trust, merge and remote model requests always ask for confirmation; Review
navigation, previews and filters never start model work, checks or writes.

`Ctrl+K` / `Cmd+K` searches commands, tasks and files (`>` commands only, `#`
tasks only); `Cmd+P` opens files; `Cmd+B` toggles the side panel; `c` starts a
new task; `?` lists every shortcut. In Review, `j`/`k` move between files, `n`
jumps to the next unreviewed file, `m` marks the current file reviewed, `v`
switches split/unified and `[`/`]` move between steps. Browser Back/Forward
restores pages, review steps and the selected file. Themes are System, Midnight,
Light, Tokyo Night and High contrast.

The UI is a set of embedded, no-build ES modules under
`internal/api/handlers/webui/` served with a strict Content-Security-Policy (no
inline scripts or styles). Run its tests with Node's built-in runner:
`make webui-test` (or `node --test internal/api/handlers/webui/test/*.test.mjs`).
They cover review authority, diff shaping, routing, fuzzy search and every
action boundary (what each operation may send, and when it must send nothing).

## Reviewing a task in the TUI

Selecting a fix opens **Task Plan**. In Features, `Enter` opens its details and
`f` prepares a plan. `←/→` or `h/l` switch focused detail sections; `Tab` / `Shift+Tab`
continue to switch workspace views. Edit the approach with `e`, write
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

Repository inventories, handbook pages, check logs and long error or confirmation
messages also scroll. Confirmation actions stay in the bottom bar. In the source
viewer, `PgUp/PgDn` moves through lines, `Home/End` jumps to the first/last line,
and `h/l` pans long lines (`0` resets the horizontal position).

In the diff, `PgUp/PgDn` scroll explanations while keeping code visible, `h/l` scroll long lines, `w` marks the current file reviewed and moves to the next unreviewed one, `f/F`
moves between files (wide terminals list them beside the diff), `e` shows rationale and criterion references, and `z` compares
against the last complete review. `P` opens the plan. Apply shows its commit
destination and requires review of every changed file; changed proposals clear
review progress. `:retry test` reuses implementation, while `:retry review`
reuses current passing checks. Attempts, stage timings and failure reasons remain
visible in workflow details.

`:investigate` requests scoped repository evidence for a draft; explicitly accept
the result before it changes the plan. `:execution` opens task details with
Overview, Timeline, Evidence, Usage and Recovery sections. `:run-output <ID>`
discloses one private invocation; `:hide-output` closes it. A waiting human input
can be approved, rejected or canceled with `:run-input <decision>`; recording the
decision does not apply files. Interrupted work requires fresh consent and checks.
Opening history never resumes a provider call.

Project execution trust is remembered for the same canonical project path across
reopening and reindexing. `:trust` toggles consent and saves revocation too.
Other checkouts and changed command scopes require their own confirmation.
Current revision and workspace checks still apply before execution.

## Merges and worktree recovery

Tasks start from the current branch. The TUI blocks generation when another
active task already owns a write path; finish or discard that task first. A saved
plan in an outdated worktree cannot start generation: refresh, use `:replan`, and
review the new plan. The old worktree remains available in **Worktrees**.

After review, **Apply** commits the fix in its isolated checkout and moves it from
Tasks to **Worktrees** as **TO_MERGE**. Its linked issues leave the issue lists
until you decide: `m` merges, marks the issues Done, and deletes the worktree and
its branch; `x` rejects the fix and returns the issues to TODO, retaining the
unmerged commits on their branch. Failed tasks remain in Tasks for inspection
and retry; matching the destination's Git HEAD does not make them integrated.

In **Worktrees**, `Enter` opens integration details. When the destination has
advanced, overlapping paths are a review hint, not proof of conflict. If a merge
conflicts, resolve it with your external Git tool and commit the merge. Press `R`
to update the project index. Saved project trust is restored automatically;
use `:trust` if the project is not yet trusted. Select the **INTEGRATED** worktree
and press `c` to complete linked findings and remove the clean checkout and merged
branch. Lowercase `r` only refreshes status; updating the index stops outdated
checks and workflows in the destination checkout.
For unfinished drafts or unmerged commits, `c` also offers explicit discard:
it stops tasks and removes saved plans/proposals and the checkout, retaining
committed work on its branch. Cancel keeps everything.
Local changes, locked worktrees and changed identities still block removal. Squash
merges and cherry-picks require review before discard because ancestry cannot
prove that the original task was integrated.

Worktrees stay visible until deletion succeeds. A failed merge shows the actual
conflict state, and a worktree cleanup warning can be retried through the same
action. If only branch deletion fails, the warning identifies the retained branch
to inspect and remove in Git; the merged issues stay Done.

## Architecture

Mini-Orca enforces strict layered ownership across the Go daemon and Go TUI:

| Layer / Path | Responsibility |
| --- | --- |
| `cmd/daemon`, `cmd/tui`, `cmd/engineering-insight-eval`, `cmd/agent-eval`, `cmd/agent-worker`, `cmd/agent-inspect` | Daemon, terminal UI, evaluation, isolated remote analysis and read-only local inspection entry points. |
| `internal/api` | Loopback HTTP decoding, browser-origin protection, local request policy, and structured error responses. |
| `internal/app` | Admission, native workflow hosts, analysis, mission scheduling, consent, Git isolation, and guarded source mutation. |
| `internal/app/agentruntime` | ADK Go agents, model adapters, graphs, tools, plugins and optional service adapters; the only package importing the SDK. |
| `internal/app/agentstate` | Private durable sessions, invocations, budgets, checkpoints, artifacts, curated memory and mission records. |
| `internal/project` | Repository facts, file/import indexing, context composition, structured `ProjectDocsReport` synthesis, and `AGENTS.md` instructions. |
| `internal/storage` | Atomic durable file primitives under `.mini-orca/`; domain packages own their persisted schemas. |
| `internal/llm`, `internal/config` | Compatible HTTP/CLI transport, provider capabilities, explicit Gemini/Vertex configuration, and three model scopes. |
| `internal/agenteval`, `internal/insighteval` | Synthetic agent task/trajectory evaluation and engineering-insight evaluation, separate from ordinary use. |
| `internal/tui` | Guarded daemon client, 5-hub TUI state machine, keyboard/colon-command handlers, and ANSI terminal renderer. |

`make build-agents` builds the worker, inspector and evaluation commands;
`make build-agents-linux` cross-builds them for Linux amd64 without CGO.
The worker requires an explicit start and a pinned source manifest. The inspector
reads local invocation history without constructing a running service. See
[agent deployment and inspection](docs/agent-deployment.md) for commands,
authentication, privacy, bounds and recovery behavior.

## Main Dependencies

| Dependency | Scope | Purpose |
| --- | --- | --- |
| **Go 1.27.3+** (`go.mod`, `gopkg.in/yaml.v3`) | Daemon, TUI & CLI | Core HTTP daemon, terminal UI, file/import indexing, analysis orchestration, and atomic `.mini-orca/` storage. |
| **Google ADK Go v2.6.0 / GenAI Go v1.74.0** | Agent runtime | Typed model/tool events, native graphs, sessions and capability-gated Google services. |
| **Model Providers** (compatible HTTP, Gemini, Vertex, `agy`, `pi`) | AI Transport | Explicit local or remote execution for `analyze`, `bug`, and `code`; CLI profiles remain completion-only. |
| **Go Quality & Race Gates** | Verification | Deterministic TUI/daemon verification (`make check`) and static/reachability/complexity/clone checks (`make quality`). |

## Supported ADK capabilities

Capabilities are admitted for the configured provider and exact operation. No
provider fallback occurs. See [configuration and limits](CONFIG.md), the
[API contract](docs/api-contract.md), and [TUI interactions](docs/UI_DESIGN_GUIDELINES.md).

| Capability | Delivered behavior and boundary |
| --- | --- |
| Models and streaming | Compatible HTTP and native Gemini/Vertex adapters preserve tool identities, schemas, cancellation and usage. CLI profiles provide completion only. Partial output never becomes an approved proposal. |
| Repository tools | Planning and file investigations use captured snippets, imports, search and impact evidence. Tools cannot expand their admitted scope or write source. |
| Workflow graphs | Sequential stages, parallel specialist reviews, conditional branches, bounded repairs and durable human-input checkpoints run through ADK with host-owned checks and budgets. |
| Sessions and artifacts | Local private journals and immutable versioned evidence support explicit inspection/export and guarded deletion. Restart retains history and charges, not execution authority. |
| Memory, skills and context | Curated memory requires acceptance and current provenance; indexed ADK skill packages load within admitted scope. Compaction and context caching are optional; remote cache retention requires consent. |
| External tools and agents | Explicit MCP connections, guarded OpenAPI adapters, A2A delegation and an optional authenticated A2A server. Discovery and source-sharing grants are separate from tool or execution permission. |
| Grounding and media | Explicit grounded research and attachment advice use eligible native Google profiles. Bounded live advice requires an explicitly configured supported Gemini live model; it captures no microphone or screen automatically. |
| Missions | Reviewed plans allocate fixed budgets to bounded children in isolated worktrees. Checked children integrate in the reviewed order with fresh base checks, explicit approval and retained recovery receipts. |
| Ambient analysis | Off until an explicit watch selection and bounded grant. Debounce, deduplication, deadlines and durable charges apply; restart never silently restores a grant. |
| Cloud copies | Optional Vertex session archives and GCS artifact copies require exact preview/confirmation, explicit credentials and durable transfer receipts. Local state remains authoritative; uncertain outcomes require explicit recovery. |
| Observability | Local source-free traces and separate reported/estimated usage, optional versioned cost estimates, and explicitly confirmed telemetry export. Missing usage and unknown cost stay unknown. |
| Evaluation and deployment | Frozen synthetic replay corpus, opt-in live evaluation, authenticated source-manifest worker and a read-only local inspector. See [deployment](docs/agent-deployment.md). |

Vertex Memory Bank is unavailable with the pinned Go adapter because the required
explicit credential injection and deletion controls are absent. Cloud archives
are copies, not remote resume or shared approval state. Unrestricted shell tools,
implicit credentials, automatic source writes and automatic merges are outside
the supported execution boundary.

Conformance and integration tests use fake providers and temporary projects.
They establish transport, lifecycle and guard behavior; they do not establish
live model quality, cloud IAM or billing savings. No matched live baseline is
recorded, so no quality, speed or cost improvement is claimed. Container execution
remains unverified where the Docker engine is unavailable.

## Run Locally

Use Go 1.27.3 or newer. Development and container builds pin 1.27.3; CI reads
the version from `go.mod`. Go can download the required toolchain automatically
when `GOTOOLCHAIN=auto` is enabled. On macOS, Go 1.27 requires macOS 13 or newer.

Read [CONFIG.md](CONFIG.md) before the first launch.

```sh
cp config.example.yaml config.yaml
just tui
# Or run daemon and TUI separately:
go run ./cmd/daemon
go run ./cmd/tui
```

The daemon defaults to `127.0.0.1:9090`; the TUI connects to `http://127.0.0.1:9090` unless `MINI_ORCA_URL` or `-url` is set. Keep `config.yaml` local. Every prompt-bearing request to a non-loopback HTTP or CLI provider requires scope-specific consent.

<a id="project-intelligence"></a>

### Project Intelligence & Storage (`.mini-orca/`)

Project-local `.mini-orca/` stores `index.json`, `project-analysis.json`, `project-docs.json`, `file-analysis/`, `findings.json`, `performance/files/`, and unified progress in `analysis/run.json`. Private `changes/` files retain conversations, captured source and proposals, feature goals/ideas, the latest grouped recovery journal, and verification evidence. `.mini-orca/autopilot/` holds isolated Git worktrees for autonomous agent tasks. Restore uses local persisted analysis without triggering model requests. See the [API guide](docs/api-contract.md) and [workflow migration](docs/api-contract.md#retired-workflows) for current contracts and boundaries.

## Development and documentation

The native Go agent evaluation harness runs a fixed, versioned synthetic corpus
through the production ADK runner and guarded repository tools:

```sh
go run ./cmd/agent-eval
go run ./cmd/agent-eval --case addition
```

The default replay mode uses an owned loopback fake provider. It reads no
workspace files and writes no proposals or runtime stores. Four task cases cover
arithmetic, comparisons and a bounded repair; five negative controls verify
scope violations, unsupported check claims, wrong proposals, incomplete output
and cancellation. The harness independently checks proposed Go return expressions
against held-out inputs using Go's constant evaluator. It never executes generated
code, arbitrary shell commands or project tests. This small corpus measures these
specific behaviors; it is not evidence of general coding quality.

JSON receipts on stdout identify corpus version/digest, prompt and scoring
versions, linked ADK/Go/build identity, hashed model/profile, budgets, tool
trajectory correctness, scope violations, unsupported claims, regression results,
repairs, token charges/reported usage and elapsed time. Missing usage stays
unknown; canceled/incomplete latency is labeled and unrun cases have no invented
measurements. Replay token usage is simulated and timing measures the local
harness. A replay campaign passes when task cases pass and negative controls are
detected; control cases retain their actual failed/incomplete/canceled states.
Receipts omit source, paths, prompts, model answers, credentials and raw errors.
Unavailable build metadata remains labeled `unavailable`.

Live mode sends only the embedded synthetic corpus to the configured **code**
profile and excludes negative controls. It requires an explicit command and
campaign limits. Omitting `--confirm-live` prints the source/destination disclosure
without sending a request:

```sh
go run ./cmd/agent-eval --mode live --config config.yaml --case addition \
  --requests 8 --input-tokens 65536 --output-tokens 8192 --confirm-live
```

Live evaluation uses one shared campaign budget, no provider fallback or hidden
retry, at most two admitted repairs, and a 60-second campaign deadline. It
supports configured HTTP providers with native tool support; CLI providers are
unavailable for this harness. The selected profile must allow 1–4096 output
tokens per call. Exit status is `0` for a passed campaign, `1` for a
failed/incomplete campaign with a receipt, and `2` for invalid configuration or
receipt output failure. Ctrl-C stops requests and emits an incomplete receipt.
No live evaluation is run by tests or CI.

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
