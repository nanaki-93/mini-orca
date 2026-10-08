# Desktop web client

Mini-Orca's desktop web interface uses React and Wails, with embedded assets and
the existing Go daemon. The supported native target is macOS 13+ on arm64. The Compose
client remains available through `make compose-desktop-run`.

## Build and run

Install Node.js 22+, npm, Go 1.25+ and the Xcode command-line tools. This directory
is an independent Go module; the daemon still targets Go 1.22. The build resolves
the pinned Wails CLI without requiring a global installation. Go's normal
toolchain selection applies when entering this module.

From the repository root:

```sh
go run ./cmd/daemon
# In a second terminal:
make desktop-run
```

`make desktop-build` (also `make web-build`) writes
`desktop/webapp/build/bin/Mini-Orca.app`, with the canonical project release
version and the desktop icon generated from maintained sources. The application can also be
launched directly from that bundle after starting the daemon. `MINI_ORCA_URL`
selects its daemon origin; the default is `http://127.0.0.1:9090`. Provider settings
and credentials stay in the daemon's local `config.yaml`.

`make web-preview` serves the production assets at `http://127.0.0.1:4392` for
layout work. Browser previews cannot open projects or execute native actions.
There is no browser-accessible proxy, CORS exception or command-execution endpoint.

## Workflow

- **Project** opens a saved project locally. **Import & analyze** creates a new
  overview, with explicit confirmation for a remote Analyze provider.
- **Analysis** selects files and available models for Bug analysis, Performance/Security
  and feature discovery, previews request limits and provider destinations,
  and runs, pauses, resumes or cancels the captured queue. Each new admission
  requires its own start confirmation for remote-provider sharing and Security
  review. **Analysis setup** combines model assignments, request limits and Last run.
  Each operation opens a searchable model picker with model icons and provider/location
  metadata. All models, Pi, Local and Configured filters do not assign jobs to models.
  The catalog combines configured models with every model available through the local
  Pi installation, including custom local servers. **Refresh models** reloads the catalog;
  Pi discovery errors leave configured choices available and explain how to retry.
  Defaults remain Bug for bug analysis and Analyze for reviews/features. Performance
  and Security share one model choice. Last run shows captured model names and icons,
  including older runs with provider details but no explicit model choices. View run
  and the preview retain destination details. Choices remain fixed when resuming.
  It also generates one project-wide set of feature suggestions using saved
  goals and policy-filtered context, honoring file exclusions. Its progress and
  request allowance stay separate from finding counts; completed ideas are reused
  on resume. Feature discovery and file analysis run independently, so a slow
  search cannot delay file results. Discovery gets at least ten minutes, or the
  selected feature model's timeout when longer; file batches keep their own time limit.
  The run shows both active steps, elapsed time and feature attempt allowance. A failed feature step leaves
  other analysis results available.
- **Bugs**, **Performance** and **Security** show searchable results and their
  evidence. Details retain complete model prose, provenance and verification
  guidance. Findings and performance hypotheses keep their reported confidence.
- **Features** uses project goals to suggest advisory new capabilities. Searches
  use the same extended deadline as feature discovery in Analysis. New ideas
  accumulate alongside existing ones, retaining prior triage decisions, and
  duplicates are skipped. Ideas generated from previous
  source become stale when the project changes and remain available for triage.
  A new search adds current ideas without revalidating older ones; only current
  ideas can be discussed in Chat. Opening an idea does not generate code.
- **Chat** captures up to eight explicit Go/Markdown paths, including new files.
  **Configure workflow** on a feature idea or Bug/Performance/Security finding
  opens the captured task setup and suggests a test path. Choose a Creation,
  Testing and Review model independently, then select **Run workflow**. The
  choices reference the daemon's configured Function, Bug and Analyze profiles
  (the respective defaults); configure their actual providers/models in `config.yaml`.
  Include a `_test.go` path. The testing agent writes tests, then the daemon runs
  them in an isolated copy before the review agent evaluates the proposal.
  The app shows each stage, captured models, check evidence and review findings;
  progress remains visible during navigation, and **Cancel workflow** stops work.
  Failed tests or requested changes block approval. A successful run waits for
  human diff review and explicit Apply. Performance remains unmeasured unless
  actual benchmark evidence is available. Interrupted runs need an explicit new
  run; the workflow never resumes provider calls from history automatically.
  Describe a task or use **Prepare fix** on a finding. Preparation generates and
  checks a proposal, with at most three repairs of failed checks. Read every diff,
  select **Review this diff**, then **Approve and apply** and confirm its scope.
  Requested tests remain required; execution trust and remote-provider consent
  are separate. New revisions clear earlier review. Local history restores a
  conversation without generation or inherited check/review authority.
