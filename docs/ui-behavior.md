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
- **Revise.** A revision request goes to the Code profile (remote consent when
  needed) and creates a new version: earlier view marks, checks and approval no
  longer apply.
- **Stale.** When the project changed since the proposal, the task reads Stale
  and offers no Apply; plan it again on the current source.

## Review and apply

- **View files.** Each changed file is marked viewed at the current proposal
  hash (`v` marks it and moves to the next unviewed file). "Since last review"
  shows only what changed since the last complete review.
- **Checks.** Running checks needs execution trust and runs in an isolated copy.
  Only required checks gate approval; an optional failure is a warning.
- **Approve.** Offered when every file is viewed, required checks pass and the
  workflow waits for the human. Approval binds this exact hash and answers a
  waiting workflow input; it writes nothing. **Reject proposal** ends a waiting
  workflow after confirmation.
- **Apply.** Writes the approved version to its checkout, optionally followed
  by a commit with the shown message. The confirmation names the destination,
  the branch for a commit, and the running tasks in that checkout that Apply
  stops.
- **Undo.** Covers only the latest Apply in a checkout and restores files only
  when they still match the applied version; a commit stays in history.

## Project

- **Open a project.** Restores saved data for a known path, otherwise imports
  and indexes it. No model request is made.
- **Trust.** Granting lists the commands checks will run; revoking stops checks,
  scans and agent verification. Trust is bound to the canonical project path.
- **Analyze.** A preview comes first; the confirmation names the file count, the
  maximum model requests, remote destinations and whether a security review is
  included. Analysis writes findings and the handbook, never source.
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
