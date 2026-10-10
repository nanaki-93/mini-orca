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

Project Studio has twelve destinations. **Overview**, **Source** and **Chat** are
the main workspaces. **Improve** groups **Analysis**, **Findings**, **Features** and
**Changes**; project tools include architecture, instructions, context, models and
history. **Models** and **History** stay at the bottom of the sidebar. The project
switcher and global search (`⌘K` or `Ctrl+K`) open focused dialogs over the current
workspace. Escape closes them and restores focus. **Terminal** opens a bottom
drawer from the status bar while keeping the workspace visible. The sidebar
collapses to icons.

- **Project** opens a saved project locally. **Import & analyze** creates a new
  overview, with explicit confirmation for a remote Analyze provider.
- **Analysis** shows four analysis operations and their selected models.
  **Manage models** opens **Models**, where independent choices for Bug analysis,
  Performance, Security and feature discovery share the searchable model picker.
  The catalog includes configured models and models available through the local
  Pi installation. **Refresh** reloads it; Pi discovery failures leave configured
  choices available. **Back to workspace** returns to the previous task or finding.
  **Context → Files & scope** manages captured file selection, exclusions and
  per-file status. **Analysis** keeps Setup and Last run together.
  Unsaved selection and setup choices survive navigation between these pages;
  save file selections before preparing a run. Request limits remain in a disclosure;
  new runs default to a 1,800-second (30-minute) file-analysis budget per batch.
  Continuations retain the saved run's limits.
  When a run is paused or interrupted, **Prepare continuation** on Analysis
  prepares the saved run with its captured settings and fresh consent. To use
  new settings, cancel the saved run in **Last run**, then prepare a new analysis.
  Preparing a run opens a preview dialog with captured scope and provider destinations. Starting it still
  confirms remote-provider sharing and Security review for that admission.
  **Analyze stale files** is available only in Analysis and refreshes outdated
  files while preserving unchanged results and the saved file selection.
  **Last run** focuses on current progress and pause, continue or cancel controls.
  The progress bar and percentage count finished file-analysis steps, including
  unsuccessful or skipped steps; feature discovery has its own status.
  **Run details** reveals result counts, captured models, coverage, provider
  destinations and file diagnostics. Failures remain visible when details are
  collapsed. Compact result rows open each category without losing run progress.
  Pausing/resuming retains captured model choices, including older runs that
  shared one Performance/Security choice. Feature discovery runs independently
  of file analysis, gets at least ten minutes (or the selected model's longer
  timeout), honors file exclusions and reuses completed ideas on resume.
  Failed feature discovery leaves other analysis results available.
- **Findings** groups **Bugs**, **Performance** and **Security** in category tabs. Each finding has a short content-based name, shared by its list row,
  detail heading and prepared fix. Older unnamed findings use a compact excerpt
  of their saved explanation. Selecting a finding opens its detail beneath the list, preserving filters and
  the selected row. Its name, category, exact path, line and declaration remain
  visible when available. The full cause leads the page, followed
  by the proposed solution, affected files and **Prepare fix** action. Provenance and confidence stay with the cause;
  **Details** contains additional evidence and triage actions, including
  **Mark as fixed**. **Full explanation** expands the solution in place without
  repeating the preview; **Show less** collapses it again.
  Creation, testing and review choices are shared with Chat and configured only
  on **Models**. **Manage models** preserves the selected finding so **Back to
  workspace** returns to its detail. Model destinations remain visible in the pickers.
  **Permissions & checks** expands sharing permissions, project
  check commands and preparation refresh. **Prepare fix** authorizes generation,
  isolated checks, remote sharing and Security intent when applicable. Changed destinations
  or project revisions require fresh preparation.
  Go fixes include a regression test path; Markdown-only fixes use the selected
  creation profile. **Go to file** opens source without starting a fix.
  Stale finding details offer **Analyze stale file**, which prepares analysis only
  for that finding's file. Preview and Start keep that target, and results for
  other files remain available. Freshness follows content and applicable guidance.
  The generated fix opens on **Changes**, with file tabs, a read-only diff and
  a check-evidence pane. **Checks** contains full diagnostics, captured workflow
  stages and model review. **Details** contains the cause, solution, scope,
  instructions and regeneration, with a link to **History**.
  **Review next file** advances through the diffs; **Apply N files** becomes available
  after every changed file has been viewed and the existing check/review guards pass.
  Regeneration clears the viewed-file state. Tabs and file selection never generate,
  check or apply a proposal. Failures and requested corrections stay visible above
  the diff. Performance remains unmeasured without benchmark evidence.
  Apply records review for that exact revision and retains post-Apply verification
  and guarded Undo. It does not automatically mark the finding fixed.
