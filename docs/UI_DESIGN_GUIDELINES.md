# Mini-Orca UI guidelines

Mini-Orca has two interfaces over the same daemon: a terminal UI (`cmd/tui`,
`internal/tui`) and a browser workspace (`web/`, served at `/ui`). Both exist
for three jobs, in this order:

1. **Start a task and watch the agents** — describe a change, give it a scope
   and "done when" criteria, start the agents and follow every stage live.
2. **Review and apply** — read every changed file, run checks, approve the exact
   version and apply it, with Undo for the latest Apply.
3. **Understand the project** — browse code, read findings with their evidence
   and read the generated handbook.

Everything else (models, roles, skills, connections, media, missions, cloud
copies) is supporting configuration and stays one step away from these jobs.
[ui-behavior.md](ui-behavior.md) is the behavior contract for every workflow;
this document owns how the interfaces are organized, look and respond.

## Principles

- **Show the next step.** Every view answers "what needs me now?" first. Tasks
  that wait for review, failed runs and branches ready to merge rise to the top;
  the primary key or button for the selected item is always visible.
- **Detail beside the list.** Lists stay on the left; the selected item's detail
  stays on the right (stacked below on narrow screens). Moving the selection
  updates the detail without leaving the list.
- **Nothing happens by looking.** Navigation, selection, filters, previews,
  disclosures and command completion never call a model, run project code or
  write source. Every action that does names its effect before it happens.
- **States keep their meaning.** Unknown, unavailable, stale, partial, failed,
  canceled and "not run" are always labeled as such; an empty list never claims
  the project is clean, and model suggestions are never shown as verified.
- **One vocabulary.** Both interfaces use the same workspace names, task
  statuses, review gate and key bindings so moving between them costs nothing.

## Information architecture

Both interfaces have the same workspaces, in the same order:

| Key | Workspace | Contents |
| --- | --- | --- |
| `1` | **Inbox** | Needs you (tasks to review, failed analysis or checks), agents working, failed or stale tasks, branches ready to merge, then the project's issues, analysis, checks and repository. |
| `2` | **Tasks** | Every task with its brief, review gate and live agent run; views: Plan, Feature ideas, Proposals, Artifacts, Media. |
| `3` | **Code** | Review when a proposal is open (changed files beside the diff); otherwise a read-only explorer (files beside the source). |
| `4` | **Issues** | Findings with filters beside the selected finding's problem, evidence, source and proposed fix. |
| `5` | **Docs** | The handbook: context, components, scenarios and decisions, architecture. |
| `6` | **Branches** | The main checkout and agent worktrees beside the selected branch's tasks, integration state and next step. |
| `7` | **Settings** | Trust, models, roles and budgets, skills, connections, external work, bootstrap, lessons, memory, ambient analysis, cloud copies. |

The browser has no numbered keys for workspaces; it uses `g` sequences
(`g h` Inbox, `g t` Tasks, `g c` Code, `g i` Issues, `g d` Docs, `g b` Branches,
`g s` Settings) and the sidebar. A task opens on its own page there, with the
GitHub pull-request structure: **Conversation**, **Files changed**, **Checks**
and a **Ready to apply?** rail.

### Task status

A task is always in exactly one of these states, shown with the same glyph and
label everywhere:

| Glyph | Label | Meaning |
| --- | --- | --- |
| `◉` | Needs review / Changes requested | Agents finished; the human decides. |
| spinner | Agents working / Canceling | A run is active. |
| `✖` | Failed / Interrupted | The run stopped; the reason and recovery are on the task. |
| `◌` | Stale | The project changed since the proposal; plan again. |
| `○` | Draft | No run yet. |
| `✔` | Applied / Committed | Written to the checkout. |
| `⊘` | Canceled / Undone | Ended without applying. |

### Review gate

