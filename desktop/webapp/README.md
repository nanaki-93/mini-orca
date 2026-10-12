# Desktop web client

Mini-Orca's desktop web interface uses React and Wails, with embedded assets and
the Go daemon. The supported native target is macOS 13+ on arm64.

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
version and the Graphite app icon from [the logo pack](../../docs/icons/README.md).
Wails generates the macOS icon bundle from its 1024-pixel PNG. The host also sets
its Dock icon from the embedded PNG at startup, avoiding stale bundle-icon caches
after local rebuilds. `make desktop-run`
launches the app bundle through macOS so its Dock icon is registered, inherits
the shell environment, and waits until the app closes. The application can also be
launched directly from that bundle after starting the daemon. `MINI_ORCA_URL`
selects its daemon origin; the default is `http://127.0.0.1:9090`. Provider settings
and credentials stay in the daemon's local `config.yaml`.

`make web-preview` serves the production assets at `http://127.0.0.1:4392` for
layout work. Browser previews cannot open projects or execute native actions.
There is no browser-accessible proxy, CORS exception or command-execution endpoint.

## Workflow

Project Studio has thirteen destinations. **Overview**, **Source** and **Chat** are
the main workspaces. **Improve** groups **Analysis**, **Findings**, **Features** and
**Changes**; project tools include Git / Worktrees, architecture, instructions,
context, models and history. **Models** and **History** stay at the bottom of the
sidebar. The project switcher and global search (`⌘K` or `Ctrl+K`) open focused
dialogs over the current workspace. Escape closes them and restores focus.
**Terminal** opens a bottom drawer from the status bar while keeping the workspace
visible. The sidebar collapses to icons.

- **Overview** keeps findings and the next action visible. **Project details**
  expands the description, path and counts. **Tests** expands each check phase,
  command, status and output; active or failed tests open automatically.
  **Run Tests** beside the top-bar search starts the checks with explicit project
  execution trust; it becomes **Cancel Tests** while they run. Viewing the results
  never executes project code.
- **Project** opens a saved project locally. **Import & analyze** creates a new
  overview, with explicit confirmation for a remote Analyze provider.
- **Analysis** shows four analysis operations and their selected models.
  **Models** in the sidebar provides independent choices for Bug analysis,
  Performance, Security and feature discovery in a searchable model picker.
  The catalog includes configured models and models available through the local
  Pi installation. **Refresh** reloads it; Pi discovery failures leave configured
  choices available. **Back to workspace** returns to the previous task or finding.
  **Context → Files & scope** manages captured file selection, exclusions and
  per-file status. **Analysis** keeps Setup and Last run together.
  Unsaved selection and setup choices survive navigation between these pages;
  save file selections before starting a run. Batch size, time budget, attempts and
  refresh options are under **Advanced settings** above the operation summaries. **Refresh** and
  **Execute analysis** sit at the top of the page. New runs default to a 1,800-second
  (30-minute) file-analysis budget per batch.
  Continuations retain the saved run's limits.
  When a run is paused or interrupted, **Execute analysis** in Setup or **Last run**
  resumes it immediately with its captured settings and fresh admission. To use
  new settings, cancel the saved run in **Last run**, then execute a new analysis.
  **Execute analysis** starts directly without confirmation dialogs, including AI Security
  review and feature discovery; its inline note explains remote sharing.
  A fresh daemon preview still guards each admission.
  **Preview analysis** optionally shows captured scope, request bounds and provider destinations
  before a separately confirmed start.
  **Analyze stale files** is available only in Analysis and refreshes outdated
  files while preserving unchanged results and the saved file selection.
  **Last run** focuses on current progress and pause, continue or cancel controls.
  The progress bar and percentage count finished file-analysis steps, including
  unsuccessful or skipped steps; feature discovery has its own status.
  **Run details** stays visible with result counts, captured models and coverage.
  **Files with failures** lists only files with a failed analysis stage; expand a
  file for its stage diagnostics. Compact result rows open each category without losing run progress.
  Pausing/resuming retains captured model choices, including older runs that
  shared one Performance/Security choice. Feature discovery runs independently
  of file analysis, gets at least ten minutes (or the selected model's longer
  timeout), honors file exclusions and reuses completed ideas on resume.
  Failed feature discovery leaves other analysis results available.
