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
- **Analysis** selects files, previews request limits and provider destinations,
  and runs, pauses, resumes or cancels the captured queue. Each new admission
  requires its own remote-provider and Security intent confirmations.
- **Bugs**, **Performance** and **Security** show searchable results and their
  evidence. Details retain complete model prose, provenance and verification
  guidance. Findings and performance hypotheses keep their reported confidence.
- **Editor** shows selectable read-only source. Choose a declaration or create a
  new one, describe a change, and edit only the returned declaration/import draft.
  Validate, run checks, inspect Review, then explicitly Apply. The receipt offers
  guarded Undo. Local edits invalidate prior checks. Returning to the editor or
  application rechecks the source; changed files need **Refresh facts**.
- **Summary** previews saved architecture and project-flow charts. **Explore**
  opens the complete architecture and flows view. Plain Mermaid reports and
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
webview storage. It does not persist source, drafts, consent or terminal output.
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
