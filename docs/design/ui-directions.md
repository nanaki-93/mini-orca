# Web workspace design directions

These are proposed alternatives to the current browser workspace, not shipped
product changes. Open [the interactive comparison](ui-directions.html) in a
browser. It is self-contained and works offline.

All four examples show the same fictional task, **Validate incoming requests**,
so the layout is the variable. The preview has no API connection, provider calls,
project execution, or source writes. Review interactions only change local demo
state. Check results are explicitly labeled as samples.

| Direction | Structure | Visual style | Best fit | Tradeoff |
| --- | --- | --- | --- | --- |
| **Orbit** | Dashboard, task pages, explorer only during Review | Deep navy, lavender, precise borders | A lightweight IDE with a calm overview of work | One extra step from the dashboard to code |
| **Paper** | Top navigation, centered document, inline file selection | Warm light surfaces, ink text, muted green | Reading, planning, and slower review | Less code visible at once |
| **Pulse** | Task inbox beside the selected task | Graphite green, mint accents, compact rows | Reviewing several proposals in sequence | Less width for long code lines |
| **Focus** | Guided task stages, a single canvas, disclosed navigation | Midnight blue, soft blue accents, generous spacing | The clearest next action with minimal visual noise | More navigation to switch tasks |

**Orbit** is the selected direction, refined into a dashboard and lightweight IDE
hybrid. Home shows the current task and its next action, with separate links to
the brief and checks. The slim rail stays visible; the file explorer appears only
on the task's **Review** page. Brief and Checks use the full content area. Returning
to Review preserves the selected file and local review progress. On narrow screens,
changed files move above the diff instead of squeezing it into a narrow column.

Paper, Pulse, and Focus remain available for comparison.

Each example supports task-section navigation, file selection, local review
progress, and an explanation disclosure. Controls that would approve or apply
real changes instead open an explicit preview notice. The additional task entries
in Pulse illustrate the queue structure and are display-only examples.

The `style` query parameter selects `orbit`, `paper`, `pulse`, or `focus`. Orbit
opens on its dashboard; the other examples open on changes. The optional `view`
parameter opens `overview`, `changes`, or `checks` directly (`dashboard` is also
available for Orbit). For example,
[Orbit review](ui-directions.html?style=orbit&view=changes) opens the explorer.
`capture=1` hides the comparison controls for inspecting a single concept.
The production UI remains in `internal/api/handlers/webui.html`.