- **Findings** groups **Bugs**, **Performance** and **Security** in category tabs. Each finding has a short content-based name, shared by its list row,
  detail heading and prepared fix. Older unnamed findings use a compact excerpt
  of their saved explanation. All three categories share the same list layout with
  search, priority filtering and ten findings per page. Priority arrows and colored
  freshness indicators include tooltips and accessible names. Numbered pages and
  previous/next arrows show the current page and page count. Search supports
  clearing with Escape; **Reset filters** clears search and priority together.
  Changing a filter returns to the first page.
  Report diagnostics and unclassified suggestions are in **Analysis details**
  above the list, with no separate file list below it.
  Selecting a finding opens a dedicated detail page. Breadcrumbs show
  **Findings → category → finding**; the category breadcrumb or **All findings**
  returns to the saved filters and page. Its name, category, exact path, line and declaration remain
  visible when available. The full cause leads the page, followed
  by the proposed solution, affected files and **Prepare fix** action. Provenance and confidence stay with the cause;
  **Delete** and **Mark as fixed** stay at the top. Delete uses the recoverable
  dismissed state; **Reopen** restores it. Additional evidence and acceptance
  criteria stay visible below the fix preparation. **Full explanation** expands the solution in place without
  repeating the preview; **Show less** collapses it again.
  Creation, testing and review choices are shared with Chat and configured only
  on **Models**. Opening **Models** from the sidebar preserves the selected finding so
  **Back to workspace** returns to its detail. Model selectors show the name and thinking
  level; sharing destinations stay in consent previews.
  A short note beside **Prepare fix** identifies remote sharing and isolated checks.
  **Refresh preparation** appears when loading fails or a selected model is unavailable.
  In a Git repository, **Prepare fix** opens workspace selection. **Generate changes**
  in a new feature conversation offers the same choice, preserving its selected file
  scope and test option. Use the current branch or create a worktree with an editable
  branch name and a path inside the target
  project's **.worktrees/** folder (created automatically and locally ignored by Git).
  Creation starts from the displayed commit; uncommitted files stay in the original
  workspace. The app keeps that project and branch selected, with its terminals open.
  Each worktree captures its own proposal, permissions and checks, so implementations
  can run concurrently. **Changes → Worktree implementations** reopens each task;
  Review, Apply, Commit and Undo affect only that task's checkout. The final
  preparation action authorizes generation, isolated checks, remote sharing and
  Security intent when applicable. Changed destinations
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
  **Review next file** advances through the diffs; **Apply & Commit** becomes available
  after every changed file has been viewed and the existing check/review guards pass.
  It executes immediately using the fix title as the commit message. Expand
  **Commit message** to edit it before applying.
  Only reviewed files enter the commit; unrelated staged changes remain staged.
  Pre-existing edits on the same paths must be committed first or prepared in a
  fresh worktree. Hooks and signing are skipped; nothing is pushed. Non-Git
  projects retain **Apply N files**. A failed commit leaves the files applied and
  offers **Retry commit**, which does not apply them again.
  Regeneration clears the viewed-file state. Tabs and file selection never generate,
  check or apply a proposal. Failures and requested corrections stay visible above
  the diff. Performance remains unmeasured without benchmark evidence.
  Apply records review for that exact revision and retains post-Apply verification
  and guarded Undo. Undo restores files as working-tree changes while retaining
  any Git commit. It does not automatically mark the finding fixed.
