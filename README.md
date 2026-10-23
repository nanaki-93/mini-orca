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
| `2` | Work | Tasks, issues, features, task plans, analysis and file scope |
| `3` | Code | Read-only files and diffs, worktrees, saved proposals |
| `4` | Docs | Context, dependencies, components, decisions and architecture |
| `5` | Settings | Project trust, models, bootstrap checklist and lessons |

`Tab` / `Shift+Tab` cycle views within a workspace. Use `↑/↓` or `j/k` to
select rows, `PgUp/PgDn` to read long pages, and `Esc` to return from details.
The bottom bar shows actions for the current view. `?` opens the keyboard
reference, `p` changes the palette, and `Ctrl+C` quits.

Press `Ctrl+K` or `:` to search commands by name or type a command directly.
The input sits above the results and the selected command's explanation.
`↑/↓` selects a suggestion, `Tab` completes it, and `Enter` runs it. Commands
needing a path or request open an editable prompt first. Use `←/→`, `Home/End`
and `Delete` to edit; `Ctrl+P/N` recalls session history and `Ctrl+U` clears the input. `Esc`
cancels without running anything. Confirmation dialogs keep keyboard focus until
you confirm or cancel them.

Workspace areas have distinct accents, with filled panels and selection highlights
across Tokyo, CRT, Amber and Paper. Wide terminals use a sidebar with a compact
model summary; narrow terminals use navigation tabs. The dashboard adds count
cards and a next-step panel when space permits, prioritizing activity navigation
in smaller terminals. Frames fit the actual terminal size, with a resize prompt below
52 columns or 16 rows. A larger terminal is recommended for reviewing diffs.

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

In the diff, `PgUp/PgDn` scroll explanations while keeping code visible, `h/l` scroll long lines, `w` marks the current file reviewed, `f/F`
moves between files, `e` shows rationale and criterion references, and `z` compares
against the last complete review. `P` opens the plan. Apply shows its commit
destination and requires review of every changed file; changed proposals clear
review progress. `:retry test` reuses implementation, while `:retry review`
reuses current passing checks. Attempts, stage timings and failure reasons remain
visible in workflow details.

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