- **Features** presents advisory ideas as compact rows. Selecting a row opens
  the complete idea in a dialog; **Project goals** expands goal editing and search. Searches
  use the same extended deadline as feature discovery in Analysis. New ideas
  accumulate alongside existing ones, retaining prior triage decisions, and
  duplicates are skipped. Ideas generated from previous
  source become stale when the project changes and remain available for triage.
  A new search adds current ideas without revalidating older ones; only current
  ideas can be discussed in Chat. Opening an idea does not generate code.
- **Chat** captures up to eight explicit Go/Markdown paths, including new files.
  **Configure workflow** on a feature idea
  opens the captured task setup and suggests a test path. Choose a Creation,
  Testing and Review model independently in **Workflow models** on the **Models**
  page, then return to Chat and select **Generate changes**. **Manage models** opens
  that page without losing the task title, file scope, request or test choice.
  New conversations clear the previous draft. These workflows use configured
  profiles; Analysis also offers Pi catalog models. The choices reference the
  daemon's configured Function, Bug and Analyze profiles
  (the respective defaults); configure their actual providers/models in `config.yaml`.
  Include a `_test.go` path. The testing agent writes tests, then the daemon runs
  them in an isolated copy before the review agent evaluates the proposal.
  The app shows each stage, captured models, check evidence and review findings;
  progress remains visible during navigation, and **Cancel workflow** stops work.
  Failed tests or requested changes block approval. A successful run displays read-only diffs and waits for explicit **Apply**. Performance remains unmeasured unless
  actual benchmark evidence is available. Interrupted runs need an explicit new
  run; the workflow never resumes provider calls from history automatically.
  Describe a task here, or use the dedicated fix workflow from a finding. Generation checks the proposal and can repair failed checks up to three times.
  Chat centers the conversation, with compact task metadata, expandable scope
  and a composer at the bottom. A compact proposal card links to review.
  **Review changes** opens **Changes**, with a read-only diff beside check evidence
  on wide windows. Compact windows stack these panes. **Back to conversation** returns to Chat.
  View every changed file, then select **Apply N files** in the review footer. This records
  review and applies only that revision; failed review, a replacement proposal or
  navigation while review is pending prevents Apply. With a captured `_test.go`
  path, creation, testing and agent review run automatically; otherwise generation
  uses the direct checked-proposal path.
  Requested tests remain required; execution trust and remote-provider consent
  are separate. New revisions clear earlier review. **History** lists saved work;
  **Resume** restores a conversation without generation or inherited check/review
  authority.
- **Instructions** opens as a readable guide. **Edit draft** switches to inline
  editing; **Load scope** loads root or directory AGENTS.md. **Add project guidelines**
  expands independent rules
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
  opens a manual proposal in **Changes** with the same file review and explicit **Apply** action.
  Agents use applicable root and directory guidance by default for analysis,
  explanations, suggestions, creation, testing and review, honoring context exclusions.
  Fix and feature implementation pages show instruction status; **Create AGENTS.md** opens the
  root instruction editor when no root guide exists. Guidance changes invalidate
  affected AI reports and proposals; navigation alone never requests a model.
