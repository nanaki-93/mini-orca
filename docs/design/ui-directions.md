# Web workspace design directions

These are design concepts for the browser workspace alongside the shipped UI.
Open [the interactive comparison](ui-directions.html) in a browser. It is
self-contained and works offline.

All four examples show the same fictional task, **Validate incoming requests**,
so the layout is the variable. The preview has no API connection, provider calls,
project execution, or source writes. Review interactions only change local demo
state. Check results are explicitly labeled as samples.

| Direction | Structure | Visual style | Best fit | Tradeoff |
| --- | --- | --- | --- | --- |
| **Orbit** | Developer dashboard + Lite IDE with contextual file explorer and split/unified diff toolbar | Deep navy, periwinkle-indigo accents, crisp 1px borders, monospace telemetry | A developer-friendly Lite IDE with a calm overview of work and one-click file review | Slightly denser chrome during code review |
| **Paper** | Top navigation, centered document, inline file selection | Warm light surfaces, ink text, muted green | Reading, planning, and slower review | Less code visible at once |
| **Pulse** | Task inbox beside the selected task | Graphite green, mint accents, compact rows | Reviewing several proposals in sequence | Less width for long code lines |
| **Focus** | Guided task stages, a single canvas, disclosed navigation | Midnight blue, soft blue accents, generous spacing | The clearest next action with minimal visual noise | More navigation to switch tasks |

**Orbit** is the selected direction, refined into a developer-friendly dashboard
and lightweight IDE hybrid:

- **Workspace Dashboard (`Home`)**: Shows workspace telemetry (changed files,
  reviewed progress, sample check status, and guarded apply state) alongside the
  active task card with an interactive pipeline strip (`Brief` → `Review` →
  `Checks` → `Guarded Apply`) and direct file-jump chips (`internal/server.go`,
  `internal/server_test.go`).
- **Lite IDE Review (`Review`)**: The slim activity rail (`Home`, `Review`,
  `Brief`, `Checks`) stays visible, while the contextual file explorer and
  review-gate summary appear only on the **Review** view. The diff editor
  provides file tabs, dual-column line numbers, a hunk header bar, and an
  interactive `Unified` / `Split` diff mode switch.
- **Responsive layout**: Brief and Checks use the full content canvas. Returning
  to Review preserves the selected file, diff layout mode, and local review
  progress. On narrow screens, changed files move above the diff instead of
  squeezing it into a narrow column.

Paper, Pulse, and Focus remain available for comparison.

Each example supports task-section navigation, file selection, local review
progress, and an explanation disclosure. Controls that would approve or apply
real changes instead open an explicit preview notice. The additional task entries
in Pulse illustrate the queue structure and are display-only examples.

The `style` query parameter selects `orbit`, `paper`, `pulse`, or `focus`. Orbit
opens on its dashboard; the other examples open on changes. The optional `view`
parameter opens `overview`, `changes`, or `checks` directly (`dashboard` is also
available for Orbit). For example,
[Orbit review](ui-directions.html?style=orbit&view=changes) opens the Lite IDE
explorer directly. `capture=1` hides the comparison controls for inspecting a
single concept. The production UI lives in `internal/api/handlers/webui/` as
embedded ES modules (`index.html`, `app.css`, `js/`).
