# Mini-Orca release notes

## v5.0.0 — Web-first workspace

The browser workspace served by the daemon at `/ui` is now Mini-Orca's only
interface.

- **Removed:** the terminal UI (`cmd/tui`, `internal/tui`), the remote agent
  worker (`cmd/agent-worker`), and the features only the terminal UI exposed:
  missions, live sessions, cloud copies, remote agents and delegations, the
  inbound A2A listener, MCP and OpenAPI integrations with API actions, grounded
  research, ambient triggers, attachments, the artifact library and curated
  memory. Their routes return 404; the `integrations` section and
  `agent_runtime.live_model`/`triggers` keys now fail startup. See
  [retired workflows](docs/api-contract.md#retired-workflows).
- **Web UI:** a calmer visual system (inset content panel, breadcrumbs, aligned
  task columns), an Inbox summary, required-versus-optional check results in the
  review gate, explained merge availability on Branches, and accurate finding
  source labels. A Playwright suite (`make webui-e2e`) runs the real UI against a
  fake daemon.
- **Review:** you have the last word. Specialists separate blocking findings
  from advisory notes, a rejected version can be overridden with a reason
  (`POST …/review-override`, bound to its hash and approving nothing), and
  approval accepts notes kept with that approval (`review_notes`).
- **Freshness:** a task goes stale only when a file it depends on changes (its
  targets and read context, the rest of the target packages, build manifests
  above them; `.gitignore`d files never count), and it names that file. Changes
  elsewhere keep it current and only mark its checks outdated. Apply, Undo,
  merges, branch switches and reindexing stop only the running workflows whose
  files they change, and say which file; the task list now carries each task's
  paths.
- **Line comments:** comment on any line of a changed file; open comments go
  to the agents as one structured repair brief with the next revision or run
  (`include_comments`) and stay as history once sent.
- **Usage:** each workflow stage records what it spent, the task page shows
  per-stage and per-task tokens and estimated cost, and Settings → Usage lists
  spend per task (`GET /api/projects/current/usage`). Live events now name
  the task, stage, status and spend, refresh only that task, announce finished
  runs, and reindexing is published.
- **Quarantine:** an unreadable agent journal or task file is set aside
  (bytes kept beside it) instead of blocking the checkout; files from a newer
  Mini-Orca are refused or skipped with an upgrade remedy; journal errors name
  the file. `GET /api/projects/current/quarantine` and the Inbox report them.
- **Write token:** every request that changes state must carry the daemon's
  per-launch token (`X-Mini-Orca-Token`). The Web UI gets it in its document;
  other local clients read the `0600` token file (`MINI_ORCA_TOKEN_FILE`, or
  `mini-orca/daemon-token` in the user configuration directory).
- **Agent models:** Settings → Models & agents now edits the model each agent
  role runs with — a configured scope or any model the Agy or Pi CLI lists — and
  the budget per run, saved per project for new runs, with a reset to
  `config.yaml`. Roles accept catalog model IDs (`POST
  …/agent-settings/control`), and agent settings report `role_models`.
- **Fixed:** a daemon started by `mini-orca` from an install folder without a
  `config.yaml` silently used the local defaults (`localhost:1234`), so every
  analysis stage failed. The daemon now also reads `~/.config/mini-orca/` and
  the platform configuration directory (symlinked binaries resolved), `make
  install` seeds `~/.config/mini-orca/config.yaml`, the launcher prints the file
  it uses, and the status bar and Settings warn when there is none. A missing
  `MINI_ORCA_CONFIG` file now stops startup.
- **Faster triage:** Issues move with `j`/`k`, fix with `f`, dismiss or reopen
  with `x`, and filter by kind; the status bar shows a running analysis and the
  code inspector shows each file's analysis stages.
- **Dark palettes:** Midnight, Graphite, Ocean, Forest, Plum, Ember and True
  black, chosen in Settings → Appearance or the command palette.
- **Delete tasks:** stale, failed and draft tasks can be deleted one at a time
  or per group (`POST …/changes/{id}/delete`); running tasks and the task
  Undo still needs are kept.
- **Analysis dashboard** (`g a`): live progress of the last or running analysis,
  failed stages grouped by reason, coverage and findings per section, models
  used and every file's stages, with analyze, retry failed, re-analyze, pause,
  resume and cancel. Issues filter by category. An analysis window now runs up
  to 30 minutes before pausing.
- **Analysis failures say why:** a failed stage names an unreachable provider, a
  timeout, a rejected request or an unusable answer instead of one generic
  message.
- **Launcher:** `mini-orca` (built and installed by `just build`) opens the
  project in the current folder: it starts the daemon when none is running,
  switches it to the folder and opens the Web UI; Ctrl+C stops a daemon it
  started.
- **Fixed:** a daemon with no `config.yaml` failed to start instead of using
  local defaults.
- **Fixed:** unreadable saved analysis progress (for example from an older
  Mini-Orca) made the overview and every analysis preview fail with 400
  "Review the request and try again". It is now set aside like other
  unreadable records, and error messages in the Web UI include the daemon's
  detail.
- **Fixed:** a Go scan canceled while its workspace was being prepared is
  reported as canceled, not failed.
- Model capabilities report completion, JSON Schema, tools and streaming only.

## v4.4.0 — Focused local TUI and daemon workflow

Mini-Orca pairs a pure Go Terminal UI (`cmd/tui`) and loopback daemon (`cmd/daemon`)
with scoped model profiles, unified analysis, Security rules and advisory
Performance review. Chat prepares checked proposals across captured Go/Markdown
files with explicit Apply and guarded grouped Undo. Legacy desktop clients,
declaration editing APIs, draft benchmarks and separate legacy analysis jobs have
been removed. See [workflow migration](docs/api-contract.md#retired-workflows),
[README.md](README.md) for current usage and [CONFIG.md](CONFIG.md) for setup.