- After Apply, **Verify applied change** checks the applied file identities and
  runs Go tests/vet in a copied workspace with fresh execution trust. Markdown-only
  verification checks text and hashes. **Reanalyze changed files** requests fresh
  source suggestions using separate provider consent. Passing checks do not
  certify acceptance criteria or automatically mark original findings fixed.
  The latest unchanged grouped proposal offers **Undo proposal**; interrupted
  writes expose recovery state and guarded restoration.
- **Source** combines a folder explorer, open-file tabs and a selectable,
  read-only source canvas with an assistant pane. **Inspect this file** expands
  declaration navigation, explanations and file analysis. Asking about a file
  seeds Chat with that file and the question without making a provider request.
  **Draft change in Chat** captures the selected file and declaration in a new task;
  it does not generate or write code. Chat produces checked proposals for the
  **Changes** workspace, where review, Apply and guarded Undo remain explicit.
  Explanations and file analysis remain available in Source. The separate
  Assistant, declaration draft, Checks, Review, receipt and draft-benchmark screens
  have been retired from this client. Returning to Source rechecks file freshness.
- **Overview** puts the current task or saved analysis in a **Continue work** or
  **Your next step** band. Opening captured work does not generate or apply changes.
  **Needs attention** links Bugs, Performance, Security and active Features counts to
  their workspaces. Feature details, estimated effort and triage stay in Features;
  unavailable or ungenerated counts remain unknown, and status indicators retain
  stale/failure states. **About this project** expands project facts, coverage and background.
  **Project activity** links **Architecture and Flow**, the
  current **Project Analysis**, and recent saved tasks; without a run, analysis
  opens setup. **Architecture** keeps architecture, entry points and project-flow
  charts together. Plain Mermaid reports and
  Markdown-fenced charts render locally, with selectable source and a readable
  fallback for older prose or render failures.
- **Context** owns analysis file selection and exclusions. Its **Provider context**
  view shows the selected source file’s included/excluded context, hashes and
  provider destination.
  **Models** selects analysis models and shared creation, testing and review
  profiles, and reads
  configuration without probing a provider. Active workflows keep their captured
  models, and model choices are disabled until they finish or are canceled.
  Model-supplied links, HTML and remote images are inert.
- **Terminal** starts a shell only after an explicit action. Tabs keep running
  while hidden. Closing tabs, switching projects and exiting the app clean up
  their owned processes. A cleanup failure remains visible and blocks switching
  or closing. Terminal use never grants execution trust for checks or benchmarks.

The interface remembers the last project path, appearance, text size and sidebar
width in webview storage. Chat drafts remain in memory during navigation.
Private project-local `.mini-orca/changes/` metadata stores
conversations, source captures/proposals, goals, ideas and grouped recovery/check
evidence. Consent and execution trust are not restored. History lists return
summaries; chosen sessions expose their complete contents. Bounds are eight paths,
256 KiB context/response, forty messages, 2 MiB per conversation/recovery journal
and two hundred conversations. Go/Markdown proposals cannot delete files.
Performance changes remain unmeasured until an explicit benchmark comparison.
Restoring a project never starts a provider request or shell. **⌘K / Ctrl+K** opens
file and command search. Actions have visible focus and descriptive labels.

Use **Collapse sidebar** below the navigation to keep only destination icons.
Every destination keeps its accessible name and hover label; **Expand sidebar**
restores labels and counts. The selected width is remembered across launches.

The footer's **G**, **P** and **M** controls select Graphite, Porcelain and Midnight.
The selected theme is remembered across launches. Graphite pairs warm charcoal
with amber, Porcelain pairs ivory with forest green, and Midnight pairs navy with
cyan. All three retain labeled success, warning and failure states. **Aa** enables
larger text. Theme changes are immediate; hover transitions respect the system's
reduced-motion preference.

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