Apply is offered only when every condition holds, and the gate lists them in
this order with their current state: **Agents finished**, **Reviewers** (the
recorded verdict, else the specialists' verdicts), **Files reviewed** (every
changed file viewed at the current proposal hash), **Checks** (every required
check passed) and, in the browser, **Your approval** of this exact version. The
gate is derived from daemon state; the daemon re-checks every condition when
Apply is requested.

## Terminal UI

### Frame

The frame is four fixed rows around a body of bordered panes:

1. **Header** — brand, project, branch and modified state on the left; live
   activity (spinner and what is running, or the live-media indicator),
   execution trust and the daemon connection on the right. When space is short
   the project name shrinks first, then the branch.
2. **Workspaces** — `1 Inbox … 7 Settings` with attention counts; the active
   workspace is filled. Narrow terminals drop the hint, then show keys only with
   a dot for workspaces that need attention.
3. **Body** — one or two panes. Each pane has a rounded border, a title in the
   top border (the workspace's views as tabs: the current one emphasized,
   `Tab`/`Shift+Tab` to switch; when they do not fit, the current view and its
   position) and a position indicator (`11–20 of 48`) in the bottom border when
   it scrolls. The focused pane's border uses the accent color.
4. **Hints** — the keys that act on the selection, the primary one filled;
   `+N more` when they do not fit. Cancel is pinned on the right of confirmations.
5. **Messages** — an error, busy work with elapsed time and queued keys, or the
   latest status message. `:messages` lists them newest first.

At 96 columns or more, split views show list and detail side by side; below
that they stack, the list taking about two fifths of the rows (half below 18
rows). Below 52 × 16 a resize prompt replaces the frame without losing state.
Every row is exactly the terminal width; tests assert this for every view.

### Panes and content

- Selected list rows have a selection background and an accent bar (`▌`).
  Lists never re-center on every key: the window moves only to keep the
  selection visible.
- Section headings inside panes are short, muted and uppercase (`READY TO
  APPLY?`, `AGENT RUN`, `CHANGED FILES`); facts are label/value rows.
- Diffs show old and new line numbers, `+`/`−` signs and tinted backgrounds for
  added and removed lines; the hunk header names the line ranges and the hunk's
  position in the file. `v` cycles focused hunk, unified stream and side by
  side; `e` shows the rationale and the criteria a file claims to meet in at most
  a third of the pane, scrolled with PgUp/PgDn so code stays visible.
- Source views show a line-number gutter and the cursor line; `h`/`l` pan.
- Pages without their own layout render inside one pane titled with the
  workspace's views; their content scrolls with PgUp/PgDn and follows the
  selected row.

### Overlays

The command palette (`Ctrl+K` or `:`), confirmations, the keyboard reference
(`?`), errors and the message log float over a faint backdrop of the
workspace. On terminals narrower than 70 columns or shorter than 18 rows they
take the full width and height. Confirmations name their exact effect and pin
their actions; long messages scroll with PgUp/PgDn. An open overlay owns the
keyboard; it never forwards keys to the page behind it.

### Keyboard

| Keys | Action |
| --- | --- |
| `1`–`7`, `[` / `]` | Workspace |
| `Tab` / `Shift+Tab` | View within the workspace |
| `↑`/`↓`, `j`/`k` | Selection |
| `←`/`→`, `h`/`l` | Detail sections, diff panning, options on settings rows |
| `Enter` | Open or primary action (Apply from the diff asks first) |
| `Esc`, `Backspace`, `Ctrl+O` | Back; previous view with its selection |
| `PgUp`/`PgDn`, `Home`/`End` | Scroll the detail |
| `Ctrl+K`, `:` | Commands |
| `?` | Keyboard reference for the current view |
| `p` | Next palette (Tokyo, CRT, Amber, Paper; `NO_COLOR` is monochrome) |
| `Ctrl+C` | Quit |

