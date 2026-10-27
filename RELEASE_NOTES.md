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
  elsewhere keep it current and only mark its checks outdated.
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