- **Instructions** loads root or directory AGENTS.md and offers independent rules
  across eight sections following [AGENTS.md conventions](https://agents.md/):
  architecture, setup/build, code style, testing, security/data, UI/accessibility,
  documentation and handoff. The catalog matches indexed languages, build manifests
  and supported imports in that scope, alongside common project rules. Each match
  shows its reason, example paths and the exact text to add. Search the choices or
  filter by section; selections persist across filters and can be cleared together.
  Reindex after project changes to refresh matches. **Add selected guidance** groups
  chosen rules under Markdown section headings in the editable draft, reusing
  matching headings without overwriting custom text. Text already in the draft or
  inherited guides is disabled to avoid duplicates.
  Existing files are registered by reading them; inherited guides show their
  origins and scope. **Preview instruction diff**
  creates a manual proposal in Chat with the same explicit Review/Apply workflow.
- After Apply, **Verify applied change** checks the applied file identities and
  runs Go tests/vet in a copied workspace with fresh execution trust. Markdown-only
  verification checks text and hashes. **Reanalyze changed files** requests fresh
  source suggestions using separate provider consent. Passing checks do not
  certify acceptance criteria or automatically mark original findings fixed.
  The latest unchanged grouped proposal offers **Undo proposal**; interrupted
  writes expose recovery state and guarded restoration.
- **Source** shows selectable read-only source. Choose a declaration or create a
  new one, describe a change, and edit only the returned declaration/import draft.
  Validate, run checks, inspect Review, then explicitly Apply. The receipt offers
  guarded Undo. Local edits invalidate prior checks. Returning to the editor or
  application rechecks the source; changed files need **Refresh facts**.
- **Summary** links Bugs, Performance, Security and active Features counts to
  their workspaces. Feature details, estimated effort and triage stay in Features;
  unavailable or ungenerated counts remain unknown, and status indicators retain
  stale/failure states. Matching **Architecture and Flow** and **Project Analysis**
  cards open saved diagrams and the current analysis run; without a run, the
  analysis card opens setup. The diagrams page keeps architecture, entry points
  and project-flow charts together. Plain Mermaid reports and
  Markdown-fenced charts render locally, with selectable source and a readable
  fallback for older prose or render failures.
- **Context** shows included/excluded files, hashes and the provider destination.
  **Models** reads configuration without probing a provider. Model-supplied links,
  HTML and remote images are inert.
- **Terminal** starts a shell only after an explicit action. Tabs keep running
  while hidden. Closing tabs, switching projects and exiting the app clean up
  their owned processes. A cleanup failure remains visible and blocks switching
  or closing. Terminal use never grants execution trust for checks or benchmarks.

The interface remembers only the last project path, appearance and text size in
webview storage. Private project-local `.mini-orca/changes/` metadata stores
conversations, source captures/proposals, goals, ideas and grouped recovery/check
evidence. Consent and execution trust are not restored. History lists return
summaries; chosen sessions expose their complete contents. Bounds are eight paths,
256 KiB context/response, forty messages, 2 MiB per conversation/recovery journal
and two hundred conversations. Go/Markdown proposals cannot delete files.
Performance changes remain unmeasured until an explicit benchmark comparison.
Restoring a project never starts a provider request or shell. **⌘K / Ctrl+K** opens
file and command search. Actions have visible focus and descriptive labels.

## Implementation and checks

`frontend/src/workspace.ts` owns immutable workflow snapshots, consent and request
lifetimes. The native bridge allowlists daemon methods/routes, bounds bodies and
responses, preserves structured failures, disallows redirects, and propagates
cancellation. Apply and Undo send guarded identities rather than source text.
If an Apply response is lost, the client blocks repeat writes until the project
is refreshed. The daemon remains the owner of all source eligibility rules.

The terminal uses creack/pty and xterm.js. Shells run with a canonical project
directory and absolute supported shell path. Its in-memory output buffer is
bounded to 1 MiB per session; xterm keeps 5,000 scrollback lines. Overflow is
reported. A stalled input write stops its session. Cleanup captures descendants,
checks process birth identities and keeps the PTY owned until cleanup completes.

```sh
make web-test
./scripts/validate.sh
```

The web gate checks formatting and types, builds production assets, exercises browser workflows
and compact/large-text layouts, and runs native transport and PTY race tests plus
Go vet. Browser fixtures live only in `frontend/tests/`. Screenshots are generated
under ignored `frontend/test-results/`. macOS tests use installed Chrome by
default; set `MINI_ORCA_TEST_BROWSER` to a Playwright browser channel to override.
Linux CI installs Playwright Chromium using `npx playwright install --with-deps chromium`.
These checks do not require a provider, live daemon, or project credentials.

Native packaging is a separate `make web-build` gate. Browser tests do not prove
OS focus behavior, native dialogs or screen-reader integration. Windows and Linux
native packaging are not supported by the build helper.