Review keys: `d` open the diff, `w` mark reviewed and move on, `W` next
unreviewed without marking, `n`/`N` hunks, `f` next file, `v` layout, `z`
compare with the last review, `a` apply and commit, `u` guarded undo, `x`
reject. Task keys: `n` new task, `Enter` stages, `r` retry, `x` cancel or reject,
`m` merge the task's branch.

### Responsiveness

Blocking daemon actions run off the UI goroutine. The header and message line
name the work and its elapsed time; keys typed meanwhile replay afterwards.
`Esc` cancels read-only work; mutations (Apply, merge, Undo, trust) always
finish. Spinners animate only while work is in flight. Resizing keeps an
explicit reading position; moving the selection follows focus again.

## Browser workspace

### Shell

- **Sidebar** — project switcher, New task, the workspaces with counts,
  Settings and collapse. Collapsed, it keeps icons with tooltips.
- **Top bar** — the checkout menu (main checkout or an agent worktree; every
  page then shows that checkout), the page title and the command palette
  trigger (`⌘K` / `Ctrl+K`).
- **Status bar** — connection to the live event stream, busy work, running
  agents, trust, models, project path, version and the shortcut reference.

Pages update from the daemon's event stream; a lost stream reconnects with
backoff and the status bar says so. Light, dark and system themes share one
token set; keep text at 4.5:1 contrast or better against its surface.

### Pages

- **Inbox** — grouped rows (Needs your review, Agents working, Failed, Ready to
  merge, Recently applied) with open issues and analysis coverage beside them.
- **Tasks** — grouped list with status filters and `j`/`k`/`Enter`.
- **Task** — Conversation (brief, criteria, scope, the agent timeline with
  reviewer verdicts, messages, revise), Files changed (file tree with viewed
  marks and progress, unified or split diff with expandable context, "since last
  review"), Checks (results with output), and the Ready to apply? rail with Apply,
  commit message and Undo.
- **Code** — virtualized file tree, editor tabs per checkout, highlighted
  source with issue markers and line links, an inspector with Explain and "New
  task with this file".
- **Issues**, **Docs**, **Branches** (merge preview with commits and diff, merge
  and abort) and **Settings** follow the same list-and-detail structure.

### Task composer

Title, optional description, kind (fix, feature, performance, security), the
files agents may change (up to eight; a companion test is suggested for every Go
file and a test file is required to start agents) and "done when" criteria.
**Save draft** and **Start agents** (`⌘↵`) are explicit; starting lists every
remote model destination for consent first.

### Implementation rules

- React + TypeScript; server state through TanStack Query only; one hash
  router whose `wt` parameter carries the selected checkout.
- Strict CSP: same-origin scripts, styles and fonts, no inline script, and one
  per-response nonce for the style tag dialogs inject. Never add inline styles
  through HTML strings; React style props are set through the CSSOM.
- Every request carries the Web UI marker header; mutations carry the guarded
  proposal identity; failures show the daemon's user message.

## Accessibility

- Color always pairs with a glyph or a word; `NO_COLOR` and monochrome palettes
  rely on bold, faint and reverse video.
- Every interactive element is reachable by keyboard; the browser shows focus
  rings and uses Radix primitives for dialogs, menus, tabs and tooltips.
- Text stays readable at the smallest supported size: nothing that explains
  an action or a failure is truncated without a way to read it in full.

## Verification

- TUI: `make tui-test` (`go test -race ./internal/tui/... ./cmd/tui/...`). The
  frame tests render every workspace and overlay at sizes from 52 × 16 to
  220 × 60 and assert exact frame size; behavior tests cover navigation,
  passive browsing, consent and identity guards. `MINI_ORCA_TUI_FRAMES=<dir> go
  test ./internal/tui -run TestWriteFramesForVisualReview` writes ANSI frames of
  every view for visual review.
- Browser: `make webui-test` (type-check and vitest), `make webui-dist-check`,
  `go test ./internal/api/handlers ./cmd/daemon`, and a browser check of changed
  pages in dark and light themes with no console or CSP errors.