- The active branch appears directly beneath the project name in the sidebar.
  Selecting it opens **Git / Worktrees**, also available in the Project navigation.
  The page shows only clean worktrees with incoming commits that can be merged
  into the current clean local branch. Current, already merged, locked, unavailable
  and otherwise blocked worktrees are hidden. Each row has its branch title and
  three aligned actions. **View implementation** opens its saved tasks without
  changing the selected project. **Discard change** confirms removal of the clean
  checkout and its local task history without merging; the branch and committed
  changes are kept. Discard rechecks the source identity and never forces deletion.
  **Merge into current branch** executes immediately after validating the current
  source and target heads. **Preview diff** optionally shows incoming commits and a
  read-only diff. After a successful merge, the
  source worktree directory and its local task history are deleted, its implementation
  closes, and its branch is kept. Cleanup never forces removal of uncommitted changes;
  a warning explains when the merge succeeded but the worktree could not be removed.
  The Merge action authorizes local Git execution; active terminals close and
  prepared evidence is cleared. Eligibility does not predict merge conflicts.
  Conflicts remain visible with their paths and an **Abort merge** action; abort
  discards conflict resolutions.
  There is no separate new-workflow sidebar action: creation starts at **Prepare fix**.
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
  View every changed file, then select **Apply & Commit** in the review footer
  (**Apply N files** for non-Git projects). This records review and applies only
  that revision; failed review, a replacement proposal or
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
  Explanations and file analysis remain available in Source. **Inspect this file**
  also contains **Scan source** and **AI Security review**; their results open
  Security without starting further analysis. Returning to Source rechecks file freshness.
- **Overview** shows the project description below its name and project stats
  directly below its path, then puts the current task or saved analysis in a
  **Continue work** or **Your next step** band. Opening captured work does not generate or apply changes.
  **Needs attention** links Bugs, Performance, Security and active Features counts to
  their workspaces. Feature details, estimated effort and triage stay in Features;
  unavailable or ungenerated counts remain unknown, and status indicators retain
  stale/failure states. Analysis coverage stays visible beside the findings summary,
  stacking below it in compact windows. **Architecture** in the sidebar keeps
  architecture, components, engineering insights, entry points and project-flow
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
  or closing. Terminal use never grants execution trust for project checks.

The interface remembers the last project path, appearance, text size and sidebar
width in webview storage. Chat drafts remain in memory during navigation.
Private project-local `.mini-orca/changes/` metadata stores
conversations, source captures/proposals, goals, ideas and grouped recovery/check
evidence. Consent and execution trust are not restored. History lists return
summaries; chosen sessions expose their complete contents. Bounds are eight paths,
256 KiB context/response, forty messages, 2 MiB per conversation/recovery journal
and two hundred conversations. Go/Markdown proposals cannot delete files.
Performance changes remain unmeasured without external benchmark evidence.
Restoring a project never starts a provider request or shell. **⌘K / Ctrl+K** opens
file and command search. Actions have visible focus and descriptive labels.

Use **Collapse sidebar** below the navigation to keep only destination icons.
Every destination keeps its accessible name and hover label; **Expand sidebar**
restores labels and counts. The selected width is remembered across launches.

The footer's **G**, **P**, **M**, **S** and **C** controls select Graphite,
Porcelain, Midnight, Signal and Crimson Violet.
The selected theme is remembered across launches. Graphite pairs warm charcoal
with amber, Porcelain pairs ivory with forest green, and Midnight pairs navy with
cyan. Signal pairs black surfaces with red actions and yellow highlights.
Crimson Violet pairs deep purple surfaces with green actions and lilac highlights.
All themes retain labeled success, warning and failure states. **Aa** enables
larger text. Theme changes are immediate; hover transitions respect the system's
reduced-motion preference.

The header uses the matching theme's transparent MO mark beside the Mini-Orca
name. The browser favicon uses the Graphite app icon. The frontend build copies
these assets from `docs/icons/` into the embedded web assets; that directory is
the maintained source for the installed branding.

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
