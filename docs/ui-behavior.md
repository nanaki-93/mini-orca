# UI behavior contract

This is the behavior contract for the workflows the Web UI drives: what each
action may do, what it must ask first, and which states keep their meaning. It
is independent of layout; [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md)
owns how pages look and are navigated. The daemon remains the authority: the UI
offers an action only when daemon state allows it, and the daemon re-checks
every guard when the action arrives.

## Shared rules

- Navigation, selection, filters, previews, disclosures and the command palette
  never dispatch a model, execute project code or write source.
- Every mutation carries the identity it was decided on (project, revision,
  proposal revision and hash); a changed identity is rejected and the UI shows
  the daemon's message instead of retrying with fresh data.
- Remote model use, check execution, Apply, Undo, merges, trust and stopping
  agents each ask first and name their exact effect. A request is never sent
  twice behind the user's back, and one mutation runs at a time.
- Unknown, unavailable, stale, partial, failed and canceled states keep their
  meaning; model suggestions are never shown as verified results.
- Pages follow the daemon's event stream; when it drops, the status bar says
  so and the stream reconnects with backoff.

## Tasks

- **Create.** A task needs a title; scope (up to eight files) and "done when"
  criteria are explicit. **Save draft** stores it without model work.
- **Start agents.** Requires a test file in scope and a trusted project. The
  confirmation lists every remote destination the Creation, Testing and Review
  roles will use; local-only runs say so. Repairs are opt-in and capped at two.
  A security task adds a security specialist.
- **Watch.** The Conversation shows each stage with its model and duration and
  each specialist's verdict and findings. **Stop agents** asks first and applies
  nothing.
- **Spend.** Each stage of the agent run shows the tokens it spent and its
  estimated cost, the task's details show its total, and Settings → Usage
  lists every task's spend with the total. Estimates come from the daemon's
  price table; a missing rate shows a lower bound or no cost, never zero. A run
  that finishes is announced with its spend.
- **Revise.** A revision request goes to the Code profile (remote consent when
  needed) and creates a new version: earlier view marks, checks and approval no
  longer apply.
- **Stale.** A task is stale only when a file it depends on changed: its
  targets and read context, the other files of the target packages, or a build
  manifest above them. The task then reads Stale, names the changed file and
  offers no Apply; plan it again on the current source. Changes elsewhere keep
  the task current.

## Review and apply

- **View files.** Each changed file is marked viewed at the current proposal
  hash (`v` marks it and moves to the next unviewed file). "Since last review"
  shows only what changed since the last complete review.
- **Line comments.** Any line of a changed file takes a comment (the gutter
  `+`); a comment writes nothing and changes no review state. Open comments
  can be deleted, show on their file in the tree, and go to the agents as one
  repair brief with the next revision (**Revise with N comments**, or the
  revise box's "Include my line comments") or agent run; sent comments are
  kept as history.
- **Checks.** Running checks needs execution trust and runs in an isolated copy.
  Only required checks gate approval; an optional failure is a warning. When
  the project changed after checks ran, they read Outdated and must run again
  before approval or Apply.
- **Reviewer findings.** Specialists separate blocking findings from advisory
  notes; only findings request changes, and notes are shown apart from them.
- **Override reviewers.** When the reviewers rejected the current version and
  its checks pass, the human may override them with a reason (up to 1024
  bytes). The verdict stays on record, the override is shown on the run and
  binds this exact hash, and the task then waits for the human's own review.
  Nothing is approved; a human rejection cannot be overridden.
- **Approve.** Offered when every file is viewed, required checks pass and the
  workflow waits for the human. Approval binds this exact hash and answers a
  waiting workflow input; it writes nothing. Optional notes (up to 2048 bytes)
  stay with the approval and are cleared with it. **Reject proposal** ends a
  waiting workflow after confirmation.
- **Apply.** Writes the approved version to its checkout, optionally followed
  by a commit with the shown message. The confirmation names the destination,
  the branch for a commit, and the running tasks in that checkout whose files
  it changes; Apply stops only those, and other tasks keep running.
- **Issues from the keyboard.** `j`/`k` select the next or previous issue and
  open its detail, `f` starts "Fix with agents", `x` dismisses or reopens it and
  `Esc` closes the detail. Chips filter by kind (bugs, performance, security).
- **Analysis everywhere.** While an analysis runs, the status bar shows its
  progress and the sidebar counts it; the code inspector shows how each stage
  went for the open file, with its reason, and links to the dashboard.
- **Delete a task.** The task page offers Delete (a button when the task is
  stale, an icon otherwise); the task list deletes every task of the "Failed or
  stale" or "Drafts" group at once, or the selected one with Delete. One
  confirmation names the tasks; project files never change. Tasks with running
  agents, and the task whose latest Apply can still be undone, are kept.
- **Undo.** Covers only the latest Apply in a checkout and restores files only
  when they still match the applied version; a commit stays in history.

## Project

- **Open a project.** Restores saved data for a known path, otherwise imports
  and indexes it. No model request is made.
- **Set-aside files.** When the daemon set aside an unreadable agent journal,
  task file or saved analysis progress, or skipped one a newer Mini-Orca wrote,
  the Inbox says so and Settings → Project lists each file with where its bytes
  went and the remedy.
- **Agent models.** Settings → Models & agents chooses the model each role runs
  with (writes the change, writes tests, reviews): a configured scope or a model
  the Agy or Pi CLI lists, each marked local or remote. It also sets the budget
  per run within the `config.yaml` ceilings. Save applies to new runs only;
  **Use config.yaml** resets. When `config.yaml` changes under saved settings,
  they are marked out of date and new runs wait for a reset. Remote models still
  ask before each run.
- **Trust.** Granting lists the commands checks will run; revoking stops checks,
  scans and agent verification. Trust is bound to the canonical project path.
- **Analyze.** A preview comes first; the confirmation names the file count, the
  maximum model requests, remote destinations and whether a security review is
  included. Analysis writes findings and the handbook, never source. A dispatch
  window covers up to 64 files and 30 minutes; a run that reaches either pauses
  and resumes from the dashboard.
- **Analysis dashboard.** Shows the last or running run live: overall progress,
  failed stages grouped by reason (with a link to the models when the provider
  could not be reached), each section's coverage and findings (opening Issues
  filtered to that category), the models used, and every file's stages with
  their reasons, filterable to failed, with findings or in progress. Controls
  follow the run: pause and cancel while it runs; resume (after a fresh preview)
  or cancel when it is paused or interrupted; otherwise analyze changes, retry
  only failed and stale stages, or re-analyze everything.
- **Issues.** Dismiss and reopen are local triage. **Fix with agents** opens the
  composer prefilled from the finding; nothing starts until the user does.
- **Explain a file.** Sends the file's admitted context to a model after remote
  consent; the explanation is transient and cannot be applied.

## Branches

- A worktree can merge only when every task on it is applied and committed and
  the main checkout is clean; otherwise the page says which condition is missing.
- Merging needs execution trust (Git can run filters and merge drivers). The
  merge preview lists commits and the diff; the confirmation names overlapping
  paths. Overlap is a review hint, not a proven conflict.
- Conflicts stop the merge and leave it for the user's Git tool; **Abort merge**
  restores the branch after confirmation. A merged worktree's linked issues are
  marked done.
